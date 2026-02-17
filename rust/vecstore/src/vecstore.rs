use crate::error::VecError;

/// Match is a single result from a vector similarity search.
#[derive(Debug, Clone)]
pub struct Match {
    /// Identifier of the matched vector.
    pub id: String,

    /// Distance between the query and matched vector.
    /// Lower values indicate higher similarity.
    pub distance: f32,
}

/// VecIndex is the interface for approximate nearest-neighbor search over
/// dense float32 vectors.
///
/// All implementations must be safe for concurrent use (Send + Sync).
pub trait VecIndex: Send + Sync {
    /// Add or update a vector with the given ID.
    fn insert(&self, id: &str, vector: &[f32]) -> Result<(), VecError>;

    /// Add or update multiple vectors at once.
    /// `ids` and `vectors` must have the same length.
    fn batch_insert(&self, ids: &[&str], vectors: &[&[f32]]) -> Result<(), VecError>;

    /// Return the top-k nearest vectors to the query, ordered by ascending
    /// distance (closest first).
    fn search(&self, query: &[f32], top_k: usize) -> Result<Vec<Match>, VecError>;

    /// Remove a vector by ID. No error if ID does not exist.
    fn delete(&self, id: &str) -> Result<(), VecError>;

    /// Return the number of vectors in the index.
    fn len(&self) -> usize;

    /// Return true if the index contains no vectors.
    fn is_empty(&self) -> bool {
        self.len() == 0
    }

    /// Ensure all pending writes are visible to subsequent searches.
    fn flush(&self) -> Result<(), VecError>;
}
