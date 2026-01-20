package buffer

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

// ErrIteratorDone is returned when iteration is complete.
var ErrIteratorDone = errors.New("iterator done")

// Buffer is a thread-safe growable buffer that implements io.Reader and
// io.Writer interfaces. Unlike BlockBuffer, this buffer automatically grows to
// accommodate data as needed, making it suitable for scenarios where the data
// size is unknown or highly variable.
//
// The buffer uses a dynamic slice that grows as needed and maintains a write
// notification channel for blocking read operations. When the buffer is empty,
// read operations block until data is written. When the buffer is closed for
// writing, read operations return io.EOF once all data has been consumed.
//
// The buffer supports graceful shutdown through CloseWrite() (which allows
// reads to continue until empty) or CloseWithError() (which immediately closes
// both ends). All blocking operations are unblocked when the buffer is closed.
type Buffer[T any] struct {
	writeNotify chan struct{}

	mu         sync.Mutex
	closeWrite bool
	closeErr   error
	buf        []T
}

// N creates a new Buffer with the specified initial capacity.
//
// This function allocates a new Buffer with an initial capacity of n elements.
// The buffer will grow automatically as needed when data is written. The
// writeNotify channel is initialized with a buffer size of 1 to allow
// non-blocking notification of waiting readers.
//
// The initial capacity is a hint for performance optimization - the buffer will
// grow beyond this capacity if needed.
func N[T any](n int) *Buffer[T] {
	return &Buffer[T]{
		writeNotify: make(chan struct{}, 1),
		buf:         make([]T, 0, n),
	}
}

// Write writes data from the provided slice to the buffer.
//
// This method implements the io.Writer interface. It appends all elements from
// the provided slice to the buffer, growing the underlying slice as needed. The
// buffer automatically expands to accommodate the new data.
//
// After writing data, this method attempts to notify any waiting readers by
// sending a signal on the writeNotify channel. The notification is non-blocking
// - if the channel is full, the notification is dropped.
//
// Returns the number of elements written (always len(p) on success) and any
// error encountered. Returns io.ErrClosedPipe if the buffer is closed for
// writing.
func (b *Buffer[T]) Write(p []T) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closeErr != nil {
		return 0, fmt.Errorf("buffer: write to closed buffer: %w", b.closeErr)
	}
	if b.closeWrite {
		return 0, fmt.Errorf("buffer: write to closed buffer: %w", io.ErrClosedPipe)
	}
	select {
	case b.writeNotify <- struct{}{}:
	default:
	}

	b.buf = append(b.buf, p...)
	return len(p), nil
}

// Discard removes and discards the next n elements from the buffer without
// reading them.
//
// This operation removes elements from the beginning of the buffer by slicing
// the internal buffer. If n is greater than the number of available elements,
// all available data is discarded and the buffer becomes empty.
//
// This is more efficient than reading and discarding data, as it avoids copying
// elements. The buffer capacity is not reduced by this operation.
//
// Returns an error if the buffer has been closed with an error.
func (b *Buffer[T]) Discard(n int) (err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closeErr != nil {
		return fmt.Errorf("buffer: skip from closed buffer: %w", b.closeErr)
	}
	if n > len(b.buf) {
		b.buf = b.buf[:0]
		return nil
	}
	b.buf = b.buf[n:]
	return nil
}

// Read reads data from the buffer into the provided slice.
//
// This method implements the io.Reader interface. It blocks until at least one
// element is available in the buffer or the buffer is closed. It attempts to
// read as many elements as possible up to len(p).
//
// The method uses a channel-based notification system to efficiently block
// waiting readers. When the buffer is empty, the method unlocks the mutex and
// waits for a write notification, then re-acquires the mutex to check the
// buffer state again.
//
// Returns the number of elements actually read and any error encountered.
// Returns io.EOF if the buffer is closed for writing and empty.
func (b *Buffer[T]) Read(p []T) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closeErr != nil {
		return 0, fmt.Errorf("buffer: read from closed buffer: %w", b.closeErr)
	}

	for len(b.buf) == 0 {
		if b.closeWrite {
			return 0, io.EOF
		}
		b.mu.Unlock()
		<-b.writeNotify
		b.mu.Lock()
		if b.closeErr != nil {
			return 0, fmt.Errorf("buffer: read from closed buffer: %w", b.closeErr)
		}
	}
	n = copy(p, b.buf)
	b.buf = b.buf[n:]
	return n, nil
}

func (b *Buffer[T]) closeWithErrorLocked(err error) error {
	if b.closeErr != nil {
		return nil
	}
	b.closeErr = err
	b.buf = nil
	if !b.closeWrite {
		b.closeWrite = true
		close(b.writeNotify)
	}
	return nil
}

// CloseWithError closes the buffer with the specified error.
//
// This method immediately closes both the read and write sides of the buffer.
// All pending read and write operations will be unblocked and return the
// specified error. If err is nil, io.ErrClosedPipe is used as the default
// error.
//
// Once closed, all subsequent read and write operations will return the
// specified error. The internal buffer is set to nil to free memory. The
// writeNotify channel is closed to unblock any waiting readers.
//
// Returns nil if the buffer was already closed, or the error if this is the
// first time the buffer is being closed.
func (b *Buffer[T]) CloseWithError(err error) error {
	if err == nil {
		err = io.ErrClosedPipe
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closeWithErrorLocked(err)
}

// Error returns the error that caused the buffer to be closed, if any.
//
// This method returns the error that was passed to CloseWithError() when the
// buffer was closed. If the buffer has not been closed or was closed without an
// error, this method returns nil.
//
// This is useful for determining why a buffer was closed and handling the error
// appropriately in the application.
func (b *Buffer[T]) Error() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closeErr
}

// Close closes the buffer, preventing further writes and reads.
//
// This method is equivalent to CloseWithError(io.ErrClosedPipe). It immediately
// closes both the read and write sides of the buffer and unblocks all pending
// operations with io.ErrClosedPipe.
//
// This method implements the io.Closer interface.
func (b *Buffer[T]) Close() error {
	return b.CloseWithError(io.ErrClosedPipe)
}

// CloseWrite closes the write side of the buffer, preventing further writes.
//
// This method provides a graceful shutdown mechanism. It prevents new writes to
// the buffer while allowing existing data to be read. Once the buffer is empty,
// subsequent read operations will return io.EOF.
//
// The writeNotify channel is closed to unblock any waiting readers, allowing
// them to check the buffer state and return io.EOF if the buffer is empty.
//
// This is useful for signaling the end of a data stream while allowing
// consumers to finish reading any remaining buffered data.
//
// Returns nil if the write side was already closed.
func (b *Buffer[T]) CloseWrite() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closeWrite {
		return nil
	}
	b.closeWrite = true
	close(b.writeNotify)
	return nil
}

// Next reads and returns the next element from the buffer.
//
// This method implements the iterator pattern, reading one element at a time.
// It blocks until an element is available in the buffer or the buffer is
// closed.
//
// Note: The current implementation reads from the end of the buffer (LIFO
// behavior) rather than the beginning (FIFO behavior). This may not be the
// intended behavior for a typical buffer implementation.
//
// Returns the next element and any error encountered. If the buffer is closed
// for writing and empty, returns ErrIteratorDone to signal the end of iteration.
func (b *Buffer[T]) Next() (t T, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closeErr != nil {
		err = fmt.Errorf("buffer: read from closed buffer: %w", b.closeErr)
		return
	}
	for len(b.buf) == 0 {
		if b.closeWrite {
			err = ErrIteratorDone
			return
		}
		b.mu.Unlock()
		<-b.writeNotify
		b.mu.Lock()
		if b.closeErr != nil {
			err = fmt.Errorf("buffer: read from closed buffer: %w", b.closeErr)
			return
		}
	}
	head := len(b.buf) - 1
	t = b.buf[head]
	b.buf = b.buf[:head]
	return
}

// Add adds a single element directly to the buffer.
//
// This method is a convenience function for adding individual elements to the
// buffer. It appends the element to the end of the buffer, growing the buffer
// as needed.
//
// This is more efficient than using Write() with a single-element slice for
// adding individual elements, as it avoids slice allocation.
//
// The method attempts to notify waiting readers by sending a signal on the
// writeNotify channel, though the notification may be dropped if the channel is
// full.
//
// Returns an error if the buffer is closed for writing or has been closed with
// an error.
func (b *Buffer[T]) Add(t T) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closeErr != nil {
		return fmt.Errorf("buffer: write to closed buffer: %w", b.closeErr)
	}
	if b.closeWrite {
		return fmt.Errorf("buffer: write to closed buffer: %w", io.ErrClosedPipe)
	}
	b.buf = append(b.buf, t)
	return nil
}

// Reset clears the buffer by setting its length to zero.
//
// This method effectively discards all buffered data by slicing the internal
// buffer to length zero. The underlying slice capacity is preserved, so
// subsequent writes will not need to reallocate memory until the capacity is
// exceeded.
//
// This operation is not affected by the buffer's closed state - it can be
// called even if the buffer has been closed. However, it does not reopen a
// closed buffer.
func (b *Buffer[T]) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = b.buf[:0]
}

// Len returns the number of elements currently in the buffer.
//
// This method returns the current length of the buffer, which is the number of
// elements that can be read before the buffer becomes empty. It provides a
// snapshot of the buffer size at the time of the call.
//
// The returned value is always between 0 and the buffer's capacity. This method
// is thread-safe and can be called concurrently with other operations.
func (b *Buffer[T]) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.buf)
}

// Bytes returns a slice containing all elements currently in the buffer.
//
// This method returns a reference to the internal buffer slice. The returned
// slice is not a copy, so modifications to it will affect the buffer contents.
// Callers should not modify the returned slice.
//
// The returned slice represents the current state of the buffer at the time of
// the call. If the buffer is empty, an empty slice is returned.
//
// This method is thread-safe and provides a snapshot of the buffer contents,
// but the returned slice should be used immediately as concurrent modifications
// may invalidate it.
func (b *Buffer[T]) Bytes() []T {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf
}
