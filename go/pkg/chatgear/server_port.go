package chatgear

import (
	"context"
	"sync"
	"time"

	"github.com/haivivi/giztoy/pkg/audio/opusrt"
	"github.com/haivivi/giztoy/pkg/audio/pcm"
)

// ServerPort implements ServerPortRx and ServerPortTx as a bidirectional audio port.
// It manages audio input buffering, output mixing, state/stats tracking, and device commands.
type ServerPort struct {
	tx      DownlinkTx
	context context.Context
	cancel  context.CancelFunc
	logger  Logger

	// Input - audio from device
	input *opusrt.RealtimeBuffer

	// Output - mixer for audio to device
	mixer      *pcm.Mixer
	background *pcm.TrackCtrl
	foreground *pcm.TrackCtrl
	overlay    *pcm.TrackCtrl

	// Stats & State
	mu        sync.RWMutex
	gearStats *GearStatsEvent
	gearState *GearStateEvent
	closed    bool // protected by mu, prevents sending to closed channels

	// Events
	statsChanges chan *GearStatsChanges
	stateEvents  chan *GearStateEvent

	// Background goroutine tracking
	wg sync.WaitGroup
}

// NewServerPort creates a new ServerPort for the given DownlinkTx.
func NewServerPort(ctx context.Context, tx DownlinkTx) *ServerPort {
	ctx, cancel := context.WithCancel(ctx)
	p := &ServerPort{
		tx:      tx,
		context: ctx,
		cancel:  cancel,
		logger:  DefaultLogger(),

		input:        opusrt.RealtimeFrom(opusrt.NewBuffer(2 * time.Minute)),
		mixer:        pcm.NewMixer(pcm.L16Mono24K),
		statsChanges: make(chan *GearStatsChanges, 32),
		stateEvents:  make(chan *GearStateEvent, 32),
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.streamingOutputLoop()
	}()
	return p
}

// --- Handle (Input) ---

// Frame reads the next Opus frame from the device.
// Implements opusrt.FrameReader.
func (p *ServerPort) Frame() (opusrt.Frame, time.Duration, error) {
	return p.input.Frame()
}

// HandleOpusFrames handles incoming Opus frames from the device.
func (p *ServerPort) HandleOpusFrames(stampedOpusFrame []byte) {
	if _, err := p.input.Write(stampedOpusFrame); err != nil {
		p.logger.ErrorPrintf("handle opus frames: %v", err)
	}
}

// HandleGearStatsEvent handles incoming stats events from the device.
func (p *ServerPort) HandleGearStatsEvent(gse *GearStatsEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	var gsc *GearStatsChanges
	if p.gearStats != nil {
		gsc = p.gearStats.MergeWith(gse.Clone())
	} else {
		p.gearStats = gse.Clone()
	}

	if gsc == nil {
		return
	}

	select {
	case p.statsChanges <- gsc:
	default:
		p.logger.WarnPrintf("stats changes channel is full, drop stats event")
	}
}

// HandleGearStateEvent handles incoming state events from the device.
func (p *ServerPort) HandleGearStateEvent(gse *GearStateEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	// Filter out-of-order events
	if p.gearState != nil && gse.Time.Before(p.gearState.Time) {
		return
	}

	p.gearState = gse.Clone()

	select {
	case p.stateEvents <- gse.Clone(): // independent clone to avoid sharing with p.gearState
	default:
		p.logger.WarnPrintf("state events channel is full, drop state event")
	}
}

// --- Track (Output) ---

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

// --- Stats (getter) ---

// StatsChanges returns a channel that receives stats change events.
func (p *ServerPort) StatsChanges() <-chan *GearStatsChanges {
	return p.statsChanges
}

// StateEvents returns a channel that receives state events.
func (p *ServerPort) StateEvents() <-chan *GearStateEvent {
	return p.stateEvents
}

// Context returns the port's context.
func (p *ServerPort) Context() context.Context {
	return p.context
}

// GearStats returns the current gear stats.
func (p *ServerPort) GearStats() (*GearStatsEvent, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.gearStats, p.gearStats != nil
}

// GearState returns the current gear state.
func (p *ServerPort) GearState() (*GearStateEvent, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.gearState, p.gearState != nil
}

// Volume returns the current volume percentage.
func (p *ServerPort) Volume() (int, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.gearStats != nil && p.gearStats.Volume != nil {
		return int(p.gearStats.Volume.Percentage), true
	}
	return 0, false
}

// LightMode returns the current light mode.
func (p *ServerPort) LightMode() (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.gearStats != nil && p.gearStats.LightMode != nil {
		return p.gearStats.LightMode.Mode, true
	}
	return "", false
}

// Brightness returns the current brightness percentage.
func (p *ServerPort) Brightness() (int, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.gearStats != nil && p.gearStats.Brightness != nil {
		return int(p.gearStats.Brightness.Percentage), true
	}
	return 0, false
}

// WifiNetwork returns the current connected WiFi network.
func (p *ServerPort) WifiNetwork() (*ConnectedWifi, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.gearStats != nil && p.gearStats.WifiNetwork != nil {
		return p.gearStats.WifiNetwork, true
	}
	return nil, false
}

// WifiStore returns the stored WiFi list.
func (p *ServerPort) WifiStore() (*StoredWifiList, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.gearStats != nil && p.gearStats.WifiStore != nil {
		return p.gearStats.WifiStore, true
	}
	return nil, false
}

// Battery returns the current battery status.
func (p *ServerPort) Battery() (pct int, isCharging bool, ok bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.gearStats != nil && p.gearStats.Battery != nil {
		return int(p.gearStats.Battery.Percentage), p.gearStats.Battery.IsCharging, true
	}
	return 0, false, false
}

// SystemVersion returns the current system version.
func (p *ServerPort) SystemVersion() (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.gearStats != nil && p.gearStats.SystemVersion != nil {
		return p.gearStats.SystemVersion.CurrentVersion, true
	}
	return "", false
}

// Cellular returns the current cellular network.
func (p *ServerPort) Cellular() (*ConnectedCellular, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.gearStats != nil && p.gearStats.Cellular != nil {
		return p.gearStats.Cellular, true
	}
	return nil, false
}

// PairStatus returns the current pair status.
func (p *ServerPort) PairStatus() (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.gearStats != nil && p.gearStats.PairStatus != nil {
		return p.gearStats.PairStatus.PairWith, true
	}
	return "", false
}

// ReadNFCTag returns the last read NFC tags.
func (p *ServerPort) ReadNFCTag() (*ReadNFCTag, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.gearStats != nil && p.gearStats.ReadNFCTag != nil {
		return p.gearStats.ReadNFCTag, true
	}
	return nil, false
}

// Shaking returns the current shaking level.
func (p *ServerPort) Shaking() (float64, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.gearStats != nil && p.gearStats.Shaking != nil {
		return p.gearStats.Shaking.Level, true
	}
	return 0, false
}

// --- Device Commands ---

// SetVolume sets the volume of the device.
func (p *ServerPort) SetVolume(volume int) error {
	cmd := SetVolume(volume)
	return p.tx.IssueCommand(p.context, &cmd, time.Now())
}

// SetLightMode sets the light mode of the device.
func (p *ServerPort) SetLightMode(mode string) error {
	cmd := SetLightMode(mode)
	return p.tx.IssueCommand(p.context, &cmd, time.Now())
}

// SetBrightness sets the brightness of the device.
func (p *ServerPort) SetBrightness(brightness int) error {
	cmd := SetBrightness(brightness)
	return p.tx.IssueCommand(p.context, &cmd, time.Now())
}

// SetWifi sets the WiFi network of the device.
func (p *ServerPort) SetWifi(ssid, password string) error {
	return p.tx.IssueCommand(p.context, &SetWifi{SSID: ssid, Password: password}, time.Now())
}

// DeleteWifi deletes a stored WiFi network.
func (p *ServerPort) DeleteWifi(ssid string) error {
	return p.tx.IssueCommand(p.context, DeleteWifi(ssid), time.Now())
}

// Reset resets the device.
func (p *ServerPort) Reset() error {
	return p.tx.IssueCommand(p.context, &Reset{}, time.Now())
}

// Unpair unpairs the device.
func (p *ServerPort) Unpair() error {
	return p.tx.IssueCommand(p.context, &Reset{Unpair: true}, time.Now())
}

// Sleep puts the device to sleep.
func (p *ServerPort) Sleep() error {
	return p.tx.IssueCommand(p.context, &Halt{Sleep: true}, time.Now())
}

// Shutdown shuts down the device.
func (p *ServerPort) Shutdown() error {
	return p.tx.IssueCommand(p.context, &Halt{Shutdown: true}, time.Now())
}

// RaiseCall raises a call on the device.
func (p *ServerPort) RaiseCall() error {
	return p.tx.IssueCommand(p.context, &Raise{Call: true}, time.Now())
}

// UpgradeFirmware initiates an OTA firmware upgrade.
func (p *ServerPort) UpgradeFirmware(ota OTA) error {
	return p.tx.IssueCommand(p.context, &ota, time.Now())
}

// --- Lifecycle ---

// Close closes the port.
func (p *ServerPort) Close() error {
	p.cancel()
	p.input.Close()
	p.mixer.Close()

	// Wait for background goroutines to finish
	p.wg.Wait()

	// Safely close channels under lock to prevent send-to-closed-channel panic
	p.mu.Lock()
	p.closed = true
	close(p.statsChanges)
	close(p.stateEvents)
	p.mu.Unlock()

	return p.tx.Close()
}

// RecvFrom receives data from the given UplinkRx until closed or error.
// This method blocks; use `go port.RecvFrom(rx)` for non-blocking operation.
// Returns the first error encountered, or nil if all iterators completed normally.
func (p *ServerPort) RecvFrom(rx UplinkRx) error {
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
	go func() {
		defer wg.Done()
		for frame, err := range rx.OpusFrames() {
			if err != nil {
				setErr(err)
				return
			}
			p.HandleOpusFrames(frame)
		}
	}()
	go func() {
		defer wg.Done()
		for state, err := range rx.States() {
			if err != nil {
				setErr(err)
				return
			}
			p.HandleGearStateEvent(state)
		}
	}()
	go func() {
		defer wg.Done()
		for stats, err := range rx.Stats() {
			if err != nil {
				setErr(err)
				return
			}
			p.HandleGearStatsEvent(stats)
		}
	}()
	wg.Wait()
	return firstErr
}

const bufferingDuration = 360 * time.Millisecond

func (p *ServerPort) streamingOutputLoop() {
	str, err := opusrt.EncodePCMStream(
		p.mixer,
		p.mixer.Output().SampleRate(),
		p.mixer.Output().Channels(),
		p.tx.OpusEncodeOptions()...,
	)
	if err != nil {
		// Don't log error if context is already cancelled (expected shutdown)
		if p.context.Err() == nil {
			p.logger.ErrorPrintf("encode pcm stream: %v", err)
		}
		return
	}

	var stamp time.Time
	for {
		select {
		case <-p.context.Done():
			return
		default:
		}

		frame, duration, err := str.Frame()
		if err != nil {
			// Don't log error if context is cancelled (expected shutdown)
			if p.context.Err() == nil {
				p.logger.ErrorPrintf("read frame from mixer: %v", err)
			}
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

		if len(frame) == 0 {
			stamp = stamp.Add(duration)
			continue
		}

		if err := p.tx.SendOpusFrames(p.context, opusrt.FromTime(stamp), frame.Clone()); err != nil {
			// Don't log error if context is cancelled (expected shutdown)
			if p.context.Err() == nil {
				p.logger.ErrorPrintf("send opus frame: %v", err)
			}
			continue
		}
		stamp = stamp.Add(frame.Duration())
	}
}
