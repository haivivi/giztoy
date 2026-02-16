use thiserror::Error;

#[derive(Error, Debug)]
pub enum EmbedError {
    #[error("embed: empty input")]
    EmptyInput,

    #[error("embed: API error: {0}")]
    Api(String),

    #[error("embed: missing embedding for index {0}")]
    MissingIndex(usize),

    #[error("embed: unexpected embedding index {index} for batch size {batch_size}")]
    UnexpectedIndex { index: usize, batch_size: usize },
}
