package transformers

import (
	"context"
	"fmt"
	"io"

	"github.com/haivivi/giztoy/go/pkg/buffer"
	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/trie"
)

var _ genx.Transformer = (*Mux)(nil)

// DefaultMux is the default transformer multiplexer.
var DefaultMux = NewMux()

// Handle registers a transformer for the given pattern to the default mux.
func Handle(pattern string, t genx.Transformer) error {
	return DefaultMux.Handle(pattern, t)
}

// Transform applies the transformer registered for the pattern using the default mux.
func Transform(ctx context.Context, pattern string, input genx.Stream) (genx.Stream, error) {
	return DefaultMux.Transform(ctx, pattern, input)
}

// Mux is a transformer multiplexer that routes requests to registered transformers
// based on pattern matching using a trie.
type Mux struct {
	mux *trie.Trie[genx.Transformer]
}

// NewMux creates a new transformer multiplexer.
func NewMux() *Mux {
	return &Mux{
		mux: trie.New[genx.Transformer](),
	}
}

// Handle registers a transformer for the given pattern.
func (m *Mux) Handle(pattern string, t genx.Transformer) error {
	return m.mux.Set(pattern, func(ptr *genx.Transformer, existed bool) error {
		if existed {
			return fmt.Errorf("transformers: transformer already registered for %s", pattern)
		}
		*ptr = t
		return nil
	})
}

// Transform implements genx.Transformer for Mux.
// It routes to the transformer registered for the given pattern.
func (m *Mux) Transform(ctx context.Context, pattern string, input genx.Stream) (genx.Stream, error) {
	t, err := m.get(pattern)
	if err != nil {
		return nil, err
	}
	return t.Transform(ctx, pattern, input)
}

func (m *Mux) get(pattern string) (genx.Transformer, error) {
	ptr, ok := m.mux.Get(pattern)
	if !ok {
		return nil, fmt.Errorf("transformers: transformer not found for %s", pattern)
	}
	t := *ptr
	if t == nil {
		return nil, fmt.Errorf("transformers: transformer not found for %s", pattern)
	}
	return t, nil
}

// errorStream is a Stream that always returns an error.
type errorStream struct {
	err error
}

func (s *errorStream) Next() (*genx.MessageChunk, error) {
	return nil, s.err
}

func (s *errorStream) Close() error {
	return nil
}

func (s *errorStream) CloseWithError(err error) error {
	return nil
}

// bufferStream wraps a buffer.Buffer as a genx.Stream.
type bufferStream struct {
	buf      *buffer.Buffer[*genx.MessageChunk]
	closed   bool
	closeErr error
}

func newBufferStream(size int) *bufferStream {
	return &bufferStream{
		buf: buffer.N[*genx.MessageChunk](size),
	}
}

func (s *bufferStream) Next() (*genx.MessageChunk, error) {
	chunk, err := s.buf.Next()
	if err != nil {
		if err == buffer.ErrIteratorDone {
			return nil, io.EOF
		}
		return nil, err
	}
	return chunk, nil
}

func (s *bufferStream) Close() error {
	if !s.closed {
		s.closed = true
		s.buf.CloseWrite()
	}
	return nil
}

func (s *bufferStream) CloseWithError(err error) error {
	if !s.closed {
		s.closed = true
		s.closeErr = err
		s.buf.CloseWithError(err)
	}
	return nil
}

func (s *bufferStream) Push(chunk *genx.MessageChunk) error {
	return s.buf.Add(chunk)
}

// streamToReader converts a genx.Stream of Text chunks to an io.Reader.
// It starts a goroutine to read from the stream and write to a pipe.
// The goroutine lifetime is governed by the stream (exits on EOF/error).
func streamToReader(stream genx.Stream) io.Reader {
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		for {
			chunk, err := stream.Next()
			if err != nil {
				if err != io.EOF {
					pw.CloseWithError(err)
				}
				return
			}

			if chunk == nil {
				continue
			}

			// Only handle Text parts
			if text, ok := chunk.Part.(genx.Text); ok {
				if _, err := pw.Write([]byte(text)); err != nil {
					return
				}
			}
		}
	}()

	return pr
}
