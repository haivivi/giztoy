package chatgear

import (
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/jsontime"
)

func TestServerPort_NewAndClose(t *testing.T) {
	port := NewServerPort()
	if port == nil {
		t.Fatal("NewServerPort returned nil")
	}
	if err := port.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Double close should be safe
	if err := port.Close(); err != nil {
		t.Fatalf("Double Close: %v", err)
	}
}

func TestServerPort_Tracks(t *testing.T) {
	port := NewServerPort()
	defer port.Close()

	// Create background track
	_, bgCtrl, err := port.NewBackgroundTrack()
	if err != nil {
		t.Fatalf("NewBackgroundTrack: %v", err)
	}
	if port.BackgroundTrackCtrl() == nil {
		t.Error("BackgroundTrackCtrl is nil")
	}

	// Create foreground track
	_, fgCtrl, err := port.NewForegroundTrack()
	if err != nil {
		t.Fatalf("NewForegroundTrack: %v", err)
	}
	if port.ForegroundTrackCtrl() == nil {
		t.Error("ForegroundTrackCtrl is nil")
	}

	// Create overlay track
	_, ovCtrl, err := port.NewOverlayTrack()
	if err != nil {
		t.Fatalf("NewOverlayTrack: %v", err)
	}
	if port.OverlayTrackCtrl() == nil {
		t.Error("OverlayTrackCtrl is nil")
	}

	// Close tracks
	bgCtrl.CloseWithError(nil)
	fgCtrl.CloseWithError(nil)
	ovCtrl.CloseWithError(nil)
}

func TestServerPort_Interrupt(t *testing.T) {
	port := NewServerPort()
	defer port.Close()

	// Create all tracks
	port.NewBackgroundTrack()
	port.NewForegroundTrack()
	port.NewOverlayTrack()

	// Verify tracks exist
	if port.BackgroundTrackCtrl() == nil {
		t.Error("BackgroundTrackCtrl is nil before interrupt")
	}
	if port.ForegroundTrackCtrl() == nil {
		t.Error("ForegroundTrackCtrl is nil before interrupt")
	}
	if port.OverlayTrackCtrl() == nil {
		t.Error("OverlayTrackCtrl is nil before interrupt")
	}

	// Interrupt
	port.Interrupt()

	// Verify tracks are cleared
	if port.BackgroundTrackCtrl() != nil {
		t.Error("BackgroundTrackCtrl should be nil after interrupt")
	}
	if port.ForegroundTrackCtrl() != nil {
		t.Error("ForegroundTrackCtrl should be nil after interrupt")
	}
	if port.OverlayTrackCtrl() != nil {
		t.Error("OverlayTrackCtrl should be nil after interrupt")
	}
}

func TestServerPort_StateGetters_Empty(t *testing.T) {
	port := NewServerPort()
	defer port.Close()

	// All getters should return not ok when empty
	if _, ok := port.State(); ok {
		t.Error("State should return not ok when empty")
	}
	if _, ok := port.Stats(); ok {
		t.Error("Stats should return not ok when empty")
	}
	if _, ok := port.Volume(); ok {
		t.Error("Volume should return not ok when empty")
	}
	if _, ok := port.LightMode(); ok {
		t.Error("LightMode should return not ok when empty")
	}
	if _, ok := port.Brightness(); ok {
		t.Error("Brightness should return not ok when empty")
	}
	if _, ok := port.WifiNetwork(); ok {
		t.Error("WifiNetwork should return not ok when empty")
	}
	if _, ok := port.WifiStore(); ok {
		t.Error("WifiStore should return not ok when empty")
	}
	if _, _, ok := port.Battery(); ok {
		t.Error("Battery should return not ok when empty")
	}
	if _, ok := port.SystemVersion(); ok {
		t.Error("SystemVersion should return not ok when empty")
	}
	if _, ok := port.Cellular(); ok {
		t.Error("Cellular should return not ok when empty")
	}
	if _, ok := port.PairStatus(); ok {
		t.Error("PairStatus should return not ok when empty")
	}
	if _, ok := port.ReadNFCTag(); ok {
		t.Error("ReadNFCTag should return not ok when empty")
	}
	if _, ok := port.Shaking(); ok {
		t.Error("Shaking should return not ok when empty")
	}
}

func TestServerPort_HandleStateEvent(t *testing.T) {
	port := NewServerPort()
	defer port.Close()

	now := time.Now()
	state := &StateEvent{
		Version: 1,
		Time:    jsontime.Milli(now),
		State:   StateRecording,
	}

	// Directly call handleStateEvent
	port.handleStateEvent(state)

	// Verify state is set
	s, ok := port.State()
	if !ok {
		t.Fatal("State should return ok")
	}
	if s.State != StateRecording {
		t.Errorf("State = %v; want StateRecording", s.State)
	}
}

func TestServerPort_HandleStateEvent_OutOfOrder(t *testing.T) {
	port := NewServerPort()
	defer port.Close()

	t1 := time.Now()
	t2 := t1.Add(-time.Hour) // Older event

	state1 := &StateEvent{
		Version: 1,
		Time:    jsontime.Milli(t1),
		State:   StateRecording,
	}
	state2 := &StateEvent{
		Version: 1,
		Time:    jsontime.Milli(t2),
		State:   StateReady, // Older state
	}

	port.handleStateEvent(state1)
	port.handleStateEvent(state2) // Should be ignored

	s, ok := port.State()
	if !ok {
		t.Fatal("State should return ok")
	}
	if s.State != StateRecording {
		t.Errorf("State = %v; want StateRecording (older event should be ignored)", s.State)
	}
}

func TestServerPort_HandleStatsEvent(t *testing.T) {
	port := NewServerPort()
	defer port.Close()

	stats := &StatsEvent{
		Volume:     &Volume{Percentage: 50},
		Battery:    &Battery{Percentage: 80, IsCharging: true},
		Brightness: &Brightness{Percentage: 70},
		LightMode:  &LightMode{Mode: "dark"},
	}

	// First stats sets baseline, returns nil
	changes := port.handleStatsEvent(stats)
	if changes != nil {
		t.Error("First stats should return nil changes")
	}

	// Verify stats are set
	vol, ok := port.Volume()
	if !ok || vol != 50 {
		t.Errorf("Volume = %d, ok=%v; want 50, true", vol, ok)
	}

	pct, charging, ok := port.Battery()
	if !ok || pct != 80 || !charging {
		t.Errorf("Battery = %d, %v, %v; want 80, true, true", pct, charging, ok)
	}

	brightness, ok := port.Brightness()
	if !ok || brightness != 70 {
		t.Errorf("Brightness = %d, %v; want 70, true", brightness, ok)
	}

	mode, ok := port.LightMode()
	if !ok || mode != "dark" {
		t.Errorf("LightMode = %s, %v; want dark, true", mode, ok)
	}
}

func TestServerPort_HandleStatsEvent_Changes(t *testing.T) {
	port := NewServerPort()
	defer port.Close()

	// Set initial stats
	stats1 := &StatsEvent{
		Volume: &Volume{Percentage: 50},
	}
	port.handleStatsEvent(stats1)

	// Update stats
	stats2 := &StatsEvent{
		Volume: &Volume{Percentage: 80},
	}
	changes := port.handleStatsEvent(stats2)

	if changes == nil {
		t.Fatal("Second stats should return changes")
	}
	if changes.Volume == nil || changes.Volume.Percentage != 80 {
		t.Error("Volume change not detected")
	}

	vol, ok := port.Volume()
	if !ok || vol != 80 {
		t.Errorf("Volume = %d, ok=%v; want 80, true", vol, ok)
	}
}

func TestServerPort_Commands(t *testing.T) {
	port := NewServerPort()
	defer port.Close()

	// Queue commands
	port.SetVolume(50)
	port.SetBrightness(80)
	port.SetLightMode("dark")
	port.SetWifi("test-ssid", "test-pass")
	port.DeleteWifi("old-ssid")
	port.Reset()
	port.Unpair()
	port.Sleep()
	port.Shutdown()
	port.RaiseCall()
	port.UpgradeFirmware(OTA{Version: "1.0.0", ImageURL: "http://example.com"})

	// Verify commands were queued (they're in commandQueue)
	// We can't easily read them without WriteTo, but at least verify no panic
}

func TestServerPort_StatsGetters(t *testing.T) {
	port := NewServerPort()
	defer port.Close()

	// Set full stats
	stats := &StatsEvent{
		WifiNetwork:   &ConnectedWifi{SSID: "test"},
		WifiStore:     &StoredWifiList{List: []WifiStoreItem{{SSID: "a"}, {SSID: "b"}}},
		SystemVersion: &SystemVersion{CurrentVersion: "1.0.0"},
		Cellular:      &ConnectedCellular{IP: "10.0.0.1"},
		PairStatus:    &PairStatus{PairWith: "device1"},
		ReadNFCTag:    &ReadNFCTag{Tags: []*NFCTag{{UID: "abc"}}},
		Shaking:       &Shaking{Level: 0.5},
	}
	port.handleStatsEvent(stats)

	// Verify all getters
	wifi, ok := port.WifiNetwork()
	if !ok || wifi.SSID != "test" {
		t.Errorf("WifiNetwork = %v, %v; want test, true", wifi, ok)
	}

	store, ok := port.WifiStore()
	if !ok || len(store.List) != 2 {
		t.Errorf("WifiStore = %v, %v", store, ok)
	}

	version, ok := port.SystemVersion()
	if !ok || version != "1.0.0" {
		t.Errorf("SystemVersion = %s, %v; want 1.0.0, true", version, ok)
	}

	cellular, ok := port.Cellular()
	if !ok || cellular.IP != "10.0.0.1" {
		t.Errorf("Cellular = %v, %v", cellular, ok)
	}

	pair, ok := port.PairStatus()
	if !ok || pair != "device1" {
		t.Errorf("PairStatus = %s, %v; want device1, true", pair, ok)
	}

	nfc, ok := port.ReadNFCTag()
	if !ok || len(nfc.Tags) != 1 {
		t.Errorf("ReadNFCTag = %v, %v", nfc, ok)
	}

	shaking, ok := port.Shaking()
	if !ok || shaking != 0.5 {
		t.Errorf("Shaking = %f, %v; want 0.5, true", shaking, ok)
	}
}

func TestServerPort_Poll(t *testing.T) {
	port := NewServerPort()

	// Close immediately - Poll should return error
	port.Close()

	_, err := port.Poll()
	if err == nil {
		t.Error("Poll should return error after close")
	}
}

func TestServerPort_ReadFrom_Audio(t *testing.T) {
	port := NewServerPort()
	defer port.Close()

	server, client := NewPipe()

	// Start ReadFrom in background
	done := make(chan error, 1)
	go func() {
		done <- port.ReadFrom(server)
	}()

	// Send audio from client
	client.SendOpusFrame(time.Now(), []byte{1, 2, 3})
	client.SendOpusFrame(time.Now(), []byte{4, 5, 6})

	// Give time for processing
	time.Sleep(50 * time.Millisecond)

	// Poll for data
	var audioCount int
	timeout := time.After(500 * time.Millisecond)
pollLoop:
	for {
		select {
		case <-timeout:
			break pollLoop
		default:
		}
		data, err := port.Poll()
		if err != nil {
			break pollLoop
		}
		if data.Audio != nil {
			audioCount++
		}
		if audioCount >= 2 {
			break pollLoop
		}
	}

	// Close to stop ReadFrom
	server.Close()
	client.Close()

	<-done

	if audioCount < 2 {
		t.Errorf("Expected 2 audio frames, got %d", audioCount)
	}
}

func TestServerPort_ReadFrom_State(t *testing.T) {
	port := NewServerPort()
	defer port.Close()

	server, client := NewPipe()

	// Start ReadFrom in background
	done := make(chan error, 1)
	go func() {
		done <- port.ReadFrom(server)
	}()

	// Send state from client
	client.SendState(&StateEvent{Version: 1, State: StateReady})

	// Give time for processing
	time.Sleep(50 * time.Millisecond)

	// Poll for data
	var stateCount int
	timeout := time.After(500 * time.Millisecond)
pollLoop:
	for {
		select {
		case <-timeout:
			break pollLoop
		default:
		}
		data, err := port.Poll()
		if err != nil {
			break pollLoop
		}
		if data.State != nil {
			stateCount++
			break pollLoop
		}
	}

	// Close to stop ReadFrom
	server.Close()
	client.Close()

	<-done

	if stateCount < 1 {
		t.Errorf("Expected at least 1 state, got %d", stateCount)
	}

	// Verify state was cached
	state, ok := port.State()
	if !ok {
		t.Error("State should be cached")
	}
	if state.State != StateReady {
		t.Errorf("State = %v; want StateReady", state.State)
	}
}

func TestServerPort_ReadFrom_Stats(t *testing.T) {
	port := NewServerPort()
	defer port.Close()

	server, client := NewPipe()

	// Start ReadFrom in background
	done := make(chan error, 1)
	go func() {
		done <- port.ReadFrom(server)
	}()

	// Send stats from client
	client.SendStats(&StatsEvent{Volume: &Volume{Percentage: 50}})

	// Give time for processing
	time.Sleep(50 * time.Millisecond)

	// Close to stop ReadFrom (stats changes only emitted on diff)
	server.Close()
	client.Close()

	<-done

	// Verify stats was cached
	stats, ok := port.Stats()
	if !ok {
		t.Error("Stats should be cached")
	}
	if stats.Volume == nil || stats.Volume.Percentage != 50 {
		t.Errorf("Volume = %v; want 50", stats.Volume)
	}
}

func TestServerPort_WriteTo_Commands(t *testing.T) {
	port := NewServerPort()
	defer port.Close()

	server, client := NewPipe()

	// Start WriteTo in background
	done := make(chan error, 1)
	go func() {
		done <- port.WriteTo(server)
	}()

	// Issue commands
	port.SetVolume(50)
	port.SetBrightness(80)

	// Give time for processing
	time.Sleep(50 * time.Millisecond)

	// Read commands on client side
	var cmdCount int
	timeout := time.After(500 * time.Millisecond)
cmdLoop:
	for {
		select {
		case <-timeout:
			break cmdLoop
		default:
		}
		for cmd, err := range client.Commands() {
			if err != nil {
				break cmdLoop
			}
			_ = cmd
			cmdCount++
			if cmdCount >= 2 {
				break cmdLoop
			}
		}
	}

	// Close to stop WriteTo
	port.Close()
	server.Close()
	client.Close()

	<-done

	if cmdCount < 2 {
		t.Errorf("Expected at least 2 commands, got %d", cmdCount)
	}
}
