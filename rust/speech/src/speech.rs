//! Speech (audio + transcript) types.

use crate::{VoiceError, VoiceSegment};
use async_trait::async_trait;
use giztoy_audio::pcm::Format;
use tokio::io::AsyncRead;

/// Error type for speech operations.
#[derive(Debug, thiserror::Error)]
pub enum SpeechError {
    #[error("end of stream")]
    Done,
    #[error("io error: {0}")]
    Io(#[from] std::io::Error),
    #[error("voice error: {0}")]
    Voice(#[from] VoiceError),
    #[error("transcription error: {0}")]
    Transcription(String),
    #[error("other error: {0}")]
    Other(String),
}

/// A stream of speech segments.
/// Unlike Voice, Speech includes text transcription alongside audio.
#[async_trait]
pub trait Speech: Send + Sync {
    /// Returns the next speech segment.
    /// Returns `SpeechError::Done` when no more segments are available.
    async fn next(&mut self) -> Result<Box<dyn SpeechSegment>, SpeechError>;

    /// Closes the speech stream and releases resources.
    async fn close(&mut self) -> Result<(), SpeechError>;
}

/// A single speech segment containing audio data and transcript.
#[async_trait]
pub trait SpeechSegment: Send + Sync {
    /// Decodes the speech segment into a voice segment.
    /// The `best` parameter suggests the preferred PCM format, but the
    /// implementation may return a different format if more suitable.
    fn decode(&self, best: Format) -> Box<dyn VoiceSegment>;

    /// Returns a reader for the transcript text.
    fn transcribe(&self) -> Box<dyn AsyncRead + Send + Unpin>;

    /// Closes the segment and releases resources.
    async fn close(&mut self) -> Result<(), SpeechError>;
}

/// A stream of Speech objects.
#[async_trait]
pub trait SpeechStream: Send + Sync {
    /// Returns the next Speech.
    /// Returns `SpeechError::Done` when no more speeches are available.
    async fn next(&mut self) -> Result<Box<dyn Speech>, SpeechError>;

    /// Closes the stream and releases resources.
    async fn close(&mut self) -> Result<(), SpeechError>;
}

#[cfg(test)]
mod speech_tests {
    use super::*;

    #[test]
    fn test_speech_error_display() {
        let err = SpeechError::Done;
        assert_eq!(err.to_string(), "end of stream");

        let err = SpeechError::Transcription("failed".to_string());
        assert!(err.to_string().contains("failed"));
    }
}
