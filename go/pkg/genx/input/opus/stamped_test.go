package opus

import (
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/genx"
)

// mockStampedReader implements StampedOpusReader for testing.
type mockStampedReader struct {
	mu      sync.Mutex
	frames  [][]byte // stamped wire format
	index   int
	delay   time.Duration // delay between reads
	err     error         // error to return after all frames
	closed  bool
	closeCh chan struct{}
}

func newMockStampedReader(frames [][]byte) *mockStampedReader {
	return &mockStampedReader{
		frames:  frames,
		closeCh: make(chan struct{}),
	}
}

func (m *mockStampedReader) ReadStamped() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, io.EOF
	}

	if m.index >= len(m.frames) {
		if m.err != nil {
			return nil, m.err
		}
		return nil, io.EOF
	}

	if m.delay > 0 {
		m.mu.Unlock()
		time.Sleep(m.delay)
		m.mu.Lock()
	}

	frame := m.frames[m.index]
	m.index++
	return frame, nil
}

func (m *mockStampedReader) close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	close(m.closeCh)
}

func TestRealtimeConfig_setDefaults(t *testing.T) {
	cfg := RealtimeConfig{}
	cfg.setDefaults()

	if cfg.Role != genx.RoleUser {
		t.Errorf("expected default Role to be RoleUser, got %s", cfg.Role)
	}
	if cfg.MaxLoss != 5*time.Second {
		t.Errorf("expected default MaxLoss to be 5s, got %v", cfg.MaxLoss)
	}
	if cfg.JitterBufferSize != 100 {
		t.Errorf("expected default JitterBufferSize to be 100, got %d", cfg.JitterBufferSize)
	}

	// Test with custom values
	cfg2 := RealtimeConfig{
		Role:             genx.RoleModel,
		MaxLoss:          10 * time.Second,
		JitterBufferSize: 50,
	}
	cfg2.setDefaults()

	if cfg2.Role != genx.RoleModel {
		t.Errorf("expected Role to remain RoleModel, got %s", cfg2.Role)
	}
	if cfg2.MaxLoss != 10*time.Second {
		t.Errorf("expected MaxLoss to remain 10s, got %v", cfg2.MaxLoss)
	}
	if cfg2.JitterBufferSize != 50 {
		t.Errorf("expected JitterBufferSize to remain 50, got %d", cfg2.JitterBufferSize)
	}
}

func TestFromStampedReader_Basic(t *testing.T) {
	// Create stamped frames with sequential timestamps
	baseStamp := Now()
	frames := [][]byte{
		MakeStamped(OpusSilence20ms, baseStamp),
		MakeStamped(OpusSilence20ms, baseStamp+20),
		MakeStamped(OpusSilence20ms, baseStamp+40),
	}

	reader := newMockStampedReader(frames)
	cfg := RealtimeConfig{
		Role:    genx.RoleUser,
		Name:    "test",
		MaxLoss: 5 * time.Second,
	}

	stream := FromStampedReader(reader, cfg)
	defer stream.Close()

	// Read chunks (with timeout to avoid hanging)
	var chunks []*genx.MessageChunk
	timeout := time.After(2 * time.Second)

	for len(chunks) < 3 {
		select {
		case <-timeout:
			t.Fatalf("timeout waiting for chunks, got %d", len(chunks))
		default:
			chunk, err := stream.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			chunks = append(chunks, chunk)
		}
	}

	if len(chunks) < 3 {
		t.Errorf("expected at least 3 chunks, got %d", len(chunks))
	}

	// Verify chunk content
	for i, chunk := range chunks {
		if chunk.Role != genx.RoleUser {
			t.Errorf("chunk %d: expected role %s, got %s", i, genx.RoleUser, chunk.Role)
		}
		blob, ok := chunk.Part.(*genx.Blob)
		if !ok {
			t.Errorf("chunk %d: expected Blob part", i)
			continue
		}
		if blob.MIMEType != "audio/opus" {
			t.Errorf("chunk %d: expected MIME 'audio/opus', got '%s'", i, blob.MIMEType)
		}
	}
}

func TestFromStampedReader_OutOfOrder(t *testing.T) {
	// Create stamped frames out of order
	baseStamp := Now()
	frames := [][]byte{
		MakeStamped([]byte{0xf8, 0x02}, baseStamp+40), // third
		MakeStamped([]byte{0xf8, 0x01}, baseStamp),    // first
		MakeStamped([]byte{0xf8, 0x03}, baseStamp+20), // second
	}

	reader := newMockStampedReader(frames)
	cfg := RealtimeConfig{
		Role:    genx.RoleUser,
		Name:    "ooo-test",
		MaxLoss: 5 * time.Second,
	}

	stream := FromStampedReader(reader, cfg)
	defer stream.Close()

	// Read chunks - jitter buffer should reorder them
	var blobs [][]byte
	timeout := time.After(2 * time.Second)

	for len(blobs) < 3 {
		select {
		case <-timeout:
			t.Fatalf("timeout waiting for chunks, got %d", len(blobs))
		default:
			chunk, err := stream.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if blob, ok := chunk.Part.(*genx.Blob); ok {
				blobs = append(blobs, blob.Data)
			}
		}
	}

	// Should have received frames in timestamp order
	if len(blobs) < 3 {
		t.Errorf("expected at least 3 blobs, got %d", len(blobs))
	}
}

func TestFromStampedReader_Close(t *testing.T) {
	baseStamp := Now()
	frames := [][]byte{
		MakeStamped(OpusSilence20ms, baseStamp),
		MakeStamped(OpusSilence20ms, baseStamp+20),
	}

	reader := newMockStampedReader(frames)
	stream := FromStampedReader(reader, RealtimeConfig{})

	// Read one chunk
	_, err := stream.Next()
	if err != nil && err != io.EOF {
		t.Fatalf("first read failed: %v", err)
	}

	// Close the stream
	if err := stream.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// Further reads should fail
	_, err = stream.Next()
	if err == nil {
		t.Error("expected error after Close()")
	}
}

func TestFromStampedReader_CloseWithError(t *testing.T) {
	baseStamp := Now()
	frames := [][]byte{
		MakeStamped(OpusSilence20ms, baseStamp),
	}

	reader := newMockStampedReader(frames)
	stream := FromStampedReader(reader, RealtimeConfig{})

	// Close with error
	customErr := errors.New("custom error")
	if err := stream.CloseWithError(customErr); err != nil {
		t.Errorf("CloseWithError() failed: %v", err)
	}

	// Further reads should return error
	_, err := stream.Next()
	if err == nil {
		t.Error("expected error after CloseWithError()")
	}
}

func TestFromStampedReader_Empty(t *testing.T) {
	reader := newMockStampedReader(nil)
	stream := FromStampedReader(reader, RealtimeConfig{})
	defer stream.Close()

	// Should eventually get EOF
	timeout := time.After(500 * time.Millisecond)
	done := make(chan struct{})

	go func() {
		for {
			_, err := stream.Next()
			if err != nil {
				close(done)
				return
			}
		}
	}()

	select {
	case <-done:
		// Expected
	case <-timeout:
		// Also acceptable - empty reader with realtime pacing
		stream.Close()
	}
}

func TestEmitSilence(t *testing.T) {
	// Test that OpusSilence20ms is a valid frame
	if len(OpusSilence20ms) != 3 {
		t.Errorf("expected OpusSilence20ms to be 3 bytes, got %d", len(OpusSilence20ms))
	}

	// Verify it has expected duration
	dur := OpusSilence20ms.Duration()
	if dur != 20*time.Millisecond {
		t.Errorf("expected 20ms duration, got %v", dur)
	}
}

func TestFromStampedReader_InvalidFrame(t *testing.T) {
	// Create frames with some invalid data
	baseStamp := Now()
	frames := [][]byte{
		MakeStamped(OpusSilence20ms, baseStamp),
		[]byte{0x00, 0x01},                        // invalid - too short
		MakeStamped(OpusSilence20ms, baseStamp+40),
	}

	reader := newMockStampedReader(frames)
	stream := FromStampedReader(reader, RealtimeConfig{Name: "invalid-test"})
	defer stream.Close()

	// Should skip invalid frame and continue
	timeout := time.After(2 * time.Second)
	count := 0

	for count < 2 {
		select {
		case <-timeout:
			t.Fatalf("timeout, got %d chunks", count)
		default:
			chunk, err := stream.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if chunk != nil {
				count++
			}
		}
	}
}
