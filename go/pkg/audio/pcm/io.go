package pcm

import (
	"errors"
	"io"
	"time"
)

// Writer is a writer for chunks of audio data.
type Writer interface {
	Write(Chunk) error
}

var _ Writer = WriteFunc(nil)

// WriteFunc is a function that implements the Writer interface.
type WriteFunc func(Chunk) error

// Write implements the Writer interface.
func (f WriteFunc) Write(c Chunk) error {
	return f(c)
}

// WriteCloser is a writer for chunks of audio data that also implements io.Closer.
type WriteCloser interface {
	Writer
	io.Closer
}

// Discard is a Writer that discards all written chunks.
var Discard Writer = discard{}

type discard struct{}

func (discard) Write(Chunk) error {
	return nil
}

// IOWriter wraps a pcm.Writer to provide an io.Writer interface.
// All bytes written are converted to DataChunks with the specified format.
func IOWriter(w Writer, f Format) io.Writer {
	return &ioWriter{w: w, f: f}
}

type ioWriter struct {
	w Writer
	f Format
}

func (w *ioWriter) Write(b []byte) (int, error) {
	err := w.w.Write(w.f.DataChunk(b))
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

// ChunkWriter wraps an io.Writer to provide a pcm.Writer interface.
// All chunks are written to the underlying writer using WriteTo.
func ChunkWriter(w io.Writer) Writer {
	return &chunkWriter{w: w}
}

type chunkWriter struct {
	w io.Writer
}

func (w *chunkWriter) Write(c Chunk) error {
	_, err := c.WriteTo(w.w)
	return err
}

// Copy copies audio data from reader r to writer w using the specified format.
// It reads data in chunks of at least 20ms duration and writes them as DataChunks.
// Returns nil on EOF, or any other error encountered during reading or writing.
func Copy(w Writer, r io.Reader, format Format) error {
	minChunk := int(format.BytesInDuration(20 * time.Millisecond))
	buf := make([]byte, 10*minChunk)
	for {
		n, err := io.ReadAtLeast(r, buf, minChunk)
		if n > 0 {
			if err := w.Write(format.DataChunk(buf[:n])); err != nil {
				return err
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return err
		}
	}
}
