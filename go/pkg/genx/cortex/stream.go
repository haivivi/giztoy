package cortex

import (
	"io"
	"sync"
	"sync/atomic"

	"github.com/haivivi/giztoy/go/pkg/genx"
)

// inputStream implements genx.Stream for user audio input.
// It collects PCM audio data and emits it as MessageChunks.
type inputStream struct {
	streamID string
	ch       chan *genx.MessageChunk
	closed   atomic.Bool

	mu        sync.Mutex
	committed bool
	closeErr  error
}

// newInputStream creates a new input stream.
func newInputStream(streamID string) *inputStream {
	s := &inputStream{
		streamID: streamID,
		ch:       make(chan *genx.MessageChunk, 256),
	}

	// Send BOS marker
	s.ch <- &genx.MessageChunk{
		Role: genx.RoleUser,
		Ctrl: &genx.StreamCtrl{
			StreamID:      streamID,
			BeginOfStream: true,
		},
	}

	return s
}

// Write writes PCM audio data to the stream.
func (s *inputStream) Write(pcmData []byte) error {
	if s.closed.Load() {
		return io.ErrClosedPipe
	}

	s.mu.Lock()
	if s.committed {
		s.mu.Unlock()
		return nil // Ignore writes after commit
	}
	s.mu.Unlock()

	chunk := &genx.MessageChunk{
		Role: genx.RoleUser,
		Part: &genx.Blob{
			MIMEType: "audio/pcm;rate=16000",
			Data:     pcmData,
		},
		Ctrl: &genx.StreamCtrl{
			StreamID: s.streamID,
		},
	}

	select {
	case s.ch <- chunk:
		return nil
	default:
		// Channel full, drop frame
		return nil
	}
}

// Commit signals end of user speech.
// After commit, Write calls are ignored and EOS is sent.
func (s *inputStream) Commit() {
	s.mu.Lock()
	if s.committed {
		s.mu.Unlock()
		return
	}
	s.committed = true
	s.mu.Unlock()

	// Send EOS marker
	s.ch <- &genx.MessageChunk{
		Role: genx.RoleUser,
		Part: &genx.Blob{
			MIMEType: "audio/pcm;rate=16000",
		},
		Ctrl: &genx.StreamCtrl{
			StreamID:    s.streamID,
			EndOfStream: true,
		},
	}
}

// Next implements genx.Stream.
func (s *inputStream) Next() (*genx.MessageChunk, error) {
	s.mu.Lock()
	err := s.closeErr
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}

	chunk, ok := <-s.ch
	if !ok {
		return nil, io.EOF
	}
	return chunk, nil
}

// Close implements genx.Stream.
func (s *inputStream) Close() error {
	return s.CloseWithError(io.EOF)
}

// CloseWithError implements genx.Stream.
func (s *inputStream) CloseWithError(err error) error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}

	s.mu.Lock()
	s.closeErr = err
	s.mu.Unlock()

	close(s.ch)
	return nil
}
