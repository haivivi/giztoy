//! Overwriting ring buffer implementation.

use std::error::Error;
use std::sync::{Arc, Condvar, Mutex};

use crate::error::{BufferError, Done};

/// A thread-safe overwriting ring buffer.
///
/// `RingBuffer<T>` is a thread-safe circular buffer with a fixed capacity.
/// When the buffer is full, new writes overwrite the oldest data instead of
/// blocking. This makes it ideal for maintaining a sliding window of the
/// most recent data.
///
/// # Semantics
///
/// - **Read**: Blocks when empty, returns data when available
/// - **Write**: Never blocks, overwrites oldest data when full
/// - **Close**: `close_write()` allows draining, `close_with_error()` immediate
///
/// # Example
///
/// ```
/// use giztoy_buffer::RingBuffer;
///
/// // Keep only the 100 most recent samples
/// let buf = RingBuffer::<f32>::new(100);
///
/// // Write samples - old data automatically overwritten
/// for i in 0..200 {
///     buf.add(i as f32).unwrap();
/// }
///
/// // Buffer contains only the last 100 samples (100..200)
/// assert_eq!(buf.len(), 100);
/// ```
pub struct RingBuffer<T> {
    inner: Arc<RingBufferInner<T>>,
}

struct RingBufferInner<T> {
    state: Mutex<RingBufferState<T>>,
    write_notify: Condvar,
}

struct RingBufferState<T> {
    buf: Vec<Option<T>>,
    // Virtual counters that track total read/write positions. These grow
    // monotonically and will wrap around after 2^64 operations on 64-bit
    // systems (2^32 on 32-bit). The implementation uses wrapping arithmetic
    // to handle this correctly.
    head: usize, // read position (virtual counter)
    tail: usize, // write position (virtual counter)
    close_write: bool,
    close_err: Option<Arc<dyn Error + Send + Sync>>,
}

impl<T> Clone for RingBuffer<T> {
    fn clone(&self) -> Self {
        RingBuffer {
            inner: Arc::clone(&self.inner),
        }
    }
}

impl<T> RingBuffer<T> {
    /// Creates a new RingBuffer with the specified capacity.
    ///
    /// The buffer will overwrite the oldest data when this capacity is exceeded.
    pub fn new(capacity: usize) -> Self {
        assert!(capacity > 0, "capacity must be greater than 0");
        let mut buf = Vec::with_capacity(capacity);
        buf.resize_with(capacity, || None);

        RingBuffer {
            inner: Arc::new(RingBufferInner {
                state: Mutex::new(RingBufferState {
                    buf,
                    head: 0,
                    tail: 0,
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
        // Use wrapping_sub to handle potential overflow correctly
        state.tail.wrapping_sub(state.head).min(state.buf.len())
    }

    /// Returns the buffer capacity.
    pub fn capacity(&self) -> usize {
        let state = self.inner.state.lock().unwrap();
        state.buf.len()
    }

    /// Returns true if the buffer is empty.
    pub fn is_empty(&self) -> bool {
        let state = self.inner.state.lock().unwrap();
        state.head == state.tail
    }

    /// Resets the buffer by clearing all data.
    ///
    /// This does not change the closed state of the buffer.
    pub fn reset(&self) {
        let mut state = self.inner.state.lock().unwrap();
        for slot in &mut state.buf {
            *slot = None;
        }
        state.head = 0;
        state.tail = 0;
    }

    /// Returns the error that caused the buffer to be closed, if any.
    pub fn error(&self) -> Option<Arc<dyn Error + Send + Sync>> {
        let state = self.inner.state.lock().unwrap();
        state.close_err.clone()
    }

    /// Closes the write side of the buffer.
    ///
    /// This prevents new writes but allows existing data to be read.
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
    pub fn close_with_error<E>(&self, err: E) -> Result<(), BufferError>
    where
        E: Error + Send + Sync + 'static,
    {
        let mut state = self.inner.state.lock().unwrap();
        if state.close_err.is_some() {
            return Ok(());
        }
        state.close_err = Some(Arc::new(err));
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

impl<T: Clone> RingBuffer<T> {
    /// Writes data to the buffer.
    ///
    /// If the buffer is full, overwrites the oldest data.
    /// Write operations never block (except when closed).
    /// Returns the number of elements written (always `data.len()` on success).
    pub fn write(&self, data: &[T]) -> Result<usize, BufferError> {
        let mut state = self.inner.state.lock().unwrap();

        // Check for errors
        if let Some(ref err) = state.close_err {
            return Err(BufferError::ClosedWithError(Arc::clone(err)));
        }
        if state.close_write {
            return Err(BufferError::Closed);
        }

        let capacity = state.buf.len();

        // Write each item to the buffer. When the buffer is full, we overwrite
        // the oldest item by advancing head. This maintains the invariant that
        // tail.wrapping_sub(head) <= capacity at all times.
        for item in data {
            let tail_idx = state.tail % capacity;
            state.buf[tail_idx] = Some(item.clone());
            state.tail = state.tail.wrapping_add(1);

            // If buffer is overfull, advance head to drop the oldest item
            if state.tail.wrapping_sub(state.head) > capacity {
                state.head = state.head.wrapping_add(1);
            }
        }

        self.inner.write_notify.notify_one();
        Ok(data.len())
    }

    /// Adds a single element to the buffer.
    ///
    /// If the buffer is full, overwrites the oldest element.
    pub fn add(&self, item: T) -> Result<(), BufferError> {
        let mut state = self.inner.state.lock().unwrap();

        // Check for errors
        if let Some(ref err) = state.close_err {
            return Err(BufferError::ClosedWithError(Arc::clone(err)));
        }
        if state.close_write {
            return Err(BufferError::Closed);
        }

        let capacity = state.buf.len();
        let tail_idx = state.tail % capacity;
        state.buf[tail_idx] = Some(item);
        state.tail = state.tail.wrapping_add(1);

        // If we've exceeded capacity, advance head (overwrite oldest)
        if state.tail.wrapping_sub(state.head) > capacity {
            state.head = state.head.wrapping_add(1);
        }

        self.inner.write_notify.notify_one();
        Ok(())
    }

    /// Reads data from the buffer.
    ///
    /// Blocks when the buffer is empty until data becomes available.
    /// Returns the number of elements actually read.
    pub fn read(&self, buf: &mut [T]) -> Result<usize, BufferError> {
        let mut state = self.inner.state.lock().unwrap();

        // Check for errors
        if let Some(ref err) = state.close_err {
            return Err(BufferError::ClosedWithError(Arc::clone(err)));
        }

        // Wait for data
        while state.head == state.tail {
            if state.close_write {
                return Ok(0);
            }
            state = self.inner.write_notify.wait(state).unwrap();
            if let Some(ref err) = state.close_err {
                return Err(BufferError::ClosedWithError(Arc::clone(err)));
            }
        }

        let capacity = state.buf.len();
        let available = state.tail.wrapping_sub(state.head).min(capacity);
        let n = std::cmp::min(buf.len(), available);

        for i in 0..n {
            let head_idx = state.head % capacity;
            buf[i] = state.buf[head_idx].take().unwrap();
            state.head = state.head.wrapping_add(1);
        }

        Ok(n)
    }

    /// Returns the next element from the buffer (iterator pattern).
    ///
    /// Blocks when the buffer is empty until data becomes available.
    /// Returns `Err(Done)` when the buffer is closed and empty.
    pub fn next(&self) -> Result<T, Done> {
        let mut state = self.inner.state.lock().unwrap();

        // Check for errors
        if state.close_err.is_some() {
            return Err(Done);
        }

        // Wait for data
        while state.head == state.tail {
            if state.close_write {
                return Err(Done);
            }
            state = self.inner.write_notify.wait(state).unwrap();
            if state.close_err.is_some() {
                return Err(Done);
            }
        }

        // Read from head
        let capacity = state.buf.len();
        let head_idx = state.head % capacity;
        let item = state.buf[head_idx].take().unwrap();
        state.head = state.head.wrapping_add(1);

        Ok(item)
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

        let capacity = state.buf.len();
        let available = state.tail.wrapping_sub(state.head).min(capacity);
        let discard_count = std::cmp::min(n, available);

        for _ in 0..discard_count {
            let head_idx = state.head % capacity;
            state.buf[head_idx] = None;
            state.head = state.head.wrapping_add(1);
        }

        Ok(())
    }

    /// Returns a copy of all elements in the buffer.
    pub fn to_vec(&self) -> Vec<T> {
        let state = self.inner.state.lock().unwrap();
        let capacity = state.buf.len();
        let count = state.tail.wrapping_sub(state.head).min(capacity);
        let mut result = Vec::with_capacity(count);

        for i in 0..count {
            let idx = state.head.wrapping_add(i) % capacity;
            if let Some(ref item) = state.buf[idx] {
                result.push(item.clone());
            }
        }

        result
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::thread;
    use std::time::Duration;

    #[test]
    fn test_basic_write_read() {
        let buf = RingBuffer::<i32>::new(4);
        buf.write(&[1, 2, 3]).unwrap();

        let mut data = vec![0; 3];
        let n = buf.read(&mut data).unwrap();
        assert_eq!(n, 3);
        assert_eq!(data, vec![1, 2, 3]);
    }

    #[test]
    fn test_add_next() {
        let buf = RingBuffer::<i32>::new(4);
        buf.add(1).unwrap();
        buf.add(2).unwrap();
        buf.add(3).unwrap();

        assert_eq!(buf.next().unwrap(), 1);
        assert_eq!(buf.next().unwrap(), 2);
        assert_eq!(buf.next().unwrap(), 3);
    }

    #[test]
    fn test_overwrite_oldest() {
        let buf = RingBuffer::<i32>::new(3);

        // Fill buffer
        buf.write(&[1, 2, 3]).unwrap();
        assert_eq!(buf.len(), 3);

        // Write more - should overwrite oldest
        buf.write(&[4, 5]).unwrap();
        assert_eq!(buf.len(), 3);

        // Should get 3, 4, 5 (1 and 2 were overwritten)
        assert_eq!(buf.next().unwrap(), 3);
        assert_eq!(buf.next().unwrap(), 4);
        assert_eq!(buf.next().unwrap(), 5);
    }

    #[test]
    fn test_overwrite_more_than_capacity() {
        let buf = RingBuffer::<i32>::new(3);

        // Write more than capacity at once
        buf.write(&[1, 2, 3, 4, 5, 6, 7]).unwrap();

        // Should only have last 3 elements
        assert_eq!(buf.len(), 3);
        assert_eq!(buf.to_vec(), vec![5, 6, 7]);
    }

    #[test]
    fn test_close_write() {
        let buf = RingBuffer::<i32>::new(4);
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
    fn test_capacity_and_len() {
        let buf = RingBuffer::<i32>::new(4);
        assert_eq!(buf.capacity(), 4);
        assert_eq!(buf.len(), 0);
        assert!(buf.is_empty());

        buf.write(&[1, 2, 3]).unwrap();
        assert_eq!(buf.len(), 3);
        assert!(!buf.is_empty());
    }

    #[test]
    fn test_concurrent_producer_consumer() {
        let buf = RingBuffer::<i32>::new(16);
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
        // Due to overwriting, we might not get all 100
        assert!(!collected.is_empty());
    }

    #[test]
    fn test_reset() {
        let buf = RingBuffer::<i32>::new(4);
        buf.write(&[1, 2, 3]).unwrap();
        assert_eq!(buf.len(), 3);

        buf.reset();
        assert_eq!(buf.len(), 0);
        assert!(buf.is_empty());
    }

    #[test]
    fn test_discard() {
        let buf = RingBuffer::<i32>::new(8);
        buf.write(&[1, 2, 3, 4, 5]).unwrap();

        buf.discard(2).unwrap();
        assert_eq!(buf.len(), 3);
        assert_eq!(buf.next().unwrap(), 3);
    }

    #[test]
    fn test_to_vec() {
        let buf = RingBuffer::<i32>::new(4);
        buf.write(&[1, 2, 3]).unwrap();

        let data = buf.to_vec();
        assert_eq!(data, vec![1, 2, 3]);

        // Original buffer unchanged
        assert_eq!(buf.len(), 3);
    }

    #[test]
    fn test_sliding_window() {
        let buf = RingBuffer::<i32>::new(5);

        // Simulate streaming data
        for i in 0..20 {
            buf.add(i).unwrap();
        }

        // Should contain last 5 values
        let data = buf.to_vec();
        assert_eq!(data, vec![15, 16, 17, 18, 19]);
    }

    #[test]
    fn test_blocking_read() {
        let buf = RingBuffer::<i32>::new(4);
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

    #[test]
    fn test_close_with_error() {
        let buf = RingBuffer::<i32>::new(4);
        buf.add(1).unwrap();

        let err = std::io::Error::new(std::io::ErrorKind::Other, "test error");
        buf.close_with_error(err).unwrap();

        // All operations return error
        assert!(buf.add(2).is_err());
        assert!(buf.read(&mut [0]).is_err());
    }

    #[test]
    fn test_error_method() {
        let buf = RingBuffer::<i32>::new(4);

        // No error initially
        assert!(buf.error().is_none());

        // After close_with_error, error() returns the error
        let err = std::io::Error::new(std::io::ErrorKind::Other, "test error");
        buf.close_with_error(err).unwrap();
        assert!(buf.error().is_some());
    }
}
