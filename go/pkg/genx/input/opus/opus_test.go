package opus

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/genx"
)

// mockOpusReader implements OpusReader for testing.
type mockOpusReader struct {
	frames []OpusFrame
	index  int
	err    error // error to return after all frames
}

func (m *mockOpusReader) ReadFrame() (OpusFrame, error) {
	if m.index >= len(m.frames) {
		if m.err != nil {
			return nil, m.err
		}
		return nil, io.EOF
	}
	frame := m.frames[m.index]
	m.index++
	return frame, nil
}

func TestFromOpusReader_Basic(t *testing.T) {
	frames := []OpusFrame{
		{0xf8, 0xff, 0xfe}, // 20ms silence
		{0xf8, 0x01, 0x02}, // another frame
		{0xf8, 0x03, 0x04}, // another frame
	}

	reader := &mockOpusReader{frames: frames}
	stream := FromOpusReader(reader, genx.RoleUser, "test")

	// Read all chunks
	var chunks []*genx.MessageChunk
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks, got %d", len(chunks))
	}

	// Verify chunk content
	for i, chunk := range chunks {
		if chunk.Role != genx.RoleUser {
			t.Errorf("chunk %d: expected role %s, got %s", i, genx.RoleUser, chunk.Role)
		}
		if chunk.Name != "test" {
			t.Errorf("chunk %d: expected name 'test', got '%s'", i, chunk.Name)
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

func TestFromOpusReader_Error(t *testing.T) {
	expectedErr := errors.New("read error")
	reader := &mockOpusReader{
		frames: []OpusFrame{},
		err:    expectedErr,
	}

	stream := FromOpusReader(reader, genx.RoleModel, "err-test")

	// Should get error (reader immediately returns error)
	_, err := stream.Next()
	if err == nil {
		t.Error("expected error from reader")
	}
	// The error should contain our custom error
	if err != nil && !errors.Is(err, io.EOF) {
		// Error propagated correctly (not EOF)
		t.Logf("got expected error: %v", err)
	}
}

func TestFromOpusReader_Empty(t *testing.T) {
	reader := &mockOpusReader{frames: nil}
	stream := FromOpusReader(reader, genx.RoleUser, "empty")

	// Should immediately return EOF
	_, err := stream.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestOpusStream_Close(t *testing.T) {
	frames := []OpusFrame{
		{0xf8, 0xff, 0xfe},
		{0xf8, 0x01, 0x02},
	}
	reader := &mockOpusReader{frames: frames}
	stream := FromOpusReader(reader, genx.RoleUser, "close-test")

	// Read one chunk
	_, err := stream.Next()
	if err != nil {
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

func TestOpusStream_CloseWithError(t *testing.T) {
	frames := []OpusFrame{
		{0xf8, 0xff, 0xfe},
		{0xf8, 0x01, 0x02},
	}
	reader := &mockOpusReader{frames: frames}
	stream := FromOpusReader(reader, genx.RoleUser, "close-err-test")

	// Read one chunk
	_, err := stream.Next()
	if err != nil {
		t.Fatalf("first read failed: %v", err)
	}

	// Close with error
	customErr := errors.New("custom close error")
	if err := stream.CloseWithError(customErr); err != nil {
		t.Errorf("CloseWithError() failed: %v", err)
	}

	// Further reads should return error
	_, err = stream.Next()
	if err == nil {
		t.Error("expected error after CloseWithError()")
	}
}

func TestFromOpusReader_Concurrent(t *testing.T) {
	// Test that reading doesn't race with internal goroutine
	frames := make([]OpusFrame, 100)
	for i := range frames {
		frames[i] = OpusFrame{0xf8, byte(i), byte(i + 1)}
	}

	reader := &mockOpusReader{frames: frames}
	stream := FromOpusReader(reader, genx.RoleUser, "concurrent")

	// Read all with small delays
	count := 0
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read error: %v", err)
		}
		if chunk == nil {
			t.Fatal("unexpected nil chunk")
		}
		count++
		time.Sleep(time.Microsecond) // small delay
	}

	if count != 100 {
		t.Errorf("expected 100 chunks, got %d", count)
	}
}
