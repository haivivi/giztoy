package chatgear

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
	"github.com/haivivi/giztoy/go/pkg/audio/pcm"
	"github.com/haivivi/giztoy/go/pkg/buffer"
)

// UplinkData represents data received from the device.
type UplinkData struct {
	// Audio is set when this is an audio frame.
	Audio *StampedOpusFrame
	// State is set when this is a state event.
	State *StateEvent
	// StatsChanges is set when there are stats changes.
	StatsChanges *StatsChanges
}

// ServerPort is a bidirectional audio port for server-side communication.
// It provides ReadFrom/WriteTo methods to connect with UplinkRx/DownlinkTx.
//
// Usage:
//
//	port := chatgear.NewServerPort()
//	go port.ReadFrom(uplinkRx)   // read from device
//	go port.WriteTo(downlinkTx)  // write to device
//	for {
//	    data, err := port.Poll()  // process uplink data
//	    // ...
//	    port.SetVolume(50)         // issue command
//	}
type ServerPort struct {
	// Uplink - from device
	uplinkQueue *buffer.Buffer[UplinkData]

	// Downlink - to device
	mixer        *pcm.Mixer
	background   *pcm.TrackCtrl
	foreground   *pcm.TrackCtrl
	overlay      *pcm.TrackCtrl
	commandQueue *buffer.Buffer[*CommandEvent]

	// State
	mu     sync.RWMutex
	stats  *StatsEvent
	state  *StateEvent
	closed bool

	logger Logger
}

// NewServerPort creates a new ServerPort.
func NewServerPort() *ServerPort {
	p := &ServerPort{
		uplinkQueue:  buffer.N[UplinkData](256),
		commandQueue: buffer.N[*CommandEvent](32),
		logger:       DefaultLogger(),
	}

	// Track count for streaming state management
	var trackCount int32
	p.mixer = pcm.NewMixer(pcm.L16Mono24K,
		pcm.WithOnTrackCreated(func() {
			if atomic.AddInt32(&trackCount, 1) == 1 {
				// First track created - start streaming
				p.IssueCommand(NewStreaming(true))
			}
		}),
		pcm.WithOnTrackClosed(func() {
			if atomic.AddInt32(&trackCount, -1) == 0 {
				// Last track closed - stop streaming
				p.IssueCommand(NewStreaming(false))
			}
		}),
	)
	return p
}

// NewServerPortWithMixer creates a new ServerPort with a custom mixer.
// This is useful for testing or when you need custom mixer options.
func NewServerPortWithMixer(mixer *pcm.Mixer) *ServerPort {
	return &ServerPort{
		uplinkQueue:  buffer.N[UplinkData](256),
		mixer:        mixer,
		commandQueue: buffer.N[*CommandEvent](32),
		logger:       DefaultLogger(),
	}
}

// =============================================================================
// Uplink: Device -> Server
// =============================================================================

// ReadFrom reads data from the given UplinkRx and queues it for Poll().
// This method blocks until the UplinkRx is closed or an error occurs.
// Use `go port.ReadFrom(rx)` for non-blocking operation.
func (p *ServerPort) ReadFrom(rx UplinkRx) error {
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

	wg.Add(3)

	// Read opus frames
	go func() {
		defer wg.Done()
		for frame, err := range rx.OpusFrames() {
			if err != nil {
				setErr(err)
				return
			}
			frameCopy := frame // copy to avoid closure capture issues
			data := UplinkData{Audio: &frameCopy}
			if err := p.uplinkQueue.Add(data); err != nil {
				setErr(err)
				return
			}
		}
	}()

	// Read state events
	go func() {
		defer wg.Done()
		for state, err := range rx.States() {
			if err != nil {
				setErr(err)
				return
			}
			p.handleStateEvent(state)
			data := UplinkData{State: state}
			if err := p.uplinkQueue.Add(data); err != nil {
				setErr(err)
				return
			}
		}
	}()

	// Read stats events
	go func() {
		defer wg.Done()
		for stats, err := range rx.Stats() {
			if err != nil {
				setErr(err)
				return
			}
			changes := p.handleStatsEvent(stats)
			if changes == nil {
				continue
			}
			data := UplinkData{StatsChanges: changes}
			if err := p.uplinkQueue.Add(data); err != nil {
				setErr(err)
				return
			}
		}
	}()

	wg.Wait()
	return firstErr
}

// Poll returns the next uplink data.
// This method blocks until data is available or the port is closed.
func (p *ServerPort) Poll() (UplinkData, error) {
	return p.uplinkQueue.Next()
}

// =============================================================================
// Handle Methods (for Listener to push data)
// =============================================================================

// HandleAudio handles an incoming audio frame from the device.
// This method is called by the Listener when a new audio frame is received.
func (p *ServerPort) HandleAudio(frame *StampedOpusFrame) {
	if frame == nil {
		return
	}
	data := UplinkData{Audio: frame}
	p.uplinkQueue.Add(data)
}

// HandleState handles an incoming state event from the device.
// This method is called by the Listener when a new state event is received.
func (p *ServerPort) HandleState(state *StateEvent) {
	if state == nil {
		return
	}
	p.handleStateEvent(state)
	data := UplinkData{State: state}
	p.uplinkQueue.Add(data)
}

// HandleStats handles an incoming stats event from the device.
// This method is called by the Listener when a new stats event is received.
func (p *ServerPort) HandleStats(stats *StatsEvent) {
	if stats == nil {
		return
	}
	changes := p.handleStatsEvent(stats)
	if changes == nil {
		return
	}
	data := UplinkData{StatsChanges: changes}
	p.uplinkQueue.Add(data)
}

// handleStateEvent updates internal state from a state event.
func (p *ServerPort) handleStateEvent(e *StateEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Filter out-of-order events
	if p.state != nil && e.Time.Before(p.state.Time) {
		return
	}
	p.state = e.Clone()
}

// handleStatsEvent updates internal stats from a stats event.
// Returns the changes if any, nil otherwise.
func (p *ServerPort) handleStatsEvent(e *StatsEvent) *StatsChanges {
	p.mu.Lock()
	defer p.mu.Unlock()

	var changes *StatsChanges
	if p.stats != nil {
		changes = p.stats.MergeWith(e.Clone())
	} else {
		p.stats = e.Clone()
	}
	return changes
}

// =============================================================================
// Downlink: Server -> Device
// =============================================================================

// WriteTo writes data to the given DownlinkTx.
// This method blocks, reading from the mixer and command queue.
// Use `go port.WriteTo(tx)` for non-blocking operation.
func (p *ServerPort) WriteTo(tx DownlinkTx) error {
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

	wg.Add(2)

	// Write audio frames from mixer
	go func() {
		defer wg.Done()
		p.streamAudioTo(tx, setErr)
	}()

	// Write commands
	go func() {
		defer wg.Done()
		for {
			cmd, err := p.commandQueue.Next()
			if err != nil {
				if err == buffer.ErrIteratorDone {
					return
				}
				setErr(err)
				return
			}
			if err := tx.IssueCommand(cmd.Payload, cmd.IssueAt.Time()); err != nil {
				setErr(err)
				return
			}
		}
	}()

	wg.Wait()
	return firstErr
}

const bufferingDuration = 360 * time.Millisecond

// streamAudioTo streams audio from mixer to DownlinkTx.
func (p *ServerPort) streamAudioTo(tx DownlinkTx, setErr func(error)) {
	encoder, err := opus.NewVoIPEncoder(
		p.mixer.Output().SampleRate(),
		p.mixer.Output().Channels(),
	)
	if err != nil {
		setErr(err)
		return
	}
	defer encoder.Close()

	frameDuration := 20 * time.Millisecond
	frameSize := int(p.mixer.Output().SamplesInDuration(frameDuration))
	pcmBytes := make([]byte, frameSize*2) // 2 bytes per sample (int16)

	var stamp time.Time
	for {
		// Read PCM from mixer (as bytes)
		n, err := p.mixer.Read(pcmBytes)
		if err != nil {
			setErr(err)
			return
		}

		// Convert bytes to int16 samples
		pcmSamples := bytesToInt16(pcmBytes[:n])

		// Encode to opus
		opusFrame, err := encoder.Encode(pcmSamples, len(pcmSamples))
		if err != nil {
			setErr(err)
			return
		}

		now := time.Now()
		delay := stamp.Sub(now)

		if delay < 0 {
			// Behind schedule: reset timestamp
			stamp = now
		} else if delay < bufferingDuration {
			// Within buffering duration: fast buffering, minimal sleep
			time.Sleep(5 * time.Millisecond)
		} else {
			// Beyond buffering duration: sleep to maintain bufferingDuration buffer
			sleepDuration := delay - bufferingDuration
			time.Sleep(sleepDuration)
		}

		if len(opusFrame) == 0 {
			stamp = stamp.Add(frameDuration)
			continue
		}

		if err := tx.SendOpusFrame(stamp, opusFrame); err != nil {
			setErr(err)
			return
		}
		stamp = stamp.Add(frameDuration)
	}
}

// =============================================================================
// Track Management (Output)
// =============================================================================

// NewBackgroundTrack creates a new background audio track.
func (p *ServerPort) NewBackgroundTrack() (pcm.Track, *pcm.TrackCtrl, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	w, ctrl, err := p.mixer.CreateTrack()
	if err != nil {
		return nil, nil, err
	}
	ctrl.SetGain(0.1)
	if p.background != nil {
		p.background.CloseWithError(nil)
	}
	p.background = ctrl
	ctrl.SetFadeOutDuration(time.Second)
	return w, ctrl, nil
}

// NewForegroundTrack creates a new foreground audio track.
func (p *ServerPort) NewForegroundTrack() (pcm.Track, *pcm.TrackCtrl, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	w, ctrl, err := p.mixer.CreateTrack()
	if err != nil {
		return nil, nil, err
	}
	if p.foreground != nil {
		p.foreground.CloseWithError(nil)
	}
	p.foreground = ctrl
	ctrl.SetFadeOutDuration(200 * time.Millisecond)
	return w, ctrl, nil
}

// NewOverlayTrack creates a new overlay audio track.
func (p *ServerPort) NewOverlayTrack() (pcm.Track, *pcm.TrackCtrl, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	w, ctrl, err := p.mixer.CreateTrack()
	if err != nil {
		return nil, nil, err
	}
	if p.overlay != nil {
		p.overlay.CloseWithError(nil)
	}
	p.overlay = ctrl
	ctrl.SetFadeOutDuration(time.Second)
	return w, ctrl, nil
}

// BackgroundTrackCtrl returns the current background track controller.
func (p *ServerPort) BackgroundTrackCtrl() *pcm.TrackCtrl {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.background
}

// ForegroundTrackCtrl returns the current foreground track controller.
func (p *ServerPort) ForegroundTrackCtrl() *pcm.TrackCtrl {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.foreground
}

// OverlayTrackCtrl returns the current overlay track controller.
func (p *ServerPort) OverlayTrackCtrl() *pcm.TrackCtrl {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.overlay
}

// Interrupt stops all output tracks immediately.
func (p *ServerPort) Interrupt() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.background != nil {
		p.background.CloseWithError(nil)
		p.background = nil
	}
	if p.foreground != nil {
		p.foreground.CloseWithError(nil)
		p.foreground = nil
	}
	if p.overlay != nil {
		p.overlay.CloseWithError(nil)
		p.overlay = nil
	}
}

// =============================================================================
// State Getters
// =============================================================================

// Stats returns the current stats.
func (p *ServerPort) Stats() (*StatsEvent, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats, p.stats != nil
}

// State returns the current state.
func (p *ServerPort) State() (*StateEvent, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state, p.state != nil
}

// Volume returns the current volume percentage.
func (p *ServerPort) Volume() (int, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.stats != nil && p.stats.Volume != nil {
		return int(p.stats.Volume.Percentage), true
	}
	return 0, false
}

// LightMode returns the current light mode.
func (p *ServerPort) LightMode() (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.stats != nil && p.stats.LightMode != nil {
		return p.stats.LightMode.Mode, true
	}
	return "", false
}

// Brightness returns the current brightness percentage.
func (p *ServerPort) Brightness() (int, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.stats != nil && p.stats.Brightness != nil {
		return int(p.stats.Brightness.Percentage), true
	}
	return 0, false
}

// WifiNetwork returns the current connected WiFi network.
func (p *ServerPort) WifiNetwork() (*ConnectedWifi, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.stats != nil && p.stats.WifiNetwork != nil {
		return p.stats.WifiNetwork, true
	}
	return nil, false
}

// WifiStore returns the stored WiFi list.
func (p *ServerPort) WifiStore() (*StoredWifiList, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.stats != nil && p.stats.WifiStore != nil {
		return p.stats.WifiStore, true
	}
	return nil, false
}

// Battery returns the current battery status.
func (p *ServerPort) Battery() (pct int, isCharging bool, ok bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.stats != nil && p.stats.Battery != nil {
		return int(p.stats.Battery.Percentage), p.stats.Battery.IsCharging, true
	}
	return 0, false, false
}

// SystemVersion returns the current system version.
func (p *ServerPort) SystemVersion() (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.stats != nil && p.stats.SystemVersion != nil {
		return p.stats.SystemVersion.CurrentVersion, true
	}
	return "", false
}

// Cellular returns the current cellular network.
func (p *ServerPort) Cellular() (*ConnectedCellular, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.stats != nil && p.stats.Cellular != nil {
		return p.stats.Cellular, true
	}
	return nil, false
}

// PairStatus returns the current pair status.
func (p *ServerPort) PairStatus() (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.stats != nil && p.stats.PairStatus != nil {
		return p.stats.PairStatus.PairWith, true
	}
	return "", false
}

// ReadNFCTag returns the last read NFC tags.
func (p *ServerPort) ReadNFCTag() (*ReadNFCTag, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.stats != nil && p.stats.ReadNFCTag != nil {
		return p.stats.ReadNFCTag, true
	}
	return nil, false
}

// Shaking returns the current shaking level.
func (p *ServerPort) Shaking() (float64, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.stats != nil && p.stats.Shaking != nil {
		return p.stats.Shaking.Level, true
	}
	return 0, false
}

// =============================================================================
// Device Commands
// =============================================================================

// IssueCommand queues a command to be sent to the device.
func (p *ServerPort) IssueCommand(cmd Command) {
	evt := NewCommandEvent(cmd, time.Now())
	p.commandQueue.Add(evt)
}

// SetVolume sets the volume of the device.
func (p *ServerPort) SetVolume(volume int) {
	cmd := SetVolume(volume)
	p.IssueCommand(&cmd)
}

// SetLightMode sets the light mode of the device.
func (p *ServerPort) SetLightMode(mode string) {
	cmd := SetLightMode(mode)
	p.IssueCommand(&cmd)
}

// SetBrightness sets the brightness of the device.
func (p *ServerPort) SetBrightness(brightness int) {
	cmd := SetBrightness(brightness)
	p.IssueCommand(&cmd)
}

// SetWifi sets the WiFi network of the device.
func (p *ServerPort) SetWifi(ssid, password string) {
	p.IssueCommand(&SetWifi{SSID: ssid, Password: password})
}

// DeleteWifi deletes a stored WiFi network.
func (p *ServerPort) DeleteWifi(ssid string) {
	p.IssueCommand(DeleteWifi(ssid))
}

// Reset resets the device.
func (p *ServerPort) Reset() {
	p.IssueCommand(&Reset{})
}

// Unpair unpairs the device.
func (p *ServerPort) Unpair() {
	p.IssueCommand(&Reset{Unpair: true})
}

// Sleep puts the device to sleep.
func (p *ServerPort) Sleep() {
	p.IssueCommand(&Halt{Sleep: true})
}

// Shutdown shuts down the device.
func (p *ServerPort) Shutdown() {
	p.IssueCommand(&Halt{Shutdown: true})
}

// RaiseCall raises a call on the device.
func (p *ServerPort) RaiseCall() {
	p.IssueCommand(&Raise{Call: true})
}

// UpgradeFirmware initiates an OTA firmware upgrade.
func (p *ServerPort) UpgradeFirmware(ota OTA) {
	p.IssueCommand(&ota)
}

// =============================================================================
// Lifecycle
// =============================================================================

// Close closes the port.
func (p *ServerPort) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	p.uplinkQueue.Close()
	p.commandQueue.Close()
	p.mixer.Close()
	return nil
}
