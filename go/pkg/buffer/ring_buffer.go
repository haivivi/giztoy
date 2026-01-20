package buffer

import (
	"fmt"
	"io"
	"slices"
	"sync"
)

// RingBuffer is a thread-safe ring buffer that implements io.Reader and io.Writer
// interfaces. Unlike other buffer types, RingBuffer overwrites the oldest data
// when the buffer is full, making it suitable for maintaining a sliding window
// of the most recent data.
//
// The buffer uses head and tail pointers to implement a circular buffer.
// When the buffer is full, new writes overwrite the oldest data and advance
// the head pointer. When the buffer is empty, read operations block until
// data is written.
type RingBuffer[T any] struct {
	writeNotify chan struct{}

	mu         sync.Mutex
	buf        []T
	head, tail int64
	closeWrite bool
	closeErr   error
}

// RingN creates a new RingBuffer with the specified size.
// The buffer will overwrite the oldest data when this capacity is exceeded.
func RingN[T any](size int) *RingBuffer[T] {
	return &RingBuffer[T]{
		writeNotify: make(chan struct{}, 1),

		buf: make([]T, size),
	}
}

// Discard removes and discards the next n elements from the buffer without reading them.
// If n is greater than the available elements, all available data is discarded.
func (rb *RingBuffer[T]) Discard(n int) (err error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.closeErr != nil {
		return fmt.Errorf("buffer: skip from closed buffer: %w", rb.closeErr)
	}
	if n > int(rb.tail-rb.head) {
		rb.head = rb.tail
		return nil
	}
	rb.head += int64(n)
	return nil
}

// Read reads data from the buffer into the provided slice.
// It blocks until data is available or the buffer is closed.
// Returns the number of elements read and any error encountered.
func (rb *RingBuffer[T]) Read(p []T) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closeErr != nil {
		return 0, fmt.Errorf("buffer: read from closed buffer: %w", rb.closeErr)
	}

	for rb.head == rb.tail {
		if rb.closeWrite {
			return 0, io.EOF
		}
		rb.mu.Unlock()
		<-rb.writeNotify
		rb.mu.Lock()
		if rb.closeErr != nil {
			return 0, fmt.Errorf("buffer: read from closed buffer: %w", rb.closeErr)
		}
	}

	avail := int(rb.tail - rb.head)
	head := int(rb.head % int64(len(rb.buf)))

	var n int
	if head+avail <= len(rb.buf) {
		n = copy(p, rb.buf[head:head+avail])
	} else {
		n = copy(p, rb.buf[head:])
		n += copy(p[n:], rb.buf[:avail-n])
	}

	rb.head += int64(n)
	return n, nil
}

// CloseWithError closes the buffer with the specified error.
// All pending operations are unblocked and return this error.
func (rb *RingBuffer[T]) CloseWithError(err error) error {
	if err == nil {
		err = io.ErrClosedPipe
	}
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.closeWithErrorLocked(err)
}

func (rb *RingBuffer[T]) closeWithErrorLocked(err error) error {
	if rb.closeErr != nil {
		return nil
	}
	rb.closeErr = err

	if !rb.closeWrite {
		rb.closeWrite = true
		close(rb.writeNotify)
	}
	return nil
}

// Error returns the error that caused the buffer to be closed, if any.
func (rb *RingBuffer[T]) Error() error {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.closeErr
}

// Close closes the buffer, preventing further writes and reads.
// Equivalent to CloseWithError(io.ErrClosedPipe).
func (rb *RingBuffer[T]) Close() error {
	return rb.CloseWithError(io.ErrClosedPipe)
}

// CloseWrite closes the write side of the buffer, preventing further writes.
// Reads will continue to work until the buffer is empty, then return EOF.
func (rb *RingBuffer[T]) CloseWrite() error {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.closeWrite {
		return nil
	}
	rb.closeWrite = true
	close(rb.writeNotify)
	return nil
}

// Write writes data from the provided slice to the buffer.
// Unlike other buffer types, RingBuffer overwrites the oldest data when full,
// rather than blocking. This ensures that the most recent data is always preserved.
// Returns the number of elements written and any error encountered.
func (rb *RingBuffer[T]) Write(p []T) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.closeErr != nil {
		return 0, fmt.Errorf("buffer: write to closed buffer: %w", rb.closeErr)
	}
	if rb.closeWrite {
		return 0, fmt.Errorf("buffer: write to closed buffer: %w", io.ErrClosedPipe)
	}

	// write to full
	bufsz := int64(len(rb.buf))
	avail := int(bufsz - (rb.tail - rb.head))
	tail := int(rb.tail % bufsz)

	var wn int
	if avail > 0 {
		if tail+avail <= len(rb.buf) {
			wn = copy(rb.buf[tail:tail+avail], p)
		} else {
			wn = copy(rb.buf[tail:], p)
			wn += copy(rb.buf[:avail-wn], p[wn:])
		}
		rb.tail += int64(wn)
	}

	leftn := len(p) - wn
	if leftn == 0 {
		return wn, nil
	}

	// Now the buffer is full, and all we write the left `leftn` bytes to the
	// buffer. We can calculate the outcome result as follows:
	//
	//  [buffer]
	//   ......
	//   ......
	//   ....bb
	//   cccc
	//
	// Where 'b' and 'c' are the data we actually write. So we just skip the
	// first `.` bytes, and then copy `b` and `c` to the buffer.

	var cbuf, bbuf []T

	if leftn <= len(rb.buf) {
		//  [buffer]
		//   cccc
		cbuf = p[len(p)-leftn:]
	} else {
		//  [buffer]
		//   ......
		//   ......
		//   ....bb
		//   cccc
		cn := leftn % len(rb.buf)                // the number of `c`
		cbuf = p[len(p)-cn:]                     // the data of `c`
		bbuf = p[len(p)-len(rb.buf) : len(p)-cn] // the data of `b`
	}

	// now we need to copy the data to the buffer, and then copy the data to the
	// buffer again.
	//
	// tail - head must be equal to buffer size, as we already write a full
	// buffer in the previous step.
	head := int(rb.head % bufsz)
	if cp1 := copy(rb.buf[head:], cbuf); cp1 < len(cbuf) {
		//  The original buffer: 	......
		//  The head and tail:      	h....t
		//  The result should be:	ccbbcc
		cp2 := copy(rb.buf, cbuf[cp1:])
		copy(rb.buf[cp2:], bbuf)
	} else {
		//  The original buffer: 	......
		//  The head and tail:       h....t
		//  The result should be:	bccccb
		bp1 := copy(rb.buf[head+cp1:], bbuf)
		copy(rb.buf, bbuf[bp1:])
	}

	// move the head to `b` position.
	rb.head += int64(len(cbuf))
	rb.tail += int64(len(cbuf))

	return len(p), nil
}

// Next reads and returns the next element from the buffer.
// It blocks until an element is available or the buffer is closed.
// Returns ErrIteratorDone when the buffer is closed and empty.
func (rb *RingBuffer[T]) Next() (t T, err error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.closeErr != nil {
		err = fmt.Errorf("buffer: read from closed buffer: %w", rb.closeErr)
		return
	}
	for rb.head == rb.tail {
		if rb.closeWrite {
			err = ErrIteratorDone
			return
		}
		rb.mu.Unlock()
		<-rb.writeNotify
		rb.mu.Lock()
		if rb.closeErr != nil {
			err = fmt.Errorf("buffer: read from closed buffer: %w", rb.closeErr)
			return
		}
	}
	head := rb.head % int64(len(rb.buf))
	chunk := rb.buf[head]
	rb.head++
	return chunk, nil
}

// Add adds a single element directly to the buffer.
// If the buffer is full, it will overwrite the oldest element and advance
// the head pointer to maintain the ring buffer behavior.
func (rb *RingBuffer[T]) Add(t T) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.closeErr != nil {
		return fmt.Errorf("buffer: write to closed buffer: %w", rb.closeErr)
	}
	if rb.closeWrite {
		return fmt.Errorf("buffer: write to closed buffer: %w", io.ErrClosedPipe)
	}
	tail := rb.tail % int64(len(rb.buf))
	rb.buf[tail] = t
	rb.tail++
	if rb.tail-rb.head > int64(len(rb.buf)) {
		rb.head++
	}
	select {
	case rb.writeNotify <- struct{}{}:
	default:
	}
	return nil
}

// Reset clears the buffer by resetting the head and tail pointers.
// This effectively discards all buffered data.
func (rb *RingBuffer[T]) Reset() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.head = 0
	rb.tail = 0
}

// Len returns the number of elements currently in the buffer.
func (rb *RingBuffer[T]) Len() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return int(rb.tail - rb.head)
}

// Bytes returns a slice containing all elements currently in the buffer.
// The returned slice is a copy of the buffered data.
func (rb *RingBuffer[T]) Bytes() []T {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	h := rb.head % int64(len(rb.buf))
	t := rb.tail % int64(len(rb.buf))
	if h < t {
		return rb.buf[h:t]
	}
	return slices.Concat(rb.buf[h:], rb.buf[:t])
}
