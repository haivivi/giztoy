package resampler

import (
	"bytes"
	"io"
	"testing"
)

func TestSampleReader_ExactMultiple(t *testing.T) {
	// Input is exactly a multiple of sample size
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	r := newSampleReader(bytes.NewReader(data), 4)

	buf := make([]byte, 8)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if n != 8 {
		t.Fatalf("Read returned %d, want 8", n)
	}
	if !bytes.Equal(buf[:n], data) {
		t.Fatalf("Read got %v, want %v", buf[:n], data)
	}
}

func TestSampleReader_PartialSample(t *testing.T) {
	// Input is not a multiple of sample size
	data := []byte{1, 2, 3, 4, 5, 6} // 6 bytes, sample size 4
	r := newSampleReader(bytes.NewReader(data), 4)

	buf := make([]byte, 8)

	// First read should return 4 bytes (one complete sample)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("First Read error: %v", err)
	}
	if n != 4 {
		t.Fatalf("First Read returned %d, want 4", n)
	}
	if !bytes.Equal(buf[:n], []byte{1, 2, 3, 4}) {
		t.Fatalf("First Read got %v, want [1,2,3,4]", buf[:n])
	}

	// Second read should return remaining 2 bytes with EOF
	n, err = r.Read(buf)
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("Second Read error = %v, want io.ErrUnexpectedEOF", err)
	}
	if n != 2 {
		t.Fatalf("Second Read returned %d, want 2", n)
	}
}

func TestSampleReader_ShortBuffer(t *testing.T) {
	data := []byte{1, 2, 3, 4}
	r := newSampleReader(bytes.NewReader(data), 4)

	// Buffer smaller than sample size
	buf := make([]byte, 2)
	_, err := r.Read(buf)
	if err != io.ErrShortBuffer {
		t.Fatalf("Read error = %v, want io.ErrShortBuffer", err)
	}
}

func TestSampleReader_BufferNotMultiple(t *testing.T) {
	// Buffer that's not a multiple of sample size should be truncated
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	r := newSampleReader(bytes.NewReader(data), 4)

	// Buffer is 6 bytes, should be truncated to 4 (1 sample)
	buf := make([]byte, 6)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if n != 4 {
		t.Fatalf("Read returned %d, want 4", n)
	}
}

func TestSampleReader_MultipleReads(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	r := newSampleReader(bytes.NewReader(data), 4)

	// Read in chunks
	buf := make([]byte, 4)

	// First read
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("First Read error: %v", err)
	}
	if n != 4 || !bytes.Equal(buf[:n], []byte{1, 2, 3, 4}) {
		t.Fatalf("First Read got %d bytes: %v", n, buf[:n])
	}

	// Second read
	n, err = r.Read(buf)
	if err != nil {
		t.Fatalf("Second Read error: %v", err)
	}
	if n != 4 || !bytes.Equal(buf[:n], []byte{5, 6, 7, 8}) {
		t.Fatalf("Second Read got %d bytes: %v", n, buf[:n])
	}

	// Third read
	n, err = r.Read(buf)
	if err != nil {
		t.Fatalf("Third Read error: %v", err)
	}
	if n != 4 || !bytes.Equal(buf[:n], []byte{9, 10, 11, 12}) {
		t.Fatalf("Third Read got %d bytes: %v", n, buf[:n])
	}

	// Fourth read should EOF
	_, err = r.Read(buf)
	if err != io.EOF {
		t.Fatalf("Fourth Read error = %v, want io.EOF", err)
	}
}

func TestSampleReader_SampleSize2(t *testing.T) {
	// Common case: mono 16-bit audio (2 bytes per sample)
	data := []byte{1, 2, 3, 4, 5, 6}
	r := newSampleReader(bytes.NewReader(data), 2)

	buf := make([]byte, 6)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if n != 6 {
		t.Fatalf("Read returned %d, want 6", n)
	}
}

func TestSampleReader_EmptyReader(t *testing.T) {
	r := newSampleReader(bytes.NewReader(nil), 4)

	buf := make([]byte, 8)
	n, err := r.Read(buf)
	if err != io.EOF {
		t.Fatalf("Read error = %v, want io.EOF", err)
	}
	if n != 0 {
		t.Fatalf("Read returned %d, want 0", n)
	}
}

func TestSampleReader_BufferedData(t *testing.T) {
	// Test that buffered data is correctly prepended to next read
	// Create a reader that returns data in chunks that don't align with sample size
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	chunkedReader := &chunkedReader{
		data:      data,
		chunkSize: 5, // Returns 5 bytes at a time, sampleSize is 4
	}
	r := newSampleReader(chunkedReader, 4)

	buf := make([]byte, 8)

	// First read:
	// - underlying returns [1,2,3,4,5] (5 bytes)
	// - 5 % 4 = 1, so 1 byte is buffered
	// - returns 4 bytes [1,2,3,4]
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("First Read error: %v", err)
	}
	if n != 4 {
		t.Fatalf("First Read returned %d, want 4", n)
	}
	if !bytes.Equal(buf[:n], []byte{1, 2, 3, 4}) {
		t.Fatalf("First Read got %v, want [1,2,3,4]", buf[:n])
	}

	// Second read:
	// - buffer has [5] (1 byte)
	// - underlying returns [6,7,8] + EOF (3 bytes)
	// - total is 4 bytes [5,6,7,8]
	n, err = r.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Second Read error: %v", err)
	}
	if n != 4 {
		t.Fatalf("Second Read returned %d, want 4", n)
	}
	if !bytes.Equal(buf[:n], []byte{5, 6, 7, 8}) {
		t.Fatalf("Second Read got %v, want [5,6,7,8]", buf[:n])
	}
}

// chunkedReader returns data in fixed-size chunks
type chunkedReader struct {
	data      []byte
	pos       int
	chunkSize int
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}

	end := r.pos + r.chunkSize
	if end > len(r.data) {
		end = len(r.data)
	}
	if end > r.pos+len(p) {
		end = r.pos + len(p)
	}

	n := copy(p, r.data[r.pos:end])
	r.pos += n

	if r.pos >= len(r.data) {
		return n, io.EOF
	}
	return n, nil
}

func TestNewSampleReader(t *testing.T) {
	data := []byte{1, 2, 3, 4}
	underlying := bytes.NewReader(data)

	r := newSampleReader(underlying, 2)
	if r == nil {
		t.Fatal("newSampleReader returned nil")
	}
	if r.sampleSize != 2 {
		t.Fatalf("sampleSize = %d, want 2", r.sampleSize)
	}
	if r.r != underlying {
		t.Fatal("underlying reader not set correctly")
	}
	if len(r.buffer) != 1 { // sampleSize - 1
		t.Fatalf("buffer size = %d, want 1", len(r.buffer))
	}
	if r.buffered != 0 {
		t.Fatalf("buffered = %d, want 0", r.buffered)
	}
}
