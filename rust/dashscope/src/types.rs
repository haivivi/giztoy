//! Type definitions for DashScope API.

use serde::{Deserialize, Serialize};

/// Common models for Qwen-Omni-Realtime.
pub const MODEL_QWEN_OMNI_TURBO_REALTIME: &str = "qwen-omni-turbo-realtime";
pub const MODEL_QWEN_OMNI_TURBO_REALTIME_LATEST: &str = "qwen-omni-turbo-realtime-latest";
pub const MODEL_QWEN3_OMNI_FLASH_REALTIME: &str = "qwen3-omni-flash-realtime";

/// Audio formats supported by DashScope.
pub const AUDIO_FORMAT_PCM16: &str = "pcm16";
pub const AUDIO_FORMAT_PCM24: &str = "pcm24";
pub const AUDIO_FORMAT_WAV: &str = "wav";
pub const AUDIO_FORMAT_MP3: &str = "mp3";

/// Voice IDs for TTS output.
pub const VOICE_CHELSIE: &str = "Chelsie";
pub const VOICE_CHERRY: &str = "Cherry";
pub const VOICE_SERENA: &str = "Serena";
pub const VOICE_ETHAN: &str = "Ethan";

/// VAD modes for voice activity detection.
pub const VAD_MODE_SERVER_VAD: &str = "server_vad";
pub const VAD_MODE_DISABLED: &str = "disabled";

/// Modalities for output.
pub const MODALITY_TEXT: &str = "text";
pub const MODALITY_AUDIO: &str = "audio";

/// Session configuration for updating session parameters.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SessionConfig {
    /// TurnDetection configures voice activity detection.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub turn_detection: Option<TurnDetection>,

    /// Input audio format (default: pcm16).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_audio_format: Option<String>,

    /// Output audio format (default: pcm16).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_audio_format: Option<String>,

    /// Voice ID for TTS output.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub voice: Option<String>,

    /// Output modalities (default: ["text", "audio"]).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub modalities: Option<Vec<String>>,

    /// System instructions.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub instructions: Option<String>,

    /// Temperature for generation (0.0-2.0).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f64>,

    /// Maximum output tokens (-1 for unlimited).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_output_tokens: Option<i32>,

    /// Enable transcription of input audio.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub enable_input_audio_transcription: Option<bool>,

    /// Model for input transcription.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_audio_transcription_model: Option<String>,
}

/// Turn detection (VAD) configuration.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TurnDetection {
    /// VAD mode: "server_vad" or "disabled".
    #[serde(rename = "type", skip_serializing_if = "Option::is_none")]
    pub detection_type: Option<String>,

    /// Padding before speech start (ms). Default: 300.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub prefix_padding_ms: Option<i32>,

    /// Silence duration to detect end of speech (ms). Range: [200, 6000]. Default: 800.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub silence_duration_ms: Option<i32>,

    /// VAD sensitivity (0.0-1.0). Default: 0.5.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub threshold: Option<f64>,
}

/// Session information from server.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SessionInfo {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub modalities: Option<Vec<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub voice: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_audio_format: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_audio_format: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub turn_detection: Option<TurnDetection>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub instructions: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_output_tokens: Option<serde_json::Value>,
}

/// Usage statistics.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct UsageStats {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub total_tokens: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_tokens: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_tokens: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_token_details: Option<TokenDetails>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_token_details: Option<TokenDetails>,
}

/// Token usage details.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TokenDetails {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub text_tokens: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio_tokens: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub image_tokens: Option<i32>,
}
