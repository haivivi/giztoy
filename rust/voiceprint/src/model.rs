use crate::VoiceprintError;

/// Extracts speaker embedding vectors from raw audio.
///
/// The input audio must be PCM16 signed little-endian, 16kHz, mono.
/// The output is a dense f32 vector whose dimensionality is
/// returned by [`VoiceprintModel::dimension`].
///
/// # Audio Requirements
///
/// - Format: PCM16 signed little-endian
/// - Sample rate: 16000 Hz
/// - Channels: 1 (mono)
/// - Minimum duration: ~400ms (6400 samples) for meaningful embeddings
///
/// # Thread Safety
///
/// Implementations must be safe for concurrent use.
pub trait VoiceprintModel: Send + Sync {
    /// Computes a speaker embedding from raw PCM16 audio.
    fn extract(&self, audio: &[u8]) -> Result<Vec<f32>, VoiceprintError>;

    /// Returns the dimensionality of the embedding vectors (e.g., 512).
    fn dimension(&self) -> usize;
}
