//! Common types for the Doubao Speech API.

use serde::{Deserialize, Serialize};

// ================== Audio Encoding ==================

/// Audio encoding format (TTS output).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "snake_case")]
pub enum AudioEncoding {
    /// PCM format
    #[default]
    Pcm,
    /// WAV format
    Wav,
    /// MP3 format
    Mp3,
    /// OGG Opus format
    #[serde(rename = "ogg_opus")]
    OggOpus,
    /// AAC format
    Aac,
    /// M4A format
    M4a,
    /// PCM S16LE format (for realtime)
    #[serde(rename = "pcm_s16le")]
    PcmS16le,
}

impl AudioEncoding {
    /// Returns the encoding as a string.
    pub fn as_str(&self) -> &'static str {
        match self {
            AudioEncoding::Pcm => "pcm",
            AudioEncoding::Wav => "wav",
            AudioEncoding::Mp3 => "mp3",
            AudioEncoding::OggOpus => "ogg_opus",
            AudioEncoding::Aac => "aac",
            AudioEncoding::M4a => "m4a",
            AudioEncoding::PcmS16le => "pcm_s16le",
        }
    }
}

// ================== Audio Format ==================

/// Audio format (ASR input).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "snake_case")]
pub enum AudioFormat {
    /// WAV format
    #[default]
    Wav,
    /// MP3 format
    Mp3,
    /// OGG format
    Ogg,
    /// M4A format
    M4a,
    /// AAC format
    Aac,
    /// PCM format
    Pcm,
    /// Raw format
    Raw,
}

impl AudioFormat {
    /// Returns the format as a string.
    pub fn as_str(&self) -> &'static str {
        match self {
            AudioFormat::Wav => "wav",
            AudioFormat::Mp3 => "mp3",
            AudioFormat::Ogg => "ogg",
            AudioFormat::M4a => "m4a",
            AudioFormat::Aac => "aac",
            AudioFormat::Pcm => "pcm",
            AudioFormat::Raw => "raw",
        }
    }
}

// ================== Sample Rate ==================

/// Audio sample rate.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[repr(i32)]
pub enum SampleRate {
    /// 8000 Hz
    #[serde(rename = "8000")]
    Rate8000 = 8000,
    /// 16000 Hz
    #[default]
    #[serde(rename = "16000")]
    Rate16000 = 16000,
    /// 22050 Hz
    #[serde(rename = "22050")]
    Rate22050 = 22050,
    /// 24000 Hz
    #[serde(rename = "24000")]
    Rate24000 = 24000,
    /// 32000 Hz
    #[serde(rename = "32000")]
    Rate32000 = 32000,
    /// 44100 Hz
    #[serde(rename = "44100")]
    Rate44100 = 44100,
    /// 48000 Hz
    #[serde(rename = "48000")]
    Rate48000 = 48000,
}

impl SampleRate {
    /// Returns the sample rate as an integer.
    pub fn as_i32(&self) -> i32 {
        *self as i32
    }
}

// ================== Language ==================

/// Language code.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub enum Language {
    /// Chinese (Mandarin)
    #[default]
    #[serde(rename = "zh-CN")]
    ZhCn,
    /// English (US)
    #[serde(rename = "en-US")]
    EnUs,
    /// English (UK)
    #[serde(rename = "en-GB")]
    EnGb,
    /// Japanese
    #[serde(rename = "ja-JP")]
    JaJp,
    /// Korean
    #[serde(rename = "ko-KR")]
    KoKr,
    /// Spanish
    #[serde(rename = "es-ES")]
    EsEs,
    /// French
    #[serde(rename = "fr-FR")]
    FrFr,
    /// German
    #[serde(rename = "de-DE")]
    DeDe,
    /// Italian
    #[serde(rename = "it-IT")]
    ItIt,
    /// Portuguese (Brazil)
    #[serde(rename = "pt-BR")]
    PtBr,
    /// Russian
    #[serde(rename = "ru-RU")]
    RuRu,
    /// Arabic
    #[serde(rename = "ar-SA")]
    ArSa,
    /// Thai
    #[serde(rename = "th-TH")]
    ThTh,
    /// Vietnamese
    #[serde(rename = "vi-VN")]
    ViVn,
    /// Indonesian
    #[serde(rename = "id-ID")]
    IdId,
    /// Malay
    #[serde(rename = "ms-MS")]
    MsMs,
}

impl Language {
    /// Returns the language code as a string.
    pub fn as_str(&self) -> &'static str {
        match self {
            Language::ZhCn => "zh-CN",
            Language::EnUs => "en-US",
            Language::EnGb => "en-GB",
            Language::JaJp => "ja-JP",
            Language::KoKr => "ko-KR",
            Language::EsEs => "es-ES",
            Language::FrFr => "fr-FR",
            Language::DeDe => "de-DE",
            Language::ItIt => "it-IT",
            Language::PtBr => "pt-BR",
            Language::RuRu => "ru-RU",
            Language::ArSa => "ar-SA",
            Language::ThTh => "th-TH",
            Language::ViVn => "vi-VN",
            Language::IdId => "id-ID",
            Language::MsMs => "ms-MS",
        }
    }
}

// ================== Task Status ==================

/// Task status.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "snake_case")]
pub enum TaskStatus {
    /// Task is pending
    #[default]
    Pending,
    /// Task is processing
    Processing,
    /// Task succeeded
    Success,
    /// Task failed
    Failed,
    /// Task was cancelled
    Cancelled,
}

impl TaskStatus {
    /// Returns true if the task is finished (success, failed, or cancelled).
    pub fn is_finished(&self) -> bool {
        matches!(
            self,
            TaskStatus::Success | TaskStatus::Failed | TaskStatus::Cancelled
        )
    }

    /// Returns true if the task succeeded.
    pub fn is_success(&self) -> bool {
        matches!(self, TaskStatus::Success)
    }
}

// ================== Common Structures ==================

/// Audio information.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct AudioInfo {
    /// Duration in milliseconds
    pub duration: i32,
    /// Sample rate
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sample_rate: Option<i32>,
    /// Number of channels
    #[serde(skip_serializing_if = "Option::is_none")]
    pub channels: Option<i32>,
    /// Bit depth
    #[serde(skip_serializing_if = "Option::is_none")]
    pub bits: Option<i32>,
}

/// Subtitle segment.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SubtitleSegment {
    /// Subtitle text
    pub text: String,
    /// Start time in milliseconds
    pub start_time: i32,
    /// End time in milliseconds
    pub end_time: i32,
}

/// Location information (for realtime conversation).
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct LocationInfo {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub longitude: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub latitude: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub city: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub country: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub province: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub district: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub town: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub country_code: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub address: Option<String>,
}

// ================== TTS Text Type ==================

/// TTS text type.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "snake_case")]
pub enum TtsTextType {
    /// Plain text
    #[default]
    Plain,
    /// SSML format
    Ssml,
}

impl TtsTextType {
    /// Returns the text type as a string.
    pub fn as_str(&self) -> &'static str {
        match self {
            TtsTextType::Plain => "plain",
            TtsTextType::Ssml => "ssml",
        }
    }
}

// ================== Voice Clone Types ==================

/// Voice clone model type.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "snake_case")]
pub enum VoiceCloneModelType {
    /// Standard model
    #[default]
    Standard,
    /// Pro model
    Pro,
}

/// Voice clone status type.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "snake_case")]
pub enum VoiceCloneStatusType {
    /// Pending
    #[default]
    Pending,
    /// Processing
    Processing,
    /// Success
    Success,
    /// Failed
    Failed,
}

// ================== Subtitle Format ==================

/// Subtitle format.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "snake_case")]
pub enum SubtitleFormat {
    /// SRT format
    #[default]
    Srt,
    /// VTT format
    Vtt,
    /// JSON format
    Json,
}
