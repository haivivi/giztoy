package transformers

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/mp3"
	"github.com/haivivi/giztoy/go/pkg/audio/codec/ogg"
	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
	"github.com/haivivi/giztoy/go/pkg/buffer"
	"github.com/haivivi/giztoy/go/pkg/genx"
)

// MP3ToOgg is a transformer that converts audio/mp3 chunks to audio/ogg (Opus) chunks.
//
// Input type: audio/mp3 or audio/mpeg
// Output type: audio/ogg
//
// EoS Handling:
//   - When receiving an audio/mp3 or audio/mpeg EoS marker, finish conversion, emit audio/ogg EoS
//   - Non-MP3 chunks are passed through unchanged
type MP3ToOgg struct {
	sampleRate int
	channels   int
	bitrate    int
}

// MP3ToOggOption configures the MP3ToOgg transformer.
type MP3ToOggOption func(*MP3ToOgg)

// WithMP3ToOggSampleRate sets the output sample rate (default 48000).
func WithMP3ToOggSampleRate(rate int) MP3ToOggOption {
	return func(c *MP3ToOgg) {
		c.sampleRate = rate
	}
}

// WithMP3ToOggChannels sets the output channels (default 1).
func WithMP3ToOggChannels(channels int) MP3ToOggOption {
	return func(c *MP3ToOgg) {
		c.channels = channels
	}
}

// WithMP3ToOggBitrate sets the Opus bitrate in bits/second (default 64000).
func WithMP3ToOggBitrate(bitrate int) MP3ToOggOption {
	return func(c *MP3ToOgg) {
		c.bitrate = bitrate
	}
}

// NewMP3ToOgg creates a new MP3 to OGG transformer.
func NewMP3ToOgg(opts ...MP3ToOggOption) *MP3ToOgg {
	c := &MP3ToOgg{
		sampleRate: 48000,
		channels:   1,
		bitrate:    64000,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Transform converts audio/mp3 Blob chunks to audio/ogg Blob chunks.
// Non-audio chunks and non-mp3 audio chunks are passed through unchanged.
// MP3ToOgg does not require connection setup, so it returns immediately.
func (c *MP3ToOgg) Transform(ctx context.Context, _ string, input genx.Stream) (genx.Stream, error) {
	outBuf := buffer.N[*genx.MessageChunk](100)
	out := &mp3ToOggStream{buf: outBuf}

	go c.transformLoop(ctx, input, outBuf)

	return out, nil
}

func (c *MP3ToOgg) transformLoop(ctx context.Context, input genx.Stream, out *buffer.Buffer[*genx.MessageChunk]) {
	defer out.CloseWrite()

	var mp3Data bytes.Buffer
	var lastChunk *genx.MessageChunk

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
				// EOF: convert any remaining MP3 data
				if mp3Data.Len() > 0 {
					if err := c.flushMP3ToOgg(&mp3Data, lastChunk, out); err != nil {
						out.CloseWithError(err)
						return
					}
				}
				return
			}
			out.CloseWithError(err)
			return
		}

		if chunk == nil {
			continue
		}

		// Check for EoS marker
		if chunk.IsEndOfStream() {
			blob, ok := chunk.Part.(*genx.Blob)
			if ok && (blob.MIMEType == "audio/mp3" || blob.MIMEType == "audio/mpeg") {
				// MP3 EoS: convert accumulated data and emit OGG EoS
				if mp3Data.Len() > 0 {
					if err := c.flushMP3ToOgg(&mp3Data, lastChunk, out); err != nil {
						out.CloseWithError(err)
						return
					}
				}
				// Emit OGG EoS
				eosChunk := genx.NewEndOfStream("audio/ogg")
				if lastChunk != nil {
					eosChunk.Role = lastChunk.Role
					eosChunk.Name = lastChunk.Name
				}
				if err := out.Add(eosChunk); err != nil {
					return
				}
				continue
			}
			// Non-MP3 EoS: pass through
			if err := out.Add(chunk); err != nil {
				return
			}
			continue
		}

		// Check if it's an MP3 blob (audio/mp3 or audio/mpeg)
		blob, ok := chunk.Part.(*genx.Blob)
		if ok && (blob.MIMEType == "audio/mp3" || blob.MIMEType == "audio/mpeg") {
			// Collect MP3 data
			mp3Data.Write(blob.Data)
			lastChunk = chunk
		} else {
			// Pass through non-MP3 chunks
			if err := out.Add(chunk); err != nil {
				return
			}
		}
	}
}

// flushMP3ToOgg converts accumulated MP3 data to OGG and outputs it.
func (c *MP3ToOgg) flushMP3ToOgg(mp3Data *bytes.Buffer, lastChunk *genx.MessageChunk, out *buffer.Buffer[*genx.MessageChunk]) error {
	oggData, err := c.convertMP3ToOgg(mp3Data.Bytes())
	if err != nil {
		return err
	}

	outChunk := &genx.MessageChunk{
		Part: &genx.Blob{
			MIMEType: "audio/ogg",
			Data:     oggData,
		},
	}

	if lastChunk != nil {
		outChunk.Role = lastChunk.Role
		outChunk.Name = lastChunk.Name
	}

	if err := out.Add(outChunk); err != nil {
		return err
	}

	mp3Data.Reset()
	return nil
}

// convertMP3ToOgg converts MP3 data to OGG/Opus format.
func (c *MP3ToOgg) convertMP3ToOgg(mp3Data []byte) ([]byte, error) {
	// Create MP3 decoder
	mp3Reader := bytes.NewReader(mp3Data)
	dec := mp3.NewDecoder(mp3Reader)
	defer dec.Close()

	// Read some PCM to detect format
	testBuf := make([]byte, 4096)
	_, testErr := dec.Read(testBuf)
	if testErr != nil && testErr != io.EOF {
		return nil, fmt.Errorf("detect mp3 format: %w", testErr)
	}

	// Get sample rate and channels from the initial decode
	sampleRate := dec.SampleRate()
	channels := dec.Channels()
	if sampleRate <= 0 {
		sampleRate = c.sampleRate // fallback
	}
	if channels <= 0 {
		channels = c.channels // fallback
	}

	// Reset decoder by recreating it
	mp3Reader = bytes.NewReader(mp3Data)
	dec.Close()
	dec = mp3.NewDecoder(mp3Reader)

	// Create Opus encoder with detected parameters
	enc, err := opus.NewVoIPEncoder(sampleRate, channels)
	if err != nil {
		return nil, fmt.Errorf("opus encoder: %w", err)
	}
	defer enc.Close()

	if err := enc.SetBitrate(c.bitrate); err != nil {
		return nil, fmt.Errorf("set bitrate: %w", err)
	}

	// Create OGG writer
	var oggBuf bytes.Buffer
	oggWriter, err := ogg.NewOpusWriter(&oggBuf, sampleRate, channels)
	if err != nil {
		return nil, fmt.Errorf("ogg writer: %w", err)
	}

	// Calculate frame size for 20ms frames
	frameSize := sampleRate * 20 / 1000     // samples per channel
	bytesPerFrame := frameSize * channels * 2 // 16-bit samples

	// Read PCM from MP3 and encode to Opus
	pcmBuf := make([]byte, bytesPerFrame)

	for {
		n, err := io.ReadFull(dec, pcmBuf)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				// Handle partial last frame
				if n > 0 {
					// Pad with zeros
					for i := n; i < len(pcmBuf); i++ {
						pcmBuf[i] = 0
					}
					frame, err := enc.EncodeBytes(pcmBuf, frameSize)
					if err != nil {
						return nil, fmt.Errorf("encode partial: %w", err)
					}
					if err := oggWriter.Write(opus.Frame(frame)); err != nil {
						return nil, fmt.Errorf("write partial: %w", err)
					}
				}
				break
			}
			return nil, fmt.Errorf("read pcm: %w", err)
		}

		// Encode PCM to Opus
		frame, err := enc.EncodeBytes(pcmBuf, frameSize)
		if err != nil {
			return nil, fmt.Errorf("encode: %w", err)
		}

		// Write to OGG
		if err := oggWriter.Write(opus.Frame(frame)); err != nil {
			return nil, fmt.Errorf("write: %w", err)
		}
	}

	// Close OGG writer to flush
	if err := oggWriter.Close(); err != nil {
		return nil, fmt.Errorf("close ogg: %w", err)
	}

	return oggBuf.Bytes(), nil
}

// mp3ToOggStream wraps a buffer as a Stream.
type mp3ToOggStream struct {
	buf    *buffer.Buffer[*genx.MessageChunk]
	closed bool
}

func (s *mp3ToOggStream) Next() (*genx.MessageChunk, error) {
	chunk, err := s.buf.Next()
	if err == buffer.ErrIteratorDone {
		return nil, io.EOF
	}
	return chunk, err
}

func (s *mp3ToOggStream) Close() error {
	if !s.closed {
		s.closed = true
		s.buf.CloseWrite()
	}
	return nil
}

func (s *mp3ToOggStream) CloseWithError(err error) error {
	if !s.closed {
		s.closed = true
		s.buf.CloseWithError(err)
	}
	return nil
}
