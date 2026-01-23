package speech

import (
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/haivivi/giztoy/pkg/audio/pcm"
	"github.com/haivivi/giztoy/pkg/minimax"

	"google.golang.org/api/iterator"
)

// MinimaxTTSHandler is a MiniMax TTS handler that implements the Synthesizer interface.
type MinimaxTTSHandler struct {
	client    *minimax.Client
	model     string
	voiceID   string
	speed     float64
	vol       float64
	pitch     int
	emotion   string
	segmenter SentenceSegmenter
}

var _ Synthesizer = (*MinimaxTTSHandler)(nil)

// MinimaxTTSOption is an option for configuring the MinimaxTTSHandler.
type MinimaxTTSOption func(*MinimaxTTSHandler)

// WithMinimaxTTSModel sets the TTS model.
func WithMinimaxTTSModel(model string) MinimaxTTSOption {
	return func(h *MinimaxTTSHandler) {
		h.model = model
	}
}

// WithMinimaxTTSVoice sets the voice ID.
func WithMinimaxTTSVoice(voiceID string) MinimaxTTSOption {
	return func(h *MinimaxTTSHandler) {
		h.voiceID = voiceID
	}
}

// WithMinimaxTTSSpeed sets the speech speed (0.5-2.0).
func WithMinimaxTTSSpeed(speed float64) MinimaxTTSOption {
	return func(h *MinimaxTTSHandler) {
		h.speed = speed
	}
}

// WithMinimaxTTSVolume sets the volume (0-10).
func WithMinimaxTTSVolume(vol float64) MinimaxTTSOption {
	return func(h *MinimaxTTSHandler) {
		h.vol = vol
	}
}

// WithMinimaxTTSPitch sets the pitch (-12 to 12).
func WithMinimaxTTSPitch(pitch int) MinimaxTTSOption {
	return func(h *MinimaxTTSHandler) {
		h.pitch = pitch
	}
}

// WithMinimaxTTSEmotion sets the emotion.
func WithMinimaxTTSEmotion(emotion string) MinimaxTTSOption {
	return func(h *MinimaxTTSHandler) {
		h.emotion = emotion
	}
}

// WithMinimaxTTSSegmenter sets the sentence segmenter.
func WithMinimaxTTSSegmenter(segmenter SentenceSegmenter) MinimaxTTSOption {
	return func(h *MinimaxTTSHandler) {
		h.segmenter = segmenter
	}
}

// NewMinimaxTTSHandler creates a new MiniMax TTS handler.
func NewMinimaxTTSHandler(client *minimax.Client, opts ...MinimaxTTSOption) *MinimaxTTSHandler {
	h := &MinimaxTTSHandler{
		client:    client,
		model:     minimax.ModelSpeech26HD,
		voiceID:   minimax.VoiceFemaleShaonv,
		speed:     1.0,
		vol:       1.0,
		segmenter: DefaultSentenceSegmenter{},
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Synthesize synthesizes text from the reader into speech.
func (h *MinimaxTTSHandler) Synthesize(ctx context.Context, name string, textStream io.Reader, format pcm.Format) (Speech, error) {
	segIter, err := h.segmenter.Segment(textStream)
	if err != nil {
		return nil, err
	}

	return &minimaxSpeech{
		ctx:         ctx,
		handler:     h,
		segmentIter: segIter,
		format:      format,
	}, nil
}

// minimaxSpeech implements the Speech interface.
type minimaxSpeech struct {
	ctx         context.Context
	handler     *MinimaxTTSHandler
	segmentIter SentenceIterator
	format      pcm.Format
	closed      bool
}

var _ Speech = (*minimaxSpeech)(nil)

func (s *minimaxSpeech) Next() (SpeechSegment, error) {
	if s.closed {
		return nil, iterator.Done
	}

	text, err := s.segmentIter.Next()
	if err != nil {
		return nil, err
	}

	// Build speech request
	req := &minimax.SpeechRequest{
		Model: s.handler.model,
		Text:  text,
		VoiceSetting: &minimax.VoiceSetting{
			VoiceID: s.handler.voiceID,
			Speed:   s.handler.speed,
			Vol:     s.handler.vol,
			Pitch:   s.handler.pitch,
			Emotion: s.handler.emotion,
		},
		AudioSetting: &minimax.AudioSetting{
			SampleRate: s.format.SampleRate(),
			Format:     minimax.AudioFormatPCM,
			Channel:    s.format.Channels(),
		},
	}

	// Collect streaming audio
	var audioBuf bytes.Buffer
	for chunk, err := range s.handler.client.Speech.SynthesizeStream(s.ctx, req) {
		if err != nil {
			return nil, err
		}
		if chunk.Audio != nil {
			audioBuf.Write(chunk.Audio)
		}
	}

	return &minimaxSpeechSegment{
		text:   text,
		audio:  audioBuf.Bytes(),
		format: s.format,
	}, nil
}

func (s *minimaxSpeech) Close() error {
	s.closed = true
	s.segmentIter.Close()
	return nil
}

// minimaxSpeechSegment implements the SpeechSegment interface.
type minimaxSpeechSegment struct {
	text   string
	audio  []byte
	format pcm.Format
}

var _ SpeechSegment = (*minimaxSpeechSegment)(nil)

func (seg *minimaxSpeechSegment) Decode(best pcm.Format) VoiceSegment {
	return &minimaxVoiceSegment{
		audio:  seg.audio,
		format: seg.format,
	}
}

func (seg *minimaxSpeechSegment) Transcribe() io.ReadCloser {
	return io.NopCloser(bytes.NewReader([]byte(seg.text)))
}

func (seg *minimaxSpeechSegment) Close() error {
	return nil
}

// minimaxVoiceSegment implements the VoiceSegment interface.
type minimaxVoiceSegment struct {
	mu     sync.Mutex
	audio  []byte
	offset int
	format pcm.Format
}

var _ VoiceSegment = (*minimaxVoiceSegment)(nil)

func (v *minimaxVoiceSegment) Read(p []byte) (n int, err error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.offset >= len(v.audio) {
		return 0, io.EOF
	}

	n = copy(p, v.audio[v.offset:])
	v.offset += n
	return n, nil
}

func (v *minimaxVoiceSegment) Format() pcm.Format {
	return v.format
}

func (v *minimaxVoiceSegment) Close() error {
	return nil
}
