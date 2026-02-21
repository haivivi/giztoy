//! Growable buffer implementation.

use std::collections::VecDeque;
use std::error::Error;
use std::sync::{Arc, Condvar, Mutex};

use crate::error::{BufferError, Done};

/// A thread-safe growable buffer.
///
/// `Buffer<T>` is a thread-safe growable buffer that automatically expands to
/// accommodate data as needed. Unlike `BlockBuffer`, this buffer never blocks
/// on write operations (unless closed).
///
/// # Semantics
///
/// - **Read**: Blocks when empty, returns data when available
/// - **Write**: Never blocks (auto-grows), fails only when closed
/// - **Close**: `close_write()` allows draining, `close_with_error()` immediate
///
/// # Example
///
/// ```
/// use giztoy_buffer::Buffer;
/// use std::thread;
///
/// let buf = Buffer::<i32>::new();
/// let buf_clone = buf.clone();
///
/// // Producer thread
/// let producer = thread::spawn(move || {
///     for i in 0..10 {
///         buf_clone.add(i).unwrap();
///     }
///     buf_clone.close_write().unwrap();
/// });
///
/// // Consumer thread
/// let mut items = Vec::new();
/// loop {
///     match buf.next() {
///         Ok(item) => items.push(item),
///         Err(_) => break,
///     }
/// }
///
/// producer.join().unwrap();
/// assert_eq!(items.len(), 10);
/// ```
pub struct Buffer<T> {
    inner: Arc<BufferInner<T>>,
}

struct BufferInner<T> {
    state: Mutex<BufferState<T>>,
    write_notify: Condvar,
}

struct BufferState<T> {
    buf: VecDeque<T>,
    close_write: bool,
    close_err: Option<Arc<dyn Error + Send + Sync>>,
}

impl<T> Clone for Buffer<T> {
    fn clone(&self) -> Self {
        Buffer {
            inner: Arc::clone(&self.inner),
        }
    }
}

impl<T> Default for Buffer<T> {
    fn default() -> Self {
        Self::new()
    }
}

impl<T> Buffer<T> {
    /// Creates a new Buffer with default capacity.
    pub fn new() -> Self {
        Self::with_capacity(0)
    }

    /// Creates a new Buffer with the specified initial capacity.
    ///
    /// The buffer will grow automatically as needed when data is written.
    /// The initial capacity is a hint for performance optimization.
    pub fn with_capacity(capacity: usize) -> Self {
        Buffer {
            inner: Arc::new(BufferInner {
                state: Mutex::new(BufferState {
                    buf: VecDeque::with_capacity(capacity),
                    close_write: false,
                    close_err: None,
                }),
                write_notify: Condvar::new(),
            }),
        }
    }

    /// Returns the number of elements currently in the buffer.
    pub fn len(&self) -> usize {
        let state = self.inner.state.lock().unwrap();
        state.buf.len()
    }

    /// Returns true if the buffer is empty.
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }

    /// Resets the buffer by clearing all data.
    ///
    /// This does not change the closed state of the buffer.
    pub fn reset(&self) {
        let mut state = self.inner.state.lock().unwrap();
        state.buf.clear();
    }

    /// Returns the error that caused the buffer to be closed, if any.
    pub fn error(&self) -> Option<Arc<dyn Error + Send + Sync>> {
        let state = self.inner.state.lock().unwrap();
        state.close_err.clone()
    }

    /// Closes the write side of the buffer.
    ///
    /// This prevents new writes but allows existing data to be read.
    /// Once the buffer is empty, `read()` returns `Ok(0)` and `next()` returns `Err(Done)`.
    pub fn close_write(&self) -> Result<(), BufferError> {
        let mut state = self.inner.state.lock().unwrap();
        if state.close_write {
            return Ok(());
        }
        state.close_write = true;
        self.inner.write_notify.notify_all();
        Ok(())
    }

    /// Closes the buffer with the specified error.
    ///
    /// All blocking operations are immediately unblocked and return the error.
    /// The internal buffer is cleared to free memory.
    pub fn close_with_error<E>(&self, err: E) -> Result<(), BufferError>
    where
        E: Error + Send + Sync + 'static,
    {
        let mut state = self.inner.state.lock().unwrap();
        if state.close_err.is_some() {
            return Ok(());
        }
        state.close_err = Some(Arc::new(err));
        state.buf.clear();
        if !state.close_write {
            state.close_write = true;
        }
        self.inner.write_notify.notify_all();
        Ok(())
    }

    /// Closes the buffer.
    ///
    /// This is semantically equivalent to `close_write()`, allowing readers to
    /// drain any remaining items while preventing further writes.
    pub fn close(&self) -> Result<(), BufferError> {
        self.close_write()
    }
}

impl<T: Clone> Buffer<T> {
    /// Writes data to the buffer.
    ///
    /// Appends all elements from `data` to the buffer, growing as needed.
    /// Returns the number of elements written (always `data.len()` on success).
    ///
    /// Returns an error if the buffer is closed.
    pub fn write(&self, data: &[T]) -> Result<usize, BufferError> {
        if data.is_empty() {
            return Ok(0);
        }

        let mut state = self.inner.state.lock().unwrap();
        if let Some(ref err) = state.close_err {
            return Err(BufferError::ClosedWithError(Arc::clone(err)));
        }
        if state.close_write {
            return Err(BufferError::Closed);
        }
        state.buf.extend(data.iter().cloned());
        self.inner.write_notify.notify_one();
        Ok(data.len())
    }

    /// Adds a single element to the buffer.
    ///
    /// This is more efficient than `write()` for single elements.
    pub fn add(&self, item: T) -> Result<(), BufferError> {
        let mut state = self.inner.state.lock().unwrap();
        if let Some(ref err) = state.close_err {
            return Err(BufferError::ClosedWithError(Arc::clone(err)));
        }
        if state.close_write {
            return Err(BufferError::Closed);
        }
        state.buf.push_back(item);
        self.inner.write_notify.notify_one();
        Ok(())
    }

    /// Reads data from the buffer.
    ///
    /// Blocks until data is available or the buffer is closed.
    /// Returns the number of elements actually read.
    /// Returns `Ok(0)` when the buffer is closed and empty.
    pub fn read(&self, buf: &mut [T]) -> Result<usize, BufferError> {
        let mut state = self.inner.state.lock().unwrap();

        // Check for close error first
        if let Some(ref err) = state.close_err {
            return Err(BufferError::ClosedWithError(Arc::clone(err)));
        }

        // Wait for data
        while state.buf.is_empty() {
            if state.close_write {
                return Ok(0);
            }
            state = self.inner.write_notify.wait(state).unwrap();
            if let Some(ref err) = state.close_err {
                return Err(BufferError::ClosedWithError(Arc::clone(err)));
            }
        }

        // Read from the front of the buffer (FIFO) - O(1) per element with VecDeque
        let n = std::cmp::min(buf.len(), state.buf.len());
        for item in buf.iter_mut().take(n) {
            *item = state.buf.pop_front().unwrap();
        }
        Ok(n)
    }

    /// Returns the next element from the buffer (iterator pattern).
    ///
    /// Blocks until data is available or the buffer is closed.
    /// Returns `Err(Done)` when the buffer is closed and empty.
    ///
    /// Note: This reads from the front of the buffer (FIFO order).
    pub fn next(&self) -> Result<T, Done> {
        let mut state = self.inner.state.lock().unwrap();

        // Check for close error first
        if state.close_err.is_some() {
            return Err(Done);
        }

        // Wait for data
        while state.buf.is_empty() {
            if state.close_write {
                return Err(Done);
            }
            state = self.inner.write_notify.wait(state).unwrap();
            if state.close_err.is_some() {
                return Err(Done);
            }
        }

        // Read from the front of the buffer (FIFO) - O(1) with VecDeque
        Ok(state.buf.pop_front().unwrap())
    }

    /// Discards the next n elements from the buffer.
    ///
    /// If n is greater than the number of available elements,
    /// all elements are discarded.
    pub fn discard(&self, n: usize) -> Result<(), BufferError> {
        let mut state = self.inner.state.lock().unwrap();
        if let Some(ref err) = state.close_err {
            return Err(BufferError::ClosedWithError(Arc::clone(err)));
        }
        let discard_count = std::cmp::min(n, state.buf.len());
        // O(1) per element with VecDeque
        for _ in 0..discard_count {
            state.buf.pop_front();
        }
        Ok(())
    }

    /// Returns a copy of all elements in the buffer.
    pub fn to_vec(&self) -> Vec<T> {
        let state = self.inner.state.lock().unwrap();
        state.buf.iter().cloned().collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::thread;
    use std::time::Duration;

    #[test]
    fn test_basic_write_read() {
        let buf = Buffer::<i32>::new();
        buf.write(&[1, 2, 3]).unwrap();

        let mut data = vec![0; 3];
        let n = buf.read(&mut data).unwrap();
        assert_eq!(n, 3);
        assert_eq!(data, vec![1, 2, 3]);
    }

    #[test]
    fn test_add_next() {
        let buf = Buffer::<i32>::new();
        buf.add(1).unwrap();
        buf.add(2).unwrap();
        buf.add(3).unwrap();

        assert_eq!(buf.next().unwrap(), 1);
        assert_eq!(buf.next().unwrap(), 2);
        assert_eq!(buf.next().unwrap(), 3);
    }

    #[test]
    fn test_close_write() {
        let buf = Buffer::<i32>::new();
        buf.add(1).unwrap();
        buf.close_write().unwrap();

        // Can still read existing data
        assert_eq!(buf.next().unwrap(), 1);

        // Now returns Done
        assert_eq!(buf.next(), Err(Done));

        // Cannot write after close
        assert!(buf.add(2).is_err());
    }

    #[test]
    fn test_close_with_error() {
        let buf = Buffer::<i32>::new();
        buf.add(1).unwrap();

        let err = std::io::Error::new(std::io::ErrorKind::Other, "test error");
        buf.close_with_error(err).unwrap();

        // All operations return error
        assert!(buf.add(2).is_err());
        assert!(buf.read(&mut [0]).is_err());
    }

    #[test]
    fn test_error_method() {
        let buf = Buffer::<i32>::new();

        // No error initially
        assert!(buf.error().is_none());

        // After close_with_error, error() returns the error
        let err = std::io::Error::new(std::io::ErrorKind::Other, "test error");
        buf.close_with_error(err).unwrap();
        assert!(buf.error().is_some());
    }

    #[test]
    fn test_concurrent_producer_consumer() {
        let buf = Buffer::<i32>::new();
        let producer_buf = buf.clone();

        let producer = thread::spawn(move || {
            for i in 0..100 {
                producer_buf.add(i).unwrap();
                thread::sleep(Duration::from_micros(10));
            }
            producer_buf.close_write().unwrap();
        });

        let mut collected = Vec::new();
        loop {
            match buf.next() {
                Ok(item) => collected.push(item),
                Err(Done) => break,
            }
        }

        producer.join().unwrap();
        assert_eq!(collected.len(), 100);
        for (i, &item) in collected.iter().enumerate() {
            assert_eq!(item, i as i32);
        }
    }

    #[test]
    fn test_len_and_is_empty() {
        let buf = Buffer::<i32>::new();
        assert!(buf.is_empty());
        assert_eq!(buf.len(), 0);

        buf.add(1).unwrap();
        assert!(!buf.is_empty());
        assert_eq!(buf.len(), 1);
    }

    #[test]
    fn test_reset() {
        let buf = Buffer::<i32>::new();
        buf.write(&[1, 2, 3]).unwrap();
        assert_eq!(buf.len(), 3);

        buf.reset();
        assert_eq!(buf.len(), 0);
    }

    #[test]
    fn test_discard() {
        let buf = Buffer::<i32>::new();
        buf.write(&[1, 2, 3, 4, 5]).unwrap();

        buf.discard(2).unwrap();
        assert_eq!(buf.len(), 3);
        assert_eq!(buf.next().unwrap(), 3);
    }

    #[test]
    fn test_to_vec() {
        let buf = Buffer::<i32>::new();
        buf.write(&[1, 2, 3]).unwrap();

        let data = buf.to_vec();
        assert_eq!(data, vec![1, 2, 3]);

        // Original buffer unchanged
        assert_eq!(buf.len(), 3);
    }

    #[test]
    fn test_blocking_read() {
        let buf = Buffer::<i32>::new();
        let reader_buf = buf.clone();

        let reader = thread::spawn(move || {
            // This should block until data is available
            reader_buf.next().unwrap()
        });

        // Give the reader time to block
        thread::sleep(Duration::from_millis(10));

        // Write data to unblock the reader
        buf.add(42).unwrap();

        let result = reader.join().unwrap();
        assert_eq!(result, 42);
    }
}
