package opusrt

import (
	"container/heap"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"
)

// Default buffer duration if not specified.
var defaultBufferedDuration = FromDuration(2 * time.Minute)

// bufferedFrame holds a frame with its timestamp for the heap.
type bufferedFrame struct {
	stamp EpochMillis
	frame Frame
}

func (f *bufferedFrame) endStamp() EpochMillis {
	return f.stamp + FromDuration(f.frame.Duration())
}

// Buffer is a jitter buffer that reorders out-of-order Opus frames
// based on their timestamps.
//
// It uses a min-heap to maintain frames sorted by timestamp,
// and detects packet loss by checking gaps between consecutive frames.
type Buffer struct {
	// Duration is the maximum buffered duration. Older frames are dropped.
	Duration EpochMillis

	// Window is currently unused but reserved for future use.
	Window EpochMillis

	mu       sync.RWMutex
	heap     frameHeap
	tail     EpochMillis // timestamp of the last returned frame's end
	buffered EpochMillis // total buffered duration
}

// frameHeap implements heap.Interface for bufferedFrame pointers.
type frameHeap []*bufferedFrame

func (h frameHeap) Len() int           { return len(h) }
func (h frameHeap) Less(i, j int) bool { return h[i].stamp < h[j].stamp }
func (h frameHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *frameHeap) Push(x interface{}) {
	*h = append(*h, x.(*bufferedFrame))
}

func (h *frameHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil // avoid memory leak
	*h = old[:n-1]
	return x
}

// NewBuffer creates a new jitter buffer with the specified maximum duration.
func NewBuffer(d time.Duration) *Buffer {
	return &Buffer{Duration: EpochMillis(d.Milliseconds())}
}

// ErrDisorderedPacket is returned when a packet arrives with a timestamp
// earlier than already processed frames.
var ErrDisorderedPacket = errors.New("opusrt: disordered packet")

// Frame returns the next frame in timestamp order.
//
// Returns:
//   - frame: The Opus frame data, or nil if there's a gap (packet loss)
//   - loss: If non-zero, indicates the duration of lost data
//   - err: io.EOF when buffer is empty
func (buf *Buffer) Frame() (Frame, time.Duration, error) {
	buf.mu.Lock()
	defer buf.mu.Unlock()

	if buf.heap.Len() == 0 {
		return nil, 0, io.EOF
	}

	first := buf.heap[0]

	// Check for gap (packet loss)
	if loss := int64(first.stamp) - int64(buf.tail); buf.tail > 0 && loss > timestampEpsilon {
		buf.tail = first.stamp
		return nil, EpochMillis(loss).Duration(), nil
	}

	buf.tail = first.endStamp()
	buf.pop()
	return first.frame, 0, nil
}

func (buf *Buffer) push(f *bufferedFrame) {
	heap.Push(&buf.heap, f)
	buf.buffered += FromDuration(f.frame.Duration())
}

func (buf *Buffer) pop() *bufferedFrame {
	pop := heap.Pop(&buf.heap).(*bufferedFrame)
	buf.buffered -= FromDuration(pop.frame.Duration())
	return pop
}

func (buf *Buffer) duration() EpochMillis {
	if buf.Duration == 0 {
		return defaultBufferedDuration
	}
	return buf.Duration
}

// Append adds a frame with its timestamp to the buffer.
//
// Frames can arrive out of order; the buffer will reorder them.
// Returns ErrDisorderedPacket if the frame's timestamp is before
// already-consumed frames.
func (buf *Buffer) Append(frame Frame, stamp EpochMillis) error {
	buf.mu.Lock()
	defer buf.mu.Unlock()

	if stamp < buf.tail {
		slog.Debug("opusrt: drop frame",
			"stamp", stamp,
			"tail", buf.tail)
		return ErrDisorderedPacket
	}

	buf.push(&bufferedFrame{frame: frame, stamp: stamp})

	// Trim buffer if it exceeds max duration
	for buf.buffered > buf.duration() {
		slog.Debug("opusrt: remove frame",
			"buffered", buf.buffered,
			"duration", buf.duration())
		buf.pop()
	}

	return nil
}

// ErrInvalidFrame is returned when the stamped frame format is invalid.
var ErrInvalidFrame = errors.New("opusrt: invalid frame")

// Write implements io.Writer for stamped frame data.
// The data should be in StampedFrame format.
func (buf *Buffer) Write(stamped []byte) (int, error) {
	frame, ts, ok := FromStamped(stamped)
	if !ok {
		return 0, ErrInvalidFrame
	}
	buf.Append(frame.Clone(), ts)
	return len(stamped), nil
}

// Len returns the number of frames in the buffer.
func (buf *Buffer) Len() int {
	buf.mu.Lock()
	defer buf.mu.Unlock()
	return buf.heap.Len()
}

// Reset clears all frames from the buffer.
func (buf *Buffer) Reset() {
	buf.mu.Lock()
	defer buf.mu.Unlock()
	buf.heap = nil
	buf.tail = 0
	buf.buffered = 0
}

// Buffered returns the total duration of buffered frames.
func (buf *Buffer) Buffered() time.Duration {
	buf.mu.RLock()
	defer buf.mu.RUnlock()
	return buf.buffered.Duration()
}
