package transformers

import (
	"context"
	"io"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/doubaospeech"
	"github.com/haivivi/giztoy/go/pkg/genx"
)

// DoubaoTTSICLV2 is a TTS transformer using Doubao seed-icl-2.0 (声音复刻 2.0).
//
// Resource ID: seed-icl-2.0
//
// Speaker: Custom cloned voice ID starting with "S_" (e.g., "S_xxxxxx")
//
// Input type: text/plain
// Output type: audio/* (audio/ogg by default)
//
// EoS Handling:
//   - When receiving a text/plain EoS marker, finish synthesis, emit audio chunks, then emit audio/* EoS
//   - Non-text chunks are passed through unchanged
type DoubaoTTSICLV2 struct {
	client      *doubaospeech.Client
	speaker     string
	format      string
	sampleRate  int
	bitRate     int
	speedRatio  float64
	volumeRatio float64
	pitchRatio  float64
	emotion     string
	language    string
}

var _ genx.Transformer = (*DoubaoTTSICLV2)(nil)

// DoubaoTTSICLV2Option is a functional option for DoubaoTTSICLV2.
type DoubaoTTSICLV2Option func(*DoubaoTTSICLV2)

// WithDoubaoTTSICLV2Format sets the audio format (pcm, mp3, ogg_opus).
func WithDoubaoTTSICLV2Format(format string) DoubaoTTSICLV2Option {
	return func(t *DoubaoTTSICLV2) {
		t.format = format
	}
}

// WithDoubaoTTSICLV2SampleRate sets the sample rate (8000, 16000, 24000, 32000).
func WithDoubaoTTSICLV2SampleRate(sampleRate int) DoubaoTTSICLV2Option {
	return func(t *DoubaoTTSICLV2) {
		t.sampleRate = sampleRate
	}
}

// WithDoubaoTTSICLV2BitRate sets the bit rate for mp3 (32000, 64000, 128000).
func WithDoubaoTTSICLV2BitRate(bitRate int) DoubaoTTSICLV2Option {
	return func(t *DoubaoTTSICLV2) {
		t.bitRate = bitRate
	}
}

// WithDoubaoTTSICLV2Speed sets the speech speed ratio (0.2-3.0).
func WithDoubaoTTSICLV2Speed(speedRatio float64) DoubaoTTSICLV2Option {
	return func(t *DoubaoTTSICLV2) {
		t.speedRatio = speedRatio
	}
}

// WithDoubaoTTSICLV2Volume sets the volume ratio (0.1-3.0).
func WithDoubaoTTSICLV2Volume(volumeRatio float64) DoubaoTTSICLV2Option {
	return func(t *DoubaoTTSICLV2) {
		t.volumeRatio = volumeRatio
	}
}

// WithDoubaoTTSICLV2Pitch sets the pitch ratio (0.1-3.0).
func WithDoubaoTTSICLV2Pitch(pitchRatio float64) DoubaoTTSICLV2Option {
	return func(t *DoubaoTTSICLV2) {
		t.pitchRatio = pitchRatio
	}
}

// WithDoubaoTTSICLV2Emotion sets the emotion (happy, sad, angry, fear, hate, surprise).
func WithDoubaoTTSICLV2Emotion(emotion string) DoubaoTTSICLV2Option {
	return func(t *DoubaoTTSICLV2) {
		t.emotion = emotion
	}
}

// WithDoubaoTTSICLV2Language sets the language (zh, en, ja, etc.).
func WithDoubaoTTSICLV2Language(language string) DoubaoTTSICLV2Option {
	return func(t *DoubaoTTSICLV2) {
		t.language = language
	}
}

// NewDoubaoTTSICLV2 creates a new DoubaoTTSICLV2 transformer.
//
// Parameters:
//   - client: Doubao speech client
//   - speaker: Custom voice ID (must start with "S_", e.g., "S_xxxxxx")
//   - opts: Optional configuration
func NewDoubaoTTSICLV2(client *doubaospeech.Client, speaker string, opts ...DoubaoTTSICLV2Option) *DoubaoTTSICLV2 {
	t := &DoubaoTTSICLV2{
		client:      client,
		speaker:     speaker,
		format:      "ogg_opus",
		sampleRate:  24000,
		speedRatio:  1.0,
		volumeRatio: 1.0,
		pitchRatio:  1.0,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// DoubaoTTSICLV2CtxKey is the context key for runtime options.
type doubaoTTSICLV2CtxKey struct{}

// DoubaoTTSICLV2CtxOptions are runtime options passed via context.
// TODO: Add fields as needed for runtime configuration.
type DoubaoTTSICLV2CtxOptions struct{}

// WithDoubaoTTSICLV2CtxOptions attaches runtime options to context.
func WithDoubaoTTSICLV2CtxOptions(ctx context.Context, opts DoubaoTTSICLV2CtxOptions) context.Context {
	return context.WithValue(ctx, doubaoTTSICLV2CtxKey{}, opts)
}

// Transform converts Text chunks to audio Blob chunks.
// DoubaoTTSICLV2 does not require connection setup, so it returns immediately.
// The ctx is unused (no initialization needed); the goroutine lifetime
// is governed by the input Stream.
func (t *DoubaoTTSICLV2) Transform(_ context.Context, _ string, input genx.Stream) (genx.Stream, error) {
	output := newBufferStream(100)

	go t.transformLoop(input, output)

	return output, nil
}

func (t *DoubaoTTSICLV2) transformLoop(input genx.Stream, output *bufferStream) {
	defer output.Close()

	// Local cancel context tied to the loop lifecycle.
	// When the loop exits, defer cancel() cancels any in-flight HTTP request.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mimeType := t.mimeType()
	var textBuilder strings.Builder
	var lastChunk *genx.MessageChunk

	for {
		chunk, err := input.Next()
		if err != nil {
			if err != io.EOF {
				output.CloseWithError(err)
				return
			}
			// EOF: synthesize any remaining text
			if textBuilder.Len() > 0 {
				if err := t.synthesize(ctx, textBuilder.String(), lastChunk, mimeType, output); err != nil {
					output.CloseWithError(err)
					return
				}
			}
			return
		}

		if chunk == nil {
			continue
		}

		lastChunk = chunk

		// Check for text EoS marker
		if chunk.IsEndOfStream() {
			if _, ok := chunk.Part.(genx.Text); ok {
				// Text EoS: synthesize accumulated text, emit audio, then emit audio EoS
			if textBuilder.Len() > 0 {
				if err := t.synthesize(ctx, textBuilder.String(), lastChunk, mimeType, output); err != nil {
					output.CloseWithError(err)
					return
				}
				textBuilder.Reset()
			}
				// Emit audio EoS
				eosChunk := genx.NewEndOfStream(mimeType)
				if lastChunk != nil {
					eosChunk.Role = lastChunk.Role
					eosChunk.Name = lastChunk.Name
				}
				if err := output.Push(eosChunk); err != nil {
					return
				}
				continue
			}
			// Non-text EoS: pass through
			if err := output.Push(chunk); err != nil {
				return
			}
			continue
		}

		// Collect text
		if text, ok := chunk.Part.(genx.Text); ok {
			textBuilder.WriteString(string(text))
		} else {
			// Non-text chunk: pass through
			if err := output.Push(chunk); err != nil {
				return
			}
		}
	}
}

func (t *DoubaoTTSICLV2) synthesize(ctx context.Context, text string, lastChunk *genx.MessageChunk, mimeType string, output *bufferStream) error {
	req := &doubaospeech.TTSV2Request{
		Text:        text,
		Speaker:     t.speaker,
		ResourceID:  doubaospeech.ResourceVoiceCloneV2,
		Format:      t.format,
		SampleRate:  t.sampleRate,
		BitRate:     t.bitRate,
		SpeedRatio:  t.speedRatio,
		VolumeRatio: t.volumeRatio,
		PitchRatio:  t.pitchRatio,
		Emotion:     t.emotion,
		Language:    t.language,
	}

	for chunk, err := range t.client.TTSV2.Stream(ctx, req) {
		if err != nil {
			return err
		}

		if chunk.Audio != nil && len(chunk.Audio) > 0 {
			outChunk := &genx.MessageChunk{
				Part: &genx.Blob{
					MIMEType: mimeType,
					Data:     chunk.Audio,
				},
			}
			if lastChunk != nil {
				outChunk.Role = lastChunk.Role
				outChunk.Name = lastChunk.Name
			}

			if err := output.Push(outChunk); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *DoubaoTTSICLV2) mimeType() string {
	switch t.format {
	case "mp3":
		return "audio/mpeg"
	case "ogg_opus":
		return "audio/ogg"
	case "pcm":
		return "audio/pcm"
	default:
		return "audio/ogg"
	}
}
