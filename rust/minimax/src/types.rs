//! Common types for the MiniMax API.

use serde::{Deserialize, Serialize};

// ==================== Output Format ====================

/// Output format for audio.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum OutputFormat {
    /// Hex-encoded audio data.
    #[default]
    Hex,
    /// URL to download the audio.
    Url,
}

// ==================== Audio Format ====================

/// Audio encoding format.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum AudioFormat {
    /// MP3 format.
    #[default]
    Mp3,
    /// PCM format.
    Pcm,
    /// FLAC format.
    Flac,
    /// WAV format.
    Wav,
}

// ==================== Voice Type ====================

/// Voice type for filtering.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum VoiceType {
    /// All voices.
    #[default]
    All,
    /// System predefined voices.
    System,
    /// Voices created via voice cloning.
    VoiceCloning,
    /// Voices created via voice design/generation.
    VoiceGeneration,
}

// ==================== File Purpose ====================

/// Intended use of an uploaded file.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum FilePurpose {
    /// Voice cloning source audio.
    VoiceClone,
    /// Voice cloning example/prompt audio.
    PromptAudio,
    /// Async TTS input files.
    T2aAsyncInput,
}

// ==================== Task Status ====================

/// Status of an async task.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TaskStatus {
    Pending,
    Queueing,
    Preparing,
    Processing,
    Success,
    Failed,
}

impl TaskStatus {
    /// Returns true if the task is still in progress.
    pub fn is_pending(&self) -> bool {
        matches!(
            self,
            TaskStatus::Pending | TaskStatus::Queueing | TaskStatus::Preparing | TaskStatus::Processing
        )
    }

    /// Returns true if the task completed successfully.
    pub fn is_success(&self) -> bool {
        matches!(self, TaskStatus::Success)
    }

    /// Returns true if the task failed.
    pub fn is_failed(&self) -> bool {
        matches!(self, TaskStatus::Failed)
    }
}

// ==================== Audio Info ====================

/// Metadata about generated audio.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct AudioInfo {
    /// Duration in milliseconds.
    #[serde(default)]
    pub audio_length: i32,

    /// Sample rate.
    #[serde(default)]
    pub audio_sample_rate: i32,

    /// Size in bytes.
    #[serde(default)]
    pub audio_size: i32,

    /// Bitrate.
    #[serde(default)]
    pub bitrate: i32,

    /// Number of words/characters.
    #[serde(default)]
    pub word_count: i32,

    /// Billable character count.
    #[serde(default)]
    pub usage_characters: i32,

    /// Audio format.
    #[serde(default)]
    pub audio_format: String,

    /// Number of channels.
    #[serde(default)]
    pub audio_channel: i32,
}

// ==================== Subtitle ====================

/// Subtitle information.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Subtitle {
    pub segments: Vec<SubtitleSegment>,
}

/// A single subtitle segment.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SubtitleSegment {
    /// Start time in milliseconds.
    pub start_time: i32,
    /// End time in milliseconds.
    pub end_time: i32,
    /// Subtitle text.
    pub text: String,
}

// ==================== Base Response ====================

/// Common response wrapper from MiniMax API.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub(crate) struct BaseResp {
    #[serde(default)]
    pub status_code: i32,
    #[serde(default)]
    pub status_msg: String,
}

impl BaseResp {
    /// Returns true if the response indicates an error.
    pub fn is_error(&self) -> bool {
        self.status_code != 0
    }
}

// ==================== Flexible ID ====================

/// A flexible ID that can be either a string or number.
/// MiniMax API sometimes returns file_id as int64, sometimes as string.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct FlexibleId(pub String);

impl FlexibleId {
    pub fn new(id: impl Into<String>) -> Self {
        Self(id.into())
    }

    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl From<String> for FlexibleId {
    fn from(s: String) -> Self {
        Self(s)
    }
}

impl From<&str> for FlexibleId {
    fn from(s: &str) -> Self {
        Self(s.to_string())
    }
}

impl From<i64> for FlexibleId {
    fn from(n: i64) -> Self {
        Self(n.to_string())
    }
}

impl std::fmt::Display for FlexibleId {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl Serialize for FlexibleId {
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        serializer.serialize_str(&self.0)
    }
}

impl<'de> Deserialize<'de> for FlexibleId {
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        struct FlexibleIdVisitor;

        impl serde::de::Visitor<'_> for FlexibleIdVisitor {
            type Value = FlexibleId;

            fn expecting(&self, formatter: &mut std::fmt::Formatter) -> std::fmt::Result {
                formatter.write_str("a string or integer")
            }

            fn visit_str<E>(self, v: &str) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                Ok(FlexibleId(v.to_string()))
            }

            fn visit_string<E>(self, v: String) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                Ok(FlexibleId(v))
            }

            fn visit_i64<E>(self, v: i64) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                Ok(FlexibleId(v.to_string()))
            }

            fn visit_u64<E>(self, v: u64) -> std::result::Result<Self::Value, E>
            where
                E: serde::de::Error,
            {
                Ok(FlexibleId(v.to_string()))
            }
        }

        deserializer.deserialize_any(FlexibleIdVisitor)
    }
}
