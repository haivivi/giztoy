package resampler

import "io"

// sampleReader wraps an io.Reader and ensures each Read returns a multiple of
// sampleSize bytes. It buffers partial data internally until a complete sample
// can be returned.
type sampleReader struct {
	// holds leftover bytes (up to sampleSize-1)
	buffer []byte

	// number of valid bytes in buffer
	buffered int

	sampleSize int

	r io.Reader
}

// newSampleReader creates a sampleReader that returns data in multiples of
// sampleSize bytes.
func newSampleReader(r io.Reader, sampleSize int) *sampleReader {
	return &sampleReader{
		buffer:     make([]byte, sampleSize-1), // max remainder is sampleSize-1
		buffered:   0,
		sampleSize: sampleSize,
		r:          r,
	}
}

// Read reads data into p, returning a 0 or a multiple of sampleSize bytes.
// Returns io.ErrShortBuffer if len(p) < sampleSize. On EOF, may return
// remaining data that is not aligned to sampleSize.
func (sr *sampleReader) Read(p []byte) (n int, err error) {
	if len(p) < sr.sampleSize {
		return 0, io.ErrShortBuffer
	}

	// Truncate p to a multiple of sampleSize
	p = p[:len(p)/sr.sampleSize*sr.sampleSize]
	if sr.buffered > 0 {
		n = copy(p, sr.buffer[:sr.buffered])
		sr.buffered = 0
	}

	rn, err := sr.r.Read(p[n:])
	n += rn
	if err != nil {
		if n%sr.sampleSize != 0 && err == io.EOF {
			return n, io.ErrUnexpectedEOF
		}
		return n, err
	}
	if mod := n % sr.sampleSize; mod != 0 {
		// Save unaligned remainder for next call
		n -= mod
		copy(sr.buffer[:mod], p[n:n+mod])
		sr.buffered = mod
	}
	return n, nil
}
