use thiserror::Error;

/// Errors returned by ONNX Runtime operations.
#[derive(Debug, Error)]
pub enum OnnxError {
    #[error("onnx: {0}")]
    Runtime(String),

    #[error("onnx: model {0:?} not registered")]
    ModelNotRegistered(String),

    #[error("onnx: empty data")]
    EmptyData,
}
