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

// DoubaoTTSV1Handler is a Doubao TTS 1.0 handler that implements the Synthesizer interface.
// It uses the classic TTS API (/api/v1/tts/stream).
type DoubaoTTSV1Handler struct {
	client      *doubaospeech.Client
	voiceType   string
	cluster     string
	encoding    doubaospeech.AudioEncoding
	speedRatio  float64
	volumeRatio float64
	pitchRatio  float64
	emotion     string
	language    doubaospeech.Language
	segmenter   SentenceSegmenter
}

var _ Synthesizer = (*DoubaoTTSV1Handler)(nil)

// DoubaoTTSV1Option is an option for configuring the DoubaoTTSV1Handler.
type DoubaoTTSV1Option func(*DoubaoTTSV1Handler)

// WithDoubaoTTSV1Voice sets the voice type.
func WithDoubaoTTSV1Voice(voiceType string) DoubaoTTSV1Option {
	return func(h *DoubaoTTSV1Handler) {
		h.voiceType = voiceType
	}
}

// WithDoubaoTTSV1Cluster sets the cluster name.
func WithDoubaoTTSV1Cluster(cluster string) DoubaoTTSV1Option {
	return func(h *DoubaoTTSV1Handler) {
		h.cluster = cluster
	}
}

// WithDoubaoTTSV1Encoding sets the audio encoding (pcm, mp3, ogg_opus, etc.).
// Using compressed formats (like MP3) reduces memory usage as compressed
// audio is stored until Decode() is called.
func WithDoubaoTTSV1Encoding(encoding doubaospeech.AudioEncoding) DoubaoTTSV1Option {
	return func(h *DoubaoTTSV1Handler) {
		h.encoding = encoding
	}
}

// WithDoubaoTTSV1Speed sets the speech speed ratio (0.5-2.0).
func WithDoubaoTTSV1Speed(speedRatio float64) DoubaoTTSV1Option {
	return func(h *DoubaoTTSV1Handler) {
		h.speedRatio = speedRatio
	}
}

// WithDoubaoTTSV1Volume sets the volume ratio (0.1-3.0).
func WithDoubaoTTSV1Volume(volumeRatio float64) DoubaoTTSV1Option {
	return func(h *DoubaoTTSV1Handler) {
		h.volumeRatio = volumeRatio
	}
}

// WithDoubaoTTSV1Pitch sets the pitch ratio (0.1-3.0).
func WithDoubaoTTSV1Pitch(pitchRatio float64) DoubaoTTSV1Option {
	return func(h *DoubaoTTSV1Handler) {
		h.pitchRatio = pitchRatio
	}
}

// WithDoubaoTTSV1Emotion sets the emotion.
func WithDoubaoTTSV1Emotion(emotion string) DoubaoTTSV1Option {
	return func(h *DoubaoTTSV1Handler) {
		h.emotion = emotion
	}
}

// WithDoubaoTTSV1Language sets the language.
func WithDoubaoTTSV1Language(language doubaospeech.Language) DoubaoTTSV1Option {
	return func(h *DoubaoTTSV1Handler) {
		h.language = language
	}
}

// WithDoubaoTTSV1Segmenter sets the sentence segmenter.
func WithDoubaoTTSV1Segmenter(segmenter SentenceSegmenter) DoubaoTTSV1Option {
	return func(h *DoubaoTTSV1Handler) {
		h.segmenter = segmenter
	}
}

// NewDoubaoTTSV1Handler creates a new Doubao TTS 1.0 handler.
func NewDoubaoTTSV1Handler(client *doubaospeech.Client, opts ...DoubaoTTSV1Option) *DoubaoTTSV1Handler {
	h := &DoubaoTTSV1Handler{
		client:      client,
		voiceType:   "zh_female_cancan",
		encoding:    doubaospeech.EncodingPCM,
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
func (h *DoubaoTTSV1Handler) Synthesize(ctx context.Context, name string, textStream io.Reader, format pcm.Format) (Speech, error) {
	segIter, err := h.segmenter.Segment(textStream)
	if err != nil {
		return nil, err
	}

	return &doubaoV1Speech{
		ctx:         ctx,
		handler:     h,
		segmentIter: segIter,
		format:      format,
	}, nil
}

// doubaoV1Speech implements the Speech interface.
type doubaoV1Speech struct {
	ctx         context.Context
	handler     *DoubaoTTSV1Handler
	segmentIter SentenceIterator
	format      pcm.Format
	closed      bool
}

var _ Speech = (*doubaoV1Speech)(nil)

func (s *doubaoV1Speech) Next() (SpeechSegment, error) {
	if s.closed {
		return nil, iterator.Done
	}

	text, err := s.segmentIter.Next()
	if err != nil {
		return nil, err
	}

	// Build TTS request
	req := &doubaospeech.TTSRequest{
		Text:        text,
		VoiceType:   s.handler.voiceType,
		Cluster:     s.handler.cluster,
		Encoding:    s.handler.encoding,
		SampleRate:  doubaospeech.SampleRate(s.format.SampleRate()),
		SpeedRatio:  s.handler.speedRatio,
		VolumeRatio: s.handler.volumeRatio,
		PitchRatio:  s.handler.pitchRatio,
		Emotion:     s.handler.emotion,
		Language:    s.handler.language,
	}

	// Collect streaming audio
	var audioBuf bytes.Buffer
	for chunk, err := range s.handler.client.TTS.SynthesizeStream(s.ctx, req) {
		if err != nil {
			return nil, err
		}
		if chunk.Audio != nil {
			audioBuf.Write(chunk.Audio)
		}
	}

	return &doubaoV1SpeechSegment{
		text:     text,
		audio:    audioBuf.Bytes(),
		encoding: s.handler.encoding,
		format:   s.format,
	}, nil
}

func (s *doubaoV1Speech) Close() error {
	s.closed = true
	s.segmentIter.Close()
	return nil
}

// doubaoV1SpeechSegment implements the SpeechSegment interface.
type doubaoV1SpeechSegment struct {
	text     string
	audio    []byte
	encoding doubaospeech.AudioEncoding // Original audio encoding
	format   pcm.Format                 // Target PCM format
}

var _ SpeechSegment = (*doubaoV1SpeechSegment)(nil)

func (seg *doubaoV1SpeechSegment) Decode(best pcm.Format) VoiceSegment {
	switch seg.encoding {
	case doubaospeech.EncodingMP3:
		// Return streaming MP3 decoder
		decoder := mp3.NewDecoder(bytes.NewReader(seg.audio))
		return &streamingVoiceSegmentV1{
			reader: decoder,
			closer: decoder,
			format: seg.format,
		}

	case doubaospeech.EncodingOGG:
		// Return streaming OGG Opus decoder
		decoder, err := newOggOpusDecoder(bytes.NewReader(seg.audio), seg.format)
		if err != nil {
			return &doubaoV1VoiceSegment{
				audio:  nil,
				format: seg.format,
				decErr: fmt.Errorf("create ogg decoder failed: %w", err),
			}
		}
		return &streamingVoiceSegmentV1{
			reader: decoder,
			closer: decoder,
			format: seg.format,
		}

	case doubaospeech.EncodingPCM, doubaospeech.EncodingPCMS16LE, "":
		// Already PCM, wrap in simple reader
		return &doubaoV1VoiceSegment{
			audio:  seg.audio,
			format: seg.format,
		}

	default:
		// Unsupported format
		return &doubaoV1VoiceSegment{
			audio:  nil,
			format: seg.format,
			decErr: fmt.Errorf("unsupported audio encoding: %s", seg.encoding),
		}
	}
}

// oggOpusDecoder is a streaming OGG Opus to PCM decoder.
// It decodes on-demand during Read() calls, minimizing memory usage.
type oggOpusDecoder struct {
	oggReader *opusrt.OggReader
	opusDec   *opus.Decoder
	format    pcm.Format

	// Buffer for decoded PCM data (holds one frame at a time)
	buf    []byte
	offset int
}

func newOggOpusDecoder(r io.Reader, format pcm.Format) (*oggOpusDecoder, error) {
	oggReader, err := opusrt.NewOggReader(r)
	if err != nil {
		return nil, err
	}

	opusDec, err := opus.NewDecoder(format.SampleRate(), format.Channels())
	if err != nil {
		oggReader.Close()
		return nil, err
	}

	return &oggOpusDecoder{
		oggReader: oggReader,
		opusDec:   opusDec,
		format:    format,
	}, nil
}

func (d *oggOpusDecoder) Read(p []byte) (n int, err error) {
	// First, drain any buffered data from previous frame
	if d.offset < len(d.buf) {
		n = copy(p, d.buf[d.offset:])
		d.offset += n
		return n, nil
	}

	// Need to decode next frame
	for {
		frame, _, err := d.oggReader.Frame()
		if err != nil {
			return 0, err // includes io.EOF
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
		// Empty frame, try next
	}
}

func (d *oggOpusDecoder) Close() error {
	d.opusDec.Close()
	return d.oggReader.Close()
}

// streamingVoiceSegmentV1 implements VoiceSegment with streaming decode.
type streamingVoiceSegmentV1 struct {
	reader io.Reader
	closer io.Closer
	format pcm.Format
}

var _ VoiceSegment = (*streamingVoiceSegmentV1)(nil)

func (v *streamingVoiceSegmentV1) Read(p []byte) (n int, err error) {
	return v.reader.Read(p)
}

func (v *streamingVoiceSegmentV1) Format() pcm.Format {
	return v.format
}

func (v *streamingVoiceSegmentV1) Close() error {
	if v.closer != nil {
		return v.closer.Close()
	}
	return nil
}

func (seg *doubaoV1SpeechSegment) Transcribe() io.ReadCloser {
	return io.NopCloser(bytes.NewReader([]byte(seg.text)))
}

func (seg *doubaoV1SpeechSegment) Close() error {
	return nil
}

// doubaoV1VoiceSegment implements the VoiceSegment interface.
type doubaoV1VoiceSegment struct {
	mu     sync.Mutex
	audio  []byte
	offset int
	format pcm.Format
	decErr error // Decoding error, if any
}

var _ VoiceSegment = (*doubaoV1VoiceSegment)(nil)

func (v *doubaoV1VoiceSegment) Read(p []byte) (n int, err error) {
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

func (v *doubaoV1VoiceSegment) Format() pcm.Format {
	return v.format
}

func (v *doubaoV1VoiceSegment) Close() error {
	return nil
}
