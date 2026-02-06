package chatgear

import (
	"context"
	"iter"
	"sync"
	"time"
	"unsafe"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
	"github.com/haivivi/giztoy/go/pkg/audio/pcm"
	"github.com/haivivi/giztoy/go/pkg/buffer"
	"github.com/haivivi/giztoy/go/pkg/jsontime"
)

// Mic is the microphone input interface.
type Mic interface {
	// Read reads PCM data from microphone.
	Read(pcm []int16) (int, error)
	// Format returns the PCM format.
	Format() pcm.Format
}

// Speaker is the speaker output interface.
type Speaker interface {
	// Write writes PCM data to speaker.
	Write(pcm []int16) (int, error)
	// Format returns the PCM format.
	Format() pcm.Format
}

// ClientPort is a bidirectional audio port for client-side (device) communication.
// It provides ReadFrom/WriteTo methods to connect with DownlinkRx/UplinkTx,
// and ReadFromMic/WriteToSpeaker to connect with physical audio devices.
//
// Usage:
//
//	port := chatgear.NewClientPort()
//	go port.ReadFromMic(mic)          // mic -> server
//	go port.WriteToSpeaker(speaker)   // server -> speaker
//	go port.ReadFrom(downlinkRx)      // read from server
//	go port.WriteTo(uplinkTx)         // write to server
//	for cmd, err := range port.Commands() {
//	    // handle commands from server
//	}
type ClientPort struct {
	// Downlink - from server
	downlinkAudio *buffer.Buffer[StampedOpusFrame]
	commandQueue  *buffer.Buffer[*CommandEvent]

	// Uplink - to server
	uplinkAudio *buffer.Buffer[StampedOpusFrame]
	uplinkState *buffer.Buffer[*StateEvent]
	uplinkStats *buffer.Buffer[*StatsEvent]

	// Internal state
	mu           sync.RWMutex
	state        State
	stats        *StatsEvent // Full stats storage
	statsPending *StatsEvent // Only changed fields (for diff upload)
	batchMode    bool        // When true, Set* methods don't queue updates
	closed       bool

	logger Logger
}

// NewClientPort creates a new ClientPort.
func NewClientPort() *ClientPort {
	return &ClientPort{
		downlinkAudio: buffer.N[StampedOpusFrame](256),
		commandQueue:  buffer.N[*CommandEvent](32),
		uplinkAudio:   buffer.N[StampedOpusFrame](256),
		uplinkState:   buffer.N[*StateEvent](32),
		uplinkStats:   buffer.N[*StatsEvent](32),
		stats:         &StatsEvent{},
		logger:        DefaultLogger(),
	}
}

// =============================================================================
// Physical Layer: Mic -> Uplink
// =============================================================================

// ReadFromMic reads audio from the given Mic and queues it for upload.
// This method blocks until the mic is closed or an error occurs.
// Use `go port.ReadFromMic(mic)` for non-blocking operation.
func (p *ClientPort) ReadFromMic(mic Mic) error {
	if mic == nil {
		return nil
	}

	encoder, err := opus.NewVoIPEncoder(mic.Format().SampleRate(), mic.Format().Channels())
	if err != nil {
		return err
	}
	defer encoder.Close()

	frameDuration := 20 * time.Millisecond
	frameSize := int(mic.Format().SamplesInDuration(frameDuration))
	pcmBuf := make([]int16, frameSize)

	var stamp time.Time
	for {
		n, err := mic.Read(pcmBuf)
		if err != nil {
			return err
		}

		opusFrame, err := encoder.Encode(pcmBuf[:n], n)
		if err != nil {
			return err
		}

		now := time.Now()
		if stamp.Before(now) {
			stamp = now
		}

		if len(opusFrame) == 0 {
			stamp = stamp.Add(frameDuration)
			continue
		}

		frame := StampedOpusFrame{
			Timestamp: stamp,
			Frame:     opusFrame,
		}
		p.logger.DebugPrintf("ReadFromMic: queueing opus frame len=%d ts=%v", len(opusFrame), stamp.Format("15:04:05.000"))
		if err := p.uplinkAudio.Add(frame); err != nil {
			return err
		}
		stamp = stamp.Add(frameDuration)
	}
}

// =============================================================================
// Physical Layer: Downlink -> Speaker
// =============================================================================

// WriteToSpeaker plays audio to the given Speaker from the downlink queue.
// This method blocks until the queue is closed or an error occurs.
// Use `go port.WriteToSpeaker(speaker)` for non-blocking operation.
func (p *ClientPort) WriteToSpeaker(speaker Speaker) error {
	if speaker == nil {
		return nil
	}

	decoder, err := opus.NewDecoder(speaker.Format().SampleRate(), speaker.Format().Channels())
	if err != nil {
		return err
	}
	defer decoder.Close()

	for {
		frame, err := p.downlinkAudio.Next()
		if err != nil {
			if err == buffer.ErrIteratorDone {
				return nil
			}
			return err
		}

		pcmBytes, err := decoder.Decode(frame.Frame)
		if err != nil {
			p.logger.ErrorPrintf("decode opus frame: %v", err)
			continue
		}

		// Convert []byte to []int16
		pcmSamples := bytesToInt16(pcmBytes)
		if _, err := speaker.Write(pcmSamples); err != nil {
			return err
		}
	}
}

// bytesToInt16 converts a byte slice to an int16 slice without copying.
func bytesToInt16(b []byte) []int16 {
	return unsafe.Slice((*int16)(unsafe.Pointer(&b[0])), len(b)/2)
}

// =============================================================================
// Network Layer: Downlink (Server -> Client)
// =============================================================================

// ReadFrom reads data from the given DownlinkRx and queues it.
// This method blocks until the DownlinkRx is closed or an error occurs.
// Use `go port.ReadFrom(rx)` for non-blocking operation.
func (p *ClientPort) ReadFrom(rx DownlinkRx) error {
	defer rx.Close()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	setErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
	}

	wg.Add(2)

	// Read opus frames
	go func() {
		defer wg.Done()
		p.logger.DebugPrintf("ReadFrom: audio reader started")
		for frame, err := range rx.OpusFrames() {
			if err != nil {
				setErr(err)
				return
			}
			p.logger.DebugPrintf("ReadFrom: received opus frame len=%d ts=%v", len(frame.Frame), frame.Timestamp.Format("15:04:05.000"))
			if err := p.downlinkAudio.Add(frame); err != nil {
				setErr(err)
				return
			}
		}
	}()

	// Read commands
	go func() {
		defer wg.Done()
		for cmd, err := range rx.Commands() {
			if err != nil {
				setErr(err)
				return
			}
			if err := p.commandQueue.Add(cmd); err != nil {
				setErr(err)
				return
			}
		}
	}()

	wg.Wait()
	return firstErr
}

// Commands returns an iterator for commands from the server.
func (p *ClientPort) Commands() iter.Seq2[*CommandEvent, error] {
	return func(yield func(*CommandEvent, error) bool) {
		for {
			cmd, err := p.commandQueue.Next()
			if err != nil {
				if err == buffer.ErrIteratorDone {
					return
				}
				yield(nil, err)
				return
			}
			if !yield(cmd, nil) {
				return
			}
		}
	}
}

// =============================================================================
// Network Layer: Uplink (Client -> Server)
// =============================================================================

// WriteTo writes data to the given UplinkTx from the uplink queues.
// This method blocks until all queues are closed or an error occurs.
// Use `go port.WriteTo(tx)` for non-blocking operation.
func (p *ClientPort) WriteTo(tx UplinkTx) error {
	defer tx.Close()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	setErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
	}

	wg.Add(3)

	// Write audio frames
	go func() {
		defer wg.Done()
		p.logger.DebugPrintf("WriteTo: audio writer started")
		for {
			frame, err := p.uplinkAudio.Next()
			if err != nil {
				if err == buffer.ErrIteratorDone {
					p.logger.DebugPrintf("WriteTo: audio iterator done")
					return
				}
				setErr(err)
				return
			}
			p.logger.DebugPrintf("WriteTo: sending opus frame len=%d ts=%v", len(frame.Frame), frame.Timestamp.Format("15:04:05.000"))
			if err := tx.SendOpusFrame(frame.Timestamp, frame.Frame); err != nil {
				setErr(err)
				return
			}
		}
	}()

	// Write state events
	go func() {
		defer wg.Done()
		for {
			state, err := p.uplinkState.Next()
			if err != nil {
				if err == buffer.ErrIteratorDone {
					return
				}
				setErr(err)
				return
			}
			if err := tx.SendState(state); err != nil {
				setErr(err)
				return
			}
		}
	}()

	// Write stats events
	go func() {
		defer wg.Done()
		for {
			stats, err := p.uplinkStats.Next()
			if err != nil {
				if err == buffer.ErrIteratorDone {
					return
				}
				setErr(err)
				return
			}
			if err := tx.SendStats(stats); err != nil {
				setErr(err)
				return
			}
		}
	}()

	wg.Wait()
	return firstErr
}

// =============================================================================
// State Management
// =============================================================================

// SetState sets the state and queues a state event for upload.
func (p *ClientPort) SetState(s State) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == s {
		return
	}
	p.state = s

	now := time.Now()
	evt := &StateEvent{
		Version:  1,
		Time:     jsontime.NowEpochMilli(),
		State:    s,
		UpdateAt: jsontime.Milli(now),
	}

	p.uplinkState.Add(evt)
}

// State returns the current state.
func (p *ClientPort) State() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// =============================================================================
// Stats Management
// =============================================================================

// queueFullStats sends the full stats (used for batch end / initialization).
func (p *ClientPort) queueFullStats() {
	p.stats.Time = jsontime.NowEpochMilli()
	evt := p.stats.Clone()
	p.uplinkStats.Add(evt)
}

// notifyStatsSend sends pending stats diff and clears pending.
// Must be called with lock held.
func (p *ClientPort) notifyStatsSend() {
	if p.statsPending == nil {
		return
	}
	p.statsPending.Time = jsontime.NowEpochMilli()
	p.uplinkStats.Add(p.statsPending)
	p.statsPending = nil
}

// BeginBatch starts batch mode - Set* methods won't queue updates until EndBatch.
func (p *ClientPort) BeginBatch() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.batchMode = true
}

// EndBatch ends batch mode and queues one full stats update with all changes.
func (p *ClientPort) EndBatch() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.batchMode = false
	p.queueFullStats()
}

// SetVolume sets the volume and queues a stats diff update.
func (p *ClientPort) SetVolume(volume int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update storage layer
	if p.stats.Volume == nil {
		p.stats.Volume = &Volume{}
	}
	now := jsontime.NowEpochMilli()
	p.stats.Volume.Percentage = float64(volume)
	p.stats.Volume.UpdateAt = now

	if p.batchMode {
		return
	}

	// Update pending layer (diff only)
	if p.statsPending == nil {
		p.statsPending = &StatsEvent{}
	}
	p.statsPending.Volume = &Volume{
		Percentage: float64(volume),
		UpdateAt:   now,
	}
	p.notifyStatsSend()
}

// SetBrightness sets the brightness and queues a stats diff update.
func (p *ClientPort) SetBrightness(brightness int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update storage layer
	if p.stats.Brightness == nil {
		p.stats.Brightness = &Brightness{}
	}
	now := jsontime.NowEpochMilli()
	p.stats.Brightness.Percentage = float64(brightness)
	p.stats.Brightness.UpdateAt = now

	if p.batchMode {
		return
	}

	// Update pending layer (diff only)
	if p.statsPending == nil {
		p.statsPending = &StatsEvent{}
	}
	p.statsPending.Brightness = &Brightness{
		Percentage: float64(brightness),
		UpdateAt:   now,
	}
	p.notifyStatsSend()
}

// SetLightMode sets the light mode and queues a stats diff update.
func (p *ClientPort) SetLightMode(mode string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update storage layer
	if p.stats.LightMode == nil {
		p.stats.LightMode = &LightMode{}
	}
	now := jsontime.NowEpochMilli()
	p.stats.LightMode.Mode = mode
	p.stats.LightMode.UpdateAt = now

	if p.batchMode {
		return
	}

	// Update pending layer (diff only)
	if p.statsPending == nil {
		p.statsPending = &StatsEvent{}
	}
	p.statsPending.LightMode = &LightMode{
		Mode:     mode,
		UpdateAt: now,
	}
	p.notifyStatsSend()
}

// SetBattery sets the battery status and queues a stats diff update.
func (p *ClientPort) SetBattery(pct int, isCharging bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update storage layer
	if p.stats.Battery == nil {
		p.stats.Battery = &Battery{}
	}
	p.stats.Battery.Percentage = float64(pct)
	p.stats.Battery.IsCharging = isCharging

	if p.batchMode {
		return
	}

	// Update pending layer (diff only)
	if p.statsPending == nil {
		p.statsPending = &StatsEvent{}
	}
	p.statsPending.Battery = &Battery{
		Percentage: float64(pct),
		IsCharging: isCharging,
	}
	p.notifyStatsSend()
}

// SetWifiNetwork sets the connected WiFi network and queues a stats diff update.
func (p *ClientPort) SetWifiNetwork(wifi *ConnectedWifi) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update storage layer
	p.stats.WifiNetwork = wifi

	if p.batchMode {
		return
	}

	// Update pending layer (diff only)
	if p.statsPending == nil {
		p.statsPending = &StatsEvent{}
	}
	p.statsPending.WifiNetwork = wifi
	p.notifyStatsSend()
}

// SetWifiStore sets the stored WiFi list and queues a stats diff update.
func (p *ClientPort) SetWifiStore(store *StoredWifiList) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update storage layer
	now := jsontime.NowEpochMilli()
	if store != nil {
		store.UpdateAt = now
	}
	p.stats.WifiStore = store

	if p.batchMode {
		return
	}

	// Update pending layer (diff only)
	if p.statsPending == nil {
		p.statsPending = &StatsEvent{}
	}
	p.statsPending.WifiStore = store
	p.notifyStatsSend()
}

// SetSystemVersion sets the system version and queues a stats diff update.
func (p *ClientPort) SetSystemVersion(version string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update storage layer
	if p.stats.SystemVersion == nil {
		p.stats.SystemVersion = &SystemVersion{}
	}
	p.stats.SystemVersion.CurrentVersion = version

	if p.batchMode {
		return
	}

	// Update pending layer (diff only)
	if p.statsPending == nil {
		p.statsPending = &StatsEvent{}
	}
	p.statsPending.SystemVersion = &SystemVersion{
		CurrentVersion: version,
	}
	p.notifyStatsSend()
}

// SetCellular sets the cellular network and queues a stats diff update.
func (p *ClientPort) SetCellular(cellular *ConnectedCellular) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update storage layer
	p.stats.Cellular = cellular

	if p.batchMode {
		return
	}

	// Update pending layer (diff only)
	if p.statsPending == nil {
		p.statsPending = &StatsEvent{}
	}
	p.statsPending.Cellular = cellular
	p.notifyStatsSend()
}

// SetPairStatus sets the pair status and queues a stats diff update.
func (p *ClientPort) SetPairStatus(pairWith string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update storage layer
	if p.stats.PairStatus == nil {
		p.stats.PairStatus = &PairStatus{}
	}
	now := jsontime.NowEpochMilli()
	p.stats.PairStatus.PairWith = pairWith
	p.stats.PairStatus.UpdateAt = now

	if p.batchMode {
		return
	}

	// Update pending layer (diff only)
	if p.statsPending == nil {
		p.statsPending = &StatsEvent{}
	}
	p.statsPending.PairStatus = &PairStatus{
		PairWith: pairWith,
		UpdateAt: now,
	}
	p.notifyStatsSend()
}

// SetReadNFCTag sets the NFC tag and queues a stats diff update.
func (p *ClientPort) SetReadNFCTag(tag *ReadNFCTag) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update storage layer
	p.stats.ReadNFCTag = tag

	if p.batchMode {
		return
	}

	// Update pending layer (diff only)
	if p.statsPending == nil {
		p.statsPending = &StatsEvent{}
	}
	p.statsPending.ReadNFCTag = tag
	p.notifyStatsSend()
}

// SetShaking sets the shaking level and queues a stats diff update.
func (p *ClientPort) SetShaking(level float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update storage layer
	if p.stats.Shaking == nil {
		p.stats.Shaking = &Shaking{}
	}
	p.stats.Shaking.Level = level

	if p.batchMode {
		return
	}

	// Update pending layer (diff only)
	if p.statsPending == nil {
		p.statsPending = &StatsEvent{}
	}
	p.statsPending.Shaking = &Shaking{
		Level: level,
	}
	p.notifyStatsSend()
}

// =============================================================================
// Periodic Reporting (Protocol)
// =============================================================================

// StartPeriodicReporting starts background state/stats reporting goroutines.
// This is part of the chatgear protocol - devices must periodically report state and stats.
// Call this after creating the ClientPort and before starting the network loops.
func (p *ClientPort) StartPeriodicReporting(ctx context.Context) {
	go p.stateSendLoop(ctx)
	go p.statsReportLoop(ctx)
}

// stateSendLoop sends state every 5 seconds unconditionally.
// This matches the C implementation behavior.
func (p *ClientPort) stateSendLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.mu.RLock()
			currentState := p.state
			p.mu.RUnlock()

			now := time.Now()
			evt := &StateEvent{
				Version:  1,
				Time:     jsontime.Milli(now),
				State:    currentState,
				UpdateAt: jsontime.Milli(now),
			}
			p.uplinkState.Add(evt)
		}
	}
}

// statsReportLoop implements tiered stats reporting:
// - Every 1 minute (20s * 3): battery, volume, brightness, light_mode, sys_ver, wifi
// - Every 2 minutes (20s * 6): shaking, cellular
// - Every 10 minutes (20s * 30): wifi_store
// Note: Initial stats should be sent via BeginBatch/EndBatch before calling StartPeriodicReporting.
func (p *ClientPort) statsReportLoop(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	rounds := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rounds++
			p.sendPeriodicStats(rounds)
		}
	}
}

// sendPeriodicStats sends stats based on the tiered reporting schedule.
// Uses the two-layer architecture: sets statsPending then calls notifyStatsSend.
func (p *ClientPort) sendPeriodicStats(rounds int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := jsontime.NowEpochMilli()

	// Ensure pending is initialized
	if p.statsPending == nil {
		p.statsPending = &StatsEvent{}
	}

	hasFields := false

	switch rounds % 3 {
	case 0:
		// Every 1 minute (20s * 3 = 60s)
		// Report: battery, volume, brightness, light_mode, sys_ver, wifi, pair_status
		if p.stats.Battery != nil {
			p.statsPending.Battery = &Battery{
				Percentage: p.stats.Battery.Percentage,
				IsCharging: p.stats.Battery.IsCharging,
			}
			hasFields = true
		}
		if p.stats.Volume != nil {
			p.statsPending.Volume = &Volume{
				Percentage: p.stats.Volume.Percentage,
				UpdateAt:   now,
			}
			hasFields = true
		}
		if p.stats.Brightness != nil {
			p.statsPending.Brightness = &Brightness{
				Percentage: p.stats.Brightness.Percentage,
				UpdateAt:   now,
			}
			hasFields = true
		}
		if p.stats.LightMode != nil {
			p.statsPending.LightMode = &LightMode{
				Mode:     p.stats.LightMode.Mode,
				UpdateAt: now,
			}
			hasFields = true
		}
		if p.stats.SystemVersion != nil {
			p.statsPending.SystemVersion = &SystemVersion{
				CurrentVersion: p.stats.SystemVersion.CurrentVersion,
			}
			hasFields = true
		}
		if p.stats.WifiNetwork != nil {
			p.statsPending.WifiNetwork = &ConnectedWifi{
				SSID:    p.stats.WifiNetwork.SSID,
				RSSI:    p.stats.WifiNetwork.RSSI,
				IP:      p.stats.WifiNetwork.IP,
				Gateway: p.stats.WifiNetwork.Gateway,
			}
			hasFields = true
		}
		// Always include pair_status in periodic reports (as per C implementation)
		if p.stats.PairStatus != nil {
			p.statsPending.PairStatus = &PairStatus{
				PairWith: p.stats.PairStatus.PairWith,
				UpdateAt: now,
			}
			hasFields = true
		}

	case 1:
		// Every 2 minutes (rounds % 6 == 1)
		if rounds%6 != 1 {
			return
		}
		// Report: shaking, cellular
		if p.stats.Shaking != nil {
			p.statsPending.Shaking = &Shaking{
				Level: p.stats.Shaking.Level,
			}
			hasFields = true
		}
		if p.stats.Cellular != nil {
			p.statsPending.Cellular = p.stats.Cellular
			hasFields = true
		}

	case 2:
		// Every 10 minutes (rounds % 30 == 2)
		if rounds%30 != 2 {
			return
		}
		// Report: wifi_store
		if p.stats.WifiStore != nil {
			p.statsPending.WifiStore = &StoredWifiList{
				List:     p.stats.WifiStore.List,
				UpdateAt: now,
			}
			hasFields = true
		}
	}

	// Only send if there's something to report
	if hasFields {
		p.notifyStatsSend()
	}
}

// =============================================================================
// Lifecycle
// =============================================================================

// Close closes the port.
func (p *ClientPort) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	p.downlinkAudio.Close()
	p.commandQueue.Close()
	p.uplinkAudio.Close()
	p.uplinkState.Close()
	p.uplinkStats.Close()
	return nil
}
