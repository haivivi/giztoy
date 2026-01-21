package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/haivivi/giztoy/pkg/audio/opusrt"
	"github.com/haivivi/giztoy/pkg/chatgear"
	"github.com/haivivi/giztoy/pkg/jsontime"
)

// Note: Audio is handled via WebRTC, format negotiated with browser

// SimulatorConfig holds configuration for the simulator.
type SimulatorConfig struct {
	MQTTURL   string
	GearID    string
	Namespace string
}

// SimulatorEvent represents an event from the simulator.
type SimulatorEvent struct {
	Type      string // "state_sent", "stats_sent", "command_received", "error"
	Timestamp time.Time
	Data      string // JSON string
}

// PowerState represents the device power state.
type PowerState int

const (
	PowerOff PowerState = iota
	PowerOn
	PowerDeepSleep
)

func (p PowerState) String() string {
	switch p {
	case PowerOff:
		return "OFF"
	case PowerOn:
		return "ON"
	case PowerDeepSleep:
		return "SLEEP"
	default:
		return "UNKNOWN"
	}
}

// Simulator simulates a gear device using chatgear.ClientPort.
type Simulator struct {
	cfg SimulatorConfig

	// chatgear port
	port *chatgear.ClientPort

	// WebRTC for browser audio
	webrtc *WebRTCBridge
	// RTP sequence number and timestamp are updated from both the WebRTC
	// callback goroutine and the playback loop. To avoid taking the
	// Simulator mutex on every audio packet, these counters are accessed
	// exclusively via sync/atomic operations and are intentionally *not*
	// protected by mu. All other mutable fields in Simulator must be
	// accessed under mu.
	rtpSeqNum    uint32
	rtpTimestamp uint32

	mu         sync.RWMutex
	state      chatgear.GearState
	powerState PowerState
	stats      *chatgear.GearStatsEvent

	// Staged stats for incremental sending (like C's stats_pending_send)
	stagedStats  *chatgear.GearStatsEvent
	statsTrigger chan struct{}

	// Simulated device state
	volume     int     // 0-100
	brightness int     // 0-100
	lightMode  string  // "auto", "on", "off"
	batteryPct float64 // 0-100
	charging   bool
	wifiSSID   string
	wifiRSSI   float64
	wifiIP     string
	wifiGW     string
	sysVersion string

	// Additional stats
	pairWith     string   // paired device ID
	shakingLevel float64  // 0-100
	wifiStore    []string // stored wifi SSIDs

	// Channels for TUI
	events chan SimulatorEvent

	ctx    context.Context
	cancel context.CancelFunc
}

// NewSimulator creates a new simulator with default device state.
func NewSimulator(cfg SimulatorConfig) *Simulator {
	s := &Simulator{
		cfg:          cfg,
		state:        chatgear.GearReady,
		powerState:   PowerOff, // Start powered off
		stats:        &chatgear.GearStatsEvent{Time: jsontime.NowEpochMilli()},
		stagedStats:  &chatgear.GearStatsEvent{},
		statsTrigger: make(chan struct{}, 1),
		events:       make(chan SimulatorEvent, 100),
		webrtc:       NewWebRTCBridge(),

		// Default device state
		volume:       70,
		brightness:   80,
		lightMode:    "auto",
		batteryPct:   85,
		charging:     false,
		wifiSSID:     "HomeWiFi",
		wifiRSSI:     -45,
		wifiIP:       "192.168.1.100",
		wifiGW:       "192.168.1.1",
		sysVersion:   "0_zh",
		pairWith:     "",
		shakingLevel: 0,
		wifiStore:    []string{"HomeWiFi", "Office", "Guest"},
	}

	// Set up WebRTC audio callback - send browser audio to server
	s.webrtc.SetOnAudioReceived(func(opusData []byte) {
		s.handleBrowserAudio(opusData)
	})

	return s
}

// Events returns the event channel.
func (s *Simulator) Events() <-chan SimulatorEvent {
	return s.events
}

// State returns the current state.
func (s *Simulator) State() chatgear.GearState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Stats returns the current stats.
func (s *Simulator) Stats() *chatgear.GearStatsEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// WebRTC returns the WebRTC bridge for audio I/O.
func (s *Simulator) WebRTC() *WebRTCBridge {
	return s.webrtc
}

// Start starts the simulator.
func (s *Simulator) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Recreate internal channels for new session
	// Note: s.events is NOT recreated - it persists for TUI to listen
	s.statsTrigger = make(chan struct{}, 1)
	s.stagedStats = &chatgear.GearStatsEvent{}

	slog.Info("connecting to MQTT broker", "url", s.cfg.MQTTURL)

	// Connect to MQTT using mqtt0
	clientConn, err := mqttDial(s.ctx, s.cfg.MQTTURL, s.cfg.Namespace, s.cfg.GearID)
	if err != nil {
		slog.Error("MQTT connection failed", "error", err)
		return err
	}
	slog.Info("MQTT connected successfully")

	// Create ClientPort
	s.port = chatgear.NewClientPort(s.ctx, clientConn)
	slog.Info("ClientPort created, ready to send/receive")

	// Start receiving from server (commands and audio)
	go func() {
		slog.Info("starting receive loop...")
		if err := s.port.RecvFrom(clientConn); err != nil {
			slog.Error("RecvFrom error", "error", err)
			s.emitEvent("error", err.Error())
		}
	}()

	// Handle commands from server
	go s.handleCommands()

	// Start audio playback loop (server -> WebRTC -> browser)
	go s.playbackLoop()
	slog.Info("audio playback loop started (WebRTC to browser)")

	// Note: recording is handled by WebRTC callback (browser -> WebRTC -> server)
	// No need for a dedicated recording loop - audio comes from browser via WebRTC

	// Send periodic stats (like C implementation)
	go s.statsQueryLoop()
	go s.statsSendLoop()

	// Send state periodically (for CALLING/RECORDING timestamp updates)
	go s.stateSendLoop()

	// Set state to READY and send initial state
	s.mu.Lock()
	s.state = chatgear.GearReady
	s.mu.Unlock()
	s.sendState()

	slog.Info("simulator started", "gearID", s.cfg.GearID)
	return nil
}

// Stop stops the simulator.
func (s *Simulator) Stop() {
	slog.Info("stopping simulator...")
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.port != nil {
		s.port.Close()
		s.port = nil
	}
	// Note: WebRTC bridge is not closed here - it persists across power cycles
	// This allows browser to stay connected while device is powered off

	// Note: the events channel is intentionally NOT closed here.
	// It is designed to be long-lived (across simulator power cycles), and
	// consumers such as the TUI must not rely on a closed events channel to
	// detect shutdown. Instead, they should also select on s.ctx.Done() (or
	// their own cancellation) and terminate when the simulator context ends.
	//
	// Only close statsTrigger (internal channel), which is used purely inside
	// the simulator implementation. Use mutex to prevent race with triggerStatsSend.
	s.mu.Lock()
	if s.statsTrigger != nil {
		close(s.statsTrigger)
		s.statsTrigger = nil
	}
	s.mu.Unlock()
	slog.Info("simulator stopped")
}

// handleCommands handles commands from the server.
func (s *Simulator) handleCommands() {
	slog.Info("command handler started")
	for {
		select {
		case <-s.ctx.Done():
			slog.Info("command handler stopped")
			return
		case cmd, ok := <-s.port.Commands():
			if !ok {
				slog.Info("command channel closed")
				return
			}
			slog.Info("received command", "type", fmt.Sprintf("%T", cmd.Payload))
			data, _ := json.Marshal(cmd)
			s.emitEvent("command_received", string(data))
			s.applyCommand(cmd)
		}
	}
}

// applyCommand applies a command to the simulator state.
func (s *Simulator) applyCommand(cmd *chatgear.SessionCommandEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch t := cmd.Payload.(type) {
	case *chatgear.Streaming:
		slog.Info("cmd: streaming", "value", bool(*t))
		if bool(*t) {
			if s.state == chatgear.GearWaitingForResponse {
				s.state = chatgear.GearStreaming
				s.sendStateLocked()
			}
		} else {
			if s.state == chatgear.GearStreaming {
				s.state = chatgear.GearReady
				s.sendStateLocked()
			}
		}

	case *chatgear.SetVolume:
		s.volume = min(max(int(*t), 0), 100)
		slog.Info("cmd: set_volume", "value", s.volume)

	case *chatgear.SetBrightness:
		s.brightness = min(max(int(*t), 0), 100)
		slog.Info("cmd: set_brightness", "value", s.brightness)

	case *chatgear.SetLightMode:
		s.lightMode = string(*t)
		slog.Info("cmd: set_light_mode", "value", s.lightMode)

	case *chatgear.SetWifi:
		slog.Info("cmd: set_wifi", "ssid", t.SSID)
		s.wifiSSID = t.SSID
		s.wifiRSSI = -50
		s.wifiIP = "192.168.1.100"
		s.wifiGW = "192.168.1.1"
		// Add to wifi store if not already present
		found := false
		for _, ssid := range s.wifiStore {
			if ssid == t.SSID {
				found = true
				break
			}
		}
		if !found {
			s.wifiStore = append(s.wifiStore, t.SSID)
		}

	case *chatgear.DeleteWifi:
		ssid := string(*t)
		slog.Info("cmd: delete_wifi", "ssid", ssid)
		// Disconnect if currently connected
		if ssid == s.wifiSSID {
			s.wifiSSID = ""
			s.wifiRSSI = 0
			s.wifiIP = ""
			s.wifiGW = ""
		}
		// Remove from wifi store
		for i, stored := range s.wifiStore {
			if stored == ssid {
				s.wifiStore = append(s.wifiStore[:i], s.wifiStore[i+1:]...)
				break
			}
		}

	case *chatgear.Reset:
		slog.Info("cmd: reset", "unpair", t.Unpair)
		// Reset to factory defaults (consistent with Reset() method)
		s.volume = 100
		s.brightness = 100
		s.lightMode = "auto"
		if t.Unpair {
			s.wifiSSID = ""
			s.wifiRSSI = 0
			s.wifiIP = ""
			s.wifiGW = ""
			s.pairWith = ""
			s.wifiStore = []string{}
		}

	case *chatgear.Halt:
		slog.Info("cmd: halt", "sleep", t.Sleep, "shutdown", t.Shutdown, "interrupt", t.Interrupt)
		if t.Interrupt {
			// Interrupt current operation
			if s.state == chatgear.GearStreaming || s.state == chatgear.GearRecording ||
				s.state == chatgear.GearWaitingForResponse {
				s.state = chatgear.GearReady
				s.sendStateLocked()
			}
		} else if t.Sleep {
			// Go to sleep (simulate as ready)
			s.state = chatgear.GearReady
			s.sendStateLocked()
		} else if t.Shutdown {
			// Shutdown (simulate as ready, in real device would power off)
			s.state = chatgear.GearReady
			s.sendStateLocked()
		}

	case *chatgear.Raise:
		slog.Info("cmd: raise", "call", t.Call)
		if t.Call {
			if s.state == chatgear.GearReady {
				s.state = chatgear.GearCalling
				s.sendStateLocked()
			}
		}

	case *chatgear.OTA:
		slog.Info("cmd: ota_upgrade", "version", t.Version)
		// Simulate OTA upgrade with context cancellation support
		go func(ctx context.Context) {
			s.mu.Lock()
			oldVersion := s.sysVersion
			s.mu.Unlock()

			// Simulate download/install progress
			for i := 0; i <= 100; i += 10 {
				select {
				case <-ctx.Done():
					slog.Info("OTA cancelled")
					return
				case <-time.After(200 * time.Millisecond):
				}
				slog.Info("OTA progress", "percent", i)
			}

			s.mu.Lock()
			if t.Version != "" {
				s.sysVersion = t.Version
			}
			s.mu.Unlock()

			slog.Info("OTA complete", "from", oldVersion, "to", t.Version)
		}(s.ctx)

	default:
		slog.Warn("cmd: unknown type", "type", fmt.Sprintf("%T", cmd.Payload))
	}
}

// playbackLoop reads opus from server and sends to browser via WebRTC.
func (s *Simulator) playbackLoop() {
	slog.Info("playback loop started (server -> WebRTC -> browser)")

	// Read opus frames from server and send to browser
	for {
		select {
		case <-s.ctx.Done():
			slog.Info("playback loop stopped")
			return
		default:
		}

		// Read opus frame from server via ClientPort's Frame() method
		frame, _, err := s.port.Frame()
		if err != nil {
			slog.Info("playback read ended", "error", err)
			return
		}

		if len(frame) == 0 {
			continue
		}

		// Send to browser via WebRTC
		if s.webrtc.IsConnected() {
			seqNum := uint16(atomic.AddUint32(&s.rtpSeqNum, 1))
			timestamp := atomic.AddUint32(&s.rtpTimestamp, 960) // 20ms at 48kHz
			if err := s.webrtc.SendAudio([]byte(frame), timestamp, seqNum); err != nil {
				slog.Error("webrtc send error", "error", err)
			}
		}
	}
}

// handleBrowserAudio handles audio received from browser via WebRTC.
// This is called from the WebRTC bridge when audio is received.
func (s *Simulator) handleBrowserAudio(opusData []byte) {
	s.mu.RLock()
	st := s.state
	port := s.port
	s.mu.RUnlock()

	// Only send audio when in recording or calling state
	if st != chatgear.GearRecording && st != chatgear.GearCalling {
		return
	}

	if port == nil {
		return
	}

	// Stamp and send to server
	frame := opusrt.Frame(opusData)
	stamped := opusrt.Stamp(frame, opusrt.FromTime(time.Now()))
	port.WriteRecordingFrame(stamped)
}

// stateSendLoop sends state periodically (every 5 seconds).
// Updates timestamp for CALLING and RECORDING states.
func (s *Simulator) stateSendLoop() {
	slog.Info("stateSendLoop started")
	defer slog.Info("stateSendLoop stopped")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastState chatgear.GearState
	var lastSentAt time.Time

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.sendStateOnce(&lastState, &lastSentAt)
		}
	}
}

// sendStateOnce sends state if needed, updating lastState and lastSentAt.
func (s *Simulator) sendStateOnce(lastState *chatgear.GearState, lastSentAt *time.Time) {
	s.mu.Lock()
	now := time.Now()
	stateChanged := s.state != *lastState

	// For CALLING and RECORDING, always update timestamp
	if s.state == chatgear.GearCalling || s.state == chatgear.GearRecording {
		stateChanged = true
	}

	if stateChanged || time.Since(*lastSentAt) > 30*time.Second {
		stateEvent := chatgear.NewGearStateEvent(s.state, now)
		port := s.port
		state := s.state
		s.mu.Unlock()

		if port != nil {
			if err := port.SendState(stateEvent); err == nil {
				*lastState = state
				*lastSentAt = now
				data, _ := json.Marshal(stateEvent)
				s.emitEvent("state_sent", string(data))
			} else {
				slog.Error("send state failed", "error", err)
			}
		}
	} else {
		s.mu.Unlock()
	}
}

// statsQueryLoop implements the C-style tiered stats querying.
// Periodically stages stats for sending based on different intervals.
// 1 min: battery, volume, brightness, lang, light_mode, sys_ver, pair_status, wlan
// 2 min: nfc_read, shaking
// 10 min: wifi_scan, wifi_store
func (s *Simulator) statsQueryLoop() {
	slog.Info("statsQueryLoop started")
	defer slog.Info("statsQueryLoop stopped")

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	// Send initial stats immediately
	s.stageAllStats()
	s.triggerStatsSend()

	rounds := -1
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			rounds++
			now := jsontime.NowEpochMilli()

			s.mu.Lock()
			switch rounds % 3 {
			case 0:
				// Every 1 minute (20s * 3 = 60s)
				// Stage: battery, volume, brightness, light_mode, sys_ver, pair_status, wlan
				s.stagedStats.Battery = &chatgear.Battery{
					Percentage: s.batteryPct,
					IsCharging: s.charging,
				}
				s.stagedStats.Volume = &chatgear.Volume{
					Percentage: float64(s.volume),
					UpdateAt:   now,
				}
				s.stagedStats.Brightness = &chatgear.Brightness{
					Percentage: float64(s.brightness),
					UpdateAt:   now,
				}
				s.stagedStats.LightMode = &chatgear.LightMode{
					Mode:     s.lightMode,
					UpdateAt: now,
				}
				s.stagedStats.SystemVersion = &chatgear.SystemVersion{
					CurrentVersion: s.sysVersion,
				}
				s.stagedStats.PairStatus = &chatgear.PairStatus{
					PairWith: s.pairWith,
					UpdateAt: now,
				}
				if s.wifiSSID != "" {
					s.stagedStats.WifiNetwork = &chatgear.ConnectedWifi{
						SSID:    s.wifiSSID,
						RSSI:    s.wifiRSSI,
						IP:      s.wifiIP,
						Gateway: s.wifiGW,
					}
				}
			case 1:
				if rounds%6 != 1 {
					s.mu.Unlock()
					continue
				}
				// Every 2 minutes
				// Stage: shaking
				s.stagedStats.Shaking = &chatgear.Shaking{
					Level: s.shakingLevel,
				}
			case 2:
				if rounds%30 != 2 {
					s.mu.Unlock()
					continue
				}
				// Every 10 minutes
				// Stage: wifi_store
				if len(s.wifiStore) > 0 {
					items := make([]chatgear.WifiStoreItem, len(s.wifiStore))
					for i, ssid := range s.wifiStore {
						items[i] = chatgear.WifiStoreItem{SSID: ssid}
					}
					s.stagedStats.WifiStore = &chatgear.StoredWifiList{
						List:     items,
						UpdateAt: now,
					}
				}
			}
			s.mu.Unlock()

			// Trigger send
			s.triggerStatsSend()
		}
	}
}

// statsSendLoop listens for stats send triggers and sends staged stats.
func (s *Simulator) statsSendLoop() {
	slog.Info("statsSendLoop started")
	defer slog.Info("statsSendLoop stopped")

	for {
		select {
		case <-s.ctx.Done():
			return
		case _, ok := <-s.statsTrigger:
			if !ok {
				// Channel closed, exit loop
				return
			}
			s.sendStagedStats()
		}
	}
}

// triggerStatsSend triggers a stats send (non-blocking).
func (s *Simulator) triggerStatsSend() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.statsTrigger == nil {
		return
	}
	select {
	case s.statsTrigger <- struct{}{}:
	default:
		// Already triggered, skip
	}
}

// stageAllStats stages all stats for sending (used for initial send).
func (s *Simulator) stageAllStats() {
	now := jsontime.NowEpochMilli()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.stagedStats.Battery = &chatgear.Battery{
		Percentage: s.batteryPct,
		IsCharging: s.charging,
	}
	s.stagedStats.Volume = &chatgear.Volume{
		Percentage: float64(s.volume),
		UpdateAt:   now,
	}
	s.stagedStats.Brightness = &chatgear.Brightness{
		Percentage: float64(s.brightness),
		UpdateAt:   now,
	}
	s.stagedStats.LightMode = &chatgear.LightMode{
		Mode:     s.lightMode,
		UpdateAt: now,
	}
	s.stagedStats.SystemVersion = &chatgear.SystemVersion{
		CurrentVersion: s.sysVersion,
	}
	s.stagedStats.PairStatus = &chatgear.PairStatus{
		PairWith: s.pairWith,
		UpdateAt: now,
	}
	if s.wifiSSID != "" {
		s.stagedStats.WifiNetwork = &chatgear.ConnectedWifi{
			SSID:    s.wifiSSID,
			RSSI:    s.wifiRSSI,
			IP:      s.wifiIP,
			Gateway: s.wifiGW,
		}
	}
	s.stagedStats.Shaking = &chatgear.Shaking{
		Level: s.shakingLevel,
	}
	if len(s.wifiStore) > 0 {
		items := make([]chatgear.WifiStoreItem, len(s.wifiStore))
		for i, ssid := range s.wifiStore {
			items[i] = chatgear.WifiStoreItem{SSID: ssid}
		}
		s.stagedStats.WifiStore = &chatgear.StoredWifiList{
			List:     items,
			UpdateAt: now,
		}
	}
}

// sendStagedStats sends the staged stats and clears them.
func (s *Simulator) sendStagedStats() {
	if s.port == nil {
		return
	}

	s.mu.Lock()
	// Check if there's anything to send
	if s.isStagedStatsEmpty() {
		s.mu.Unlock()
		return
	}

	// Set timestamp
	s.stagedStats.Time = jsontime.NowEpochMilli()

	// Clone and clear staged stats
	stats := s.stagedStats.Clone()
	s.stagedStats = &chatgear.GearStatsEvent{}
	s.mu.Unlock()

	if err := s.port.SendStats(stats); err == nil {
		data, _ := json.Marshal(stats)
		slog.Info("stats sent", "data", string(data))
		s.emitEvent("stats_sent", string(data))
	} else {
		slog.Error("SendStats error", "error", err)
	}
}

// isStagedStatsEmpty checks if staged stats has any fields set.
// Must be called with s.mu held.
func (s *Simulator) isStagedStatsEmpty() bool {
	st := s.stagedStats
	return st.Battery == nil &&
		st.Volume == nil &&
		st.Brightness == nil &&
		st.LightMode == nil &&
		st.SystemVersion == nil &&
		st.WifiNetwork == nil &&
		st.WifiStore == nil &&
		st.PairStatus == nil &&
		st.Shaking == nil &&
		st.ReadNFCTag == nil &&
		st.Cellular == nil
}

// StartRecording starts recording (button pressed).
// Valid from: READY, WAITING_FOR_RESPONSE, STREAMING (interrupt and re-record)
func (s *Simulator) StartRecording() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.state {
	case chatgear.GearReady, chatgear.GearWaitingForResponse, chatgear.GearStreaming:
		s.state = chatgear.GearRecording
		s.sendStateLocked()
		return true
	default:
		return false
	}
}

// EndRecording ends recording and waits for response (button released).
// Valid from: RECORDING
func (s *Simulator) EndRecording() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == chatgear.GearRecording {
		s.state = chatgear.GearWaitingForResponse
		s.sendStateLocked()
		return true
	}
	return false
}

// StartCalling starts calling mode.
// Valid from: READY
func (s *Simulator) StartCalling() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == chatgear.GearReady {
		s.state = chatgear.GearCalling
		s.sendStateLocked()
		return true
	}
	return false
}

// EndCalling ends calling mode.
// Valid from: CALLING
func (s *Simulator) EndCalling() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == chatgear.GearCalling {
		s.state = chatgear.GearReady
		s.sendStateLocked()
		return true
	}
	return false
}

// Cancel cancels/interrupts current operation back to READY.
// Valid from: any state except READY
func (s *Simulator) Cancel() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != chatgear.GearReady {
		s.state = chatgear.GearReady
		s.sendStateLocked()
		return true
	}
	return false
}

// sendState sends the current state (acquires lock).
func (s *Simulator) sendState() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendStateLocked()
}

// sendStateLocked sends the current state (must hold lock).
// Uses goroutine to avoid blocking while holding the lock.
func (s *Simulator) sendStateLocked() {
	if s.port == nil {
		return
	}
	stateEvent := chatgear.NewGearStateEvent(s.state, time.Now())
	port := s.port // capture reference for goroutine
	ctx := s.ctx   // capture context for cancellation check
	go func() {
		// Check context before sending to avoid work after shutdown
		if ctx != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
		if err := port.SendState(stateEvent); err == nil {
			data, _ := json.Marshal(stateEvent)
			s.emitEvent("state_sent", string(data))
		} else {
			slog.Error("send state failed", "error", err)
		}
	}()
}

func (s *Simulator) emitEvent(eventType, data string) {
	if s.events == nil {
		return
	}
	select {
	case s.events <- SimulatorEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}:
	default:
		// Drop event if channel is full
	}
}

// --- Getters for TUI ---

// Volume returns current volume (0-100).
func (s *Simulator) Volume() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.volume
}

// Brightness returns current brightness (0-100).
func (s *Simulator) Brightness() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.brightness
}

// LightMode returns current light mode.
func (s *Simulator) LightMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lightMode
}

// Battery returns current battery percentage and charging status.
func (s *Simulator) Battery() (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.batteryPct, s.charging
}

// Wifi returns current wifi SSID and RSSI.
func (s *Simulator) Wifi() (string, float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.wifiSSID, s.wifiRSSI
}

// SystemVersion returns the system version.
func (s *Simulator) SystemVersion() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sysVersion
}

// --- Setters for TUI commands ---
// All setters update staged stats and trigger send.

// SetVolume sets volume (0-100).
func (s *Simulator) SetVolume(v int) {
	s.mu.Lock()
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	s.volume = v
	s.stagedStats.Volume = &chatgear.Volume{
		Percentage: float64(v),
		UpdateAt:   jsontime.NowEpochMilli(),
	}
	s.mu.Unlock()
	s.triggerStatsSend()
}

// SetBrightness sets brightness (0-100).
func (s *Simulator) SetBrightness(b int) {
	s.mu.Lock()
	if b < 0 {
		b = 0
	}
	if b > 100 {
		b = 100
	}
	s.brightness = b
	s.stagedStats.Brightness = &chatgear.Brightness{
		Percentage: float64(b),
		UpdateAt:   jsontime.NowEpochMilli(),
	}
	s.mu.Unlock()
	s.triggerStatsSend()
}

// SetLightMode sets light mode ("auto", "on", "off").
func (s *Simulator) SetLightMode(mode string) {
	s.mu.Lock()
	s.lightMode = mode
	s.stagedStats.LightMode = &chatgear.LightMode{
		Mode:     mode,
		UpdateAt: jsontime.NowEpochMilli(),
	}
	s.mu.Unlock()
	s.triggerStatsSend()
}

// SetBattery sets battery status.
func (s *Simulator) SetBattery(pct float64, charging bool) {
	s.mu.Lock()
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	s.batteryPct = pct
	s.charging = charging
	s.stagedStats.Battery = &chatgear.Battery{
		Percentage: pct,
		IsCharging: charging,
	}
	s.mu.Unlock()
	s.triggerStatsSend()
}

// SetWifi sets wifi status.
func (s *Simulator) SetWifi(ssid string, rssi float64) {
	s.mu.Lock()
	s.wifiSSID = ssid
	s.wifiRSSI = rssi
	if ssid != "" {
		s.wifiIP = "192.168.1.100"
		s.wifiGW = "192.168.1.1"
		s.stagedStats.WifiNetwork = &chatgear.ConnectedWifi{
			SSID:    ssid,
			RSSI:    rssi,
			IP:      s.wifiIP,
			Gateway: s.wifiGW,
		}
	} else {
		s.wifiIP = ""
		s.wifiGW = ""
		// Send empty wifi to indicate disconnected
		s.stagedStats.WifiNetwork = &chatgear.ConnectedWifi{}
	}
	s.mu.Unlock()
	s.triggerStatsSend()
}

// WifiIP returns the WiFi IP address.
func (s *Simulator) WifiIP() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.wifiIP
}

// PairStatus returns the paired device ID.
func (s *Simulator) PairStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pairWith
}

// SetPairStatus sets the paired device ID.
func (s *Simulator) SetPairStatus(pairWith string) {
	s.mu.Lock()
	s.pairWith = pairWith
	s.stagedStats.PairStatus = &chatgear.PairStatus{
		PairWith: pairWith,
		UpdateAt: jsontime.NowEpochMilli(),
	}
	s.mu.Unlock()
	s.triggerStatsSend()
}

// Shaking returns the current shaking level.
func (s *Simulator) Shaking() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shakingLevel
}

// SetShaking sets the shaking level.
func (s *Simulator) SetShaking(level float64) {
	s.mu.Lock()
	s.shakingLevel = level
	s.stagedStats.Shaking = &chatgear.Shaking{
		Level: level,
	}
	s.mu.Unlock()
	s.triggerStatsSend()
}

// WifiStore returns the stored WiFi SSIDs.
func (s *Simulator) WifiStore() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.wifiStore))
	copy(result, s.wifiStore)
	return result
}

// AddWifiStore adds a WiFi SSID to stored list.
func (s *Simulator) AddWifiStore(ssid string) {
	s.mu.Lock()
	// Check if already exists
	for _, stored := range s.wifiStore {
		if stored == ssid {
			s.mu.Unlock()
			return
		}
	}
	s.wifiStore = append(s.wifiStore, ssid)
	s.stageWifiStore()
	s.mu.Unlock()
	s.triggerStatsSend()
}

// RemoveWifiStore removes a WiFi SSID from stored list.
func (s *Simulator) RemoveWifiStore(ssid string) {
	s.mu.Lock()
	for i, stored := range s.wifiStore {
		if stored == ssid {
			s.wifiStore = append(s.wifiStore[:i], s.wifiStore[i+1:]...)
			s.stageWifiStore()
			s.mu.Unlock()
			s.triggerStatsSend()
			return
		}
	}
	s.mu.Unlock()
}

// stageWifiStore stages the current wifi store list.
// Must be called with s.mu held.
func (s *Simulator) stageWifiStore() {
	items := make([]chatgear.WifiStoreItem, len(s.wifiStore))
	for i, ssid := range s.wifiStore {
		items[i] = chatgear.WifiStoreItem{SSID: ssid}
	}
	s.stagedStats.WifiStore = &chatgear.StoredWifiList{
		List:     items,
		UpdateAt: jsontime.NowEpochMilli(),
	}
}

// SetVersion sets the system version.
func (s *Simulator) SetVersion(version string) {
	s.mu.Lock()
	s.sysVersion = version
	s.stagedStats.SystemVersion = &chatgear.SystemVersion{
		CurrentVersion: version,
	}
	s.mu.Unlock()
	s.triggerStatsSend()
}

// --- TUI Helper: Get all status as a summary string ---

// StatusSummary returns a formatted status summary for TUI display.
func (s *Simulator) StatusSummary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chargingIcon := ""
	if s.charging {
		chargingIcon = "⚡"
	}

	wifiInfo := "disconnected"
	if s.wifiSSID != "" {
		wifiInfo = s.wifiSSID
	}

	return fmt.Sprintf(
		"State: %-12s | 🔋 %.0f%%%s | 🔊 %d%% | 💡 %d%% (%s) | 📶 %s",
		s.state.String(),
		s.batteryPct,
		chargingIcon,
		s.volume,
		s.brightness,
		s.lightMode,
		wifiInfo,
	)
}

// SendStats sends current stats to the server immediately.
// SendStats triggers sending staged stats (called by web UI).
// Note: The actual stats are staged by setter methods, this just triggers the send.
func (s *Simulator) SendStats() {
	s.triggerStatsSend()
}

// --- Power Control ---

// PowerState returns the current power state.
func (s *Simulator) PowerState() PowerState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.powerState
}

// PowerOn turns on the device, loads saved state, and starts MQTT connection.
func (s *Simulator) PowerOn() error {
	s.mu.Lock()
	if s.powerState == PowerOn {
		s.mu.Unlock()
		return nil
	}
	s.powerState = PowerOn
	s.mu.Unlock()

	// Load saved state first
	s.LoadState()

	// Start MQTT connection
	if err := s.Start(context.Background()); err != nil {
		s.mu.Lock()
		s.powerState = PowerOff
		s.mu.Unlock()
		return err
	}
	return nil
}

// PowerOff turns off the device, saves state, and stops MQTT connection.
func (s *Simulator) PowerOff() bool {
	s.mu.Lock()
	if s.powerState == PowerOff {
		s.mu.Unlock()
		return false
	}
	s.powerState = PowerOff
	s.mu.Unlock()

	// Stop MQTT connection
	s.Stop()

	// Save state before shutdown
	s.SaveState()
	return true
}

// DeepSleep puts the device into deep sleep, saves state, and stops MQTT connection.
func (s *Simulator) DeepSleep() bool {
	s.mu.Lock()
	if s.powerState == PowerDeepSleep {
		s.mu.Unlock()
		return false
	}
	s.powerState = PowerDeepSleep
	s.mu.Unlock()

	// Stop MQTT connection
	s.Stop()

	// Save state before sleep
	s.SaveState()
	return true
}

// --- Persistence ---

// PersistentState is the state saved to disk.
type PersistentState struct {
	Volume     int      `json:"volume"`
	Brightness int      `json:"brightness"`
	LightMode  string   `json:"light_mode"`
	PairWith   string   `json:"pair_with"`
	WifiStore  []string `json:"wifi_store"`
	WifiSSID   string   `json:"wifi_ssid"`
	WifiRSSI   float64  `json:"wifi_rssi"`
	SysVersion string   `json:"sys_version"`
}

// stateFilePath returns the path to the state file.
func (s *Simulator) stateFilePath() string {
	return s.cfg.GearID + ".json"
}

// SaveState saves the current state to disk.
func (s *Simulator) SaveState() error {
	s.mu.RLock()
	state := PersistentState{
		Volume:     s.volume,
		Brightness: s.brightness,
		LightMode:  s.lightMode,
		PairWith:   s.pairWith,
		WifiStore:  s.wifiStore,
		WifiSSID:   s.wifiSSID,
		WifiRSSI:   s.wifiRSSI,
		SysVersion: s.sysVersion,
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	path := s.stateFilePath()
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	slog.Info("state saved", "path", path)
	return nil
}

// LoadState loads the state from disk.
func (s *Simulator) LoadState() error {
	path := s.stateFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("no saved state found")
			return nil
		}
		return err
	}

	var state PersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	s.mu.Lock()
	s.volume = state.Volume
	s.brightness = state.Brightness
	s.lightMode = state.LightMode
	s.pairWith = state.PairWith
	s.wifiStore = state.WifiStore
	s.wifiSSID = state.WifiSSID
	s.wifiRSSI = state.WifiRSSI
	if state.SysVersion != "" {
		s.sysVersion = state.SysVersion
	}
	if s.wifiSSID != "" {
		s.wifiIP = "192.168.1.100"
		s.wifiGW = "192.168.1.1"
	}
	s.mu.Unlock()

	slog.Info("state loaded", "path", path)
	return nil
}

// LoadStateOrDefaults loads saved state if exists.
// fallbackVersion is used if no saved state or saved state has no version.
func (s *Simulator) LoadStateOrDefaults(fallbackVersion string) {
	// Set fallback version first
	s.mu.Lock()
	s.sysVersion = fallbackVersion
	s.mu.Unlock()

	path := s.stateFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		// No saved state - keep NewSimulator defaults + fallback version
		slog.Info("no saved state, using defaults", "version", fallbackVersion)
		return
	}

	var state PersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		slog.Error("failed to parse saved state", "error", err)
		return
	}

	s.mu.Lock()
	s.volume = state.Volume
	s.brightness = state.Brightness
	s.lightMode = state.LightMode
	s.pairWith = state.PairWith
	s.wifiStore = state.WifiStore
	s.wifiSSID = state.WifiSSID
	s.wifiRSSI = state.WifiRSSI
	if state.SysVersion != "" {
		s.sysVersion = state.SysVersion
	}
	// Keep fallbackVersion if not in saved state
	if s.wifiSSID != "" {
		s.wifiIP = "192.168.1.100"
		s.wifiGW = "192.168.1.1"
	}
	s.mu.Unlock()

	slog.Info("state loaded", "path", path, "version", s.sysVersion)
}

// Reset resets the device to factory defaults.
// Clears wifi_store, pair_status; resets volume/brightness to 100; preserves sysVersion.
func (s *Simulator) Reset() {
	s.mu.Lock()
	// Reset to factory defaults (sysVersion preserved)
	s.volume = 100
	s.brightness = 100
	s.lightMode = "auto"
	s.pairWith = ""
	s.wifiStore = []string{}
	s.wifiSSID = ""
	s.wifiRSSI = 0
	s.wifiIP = ""
	s.wifiGW = ""
	// sysVersion is preserved
	s.powerState = PowerOff
	s.mu.Unlock()

	// Save the reset state (preserves sysVersion in the file)
	if err := s.SaveState(); err != nil {
		slog.Error("failed to save reset state", "error", err)
	} else {
		slog.Info("factory reset complete")
	}
}
