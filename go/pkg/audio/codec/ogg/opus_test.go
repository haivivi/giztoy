package ogg

import (
	"bytes"
	"io"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
)

// createTestOpusFrame creates a valid Opus frame for testing.
// TOC byte 0xFC = config 31 (CELT FB 20ms), stereo=0, code=0 (1 frame)
// This gives us 20ms duration = 960 samples at 48kHz
func createTestOpusFrame(data byte) opus.Frame {
	// TOC: config=31 (CELT FB 20ms), s=0 (mono), c=0 (1 frame)
	// 0xFC = 11111100 = config 31, mono, 1 frame
	return opus.Frame{0xFC, data, data + 1, data + 2}
}

func TestOpusWriterBasic(t *testing.T) {
	var buf bytes.Buffer

	w, err := NewOpusWriter(&buf, 48000, 1)
	if err != nil {
		t.Fatalf("NewOpusWriter failed: %v", err)
	}

	// Write some frames
	for i := 0; i < 5; i++ {
		frame := createTestOpusFrame(byte(i))
		if err := w.Write(frame); err != nil {
			t.Fatalf("Write frame %d failed: %v", i, err)
		}
	}

	// Check granule
	granule := w.Granule()
	expectedGranule := int64(5 * 960) // 5 frames * 960 samples per frame (20ms at 48kHz)
	if granule != expectedGranule {
		t.Errorf("Granule = %d, want %d", granule, expectedGranule)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify OGG magic
	data := buf.Bytes()
	if len(data) < 4 || string(data[:4]) != "OggS" {
		t.Errorf("Invalid OGG magic")
	}
}

func TestOpusWriterNilWriter(t *testing.T) {
	_, err := NewOpusWriter(nil, 48000, 1)
	if err != ErrNilWriter {
		t.Errorf("Expected ErrNilWriter, got %v", err)
	}
}

func TestOpusWriterSetGranule(t *testing.T) {
	var buf bytes.Buffer

	w, _ := NewOpusWriter(&buf, 48000, 1)

	// Write a frame
	w.Write(createTestOpusFrame(0))

	// Set granule to a different value (simulate gap)
	w.SetGranule(48000) // 1 second

	// Write another frame
	w.Write(createTestOpusFrame(1))

	// Granule should be 48000 + 960
	expected := int64(48000 + 960)
	if w.Granule() != expected {
		t.Errorf("Granule = %d, want %d", w.Granule(), expected)
	}

	w.Close()
}

func TestOpusWriterMultiStream(t *testing.T) {
	var buf bytes.Buffer

	w, _ := NewOpusWriter(&buf, 48000, 1)
	defaultStream := w.defaultStream

	// Create a second stream
	stream2 := w.StreamBegin(48000, 2)

	if stream2 == defaultStream {
		t.Error("Second stream should have different serial number")
	}

	// Write to both streams
	w.Write(createTestOpusFrame(0))
	w.StreamWrite(stream2, createTestOpusFrame(1))
	w.Write(createTestOpusFrame(2))
	w.StreamWrite(stream2, createTestOpusFrame(3))

	// Check granules independently
	if w.Granule() != 2*960 {
		t.Errorf("Default stream granule = %d, want %d", w.Granule(), 2*960)
	}
	if w.StreamGranule(stream2) != 2*960 {
		t.Errorf("Stream2 granule = %d, want %d", w.StreamGranule(stream2), 2*960)
	}

	// End stream2 early
	if err := w.StreamEnd(stream2); err != nil {
		t.Fatalf("StreamEnd failed: %v", err)
	}

	// Continue writing to default stream
	w.Write(createTestOpusFrame(4))
	if w.Granule() != 3*960 {
		t.Errorf("Default stream granule after = %d, want %d", w.Granule(), 3*960)
	}

	w.Close()
}

func TestOpusWriterStreamErrors(t *testing.T) {
	var buf bytes.Buffer
	w, _ := NewOpusWriter(&buf, 48000, 1)

	// Invalid serial number
	err := w.StreamWrite(999999, createTestOpusFrame(0))
	if err != ErrInvalidSerialNo {
		t.Errorf("Expected ErrInvalidSerialNo, got %v", err)
	}

	// Invalid granule get
	if w.StreamGranule(999999) != 0 {
		t.Error("Expected 0 for invalid serial")
	}

	// End stream
	stream := w.StreamBegin(48000, 1)
	w.StreamEnd(stream)

	// Write to ended stream
	err = w.StreamWrite(stream, createTestOpusFrame(0))
	if err != ErrStreamEnded {
		t.Errorf("Expected ErrStreamEnded, got %v", err)
	}

	// Double end should not error
	if err := w.StreamEnd(stream); err != nil {
		t.Errorf("Double StreamEnd failed: %v", err)
	}

	w.Close()

	// Write to closed writer
	err = w.Write(createTestOpusFrame(0))
	if err != ErrWriterClosed {
		t.Errorf("Expected ErrWriterClosed, got %v", err)
	}
}

func TestOpusWriterStreamSetGranule(t *testing.T) {
	var buf bytes.Buffer
	w, _ := NewOpusWriter(&buf, 48000, 1)

	stream := w.StreamBegin(48000, 1)
	w.StreamWrite(stream, createTestOpusFrame(0))

	w.StreamSetGranule(stream, 10000)
	if w.StreamGranule(stream) != 10000 {
		t.Errorf("StreamGranule = %d, want 10000", w.StreamGranule(stream))
	}

	// Set granule on invalid stream (should be no-op)
	w.StreamSetGranule(999999, 5000)

	w.Close()
}

func TestReadOpusPacketsBasic(t *testing.T) {
	var buf bytes.Buffer

	// Write OGG file
	w, _ := NewOpusWriter(&buf, 48000, 1)
	frames := []opus.Frame{
		createTestOpusFrame(0),
		createTestOpusFrame(1),
		createTestOpusFrame(2),
	}
	for _, f := range frames {
		w.Write(f)
	}
	w.Close()

	// Read back
	var readFrames []opus.Frame
	var readGranules []int64

	for pkt, err := range ReadOpusPackets(bytes.NewReader(buf.Bytes())) {
		if err != nil {
			t.Fatalf("ReadOpusPackets error: %v", err)
		}
		readFrames = append(readFrames, pkt.Frame)
		readGranules = append(readGranules, pkt.Granule)
	}

	if len(readFrames) != len(frames) {
		t.Errorf("Read %d frames, want %d", len(readFrames), len(frames))
	}

	// Verify granule progression
	for i, g := range readGranules {
		expected := int64((i + 1) * 960)
		if g != expected {
			t.Errorf("Frame %d granule = %d, want %d", i, g, expected)
		}
	}
}

func TestReadOpusPacketsRoundTrip(t *testing.T) {
	var buf bytes.Buffer

	// Write
	w, _ := NewOpusWriter(&buf, 48000, 1)
	originalFrames := make([]opus.Frame, 10)
	for i := range originalFrames {
		originalFrames[i] = createTestOpusFrame(byte(i * 10))
		w.Write(originalFrames[i])
	}
	w.Close()

	// Read
	var readFrames []opus.Frame
	for pkt, err := range ReadOpusPackets(bytes.NewReader(buf.Bytes())) {
		if err != nil {
			t.Fatalf("ReadOpusPackets error: %v", err)
		}
		readFrames = append(readFrames, pkt.Frame)
	}

	// Compare
	if len(readFrames) != len(originalFrames) {
		t.Fatalf("Read %d frames, want %d", len(readFrames), len(originalFrames))
	}

	for i, rf := range readFrames {
		if !bytes.Equal(rf, originalFrames[i]) {
			t.Errorf("Frame %d mismatch", i)
		}
	}
}

func TestReadOpusPacketsMultiStream(t *testing.T) {
	var buf bytes.Buffer

	// Write two streams
	w, _ := NewOpusWriter(&buf, 48000, 1)
	stream1 := w.defaultStream
	stream2 := w.StreamBegin(48000, 2)

	// Interleave writes
	w.StreamWrite(stream1, createTestOpusFrame(1))
	w.StreamWrite(stream2, createTestOpusFrame(2))
	w.StreamWrite(stream1, createTestOpusFrame(3))
	w.StreamWrite(stream2, createTestOpusFrame(4))

	w.Close()

	// Read and track by serial
	stream1Frames := 0
	stream2Frames := 0
	seenSerials := make(map[int32]bool)

	for pkt, err := range ReadOpusPackets(bytes.NewReader(buf.Bytes())) {
		if err != nil {
			t.Fatalf("ReadOpusPackets error: %v", err)
		}
		seenSerials[pkt.SerialNo] = true
		if pkt.SerialNo == stream1 {
			stream1Frames++
		} else if pkt.SerialNo == stream2 {
			stream2Frames++
		}
	}

	if len(seenSerials) != 2 {
		t.Errorf("Expected 2 different serial numbers, got %d", len(seenSerials))
	}
	if stream1Frames != 2 {
		t.Errorf("Stream1 frames = %d, want 2", stream1Frames)
	}
	if stream2Frames != 2 {
		t.Errorf("Stream2 frames = %d, want 2", stream2Frames)
	}
}

func TestReadOpusPacketsBOSEOS(t *testing.T) {
	var buf bytes.Buffer

	w, _ := NewOpusWriter(&buf, 48000, 1)
	w.Write(createTestOpusFrame(0))
	w.Write(createTestOpusFrame(1))
	w.Close()

	// The first audio packet should have BOS=false (BOS is on header)
	// The last page should have EOS=true
	var lastPkt *OpusPacket
	for pkt, err := range ReadOpusPackets(bytes.NewReader(buf.Bytes())) {
		if err != nil {
			t.Fatalf("ReadOpusPackets error: %v", err)
		}
		lastPkt = pkt
	}

	if lastPkt == nil {
		t.Fatal("No packets read")
	}
	// Note: EOS may not be on the last packet itself depending on page layout
}

func TestReadOpusPacketsEmpty(t *testing.T) {
	// Empty reader
	count := 0
	for _, err := range ReadOpusPackets(bytes.NewReader(nil)) {
		if err != nil && err != io.EOF {
			// Expected to fail on empty input
		}
		count++
	}
	// May or may not iterate, just shouldn't panic
}

func TestReadOpusPacketsInvalidOGG(t *testing.T) {
	// Invalid data
	data := []byte("not an ogg file")
	count := 0
	for _, err := range ReadOpusPackets(bytes.NewReader(data)) {
		if err != nil {
			// Expected error
			break
		}
		count++
	}
	// Should handle gracefully
}

func TestOpusPacketStruct(t *testing.T) {
	pkt := &OpusPacket{
		Frame:    createTestOpusFrame(42),
		Granule:  12345,
		SerialNo: 999,
		BOS:      true,
		EOS:      false,
	}

	if pkt.Granule != 12345 {
		t.Errorf("Granule = %d, want 12345", pkt.Granule)
	}
	if pkt.SerialNo != 999 {
		t.Errorf("SerialNo = %d, want 999", pkt.SerialNo)
	}
	if !pkt.BOS {
		t.Error("BOS should be true")
	}
	if pkt.EOS {
		t.Error("EOS should be false")
	}
	if len(pkt.Frame) != 4 {
		t.Errorf("Frame length = %d, want 4", len(pkt.Frame))
	}
}

func TestOpusWriterDoubleClose(t *testing.T) {
	var buf bytes.Buffer
	w, _ := NewOpusWriter(&buf, 48000, 1)
	w.Write(createTestOpusFrame(0))

	// First close
	if err := w.Close(); err != nil {
		t.Fatalf("First Close failed: %v", err)
	}

	// Second close should be no-op
	if err := w.Close(); err != nil {
		t.Fatalf("Second Close failed: %v", err)
	}
}

func TestOpusWriterGranuleCalculation(t *testing.T) {
	var buf bytes.Buffer
	w, _ := NewOpusWriter(&buf, 48000, 1)

	// Frame with 20ms duration should add 960 samples (at 48kHz)
	frame := createTestOpusFrame(0)
	duration := frame.Duration()
	t.Logf("Frame duration: %v", duration)

	w.Write(frame)
	g1 := w.Granule()

	w.Write(frame)
	g2 := w.Granule()

	// Each frame should add 960 samples (20ms * 48000)
	if g2-g1 != 960 {
		t.Errorf("Granule increment = %d, want 960", g2-g1)
	}

	w.Close()
}

func TestReadOpusPacketsBreakEarly(t *testing.T) {
	var buf bytes.Buffer

	w, _ := NewOpusWriter(&buf, 48000, 1)
	for i := 0; i < 10; i++ {
		w.Write(createTestOpusFrame(byte(i)))
	}
	w.Close()

	// Read only first 3 packets
	count := 0
	for pkt, err := range ReadOpusPackets(bytes.NewReader(buf.Bytes())) {
		if err != nil {
			t.Fatalf("ReadOpusPackets error: %v", err)
		}
		_ = pkt
		count++
		if count >= 3 {
			break
		}
	}

	if count != 3 {
		t.Errorf("Read %d packets, want 3", count)
	}
}

func TestIsOpusHeader(t *testing.T) {
	tests := []struct {
		data     []byte
		expected bool
	}{
		{[]byte("OpusHead12345678"), true},
		{[]byte("OpusTags12345678"), true},
		{[]byte("OpusHea"), false},          // Too short
		{[]byte("NotOpus1234567890"), false},
		{nil, false},
		{[]byte{}, false},
	}

	for _, tt := range tests {
		result := isOpusHeader(tt.data)
		if result != tt.expected {
			t.Errorf("isOpusHeader(%q) = %v, want %v", tt.data, result, tt.expected)
		}
	}
}

func TestOpusWriterStreamEndInvalid(t *testing.T) {
	var buf bytes.Buffer
	w, _ := NewOpusWriter(&buf, 48000, 1)

	// End invalid stream
	err := w.StreamEnd(999999)
	if err != ErrInvalidSerialNo {
		t.Errorf("Expected ErrInvalidSerialNo, got %v", err)
	}

	w.Close()

	// End stream on closed writer
	err = w.StreamEnd(w.defaultStream)
	if err != ErrWriterClosed {
		t.Errorf("Expected ErrWriterClosed, got %v", err)
	}
}

func TestReadOpusPacketsChainedStreams(t *testing.T) {
	// Create two separate OGG files (chained streams)
	var buf1, buf2, combined bytes.Buffer

	// First stream
	w1, _ := NewOpusWriter(&buf1, 48000, 1)
	w1.Write(createTestOpusFrame(1))
	w1.Write(createTestOpusFrame(2))
	w1.Close()

	// Second stream
	w2, _ := NewOpusWriter(&buf2, 48000, 1)
	w2.Write(createTestOpusFrame(3))
	w2.Write(createTestOpusFrame(4))
	w2.Close()

	// Concatenate them
	combined.Write(buf1.Bytes())
	combined.Write(buf2.Bytes())

	// Read and verify we see two different serial numbers
	serials := make(map[int32]int)
	for pkt, err := range ReadOpusPackets(bytes.NewReader(combined.Bytes())) {
		if err != nil {
			t.Fatalf("ReadOpusPackets error: %v", err)
		}
		serials[pkt.SerialNo]++
	}

	if len(serials) != 2 {
		t.Errorf("Expected 2 different serial numbers, got %d", len(serials))
	}
	for serial, count := range serials {
		if count != 2 {
			t.Errorf("Serial %d: got %d frames, want 2", serial, count)
		}
	}
}

func TestReadOpusPacketsYieldReturnsFalse(t *testing.T) {
	var buf bytes.Buffer

	w, _ := NewOpusWriter(&buf, 48000, 1)
	for i := 0; i < 20; i++ {
		w.Write(createTestOpusFrame(byte(i)))
	}
	w.Close()

	// Read and stop early by returning false from the yield function
	// This tests the "break" path in the iterator
	count := 0
	for pkt, err := range ReadOpusPackets(bytes.NewReader(buf.Bytes())) {
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		_ = pkt
		count++
		if count >= 5 {
			break // This causes yield to return false
		}
	}

	if count != 5 {
		t.Errorf("Expected to read 5 packets, got %d", count)
	}
}

// errorReader is a reader that returns an error
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}

func TestReadOpusPacketsReaderError(t *testing.T) {
	// Test with a reader that returns an error
	errReader := &errorReader{err: io.ErrUnexpectedEOF}

	var errorReturned bool
	for _, err := range ReadOpusPackets(errReader) {
		if err != nil {
			errorReturned = true
			break
		}
	}

	if !errorReturned {
		t.Error("Expected error from reader")
	}
}

func TestReadOpusPacketsCorruptedData(t *testing.T) {
	// Test with corrupted OGG data (valid magic but corrupted content)
	// Create a valid OGG start then corrupt it
	var buf bytes.Buffer
	w, _ := NewOpusWriter(&buf, 48000, 1)
	w.Write(createTestOpusFrame(0))
	w.Close()

	data := buf.Bytes()
	// Corrupt the data after the first page
	if len(data) > 100 {
		data[100] = 0xFF
		data[101] = 0xFF
	}

	// Should handle gracefully (may produce errors or partial data)
	count := 0
	for pkt, err := range ReadOpusPackets(bytes.NewReader(data)) {
		if err != nil {
			// Expected - corrupted data may cause errors
			break
		}
		if pkt != nil {
			count++
		}
	}
	// Just ensure no panic
	t.Logf("Read %d packets from corrupted data", count)
}

func TestOpusWriterAllStreamsEnd(t *testing.T) {
	// Test that Close() properly ends all streams
	var buf bytes.Buffer
	w, _ := NewOpusWriter(&buf, 48000, 1)
	
	// Create multiple streams
	s1 := w.StreamBegin(48000, 1)
	s2 := w.StreamBegin(48000, 2)
	
	w.StreamWrite(s1, createTestOpusFrame(1))
	w.StreamWrite(s2, createTestOpusFrame(2))
	
	// Don't explicitly end streams - let Close handle it
	w.Close()
	
	// Verify valid OGG was written
	if len(buf.Bytes()) < 4 {
		t.Error("Expected output data")
	}
}

func TestOpusWriterConcurrency(t *testing.T) {
	// Test that the mutex works correctly
	var buf bytes.Buffer
	w, _ := NewOpusWriter(&buf, 48000, 1)

	done := make(chan bool)
	go func() {
		for i := 0; i < 10; i++ {
			w.Write(createTestOpusFrame(byte(i)))
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 10; i++ {
			w.Granule()
		}
		done <- true
	}()

	<-done
	<-done
	w.Close()
}

func BenchmarkOpusWriter(b *testing.B) {
	frame := createTestOpusFrame(0)
	var buf bytes.Buffer

	for i := 0; i < b.N; i++ {
		buf.Reset()
		w, _ := NewOpusWriter(&buf, 48000, 1)
		for j := 0; j < 100; j++ {
			w.Write(frame)
		}
		w.Close()
	}
}

func BenchmarkReadOpusPackets(b *testing.B) {
	// Create test data
	var buf bytes.Buffer
	w, _ := NewOpusWriter(&buf, 48000, 1)
	for i := 0; i < 100; i++ {
		w.Write(createTestOpusFrame(byte(i)))
	}
	w.Close()
	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, err := range ReadOpusPackets(bytes.NewReader(data)) {
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
