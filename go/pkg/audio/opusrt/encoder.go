package opusrt

import (
	"io"
	"time"

	"github.com/haivivi/giztoy/pkg/audio/codec/opus"
)

// EncodePCMOption is an interface for configuring EncodePCMStream behavior.
type EncodePCMOption interface {
	applyEncoderOption(s *OpusFrameStream)
}

// WithFramePacking enables/disables frame packing.
type WithFramePacking struct {
	Enabled bool
}

func (w WithFramePacking) applyEncoderOption(s *OpusFrameStream) {
	// Frame packing implementation would go here
}

// OpusFrameStream wraps an opus.Encoder to implement FrameReader.
type OpusFrameStream struct {
	enc        *opus.Encoder
	pcm        io.Reader
	buf        []byte // PCM input buffer (bytes)
	sampleRate int
	channels   int
	frameSize  int // samples per channel per frame
}

// EncodePCMStream creates an Opus frame stream from PCM data.
// The PCM data should be 16-bit signed integers, interleaved stereo if channels=2.
func EncodePCMStream(pcm io.Reader, sampleRate, channels int, opts ...EncodePCMOption) (*OpusFrameStream, error) {
	enc, err := opus.NewVoIPEncoder(sampleRate, channels)
	if err != nil {
		return nil, err
	}

	// 20ms frame
	frameSize := sampleRate * 20 / 1000
	// Bytes per frame: frameSize * channels * 2 (16-bit)
	bufSize := frameSize * channels * 2

	s := &OpusFrameStream{
		enc:        enc,
		pcm:        pcm,
		buf:        make([]byte, bufSize),
		sampleRate: sampleRate,
		channels:   channels,
		frameSize:  frameSize,
	}

	// Apply options
	for _, opt := range opts {
		opt.applyEncoderOption(s)
	}

	return s, nil
}

// Frame returns the next encoded Opus frame.
func (s *OpusFrameStream) Frame() (Frame, time.Duration, error) {
	n, err := io.ReadFull(s.pcm, s.buf)
	if err != nil {
		return nil, 0, err
	}
	if n == 0 {
		return nil, 0, io.EOF
	}

	// Encode PCM bytes to Opus
	encoded, err := s.enc.EncodeBytes(s.buf[:n], s.frameSize)
	if err != nil {
		return nil, 0, err
	}

	frame := make(Frame, len(encoded))
	copy(frame, encoded)

	return frame, frame.Duration(), nil
}

// Close releases encoder resources.
func (s *OpusFrameStream) Close() error {
	if s.enc != nil {
		s.enc.Close()
		s.enc = nil
	}
	return nil
}
