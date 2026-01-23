package speech

import (
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/haivivi/giztoy/pkg/audio/pcm"
	"github.com/haivivi/giztoy/pkg/doubaospeech"

	"google.golang.org/api/iterator"
)

// DoubaoTTSV2Handler is a Doubao TTS 2.0 handler that implements the Synthesizer interface.
// It uses the BigModel TTS API (/api/v3/tts/unidirectional).
type DoubaoTTSV2Handler struct {
	client      *doubaospeech.Client
	speaker     string
	resourceID  string
	format      string
	bitRate     int
	speedRatio  float64
	volumeRatio float64
	pitchRatio  float64
	emotion     string
	language    string
	segmenter   SentenceSegmenter
}

var _ Synthesizer = (*DoubaoTTSV2Handler)(nil)

// DoubaoTTSV2Option is an option for configuring the DoubaoTTSV2Handler.
type DoubaoTTSV2Option func(*DoubaoTTSV2Handler)

// WithDoubaoTTSV2Speaker sets the speaker voice type.
func WithDoubaoTTSV2Speaker(speaker string) DoubaoTTSV2Option {
	return func(h *DoubaoTTSV2Handler) {
		h.speaker = speaker
	}
}

// WithDoubaoTTSV2ResourceID sets the resource ID (e.g., seed-tts-1.0, seed-tts-2.0).
func WithDoubaoTTSV2ResourceID(resourceID string) DoubaoTTSV2Option {
	return func(h *DoubaoTTSV2Handler) {
		h.resourceID = resourceID
	}
}

// WithDoubaoTTSV2Format sets the audio format (pcm, mp3, ogg_opus).
func WithDoubaoTTSV2Format(format string) DoubaoTTSV2Option {
	return func(h *DoubaoTTSV2Handler) {
		h.format = format
	}
}

// WithDoubaoTTSV2BitRate sets the bit rate for mp3 format.
func WithDoubaoTTSV2BitRate(bitRate int) DoubaoTTSV2Option {
	return func(h *DoubaoTTSV2Handler) {
		h.bitRate = bitRate
	}
}

// WithDoubaoTTSV2Speed sets the speech speed ratio (0.2-3.0).
func WithDoubaoTTSV2Speed(speedRatio float64) DoubaoTTSV2Option {
	return func(h *DoubaoTTSV2Handler) {
		h.speedRatio = speedRatio
	}
}

// WithDoubaoTTSV2Volume sets the volume ratio (0.1-3.0).
func WithDoubaoTTSV2Volume(volumeRatio float64) DoubaoTTSV2Option {
	return func(h *DoubaoTTSV2Handler) {
		h.volumeRatio = volumeRatio
	}
}

// WithDoubaoTTSV2Pitch sets the pitch ratio (0.1-3.0).
func WithDoubaoTTSV2Pitch(pitchRatio float64) DoubaoTTSV2Option {
	return func(h *DoubaoTTSV2Handler) {
		h.pitchRatio = pitchRatio
	}
}

// WithDoubaoTTSV2Emotion sets the emotion (happy, sad, angry, fear, hate, surprise).
func WithDoubaoTTSV2Emotion(emotion string) DoubaoTTSV2Option {
	return func(h *DoubaoTTSV2Handler) {
		h.emotion = emotion
	}
}

// WithDoubaoTTSV2Language sets the language (zh, en, ja, etc.).
func WithDoubaoTTSV2Language(language string) DoubaoTTSV2Option {
	return func(h *DoubaoTTSV2Handler) {
		h.language = language
	}
}

// WithDoubaoTTSV2Segmenter sets the sentence segmenter.
func WithDoubaoTTSV2Segmenter(segmenter SentenceSegmenter) DoubaoTTSV2Option {
	return func(h *DoubaoTTSV2Handler) {
		h.segmenter = segmenter
	}
}

// NewDoubaoTTSV2Handler creates a new Doubao TTS 2.0 handler.
func NewDoubaoTTSV2Handler(client *doubaospeech.Client, opts ...DoubaoTTSV2Option) *DoubaoTTSV2Handler {
	h := &DoubaoTTSV2Handler{
		client:      client,
		speaker:     "zh_female_cancan",
		resourceID:  doubaospeech.ResourceTTSV2, // Default to TTS 2.0
		format:      "pcm",
		speedRatio:  1.0,
		volumeRatio: 1.0,
		pitchRatio:  1.0,
		segmenter:   DefaultSentenceSegmenter{},
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Synthesize synthesizes text from the reader into speech.
func (h *DoubaoTTSV2Handler) Synthesize(ctx context.Context, name string, textStream io.Reader, format pcm.Format) (Speech, error) {
	segIter, err := h.segmenter.Segment(textStream)
	if err != nil {
		return nil, err
	}

	return &doubaoV2Speech{
		ctx:         ctx,
		handler:     h,
		segmentIter: segIter,
		format:      format,
	}, nil
}

// doubaoV2Speech implements the Speech interface.
type doubaoV2Speech struct {
	ctx         context.Context
	handler     *DoubaoTTSV2Handler
	segmentIter SentenceIterator
	format      pcm.Format
	closed      bool
}

var _ Speech = (*doubaoV2Speech)(nil)

func (s *doubaoV2Speech) Next() (SpeechSegment, error) {
	if s.closed {
		return nil, iterator.Done
	}

	text, err := s.segmentIter.Next()
	if err != nil {
		return nil, err
	}

	// Build TTS V2 request
	req := &doubaospeech.TTSV2Request{
		Text:        text,
		Speaker:     s.handler.speaker,
		ResourceID:  s.handler.resourceID,
		Format:      s.handler.format,
		SampleRate:  s.format.SampleRate(),
		BitRate:     s.handler.bitRate,
		SpeedRatio:  s.handler.speedRatio,
		VolumeRatio: s.handler.volumeRatio,
		PitchRatio:  s.handler.pitchRatio,
		Emotion:     s.handler.emotion,
		Language:    s.handler.language,
	}

	// Collect streaming audio
	var audioBuf bytes.Buffer
	for chunk, err := range s.handler.client.TTSV2.Stream(s.ctx, req) {
		if err != nil {
			return nil, err
		}
		if chunk.Audio != nil {
			audioBuf.Write(chunk.Audio)
		}
	}

	return &doubaoV2SpeechSegment{
		text:   text,
		audio:  audioBuf.Bytes(),
		format: s.format,
	}, nil
}

func (s *doubaoV2Speech) Close() error {
	s.closed = true
	s.segmentIter.Close()
	return nil
}

// doubaoV2SpeechSegment implements the SpeechSegment interface.
type doubaoV2SpeechSegment struct {
	text   string
	audio  []byte
	format pcm.Format
}

var _ SpeechSegment = (*doubaoV2SpeechSegment)(nil)

func (seg *doubaoV2SpeechSegment) Decode(best pcm.Format) VoiceSegment {
	return &doubaoV2VoiceSegment{
		audio:  seg.audio,
		format: seg.format,
	}
}

func (seg *doubaoV2SpeechSegment) Transcribe() io.ReadCloser {
	return io.NopCloser(bytes.NewReader([]byte(seg.text)))
}

func (seg *doubaoV2SpeechSegment) Close() error {
	return nil
}

// doubaoV2VoiceSegment implements the VoiceSegment interface.
type doubaoV2VoiceSegment struct {
	mu     sync.Mutex
	audio  []byte
	offset int
	format pcm.Format
}

var _ VoiceSegment = (*doubaoV2VoiceSegment)(nil)

func (v *doubaoV2VoiceSegment) Read(p []byte) (n int, err error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.offset >= len(v.audio) {
		return 0, io.EOF
	}

	n = copy(p, v.audio[v.offset:])
	v.offset += n
	return n, nil
}

func (v *doubaoV2VoiceSegment) Format() pcm.Format {
	return v.format
}

func (v *doubaoV2VoiceSegment) Close() error {
	return nil
}
