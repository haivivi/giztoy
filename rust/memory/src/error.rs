use thiserror::Error;

#[derive(Error, Debug)]
pub enum MemoryError {
    #[error("memory: {0}")]
    General(String),

    #[error("memory: compressor not configured")]
    NoCompressor,

    #[error(
        "memory: embed dimension mismatch for model {model}: stored {stored}, current {current}"
    )]
    EmbedDimensionMismatch {
        model: String,
        stored: usize,
        current: usize,
    },

    #[error("memory: embedder dimension mismatch: host={host}, per_persona={per_persona}")]
    EmbedderDimensionMismatch { host: usize, per_persona: usize },

    #[error("memory: recall error: {0}")]
    Recall(#[from] giztoy_recall::RecallError),

    #[error("memory: graph error: {0}")]
    Graph(#[from] giztoy_graph::GraphError),

    #[error("memory: kv error: {0}")]
    KV(#[from] openerp_kv::KVError),

    #[error("memory: serialization error: {0}")]
    Serialization(String),
}
