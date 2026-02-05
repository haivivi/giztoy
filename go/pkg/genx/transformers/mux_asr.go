package transformers

import (
	"context"
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/trie"
)

// ASRMux is the default ASR transformer multiplexer.
var ASRMux = NewASRMux()

// HandleASR registers an ASR transformer for the given pattern to the default mux.
func HandleASR(pattern string, t genx.Transformer) error {
	return ASRMux.Handle(pattern, t)
}

// ASR is a multiplexer for ASR transformers. It routes recognition requests
// to the appropriate registered transformer based on a model name pattern.
//
// Usage:
//
//	// Register a transformer
//	transformers.HandleASR("doubao-sauc", NewDoubaoASRSAUC(client))
//
//	// Create an ASR stream
//	asr := transformers.ASRMux.Create(ctx, "doubao-sauc")
//	asr.Send(audioData) // Send audio chunks
//	asr.Close()          // Signal end of audio
//	for chunk := range asr.Output() { ... } // Receive text chunks
type ASR struct {
	mux *trie.Trie[genx.Transformer]
}

// NewASRMux creates a new ASR transformer multiplexer.
func NewASRMux() *ASR {
	return &ASR{
		mux: trie.New[genx.Transformer](),
	}
}

// Handle registers an ASR transformer for the given pattern.
func (m *ASR) Handle(pattern string, t genx.Transformer) error {
	return m.mux.Set(pattern, func(ptr *genx.Transformer, existed bool) error {
		if existed {
			return fmt.Errorf("asr: transformer already registered for %s", pattern)
		}
		*ptr = t
		return nil
	})
}

// Create creates a new ASR session for the given model pattern.
// Returns an ASRSession that can be used to send audio and receive text.
func (m *ASR) Create(ctx context.Context, pattern string) (*ASRSession, error) {
	ptr, ok := m.mux.Get(pattern)
	if !ok {
		return nil, fmt.Errorf("asr: transformer not found for %s", pattern)
	}
	t := *ptr
	if t == nil {
		return nil, fmt.Errorf("asr: transformer not found for %s", pattern)
	}

	// Create input stream for audio
	inputStream := newBufferStream(100)

	// Start the transformer
	outputStream, err := t.Transform(ctx, pattern, inputStream)
	if err != nil {
		inputStream.Close()
		return nil, fmt.Errorf("asr: transform failed: %w", err)
	}

	return &ASRSession{
		input:  inputStream,
		output: outputStream,
	}, nil
}

// ASRSession represents an active ASR session.
// Audio data is sent via Send(), and text results are received via Output().
type ASRSession struct {
	input  *bufferStream
	output genx.Stream
}

// Send sends audio data to the ASR session.
// The mimeType should be like "audio/opus", "audio/ogg", "audio/pcm", etc.
func (s *ASRSession) Send(data []byte, mimeType string) error {
	chunk := &genx.MessageChunk{
		Part: &genx.Blob{
			MIMEType: mimeType,
			Data:     data,
		},
	}
	return s.input.Push(chunk)
}

// Close signals the end of audio input.
// This should be called after all audio has been sent.
func (s *ASRSession) Close() error {
	// Send EOS marker
	eosChunk := genx.NewEndOfStream("audio/opus")
	if err := s.input.Push(eosChunk); err != nil {
		return err
	}
	return s.input.Close()
}

// Output returns the output stream for receiving text chunks.
// The stream will emit Text chunks with recognized text.
func (s *ASRSession) Output() genx.Stream {
	return s.output
}

// CloseAll closes both input and output streams.
func (s *ASRSession) CloseAll() error {
	s.input.Close()
	if s.output != nil {
		return s.output.Close()
	}
	return nil
}
