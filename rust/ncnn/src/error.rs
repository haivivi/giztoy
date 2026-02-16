use thiserror::Error;

/// Errors returned by ncnn operations.
#[derive(Debug, Error)]
pub enum NcnnError {
    #[error("ncnn: {0}")]
    Internal(String),

    #[error("ncnn: model {0:?} not registered")]
    ModelNotRegistered(String),

    #[error("ncnn: empty data")]
    EmptyData,
}
