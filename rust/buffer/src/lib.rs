//! Thread-safe streaming buffer implementations.
//!
//! This crate provides thread-safe buffer implementations for streaming data
//! between producers and consumers. It offers three main buffer types:
//!
//! - [`Buffer<T>`]: A growable buffer that automatically expands
//! - [`BlockBuffer<T>`]: A fixed-size buffer that blocks when full
//! - [`RingBuffer<T>`]: A fixed-size buffer that overwrites oldest data when full
//!
//! # Buffer Types
//!
//! ## Buffer (Growable)
//!
//! [`Buffer<T>`] is ideal when you don't know the data size in advance or when
//! memory usage is not a concern. It never blocks on write operations.
//!
//! ```
//! use giztoy_buffer::Buffer;
//!
//! let buf = Buffer::<i32>::new();
//! buf.write(&[1, 2, 3]).unwrap();
//!
//! let mut data = vec![0; 3];
//! let n = buf.read(&mut data).unwrap();
//! assert_eq!(data, vec![1, 2, 3]);
//! ```
//!
//! ## BlockBuffer (Fixed, Blocking)
//!
//! [`BlockBuffer<T>`] provides backpressure by blocking writes when the buffer
//! is full. This is useful for flow control between producers and consumers.
//!
//! ```
//! use giztoy_buffer::BlockBuffer;
//!
//! let buf = BlockBuffer::<i32>::new(4);
//! buf.write(&[1, 2, 3, 4]).unwrap();  // Buffer is now full
//! // Next write would block until space is available
//! ```
//!
//! ## RingBuffer (Fixed, Overwriting)
//!
//! [`RingBuffer<T>`] maintains a sliding window of the most recent data by
//! overwriting the oldest elements when full. Perfect for keeping recent
//! samples or maintaining a bounded history.
//!
//! ```
//! use giztoy_buffer::RingBuffer;
//!
//! let buf = RingBuffer::<i32>::new(3);
//! buf.write(&[1, 2, 3, 4, 5]).unwrap();  // Overwrites 1, 2
//! assert_eq!(buf.to_vec(), vec![3, 4, 5]);
//! ```
//!
//! # Closing Buffers
//!
//! All buffer types support two modes of closing:
//!
//! - `close_write()`: Prevents new writes but allows reading existing data
//! - `close_with_error()`: Immediately closes and returns the error to all operations
//!
//! # Thread Safety
//!
//! All buffer types are `Send + Sync` and can be safely shared between threads
//! using `Clone` (which shares the underlying buffer via `Arc`).
//!
//! # Convenience Functions
//!
//! The [`bytes`] module provides convenience functions for creating pre-sized
//! byte buffers:
//!
//! ```
//! use giztoy_buffer::{bytes_4kb, block_bytes_4kb, ring_bytes_4kb};
//!
//! let growable = bytes_4kb();
//! let blocking = block_bytes_4kb();
//! let ring = ring_bytes_4kb();
//! ```

mod block_buffer;
mod buffer;
mod bytes;
mod error;
mod ring_buffer;

pub use block_buffer::BlockBuffer;
pub use buffer::Buffer;
pub use bytes::*;
pub use error::{BufferError, Done};
pub use ring_buffer::RingBuffer;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_buffer_is_send_sync() {
        fn assert_send_sync<T: Send + Sync>() {}
        assert_send_sync::<Buffer<i32>>();
        assert_send_sync::<BlockBuffer<i32>>();
        assert_send_sync::<RingBuffer<i32>>();
    }

    #[test]
    fn test_buffer_is_clone() {
        fn assert_clone<T: Clone>() {}
        assert_clone::<Buffer<i32>>();
        assert_clone::<BlockBuffer<i32>>();
        assert_clone::<RingBuffer<i32>>();
    }
}
