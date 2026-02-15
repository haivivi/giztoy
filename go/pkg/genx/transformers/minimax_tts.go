package transformers

import (
	"context"
	"io"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/minimax"
)

// MinimaxTTS is a TTS transformer using MiniMax text-to-speech API.
//
// Model: speech-02-hd (default)
//
// Input type: text/plain
// Output type: audio/* (audio/mpeg by default)
//
// EoS Handling:
//   - When receiving a text/plain EoS marker, finish synthesis, emit audio chunks, then emit audio/* EoS
//   - Non-text chunks are passed through unchanged
type MinimaxTTS struct {
	client     *minimax.Client
	model      string
	voiceID    string
	speed      float64
	vol        float64
	pitch      int
	emotion    string
	format     string
	sampleRate int
	bitrate    int
}

var _ genx.Transformer = (*MinimaxTTS)(nil)

// MinimaxTTSOption is a functional option for MinimaxTTS.
type MinimaxTTSOption func(*MinimaxTTS)

// WithMinimaxTTSModel sets the model.
func WithMinimaxTTSModel(model string) MinimaxTTSOption {
	return func(t *MinimaxTTS) {
		t.model = model
	}
}

// WithMinimaxTTSSpeed sets the speech speed (0.5-2.0).
func WithMinimaxTTSSpeed(speed float64) MinimaxTTSOption {
	return func(t *MinimaxTTS) {
		t.speed = speed
	}
}

// WithMinimaxTTSVolume sets the volume (0-10).
func WithMinimaxTTSVolume(vol float64) MinimaxTTSOption {
	return func(t *MinimaxTTS) {
		t.vol = vol
	}
}

// WithMinimaxTTSPitch sets the pitch adjustment (-12 to 12).
func WithMinimaxTTSPitch(pitch int) MinimaxTTSOption {
	return func(t *MinimaxTTS) {
		t.pitch = pitch
	}
}

// WithMinimaxTTSEmotion sets the emotion.
// Options: happy, sad, angry, fearful, disgusted, surprised, neutral
func WithMinimaxTTSEmotion(emotion string) MinimaxTTSOption {
	return func(t *MinimaxTTS) {
		t.emotion = emotion
	}
}

// WithMinimaxTTSFormat sets the audio format.
// Options: mp3, pcm, flac, wav
func WithMinimaxTTSFormat(format string) MinimaxTTSOption {
	return func(t *MinimaxTTS) {
		t.format = format
	}
}

// WithMinimaxTTSSampleRate sets the sample rate.
// Options: 8000, 16000, 22050, 24000, 32000, 44100
func WithMinimaxTTSSampleRate(sampleRate int) MinimaxTTSOption {
	return func(t *MinimaxTTS) {
		t.sampleRate = sampleRate
	}
}

// WithMinimaxTTSBitrate sets the bitrate.
// Options: 32000, 64000, 128000, 256000
func WithMinimaxTTSBitrate(bitrate int) MinimaxTTSOption {
	return func(t *MinimaxTTS) {
		t.bitrate = bitrate
	}
}

// NewMinimaxTTS creates a new MinimaxTTS transformer.
//
// Parameters:
//   - client: MiniMax client
//   - voiceID: Voice identifier (e.g., "female-shaonv", "male-qn-qingse")
//   - opts: Optional configuration
func NewMinimaxTTS(client *minimax.Client, voiceID string, opts ...MinimaxTTSOption) *MinimaxTTS {
	t := &MinimaxTTS{
		client:     client,
		model:      "speech-02-hd",
		voiceID:    voiceID,
		speed:      1.0,
		vol:        1.0,
		format:     "mp3",
		sampleRate: 32000,
		bitrate:    128000,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// MinimaxTTSCtxKey is the context key for runtime options.
type minimaxTTSCtxKey struct{}

// MinimaxTTSCtxOptions are runtime options passed via context.
// TODO: Add fields as needed for runtime configuration.
type MinimaxTTSCtxOptions struct{}

// WithMinimaxTTSCtxOptions attaches runtime options to context.
func WithMinimaxTTSCtxOptions(ctx context.Context, opts MinimaxTTSCtxOptions) context.Context {
	return context.WithValue(ctx, minimaxTTSCtxKey{}, opts)
}

// Transform converts Text chunks to audio Blob chunks.
// MinimaxTTS does not require connection setup, so it returns immediately.
// The ctx is unused (no initialization needed); the goroutine lifetime
// is governed by the input Stream.
func (t *MinimaxTTS) Transform(_ context.Context, _ string, input genx.Stream) (genx.Stream, error) {
	output := newBufferStream(100)

	go t.transformLoop(input, output)

	return output, nil
}

func (t *MinimaxTTS) transformLoop(input genx.Stream, output *bufferStream) {
	defer output.Close()

	// Local cancel context tied to the loop lifecycle.
	// When the loop exits, defer cancel() cancels any in-flight HTTP request.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mimeType := t.mimeType()
	var textBuilder strings.Builder
	var lastChunk *genx.MessageChunk
	var currentStreamID string

	for {
		chunk, err := input.Next()
		if err != nil {
			if err != io.EOF {
				output.CloseWithError(err)
				return
			}
			// EOF: synthesize any remaining text
			if textBuilder.Len() > 0 {
				if err := t.synthesize(ctx, textBuilder.String(), lastChunk, currentStreamID, mimeType, output); err != nil {
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

		// Track StreamID from input - inherit or generate new one
		if chunk.Ctrl != nil && chunk.Ctrl.StreamID != "" {
			currentStreamID = chunk.Ctrl.StreamID
		} else if currentStreamID == "" {
			// Generate new StreamID if none provided
			currentStreamID = genx.NewStreamID()
		}

		// Check for text EoS marker
		if chunk.IsEndOfStream() {
			if _, ok := chunk.Part.(genx.Text); ok {
				// Text EoS: synthesize accumulated text, emit audio, then emit audio EoS
				if textBuilder.Len() > 0 {
				if err := t.synthesize(ctx, textBuilder.String(), lastChunk, currentStreamID, mimeType, output); err != nil {
					output.CloseWithError(err)
					return
				}
				textBuilder.Reset()
				}
				// Emit audio EoS with StreamID
				eosChunk := &genx.MessageChunk{
					Part: &genx.Blob{MIMEType: mimeType},
					Ctrl: &genx.StreamCtrl{StreamID: currentStreamID, EndOfStream: true},
				}
				if lastChunk != nil {
					eosChunk.Role = lastChunk.Role
					eosChunk.Name = lastChunk.Name
				}
				if err := output.Push(eosChunk); err != nil {
					return
				}
				// Reset StreamID for next synthesis segment
				currentStreamID = ""
				continue
			}
			// Non-text EoS: pass through with StreamID
			if chunk.Ctrl == nil {
				chunk.Ctrl = &genx.StreamCtrl{}
			}
			chunk.Ctrl.StreamID = currentStreamID
			if err := output.Push(chunk); err != nil {
				return
			}
			continue
		}

		// Collect text
		if text, ok := chunk.Part.(genx.Text); ok {
			textBuilder.WriteString(string(text))
		} else {
			// Non-text chunk: pass through with StreamID
			if chunk.Ctrl == nil {
				chunk.Ctrl = &genx.StreamCtrl{}
			}
			chunk.Ctrl.StreamID = currentStreamID
			if err := output.Push(chunk); err != nil {
				return
			}
		}
	}
}

func (t *MinimaxTTS) synthesize(ctx context.Context, text string, lastChunk *genx.MessageChunk, streamID, mimeType string, output *bufferStream) error {
	// Emit BOS at the start of synthesis
	bosChunk := &genx.MessageChunk{
		Ctrl: &genx.StreamCtrl{StreamID: streamID, BeginOfStream: true},
	}
	if lastChunk != nil {
		bosChunk.Role = lastChunk.Role
		bosChunk.Name = lastChunk.Name
	}
	if err := output.Push(bosChunk); err != nil {
		return err
	}

	req := &minimax.SpeechRequest{
		Model: t.model,
		Text:  text,
		VoiceSetting: &minimax.VoiceSetting{
			VoiceID: t.voiceID,
			Speed:   t.speed,
			Vol:     t.vol,
			Pitch:   t.pitch,
			Emotion: t.emotion,
		},
		AudioSetting: &minimax.AudioSetting{
			Format:     minimax.AudioFormat(t.format),
			SampleRate: t.sampleRate,
			Bitrate:    t.bitrate,
		},
	}

	for chunk, err := range t.client.Speech.SynthesizeStream(ctx, req) {
		if err != nil {
			return err
		}

		if chunk.Audio != nil && len(chunk.Audio) > 0 {
			outChunk := &genx.MessageChunk{
				Part: &genx.Blob{
					MIMEType: mimeType,
					Data:     chunk.Audio,
				},
				Ctrl: &genx.StreamCtrl{StreamID: streamID},
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

func (t *MinimaxTTS) mimeType() string {
	switch t.format {
	case "mp3":
		return "audio/mpeg"
	case "pcm":
		return "audio/pcm"
	case "flac":
		return "audio/flac"
	case "wav":
		return "audio/wav"
	default:
		return "audio/mpeg"
	}
}
