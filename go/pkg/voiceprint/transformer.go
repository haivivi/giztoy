package voiceprint

import (
	"context"
	"io"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/buffer"
	"github.com/haivivi/giztoy/go/pkg/genx"
)

// VoiceprintTransformer is a [genx.Transformer] that annotates audio streams
// with speaker identification labels.
//
// Input type: audio/pcm (PCM16 signed little-endian, 16kHz, mono)
// Output type: audio/pcm (pass-through, with Ctrl.Label set to speaker label)
//
// The transformer accumulates PCM audio and periodically runs the voiceprint
// pipeline (Model → Hasher → Detector). It annotates audio chunks with the
// detected speaker label in Ctrl.Label (e.g., "voice:A3F8").
//
// Non-audio chunks are passed through unchanged.
//
// # EoS Handling
//
//   - When receiving an audio/pcm EoS, process any remaining audio, then emit audio/pcm EoS
//   - Non-audio EoS markers are passed through unchanged
type VoiceprintTransformer struct {
	model    Model
	hasher   *Hasher
	detector *Detector

	// segmentDuration is the analysis window in milliseconds.
	// Default: 400ms (6400 samples at 16kHz = 12800 bytes).
	segmentDuration int

	// sampleRate for PCM input. Default: 16000.
	sampleRate int
}

var _ genx.Transformer = (*VoiceprintTransformer)(nil)

// TransformerOption configures a VoiceprintTransformer.
type TransformerOption func(*VoiceprintTransformer)

// WithSegmentDuration sets the analysis window duration in milliseconds.
// Default: 400.
func WithSegmentDuration(ms int) TransformerOption {
	return func(t *VoiceprintTransformer) {
		if ms > 0 {
			t.segmentDuration = ms
		}
	}
}

// WithSampleRate sets the expected PCM sample rate. Default: 16000.
func WithSampleRate(rate int) TransformerOption {
	return func(t *VoiceprintTransformer) {
		if rate > 0 {
			t.sampleRate = rate
		}
	}
}

// NewTransformer creates a VoiceprintTransformer with the given model, hasher,
// and detector. The model, hasher, and detector must not be nil.
func NewTransformer(model Model, hasher *Hasher, detector *Detector, opts ...TransformerOption) *VoiceprintTransformer {
	t := &VoiceprintTransformer{
		model:           model,
		hasher:          hasher,
		detector:        detector,
		segmentDuration: 400,
		sampleRate:      16000,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Transform implements [genx.Transformer]. It starts a background goroutine
// that reads audio chunks, runs voiceprint detection, and emits annotated
// chunks to the output stream.
func (t *VoiceprintTransformer) Transform(ctx context.Context, _ string, input genx.Stream) (genx.Stream, error) {
	outBuf := buffer.N[*genx.MessageChunk](100)
	out := &vpStream{buf: outBuf}

	go t.transformLoop(ctx, input, outBuf)

	return out, nil
}

// segmentBytes returns the number of PCM bytes per analysis segment.
// PCM16 mono: sampleRate * 2 bytes/sample * duration_ms / 1000.
func (t *VoiceprintTransformer) segmentBytes() int {
	return t.sampleRate * 2 * t.segmentDuration / 1000
}

func (t *VoiceprintTransformer) transformLoop(ctx context.Context, input genx.Stream, out *buffer.Buffer[*genx.MessageChunk]) {
	defer out.CloseWrite()

	var (
		pcmAccum    []byte // accumulated PCM data
		lastLabel   string // last detected speaker label
		segBytes    = t.segmentBytes()
	)

	for {
		select {
		case <-ctx.Done():
			out.CloseWithError(ctx.Err())
			return
		default:
		}

		chunk, err := input.Next()
		if err != nil {
			if err == io.EOF {
				// Process any remaining audio.
				if len(pcmAccum) > 0 {
					lastLabel = t.processSegment(pcmAccum, lastLabel)
				}
				return
			}
			out.CloseWithError(err)
			return
		}

		if chunk == nil {
			continue
		}

		// Handle EoS markers.
		if chunk.IsEndOfStream() {
			blob, ok := chunk.Part.(*genx.Blob)
			if ok && isPCM(blob.MIMEType) {
				// Process remaining audio before EoS.
				if len(pcmAccum) > 0 {
					lastLabel = t.processSegment(pcmAccum, lastLabel)
					pcmAccum = pcmAccum[:0]
				}
				// Emit annotated EoS.
				eos := genx.NewEndOfStream(blob.MIMEType)
				eos.Role = chunk.Role
				eos.Name = chunk.Name
				annotateLabel(eos, lastLabel)
				if err := out.Add(eos); err != nil {
					return
				}
				continue
			}
			// Non-PCM EoS: pass through.
			if err := out.Add(chunk); err != nil {
				return
			}
			continue
		}

		// Check if it's a PCM blob.
		blob, ok := chunk.Part.(*genx.Blob)
		if ok && isPCM(blob.MIMEType) {
			pcmAccum = append(pcmAccum, blob.Data...)

			// Process complete segments.
			for len(pcmAccum) >= segBytes {
				lastLabel = t.processSegment(pcmAccum[:segBytes], lastLabel)
				pcmAccum = pcmAccum[segBytes:]
			}

			// Emit the original chunk with speaker annotation.
			annotateLabel(chunk, lastLabel)
			if err := out.Add(chunk); err != nil {
				return
			}
		} else {
			// Non-PCM chunk: pass through unchanged.
			if err := out.Add(chunk); err != nil {
				return
			}
		}
	}
}

// processSegment runs the voiceprint pipeline on a PCM segment and returns
// the updated speaker label.
func (t *VoiceprintTransformer) processSegment(pcm []byte, currentLabel string) string {
	embedding, err := t.model.Extract(pcm)
	if err != nil {
		// Extraction failed — keep current label.
		return currentLabel
	}

	hash := t.hasher.Hash(embedding)
	result := t.detector.Feed(hash)
	if result == nil {
		return currentLabel
	}

	switch result.Status {
	case StatusSingle:
		return result.Speaker
	case StatusOverlap:
		return result.Speaker // primary speaker
	default:
		return currentLabel // keep previous on unknown
	}
}

// annotateLabel sets the Ctrl.Label on a chunk to the speaker label.
func annotateLabel(chunk *genx.MessageChunk, label string) {
	if label == "" {
		return
	}
	if chunk.Ctrl == nil {
		chunk.Ctrl = &genx.StreamCtrl{}
	}
	chunk.Ctrl.Label = label
}

// isPCM returns true for PCM MIME types.
func isPCM(mime string) bool {
	return mime == "audio/pcm" || strings.HasPrefix(mime, "audio/pcm;")
}

// vpStream wraps a buffer as a genx.Stream.
type vpStream struct {
	buf    *buffer.Buffer[*genx.MessageChunk]
	closed bool
}

func (s *vpStream) Next() (*genx.MessageChunk, error) {
	chunk, err := s.buf.Next()
	if err == buffer.ErrIteratorDone {
		return nil, io.EOF
	}
	return chunk, err
}

func (s *vpStream) Close() error {
	if !s.closed {
		s.closed = true
		s.buf.CloseWrite()
	}
	return nil
}

func (s *vpStream) CloseWithError(err error) error {
	if !s.closed {
		s.closed = true
		s.buf.CloseWithError(err)
	}
	return nil
}
