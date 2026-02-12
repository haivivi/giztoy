package transformers

import (
	"context"
	"io"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/voiceprint"
)

// Voiceprint is a transformer that annotates audio streams with speaker
// identification labels.
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
// EoS Handling:
//   - When receiving an audio/pcm EoS, process any remaining audio, then emit audio/pcm EoS
//   - Non-audio EoS markers are passed through unchanged
type Voiceprint struct {
	model    voiceprint.Model
	hasher   *voiceprint.Hasher

	// detectorOpts are used to create a fresh Detector per Transform call.
	// Each pipeline gets its own Detector to avoid concurrent write races.
	detectorOpts []voiceprint.DetectorOption

	segmentDuration int // analysis window in milliseconds (default 400)
	sampleRate      int // PCM sample rate (default 16000)
}

var _ genx.Transformer = (*Voiceprint)(nil)

// VoiceprintOption configures a Voiceprint transformer.
type VoiceprintOption func(*Voiceprint)

// WithVoiceprintSegmentDuration sets the analysis window duration in milliseconds.
func WithVoiceprintSegmentDuration(ms int) VoiceprintOption {
	return func(t *Voiceprint) {
		if ms > 0 {
			t.segmentDuration = ms
		}
	}
}

// WithVoiceprintSampleRate sets the expected PCM sample rate.
func WithVoiceprintSampleRate(rate int) VoiceprintOption {
	return func(t *Voiceprint) {
		if rate > 0 {
			t.sampleRate = rate
		}
	}
}

// NewVoiceprint creates a Voiceprint transformer.
//
// Parameters:
//   - model: speaker embedding extractor (e.g., NCNNModel)
//   - hasher: LSH hasher for embedding → hash
//   - detectorOpts: options for creating per-pipeline Detector instances
func NewVoiceprint(model voiceprint.Model, hasher *voiceprint.Hasher, detectorOpts []voiceprint.DetectorOption, opts ...VoiceprintOption) *Voiceprint {
	t := &Voiceprint{
		model:           model,
		hasher:          hasher,
		detectorOpts:    detectorOpts,
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
func (t *Voiceprint) Transform(ctx context.Context, _ string, input genx.Stream) (genx.Stream, error) {
	output := newBufferStream(100)

	// Create a fresh Detector per pipeline to avoid concurrent write races.
	detector := voiceprint.NewDetector(t.detectorOpts...)

	go t.transformLoop(ctx, input, output, detector)

	return output, nil
}

// segmentBytes returns the number of PCM bytes per analysis segment.
func (t *Voiceprint) segmentBytes() int {
	return t.sampleRate * 2 * t.segmentDuration / 1000
}

func (t *Voiceprint) transformLoop(ctx context.Context, input genx.Stream, output *bufferStream, detector *voiceprint.Detector) {
	defer output.Close()

	var (
		pcmAccum  []byte
		lastLabel string
		segBytes  = t.segmentBytes()
	)

	for {
		select {
		case <-ctx.Done():
			output.CloseWithError(ctx.Err())
			return
		default:
		}

		chunk, err := input.Next()
		if err != nil {
			if err == io.EOF {
				if len(pcmAccum) > 0 {
			lastLabel = t.processSegment(pcmAccum, lastLabel, detector)
				_ = lastLabel // processed but stream ends
				}
				return
			}
			output.CloseWithError(err)
			return
		}

		if chunk == nil {
			continue
		}

		// Handle EoS markers.
		if chunk.IsEndOfStream() {
			if blob, ok := chunk.Part.(*genx.Blob); ok && isPCMMIME(blob.MIMEType) {
				if len(pcmAccum) > 0 {
			lastLabel = t.processSegment(pcmAccum, lastLabel, detector)
				pcmAccum = pcmAccum[:0]
				}
				eos := genx.NewEndOfStream(blob.MIMEType)
				eos.Role = chunk.Role
				eos.Name = chunk.Name
				annotateLabel(eos, lastLabel)
				if err := output.Push(eos); err != nil {
					return
				}
				continue
			}
			if err := output.Push(chunk); err != nil {
				return
			}
			continue
		}

		// Handle PCM audio blobs.
		if blob, ok := chunk.Part.(*genx.Blob); ok && isPCMMIME(blob.MIMEType) {
			pcmAccum = append(pcmAccum, blob.Data...)

			for len(pcmAccum) >= segBytes {
				lastLabel = t.processSegment(pcmAccum[:segBytes], lastLabel, detector)
				pcmAccum = pcmAccum[segBytes:]
			}

			annotateLabel(chunk, lastLabel)
			if err := output.Push(chunk); err != nil {
				return
			}
		} else {
			if err := output.Push(chunk); err != nil {
				return
			}
		}
	}
}

func (t *Voiceprint) processSegment(pcm []byte, currentLabel string, detector *voiceprint.Detector) string {
	embedding, err := t.model.Extract(pcm)
	if err != nil {
		return currentLabel
	}

	hash := t.hasher.Hash(embedding)
	result := detector.Feed(hash)
	if result == nil {
		return currentLabel
	}

	switch result.Status {
	case voiceprint.StatusSingle:
		return result.Speaker
	case voiceprint.StatusOverlap:
		return result.Speaker
	default:
		return currentLabel
	}
}

func annotateLabel(chunk *genx.MessageChunk, label string) {
	if label == "" {
		return
	}
	if chunk.Ctrl == nil {
		chunk.Ctrl = &genx.StreamCtrl{}
	}
	chunk.Ctrl.Label = label
}

func isPCMMIME(mime string) bool {
	return mime == "audio/pcm" || strings.HasPrefix(mime, "audio/pcm;")
}
