package gear

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/haivivi/giztoy/go/pkg/chatgear"
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

	// chatgear port and connection
	port *chatgear.ClientPort
	conn *chatgear.MQTTClientConn

	// WebRTC for browser audio
	webrtc  *WebRTCBridge
	mic     *WebRTCMic
	speaker *WebRTCSpeaker

	// Web server for log forwarding
	webServer *WebServer

	mu         sync.RWMutex
	powerState PowerState

	// Simulated device state (for reading back values)
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

	// Channels for web events
	events chan SimulatorEvent

	ctx    context.Context
	cancel context.CancelFunc
}

// NewSimulator creates a new simulator with default device state.
func NewSimulator(cfg SimulatorConfig) *Simulator {
	s := &Simulator{
		cfg:        cfg,
		powerState: PowerOff, // Start powered off
		events:     make(chan SimulatorEvent, 100),
		webrtc:     NewWebRTCBridge(),

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

	return s
}

// Events returns the event channel.
func (s *Simulator) Events() <-chan SimulatorEvent {
	return s.events
}

// SetWebServer sets the web server for log forwarding.
func (s *Simulator) SetWebServer(ws *WebServer) {
	s.webServer = ws
}

// webLogger wraps chatgear.Logger and forwards logs to WebServer.
type webLogger struct {
	ws *WebServer

	// Audio frame counters for summary logging
	txAudioFrames int
	rxAudioFrames int
	lastAudioLog  time.Time
}

func (l *webLogger) InfoPrintf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slog.Info("chatgear: " + msg)
	if l.ws != nil {
		l.ws.AddLog(fmt.Sprintf(`{"type":"info","msg":%q}`, msg))
	}
}

func (l *webLogger) WarnPrintf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slog.Warn("chatgear: " + msg)
	if l.ws != nil {
		l.ws.AddLog(fmt.Sprintf(`{"type":"warn","msg":%q}`, msg))
	}
}

func (l *webLogger) DebugPrintf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slog.Debug("chatgear: " + msg)

	// Forward audio logs as summary every second
	if l.ws != nil {
		if strings.Contains(msg, "MQTT TX audio") {
			l.txAudioFrames++
			l.maybeLogAudioSummary()
		} else if strings.Contains(msg, "MQTT RX audio") {
			l.rxAudioFrames++
			l.maybeLogAudioSummary()
		}
	}
}

func (l *webLogger) maybeLogAudioSummary() {
	now := time.Now()
	if now.Sub(l.lastAudioLog) >= time.Second {
		if l.txAudioFrames > 0 || l.rxAudioFrames > 0 {
			msg := fmt.Sprintf("Audio: TX %d frames, RX %d frames (last 1s)", l.txAudioFrames, l.rxAudioFrames)
			l.ws.AddLog(fmt.Sprintf(`{"type":"debug","msg":%q}`, msg))
			l.txAudioFrames = 0
			l.rxAudioFrames = 0
		}
		l.lastAudioLog = now
	}
}

func (l *webLogger) ErrorPrintf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slog.Error("chatgear: " + msg)
	if l.ws != nil {
		l.ws.AddLog(fmt.Sprintf(`{"type":"error","msg":%q}`, msg))
	}
}

func (l *webLogger) Errorf(format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	slog.Error("chatgear: " + msg)
	if l.ws != nil {
		l.ws.AddLog(fmt.Sprintf(`{"type":"error","msg":%q}`, msg))
	}
	return fmt.Errorf(format, args...)
}

// State returns the current state.
func (s *Simulator) State() chatgear.State {
	if s.port != nil {
		return s.port.State()
	}
	return chatgear.StateReady
}

// WebRTC returns the WebRTC bridge for audio I/O.
func (s *Simulator) WebRTC() *WebRTCBridge {
	return s.webrtc
}

// Start starts the simulator.
func (s *Simulator) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	slog.Info("connecting to MQTT broker", "url", s.cfg.MQTTURL)

	// Connect to MQTT using chatgear.DialMQTT with custom logger
	conn, err := chatgear.DialMQTT(s.ctx, chatgear.MQTTClientConfig{
		Addr:   s.cfg.MQTTURL,
		Scope:  s.cfg.Namespace,
		GearID: s.cfg.GearID,
		Logger: &webLogger{ws: s.webServer},
	})
	if err != nil {
		slog.Error("MQTT connection failed", "error", err)
		return err
	}
	s.conn = conn
	slog.Info("MQTT connected successfully")

	// Create ClientPort
	s.port = chatgear.NewClientPort()

	// Start periodic state/stats reporting (protocol requirement)
	s.port.StartPeriodicReporting(s.ctx)

	// Create WebRTC audio adapters
	mic, err := NewWebRTCMic(s.webrtc)
	if err != nil {
		s.conn.Close()
		return fmt.Errorf("create webrtc mic: %w", err)
	}
	s.mic = mic

	speaker, err := NewWebRTCSpeaker(s.webrtc)
	if err != nil {
		s.mic.Close()
		s.conn.Close()
		return fmt.Errorf("create webrtc speaker: %w", err)
	}
	s.speaker = speaker

	// Start ClientPort goroutines
	go func() {
		if err := s.port.ReadFromMic(s.mic); err != nil {
			slog.Info("ReadFromMic ended", "error", err)
		}
	}()
	go func() {
		if err := s.port.WriteToSpeaker(s.speaker); err != nil {
			slog.Info("WriteToSpeaker ended", "error", err)
		}
	}()
	go func() {
		if err := s.port.ReadFrom(s.conn); err != nil {
			slog.Info("ReadFrom ended", "error", err)
		}
	}()
	go func() {
		if err := s.port.WriteTo(s.conn); err != nil {
			slog.Info("WriteTo ended", "error", err)
		}
	}()

	// Handle commands from ClientPort
	go s.handleCommands()

	// Set initial state and stats
	s.port.SetState(chatgear.StateReady)
	s.initializeStats()

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
	if s.mic != nil {
		s.mic.Close()
		s.mic = nil
	}
	if s.speaker != nil {
		s.speaker.Close()
		s.speaker = nil
	}
	if s.port != nil {
		s.port.Close()
		s.port = nil
	}
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	// Note: WebRTC bridge is not closed here - it persists across power cycles
	// This allows browser to stay connected while device is powered off
	slog.Info("simulator stopped")
}

// handleCommands handles commands from the server.
func (s *Simulator) handleCommands() {
	slog.Info("command handler started")
	for cmd, err := range s.port.Commands() {
		if err != nil {
			slog.Error("command receive error", "error", err)
			break
		}
		slog.Info("received command", "type", fmt.Sprintf("%T", cmd.Payload))
		data, _ := json.Marshal(cmd)
		s.emitEvent("command_received", string(data))
		s.applyCommand(cmd)
	}
	slog.Info("command handler stopped")
}

// applyCommand applies a command to the simulator state.
func (s *Simulator) applyCommand(cmd *chatgear.CommandEvent) {
	switch t := cmd.Payload.(type) {
	case *chatgear.Streaming:
		slog.Info("cmd: streaming", "value", bool(*t))
		if bool(*t) {
			if s.port.State() == chatgear.StateWaitingForResponse {
				s.port.SetState(chatgear.StateStreaming)
			}
		} else {
			if s.port.State() == chatgear.StateStreaming {
				s.port.SetState(chatgear.StateReady)
			}
		}

	case *chatgear.SetVolume:
		v := min(max(int(*t), 0), 100)
		slog.Info("cmd: set_volume", "value", v)
		s.mu.Lock()
		s.volume = v
		s.mu.Unlock()
		s.port.SetVolume(v)

	case *chatgear.SetBrightness:
		b := min(max(int(*t), 0), 100)
		slog.Info("cmd: set_brightness", "value", b)
		s.mu.Lock()
		s.brightness = b
		s.mu.Unlock()
		s.port.SetBrightness(b)

	case *chatgear.SetLightMode:
		mode := string(*t)
		slog.Info("cmd: set_light_mode", "value", mode)
		s.mu.Lock()
		s.lightMode = mode
		s.mu.Unlock()
		s.port.SetLightMode(mode)

	case *chatgear.SetWifi:
		slog.Info("cmd: set_wifi", "ssid", t.SSID)
		s.mu.Lock()
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
		s.mu.Unlock()
		s.port.SetWifiNetwork(&chatgear.ConnectedWifi{
			SSID:    t.SSID,
			RSSI:    -50,
			IP:      "192.168.1.100",
			Gateway: "192.168.1.1",
		})

	case *chatgear.DeleteWifi:
		ssid := string(*t)
		slog.Info("cmd: delete_wifi", "ssid", ssid)
		s.mu.Lock()
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
		s.mu.Unlock()

	case *chatgear.Reset:
		slog.Info("cmd: reset", "unpair", t.Unpair)
		s.mu.Lock()
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
		s.mu.Unlock()
		// Report changes via port
		s.port.SetVolume(100)
		s.port.SetBrightness(100)
		s.port.SetLightMode("auto")

	case *chatgear.Halt:
		slog.Info("cmd: halt", "sleep", t.Sleep, "shutdown", t.Shutdown, "interrupt", t.Interrupt)
		if t.Interrupt {
			st := s.port.State()
			if st == chatgear.StateStreaming || st == chatgear.StateRecording ||
				st == chatgear.StateWaitingForResponse {
				s.port.SetState(chatgear.StateReady)
			}
		} else if t.Sleep || t.Shutdown {
			s.port.SetState(chatgear.StateReady)
		}

	case *chatgear.Raise:
		slog.Info("cmd: raise", "call", t.Call)
		if t.Call {
			if s.port.State() == chatgear.StateReady {
				s.port.SetState(chatgear.StateCalling)
			}
		}

	case *chatgear.OTA:
		slog.Info("cmd: ota_upgrade", "version", t.Version)
		go func(ctx context.Context) {
			s.mu.Lock()
			oldVersion := s.sysVersion
			s.mu.Unlock()

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
			s.port.SetSystemVersion(t.Version)

			slog.Info("OTA complete", "from", oldVersion, "to", t.Version)
		}(s.ctx)

	default:
		slog.Warn("cmd: unknown type", "type", fmt.Sprintf("%T", cmd.Payload))
	}
}

// initializeStats sets initial stats values on the ClientPort.
func (s *Simulator) initializeStats() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Use batch mode to send only one stats update at the end
	s.port.BeginBatch()
	defer s.port.EndBatch()

	// Set all initial stats via ClientPort
	s.port.SetVolume(s.volume)
	s.port.SetBrightness(s.brightness)
	s.port.SetLightMode(s.lightMode)
	s.port.SetBattery(int(s.batteryPct), s.charging)
	s.port.SetSystemVersion(s.sysVersion)
	if s.wifiSSID != "" {
		s.port.SetWifiNetwork(&chatgear.ConnectedWifi{
			SSID:    s.wifiSSID,
			RSSI:    s.wifiRSSI,
			IP:      s.wifiIP,
			Gateway: s.wifiGW,
		})
	}
}

// StartRecording starts recording (button pressed).
// Valid from: READY, WAITING_FOR_RESPONSE, STREAMING (interrupt and re-record)
func (s *Simulator) StartRecording() bool {
	if s.port == nil {
		return false
	}
	switch s.port.State() {
	case chatgear.StateReady, chatgear.StateWaitingForResponse, chatgear.StateStreaming:
		s.port.SetState(chatgear.StateRecording)
		return true
	default:
		return false
	}
}

// EndRecording ends recording and waits for response (button released).
// Valid from: RECORDING
func (s *Simulator) EndRecording() bool {
	if s.port == nil {
		return false
	}
	if s.port.State() == chatgear.StateRecording {
		s.port.SetState(chatgear.StateWaitingForResponse)
		return true
	}
	return false
}

// StartCalling starts calling mode.
// Valid from: READY
func (s *Simulator) StartCalling() bool {
	if s.port == nil {
		return false
	}
	if s.port.State() == chatgear.StateReady {
		s.port.SetState(chatgear.StateCalling)
		return true
	}
	return false
}

// EndCalling ends calling mode.
// Valid from: CALLING
func (s *Simulator) EndCalling() bool {
	if s.port == nil {
		return false
	}
	if s.port.State() == chatgear.StateCalling {
		s.port.SetState(chatgear.StateReady)
		return true
	}
	return false
}

// Cancel cancels/interrupts current operation back to READY.
// Valid from: any state except READY
func (s *Simulator) Cancel() bool {
	if s.port == nil {
		return false
	}
	if s.port.State() != chatgear.StateReady {
		s.port.SetState(chatgear.StateReady)
		return true
	}
	return false
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

// --- Setters for web commands ---
// All setters update local state and report via ClientPort.

// SetVolume sets volume (0-100).
func (s *Simulator) SetVolume(v int) {
	v = min(max(v, 0), 100)
	s.mu.Lock()
	s.volume = v
	s.mu.Unlock()
	if s.port != nil {
		s.port.SetVolume(v)
	}
}

// SetBrightness sets brightness (0-100).
func (s *Simulator) SetBrightness(b int) {
	b = min(max(b, 0), 100)
	s.mu.Lock()
	s.brightness = b
	s.mu.Unlock()
	if s.port != nil {
		s.port.SetBrightness(b)
	}
}

// SetLightMode sets light mode ("auto", "on", "off").
func (s *Simulator) SetLightMode(mode string) {
	s.mu.Lock()
	s.lightMode = mode
	s.mu.Unlock()
	if s.port != nil {
		s.port.SetLightMode(mode)
	}
}

// SetBattery sets battery status.
func (s *Simulator) SetBattery(pct float64, charging bool) {
	pct = min(max(pct, 0), 100)
	s.mu.Lock()
	s.batteryPct = pct
	s.charging = charging
	s.mu.Unlock()
	if s.port != nil {
		s.port.SetBattery(int(pct), charging)
	}
}

// SetWifi sets wifi status.
func (s *Simulator) SetWifi(ssid string, rssi float64) {
	s.mu.Lock()
	s.wifiSSID = ssid
	s.wifiRSSI = rssi
	if ssid != "" {
		s.wifiIP = "192.168.1.100"
		s.wifiGW = "192.168.1.1"
	} else {
		s.wifiIP = ""
		s.wifiGW = ""
	}
	s.mu.Unlock()
	if s.port != nil {
		if ssid != "" {
			s.port.SetWifiNetwork(&chatgear.ConnectedWifi{
				SSID:    ssid,
				RSSI:    rssi,
				IP:      "192.168.1.100",
				Gateway: "192.168.1.1",
			})
		} else {
			s.port.SetWifiNetwork(nil)
		}
	}
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
	s.mu.Unlock()
	// Note: PairStatus reporting not yet implemented in ClientPort
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
	s.mu.Unlock()
	// Note: Shaking reporting not yet implemented in ClientPort
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
	for _, stored := range s.wifiStore {
		if stored == ssid {
			s.mu.Unlock()
			return
		}
	}
	s.wifiStore = append(s.wifiStore, ssid)
	s.mu.Unlock()
	// Note: WifiStore reporting not yet implemented in ClientPort
}

// RemoveWifiStore removes a WiFi SSID from stored list.
func (s *Simulator) RemoveWifiStore(ssid string) {
	s.mu.Lock()
	for i, stored := range s.wifiStore {
		if stored == ssid {
			s.wifiStore = append(s.wifiStore[:i], s.wifiStore[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
}

// SetVersion sets the system version.
func (s *Simulator) SetVersion(version string) {
	s.mu.Lock()
	s.sysVersion = version
	s.mu.Unlock()
	if s.port != nil {
		s.port.SetSystemVersion(version)
	}
}

// --- Status Helper ---

// StatusSummary returns a formatted status summary for display.
func (s *Simulator) StatusSummary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chargingIcon := ""
	if s.charging {
		chargingIcon = "+"
	}

	wifiInfo := "disconnected"
	if s.wifiSSID != "" {
		wifiInfo = s.wifiSSID
	}

	state := chatgear.StateReady
	if s.port != nil {
		state = s.port.State()
	}

	return fmt.Sprintf(
		"State: %-12s | Batt: %.0f%%%s | Vol: %d%% | Bright: %d%% (%s) | WiFi: %s",
		state.String(),
		s.batteryPct,
		chargingIcon,
		s.volume,
		s.brightness,
		s.lightMode,
		wifiInfo,
	)
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
