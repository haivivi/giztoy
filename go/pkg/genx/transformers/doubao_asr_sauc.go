package transformers

import (
	"context"
	"io"

	"github.com/haivivi/giztoy/go/pkg/doubaospeech"
	"github.com/haivivi/giztoy/go/pkg/genx"
)

// DoubaoASRSAUC is an ASR transformer using Doubao BigModel ASR (大模型语音识别).
//
// Resource ID: volc.bigasr.sauc.duration
//
// Input type: audio/* (audio/ogg, audio/pcm, etc.)
// Output type: text/plain
//
// EoS Handling:
//   - When receiving an audio/* EoS marker, finish current ASR, emit results, then emit text/plain EoS
//   - Non-audio chunks are passed through unchanged
//
// Note: The input audio format must match the configured format.
type DoubaoASRSAUC struct {
	client     *doubaospeech.Client
	format     string
	sampleRate int
	channels   int
	bits       int
	language   string
	enableITN  bool
	enablePunc bool
	hotwords   []string
	resultType string // "single" (default) or "full"
}

var _ genx.Transformer = (*DoubaoASRSAUC)(nil)

// DoubaoASRSAUCOption is a functional option for DoubaoASRSAUC.
type DoubaoASRSAUCOption func(*DoubaoASRSAUC)

// WithDoubaoASRSAUCFormat sets the audio format (pcm, wav, mp3, ogg_opus).
func WithDoubaoASRSAUCFormat(format string) DoubaoASRSAUCOption {
	return func(t *DoubaoASRSAUC) {
		t.format = format
	}
}

// WithDoubaoASRSAUCSampleRate sets the sample rate (8000, 16000, etc.).
func WithDoubaoASRSAUCSampleRate(sampleRate int) DoubaoASRSAUCOption {
	return func(t *DoubaoASRSAUC) {
		t.sampleRate = sampleRate
	}
}

// WithDoubaoASRSAUCChannels sets the number of channels (1 or 2).
func WithDoubaoASRSAUCChannels(channels int) DoubaoASRSAUCOption {
	return func(t *DoubaoASRSAUC) {
		t.channels = channels
	}
}

// WithDoubaoASRSAUCBits sets the bits per sample (16, etc.).
func WithDoubaoASRSAUCBits(bits int) DoubaoASRSAUCOption {
	return func(t *DoubaoASRSAUC) {
		t.bits = bits
	}
}

// WithDoubaoASRSAUCLanguage sets the language (zh-CN, en-US, ja-JP, etc.).
func WithDoubaoASRSAUCLanguage(language string) DoubaoASRSAUCOption {
	return func(t *DoubaoASRSAUC) {
		t.language = language
	}
}

// WithDoubaoASRSAUCEnableITN enables Inverse Text Normalization.
func WithDoubaoASRSAUCEnableITN(enable bool) DoubaoASRSAUCOption {
	return func(t *DoubaoASRSAUC) {
		t.enableITN = enable
	}
}

// WithDoubaoASRSAUCEnablePunc enables punctuation.
func WithDoubaoASRSAUCEnablePunc(enable bool) DoubaoASRSAUCOption {
	return func(t *DoubaoASRSAUC) {
		t.enablePunc = enable
	}
}

// WithDoubaoASRSAUCHotwords sets hotwords for recognition boost.
func WithDoubaoASRSAUCHotwords(hotwords []string) DoubaoASRSAUCOption {
	return func(t *DoubaoASRSAUC) {
		t.hotwords = hotwords
	}
}

// WithDoubaoASRSAUCResultType sets the result type.
// Options: "single" (default, only definite results), "full" (all results including interim).
func WithDoubaoASRSAUCResultType(resultType string) DoubaoASRSAUCOption {
	return func(t *DoubaoASRSAUC) {
		t.resultType = resultType
	}
}

// NewDoubaoASRSAUC creates a new DoubaoASRSAUC transformer.
//
// Parameters:
//   - client: Doubao speech client
//   - opts: Optional configuration
func NewDoubaoASRSAUC(client *doubaospeech.Client, opts ...DoubaoASRSAUCOption) *DoubaoASRSAUC {
	t := &DoubaoASRSAUC{
		client:     client,
		format:     "ogg", // Use "ogg" instead of "ogg_opus" for compatibility
		sampleRate: 16000,
		channels:   1,
		bits:       16,
		language:   "zh-CN",
		enableITN:  true,
		enablePunc: true,
		resultType: "single", // only definite results
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// DoubaoASRSAUCCtxKey is the context key for runtime options.
type doubaoASRSAUCCtxKey struct{}

// DoubaoASRSAUCCtxOptions are runtime options passed via context.
// TODO: Add fields as needed for runtime configuration.
type DoubaoASRSAUCCtxOptions struct{}

// WithDoubaoASRSAUCCtxOptions attaches runtime options to context.
func WithDoubaoASRSAUCCtxOptions(ctx context.Context, opts DoubaoASRSAUCCtxOptions) context.Context {
	return context.WithValue(ctx, doubaoASRSAUCCtxKey{}, opts)
}

// Transform converts audio Blob chunks to Text chunks.
// DoubaoASRSAUC creates sessions on demand, so it returns immediately.
// The ctx is unused (session creation happens lazily in the loop);
// the goroutine lifetime is governed by the input Stream.
func (t *DoubaoASRSAUC) Transform(_ context.Context, _ string, input genx.Stream) (genx.Stream, error) {
	output := newBufferStream(100)

	go t.transformLoop(input, output)

	return output, nil
}

func (t *DoubaoASRSAUC) transformLoop(input genx.Stream, output *bufferStream) {
	defer output.Close()

	// Track last chunk for metadata
	var lastChunk *genx.MessageChunk
	var session *doubaospeech.ASRV2Session
	var resultsCh chan *genx.MessageChunk
	var resultsDone chan error

	// Helper to start a new ASR session
	startSession := func() error {
		var err error
		session, err = t.openSession(context.Background())
		if err != nil {
			return err
		}
		resultsCh = make(chan *genx.MessageChunk, 100)
		resultsDone = make(chan error, 1)
		go t.receiveResults(session, lastChunk, resultsCh, resultsDone)
		// Forward results to output as they arrive
		go func() {
			for chunk := range resultsCh {
				output.Push(chunk)
			}
		}()
		return nil
	}

	// Helper to finish current session
	finishSession := func() error {
		if session == nil {
			return nil
		}
		session.SendAudio(context.Background(), nil, true)
		err := <-resultsDone
		session.Close()
		session = nil
		return err
	}

	// Process input stream
	for {

		chunk, err := input.Next()
		if err != nil {
			if err != io.EOF {
				if session != nil {
					session.Close()
				}
				output.CloseWithError(err)
				return
			}
			// EOF: finish current session
			if err := finishSession(); err != nil {
				output.CloseWithError(err)
				return
			}
			return
		}

		if chunk == nil {
			continue
		}

		lastChunk = chunk

		// Check for EoS marker with audio MIME type
		if chunk.IsEndOfStream() {
			if blob, ok := chunk.Part.(*genx.Blob); ok && isAudioMIME(blob.MIMEType) {
				// Audio EoS: finish current session, emit text EoS
				if err := finishSession(); err != nil {
					output.CloseWithError(err)
					return
				}
				eosChunk := genx.NewTextEndOfStream()
				eosChunk.Role = lastChunk.Role
				eosChunk.Name = lastChunk.Name
				if err := output.Push(eosChunk); err != nil {
					return
				}
				continue
			}
			// Non-audio EoS: pass through
			if err := output.Push(chunk); err != nil {
				return
			}
			continue
		}

		// Handle audio blob
		if blob, ok := chunk.Part.(*genx.Blob); ok && isAudioMIME(blob.MIMEType) {
			// Start session on first audio chunk
			if session == nil {
				if err := startSession(); err != nil {
					output.CloseWithError(err)
					return
				}
			}
			// Send audio to ASR
			if err := session.SendAudio(context.Background(), blob.Data, false); err != nil {
				session.Close()
				output.CloseWithError(err)
				return
			}
		} else {
			// Non-audio chunk: pass through
			if err := output.Push(chunk); err != nil {
				return
			}
		}
	}
}

func (t *DoubaoASRSAUC) openSession(ctx context.Context) (*doubaospeech.ASRV2Session, error) {
	config := &doubaospeech.ASRV2Config{
		Format:     t.format,
		SampleRate: t.sampleRate,
		Channels:   t.channels,
		Bits:       t.bits,
		Language:   t.language,
		EnableITN:  t.enableITN,
		EnablePunc: t.enablePunc,
		Hotwords:   t.hotwords,
		ResultType: t.resultType,
	}
	return t.client.ASRV2.OpenStreamSession(ctx, config)
}

func (t *DoubaoASRSAUC) receiveResults(session *doubaospeech.ASRV2Session, lastChunk *genx.MessageChunk, resultsCh chan<- *genx.MessageChunk, done chan<- error) {
	defer close(resultsCh)

	// Track processed utterances by end time to avoid duplicates
	lastEndTime := 0

	for result, err := range session.Recv() {
		if err != nil {
			done <- err
			return
		}

		// Process definite utterances from the utterances array
		if len(result.Utterances) > 0 {
			for _, utt := range result.Utterances {
				if utt.Definite && utt.EndTime > lastEndTime && utt.Text != "" {
					outChunk := &genx.MessageChunk{
						Part: genx.Text(utt.Text),
					}
					if lastChunk != nil {
						outChunk.Role = lastChunk.Role
						outChunk.Name = lastChunk.Name
					}
					resultsCh <- outChunk
					lastEndTime = utt.EndTime
				}
			}
		} else if result.IsFinal && result.Text != "" {
			outChunk := &genx.MessageChunk{
				Part: genx.Text(result.Text),
			}
			if lastChunk != nil {
				outChunk.Role = lastChunk.Role
				outChunk.Name = lastChunk.Name
			}
			resultsCh <- outChunk
		}
	}
	done <- nil
}

// isAudioMIME checks if a MIME type is audio
func isAudioMIME(mimeType string) bool {
	return len(mimeType) >= 6 && mimeType[:6] == "audio/"
}
