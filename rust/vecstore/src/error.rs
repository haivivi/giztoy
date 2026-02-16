use thiserror::Error;

#[derive(Error, Debug)]
pub enum VecError {
    #[error("vecstore: dimension mismatch: got {got}, want {want}")]
    DimensionMismatch { got: usize, want: usize },

    #[error("vecstore: batch length mismatch: {ids} ids, {vectors} vectors")]
    BatchLengthMismatch { ids: usize, vectors: usize },

    #[error("vecstore: {0}")]
    Io(String),

    #[error("vecstore: invalid format: {0}")]
    InvalidFormat(String),
}
