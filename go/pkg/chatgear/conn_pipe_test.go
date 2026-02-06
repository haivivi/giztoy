package chatgear

import (
	"sync"
	"testing"
	"time"
)

func TestNewPipe(t *testing.T) {
	server, client := NewPipe()
	if server == nil {
		t.Fatal("NewPipe returned nil server")
	}
	if client == nil {
		t.Fatal("NewPipe returned nil client")
	}

	// Close both ends
	server.Close()
	client.Close()
}

func TestPipe_OpusFrames(t *testing.T) {
	server, client := NewPipe()
	defer server.Close()
	defer client.Close()

	// Send frame from client
	frame := []byte{1, 2, 3, 4}
	stamp := time.Now()
	err := client.SendOpusFrame(stamp, frame)
	if err != nil {
		t.Fatalf("SendOpusFrame: %v", err)
	}

	// Receive on server
	var received StampedOpusFrame
	for f, err := range server.OpusFrames() {
		if err != nil {
			t.Fatalf("OpusFrames: %v", err)
		}
		received = f
		break
	}

	if received.Timestamp.Unix() != stamp.Unix() {
		t.Errorf("Timestamp mismatch: got %v, want %v", received.Timestamp, stamp)
	}
	if len(received.Frame) != len(frame) {
		t.Errorf("Frame length: got %d, want %d", len(received.Frame), len(frame))
	}
}

func TestPipe_States(t *testing.T) {
	server, client := NewPipe()
	defer server.Close()
	defer client.Close()

	// Send state from client
	state := &StateEvent{
		Version: 1,
		State:   StateReady,
	}
	err := client.SendState(state)
	if err != nil {
		t.Fatalf("SendState: %v", err)
	}

	// Receive on server
	var received *StateEvent
	for s, err := range server.States() {
		if err != nil {
			t.Fatalf("States: %v", err)
		}
		received = s
		break
	}

	if received.State != StateReady {
		t.Errorf("State: got %v, want StateReady", received.State)
	}
}

func TestPipe_Stats(t *testing.T) {
	server, client := NewPipe()
	defer server.Close()
	defer client.Close()

	// Send stats from client
	stats := &StatsEvent{
		Volume: &Volume{Percentage: 50},
	}
	err := client.SendStats(stats)
	if err != nil {
		t.Fatalf("SendStats: %v", err)
	}

	// Receive on server
	var received *StatsEvent
	for s, err := range server.Stats() {
		if err != nil {
			t.Fatalf("Stats: %v", err)
		}
		received = s
		break
	}

	if received.Volume == nil || received.Volume.Percentage != 50 {
		t.Errorf("Volume: got %v, want 50", received.Volume)
	}
}

func TestPipe_Commands(t *testing.T) {
	server, client := NewPipe()
	defer server.Close()
	defer client.Close()

	// Send command from server
	cmd := NewSetVolume(75)
	err := server.IssueCommand(cmd, time.Now())
	if err != nil {
		t.Fatalf("IssueCommand: %v", err)
	}

	// Receive on client
	var received *CommandEvent
	for c, err := range client.Commands() {
		if err != nil {
			t.Fatalf("Commands: %v", err)
		}
		received = c
		break
	}

	if received.Type != "set_volume" {
		t.Errorf("Command type: got %q, want set_volume", received.Type)
	}
}

func TestPipe_ServerToClientOpus(t *testing.T) {
	server, client := NewPipe()
	defer server.Close()
	defer client.Close()

	// Send frame from server
	frame := []byte{5, 6, 7, 8}
	stamp := time.Now()
	err := server.SendOpusFrame(stamp, frame)
	if err != nil {
		t.Fatalf("SendOpusFrame: %v", err)
	}

	// Receive on client
	var received StampedOpusFrame
	for f, err := range client.OpusFrames() {
		if err != nil {
			t.Fatalf("OpusFrames: %v", err)
		}
		received = f
		break
	}

	if len(received.Frame) != len(frame) {
		t.Errorf("Frame length: got %d, want %d", len(received.Frame), len(frame))
	}
}

func TestPipe_LatestStats(t *testing.T) {
	server, client := NewPipe()
	defer server.Close()
	defer client.Close()

	// Initially nil
	if server.LatestStats() != nil {
		t.Error("LatestStats should be nil initially")
	}

	// Send stats
	stats := &StatsEvent{
		Battery: &Battery{Percentage: 80},
	}
	client.SendStats(stats)

	// Read to update cache
	for s, _ := range server.Stats() {
		_ = s
		break
	}

	// Now should be set
	latest := server.LatestStats()
	if latest == nil {
		t.Fatal("LatestStats should not be nil after reading")
	}
	if latest.Battery == nil || latest.Battery.Percentage != 80 {
		t.Errorf("Battery: got %v, want 80", latest.Battery)
	}
}

func TestPipe_CloseWithError(t *testing.T) {
	server, client := NewPipe()

	// Close server with error
	server.CloseWithError(nil)

	// Client close should also work
	client.CloseWithError(nil)
}

func TestPipe_Bidirectional(t *testing.T) {
	server, client := NewPipe()
	defer server.Close()
	defer client.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// Client sends audio to server
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			client.SendOpusFrame(time.Now(), []byte{byte(i)})
		}
	}()

	// Server sends commands to client
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			server.IssueCommand(NewSetVolume(i*10), time.Now())
		}
	}()

	wg.Wait()
}

func TestPipe_SendAfterClose(t *testing.T) {
	server, client := NewPipe()

	// Close first
	server.Close()
	client.Close()

	// Send after close - should not error (returns nil)
	err := client.SendOpusFrame(time.Now(), []byte{1, 2, 3})
	if err != nil {
		t.Errorf("SendOpusFrame after close: %v", err)
	}

	err = client.SendState(&StateEvent{State: StateReady})
	if err != nil {
		t.Errorf("SendState after close: %v", err)
	}

	err = client.SendStats(&StatsEvent{})
	if err != nil {
		t.Errorf("SendStats after close: %v", err)
	}

	err = server.SendOpusFrame(time.Now(), []byte{1, 2, 3})
	if err != nil {
		t.Errorf("Server SendOpusFrame after close: %v", err)
	}

	err = server.IssueCommand(NewSetVolume(50), time.Now())
	if err != nil {
		t.Errorf("IssueCommand after close: %v", err)
	}
}

func TestPipe_ChannelFull(t *testing.T) {
	server, client := NewPipe()
	defer server.Close()
	defer client.Close()

	// Fill up channels without reading (channel size is 10)
	for i := 0; i < 20; i++ {
		// Should not block - drops when full
		client.SendOpusFrame(time.Now(), []byte{byte(i)})
		client.SendState(&StateEvent{State: StateReady})
		client.SendStats(&StatsEvent{})
		server.SendOpusFrame(time.Now(), []byte{byte(i)})
		server.IssueCommand(NewSetVolume(i), time.Now())
	}
	// If we get here without blocking, the default case is working
}
