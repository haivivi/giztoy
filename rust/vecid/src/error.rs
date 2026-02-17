use thiserror::Error;

/// Errors returned by vecid operations.
#[derive(Debug, Error)]
pub enum VecIdError {
    #[error("store error: {0}")]
    Store(String),

    #[error("dimension mismatch: expected {expected}, got {got}")]
    DimensionMismatch { expected: usize, got: usize },
}
