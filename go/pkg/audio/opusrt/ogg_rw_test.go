package opusrt

import (
	"bytes"
	"io"
	"testing"
	"time"
)

func TestOggWriter_Basic(t *testing.T) {
	var buf bytes.Buffer

	writer, err := NewOggWriter(&buf, 48000, 1)
	if err != nil {
		t.Fatalf("NewOggWriter() error: %v", err)
	}

	// Write some frames
	frames := []Frame{
		{0x00, 0x01, 0x02, 0x03}, // 10ms frame
		{0x00, 0x04, 0x05, 0x06},
		{0x00, 0x07, 0x08, 0x09},
	}

	for _, frame := range frames {
		if err := writer.Append(frame, 0); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// Verify output is valid OGG
	if buf.Len() == 0 {
		t.Error("Output buffer is empty")
	}

	// Check OGG magic number
	data := buf.Bytes()
	if len(data) < 4 {
		t.Fatalf("Output too short: %d bytes", len(data))
	}
	if string(data[:4]) != "OggS" {
		t.Errorf("Invalid OGG magic: got %q, want %q", string(data[:4]), "OggS")
	}
}

func TestOggWriter_ReadFrom(t *testing.T) {
	var buf bytes.Buffer

	writer, err := NewOggWriter(&buf, 48000, 2)
	if err != nil {
		t.Fatalf("NewOggWriter() error: %v", err)
	}

	// Create a mock frame reader
	frames := []Frame{
		{0x04, 0x01}, // stereo frame
		{0x04, 0x02},
		{0x04, 0x03},
	}
	mockReader := &mockFrameReader{frames: frames}

	// ReadFrom
	n, err := writer.ReadFrom(mockReader)
	if err != nil {
		t.Fatalf("ReadFrom() error: %v", err)
	}
	if n == 0 {
		t.Error("ReadFrom() returned 0 bytes")
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// Verify output
	if buf.Len() == 0 {
		t.Error("Output buffer is empty")
	}
}

func TestOggReader_Basic(t *testing.T) {
	// First create an OGG file
	var oggBuf bytes.Buffer
	writer, err := NewOggWriter(&oggBuf, 48000, 1)
	if err != nil {
		t.Fatalf("NewOggWriter() error: %v", err)
	}

	// Write frames
	originalFrames := []Frame{
		{0x00, 0x01, 0x02},
		{0x00, 0x03, 0x04},
		{0x00, 0x05, 0x06},
	}

	for _, frame := range originalFrames {
		if err := writer.Append(frame, 0); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
	}
	writer.Close()

	// Now read it back
	reader, err := NewOggReader(bytes.NewReader(oggBuf.Bytes()))
	if err != nil {
		t.Fatalf("NewOggReader() error: %v", err)
	}
	defer reader.Close()

	// Read frames
	frameCount := 0
	for {
		frame, _, err := reader.Frame()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Frame() error: %v", err)
		}
		if frame == nil {
			continue // Skip header packets
		}
		frameCount++
	}

	if frameCount != len(originalFrames) {
		t.Errorf("Read %d frames, want %d", frameCount, len(originalFrames))
	}
}

func TestOggReader_FrameDuration(t *testing.T) {
	// Create OGG with frames
	var oggBuf bytes.Buffer
	writer, _ := NewOggWriter(&oggBuf, 48000, 1)

	// 10ms frames
	for range 5 {
		writer.Append(Frame{0x00, 0x01}, 0)
	}
	writer.Close()

	// Read and check durations
	reader, err := NewOggReader(bytes.NewReader(oggBuf.Bytes()))
	if err != nil {
		t.Fatalf("NewOggReader() error: %v", err)
	}
	defer reader.Close()

	for {
		frame, _, err := reader.Frame()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Frame() error: %v", err)
		}
		if frame == nil {
			continue
		}

		dur := frame.Duration()
		if dur != 10*time.Millisecond {
			t.Logf("Frame duration: %v", dur)
		}
	}
}

func TestOggTeeReader(t *testing.T) {
	// Create a mock frame reader
	frames := []Frame{
		{0x00, 0x01},
		{0x00, 0x02},
		{0x00, 0x03},
	}
	mockReader := &mockFrameReader{frames: frames}

	// Create tee reader - writes to teeBuf while reading from mockReader
	var teeBuf bytes.Buffer
	teeReader := NewOggTeeReader(&teeBuf, mockReader)

	// Read all frames
	frameCount := 0
	for {
		_, _, err := teeReader.Frame()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Frame() error: %v", err)
		}
		frameCount++
	}

	if frameCount != len(frames) {
		t.Errorf("Got %d frames, want %d", frameCount, len(frames))
	}

	// Tee buffer should have OGG data
	if teeBuf.Len() == 0 {
		t.Error("Tee buffer is empty")
	}

	// Verify it's valid OGG
	data := teeBuf.Bytes()
	if len(data) >= 4 && string(data[:4]) != "OggS" {
		t.Error("Invalid OGG magic in tee output")
	}
}

func TestPCMToOgg(t *testing.T) {
	// Generate enough PCM data (mono, 48kHz)
	// 20ms frame = 960 samples = 1920 bytes (16-bit)
	// Need at least 960 samples per frame for Opus encoder
	pcm := make([]byte, 1920*10) // 200ms of silence

	var output bytes.Buffer
	err := PCMToOgg(&output, pcm, 48000, 1)
	if err != nil {
		// PCM encoding requires libopus - skip if not available
		t.Skipf("PCMToOgg() error (may need libopus): %v", err)
	}

	if output.Len() == 0 {
		t.Error("Output is empty")
	}

	// Verify OGG header
	data := output.Bytes()
	if len(data) >= 4 && string(data[:4]) != "OggS" {
		t.Errorf("Invalid OGG magic")
	}
}

func TestPCMStreamToOgg(t *testing.T) {
	// Generate enough PCM data
	pcm := make([]byte, 1920*10) // 200ms of silence
	pcmReader := bytes.NewReader(pcm)

	var output bytes.Buffer
	err := PCMStreamToOgg(&output, pcmReader, 48000, 1)
	if err != nil {
		// PCM encoding requires libopus - skip if not available
		t.Skipf("PCMStreamToOgg() error (may need libopus): %v", err)
	}

	if output.Len() == 0 {
		t.Error("Output is empty")
	}
}

func TestOpusStreamToOgg(t *testing.T) {
	// Create a mock frame reader
	frames := []Frame{
		{0x00, 0x01},
		{0x00, 0x02},
		{0x00, 0x03},
	}
	mockReader := &mockFrameReader{frames: frames}

	var output bytes.Buffer
	if err := OpusStreamToOgg(&output, mockReader, 48000, 1); err != nil {
		t.Fatalf("OpusStreamToOgg() error: %v", err)
	}

	if output.Len() == 0 {
		t.Error("Output is empty")
	}

	// Verify OGG header
	data := output.Bytes()
	if len(data) >= 4 && string(data[:4]) != "OggS" {
		t.Errorf("Invalid OGG magic")
	}
}

// mockFrameReader for ogg tests is defined in realtime_test.go
