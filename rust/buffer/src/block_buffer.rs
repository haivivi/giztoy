//! Fixed-size blocking buffer implementation.

use std::error::Error;
use std::sync::{Arc, Condvar, Mutex};

use crate::error::{BufferError, Done};

/// A thread-safe fixed-size blocking buffer.
///
/// `BlockBuffer<T>` is a thread-safe circular buffer with a fixed capacity.
/// It blocks on write when full and blocks on read when empty, providing
/// backpressure for flow control between producers and consumers.
///
/// # Semantics
///
/// - **Read**: Blocks when empty, returns data when available
/// - **Write**: Blocks when full, writes when space available
/// - **Close**: `close_write()` allows draining, `close_with_error()` immediate
///
/// # Example
///
/// ```
/// use giztoy_buffer::BlockBuffer;
/// use std::thread;
///
/// let buf = BlockBuffer::<i32>::new(4);
/// let buf_clone = buf.clone();
///
/// // Producer thread (will block when buffer is full)
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
pub struct BlockBuffer<T> {
    inner: Arc<BlockBufferInner<T>>,
}

struct BlockBufferInner<T> {
    state: Mutex<BlockBufferState<T>>,
    not_full: Condvar,
    not_empty: Condvar,
}

struct BlockBufferState<T> {
    buf: Vec<Option<T>>,
    head: usize,          // read position
    tail: usize,          // write position
    count: usize,         // current element count
    close_write: bool,
    close_err: Option<Arc<dyn Error + Send + Sync>>,
}

impl<T> Clone for BlockBuffer<T> {
    fn clone(&self) -> Self {
        BlockBuffer {
            inner: Arc::clone(&self.inner),
        }
    }
}

impl<T> BlockBuffer<T> {
    /// Creates a new BlockBuffer with the specified capacity.
    ///
    /// The buffer will block on write when this capacity is reached.
    pub fn new(capacity: usize) -> Self {
        assert!(capacity > 0, "capacity must be greater than 0");
        let mut buf = Vec::with_capacity(capacity);
        buf.resize_with(capacity, || None);

        BlockBuffer {
            inner: Arc::new(BlockBufferInner {
                state: Mutex::new(BlockBufferState {
                    buf,
                    head: 0,
                    tail: 0,
                    count: 0,
                    close_write: false,
                    close_err: None,
                }),
                not_full: Condvar::new(),
                not_empty: Condvar::new(),
            }),
        }
    }

    /// Creates a BlockBuffer from an existing Vec.
    ///
    /// The Vec's length determines the buffer capacity. The buffer is created
    /// full, so writes will block until some data is read.
    ///
    /// # Panics
    ///
    /// Panics if the Vec is empty.
    pub fn from_vec(data: Vec<T>) -> Self {
        let capacity = data.len();
        assert!(capacity > 0, "capacity must be greater than 0");

        let count = data.len();
        let buf: Vec<Option<T>> = data.into_iter().map(Some).collect();

        // Set tail to next write position (count % capacity = 0 for full buffer)
        let tail = count % capacity;

        BlockBuffer {
            inner: Arc::new(BlockBufferInner {
                state: Mutex::new(BlockBufferState {
                    buf,
                    head: 0,
                    tail,
                    count,
                    close_write: false,
                    close_err: None,
                }),
                not_full: Condvar::new(),
                not_empty: Condvar::new(),
            }),
        }
    }

    /// Returns the number of elements currently in the buffer.
    pub fn len(&self) -> usize {
        let state = self.inner.state.lock().unwrap();
        state.count
    }

    /// Returns the buffer capacity.
    pub fn capacity(&self) -> usize {
        let state = self.inner.state.lock().unwrap();
        state.buf.len()
    }

    /// Returns true if the buffer is empty.
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }

    /// Returns true if the buffer is full.
    pub fn is_full(&self) -> bool {
        let state = self.inner.state.lock().unwrap();
        state.count == state.buf.len()
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
        state.count = 0;
        self.inner.not_full.notify_all();
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
        self.inner.not_empty.notify_all();
        self.inner.not_full.notify_all();
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
        self.inner.not_empty.notify_all();
        self.inner.not_full.notify_all();
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

impl<T: Clone> BlockBuffer<T> {
    /// Writes data to the buffer.
    ///
    /// Blocks when the buffer is full until space becomes available.
    /// Returns the number of elements actually written.
    pub fn write(&self, data: &[T]) -> Result<usize, BufferError> {
        if data.is_empty() {
            return Ok(0);
        }

        let mut state = self.inner.state.lock().unwrap();
        let mut written = 0;

        loop {
            // Check for close/error conditions
            if let Some(ref err) = state.close_err {
                return if written > 0 {
                    Ok(written)
                } else {
                    Err(BufferError::ClosedWithError(Arc::clone(err)))
                };
            }
            if state.close_write {
                return if written > 0 {
                    Ok(written)
                } else {
                    Err(BufferError::Closed)
                };
            }

            let capacity = state.buf.len();
            let available_space = capacity - state.count;

            if available_space > 0 {
                // Write as many items as possible in this batch
                let to_write = std::cmp::min(available_space, data.len() - written);

                for i in 0..to_write {
                    let tail = state.tail;
                    state.buf[tail] = Some(data[written + i].clone());
                    state.tail = (tail + 1) % capacity;
                }
                state.count += to_write;
                written += to_write;

                self.inner.not_empty.notify_one();

                if written == data.len() {
                    return Ok(written);
                }
            }

            // Buffer is full, wait for space
            state = self.inner.not_full.wait(state).unwrap();
        }
    }

    /// Adds a single element to the buffer.
    ///
    /// Blocks when the buffer is full until space becomes available.
    pub fn add(&self, item: T) -> Result<(), BufferError> {
        let mut state = self.inner.state.lock().unwrap();

        // Check for errors
        if let Some(ref err) = state.close_err {
            return Err(BufferError::ClosedWithError(Arc::clone(err)));
        }
        if state.close_write {
            return Err(BufferError::Closed);
        }

        // Wait for space
        while state.count == state.buf.len() {
            state = self.inner.not_full.wait(state).unwrap();
            if let Some(ref err) = state.close_err {
                return Err(BufferError::ClosedWithError(Arc::clone(err)));
            }
            if state.close_write {
                return Err(BufferError::Closed);
            }
        }

        // Write the item
        let tail = state.tail;
        let capacity = state.buf.len();
        state.buf[tail] = Some(item);
        state.tail = (tail + 1) % capacity;
        state.count += 1;

        self.inner.not_empty.notify_one();
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
        while state.count == 0 {
            if state.close_write {
                return Ok(0);
            }
            state = self.inner.not_empty.wait(state).unwrap();
            if let Some(ref err) = state.close_err {
                return Err(BufferError::ClosedWithError(Arc::clone(err)));
            }
        }

        // Read as much as possible
        let n = std::cmp::min(buf.len(), state.count);
        let capacity = state.buf.len();
        for i in 0..n {
            let head = state.head;
            buf[i] = state.buf[head].take().unwrap();
            state.head = (head + 1) % capacity;
            state.count -= 1;
        }

        self.inner.not_full.notify_one();
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
        while state.count == 0 {
            if state.close_write {
                return Err(Done);
            }
            state = self.inner.not_empty.wait(state).unwrap();
            if state.close_err.is_some() {
                return Err(Done);
            }
        }

        // Read from head
        let head = state.head;
        let capacity = state.buf.len();
        let item = state.buf[head].take().unwrap();
        state.head = (head + 1) % capacity;
        state.count -= 1;

        self.inner.not_full.notify_one();
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

        let discard_count = std::cmp::min(n, state.count);
        let capacity = state.buf.len();
        for _ in 0..discard_count {
            let head = state.head;
            state.buf[head] = None;
            state.head = (head + 1) % capacity;
            state.count -= 1;
        }

        self.inner.not_full.notify_all();
        Ok(())
    }

    /// Returns a copy of all elements in the buffer.
    pub fn to_vec(&self) -> Vec<T> {
        let state = self.inner.state.lock().unwrap();
        let mut result = Vec::with_capacity(state.count);

        let mut idx = state.head;
        for _ in 0..state.count {
            if let Some(ref item) = state.buf[idx] {
                result.push(item.clone());
            }
            idx = (idx + 1) % state.buf.len();
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
        let buf = BlockBuffer::<i32>::new(4);
        buf.write(&[1, 2, 3]).unwrap();

        let mut data = vec![0; 3];
        let n = buf.read(&mut data).unwrap();
        assert_eq!(n, 3);
        assert_eq!(data, vec![1, 2, 3]);
    }

    #[test]
    fn test_add_next() {
        let buf = BlockBuffer::<i32>::new(4);
        buf.add(1).unwrap();
        buf.add(2).unwrap();
        buf.add(3).unwrap();

        assert_eq!(buf.next().unwrap(), 1);
        assert_eq!(buf.next().unwrap(), 2);
        assert_eq!(buf.next().unwrap(), 3);
    }

    #[test]
    fn test_close_write() {
        let buf = BlockBuffer::<i32>::new(4);
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
    fn test_blocking_write() {
        let buf = BlockBuffer::<i32>::new(2);
        let writer_buf = buf.clone();

        let writer = thread::spawn(move || {
            // First two writes should succeed immediately
            writer_buf.add(1).unwrap();
            writer_buf.add(2).unwrap();
            // Third write should block until consumer reads
            writer_buf.add(3).unwrap();
        });

        // Give writer time to fill buffer and block
        thread::sleep(Duration::from_millis(50));

        // Read to unblock writer
        assert_eq!(buf.next().unwrap(), 1);

        writer.join().unwrap();

        // Verify remaining data
        assert_eq!(buf.next().unwrap(), 2);
        assert_eq!(buf.next().unwrap(), 3);
    }

    #[test]
    fn test_capacity_and_len() {
        let buf = BlockBuffer::<i32>::new(4);
        assert_eq!(buf.capacity(), 4);
        assert_eq!(buf.len(), 0);
        assert!(buf.is_empty());
        assert!(!buf.is_full());

        buf.write(&[1, 2, 3, 4]).unwrap();
        assert_eq!(buf.len(), 4);
        assert!(!buf.is_empty());
        assert!(buf.is_full());
    }

    #[test]
    fn test_concurrent_producer_consumer() {
        let buf = BlockBuffer::<i32>::new(4);
        let producer_buf = buf.clone();

        let producer = thread::spawn(move || {
            for i in 0..100 {
                producer_buf.add(i).unwrap();
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
    fn test_reset() {
        let buf = BlockBuffer::<i32>::new(4);
        buf.write(&[1, 2, 3]).unwrap();
        assert_eq!(buf.len(), 3);

        buf.reset();
        assert_eq!(buf.len(), 0);
        assert!(buf.is_empty());
    }

    #[test]
    fn test_discard() {
        let buf = BlockBuffer::<i32>::new(8);
        buf.write(&[1, 2, 3, 4, 5]).unwrap();

        buf.discard(2).unwrap();
        assert_eq!(buf.len(), 3);
        assert_eq!(buf.next().unwrap(), 3);
    }

    #[test]
    fn test_to_vec() {
        let buf = BlockBuffer::<i32>::new(4);
        buf.write(&[1, 2, 3]).unwrap();

        let data = buf.to_vec();
        assert_eq!(data, vec![1, 2, 3]);

        // Original buffer unchanged
        assert_eq!(buf.len(), 3);
    }

    #[test]
    fn test_from_vec() {
        let buf = BlockBuffer::from_vec(vec![1, 2, 3]);
        assert_eq!(buf.capacity(), 3);
        assert_eq!(buf.len(), 3);

        assert_eq!(buf.next().unwrap(), 1);
        assert_eq!(buf.next().unwrap(), 2);
        assert_eq!(buf.next().unwrap(), 3);
    }

    #[test]
    fn test_from_vec_write_after_read() {
        let buf = BlockBuffer::from_vec(vec![1, 2, 3]);

        // Read one element to make space
        assert_eq!(buf.next().unwrap(), 1);

        // Write a new element
        buf.add(4).unwrap();

        // Read remaining in FIFO order: 2, 3, 4
        assert_eq!(buf.next().unwrap(), 2);
        assert_eq!(buf.next().unwrap(), 3);
        assert_eq!(buf.next().unwrap(), 4);
    }

    #[test]
    fn test_wrap_around() {
        let buf = BlockBuffer::<i32>::new(3);

        // Fill buffer
        buf.write(&[1, 2, 3]).unwrap();

        // Read some
        assert_eq!(buf.next().unwrap(), 1);
        assert_eq!(buf.next().unwrap(), 2);

        // Write more (should wrap around)
        buf.write(&[4, 5]).unwrap();

        // Read remaining
        assert_eq!(buf.next().unwrap(), 3);
        assert_eq!(buf.next().unwrap(), 4);
        assert_eq!(buf.next().unwrap(), 5);
    }

    #[test]
    fn test_close_with_error() {
        let buf = BlockBuffer::<i32>::new(4);
        buf.add(1).unwrap();

        let err = std::io::Error::new(std::io::ErrorKind::Other, "test error");
        buf.close_with_error(err).unwrap();

        // All operations return error
        assert!(buf.add(2).is_err());
        assert!(buf.read(&mut [0]).is_err());
    }

    #[test]
    fn test_error_method() {
        let buf = BlockBuffer::<i32>::new(4);

        // No error initially
        assert!(buf.error().is_none());

        // After close_with_error, error() returns the error
        let err = std::io::Error::new(std::io::ErrorKind::Other, "test error");
        buf.close_with_error(err).unwrap();
        assert!(buf.error().is_some());
    }
}
