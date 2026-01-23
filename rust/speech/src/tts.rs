//! Text-to-speech synthesis.

use crate::{Speech, SpeechError};
use async_trait::async_trait;
use giztoy_audio::pcm::Format;
use giztoy_trie::Trie;
use std::sync::Arc;
use tokio::io::AsyncRead;
use tokio::sync::RwLock;
use tracing::warn;

/// Error type for TTS operations.
#[derive(Debug, thiserror::Error)]
pub enum TTSError {
    #[error("synthesizer not found: {0}")]
    NotFound(String),
    #[error("synthesis failed: {0}")]
    SynthesisFailed(String),
    #[error("speech error: {0}")]
    Speech(#[from] SpeechError),
    #[error("pattern error: {0}")]
    Pattern(String),
    #[error("other error: {0}")]
    Other(String),
}

/// Interface for a text-to-speech synthesizer.
#[async_trait]
pub trait Synthesizer: Send + Sync {
    /// Synthesizes the text from the reader into speech.
    async fn synthesize(
        &self,
        name: &str,
        text_stream: Box<dyn AsyncRead + Send + Unpin>,
        format: Format,
    ) -> Result<Box<dyn Speech>, TTSError>;
}

/// Function type that implements the Synthesizer interface.
pub type SynthesizeFunc = Box<
    dyn Fn(
            String,
            Box<dyn AsyncRead + Send + Unpin>,
            Format,
        ) -> std::pin::Pin<Box<dyn std::future::Future<Output = Result<Box<dyn Speech>, TTSError>> + Send>>
        + Send
        + Sync,
>;

/// A multiplexer for TTS synthesizers.
pub struct TTS {
    mux: Arc<RwLock<Trie<Arc<dyn Synthesizer>>>>,
}

impl Default for TTS {
    fn default() -> Self {
        Self::new()
    }
}

impl TTS {
    /// Creates a new TTS multiplexer.
    pub fn new() -> Self {
        Self {
            mux: Arc::new(RwLock::new(Trie::new())),
        }
    }

    /// Registers a synthesizer for the given name pattern.
    pub async fn handle(&self, name: &str, synthesizer: Arc<dyn Synthesizer>) -> Result<(), TTSError> {
        let mut mux = self.mux.write().await;
        mux.set(name, |existing| {
            if existing.is_some() {
                warn!(name = %name, "tts: synthesizer already registered");
            }
            Ok::<_, giztoy_trie::InvalidPatternError>(synthesizer)
        })
        .map_err(|e| TTSError::Pattern(e.to_string()))
    }

    /// Synthesizes speech for the given name.
    pub async fn synthesize(
        &self,
        name: &str,
        text_stream: Box<dyn AsyncRead + Send + Unpin>,
        format: Format,
    ) -> Result<Box<dyn Speech>, TTSError> {
        let mux = self.mux.read().await;
        let syn = mux.get(name).ok_or_else(|| TTSError::NotFound(name.to_string()))?;
        syn.synthesize(name, text_stream, format).await
    }
}

#[async_trait]
impl Synthesizer for TTS {
    async fn synthesize(
        &self,
        name: &str,
        text_stream: Box<dyn AsyncRead + Send + Unpin>,
        format: Format,
    ) -> Result<Box<dyn Speech>, TTSError> {
        TTS::synthesize(self, name, text_stream, format).await
    }
}

#[cfg(test)]
mod tts_tests {
    use super::*;

    #[test]
    fn test_tts_error_display() {
        let err = TTSError::NotFound("voice/en".to_string());
        assert!(err.to_string().contains("voice/en"));

        let err = TTSError::SynthesisFailed("timeout".to_string());
        assert!(err.to_string().contains("timeout"));
    }

    #[tokio::test]
    async fn test_tts_not_found() {
        let tts = TTS::new();
        let result = tts
            .synthesize(
                "nonexistent",
                Box::new(tokio::io::empty()),
                Format::L16Mono16K,
            )
            .await;
        assert!(matches!(result, Err(TTSError::NotFound(_))));
    }
}
