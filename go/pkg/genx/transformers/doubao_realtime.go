package transformers

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/doubaospeech"
	"github.com/haivivi/giztoy/go/pkg/genx"
)

// DoubaoRealtime is a realtime transformer using Doubao realtime dialogue.
//
// Resource ID: volc.speech.dialog
//
// This is a bidirectional transformer:
// Input: genx.Stream with audio Blob chunks (user audio)
// Output: genx.Stream with audio Blob chunks (model response)
//
// Internally uses ASR → LLM → TTS pipeline.
type DoubaoRealtime struct {
	client            *doubaospeech.Client
	speaker           string
	format            string
	sampleRate        int
	channels          int
	botName           string
	systemRole        string
	vadWindowMs       int // VAD end detection window in milliseconds
	speakingStyle     string
	characterManifest string
	model             string // Model version: O, SC, 1.2.1.0 (O2.0), 2.2.0.0 (SC2.0)
}

var _ genx.Transformer = (*DoubaoRealtime)(nil)

// DoubaoRealtimeOption is a functional option for DoubaoRealtime.
type DoubaoRealtimeOption func(*DoubaoRealtime)

// WithDoubaoRealtimeSpeaker sets the TTS speaker voice.
func WithDoubaoRealtimeSpeaker(speaker string) DoubaoRealtimeOption {
	return func(t *DoubaoRealtime) {
		t.speaker = speaker
	}
}

// WithDoubaoRealtimeFormat sets the audio format.
func WithDoubaoRealtimeFormat(format string) DoubaoRealtimeOption {
	return func(t *DoubaoRealtime) {
		t.format = format
	}
}

// WithDoubaoRealtimeSampleRate sets the sample rate.
func WithDoubaoRealtimeSampleRate(sampleRate int) DoubaoRealtimeOption {
	return func(t *DoubaoRealtime) {
		t.sampleRate = sampleRate
	}
}

// WithDoubaoRealtimeChannels sets the number of channels.
func WithDoubaoRealtimeChannels(channels int) DoubaoRealtimeOption {
	return func(t *DoubaoRealtime) {
		t.channels = channels
	}
}

// WithDoubaoRealtimeBotName sets the bot name.
func WithDoubaoRealtimeBotName(botName string) DoubaoRealtimeOption {
	return func(t *DoubaoRealtime) {
		t.botName = botName
	}
}

// WithDoubaoRealtimeSystemRole sets the system role/prompt.
func WithDoubaoRealtimeSystemRole(systemRole string) DoubaoRealtimeOption {
	return func(t *DoubaoRealtime) {
		t.systemRole = systemRole
	}
}

// WithDoubaoRealtimeVADWindow sets the VAD end detection window in milliseconds.
// Smaller values (100-200ms) give faster response but may cut off speech.
// Larger values (500-1000ms) are more tolerant of pauses but slower.
func WithDoubaoRealtimeVADWindow(windowMs int) DoubaoRealtimeOption {
	return func(t *DoubaoRealtime) {
		t.vadWindowMs = windowMs
	}
}

// WithDoubaoRealtimeSpeakingStyle sets the speaking style.
func WithDoubaoRealtimeSpeakingStyle(style string) DoubaoRealtimeOption {
	return func(t *DoubaoRealtime) {
		t.speakingStyle = style
	}
}

// WithDoubaoRealtimeCharacterManifest sets the character manifest for role-playing.
func WithDoubaoRealtimeCharacterManifest(manifest string) DoubaoRealtimeOption {
	return func(t *DoubaoRealtime) {
		t.characterManifest = manifest
	}
}

// WithDoubaoRealtimeModel sets the model version.
// Valid values: "O" (default), "SC", "1.2.1.0" (O2.0), "2.2.0.0" (SC2.0)
func WithDoubaoRealtimeModel(model string) DoubaoRealtimeOption {
	return func(t *DoubaoRealtime) {
		t.model = model
	}
}

// NewDoubaoRealtime creates a new DoubaoRealtime transformer.
//
// Parameters:
//   - client: Doubao speech client
//   - opts: Optional configuration
func NewDoubaoRealtime(client *doubaospeech.Client, opts ...DoubaoRealtimeOption) *DoubaoRealtime {
	t := &DoubaoRealtime{
		client:      client,
		speaker:     "zh_female_vv_jupiter_bigtts", // O version default voice
		format:      "pcm_s16le",                   // 16-bit PCM for portaudio compatibility
		sampleRate:  24000,
		channels:    1,
		vadWindowMs: 200,   // Default VAD window
		model:       "O",   // Default to O version
		botName:     "豆包", // Default bot name
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// DoubaoRealtimeCtxKey is the context key for runtime options.
type doubaoRealtimeCtxKey struct{}

// DoubaoRealtimeCtxOptions are runtime options passed via context.
// TODO: Add fields as needed for runtime configuration.
type DoubaoRealtimeCtxOptions struct{}

// WithDoubaoRealtimeCtxOptions attaches runtime options to context.
func WithDoubaoRealtimeCtxOptions(ctx context.Context, opts DoubaoRealtimeCtxOptions) context.Context {
	return context.WithValue(ctx, doubaoRealtimeCtxKey{}, opts)
}

// Transform converts audio input to audio output via realtime dialogue.
// It synchronously waits for the connection to be established before returning.
func (t *DoubaoRealtime) Transform(ctx context.Context, _ string, input genx.Stream) (genx.Stream, error) {
	// Build config with ASR settings
	config := &doubaospeech.RealtimeConfig{
		ASR: doubaospeech.RealtimeASRConfig{
			Extra: map[string]any{
				"end_smooth_window_ms": t.vadWindowMs,
			},
		},
		TTS: doubaospeech.RealtimeTTSConfig{
			Speaker: t.speaker,
			AudioConfig: doubaospeech.RealtimeAudioConfig{
				Format:     t.format,
				SampleRate: t.sampleRate,
				Channel:    t.channels,
			},
		},
		Dialog: doubaospeech.RealtimeDialogConfig{
			BotName:           t.botName,
			SystemRole:        t.systemRole,
			SpeakingStyle:     t.speakingStyle,
			CharacterManifest: t.characterManifest,
			Extra: map[string]any{
				"model": t.model, // Model version: O, SC, etc.
			},
		},
	}

	// Connect to realtime service (synchronous)
	session, err := t.client.Realtime.Connect(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("doubao realtime connect: %w", err)
	}

	output := newBufferStream(100)
	go t.processLoop(ctx, input, output, session)

	return output, nil
}

func (t *DoubaoRealtime) processLoop(ctx context.Context, input genx.Stream, output *bufferStream, session *doubaospeech.RealtimeSession) {
	defer output.Close()
	defer session.Close()

	// StreamID management - queue for correlating input to output
	var streamIDMu sync.Mutex
	var streamIDQueue []string
	var responseStreamID string

	pushStreamID := func(id string) {
		streamIDMu.Lock()
		defer streamIDMu.Unlock()
		streamIDQueue = append(streamIDQueue, id)
		slog.Debug("doubao: pushed streamID", "id", id, "queueLen", len(streamIDQueue))
	}

	popStreamIDForResponse := func() {
		streamIDMu.Lock()
		defer streamIDMu.Unlock()
		if len(streamIDQueue) > 0 {
			responseStreamID = streamIDQueue[0]
			streamIDQueue = streamIDQueue[1:]
			slog.Debug("doubao: popped streamID for response", "id", responseStreamID)
		} else {
			// Generate a new StreamID if queue is empty
			responseStreamID = genx.NewStreamID()
			slog.Debug("doubao: generated new streamID for response", "id", responseStreamID)
		}
	}

	getResponseStreamID := func() string {
		streamIDMu.Lock()
		defer streamIDMu.Unlock()
		return responseStreamID
	}

	// Start goroutine to receive events
	eventsDone := make(chan struct{})
	go func() {
		defer close(eventsDone)
		for event, err := range session.Recv() {
			if err != nil {
				slog.Error("doubao: recv error", "error", err)
				output.CloseWithError(err)
				return
			}

			slog.Debug("doubao: received event", "type", event.Type, "text", event.Text, "audioLen", len(event.Audio))

			// Get StreamID for this response
			streamID := getResponseStreamID()

			// Handle different event types
			switch event.Type {
		case doubaospeech.EventASRInfo:
			// ASR detected speech - log for debugging
			// Note: Do NOT interrupt here. EventASRInfo is just speech detection,
			// not a user interruption. Interruption should be handled by the
			// cortex layer based on device state changes (e.g., user pressing button).
			slog.Info("doubao: ASR info - speech detected")

			case doubaospeech.EventASRResponse:
				// ASR text result
				slog.Info("doubao: ASR response", "text", event.Text)
				if event.Text != "" {
					outChunk := &genx.MessageChunk{
						Role: genx.RoleUser,
						Part: genx.Text(event.Text),
						Ctrl: &genx.StreamCtrl{StreamID: streamID},
					}
					if err := output.Push(outChunk); err != nil {
						return
					}
				}

			case doubaospeech.EventASREnded:
				// User speech ended - pop StreamID for upcoming response
				slog.Info("doubao: ASR ended")
				popStreamIDForResponse()

			case doubaospeech.EventTTSStarted:
				// TTS started - send BOS to signal start of audio stream
				slog.Info("doubao: TTS started, sending BOS", "streamID", streamID)
				bosChunk := &genx.MessageChunk{
					Role: genx.RoleModel,
					Part: &genx.Blob{MIMEType: t.mimeType()},
					Ctrl: &genx.StreamCtrl{StreamID: streamID, BeginOfStream: true},
				}
				if err := output.Push(bosChunk); err != nil {
					return
				}

				// Also send text if available
				if event.TTSInfo != nil && event.TTSInfo.Content != "" {
					outChunk := &genx.MessageChunk{
						Role: genx.RoleModel,
						Part: genx.Text(event.TTSInfo.Content),
						Ctrl: &genx.StreamCtrl{StreamID: streamID},
					}
					if err := output.Push(outChunk); err != nil {
						return
					}
				}

			case doubaospeech.EventChatResponse:
				// Model text response
				if event.Text != "" {
					slog.Debug("doubao: chat response", "text", event.Text)
					outChunk := &genx.MessageChunk{
						Role: genx.RoleModel,
						Part: genx.Text(event.Text),
						Ctrl: &genx.StreamCtrl{StreamID: streamID},
					}
					if err := output.Push(outChunk); err != nil {
						return
					}
				}

			case doubaospeech.EventAudioReceived:
				// Audio chunk received
				if len(event.Audio) > 0 {
					slog.Debug("doubao: audio received", "len", len(event.Audio))
					outChunk := &genx.MessageChunk{
						Role: genx.RoleModel,
						Part: &genx.Blob{
							MIMEType: t.mimeType(),
							Data:     event.Audio,
						},
						Ctrl: &genx.StreamCtrl{StreamID: streamID},
					}
					if err := output.Push(outChunk); err != nil {
						return
					}
				}

			case doubaospeech.EventTTSFinished:
				// TTS finished - send EOS to signal end of audio stream
				slog.Info("doubao: TTS finished, sending EOS", "streamID", streamID)
				eosChunk := &genx.MessageChunk{
					Role: genx.RoleModel,
					Part: &genx.Blob{MIMEType: t.mimeType()},
					Ctrl: &genx.StreamCtrl{StreamID: streamID, EndOfStream: true},
				}
				if err := output.Push(eosChunk); err != nil {
					return
				}
				// Don't return - continue listening for more events (multi-turn)

			case doubaospeech.EventChatEnded:
				// Model response ended (text complete, audio may follow)
				slog.Debug("doubao: chat ended")

			case doubaospeech.EventSessionEnded:
				// Session ended
				slog.Info("doubao: session ended")
				return
			}
		}
	}()

	slog.Info("doubao: starting audio send loop")

	// Send audio to realtime service
	audioSent := 0
	for {
		select {
		case <-ctx.Done():
			slog.Info("doubao: context cancelled")
			output.CloseWithError(ctx.Err())
			return
		case <-eventsDone:
			slog.Info("doubao: events done")
			return
		default:
		}

		chunk, err := input.Next()
		if err != nil {
			if err != io.EOF && err != genx.ErrDone {
				slog.Error("doubao: input error", "error", err)
				output.CloseWithError(err)
			} else {
				slog.Info("doubao: input EOF", "audioSent", audioSent)
			}
			// Wait for remaining events
			<-eventsDone
			return
		}

		if chunk == nil {
			continue
		}

		// Track StreamID from BOS marker only
		if chunk.IsBeginOfStream() && chunk.Ctrl != nil && chunk.Ctrl.StreamID != "" {
			pushStreamID(chunk.Ctrl.StreamID)
			slog.Info("doubao: received BOS", "streamID", chunk.Ctrl.StreamID)
			continue
		}

		// Handle EOS - send silence to help VAD detect end of speech
		if chunk.IsEndOfStream() {
			slog.Info("doubao: received EOS, sending silence for VAD", "audioSent", audioSent)
			// Send 500ms of silence (16kHz, 16-bit mono = 16000 bytes for 500ms)
			silence := make([]byte, 16000)
			if err := session.SendAudio(ctx, silence); err != nil {
				slog.Error("doubao: send silence error", "error", err)
			}
			// Don't return - wait for Doubao to process and respond
			continue
		}

		// Send based on part type
		switch p := chunk.Part.(type) {
		case *genx.Blob:
			// Send audio blob
			if len(p.Data) > 0 {
				audioSent++
				if audioSent%50 == 1 { // Log every 50 chunks (1 second at 20ms chunks)
					slog.Debug("doubao: sending audio chunk", "len", len(p.Data), "totalSent", audioSent)
				}
				if err := session.SendAudio(ctx, p.Data); err != nil {
					slog.Error("doubao: send audio error", "error", err)
					output.CloseWithError(err)
					return
				}
			}
		case genx.Text:
			// Send text query
			if len(p) > 0 {
				slog.Info("doubao: sending text", "text", string(p))
				if err := session.SendText(ctx, string(p)); err != nil {
					slog.Error("doubao: send text error", "error", err)
					output.CloseWithError(err)
					return
				}
			}
		}
	}
}

func (t *DoubaoRealtime) mimeType() string {
	switch t.format {
	case "mp3":
		return "audio/mpeg"
	case "ogg_opus":
		return "audio/ogg"
	case "pcm", "pcm_s16le":
		return "audio/pcm"
	default:
		return "audio/pcm"
	}
}
