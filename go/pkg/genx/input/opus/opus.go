package opus

import (
	"io"

	"github.com/haivivi/giztoy/go/pkg/buffer"
	"github.com/haivivi/giztoy/go/pkg/genx"
)

// OpusReader reads sequential Opus frames (no timestamps).
type OpusReader interface {
	// ReadFrame returns the next Opus frame.
	// Returns io.EOF when the stream ends.
	ReadFrame() (OpusFrame, error)
}

// FromOpusReader creates a genx.Stream from sequential Opus frames.
// No jitter buffer, no realtime pacing - just wraps frames as MessageChunks.
//
// The returned Stream produces MessageChunks with MIMEType "audio/opus".
func FromOpusReader(reader OpusReader, role genx.Role, name string) genx.Stream {
	s := &opusStream{
		reader: reader,
		role:   role,
		name:   name,
		chunks: buffer.N[*genx.MessageChunk](256),
	}

	go s.readLoop()

	return s
}

type opusStream struct {
	reader OpusReader
	role   genx.Role
	name   string
	chunks *buffer.Buffer[*genx.MessageChunk]
}

// Next returns the next MessageChunk from the stream.
func (s *opusStream) Next() (*genx.MessageChunk, error) {
	chunk, err := s.chunks.Next()
	if err != nil {
		if err == buffer.ErrIteratorDone {
			return nil, io.EOF
		}
		return nil, err
	}
	return chunk, nil
}

// Close closes the stream.
func (s *opusStream) Close() error {
	return s.chunks.Close()
}

// CloseWithError closes the stream with an error.
func (s *opusStream) CloseWithError(err error) error {
	return s.chunks.CloseWithError(err)
}

func (s *opusStream) readLoop() {
	defer s.chunks.CloseWrite()

	for {
		frame, err := s.reader.ReadFrame()
		if err != nil {
			if err != io.EOF {
				s.chunks.CloseWithError(err)
			}
			return
		}

		chunk := &genx.MessageChunk{
			Role: s.role,
			Name: s.name,
			Part: &genx.Blob{
				MIMEType: "audio/opus",
				Data:     frame.Clone(),
			},
		}

		if err := s.chunks.Add(chunk); err != nil {
			return
		}
	}
}
