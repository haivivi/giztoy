package opus

import (
	"bytes"
	"io"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/ogg"
	codecopus "github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
	"github.com/haivivi/giztoy/go/pkg/genx"
)

func TestIsOpusHeader(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{"OpusHead", []byte("OpusHead...."), true},
		{"OpusTags", []byte("OpusTags...."), true},
		{"too short", []byte("Opus"), false},
		{"empty", []byte{}, false},
		{"random data", []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, false},
		{"almost OpusHead", []byte("OpusHea"), false},
		{"OpusHead exact", []byte("OpusHead"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOpusHeader(tt.data)
			if got != tt.expected {
				t.Errorf("isOpusHeader(%q) = %v, want %v", tt.data, got, tt.expected)
			}
		})
	}
}

func TestFromOggReader_InvalidData(t *testing.T) {
	// Test with invalid OGG data
	invalidData := []byte("not valid ogg data")
	stream := FromOggReader(bytes.NewReader(invalidData), genx.RoleUser, "invalid")

	// Should get error or EOF (depending on OGG decoder behavior)
	_, err := stream.Next()
	if err == nil {
		t.Error("expected error for invalid OGG data")
	}
}

func TestFromOggReader_Empty(t *testing.T) {
	stream := FromOggReader(bytes.NewReader(nil), genx.RoleUser, "empty")

	// Should get error or EOF
	_, err := stream.Next()
	if err == nil {
		t.Error("expected error for empty reader")
	}
}

func TestOggStream_Close(t *testing.T) {
	// Create a minimal invalid OGG to trigger the decoder
	stream := FromOggReader(bytes.NewReader([]byte{0x00}), genx.RoleUser, "close-test")

	// Close immediately
	if err := stream.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// Further reads should fail
	_, err := stream.Next()
	if err == nil {
		t.Error("expected error after Close()")
	}
}

func TestOggStream_CloseWithError(t *testing.T) {
	stream := FromOggReader(bytes.NewReader([]byte{0x00}), genx.RoleUser, "close-err-test")

	// Close with error
	customErr := io.ErrUnexpectedEOF
	if err := stream.CloseWithError(customErr); err != nil {
		t.Errorf("CloseWithError() failed: %v", err)
	}

	// Further reads should fail
	_, err := stream.Next()
	if err == nil {
		t.Error("expected error after CloseWithError()")
	}
}

// createValidOgg creates a valid OGG Opus stream using ogg.OpusWriter.
func createValidOgg(frames [][]byte) []byte {
	var buf bytes.Buffer
	writer, err := ogg.NewOpusWriter(&buf, 48000, 1)
	if err != nil {
		return nil
	}

	for _, frame := range frames {
		if err := writer.Write(codecopus.Frame(frame)); err != nil {
			return nil
		}
	}

	if err := writer.Close(); err != nil {
		return nil
	}

	return buf.Bytes()
}

func TestFromOggReader_ValidOgg(t *testing.T) {
	// Create valid OGG with frames
	frames := [][]byte{
		{0x00, 0x01, 0x02}, // 10ms SILK NB frame
		{0x00, 0x03, 0x04},
		{0x00, 0x05, 0x06},
	}

	oggData := createValidOgg(frames)
	if oggData == nil {
		t.Skip("could not create valid OGG data")
	}

	stream := FromOggReader(bytes.NewReader(oggData), genx.RoleModel, "valid-ogg")
	defer stream.Close()

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

	if len(chunks) != len(frames) {
		t.Errorf("expected %d chunks, got %d", len(frames), len(chunks))
	}

	// Verify chunk content
	for i, chunk := range chunks {
		if chunk.Role != genx.RoleModel {
			t.Errorf("chunk %d: expected role %s, got %s", i, genx.RoleModel, chunk.Role)
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

func TestFromOggReader_MultipleStreams(t *testing.T) {
	// Create valid OGG with frames
	frames := [][]byte{
		{0x00, 0x01},
		{0x00, 0x02},
	}

	oggData := createValidOgg(frames)
	if oggData == nil {
		t.Skip("could not create valid OGG data")
	}

	// Concatenate two OGG streams
	doubleOgg := append(oggData, oggData...)

	stream := FromOggReader(bytes.NewReader(doubleOgg), genx.RoleUser, "multi")
	defer stream.Close()

	// Read all chunks - should handle concatenated streams
	count := 0
	for {
		_, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		count++
	}

	// Should get frames from both streams
	if count < len(frames) {
		t.Errorf("expected at least %d chunks, got %d", len(frames), count)
	}
}

func TestFromOggReader_LargeFile(t *testing.T) {
	// Create valid OGG with many frames
	frames := make([][]byte, 100)
	for i := range frames {
		frames[i] = []byte{0x00, byte(i), byte(i + 1)}
	}

	oggData := createValidOgg(frames)
	if oggData == nil {
		t.Skip("could not create valid OGG data")
	}

	stream := FromOggReader(bytes.NewReader(oggData), genx.RoleUser, "large")
	defer stream.Close()

	// Read all chunks
	count := 0
	for {
		_, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		count++
	}

	if count != len(frames) {
		t.Errorf("expected %d chunks, got %d", len(frames), count)
	}
}
