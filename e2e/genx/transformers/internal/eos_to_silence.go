// Package internal provides shared utilities for transformer examples.
package internal

import (
	"bytes"
	"context"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/ogg"
	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
	"github.com/haivivi/giztoy/go/pkg/genx"
)

// EOSToSilence converts audio EOS markers to silence audio.
// This simulates natural pauses in speech for VAD testing.
//
// When receiving an audio/* EOS marker, it generates silence audio
// of the configured duration in the same format.
// All other chunks are passed through unchanged.
type EOSToSilence struct {
	duration   time.Duration
	sampleRate int
	channels   int
}

// NewEOSToSilence creates a new EOSToSilence transformer.
//
// Parameters:
//   - duration: Duration of silence to generate (e.g., 2*time.Second)
//   - sampleRate: Audio sample rate (e.g., 24000)
//   - channels: Number of audio channels (usually 1)
func NewEOSToSilence(duration time.Duration, sampleRate, channels int) *EOSToSilence {
	return &EOSToSilence{
		duration:   duration,
		sampleRate: sampleRate,
		channels:   channels,
	}
}

// Transform converts audio EOS markers to silence audio.
// EOSToSilence does not require connection setup, so it returns immediately.
func (t *EOSToSilence) Transform(ctx context.Context, input genx.Stream) (genx.Stream, error) {
	return &eosToSilenceStream{
		input:       input,
		transformer: t,
		ctx:         ctx,
	}, nil
}

type eosToSilenceStream struct {
	input       genx.Stream
	transformer *EOSToSilence
	ctx         context.Context
	pending     *genx.MessageChunk // buffered silence chunk to emit
}

func (s *eosToSilenceStream) Next() (*genx.MessageChunk, error) {
	// Return pending silence chunk if any
	if s.pending != nil {
		chunk := s.pending
		s.pending = nil
		return chunk, nil
	}

	for {
		select {
		case <-s.ctx.Done():
			return nil, s.ctx.Err()
		default:
		}

		chunk, err := s.input.Next()
		if err != nil {
			return nil, err
		}

		if chunk == nil {
			continue
		}

		// Check for audio EOS marker
		if chunk.IsEndOfStream() {
			if blob, ok := chunk.Part.(*genx.Blob); ok && IsAudioMIME(blob.MIMEType) {
				// Generate silence audio in the same format
				silenceData, err := s.generateSilence(blob.MIMEType)
				if err != nil {
					return nil, err
				}

				// Return silence chunk (EOS is consumed, not passed through)
				return &genx.MessageChunk{
					Role: chunk.Role,
					Name: chunk.Name,
					Part: &genx.Blob{
						MIMEType: blob.MIMEType,
						Data:     silenceData,
					},
				}, nil
			}
		}

		// Pass through all other chunks
		return chunk, nil
	}
}

func (s *eosToSilenceStream) Close() error {
	return s.input.Close()
}

func (s *eosToSilenceStream) CloseWithError(err error) error {
	return s.input.CloseWithError(err)
}

// generateSilence generates silence audio data for the given MIME type.
func (s *eosToSilenceStream) generateSilence(mimeType string) ([]byte, error) {
	switch mimeType {
	case "audio/pcm":
		return s.generatePCMSilence(), nil
	case "audio/ogg":
		return s.generateOggSilence()
	default:
		// Default to PCM silence
		return s.generatePCMSilence(), nil
	}
}

// generatePCMSilence generates PCM16 silence (zeros).
func (s *eosToSilenceStream) generatePCMSilence() []byte {
	// PCM16: 2 bytes per sample per channel
	bytesPerSecond := s.transformer.sampleRate * s.transformer.channels * 2
	totalBytes := int(s.transformer.duration.Seconds() * float64(bytesPerSecond))
	return make([]byte, totalBytes)
}

// generateOggSilence generates OGG/Opus silence.
func (s *eosToSilenceStream) generateOggSilence() ([]byte, error) {
	sampleRate := s.transformer.sampleRate
	channels := s.transformer.channels
	duration := s.transformer.duration

	// Create Opus encoder
	enc, err := opus.NewVoIPEncoder(sampleRate, channels)
	if err != nil {
		return nil, err
	}
	defer enc.Close()

	// Create OGG writer
	var buf bytes.Buffer
	oggWriter, err := ogg.NewOpusWriter(&buf, sampleRate, channels)
	if err != nil {
		return nil, err
	}

	// Generate silence frames (20ms each)
	frameDuration := 20 * time.Millisecond
	frameSize := sampleRate * int(frameDuration.Milliseconds()) / 1000
	bytesPerFrame := frameSize * channels * 2

	silencePCM := make([]byte, bytesPerFrame) // zeros = silence

	totalFrames := int(duration / frameDuration)
	for i := 0; i < totalFrames; i++ {
		frame, err := enc.EncodeBytes(silencePCM, frameSize)
		if err != nil {
			return nil, err
		}
		if err := oggWriter.Write(opus.Frame(frame)); err != nil {
			return nil, err
		}
	}

	if err := oggWriter.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// IsAudioMIME checks if a MIME type is audio.
func IsAudioMIME(mimeType string) bool {
	return len(mimeType) >= 6 && mimeType[:6] == "audio/"
}
