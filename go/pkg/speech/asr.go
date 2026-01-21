package speech

import (
	"context"
	"fmt"

	"github.com/haivivi/giztoy/pkg/audio/opusrt"
	"github.com/haivivi/giztoy/pkg/trie"
)

// ASRMux is the default multiplexer for ASR transcribers.
var ASRMux = NewASRMux()

// HandleASR registers a StreamTranscriber for the given name with the default mux.
func HandleASR(name string, transcriber StreamTranscriber) error {
	return ASRMux.Handle(name, transcriber)
}

// HandleASRFunc registers a TranscribeStreamFunc for the given name with the default mux.
func HandleASRFunc(name string, f TranscribeStreamFunc) error {
	return ASRMux.HandleFunc(name, f)
}

// TranscribeStream performs streaming transcription on an Opus audio stream
// using the default mux.
func TranscribeStream(ctx context.Context, name string, opus opusrt.FrameReader) (SpeechStream, error) {
	return ASRMux.TranscribeStream(ctx, name, opus)
}

// Transcribe transcribes an entire Opus audio stream using the default mux.
func Transcribe(ctx context.Context, name string, opus opusrt.FrameReader) (Speech, error) {
	return ASRMux.Transcribe(ctx, name, opus)
}

// StreamTranscriber is the interface that wraps the TranscribeStream method.
type StreamTranscriber interface {
	// TranscribeStream performs streaming transcription on an Opus audio stream.
	TranscribeStream(ctx context.Context, model string, opus opusrt.FrameReader) (SpeechStream, error)
}

// TranscribeStreamFunc is an adapter to allow the use of ordinary functions as
// StreamTranscribers.
type TranscribeStreamFunc func(ctx context.Context, model string, opus opusrt.FrameReader) (SpeechStream, error)

// TranscribeStream calls the underlying function.
func (f TranscribeStreamFunc) TranscribeStream(ctx context.Context, model string, opus opusrt.FrameReader) (SpeechStream, error) {
	return f(ctx, model, opus)
}

// Transcriber is the interface that wraps the Transcribe method.
type Transcriber interface {
	// Transcribe transcribes an entire Opus audio stream.
	Transcribe(ctx context.Context, model string, opus opusrt.FrameReader) (Speech, error)
}

// ASR is a multiplexer for ASR transcribers. It routes transcription requests
// to the appropriate registered transcriber based on a name.
type ASR struct {
	mux *trie.Trie[StreamTranscriber]
}

// NewASRMux creates and returns a new ASR multiplexer.
func NewASRMux() *ASR {
	return &ASR{
		mux: trie.New[StreamTranscriber](),
	}
}

// Handle registers a StreamTranscriber for the given name.
func (m *ASR) Handle(name string, transcriber StreamTranscriber) error {
	return m.mux.SetValue(name, transcriber)
}

// HandleFunc registers a TranscribeStreamFunc for the given name.
func (m *ASR) HandleFunc(name string, f TranscribeStreamFunc) error {
	return m.Handle(name, f)
}

// TranscribeStream performs streaming transcription on an Opus audio stream. It
// dispatches the request to the transcriber registered for the given name.
func (m *ASR) TranscribeStream(ctx context.Context, name string, opus opusrt.FrameReader) (SpeechStream, error) {
	transcriber, ok := m.mux.GetValue(name)
	if !ok || transcriber == nil {
		return nil, fmt.Errorf("asr: transcriber not found for %s", name)
	}
	return transcriber.TranscribeStream(ctx, name, opus)
}

// Transcribe transcribes an entire Opus audio stream. It returns the complete
// transcription. It dispatches the request to the transcriber registered for the
// given name. If the transcriber implements the Transcriber interface, it will
// be used directly. Otherwise, it will fall back to the streaming interface and
// collect the results.
func (m *ASR) Transcribe(ctx context.Context, name string, opus opusrt.FrameReader) (Speech, error) {
	transcriber, ok := m.mux.GetValue(name)
	if !ok || transcriber == nil {
		return nil, fmt.Errorf("asr: transcriber not found for %s", name)
	}
	if t, ok := transcriber.(Transcriber); ok {
		return t.Transcribe(ctx, name, opus)
	}
	stream, err := m.TranscribeStream(ctx, name, opus)
	if err != nil {
		return nil, err
	}
	return CollectSpeech(stream), nil
}
