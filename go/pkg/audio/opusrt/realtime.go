package opusrt

import (
	"errors"
	"io"
	"sync/atomic"
	"time"

	pkgbuf "github.com/haivivi/giztoy/pkg/buffer"
)

// ErrDone is returned when the stream is exhausted.
var ErrDone = errors.New("opusrt: done")

// timestampEpsilon is the tolerance in milliseconds for timestamp comparisons.
// This accounts for minor timing variations in real-time audio streaming.
// A 2ms tolerance is chosen as it's smaller than the shortest Opus frame
// duration (2.5ms) while allowing for reasonable clock drift.
const timestampEpsilon = 2

// RealtimeBuffer wraps a Buffer to simulate real-time playback.
//
// It reads frames from the underlying buffer and generates loss events
// when data is not available at the expected time. This is useful for
// implementing packet loss concealment (PLC) in audio decoders.
type RealtimeBuffer struct {
	opus *Buffer
	evts *pkgbuf.BlockBuffer[frameAndLoss]

	// readTick = opus.tail + offset
	readTick EpochMillis
	offset   EpochMillis

	closeWrite atomic.Bool
}

// RealtimeFrom creates a RealtimeBuffer from an existing Buffer.
// Starts a background goroutine to pull frames in real-time.
func RealtimeFrom(buf *Buffer) *RealtimeBuffer {
	rb := pkgbuf.BlockN[frameAndLoss](1024)
	rtb := &RealtimeBuffer{opus: buf, evts: rb}
	go rtb.pull()
	return rtb
}

type frameAndLoss struct {
	frame Frame
	loss  time.Duration
}

func (rtb *RealtimeBuffer) takeOne() (frame Frame, loss time.Duration, nextReadTick EpochMillis, ok bool) {
	rtb.opus.mu.Lock()
	defer rtb.opus.mu.Unlock()

	if rtb.opus.heap.Len() == 0 {
		return nil, 0, 0, false
	}
	top := rtb.opus.heap[0]

	// Initialize timing on first frame
	if rtb.opus.tail == 0 {
		rtb.opus.tail = top.stamp
		rtb.offset = Now() - rtb.opus.tail
	}

	// Check for gap (packet loss)
	if loss := int64(top.stamp) - int64(rtb.readTick-rtb.offset); loss > timestampEpsilon {
		rtb.opus.tail = top.stamp
		rt := (rtb.opus.tail - timestampEpsilon) + rtb.offset
		return nil, EpochMillis(loss).Duration(), rt, true
	}

	rtb.opus.tail = top.endStamp()
	rtb.opus.pop()
	rt := (rtb.opus.tail - timestampEpsilon) + rtb.offset
	return top.frame, 0, rt, true
}

func (rtb *RealtimeBuffer) pull() {
	const (
		pullInterval  = 20
		lossStep      = pullInterval * time.Millisecond
		lossThreshold = 10 * lossStep
	)

	defer func() {
		rtb.evts.CloseWrite()
	}()

	var (
		lossSum   time.Duration
		inLossing bool
	)

	for {
		// Wait until next read time
		if d := rtb.readTick - Now(); d > 0 {
			time.Sleep(time.Duration(d) * time.Millisecond)
		}

		f, loss, readTick, ok := rtb.takeOne()
		if !ok {
			if rtb.closeWrite.Load() {
				return
			}

			// No data available - accumulate loss
			if lossSum > lossThreshold {
				var v time.Duration
				if !inLossing {
					inLossing = true
					v = lossThreshold
				} else {
					v = lossStep
				}
				if err := rtb.evts.Add(frameAndLoss{
					frame: nil,
					loss:  v,
				}); err != nil {
					return
				}
			}
			lossSum += lossStep
			rtb.readTick = Now() + pullInterval
			continue
		}

		if loss == 0 {
			// Got a valid frame
			rtb.readTick += FromDuration(f.Duration())
			lossSum = 0
			inLossing = false
			if err := rtb.evts.Add(frameAndLoss{
				frame: f,
			}); err != nil {
				return
			}
		} else {
			// Gap detected - report loss
			if err := rtb.evts.Add(frameAndLoss{
				loss: loss,
			}); err != nil {
				return
			}
			lossSum = 0
		}

		if readTick > rtb.readTick {
			rtb.readTick = readTick
		}
	}
}

// Frame returns the next frame or loss event.
//
// Returns:
//   - frame: The Opus frame data, or nil if there's a gap
//   - loss: If non-zero, indicates the duration of lost data
//   - err: io.EOF when the stream ends
func (rtb *RealtimeBuffer) Frame() (Frame, time.Duration, error) {
	rtf, err := rtb.evts.Next()
	if err != nil {
		if errors.Is(err, ErrDone) || errors.Is(err, io.ErrClosedPipe) {
			return nil, 0, io.EOF
		}
		return nil, 0, err
	}
	return rtf.frame, rtf.loss, nil
}

// Reset clears the underlying buffer.
func (rtb *RealtimeBuffer) Reset() {
	rtb.opus.Reset()
}

// Write implements io.Writer for stamped frame data.
func (rtb *RealtimeBuffer) Write(stamped []byte) (int, error) {
	if rtb.closeWrite.Load() {
		return 0, io.ErrClosedPipe
	}
	return rtb.opus.Write(stamped)
}

// CloseWrite signals that no more data will be written.
func (rtb *RealtimeBuffer) CloseWrite() error {
	rtb.closeWrite.Store(true)
	return nil
}

// Close closes the buffer and releases resources.
func (rtb *RealtimeBuffer) Close() error {
	return rtb.evts.Close()
}

// CloseWithError closes the buffer with a specific error.
func (rtb *RealtimeBuffer) CloseWithError(err error) error {
	return rtb.evts.CloseWithError(err)
}

// RealtimeReader wraps a FrameReader to simulate real-time playback.
// It sleeps between frames to match real-time pacing.
type RealtimeReader[T FrameReader] struct {
	FrameReader T

	// DelayFunc is called to adjust the delay between frames.
	// If nil, the delay will be the frame's duration.
	DelayFunc func(duration, gap time.Duration) time.Duration

	duration time.Duration
	nextRead time.Time
}

// Frame returns the next frame, sleeping to maintain real-time pacing.
func (r *RealtimeReader[T]) Frame() (Frame, time.Duration, error) {
	if r.nextRead.IsZero() {
		// First frame
		f, d, err := r.FrameReader.Frame()
		if err != nil {
			return nil, 0, err
		}
		if d == 0 {
			d = f.Duration()
		}
		r.nextRead = time.Now().Add(d)
		r.duration = d
		return f, d, nil
	}

	f, d, err := r.FrameReader.Frame()
	if err != nil {
		return nil, 0, err
	}

	// Calculate and apply delay
	gap := time.Until(r.nextRead)
	if r.DelayFunc != nil {
		gap = r.DelayFunc(r.duration, gap)
	}
	if gap > 0 {
		time.Sleep(gap)
	}

	if d == 0 {
		d = f.Duration()
	}
	r.nextRead = r.nextRead.Add(d)
	r.duration += d
	return f, d, nil
}
