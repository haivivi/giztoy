package chatgear

import (
	"io"
	"testing"
	"testing/synctest"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/pcm"
)

func TestClientPort_NewAndClose(t *testing.T) {
	port := NewClientPort()
	if port == nil {
		t.Fatal("NewClientPort returned nil")
	}
	if err := port.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Double close should be safe
	if err := port.Close(); err != nil {
		t.Fatalf("Double Close: %v", err)
	}
}

func TestClientPort_State(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	// Initial state should be StateUnknown (zero value)
	if port.State() != StateUnknown {
		t.Errorf("Initial state = %v; want StateUnknown", port.State())
	}

	// Set state
	port.SetState(StateRecording)
	if port.State() != StateRecording {
		t.Errorf("State = %v; want StateRecording", port.State())
	}

	// Set same state should not queue again
	port.SetState(StateRecording)

	// Change state
	port.SetState(StateReady)
	if port.State() != StateReady {
		t.Errorf("State = %v; want StateReady", port.State())
	}
}

func TestClientPort_SetVolume(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	port.SetVolume(50)
	port.SetVolume(80)
	// Verify no panic, stats are internal
}

func TestClientPort_SetBrightness(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	port.SetBrightness(50)
	port.SetBrightness(100)
}

func TestClientPort_SetLightMode(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	port.SetLightMode("dark")
	port.SetLightMode("light")
}

func TestClientPort_SetBattery(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	port.SetBattery(80, true)
	port.SetBattery(50, false)
}

func TestClientPort_SetWifiNetwork(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	port.SetWifiNetwork(&ConnectedWifi{SSID: "test"})
	port.SetWifiNetwork(nil)
}

func TestClientPort_SetWifiStore(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	port.SetWifiStore(&StoredWifiList{List: []WifiStoreItem{{SSID: "a"}, {SSID: "b"}}})
	port.SetWifiStore(nil)
}

func TestClientPort_SetSystemVersion(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	port.SetSystemVersion("1.0.0")
	port.SetSystemVersion("2.0.0")
}

func TestClientPort_SetCellular(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	port.SetCellular(&ConnectedCellular{IP: "10.0.0.1"})
	port.SetCellular(nil)
}

func TestClientPort_SetPairStatus(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	port.SetPairStatus("device1")
	port.SetPairStatus("")
}

func TestClientPort_SetReadNFCTag(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	port.SetReadNFCTag(&ReadNFCTag{Tags: []*NFCTag{{UID: "abc"}}})
	port.SetReadNFCTag(nil)
}

func TestClientPort_SetShaking(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	port.SetShaking(0.5)
	port.SetShaking(1.0)
}

func TestClientPort_ReadFromMic_Nil(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	// ReadFromMic with nil should return immediately
	if err := port.ReadFromMic(nil); err != nil {
		t.Errorf("ReadFromMic(nil) = %v; want nil", err)
	}
}

func TestClientPort_WriteToSpeaker_Nil(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	// WriteToSpeaker with nil should return immediately
	if err := port.WriteToSpeaker(nil); err != nil {
		t.Errorf("WriteToSpeaker(nil) = %v; want nil", err)
	}
}

func TestClientPort_MultipleStateChanges(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	states := []State{
		StateReady,
		StateRecording,
		StateWaitingForResponse,
		StateStreaming,
		StateInterrupted,
		StateReady,
	}

	for _, s := range states {
		port.SetState(s)
		if port.State() != s {
			t.Errorf("State = %v; want %v", port.State(), s)
		}
	}
}

func TestClientPort_AllStats(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	// Set all stats types
	port.SetVolume(50)
	port.SetBrightness(80)
	port.SetLightMode("dark")
	port.SetBattery(75, true)
	port.SetWifiNetwork(&ConnectedWifi{SSID: "test", IP: "192.168.1.1"})
	port.SetWifiStore(&StoredWifiList{List: []WifiStoreItem{{SSID: "a"}, {SSID: "b"}, {SSID: "c"}}})
	port.SetSystemVersion("1.2.3")
	port.SetCellular(&ConnectedCellular{IP: "10.0.0.1"})
	port.SetPairStatus("paired-device")
	port.SetReadNFCTag(&ReadNFCTag{Tags: []*NFCTag{{UID: "tag1"}, {UID: "tag2"}}})
	port.SetShaking(0.75)

	// All calls should succeed without panic
}

func TestClientPort_ReadFrom_Commands(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	server, client := NewPipe()

	// Start ReadFrom in background
	done := make(chan error, 1)
	go func() {
		done <- port.ReadFrom(client)
	}()

	// Send commands from server
	server.IssueCommand(NewSetVolume(50), time.Now())
	server.IssueCommand(NewSetVolume(60), time.Now())

	// Give time for processing
	time.Sleep(50 * time.Millisecond)

	// Read commands from port
	var cmdCount int
	for cmd, err := range port.Commands() {
		if err != nil {
			break
		}
		if cmd.Type == "set_volume" {
			cmdCount++
		}
		if cmdCount >= 2 {
			break
		}
	}

	// Close to stop ReadFrom
	client.Close()
	server.Close()

	<-done

	if cmdCount < 2 {
		t.Errorf("Expected 2 commands, got %d", cmdCount)
	}
}

func TestClientPort_WriteTo_State(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	server, client := NewPipe()

	// Start WriteTo in background
	done := make(chan error, 1)
	go func() {
		done <- port.WriteTo(client)
	}()

	// Set state - this queues for upload
	port.SetState(StateReady)
	port.SetState(StateRecording)

	// Give time for WriteTo to process
	time.Sleep(50 * time.Millisecond)

	// Read states on server side
	var stateCount int
	timeout := time.After(500 * time.Millisecond)
stateLoop:
	for {
		select {
		case <-timeout:
			break stateLoop
		default:
		}
		for state, err := range server.States() {
			if err != nil {
				break stateLoop
			}
			_ = state
			stateCount++
			if stateCount >= 2 {
				break stateLoop
			}
		}
	}

	// Close to stop WriteTo
	port.Close()
	client.Close()
	server.Close()

	<-done

	if stateCount < 1 {
		t.Errorf("Expected at least 1 state, got %d", stateCount)
	}
}

func TestClientPort_WriteTo_Stats(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	server, client := NewPipe()

	// Start WriteTo in background
	done := make(chan error, 1)
	go func() {
		done <- port.WriteTo(client)
	}()

	// Set stats - this queues for upload
	port.SetVolume(50)
	port.SetBrightness(80)

	// Give time for WriteTo to process
	time.Sleep(50 * time.Millisecond)

	// Read stats on server side
	var statsCount int
	timeout := time.After(500 * time.Millisecond)
statsLoop:
	for {
		select {
		case <-timeout:
			break statsLoop
		default:
		}
		for stats, err := range server.Stats() {
			if err != nil {
				break statsLoop
			}
			_ = stats
			statsCount++
			if statsCount >= 2 {
				break statsLoop
			}
		}
	}

	// Close to stop WriteTo
	port.Close()
	client.Close()
	server.Close()

	<-done

	if statsCount < 1 {
		t.Errorf("Expected at least 1 stats, got %d", statsCount)
	}
}

// mockMic is a test implementation of Mic interface
type mockMic struct {
	format    pcm.Format
	data      []int16
	pos       int
	readCount int
	maxReads  int
}

func newMockMic(format pcm.Format, data []int16, maxReads int) *mockMic {
	return &mockMic{
		format:   format,
		data:     data,
		maxReads: maxReads,
	}
}

func (m *mockMic) Read(buf []int16) (int, error) {
	if m.readCount >= m.maxReads {
		return 0, io.EOF
	}
	m.readCount++

	n := copy(buf, m.data[m.pos:])
	m.pos += n
	if m.pos >= len(m.data) {
		m.pos = 0 // Loop
	}
	return n, nil
}

func (m *mockMic) Format() pcm.Format {
	return m.format
}

// mockSpeaker is a test implementation of Speaker interface
type mockSpeaker struct {
	format     pcm.Format
	data       []int16
	writeCount int
	maxWrites  int
}

func newMockSpeaker(format pcm.Format, maxWrites int) *mockSpeaker {
	return &mockSpeaker{
		format:    format,
		maxWrites: maxWrites,
	}
}

func (s *mockSpeaker) Write(buf []int16) (int, error) {
	if s.writeCount >= s.maxWrites {
		return 0, io.EOF
	}
	s.writeCount++
	s.data = append(s.data, buf...)
	return len(buf), nil
}

func (s *mockSpeaker) Format() pcm.Format {
	return s.format
}

func TestClientPort_ReadFromMic(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	// Create mock mic with 20ms of audio at 24kHz mono
	format := pcm.L16Mono24K
	frameSize := int(format.SamplesInDuration(20 * time.Millisecond))
	audioData := make([]int16, frameSize*3) // 3 frames worth
	for i := range audioData {
		audioData[i] = int16(i % 1000)
	}
	mic := newMockMic(format, audioData, 3)

	// Start ReadFromMic
	done := make(chan error, 1)
	go func() {
		done <- port.ReadFromMic(mic)
	}()

	// Wait for it to complete (mic returns EOF after maxReads)
	err := <-done
	if err != io.EOF {
		t.Errorf("Expected io.EOF, got %v", err)
	}
}

func TestClientPort_WriteToSpeaker(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	// Create mock speaker
	format := pcm.L16Mono24K
	speaker := newMockSpeaker(format, 10)

	// Queue some audio frames - mix of valid and invalid
	port.downlinkAudio.Add(StampedOpusFrame{
		Timestamp: time.Now(),
		Frame:     []byte{0x00}, // Invalid frame - will be skipped
	})
	port.downlinkAudio.Add(StampedOpusFrame{
		Timestamp: time.Now(),
		Frame:     []byte{0xfc, 0xff, 0xfe}, // Another frame
	})

	// Close the queue to signal end
	port.downlinkAudio.Close()

	// Start WriteToSpeaker
	err := port.WriteToSpeaker(speaker)
	if err != nil {
		t.Logf("WriteToSpeaker returned: %v", err)
	}
}

func TestClientPort_WriteToSpeaker_SpeakerError(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	// Create mock speaker that errors
	format := pcm.L16Mono24K
	speaker := &errorSpeaker{format: format}

	// Queue a valid opus frame - we need to encode something first
	// Use a simple frame that opus can decode
	port.downlinkAudio.Add(StampedOpusFrame{
		Timestamp: time.Now(),
		Frame:     []byte{0xfc, 0xff, 0xfe}, // Try to decode
	})

	// Close the queue to signal end
	port.downlinkAudio.Close()

	// WriteToSpeaker should handle decode errors gracefully
	err := port.WriteToSpeaker(speaker)
	// Either nil (if all frames failed to decode) or error from speaker
	t.Logf("WriteToSpeaker returned: %v", err)
}

// errorSpeaker always returns error on write
type errorSpeaker struct {
	format pcm.Format
}

func (s *errorSpeaker) Write(buf []int16) (int, error) {
	return 0, io.ErrClosedPipe
}

func (s *errorSpeaker) Format() pcm.Format {
	return s.format
}

func TestClientPort_Commands_Empty(t *testing.T) {
	port := NewClientPort()

	// Close immediately
	port.Close()

	// Commands should not block forever
	for cmd, err := range port.Commands() {
		if err != nil {
			break
		}
		_ = cmd
	}
}

func TestBytesToInt16(t *testing.T) {
	// Test conversion
	bytes := []byte{0x01, 0x00, 0xFF, 0x7F} // [1, 32767] in little-endian
	samples := bytesToInt16(bytes)

	if len(samples) != 2 {
		t.Fatalf("Expected 2 samples, got %d", len(samples))
	}
	if samples[0] != 1 {
		t.Errorf("samples[0] = %d; want 1", samples[0])
	}
	if samples[1] != 32767 {
		t.Errorf("samples[1] = %d; want 32767", samples[1])
	}
}

func TestClientPort_ReadFrom_OpusAndCommands(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	server, client := NewPipe()

	// Start ReadFrom in background
	done := make(chan error, 1)
	go func() {
		done <- port.ReadFrom(client)
	}()

	// Send opus frames from server
	server.SendOpusFrame(time.Now(), []byte{0xfc, 0xff, 0xfe})
	server.SendOpusFrame(time.Now(), []byte{0xfc, 0xff, 0xfe})

	// Send commands
	server.IssueCommand(NewSetVolume(50), time.Now())

	// Give time for processing
	time.Sleep(50 * time.Millisecond)

	// Close connections
	client.Close()
	server.Close()

	// Wait for ReadFrom to finish
	err := <-done
	if err != nil {
		t.Logf("ReadFrom returned: %v", err)
	}
}

func TestClientPort_WriteTo_Audio(t *testing.T) {
	port := NewClientPort()

	server, client := NewPipe()

	// Add audio to uplink queue
	port.uplinkAudio.Add(StampedOpusFrame{
		Timestamp: time.Now(),
		Frame:     []byte{0xfc, 0xff, 0xfe},
	})
	port.uplinkAudio.Add(StampedOpusFrame{
		Timestamp: time.Now(),
		Frame:     []byte{0xfc, 0xff, 0xfe},
	})

	// Start WriteTo in background
	done := make(chan error, 1)
	go func() {
		done <- port.WriteTo(client)
	}()

	// Give time for WriteTo to start
	time.Sleep(50 * time.Millisecond)

	// Close to stop WriteTo
	port.Close()
	client.Close()
	server.Close()

	// Wait for WriteTo to finish
	err := <-done
	if err != nil {
		t.Logf("WriteTo returned: %v", err)
	}
}

func TestClientPort_SetLogger(t *testing.T) {
	port := NewClientPort()
	defer port.Close()

	// Basic operations with custom logger
	port.SetState(StateReady)
	port.SetVolume(50)
}

// =============================================================================
// Periodic Reporting Tests (using synctest)
// =============================================================================

func TestClientPort_StatePeriodic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		port := NewClientPort()
		defer port.Close()

		// Set initial state
		port.SetState(StateReady)

		// Drain initial state event from SetState
		initialCount := drainStateBuffer(port)
		if initialCount != 1 {
			t.Errorf("Expected 1 initial state event, got %d", initialCount)
		}

		// Start periodic reporting
		ctx := t.Context()
		port.StartPeriodicReporting(ctx)

		// Before 5s - no periodic state yet
		time.Sleep(4 * time.Second)
		synctest.Wait()
		count := drainStateBuffer(port)
		if count != 0 {
			t.Errorf("Before 5s: expected 0 state events, got %d", count)
		}

		// At 5s - should get first periodic state
		time.Sleep(1 * time.Second)
		synctest.Wait()
		count = drainStateBuffer(port)
		if count != 1 {
			t.Errorf("At 5s: expected 1 state event, got %d", count)
		}

		// At 10s - should get second periodic state
		time.Sleep(5 * time.Second)
		synctest.Wait()
		count = drainStateBuffer(port)
		if count != 1 {
			t.Errorf("At 10s: expected 1 state event, got %d", count)
		}
	})
}

func TestClientPort_StateImmediateOnChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		port := NewClientPort()
		defer port.Close()

		// Set initial state and drain
		port.SetState(StateReady)
		drainStateBuffer(port)

		// Start periodic reporting
		ctx := t.Context()
		port.StartPeriodicReporting(ctx)

		// Change state - should trigger immediate send
		port.SetState(StateRecording)
		synctest.Wait()

		count := drainStateBuffer(port)
		if count != 1 {
			t.Errorf("After state change: expected 1 immediate state event, got %d", count)
		}

		// Verify the state value
		if port.State() != StateRecording {
			t.Errorf("State = %v; want StateRecording", port.State())
		}
	})
}

func TestClientPort_StateCallingUpdatesTimestamp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		port := NewClientPort()
		defer port.Close()

		// Set CALLING state
		port.SetState(StateCalling)
		drainStateBuffer(port)

		// Start periodic reporting
		ctx := t.Context()
		port.StartPeriodicReporting(ctx)

		// Wait 5s - should get state with updated timestamp
		time.Sleep(5 * time.Second)
		synctest.Wait()
		count := drainStateBuffer(port)
		if count != 1 {
			t.Errorf("At 5s (CALLING): expected 1 state event, got %d", count)
		}

		// Wait another 5s - should get another state (CALLING always sends)
		time.Sleep(5 * time.Second)
		synctest.Wait()
		count = drainStateBuffer(port)
		if count != 1 {
			t.Errorf("At 10s (CALLING): expected 1 state event, got %d", count)
		}
	})
}

// drainStateBuffer reads all available state events from the buffer and returns count
func drainStateBuffer(port *ClientPort) int {
	count := 0
	for port.uplinkState.Len() > 0 {
		_, err := port.uplinkState.Next()
		if err != nil {
			break
		}
		count++
	}
	return count
}

// drainStatsBuffer reads all available stats events from the buffer and returns them
func drainStatsBuffer(port *ClientPort) []*StatsEvent {
	var events []*StatsEvent
	for port.uplinkStats.Len() > 0 {
		evt, err := port.uplinkStats.Next()
		if err != nil {
			break
		}
		events = append(events, evt)
	}
	return events
}

func TestClientPort_StatsTieredPeriodic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		port := NewClientPort()
		defer port.Close()

		// Initialize some stats data
		port.BeginBatch()
		port.SetVolume(50)
		port.SetBrightness(80)
		port.SetLightMode("auto")
		port.SetBattery(75, false)
		port.SetSystemVersion("1.0.0")
		port.SetWifiNetwork(&ConnectedWifi{SSID: "test", IP: "192.168.1.1"})
		port.SetShaking(0.5)
		port.SetWifiStore(&StoredWifiList{List: []WifiStoreItem{{SSID: "a"}}})
		port.EndBatch()

		// Drain initial batch stats
		drainStatsBuffer(port)

		// Start periodic reporting
		ctx := t.Context()
		port.StartPeriodicReporting(ctx)

		// Before 20s - no periodic stats yet
		time.Sleep(19 * time.Second)
		synctest.Wait()
		events := drainStatsBuffer(port)
		if len(events) != 0 {
			t.Errorf("Before 20s: expected 0 stats events, got %d", len(events))
		}

		// At 20s (tick 1, rounds=1) - tier 1: shaking, cellular
		time.Sleep(1 * time.Second)
		synctest.Wait()
		events = drainStatsBuffer(port)
		if len(events) != 1 {
			t.Errorf("At 20s: expected 1 stats event (tier 1), got %d", len(events))
		} else if events[0].Shaking == nil {
			t.Error("At 20s: expected Shaking field in tier 1 stats")
		}

		// At 40s (tick 2, rounds=2) - tier 2: wifi_store
		time.Sleep(20 * time.Second)
		synctest.Wait()
		events = drainStatsBuffer(port)
		if len(events) != 1 {
			t.Errorf("At 40s: expected 1 stats event (tier 2), got %d", len(events))
		} else if events[0].WifiStore == nil {
			t.Error("At 40s: expected WifiStore field in tier 2 stats")
		}

		// At 60s (tick 3, rounds=3) - tier 0: basic stats
		time.Sleep(20 * time.Second)
		synctest.Wait()
		events = drainStatsBuffer(port)
		if len(events) != 1 {
			t.Errorf("At 60s: expected 1 stats event (tier 0), got %d", len(events))
		} else {
			evt := events[0]
			if evt.Volume == nil {
				t.Error("At 60s: expected Volume field in tier 0 stats")
			}
			if evt.Brightness == nil {
				t.Error("At 60s: expected Brightness field in tier 0 stats")
			}
			if evt.Battery == nil {
				t.Error("At 60s: expected Battery field in tier 0 stats")
			}
		}

		// At 80s (tick 4, rounds=4) - case 1 but rounds%6!=1, should skip
		time.Sleep(20 * time.Second)
		synctest.Wait()
		events = drainStatsBuffer(port)
		if len(events) != 0 {
			t.Errorf("At 80s: expected 0 stats events (skipped), got %d", len(events))
		}

		// At 120s (tick 6, rounds=6) - tier 0 again
		time.Sleep(40 * time.Second)
		synctest.Wait()
		events = drainStatsBuffer(port)
		if len(events) != 1 {
			t.Errorf("At 120s: expected 1 stats event (tier 0), got %d", len(events))
		}

		// At 140s (tick 7, rounds=7) - tier 1 again (7%6=1)
		time.Sleep(20 * time.Second)
		synctest.Wait()
		events = drainStatsBuffer(port)
		if len(events) != 1 {
			t.Errorf("At 140s: expected 1 stats event (tier 1), got %d", len(events))
		} else if events[0].Shaking == nil {
			t.Error("At 140s: expected Shaking field in tier 1 stats")
		}
	})
}

func TestClientPort_StatsImmediateChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		port := NewClientPort()
		defer port.Close()

		// Initialize with batch (no immediate sends during batch)
		port.BeginBatch()
		port.SetVolume(50)
		port.SetBrightness(80)
		port.EndBatch()

		// Drain initial batch stats (should be 1 full stats)
		events := drainStatsBuffer(port)
		if len(events) != 1 {
			t.Errorf("After EndBatch: expected 1 stats event, got %d", len(events))
		}

		// Now individual Set* calls should send immediately (diff only)

		// SetVolume - should send only Volume
		port.SetVolume(60)
		synctest.Wait()
		events = drainStatsBuffer(port)
		if len(events) != 1 {
			t.Errorf("After SetVolume: expected 1 stats event, got %d", len(events))
		} else {
			evt := events[0]
			if evt.Volume == nil {
				t.Error("SetVolume should include Volume field")
			}
			if evt.Volume != nil && evt.Volume.Percentage != 60 {
				t.Errorf("Volume.Percentage = %v; want 60", evt.Volume.Percentage)
			}
			// Should NOT include other fields (diff only)
			if evt.Brightness != nil {
				t.Error("SetVolume should NOT include Brightness field (diff only)")
			}
			if evt.Battery != nil {
				t.Error("SetVolume should NOT include Battery field (diff only)")
			}
		}

		// SetBrightness - should send only Brightness
		port.SetBrightness(100)
		synctest.Wait()
		events = drainStatsBuffer(port)
		if len(events) != 1 {
			t.Errorf("After SetBrightness: expected 1 stats event, got %d", len(events))
		} else {
			evt := events[0]
			if evt.Brightness == nil {
				t.Error("SetBrightness should include Brightness field")
			}
			if evt.Brightness != nil && evt.Brightness.Percentage != 100 {
				t.Errorf("Brightness.Percentage = %v; want 100", evt.Brightness.Percentage)
			}
			// Should NOT include other fields (diff only)
			if evt.Volume != nil {
				t.Error("SetBrightness should NOT include Volume field (diff only)")
			}
		}

		// SetBattery - should send only Battery
		port.SetBattery(50, true)
		synctest.Wait()
		events = drainStatsBuffer(port)
		if len(events) != 1 {
			t.Errorf("After SetBattery: expected 1 stats event, got %d", len(events))
		} else {
			evt := events[0]
			if evt.Battery == nil {
				t.Error("SetBattery should include Battery field")
			}
			if evt.Battery != nil {
				if evt.Battery.Percentage != 50 {
					t.Errorf("Battery.Percentage = %v; want 50", evt.Battery.Percentage)
				}
				if !evt.Battery.IsCharging {
					t.Error("Battery.IsCharging = false; want true")
				}
			}
			// Should NOT include other fields (diff only)
			if evt.Volume != nil {
				t.Error("SetBattery should NOT include Volume field (diff only)")
			}
			if evt.Brightness != nil {
				t.Error("SetBattery should NOT include Brightness field (diff only)")
			}
		}
	})
}

func TestClientPort_StatsBatchMode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		port := NewClientPort()
		defer port.Close()

		// In batch mode, Set* calls should NOT send immediately
		port.BeginBatch()
		port.SetVolume(50)
		port.SetBrightness(80)
		port.SetLightMode("dark")
		port.SetBattery(75, true)
		synctest.Wait()

		// Should have no stats yet (batch mode)
		events := drainStatsBuffer(port)
		if len(events) != 0 {
			t.Errorf("During batch: expected 0 stats events, got %d", len(events))
		}

		// EndBatch should send one full stats
		port.EndBatch()
		synctest.Wait()
		events = drainStatsBuffer(port)
		if len(events) != 1 {
			t.Errorf("After EndBatch: expected 1 stats event, got %d", len(events))
		} else {
			evt := events[0]
			// Should include all fields set during batch
			if evt.Volume == nil {
				t.Error("EndBatch stats should include Volume")
			}
			if evt.Brightness == nil {
				t.Error("EndBatch stats should include Brightness")
			}
			if evt.LightMode == nil {
				t.Error("EndBatch stats should include LightMode")
			}
			if evt.Battery == nil {
				t.Error("EndBatch stats should include Battery")
			}
		}
	})
}
