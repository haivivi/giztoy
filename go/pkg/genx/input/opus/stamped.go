package opus

import (
	"io"
	"sync/atomic"
	"time"

	"github.com/haivivi/giztoy/go/pkg/buffer"
	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/input"
)

// StampedOpusReader reads timestamped Opus frames from a real-time source.
// The returned bytes should be in the wire format:
//
//	[Version(1B) | Timestamp(7B big-endian ms) | OpusFrameData(N)]
type StampedOpusReader interface {
	ReadStamped() ([]byte, error)
}

// RealtimeConfig configures real-time stream behavior.
type RealtimeConfig struct {
	// Role is the message role (default: RoleUser).
	Role genx.Role

	// Name is the producer name.
	Name string

	// MaxLoss is the maximum silence duration before resync (default: 5s).
	// When a gap exceeds this duration, the stream resyncs to the next frame
	// instead of generating silence.
	MaxLoss time.Duration

	// JitterBufferSize is the max number of frames in jitter buffer (default: 100).
	JitterBufferSize int
}

func (c *RealtimeConfig) setDefaults() {
	if c.Role == "" {
		c.Role = genx.RoleUser
	}
	if c.MaxLoss == 0 {
		c.MaxLoss = 5 * time.Second
	}
	if c.JitterBufferSize == 0 {
		c.JitterBufferSize = 100
	}
}

// FromStampedReader creates a genx.Stream from a StampedOpusReader.
//
// This function starts background goroutines that:
//   - Read frames from the reader and add them to a jitter buffer
//   - Pull frames from the jitter buffer with real-time pacing
//   - Generate silence frames for gaps smaller than MaxLoss
//   - Resync (skip silence) for gaps larger than MaxLoss
//
// The returned Stream produces MessageChunks with MIMEType "audio/opus".
func FromStampedReader(reader StampedOpusReader, cfg RealtimeConfig) genx.Stream {
	cfg.setDefaults()

	s := &stampedStream{
		reader:  reader,
		cfg:     cfg,
		jitter:  input.NewJitterBuffer[EpochMillis, StampedFrame](cfg.JitterBufferSize),
		chunks:  buffer.N[*genx.MessageChunk](256),
		frameCh: make(chan StampedFrame, 64),
	}

	go s.readLoop()
	go s.pullLoop()

	return s
}

type stampedStream struct {
	reader StampedOpusReader
	cfg    RealtimeConfig
	jitter *input.JitterBuffer[EpochMillis, StampedFrame]
	chunks *buffer.Buffer[*genx.MessageChunk]

	// frameCh passes frames from readLoop to pullLoop
	frameCh chan StampedFrame

	// closed indicates reader has finished
	closed atomic.Bool
}

// Next returns the next MessageChunk from the stream.
func (s *stampedStream) Next() (*genx.MessageChunk, error) {
	chunk, err := s.chunks.Next()
	if err != nil {
		if err == buffer.ErrIteratorDone {
			return nil, io.EOF
		}
		return nil, err
	}
	return chunk, nil
}

// Close closes the stream.
func (s *stampedStream) Close() error {
	return s.chunks.Close()
}

// CloseWithError closes the stream with an error.
func (s *stampedStream) CloseWithError(err error) error {
	return s.chunks.CloseWithError(err)
}

// readLoop reads from the StampedOpusReader and sends to frameCh.
func (s *stampedStream) readLoop() {
	defer func() {
		s.closed.Store(true)
		close(s.frameCh)
	}()

	for {
		data, err := s.reader.ReadStamped()
		if err != nil {
			if err != io.EOF {
				s.chunks.CloseWithError(err)
			}
			return
		}

		frame, stamp, ok := ParseStamped(data)
		if !ok {
			continue // skip invalid frames
		}

		select {
		case s.frameCh <- StampedFrame{Frame: frame.Clone(), Stamp: stamp}:
		default:
			// Channel full, drop frame (should not happen with proper sizing)
		}
	}
}

// pullLoop pulls frames from jitter buffer with real-time pacing.
func (s *stampedStream) pullLoop() {
	defer s.chunks.CloseWrite()

	const pollInterval = 10 * time.Millisecond

	var (
		nextTick   EpochMillis // expected timestamp of next frame
		lastStamp  EpochMillis // timestamp of last emitted frame
		frameCount int
	)

	for {
		// Drain incoming frames to jitter buffer
		s.drainToJitter()

		// Check if we're done
		if s.closed.Load() && s.jitter.Len() == 0 {
			return
		}

		// Get next frame from jitter buffer
		sf, ok := s.jitter.Peek()
		if !ok {
			// No frames available, wait a bit
			time.Sleep(pollInterval)
			continue
		}

		// Initialize timing on first frame
		if frameCount == 0 {
			nextTick = Now()
			lastStamp = sf.Stamp
			frameCount++
			s.jitter.Pop()
			s.emit(sf.Frame)
			continue
		}

		// Calculate gap from last frame
		gap := sf.Stamp.Sub(lastStamp) - sf.Frame.Duration()

		if gap > s.cfg.MaxLoss {
			// Large gap - resync (skip silence generation)
			nextTick = Now()
			lastStamp = sf.Stamp
		} else if gap > 0 {
			// Small gap - generate silence
			s.emitSilence(gap)
			nextTick = nextTick.Add(gap)
		}

		// Wait until it's time to emit this frame
		waitUntil := nextTick
		if wait := waitUntil.Sub(Now()); wait > 0 {
			time.Sleep(wait)
		}

		// Emit the frame
		s.jitter.Pop()
		s.emit(sf.Frame)

		frameDuration := sf.Frame.Duration()
		nextTick = nextTick.Add(frameDuration)
		lastStamp = sf.Stamp + FromDuration(frameDuration)
		frameCount++
	}
}

// drainToJitter moves all available frames from frameCh to jitter buffer.
func (s *stampedStream) drainToJitter() {
	for {
		select {
		case sf, ok := <-s.frameCh:
			if !ok {
				return
			}
			s.jitter.Push(sf)
		default:
			return
		}
	}
}

// emit sends an Opus frame as a MessageChunk.
func (s *stampedStream) emit(frame OpusFrame) {
	chunk := &genx.MessageChunk{
		Role: s.cfg.Role,
		Name: s.cfg.Name,
		Part: &genx.Blob{
			MIMEType: "audio/opus",
			Data:     frame,
		},
	}
	s.chunks.Add(chunk)
}

// OpusSilence20ms is a 20ms Opus silence frame (mono, fullband, CELT-only).
// This is a valid Opus frame that decodes to silence.
var OpusSilence20ms = OpusFrame{0xf8, 0xff, 0xfe}

// emitSilence generates silence frames for the given duration.
func (s *stampedStream) emitSilence(duration time.Duration) {
	const silenceFrameDuration = 20 * time.Millisecond

	for duration >= silenceFrameDuration {
		s.emit(OpusSilence20ms)
		duration -= silenceFrameDuration
	}
}
