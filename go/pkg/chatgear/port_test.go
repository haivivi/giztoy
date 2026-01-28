package chatgear

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/opusrt"
)

func TestClientServerPort_OpusFrames_ClientToServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create pipe connection
	serverConn, clientConn := NewPipe()

	// Create ports
	clientPort := NewClientPort(ctx, clientConn)
	serverPort := NewServerPort(ctx, serverConn)

	// Start receiving
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		clientPort.RecvFrom(clientConn)
	}()
	go func() {
		defer wg.Done()
		serverPort.RecvFrom(serverConn)
	}()

	// Register cleanup to ensure resources are released even on test failure
	t.Cleanup(func() {
		clientConn.Close()
		serverConn.Close()
		wg.Wait()
		clientPort.Close()
		serverPort.Close()
	})

	// Client sends opus frame to server
	frame := []byte{0xFC, 0x01, 0x02, 0x03} // CELT FB 20ms
	stamp := opusrt.FromTime(time.Now())
	stamped := opusrt.Stamp(frame, stamp)

	// Send via client port
	if err := clientPort.SendOpusFrames(stamped); err != nil {
		t.Fatalf("SendOpusFrames: %v", err)
	}

	// Read from server port with timeout
	// The RealtimeBuffer has a pull goroutine that processes frames asynchronously,
	// and may return loss events (nil frames) before the actual frame arrives.
	// We need to skip loss events and wait for the actual frame.
	frameCh := make(chan opusrt.Frame, 1)
	errCh := make(chan error, 1)
	go func() {
		for {
			readFrame, loss, err := serverPort.Frame()
			if err != nil {
				errCh <- err
				return
			}
			// Skip loss events (nil frames with non-zero loss duration)
			if readFrame == nil && loss > 0 {
				continue
			}
			// Skip empty frames
			if len(readFrame) == 0 {
				continue
			}
			frameCh <- readFrame
			return
		}
	}()

	select {
	case readFrame := <-frameCh:
		// Verify frame content
		if len(readFrame) != len(frame) {
			t.Errorf("frame length mismatch: got %d, want %d", len(readFrame), len(frame))
		}
	case err := <-errCh:
		t.Fatalf("Frame: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for frame")
	}
}

func TestClientServerPort_StateEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverConn, clientConn := NewPipe()

	clientPort := NewClientPort(ctx, clientConn)
	serverPort := NewServerPort(ctx, serverConn)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		clientPort.RecvFrom(clientConn)
	}()
	go func() {
		defer wg.Done()
		serverPort.RecvFrom(serverConn)
	}()

	// Client sends state event
	state := &GearStateEvent{State: GearRecording}
	if err := clientPort.SendState(state); err != nil {
		t.Fatalf("SendState: %v", err)
	}

	// Server receives state event
	select {
	case received := <-serverPort.StateEvents():
		if received.State != GearRecording {
			t.Errorf("expected GearRecording, got %v", received.State)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for state event")
	}

	// Verify GearState getter
	gearState, ok := serverPort.GearState()
	if !ok {
		t.Error("GearState() returned not ok")
	}
	if gearState.State != GearRecording {
		t.Errorf("expected GearRecording, got %v", gearState.State)
	}

	// Cleanup: close connections first to unblock RecvFrom goroutines, then wait
	clientConn.Close()
	serverConn.Close()
	wg.Wait()
	clientPort.Close()
	serverPort.Close()
}

func TestClientServerPort_StatsEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverConn, clientConn := NewPipe()

	clientPort := NewClientPort(ctx, clientConn)
	serverPort := NewServerPort(ctx, serverConn)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		clientPort.RecvFrom(clientConn)
	}()
	go func() {
		defer wg.Done()
		serverPort.RecvFrom(serverConn)
	}()

	// First stats sets the baseline (no change event emitted)
	stats1 := &GearStatsEvent{
		Volume: &Volume{Percentage: 50},
	}
	if err := clientPort.SendStats(stats1); err != nil {
		t.Fatalf("SendStats: %v", err)
	}

	// Wait for stats to be processed
	time.Sleep(50 * time.Millisecond)

	// Verify initial stats are set
	gearStats, ok := serverPort.GearStats()
	if !ok {
		t.Error("GearStats() returned not ok after first stats")
	}
	if gearStats.Volume == nil || gearStats.Volume.Percentage != 50 {
		t.Error("GearStats() did not return correct initial volume")
	}

	// Second stats triggers a change event
	stats2 := &GearStatsEvent{
		Volume: &Volume{Percentage: 75},
	}
	if err := clientPort.SendStats(stats2); err != nil {
		t.Fatalf("SendStats: %v", err)
	}

	// Server receives stats change
	select {
	case <-serverPort.StatsChanges():
		// Got stats change
		vol, ok := serverPort.Volume()
		if !ok {
			t.Error("Volume() returned not ok")
		}
		if vol != 75 {
			t.Errorf("expected volume 75, got %d", vol)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for stats change")
	}

	// Cleanup: close connections first to unblock RecvFrom goroutines, then wait
	clientConn.Close()
	serverConn.Close()
	wg.Wait()
	clientPort.Close()
	serverPort.Close()
}

func TestClientServerPort_Commands(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverConn, clientConn := NewPipe()

	clientPort := NewClientPort(ctx, clientConn)
	serverPort := NewServerPort(ctx, serverConn)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		clientPort.RecvFrom(clientConn)
	}()
	go func() {
		defer wg.Done()
		serverPort.RecvFrom(serverConn)
	}()

	// Server sends command to client
	if err := serverPort.SetVolume(50); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}

	// Client receives command
	select {
	case cmd := <-clientPort.Commands():
		if cmd == nil {
			t.Fatal("received nil command")
		}
		if cmd.Type != "set_volume" {
			t.Errorf("expected command type 'set_volume', got '%s'", cmd.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for command")
	}

	// Cleanup: close connections first to unblock RecvFrom goroutines, then wait
	clientConn.Close()
	serverConn.Close()
	wg.Wait()
	clientPort.Close()
	serverPort.Close()
}

func TestClientServerPort_Tracks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverConn, clientConn := NewPipe()

	clientPort := NewClientPort(ctx, clientConn)
	serverPort := NewServerPort(ctx, serverConn)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		clientPort.RecvFrom(clientConn)
	}()
	go func() {
		defer wg.Done()
		serverPort.RecvFrom(serverConn)
	}()

	// Create tracks
	_, bgCtrl, err := serverPort.NewBackgroundTrack()
	if err != nil {
		t.Fatalf("NewBackgroundTrack: %v", err)
	}

	_, fgCtrl, err := serverPort.NewForegroundTrack()
	if err != nil {
		t.Fatalf("NewForegroundTrack: %v", err)
	}

	_, ovCtrl, err := serverPort.NewOverlayTrack()
	if err != nil {
		t.Fatalf("NewOverlayTrack: %v", err)
	}

	// Verify tracks exist
	if serverPort.BackgroundTrackCtrl() == nil {
		t.Error("BackgroundTrackCtrl is nil")
	}
	if serverPort.ForegroundTrackCtrl() == nil {
		t.Error("ForegroundTrackCtrl is nil")
	}
	if serverPort.OverlayTrackCtrl() == nil {
		t.Error("OverlayTrackCtrl is nil")
	}

	// Close tracks manually
	bgCtrl.CloseWithError(nil)
	fgCtrl.CloseWithError(nil)
	ovCtrl.CloseWithError(nil)

	// Cleanup: close connections first to unblock RecvFrom goroutines, then wait
	clientConn.Close()
	serverConn.Close()
	wg.Wait()
	clientPort.Close()
	serverPort.Close()
}

func TestClientServerPort_Interrupt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverConn, clientConn := NewPipe()

	clientPort := NewClientPort(ctx, clientConn)
	serverPort := NewServerPort(ctx, serverConn)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		clientPort.RecvFrom(clientConn)
	}()
	go func() {
		defer wg.Done()
		serverPort.RecvFrom(serverConn)
	}()

	// Create tracks
	serverPort.NewBackgroundTrack()
	serverPort.NewForegroundTrack()
	serverPort.NewOverlayTrack()

	// Verify tracks exist
	if serverPort.BackgroundTrackCtrl() == nil {
		t.Error("BackgroundTrackCtrl is nil")
	}
	if serverPort.ForegroundTrackCtrl() == nil {
		t.Error("ForegroundTrackCtrl is nil")
	}
	if serverPort.OverlayTrackCtrl() == nil {
		t.Error("OverlayTrackCtrl is nil")
	}

	// Interrupt all tracks
	serverPort.Interrupt()

	// Verify tracks are cleared
	if serverPort.BackgroundTrackCtrl() != nil {
		t.Error("BackgroundTrackCtrl should be nil after interrupt")
	}
	if serverPort.ForegroundTrackCtrl() != nil {
		t.Error("ForegroundTrackCtrl should be nil after interrupt")
	}
	if serverPort.OverlayTrackCtrl() != nil {
		t.Error("OverlayTrackCtrl should be nil after interrupt")
	}

	// Cleanup: close connections first to unblock RecvFrom goroutines, then wait
	clientConn.Close()
	serverConn.Close()
	wg.Wait()
	clientPort.Close()
	serverPort.Close()
}

func TestClientServerPort_MultipleStatsUpdates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverConn, clientConn := NewPipe()

	clientPort := NewClientPort(ctx, clientConn)
	serverPort := NewServerPort(ctx, serverConn)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		clientPort.RecvFrom(clientConn)
	}()
	go func() {
		defer wg.Done()
		serverPort.RecvFrom(serverConn)
	}()

	// First stats sets the baseline (no change event)
	stats1 := &GearStatsEvent{
		Volume: &Volume{Percentage: 50},
	}
	if err := clientPort.SendStats(stats1); err != nil {
		t.Fatalf("SendStats: %v", err)
	}

	// Wait for stats to be processed
	time.Sleep(50 * time.Millisecond)

	// Second stats triggers a change event
	stats2 := &GearStatsEvent{
		Volume: &Volume{Percentage: 80},
	}
	if err := clientPort.SendStats(stats2); err != nil {
		t.Fatalf("SendStats: %v", err)
	}

	select {
	case <-serverPort.StatsChanges():
		vol, ok := serverPort.Volume()
		if !ok {
			t.Error("Volume() returned not ok")
		}
		if vol != 80 {
			t.Errorf("expected volume 80, got %d", vol)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for stats change")
	}

	// Third stats triggers another change event
	stats3 := &GearStatsEvent{
		Volume: &Volume{Percentage: 100},
	}
	if err := clientPort.SendStats(stats3); err != nil {
		t.Fatalf("SendStats: %v", err)
	}

	select {
	case <-serverPort.StatsChanges():
		vol, ok := serverPort.Volume()
		if !ok {
			t.Error("Volume() returned not ok")
		}
		if vol != 100 {
			t.Errorf("expected volume 100, got %d", vol)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for third stats change")
	}

	// Cleanup: close connections first to unblock RecvFrom goroutines, then wait
	clientConn.Close()
	serverConn.Close()
	wg.Wait()
	clientPort.Close()
	serverPort.Close()
}
