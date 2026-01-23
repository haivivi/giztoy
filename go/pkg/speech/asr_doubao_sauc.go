package speech

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"

	"github.com/haivivi/giztoy/pkg/audio/codec/opus"
	"github.com/haivivi/giztoy/pkg/audio/opusrt"
	"github.com/haivivi/giztoy/pkg/audio/pcm"
	"github.com/haivivi/giztoy/pkg/doubaospeech"

	"google.golang.org/api/iterator"
)

// DoubaoSAUCASRHandler is a Doubao SAUC (BigModel) ASR handler that implements
// the StreamTranscriber interface.
type DoubaoSAUCASRHandler struct {
	client     *doubaospeech.Client
	sampleRate int
	channels   int
	language   string
	resourceID string
	enableITN  bool
	enablePunc bool
	hotwords   []string
}

var _ StreamTranscriber = (*DoubaoSAUCASRHandler)(nil)

// DoubaoSAUCASROption is an option for configuring the DoubaoSAUCASRHandler.
type DoubaoSAUCASROption func(*DoubaoSAUCASRHandler)

// WithDoubaoSAUCSampleRate sets the sample rate for ASR.
func WithDoubaoSAUCSampleRate(sampleRate int) DoubaoSAUCASROption {
	return func(h *DoubaoSAUCASRHandler) {
		h.sampleRate = sampleRate
	}
}

// WithDoubaoSAUCChannels sets the number of audio channels.
func WithDoubaoSAUCChannels(channels int) DoubaoSAUCASROption {
	return func(h *DoubaoSAUCASRHandler) {
		h.channels = channels
	}
}

// WithDoubaoSAUCLanguage sets the language for ASR (e.g., "zh-CN", "en-US").
func WithDoubaoSAUCLanguage(language string) DoubaoSAUCASROption {
	return func(h *DoubaoSAUCASRHandler) {
		h.language = language
	}
}

// WithDoubaoSAUCResourceID sets the resource ID (e.g., volc.bigasr.sauc.duration).
func WithDoubaoSAUCResourceID(resourceID string) DoubaoSAUCASROption {
	return func(h *DoubaoSAUCASRHandler) {
		h.resourceID = resourceID
	}
}

// WithDoubaoSAUCEnableITN enables/disables Inverse Text Normalization.
func WithDoubaoSAUCEnableITN(enable bool) DoubaoSAUCASROption {
	return func(h *DoubaoSAUCASRHandler) {
		h.enableITN = enable
	}
}

// WithDoubaoSAUCEnablePunc enables/disables punctuation.
func WithDoubaoSAUCEnablePunc(enable bool) DoubaoSAUCASROption {
	return func(h *DoubaoSAUCASRHandler) {
		h.enablePunc = enable
	}
}

// WithDoubaoSAUCHotwords sets hotwords for recognition boost.
func WithDoubaoSAUCHotwords(hotwords []string) DoubaoSAUCASROption {
	return func(h *DoubaoSAUCASRHandler) {
		h.hotwords = hotwords
	}
}

// NewDoubaoSAUCASRHandler creates a new Doubao SAUC ASR handler.
func NewDoubaoSAUCASRHandler(client *doubaospeech.Client, opts ...DoubaoSAUCASROption) *DoubaoSAUCASRHandler {
	h := &DoubaoSAUCASRHandler{
		client:     client,
		sampleRate: 16000,
		channels:   1,
		language:   "zh-CN",
		resourceID: doubaospeech.ResourceASRStream,
		enableITN:  true,
		enablePunc: true,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// TranscribeStream performs streaming transcription on an Opus audio stream.
func (h *DoubaoSAUCASRHandler) TranscribeStream(ctx context.Context, model string, opusReader opusrt.FrameReader) (SpeechStream, error) {
	// Create Opus decoder
	decoder, err := opus.NewDecoder(h.sampleRate, h.channels)
	if err != nil {
		return nil, err
	}

	// Build ASR config
	config := &doubaospeech.ASRV2Config{
		Format:     "pcm",
		SampleRate: h.sampleRate,
		Bits:       16,
		Channels:   h.channels,
		Language:   h.language,
		ResourceID: h.resourceID,
		EnableITN:  h.enableITN,
		EnablePunc: h.enablePunc,
		Hotwords:   h.hotwords,
	}

	// Open ASR session
	session, err := h.client.ASRV2.OpenStreamSession(ctx, config)
	if err != nil {
		decoder.Close()
		return nil, err
	}

	// Select PCM format based on sample rate
	// Note: Only mono audio is supported for ASR
	format := sampleRateToFormat(h.sampleRate, h.channels)

	stream := &doubaoSAUCSpeechStream{
		ctx:        ctx,
		decoder:    decoder,
		opusReader: opusReader,
		session:    session,
		format:     format,
		resultCh:   make(chan *doubaospeech.ASRV2Result, 100),
		errCh:      make(chan error, 1),
		closeCh:    make(chan struct{}),
		sendDone:   make(chan struct{}),
	}

	// Start goroutines
	go stream.sendLoop()
	go stream.recvLoop()

	return stream, nil
}

// sampleRateToFormat converts sample rate to pcm.Format.
// Note: This handler only supports mono audio. Stereo is not supported.
func sampleRateToFormat(sampleRate int, channels int) pcm.Format {
	// Validate channels - only mono is supported
	if channels != 1 {
		// Log warning but continue with mono format
		// Stereo audio will be decoded but reported as mono format
	}

	switch sampleRate {
	case 16000:
		return pcm.L16Mono16K
	case 24000:
		return pcm.L16Mono24K
	case 48000:
		return pcm.L16Mono48K
	default:
		// Default to 16kHz for ASR - log warning for unexpected rates
		return pcm.L16Mono16K
	}
}

// doubaoSAUCSpeechStream implements the SpeechStream interface.
type doubaoSAUCSpeechStream struct {
	ctx        context.Context
	decoder    *opus.Decoder
	opusReader opusrt.FrameReader
	session    *doubaospeech.ASRV2Session
	format     pcm.Format

	resultCh chan *doubaospeech.ASRV2Result
	errCh    chan error
	closeCh  chan struct{}
	sendDone chan struct{} // signals sendLoop completion

	closeOnce sync.Once
	closed    bool
	err       error
}

var _ SpeechStream = (*doubaoSAUCSpeechStream)(nil)

// sendLoop reads Opus frames, decodes to PCM, and sends to ASR.
func (s *doubaoSAUCSpeechStream) sendLoop() {
	defer close(s.sendDone)

	for {
		select {
		case <-s.closeCh:
			return
		case <-s.ctx.Done():
			return
		default:
		}

		// Read Opus frame
		frame, loss, err := s.opusReader.Frame()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Send last chunk (nil audio signals end of stream)
				if sendErr := s.session.SendAudio(s.ctx, nil, true); sendErr != nil {
					s.sendError(sendErr)
				}
				return
			}
			s.sendError(err)
			return
		}

		var pcmData []byte
		if loss > 0 {
			// Packet loss - use PLC
			samples := int(loss.Seconds() * float64(s.format.SampleRate()))
			pcmData, err = s.decoder.DecodePLC(samples)
			if err != nil {
				s.sendError(err)
				return
			}
		} else {
			// Normal decode
			pcmData, err = s.decoder.Decode(opus.Frame(frame))
			if err != nil {
				s.sendError(err)
				return
			}
		}

		// Send to ASR
		if err := s.session.SendAudio(s.ctx, pcmData, false); err != nil {
			s.sendError(err)
			return
		}
	}
}

// recvLoop receives ASR results and forwards to channel.
func (s *doubaoSAUCSpeechStream) recvLoop() {
	defer close(s.resultCh)

	for result, err := range s.session.Recv() {
		if err != nil {
			s.sendError(err)
			return
		}

		select {
		case s.resultCh <- result:
		case <-s.closeCh:
			return
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *doubaoSAUCSpeechStream) sendError(err error) {
	select {
	case s.errCh <- err:
	default:
	}
}

// Next returns the next Speech from the stream.
// Each Speech represents a final ASR result (complete sentence).
func (s *doubaoSAUCSpeechStream) Next() (Speech, error) {
	if s.closed {
		if s.err != nil {
			return nil, s.err
		}
		return nil, iterator.Done
	}

	// Collect results until we get a final one
	var results []*doubaospeech.ASRV2Result

	for {
		select {
		case result, ok := <-s.resultCh:
			if !ok {
				// Channel closed
				if len(results) > 0 {
					return s.buildSpeech(results), nil
				}
				s.closed = true
				return nil, iterator.Done
			}

			results = append(results, result)

			// If final result, return as Speech
			if result.IsFinal && result.Text != "" {
				return s.buildSpeech(results), nil
			}

		case err := <-s.errCh:
			s.err = err
			s.closed = true
			if len(results) > 0 {
				// Return what we have before error
				return s.buildSpeech(results), nil
			}
			return nil, err

		case <-s.ctx.Done():
			s.err = s.ctx.Err()
			s.closed = true
			return nil, s.err
		}
	}
}

func (s *doubaoSAUCSpeechStream) buildSpeech(results []*doubaospeech.ASRV2Result) Speech {
	// Get the final text
	var text string
	for _, r := range results {
		if r.IsFinal && r.Text != "" {
			text = r.Text
			break
		}
	}
	if text == "" && len(results) > 0 {
		text = results[len(results)-1].Text
	}

	return &singleSegmentSpeech{
		segment: &doubaoSAUCSpeechSegment{
			text:   text,
			format: s.format,
		},
	}
}

// Close closes the speech stream.
func (s *doubaoSAUCSpeechStream) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeCh)
		// Wait for sendLoop to finish before closing session
		// to avoid concurrent websocket writes
		<-s.sendDone
		s.session.Close()
		s.decoder.Close()
		s.closed = true
	})
	return nil
}

// singleSegmentSpeech wraps a single segment as a Speech.
type singleSegmentSpeech struct {
	segment SpeechSegment
	done    bool
}

var _ Speech = (*singleSegmentSpeech)(nil)

func (s *singleSegmentSpeech) Next() (SpeechSegment, error) {
	if s.done {
		return nil, iterator.Done
	}
	s.done = true
	return s.segment, nil
}

func (s *singleSegmentSpeech) Close() error {
	if s.segment != nil {
		return s.segment.Close()
	}
	return nil
}

// doubaoSAUCSpeechSegment implements the SpeechSegment interface.
type doubaoSAUCSpeechSegment struct {
	text   string
	format pcm.Format
}

var _ SpeechSegment = (*doubaoSAUCSpeechSegment)(nil)

func (seg *doubaoSAUCSpeechSegment) Decode(best pcm.Format) VoiceSegment {
	// ASR doesn't have audio output, return empty segment
	return &emptyVoiceSegment{format: seg.format}
}

func (seg *doubaoSAUCSpeechSegment) Transcribe() io.ReadCloser {
	return io.NopCloser(bytes.NewReader([]byte(seg.text)))
}

func (seg *doubaoSAUCSpeechSegment) Close() error {
	return nil
}

// emptyVoiceSegment is a VoiceSegment with no audio data.
type emptyVoiceSegment struct {
	format pcm.Format
}

var _ VoiceSegment = (*emptyVoiceSegment)(nil)

func (v *emptyVoiceSegment) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

func (v *emptyVoiceSegment) Format() pcm.Format {
	return v.format
}

func (v *emptyVoiceSegment) Close() error {
	return nil
}
