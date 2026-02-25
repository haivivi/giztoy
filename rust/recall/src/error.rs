use thiserror::Error;

#[derive(Error, Debug)]
pub enum RecallError {
    #[error("recall: storage error: {0}")]
    Storage(String),

    #[error("recall: graph error: {0}")]
    Graph(String),

    #[error("recall: vector error: {0}")]
    Vector(String),

    #[error("recall: embed error: {0}")]
    Embed(String),

    #[error("recall: serialization error: {0}")]
    Serialization(String),
}

impl From<giztoy_kv::KVError> for RecallError {
    fn from(e: giztoy_kv::KVError) -> Self {
        RecallError::Storage(e.to_string())
    }
}

impl From<giztoy_graph::GraphError> for RecallError {
    fn from(e: giztoy_graph::GraphError) -> Self {
        RecallError::Graph(e.to_string())
    }
}

impl From<giztoy_vecstore::VecError> for RecallError {
    fn from(e: giztoy_vecstore::VecError) -> Self {
        RecallError::Vector(e.to_string())
    }
}

impl From<giztoy_embed::EmbedError> for RecallError {
    fn from(e: giztoy_embed::EmbedError) -> Self {
        RecallError::Embed(e.to_string())
    }
}
