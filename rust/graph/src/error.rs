use thiserror::Error;

#[derive(Error, Debug)]
pub enum GraphError {
    #[error("graph: not found")]
    NotFound,

    #[error("graph: label contains separator: {0:?} contains ':'")]
    InvalidLabel(String),

    #[error("graph: storage error: {0}")]
    Storage(String),

    #[error("graph: serialization error: {0}")]
    Serialization(String),
}
