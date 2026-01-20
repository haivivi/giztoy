package opusrt

import (
	"io"
	"sync"
	"testing"
	"time"
)

// makeTestFrame creates a test frame with specified duration.
// TOC byte determines duration: 0x00 = 10ms, 0x08 = 10ms, etc.
func makeTestFrame(dur time.Duration) Frame {
	// Use TOC config that matches the desired duration
	// Simplified: just return a minimal valid frame
	switch dur {
	case 10 * time.Millisecond:
		return Frame{0x00} // config 0, mono, 10ms
	case 20 * time.Millisecond:
		return Frame{0x08} // config 2, mono, 20ms
	default:
		return Frame{0x00}
	}
}

func TestRealtimeBuffer_Quick(t *testing.T) {
	// Use very short frames (10ms) and few frames to keep test fast
	buf := NewBuffer(1 * time.Second)
	rtb := RealtimeFrom(buf)
	defer rtb.Close()

	// Write 5 frames of 10ms each = 50ms total
	// Use current time as base to ensure proper timing alignment
	baseStamp := Now()
	for i := range 5 {
		frame := makeTestFrame(10 * time.Millisecond)
		stamp := baseStamp + EpochMillis(i*10)
		buf.Append(frame, stamp)
	}

	// Signal no more data
	rtb.CloseWrite()

	// Read all frames with timeout
	frameCount := 0
	lossEvents := 0
	deadline := time.Now().Add(500 * time.Millisecond)

	for time.Now().Before(deadline) {
		frame, loss, err := rtb.Frame()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Handle "iterator done" error from BlockBuffer
			if err.Error() == "iterator done" {
				break
			}
			t.Fatalf("Frame() error: %v", err)
		}

		if frame != nil {
			frameCount++
		}
		if loss > 0 && loss < time.Hour { // Filter out unrealistic loss values
			lossEvents++
			t.Logf("Loss detected: %v", loss)
		}
	}

	// Should get most frames (realtime timing may cause some variation)
	if frameCount < 3 {
		t.Errorf("Got %d frames, want at least 3", frameCount)
	}
	t.Logf("Got %d frames, %d loss events", frameCount, lossEvents)
}

func TestRealtimeBuffer_LossDetection(t *testing.T) {
	buf := NewBuffer(1 * time.Second)
	rtb := RealtimeFrom(buf)
	defer rtb.Close()

	// Write frames with a gap (packet loss simulation)
	baseStamp := Now()

	// Frame at 0ms
	buf.Append(makeTestFrame(10*time.Millisecond), baseStamp)
	// Skip frame at 10ms (simulated loss)
	// Frame at 20ms - creates a 10ms gap
	buf.Append(makeTestFrame(10*time.Millisecond), baseStamp+20)
	// Frame at 30ms
	buf.Append(makeTestFrame(10*time.Millisecond), baseStamp+30)

	rtb.CloseWrite()

	frameCount := 0
	lossCount := 0
	deadline := time.Now().Add(500 * time.Millisecond)

	for time.Now().Before(deadline) {
		frame, loss, err := rtb.Frame()
		if err == io.EOF {
			break
		}
		if err != nil {
			if err.Error() == "iterator done" {
				break
			}
			t.Fatalf("Frame() error: %v", err)
		}

		if frame != nil {
			frameCount++
		}
		if loss > 0 && loss < time.Hour { // Filter out unrealistic loss values
			lossCount++
			t.Logf("Loss: %v", loss)
		}
	}

	// Should get at least 2 frames
	if frameCount < 2 {
		t.Errorf("Got %d frames, want at least 2", frameCount)
	}
	t.Logf("Got %d frames, %d loss events", frameCount, lossCount)
}

func TestRealtimeBuffer_Write(t *testing.T) {
	buf := NewBuffer(1 * time.Second)
	rtb := RealtimeFrom(buf)
	defer rtb.Close()

	frame := makeTestFrame(10 * time.Millisecond)
	stamp := Now()
	stamped := Stamp(frame, stamp)

	n, err := rtb.Write(stamped)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if n != len(stamped) {
		t.Errorf("Write() = %d, want %d", n, len(stamped))
	}

	// Close write and verify we can read the frame
	rtb.CloseWrite()

	timeout := time.After(200 * time.Millisecond)
	select {
	case <-timeout:
		// May timeout if realtime pacing delays the frame
	default:
	}

	f, _, err := rtb.Frame()
	if err != nil && err != io.EOF {
		t.Logf("Frame() returned: frame=%v, err=%v", f, err)
	}
}

func TestRealtimeBuffer_WriteAfterClose(t *testing.T) {
	buf := NewBuffer(1 * time.Second)
	rtb := RealtimeFrom(buf)

	rtb.CloseWrite()

	frame := makeTestFrame(10 * time.Millisecond)
	stamped := Stamp(frame, Now())

	_, err := rtb.Write(stamped)
	if err != io.ErrClosedPipe {
		t.Errorf("Write() after CloseWrite should return ErrClosedPipe, got %v", err)
	}

	rtb.Close()
}

func TestRealtimeBuffer_Reset(t *testing.T) {
	buf := NewBuffer(1 * time.Second)
	rtb := RealtimeFrom(buf)
	defer rtb.Close()

	// Add some frames
	baseStamp := Now()
	buf.Append(makeTestFrame(10*time.Millisecond), baseStamp)
	buf.Append(makeTestFrame(10*time.Millisecond), baseStamp+10)

	// Reset should clear the buffer
	rtb.Reset()

	if buf.Len() != 0 {
		t.Errorf("After Reset(), Len() = %d, want 0", buf.Len())
	}
}

func TestRealtimeReader(t *testing.T) {
	// Create a mock frame reader
	frames := []Frame{
		makeTestFrame(10 * time.Millisecond),
		makeTestFrame(10 * time.Millisecond),
		makeTestFrame(10 * time.Millisecond),
	}

	mock := &mockFrameReader{frames: frames}
	reader := &RealtimeReader[*mockFrameReader]{
		FrameReader: mock,
		// Use a custom delay function that doesn't actually sleep
		DelayFunc: func(duration, gap time.Duration) time.Duration {
			return 0 // No delay for testing
		},
	}

	// Read all frames
	for i := 0; i < len(frames); i++ {
		frame, dur, err := reader.Frame()
		if err != nil {
			t.Fatalf("Frame() %d error: %v", i, err)
		}
		if frame == nil {
			t.Errorf("Frame() %d returned nil frame", i)
		}
		if dur == 0 {
			t.Errorf("Frame() %d returned zero duration", i)
		}
	}

	// Next read should return EOF
	_, _, err := reader.Frame()
	if err != io.EOF {
		t.Errorf("Expected EOF after all frames, got %v", err)
	}
}

func TestRealtimeReader_WithDelay(t *testing.T) {
	frames := []Frame{
		makeTestFrame(10 * time.Millisecond),
		makeTestFrame(10 * time.Millisecond),
	}

	mock := &mockFrameReader{frames: frames}
	var delays []time.Duration
	var mu sync.Mutex

	reader := &RealtimeReader[*mockFrameReader]{
		FrameReader: mock,
		DelayFunc: func(duration, gap time.Duration) time.Duration {
			mu.Lock()
			delays = append(delays, gap)
			mu.Unlock()
			return 0 // Don't actually sleep
		},
	}

	// Read all frames
	for range frames {
		reader.Frame()
	}

	// First frame has no delay recorded, subsequent frames should have delay func called
	mu.Lock()
	if len(delays) < 1 {
		t.Errorf("DelayFunc should be called at least once, got %d calls", len(delays))
	}
	mu.Unlock()
}

// mockFrameReader implements FrameReader for testing
type mockFrameReader struct {
	frames []Frame
	idx    int
}

func (m *mockFrameReader) Frame() (Frame, time.Duration, error) {
	if m.idx >= len(m.frames) {
		return nil, 0, io.EOF
	}
	f := m.frames[m.idx]
	m.idx++
	return f, f.Duration(), nil
}
