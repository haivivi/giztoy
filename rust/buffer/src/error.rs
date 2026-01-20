//! Error types for buffer operations.

use std::error::Error;
use std::fmt;
use std::sync::Arc;

/// Buffer operation error.
///
/// This enum represents errors that can occur during buffer operations.
/// It supports both simple closure (without an associated error) and
/// closure with an attached error for more detailed error reporting.
#[derive(Debug, Clone)]
pub enum BufferError {
    /// Buffer has been closed (write side).
    Closed,
    /// Buffer has been closed with an associated error.
    ClosedWithError(Arc<dyn Error + Send + Sync>),
}

impl fmt::Display for BufferError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            BufferError::Closed => write!(f, "buffer: closed"),
            BufferError::ClosedWithError(e) => write!(f, "buffer: closed with error: {}", e),
        }
    }
}

impl Error for BufferError {
    fn source(&self) -> Option<&(dyn Error + 'static)> {
        match self {
            BufferError::ClosedWithError(e) => Some(e.as_ref()),
            _ => None,
        }
    }
}

/// Iterator completion marker.
///
/// This error type is returned by the `next()` method when the buffer
/// has been closed for writing and all data has been consumed.
/// It signals the end of iteration, similar to `io::EOF` but for the
/// iterator pattern.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Done;

impl fmt::Display for Done {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "iterator done")
    }
}

impl Error for Done {}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io;

    #[test]
    fn test_buffer_error_display() {
        let err = BufferError::Closed;
        assert_eq!(format!("{}", err), "buffer: closed");

        let io_err: Arc<dyn Error + Send + Sync> =
            Arc::new(io::Error::new(io::ErrorKind::Other, "test error"));
        let err = BufferError::ClosedWithError(io_err);
        assert!(format!("{}", err).contains("test error"));
    }

    #[test]
    fn test_done_display() {
        let done = Done;
        assert_eq!(format!("{}", done), "iterator done");
    }

    #[test]
    fn test_done_equality() {
        assert_eq!(Done, Done);
    }
}
