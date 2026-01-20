package buffer

import (
	"fmt"
	"io"
	"slices"
	"sync"
)

// BlockBuffer is a thread-safe fixed-size circular buffer that implements
// io.Reader and io.Writer interfaces. It blocks when full or empty, providing
// predictable memory usage and flow control.
//
// The buffer uses head and tail pointers to implement a circular buffer.
// Write operations block when the buffer is full, and read operations block
// when the buffer is empty. This makes it ideal for scenarios requiring
// controlled data flow between producers and consumers.
type BlockBuffer[T any] struct {
	cond *sync.Cond

	mu         sync.Mutex
	buf        []T
	head, tail int64
	closeWrite bool
	closeErr   error
}

// Block creates a new BlockBuffer using the provided slice as the underlying buffer.
// The buffer size is determined by the length of the provided slice.
func Block[T any](buf []T) *BlockBuffer[T] {
	v := &BlockBuffer[T]{
		buf: buf,
	}
	v.cond = sync.NewCond(&v.mu)
	return v
}

// BlockN creates a new BlockBuffer with the specified size.
func BlockN[T any](size int) *BlockBuffer[T] {
	return Block(make([]T, size))
}

// Discard removes and discards the next n elements from the buffer without
// reading them.
//
// This operation advances the head pointer by n positions, effectively removing
// elements from the front of the buffer. If n is greater than the number of
// available elements, all available data is discarded and the buffer becomes
// empty.
//
// Returns an error if the buffer has been closed with an error.
func (bb *BlockBuffer[T]) Discard(n int) (err error) {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	if bb.closeErr != nil {
		return fmt.Errorf("buffer: skip from closed buffer: %w", bb.closeErr)
	}
	if n > int(bb.tail-bb.head) {
		bb.head = bb.tail
		return nil
	}
	bb.head += int64(n)
	return nil
}

// Read reads data from the buffer into the provided slice.
//
// This method implements the io.Reader interface. It blocks until at least one
// element is available in the buffer or the buffer is closed. It attempts to
// read as many elements as possible up to len(p).
//
// The method handles the circular nature of the buffer by potentially reading
// from two segments: from the head to the end of the buffer, and from the
// beginning of the buffer if the data wraps around.
//
// Returns the number of elements actually read and any error encountered.
// Returns io.EOF if the buffer is closed for writing and empty.
func (bb *BlockBuffer[T]) Read(p []T) (int, error) {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	if bb.closeErr != nil {
		return 0, fmt.Errorf("buffer: read from closed buffer: %w", bb.closeErr)
	}

	for bb.head == bb.tail {
		if bb.closeWrite {
			return 0, io.EOF
		}
		bb.cond.Wait()
		if bb.closeErr != nil {
			return 0, fmt.Errorf("buffer: read from closed buffer: %w", bb.closeErr)
		}
	}

	avail := int(bb.tail - bb.head)
	head := int(bb.head % int64(len(bb.buf)))

	var n int
	if head+avail <= len(bb.buf) {
		n = copy(p, bb.buf[head:head+avail])
	} else {
		n = copy(p, bb.buf[head:])
		n += copy(p[n:], bb.buf[:avail-n])
	}

	bb.head += int64(n)
	bb.cond.Signal()
	return n, nil
}

// Write writes data from the provided slice to the buffer.
//
// This method implements the io.Writer interface. It blocks if the buffer is
// full until space becomes available. It attempts to write all elements from
// the provided slice, potentially blocking multiple times if the buffer becomes
// full during the write operation.
//
// The method handles the circular nature of the buffer by potentially writing
// to two segments: from the tail to the end of the buffer, and from the
// beginning of the buffer if the data wraps around.
//
// Returns the number of elements actually written and any error encountered.
// Returns io.ErrClosedPipe if the buffer is closed for writing.
func (bb *BlockBuffer[T]) Write(p []T) (int, error) {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	if bb.closeErr != nil {
		return 0, fmt.Errorf("buffer: write to closed buffer: %w", bb.closeErr)
	}
	if bb.closeWrite {
		return 0, fmt.Errorf("buffer: write to closed buffer: %w", io.ErrClosedPipe)
	}

	wn := 0
	bufsz := int64(len(bb.buf))
	for len(p) > 0 {
		for bb.tail-bb.head == bufsz {
			bb.cond.Wait()
			if bb.closeErr != nil {
				return wn, fmt.Errorf("buffer: write to closed buffer: %w", bb.closeErr)
			}
			if bb.closeWrite {
				return wn, fmt.Errorf("buffer: write to closed buffer: %w", io.ErrClosedPipe)
			}
		}
		avail := int(bufsz - (bb.tail - bb.head))
		tail := int(bb.tail % bufsz)

		var n int
		if tail+avail <= len(bb.buf) {
			n = copy(bb.buf[tail:tail+avail], p)
		} else {
			n = copy(bb.buf[tail:], p)
			n += copy(bb.buf[:avail-n], p[n:])
		}

		bb.tail += int64(n)
		p = p[n:]
		wn += n
		bb.cond.Signal()
	}
	return wn, nil
}

// CloseWithError closes the buffer with the specified error.
//
// This method immediately closes both the read and write sides of the buffer.
// All pending read and write operations will be unblocked and return the
// specified error. If err is nil, io.ErrClosedPipe is used as the default
// error.
//
// Once closed, all subsequent read and write operations will return the
// specified error. The error can be retrieved later using the Error() method.
//
// Returns nil if the buffer was already closed, or the error if this is the
// first time the buffer is being closed.
func (bb *BlockBuffer[T]) CloseWithError(err error) error {
	if err == nil {
		err = io.ErrClosedPipe
	}
	bb.mu.Lock()
	defer bb.mu.Unlock()
	return bb.closeWithErrorLocked(err)
}

func (bb *BlockBuffer[T]) closeWithErrorLocked(err error) error {
	if bb.closeErr != nil {
		return nil
	}
	bb.closeErr = err
	if !bb.closeWrite {
		bb.closeWrite = true
	}
	bb.cond.Broadcast()
	return nil
}

// Error returns the error that caused the buffer to be closed, if any.
//
// This method returns the error that was passed to CloseWithError() when the
// buffer was closed. If the buffer has not been closed or was closed without an
// error, this method returns nil.
func (bb *BlockBuffer[T]) Error() error {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	return bb.closeErr
}

// Close closes the buffer, preventing further writes and reads.
//
// This method is equivalent to CloseWithError(io.ErrClosedPipe). It immediately
// closes both the read and write sides of the buffer and unblocks all pending
// operations with io.ErrClosedPipe.
//
// This method implements the io.Closer interface.
func (bb *BlockBuffer[T]) Close() error {
	return bb.CloseWithError(io.ErrClosedPipe)
}

// CloseWrite closes the write side of the buffer, preventing further writes.
//
// This method provides a graceful shutdown mechanism. It prevents new writes to
// the buffer while allowing existing data to be read. Once the buffer is empty,
// subsequent read operations will return io.EOF.
//
// This is useful for signaling the end of a data stream while allowing
// consumers to finish reading any remaining buffered data.
//
// Returns nil if the write side was already closed.
func (bb *BlockBuffer[T]) CloseWrite() error {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	if bb.closeWrite {
		return nil
	}
	bb.closeWrite = true
	bb.cond.Broadcast()
	return nil
}

// Next reads and returns the next element from the buffer.
//
// This method implements the iterator pattern, reading one element at a time.
// It blocks until an element is available in the buffer or the buffer is
// closed.
//
// Returns the next element and any error encountered. If the buffer is closed
// for writing and empty, returns ErrIteratorDone to signal the end of iteration.
// This is useful for implementing streaming interfaces that process elements
// one at a time.
func (bb *BlockBuffer[T]) Next() (t T, err error) {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	if bb.closeErr != nil {
		err = fmt.Errorf("buffer: read from closed buffer: %w", bb.closeErr)
		return
	}
	for bb.head == bb.tail {
		if bb.closeWrite {
			err = ErrIteratorDone
			return
		}
		bb.cond.Wait()
		if bb.closeErr != nil {
			err = fmt.Errorf("buffer: read from closed buffer: %w", bb.closeErr)
			return
		}
	}
	head := bb.head % int64(len(bb.buf))
	chunk := bb.buf[head]
	bb.head++
	bb.cond.Signal()
	return chunk, nil
}

// Add adds a single element directly to the buffer.
//
// This method is a convenience function for adding individual elements to the
// buffer. It blocks if the buffer is full until space becomes available. The
// element is stored directly in the buffer without explicit copying.
//
// This is more efficient than using Write() with a single-element slice for
// adding individual elements, as it avoids slice allocation and copying.
//
// Returns an error if the buffer is closed for writing or has been closed with
// an error.
func (bb *BlockBuffer[T]) Add(t T) error {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	if bb.closeErr != nil {
		return fmt.Errorf("buffer: write to closed buffer: %w", bb.closeErr)
	}
	if bb.closeWrite {
		return fmt.Errorf("buffer: write to closed buffer: %w", io.ErrClosedPipe)
	}
	bufsz := int64(len(bb.buf))
	for bb.tail-bb.head == bufsz {
		bb.cond.Wait()
		if bb.closeErr != nil {
			return fmt.Errorf("buffer: write to closed buffer: %w", bb.closeErr)
		}
		if bb.closeWrite {
			return fmt.Errorf("buffer: write to closed buffer: %w", io.ErrClosedPipe)
		}
	}
	tail := bb.tail % int64(len(bb.buf))
	bb.buf[tail] = t
	bb.tail++
	bb.cond.Signal()
	return nil
}

// Reset clears the buffer by resetting the head and tail pointers.
//
// This method effectively discards all buffered data by setting both head and
// tail pointers to 0. The underlying buffer slice is not modified, but all
// previously stored data becomes inaccessible.
//
// This operation is not affected by the buffer's closed state - it can be
// called even if the buffer has been closed. However, it does not reopen a
// closed buffer.
func (bb *BlockBuffer[T]) Reset() {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	bb.head = 0
	bb.tail = 0
}

// Len returns the number of elements currently in the buffer.
//
// This method returns the current length of the buffer, which is the difference
// between the tail and head pointers. It provides a snapshot of the buffer size
// at the time of the call.
//
// The returned value is always between 0 and the buffer's capacity. This method
// is thread-safe and can be called concurrently with other operations.
func (bb *BlockBuffer[T]) Len() int {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	return int(bb.tail - bb.head)
}

// Bytes returns a slice containing all elements currently in the buffer.
//
// This method creates and returns a new slice containing all elements currently
// stored in the buffer. The returned slice is a copy of the buffered data, so
// modifications to it will not affect the buffer.
//
// Due to the circular nature of the buffer, the data may be split across two
// segments in the underlying buffer. This method handles this by concatenating
// the segments using slices.Concat.
//
// The returned slice will be empty if the buffer is empty. This method is
// thread-safe and provides a snapshot of the buffer contents.
func (bb *BlockBuffer[T]) Bytes() []T {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	h := bb.head % int64(len(bb.buf))
	t := bb.tail % int64(len(bb.buf))
	if h < t {
		return bb.buf[h:t]
	}
	return slices.Concat(bb.buf[h:], bb.buf[:t])
}
