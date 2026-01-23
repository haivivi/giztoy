//! Automatic speech recognition.

use crate::{Speech, SpeechError, SpeechStream};
use async_trait::async_trait;
use giztoy_audio::opusrt::FrameReader;
use giztoy_trie::Trie;
use std::sync::Arc;
use tokio::sync::RwLock;

/// Error type for ASR operations.
#[derive(Debug, thiserror::Error)]
pub enum ASRError {
    #[error("transcriber not found: {0}")]
    NotFound(String),
    #[error("transcription failed: {0}")]
    TranscriptionFailed(String),
    #[error("speech error: {0}")]
    Speech(#[from] SpeechError),
    #[error("pattern error: {0}")]
    Pattern(String),
    #[error("other error: {0}")]
    Other(String),
}

/// Interface for streaming speech recognition.
#[async_trait]
pub trait StreamTranscriber: Send + Sync {
    /// Performs streaming transcription on an Opus audio stream.
    async fn transcribe_stream(
        &self,
        model: &str,
        opus: Box<dyn FrameReader + Send>,
    ) -> Result<Box<dyn SpeechStream>, ASRError>;
}

/// Interface for complete speech recognition.
#[async_trait]
pub trait Transcriber: StreamTranscriber {
    /// Transcribes an entire Opus audio stream.
    async fn transcribe(
        &self,
        model: &str,
        opus: Box<dyn FrameReader + Send>,
    ) -> Result<Box<dyn Speech>, ASRError>;
}

/// A multiplexer for ASR transcribers.
pub struct ASR {
    mux: Arc<RwLock<Trie<Arc<dyn StreamTranscriber>>>>,
}

impl Default for ASR {
    fn default() -> Self {
        Self::new()
    }
}

impl ASR {
    /// Creates a new ASR multiplexer.
    pub fn new() -> Self {
        Self {
            mux: Arc::new(RwLock::new(Trie::new())),
        }
    }

    /// Registers a transcriber for the given name pattern.
    pub async fn handle(&self, name: &str, transcriber: Arc<dyn StreamTranscriber>) -> Result<(), ASRError> {
        let mut mux = self.mux.write().await;
        mux.set(name, |_| Ok::<_, giztoy_trie::InvalidPatternError>(transcriber))
            .map_err(|e| ASRError::Pattern(e.to_string()))
    }

    /// Performs streaming transcription on an Opus audio stream.
    pub async fn transcribe_stream(
        &self,
        name: &str,
        opus: Box<dyn FrameReader + Send>,
    ) -> Result<Box<dyn SpeechStream>, ASRError> {
        let mux = self.mux.read().await;
        let transcriber = mux.get(name).ok_or_else(|| ASRError::NotFound(name.to_string()))?;
        transcriber.transcribe_stream(name, opus).await
    }

    /// Transcribes an entire Opus audio stream.
    /// If the transcriber implements the Transcriber interface, it will be used directly.
    /// Otherwise, it will fall back to the streaming interface and collect the results.
    pub async fn transcribe(
        &self,
        name: &str,
        opus: Box<dyn FrameReader + Send>,
    ) -> Result<Box<dyn Speech>, ASRError> {
        let mux = self.mux.read().await;
        let transcriber = mux.get(name).ok_or_else(|| ASRError::NotFound(name.to_string()))?;

        // Try to use the Transcriber interface if available
        // For now, we always use streaming and collect
        let stream = transcriber.transcribe_stream(name, opus).await?;
        Ok(Box::new(crate::util::SpeechCollector::new(stream)))
    }
}

#[async_trait]
impl StreamTranscriber for ASR {
    async fn transcribe_stream(
        &self,
        model: &str,
        opus: Box<dyn FrameReader + Send>,
    ) -> Result<Box<dyn SpeechStream>, ASRError> {
        ASR::transcribe_stream(self, model, opus).await
    }
}

#[cfg(test)]
mod asr_tests {
    use super::*;
    use giztoy_audio::opusrt::Frame;
    use std::time::Duration;

    #[test]
    fn test_asr_error_display() {
        let err = ASRError::NotFound("model/en".to_string());
        assert!(err.to_string().contains("model/en"));

        let err = ASRError::TranscriptionFailed("timeout".to_string());
        assert!(err.to_string().contains("timeout"));
    }

    #[tokio::test]
    async fn test_asr_not_found() {
        let asr = ASR::new();

        // Create a mock frame reader
        struct MockFrameReader;

        impl FrameReader for MockFrameReader {
            fn next_frame(&mut self) -> Result<(Option<Frame>, Duration), std::io::Error> {
                Ok((None, Duration::ZERO))
            }
        }

        let result = asr.transcribe_stream("nonexistent", Box::new(MockFrameReader)).await;
        assert!(matches!(result, Err(ASRError::NotFound(_))));
    }
}
