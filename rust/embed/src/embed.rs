use crate::error::EmbedError;

/// Embedder converts text into dense float32 vectors.
///
/// Implementations must be safe for concurrent use (Send + Sync).
#[async_trait::async_trait]
pub trait Embedder: Send + Sync {
    /// Return the embedding vector for a single text.
    async fn embed(&self, text: &str) -> Result<Vec<f32>, EmbedError>;

    /// Return embedding vectors for multiple texts.
    /// Implementations may split large batches into smaller API calls.
    async fn embed_batch(&self, texts: &[&str]) -> Result<Vec<Vec<f32>>, EmbedError>;

    /// Return the dimensionality of the output vectors.
    fn dimension(&self) -> usize;
}
