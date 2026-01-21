package speech

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/haivivi/giztoy/pkg/audio/pcm"
	"github.com/haivivi/giztoy/pkg/trie"
)

// TTSMux is the default multiplexer for TTS synthesizers.
var TTSMux = NewTTSMux()

// HandleTTS registers a synthesizer for the given pattern with the default mux.
func HandleTTS(pattern string, synthesizer Synthesizer) error {
	return TTSMux.Handle(pattern, synthesizer)
}

// HandleTTSFunc registers a synthesizer function for the given pattern with the default mux.
func HandleTTSFunc(pattern string, f SynthesizeFunc) error {
	return TTSMux.HandleFunc(pattern, f)
}

// Synthesize performs text-to-speech synthesis using the default multiplexer.
func Synthesize(ctx context.Context, name string, textStream io.Reader, format pcm.Format) (Speech, error) {
	return TTSMux.Synthesize(ctx, name, textStream, format)
}

// Revoice synthesizes new speech audio by passing the decoded audio of the
// input speech through the specified TTS synthesizer via the default
// multiplexer.
func Revoice(ctx context.Context, name string, speech Speech, format pcm.Format) (Speech, error) {
	pr, pw := io.Pipe()
	go func() {
		if _, err := CopySpeech(pcm.Discard, pw, speech); err != nil {
			pw.CloseWithError(err)
		} else {
			pw.Close()
		}
	}()
	return Synthesize(ctx, name, pr, format)
}

// SentenceIterator is an interface for iterating over sentences.
type SentenceIterator interface {
	// Next returns the next sentence.
	Next() (string, error)
	// Close closes the iterator.
	Close()
}

// SentenceSegmenter is an interface for segmenting text into sentences.
type SentenceSegmenter interface {
	// Segment segments the text from the reader into sentences.
	Segment(io.Reader) (SentenceIterator, error)
}

// AudioDecoder is an interface for decoding audio data.
type AudioDecoder interface {
	// Decode decodes the audio data from the reader into the given PCM format.
	Decode(r io.Reader, format pcm.Format) (io.Reader, error)
}

// TextSynthesizer is an interface for synthesizing text into audio data.
type TextSynthesizer interface {
	// SynthesizeText synthesizes the text into audio data and writes it to the writer.
	SynthesizeText(ctx context.Context, text string, w io.Writer) error
}

// Synthesizer is an interface for a text-to-speech synthesizer.
type Synthesizer interface {
	// Synthesize synthesizes the text from the reader into speech.
	Synthesize(ctx context.Context, name string, textStream io.Reader, format pcm.Format) (Speech, error)
}

// SynthesizeFunc is a function that implements the Synthesizer interface.
type SynthesizeFunc func(ctx context.Context, name string, textStream io.Reader, format pcm.Format) (Speech, error)

// Synthesize implements the Synthesizer interface.
func (f SynthesizeFunc) Synthesize(ctx context.Context, name string, textStream io.Reader, format pcm.Format) (Speech, error) {
	return f(ctx, name, textStream, format)
}

// TTS is a multiplexer for TTS synthesizers.
type TTS struct {
	mux *trie.Trie[Synthesizer]
}

var _ Synthesizer = (*TTS)(nil)

// NewTTSMux creates a new TTS multiplexer.
func NewTTSMux() *TTS {
	return &TTS{
		mux: trie.New[Synthesizer](),
	}
}

// Handle registers a synthesizer for the given name.
func (r *TTS) Handle(name string, synthesizer Synthesizer) error {
	return r.mux.Set(name, func(ptr *Synthesizer, existed bool) error {
		*ptr = synthesizer
		if existed {
			slog.Warn("tts: synthesizer already registered", "name", name)
		}
		return nil
	})
}

// HandleFunc registers a synthesizer function for the given name.
func (r *TTS) HandleFunc(name string, f SynthesizeFunc) error {
	return r.Handle(name, f)
}

// Synthesize synthesizes speech for the given name.
func (r *TTS) Synthesize(ctx context.Context, name string, textStream io.Reader, format pcm.Format) (Speech, error) {
	syn, ok := r.mux.GetValue(name)
	if !ok || syn == nil {
		return nil, fmt.Errorf("tts: synthesizer not found for %s", name)
	}
	return syn.Synthesize(ctx, name, textStream, format)
}
