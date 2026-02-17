use thiserror::Error;

/// Errors returned by voiceprint operations.
#[derive(Debug, Error)]
pub enum VoiceprintError {
    #[error("audio too short: need at least {min_bytes} bytes, got {got_bytes}")]
    AudioTooShort { min_bytes: usize, got_bytes: usize },

    #[error("dimension mismatch: expected {expected}, got {got}")]
    DimensionMismatch { expected: usize, got: usize },

    #[error("model error: {0}")]
    Model(String),

    #[error("model is closed")]
    Closed,
}
