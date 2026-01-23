//! Voice (pure audio) types.

use async_trait::async_trait;
use giztoy_audio::pcm::Format;
use std::io;

/// Error type for voice operations.
#[derive(Debug, thiserror::Error)]
pub enum VoiceError {
    #[error("end of stream")]
    Done,
    #[error("io error: {0}")]
    Io(#[from] io::Error),
    #[error("decode error: {0}")]
    Decode(String),
    #[error("other error: {0}")]
    Other(String),
}

/// A stream of audio segments.
#[async_trait]
pub trait Voice: Send + Sync {
    /// Returns the next voice segment.
    /// Returns `VoiceError::Done` when no more segments are available.
    async fn next(&mut self) -> Result<Box<dyn VoiceSegment>, VoiceError>;

    /// Closes the voice stream and releases resources.
    async fn close(&mut self) -> Result<(), VoiceError>;
}

/// A single audio segment, which can be read as PCM data.
#[async_trait]
pub trait VoiceSegment: Send + Sync {
    /// Reads PCM data into the buffer.
    /// Returns the number of bytes read.
    async fn read(&mut self, buf: &mut [u8]) -> Result<usize, VoiceError>;

    /// Returns the PCM format of this segment.
    fn format(&self) -> Format;

    /// Closes the segment and releases resources.
    async fn close(&mut self) -> Result<(), VoiceError>;
}

/// A stream of Voice objects.
#[async_trait]
pub trait VoiceStream: Send + Sync {
    /// Returns the next Voice.
    /// Returns `VoiceError::Done` when no more voices are available.
    async fn next(&mut self) -> Result<Box<dyn Voice>, VoiceError>;

    /// Closes the stream and releases resources.
    async fn close(&mut self) -> Result<(), VoiceError>;
}

#[cfg(test)]
mod voice_tests {
    use super::*;

    #[test]
    fn test_voice_error_display() {
        let err = VoiceError::Done;
        assert_eq!(err.to_string(), "end of stream");

        let err = VoiceError::Decode("invalid format".to_string());
        assert!(err.to_string().contains("invalid format"));
    }
}
