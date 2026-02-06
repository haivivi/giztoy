package transformers

import (
	"context"
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/trie"
)

// TTSMux is the default TTS transformer multiplexer.
var TTSMux = NewTTSMux()

// HandleTTS registers a TTS transformer for the given pattern to the default mux.
func HandleTTS(pattern string, t genx.Transformer) error {
	return TTSMux.Handle(pattern, t)
}

// TTS is a multiplexer for TTS transformers. It routes synthesis requests
// to the appropriate registered transformer based on a model name pattern.
//
// Usage:
//
//	// Register a transformer
//	transformers.HandleTTS("doubao-v2", NewDoubaoTTSSeedV2(client, "zh_female_cancan"))
//
//	// Create a TTS stream
//	stream, err := transformers.TTSMux.Synthesize(ctx, "doubao-v2", "Hello world")
//	for chunk := range stream { ... } // Receive audio chunks
type TTS struct {
	mux *trie.Trie[genx.Transformer]
}

// NewTTSMux creates a new TTS transformer multiplexer.
func NewTTSMux() *TTS {
	return &TTS{
		mux: trie.New[genx.Transformer](),
	}
}

// Handle registers a TTS transformer for the given pattern.
func (m *TTS) Handle(pattern string, t genx.Transformer) error {
	return m.mux.Set(pattern, func(ptr *genx.Transformer, existed bool) error {
		if existed {
			return fmt.Errorf("tts: transformer already registered for %s", pattern)
		}
		*ptr = t
		return nil
	})
}

// Synthesize creates a TTS stream for the given model pattern and text.
// Returns a genx.Stream that emits audio Blob chunks.
func (m *TTS) Synthesize(ctx context.Context, pattern string, text string) (genx.Stream, error) {
	ptr, ok := m.mux.Get(pattern)
	if !ok {
		return nil, fmt.Errorf("tts: transformer not found for %s", pattern)
	}
	t := *ptr
	if t == nil {
		return nil, fmt.Errorf("tts: transformer not found for %s", pattern)
	}

	// Create input stream with text
	inputStream := newBufferStream(10)

	// Send the text as a single chunk
	textChunk := &genx.MessageChunk{
		Part: genx.Text(text),
	}
	if err := inputStream.Push(textChunk); err != nil {
		inputStream.Close()
		return nil, fmt.Errorf("tts: push text failed: %w", err)
	}

	// Send text EOS to signal end of input
	eosChunk := genx.NewTextEndOfStream()
	if err := inputStream.Push(eosChunk); err != nil {
		inputStream.Close()
		return nil, fmt.Errorf("tts: push eos failed: %w", err)
	}

	// Close input stream
	inputStream.Close()

	// Start the transformer
	outputStream, err := t.Transform(ctx, pattern, inputStream)
	if err != nil {
		return nil, fmt.Errorf("tts: transform failed: %w", err)
	}

	return outputStream, nil
}

// SynthesizeStream creates a TTS session for streaming text input.
// Returns a TTSSession that can be used to send text and receive audio.
func (m *TTS) SynthesizeStream(ctx context.Context, pattern string) (*TTSSession, error) {
	ptr, ok := m.mux.Get(pattern)
	if !ok {
		return nil, fmt.Errorf("tts: transformer not found for %s", pattern)
	}
	t := *ptr
	if t == nil {
		return nil, fmt.Errorf("tts: transformer not found for %s", pattern)
	}

	// Create input stream for text
	inputStream := newBufferStream(100)

	// Start the transformer
	outputStream, err := t.Transform(ctx, pattern, inputStream)
	if err != nil {
		inputStream.Close()
		return nil, fmt.Errorf("tts: transform failed: %w", err)
	}

	return &TTSSession{
		input:  inputStream,
		output: outputStream,
	}, nil
}

// TTSSession represents an active TTS session.
// Text data is sent via Send(), and audio results are received via Output().
type TTSSession struct {
	input  *bufferStream
	output genx.Stream
}

// Send sends text to the TTS session.
func (s *TTSSession) Send(text string) error {
	chunk := &genx.MessageChunk{
		Part: genx.Text(text),
	}
	return s.input.Push(chunk)
}

// Close signals the end of text input.
// This should be called after all text has been sent.
func (s *TTSSession) Close() error {
	// Send text EOS marker
	eosChunk := genx.NewTextEndOfStream()
	if err := s.input.Push(eosChunk); err != nil {
		return err
	}
	return s.input.Close()
}

// Output returns the output stream for receiving audio chunks.
// The stream will emit Blob chunks with synthesized audio.
func (s *TTSSession) Output() genx.Stream {
	return s.output
}

// CloseAll closes both input and output streams.
func (s *TTSSession) CloseAll() error {
	s.input.Close()
	if s.output != nil {
		return s.output.Close()
	}
	return nil
}
