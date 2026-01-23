package speech

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/haivivi/giztoy/pkg/audio/codec/mp3"
	"github.com/haivivi/giztoy/pkg/audio/codec/opus"
	"github.com/haivivi/giztoy/pkg/audio/opusrt"
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
// Using compressed formats (like MP3 or OGG Opus) reduces memory usage as
// compressed audio is stored until Decode() is called.
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
		text:        text,
		audio:       audioBuf.Bytes(),
		audioFormat: s.handler.format,
		format:      s.format,
	}, nil
}

func (s *doubaoV2Speech) Close() error {
	s.closed = true
	s.segmentIter.Close()
	return nil
}

// doubaoV2SpeechSegment implements the SpeechSegment interface.
type doubaoV2SpeechSegment struct {
	text        string
	audio       []byte
	audioFormat string     // Original audio format (pcm, mp3, ogg_opus)
	format      pcm.Format // Target PCM format
}

var _ SpeechSegment = (*doubaoV2SpeechSegment)(nil)

func (seg *doubaoV2SpeechSegment) Decode(best pcm.Format) VoiceSegment {
	switch seg.audioFormat {
	case "mp3":
		// Return streaming MP3 decoder
		decoder := mp3.NewDecoder(bytes.NewReader(seg.audio))
		return &streamingVoiceSegmentV2{
			reader: decoder,
			closer: decoder,
			format: seg.format,
		}

	case "ogg_opus":
		// Return streaming OGG Opus decoder
		decoder, err := newOggOpusDecoderV2(bytes.NewReader(seg.audio), seg.format)
		if err != nil {
			return &doubaoV2VoiceSegment{
				audio:  nil,
				format: seg.format,
				decErr: fmt.Errorf("create ogg decoder failed: %w", err),
			}
		}
		return &streamingVoiceSegmentV2{
			reader: decoder,
			closer: decoder,
			format: seg.format,
		}

	case "pcm", "":
		// Already PCM, wrap in simple reader
		return &doubaoV2VoiceSegment{
			audio:  seg.audio,
			format: seg.format,
		}

	default:
		// Unsupported format
		return &doubaoV2VoiceSegment{
			audio:  nil,
			format: seg.format,
			decErr: fmt.Errorf("unsupported audio format: %s", seg.audioFormat),
		}
	}
}

// oggOpusDecoderV2 is a streaming OGG Opus to PCM decoder.
type oggOpusDecoderV2 struct {
	oggReader *opusrt.OggReader
	opusDec   *opus.Decoder
	format    pcm.Format
	buf       []byte
	offset    int
}

func newOggOpusDecoderV2(r io.Reader, format pcm.Format) (*oggOpusDecoderV2, error) {
	oggReader, err := opusrt.NewOggReader(r)
	if err != nil {
		return nil, err
	}

	opusDec, err := opus.NewDecoder(format.SampleRate(), format.Channels())
	if err != nil {
		oggReader.Close()
		return nil, err
	}

	return &oggOpusDecoderV2{
		oggReader: oggReader,
		opusDec:   opusDec,
		format:    format,
	}, nil
}

func (d *oggOpusDecoderV2) Read(p []byte) (n int, err error) {
	// Drain buffered data first
	if d.offset < len(d.buf) {
		n = copy(p, d.buf[d.offset:])
		d.offset += n
		return n, nil
	}

	// Decode next frame
	for {
		frame, _, err := d.oggReader.Frame()
		if err != nil {
			return 0, err
		}

		decoded, err := d.opusDec.Decode(opus.Frame(frame))
		if err != nil {
			return 0, err
		}

		if len(decoded) > 0 {
			d.buf = decoded
			d.offset = 0
			n = copy(p, d.buf)
			d.offset = n
			return n, nil
		}
	}
}

func (d *oggOpusDecoderV2) Close() error {
	d.opusDec.Close()
	return d.oggReader.Close()
}

// streamingVoiceSegmentV2 implements VoiceSegment with streaming decode.
type streamingVoiceSegmentV2 struct {
	reader io.Reader
	closer io.Closer
	format pcm.Format
}

var _ VoiceSegment = (*streamingVoiceSegmentV2)(nil)

func (v *streamingVoiceSegmentV2) Read(p []byte) (n int, err error) {
	return v.reader.Read(p)
}

func (v *streamingVoiceSegmentV2) Format() pcm.Format {
	return v.format
}

func (v *streamingVoiceSegmentV2) Close() error {
	if v.closer != nil {
		return v.closer.Close()
	}
	return nil
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
	decErr error // Decoding error, if any
}

var _ VoiceSegment = (*doubaoV2VoiceSegment)(nil)

func (v *doubaoV2VoiceSegment) Read(p []byte) (n int, err error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Return decoding error if any
	if v.decErr != nil {
		return 0, v.decErr
	}

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
