//! Type definitions for OpenAI Realtime API.

use serde::{Deserialize, Serialize};
use std::collections::HashMap;

// ============================================================================
// Models
// ============================================================================

/// GPT-4o realtime preview model.
pub const MODEL_GPT4O_REALTIME_PREVIEW: &str = "gpt-4o-realtime-preview";
/// GPT-4o realtime preview model (2024-12-17 version).
pub const MODEL_GPT4O_REALTIME_PREVIEW_20241217: &str = "gpt-4o-realtime-preview-2024-12-17";
/// GPT-4o mini realtime preview model.
pub const MODEL_GPT4O_MINI_REALTIME_PREVIEW: &str = "gpt-4o-mini-realtime-preview";
/// GPT-4o mini realtime preview model (2024-12-17 version).
pub const MODEL_GPT4O_MINI_REALTIME_PREVIEW_20241217: &str = "gpt-4o-mini-realtime-preview-2024-12-17";

// ============================================================================
// Audio Formats
// ============================================================================

/// 16-bit PCM audio at 24kHz, mono, little-endian.
pub const AUDIO_FORMAT_PCM16: &str = "pcm16";
/// G.711 μ-law audio at 8kHz.
pub const AUDIO_FORMAT_G711_ULAW: &str = "g711_ulaw";
/// G.711 A-law audio at 8kHz.
pub const AUDIO_FORMAT_G711_ALAW: &str = "g711_alaw";

// ============================================================================
// Voices
// ============================================================================

pub const VOICE_ALLOY: &str = "alloy";
pub const VOICE_ASH: &str = "ash";
pub const VOICE_BALLAD: &str = "ballad";
pub const VOICE_CORAL: &str = "coral";
pub const VOICE_ECHO: &str = "echo";
pub const VOICE_SAGE: &str = "sage";
pub const VOICE_SHIMMER: &str = "shimmer";
pub const VOICE_VERSE: &str = "verse";

// ============================================================================
// VAD Modes
// ============================================================================

/// Server-side voice activity detection.
pub const VAD_SERVER_VAD: &str = "server_vad";
/// Semantic voice activity detection.
pub const VAD_SEMANTIC_VAD: &str = "semantic_vad";

// ============================================================================
// Modalities
// ============================================================================

pub const MODALITY_TEXT: &str = "text";
pub const MODALITY_AUDIO: &str = "audio";

// ============================================================================
// Tool Choice
// ============================================================================

pub const TOOL_CHOICE_AUTO: &str = "auto";
pub const TOOL_CHOICE_NONE: &str = "none";
pub const TOOL_CHOICE_REQUIRED: &str = "required";

// ============================================================================
// Configuration Types
// ============================================================================

/// Configuration for establishing a realtime connection.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ConnectConfig {
    /// Model ID to use. Default: gpt-4o-realtime-preview
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub model: String,
}

/// Configuration for updating session parameters.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SessionConfig {
    /// Output modalities. Default: ["text", "audio"]
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub modalities: Vec<String>,

    /// System prompt.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub instructions: Option<String>,

    /// Voice ID for audio output.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub voice: Option<String>,

    /// Input audio format. Default: pcm16
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub input_audio_format: Option<String>,

    /// Output audio format. Default: pcm16
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub output_audio_format: Option<String>,

    /// Input audio transcription config.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub input_audio_transcription: Option<TranscriptionConfig>,

    /// Voice activity detection config.
    /// Set to None to keep current setting.
    /// Use `turn_detection_disabled` to explicitly disable VAD.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub turn_detection: Option<TurnDetection>,

    /// When true, sends "turn_detection": null explicitly.
    #[serde(skip)]
    pub turn_detection_disabled: bool,

    /// Available function tools.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub tools: Vec<Tool>,

    /// Tool choice: "auto", "none", "required", or specific function.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tool_choice: Option<serde_json::Value>,

    /// Temperature (0.6-1.2). Default: 0.8
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f64>,

    /// Max response output tokens. Use "inf" for unlimited.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_response_output_tokens: Option<serde_json::Value>,
}

impl SessionConfig {
    /// Creates a new session config with VAD disabled (manual mode).
    pub fn with_vad_disabled() -> Self {
        Self {
            turn_detection_disabled: true,
            ..Default::default()
        }
    }

    /// Converts to JSON value, handling turn_detection_disabled.
    pub fn to_json_value(&self) -> serde_json::Value {
        let mut map = serde_json::Map::new();

        if !self.modalities.is_empty() {
            map.insert("modalities".to_string(), serde_json::json!(self.modalities));
        }
        if let Some(ref instructions) = self.instructions {
            map.insert("instructions".to_string(), serde_json::json!(instructions));
        }
        if let Some(ref voice) = self.voice {
            map.insert("voice".to_string(), serde_json::json!(voice));
        }
        if let Some(ref format) = self.input_audio_format {
            map.insert("input_audio_format".to_string(), serde_json::json!(format));
        }
        if let Some(ref format) = self.output_audio_format {
            map.insert("output_audio_format".to_string(), serde_json::json!(format));
        }
        if let Some(ref transcription) = self.input_audio_transcription {
            map.insert("input_audio_transcription".to_string(), serde_json::json!(transcription));
        }

        // Handle turn_detection
        if self.turn_detection_disabled {
            map.insert("turn_detection".to_string(), serde_json::Value::Null);
        } else if let Some(ref td) = self.turn_detection {
            map.insert("turn_detection".to_string(), serde_json::json!(td));
        }

        if !self.tools.is_empty() {
            map.insert("tools".to_string(), serde_json::json!(self.tools));
        }
        if let Some(ref tool_choice) = self.tool_choice {
            map.insert("tool_choice".to_string(), tool_choice.clone());
        }
        if let Some(temperature) = self.temperature {
            map.insert("temperature".to_string(), serde_json::json!(temperature));
        }
        if let Some(ref max_tokens) = self.max_response_output_tokens {
            map.insert("max_response_output_tokens".to_string(), max_tokens.clone());
        }

        serde_json::Value::Object(map)
    }
}

/// Transcription configuration.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TranscriptionConfig {
    /// Transcription model. Default: whisper-1
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
}

/// Voice activity detection configuration.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TurnDetection {
    /// VAD mode: "server_vad" or "semantic_vad".
    #[serde(rename = "type", default, skip_serializing_if = "Option::is_none")]
    pub detection_type: Option<String>,

    /// VAD sensitivity (0.0-1.0). Default: 0.5
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub threshold: Option<f64>,

    /// Padding before speech start (ms). Default: 300
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub prefix_padding_ms: Option<i32>,

    /// Silence duration to detect end of speech (ms). Default: 500
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub silence_duration_ms: Option<i32>,

    /// Auto-create response when VAD detects end of speech. Default: true
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub create_response: Option<bool>,

    /// Interrupt current response on new speech. Default: true
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub interrupt_response: Option<bool>,

    /// Response eagerness for semantic_vad: "low", "medium", "high". Default: "medium"
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub eagerness: Option<String>,
}

/// Function tool definition.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Tool {
    /// Tool type. Always "function".
    #[serde(rename = "type")]
    pub tool_type: String,

    /// Function name.
    pub name: String,

    /// Function description.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,

    /// JSON Schema for function parameters.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub parameters: Option<HashMap<String, serde_json::Value>>,
}

impl Tool {
    /// Creates a new function tool.
    pub fn function(name: impl Into<String>) -> Self {
        Self {
            tool_type: "function".to_string(),
            name: name.into(),
            description: None,
            parameters: None,
        }
    }

    /// Sets the description.
    pub fn with_description(mut self, description: impl Into<String>) -> Self {
        self.description = Some(description.into());
        self
    }

    /// Sets the parameters schema.
    pub fn with_parameters(mut self, parameters: HashMap<String, serde_json::Value>) -> Self {
        self.parameters = Some(parameters);
        self
    }
}

// ============================================================================
// Response Create Options
// ============================================================================

/// Options for creating a response.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ResponseCreateOptions {
    /// Output modalities for this response.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub modalities: Vec<String>,

    /// Instructions override for this response.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub instructions: Option<String>,

    /// Voice override for this response.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub voice: Option<String>,

    /// Output audio format override.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub output_audio_format: Option<String>,

    /// Tools override for this response.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub tools: Vec<Tool>,

    /// Tool choice override.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tool_choice: Option<serde_json::Value>,

    /// Temperature override.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f64>,

    /// Max output tokens for this response.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_output_tokens: Option<serde_json::Value>,

    /// Conversation handling: "auto" (default) or "none".
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub conversation: Option<String>,

    /// Input items instead of using the buffer.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub input: Vec<ConversationItem>,
}

// ============================================================================
// Resource Types
// ============================================================================

/// Session state returned by the server.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SessionResource {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub object: String,
    #[serde(default)]
    pub model: String,
    #[serde(default)]
    pub expires_at: i64,
    #[serde(default)]
    pub modalities: Vec<String>,
    #[serde(default)]
    pub instructions: String,
    #[serde(default)]
    pub voice: String,
    #[serde(default)]
    pub input_audio_format: String,
    #[serde(default)]
    pub output_audio_format: String,
    #[serde(default)]
    pub input_audio_transcription: Option<TranscriptionConfig>,
    #[serde(default)]
    pub turn_detection: Option<TurnDetection>,
    #[serde(default)]
    pub tools: Vec<Tool>,
    #[serde(default)]
    pub tool_choice: Option<serde_json::Value>,
    #[serde(default)]
    pub temperature: f64,
    #[serde(default)]
    pub max_response_output_tokens: Option<serde_json::Value>,
}

/// Conversation resource.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ConversationResource {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub object: String,
}

/// Conversation item.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ConversationItem {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub object: Option<String>,
    /// "message", "function_call", "function_call_output"
    #[serde(rename = "type", default, skip_serializing_if = "Option::is_none")]
    pub item_type: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub status: Option<String>,
    /// "user", "assistant", "system"
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub role: Option<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub content: Vec<ContentPart>,
    /// For function_call_output
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub call_id: Option<String>,
    /// For function_call
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    /// For function_call
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub arguments: Option<String>,
    /// For function_call_output
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub output: Option<String>,
}

impl ConversationItem {
    /// Creates a user text message item.
    pub fn user_text(text: impl Into<String>) -> Self {
        Self {
            item_type: Some("message".to_string()),
            role: Some("user".to_string()),
            content: vec![ContentPart {
                content_type: Some("input_text".to_string()),
                text: Some(text.into()),
                ..Default::default()
            }],
            ..Default::default()
        }
    }

    /// Creates an assistant text message item.
    pub fn assistant_text(text: impl Into<String>) -> Self {
        Self {
            item_type: Some("message".to_string()),
            role: Some("assistant".to_string()),
            content: vec![ContentPart {
                content_type: Some("text".to_string()),
                text: Some(text.into()),
                ..Default::default()
            }],
            ..Default::default()
        }
    }

    /// Creates a user audio message item.
    pub fn user_audio(audio_base64: impl Into<String>, transcript: Option<String>) -> Self {
        Self {
            item_type: Some("message".to_string()),
            role: Some("user".to_string()),
            content: vec![ContentPart {
                content_type: Some("input_audio".to_string()),
                audio: Some(audio_base64.into()),
                transcript,
                ..Default::default()
            }],
            ..Default::default()
        }
    }

    /// Creates a function call output item.
    pub fn function_call_output(call_id: impl Into<String>, output: impl Into<String>) -> Self {
        Self {
            item_type: Some("function_call_output".to_string()),
            call_id: Some(call_id.into()),
            output: Some(output.into()),
            ..Default::default()
        }
    }
}

/// Content part of a message.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ContentPart {
    /// "input_text", "input_audio", "item_reference", "text", "audio"
    #[serde(rename = "type", default, skip_serializing_if = "Option::is_none")]
    pub content_type: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub text: Option<String>,
    /// Base64 encoded audio
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub audio: Option<String>,
    /// Transcript for audio parts
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub transcript: Option<String>,
    /// For item_reference
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
}

/// Response resource.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ResponseResource {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub object: String,
    /// "in_progress", "completed", "cancelled", "incomplete", "failed"
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub status_details: Option<StatusDetails>,
    #[serde(default)]
    pub output: Vec<ConversationItem>,
    #[serde(default)]
    pub usage: Option<Usage>,
}

/// Status details.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct StatusDetails {
    #[serde(rename = "type", default)]
    pub details_type: String,
    #[serde(default)]
    pub reason: String,
    #[serde(default)]
    pub error: Option<ErrorInfo>,
}

/// Error information.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ErrorInfo {
    #[serde(rename = "type", default)]
    pub error_type: String,
    #[serde(default)]
    pub code: String,
    #[serde(default)]
    pub message: String,
    #[serde(default)]
    pub param: Option<String>,
}

/// Token usage information.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Usage {
    #[serde(default)]
    pub total_tokens: i32,
    #[serde(default)]
    pub input_tokens: i32,
    #[serde(default)]
    pub output_tokens: i32,
    #[serde(default)]
    pub input_token_details: Option<TokenDetails>,
    #[serde(default)]
    pub output_token_details: Option<TokenDetails>,
}

/// Token details.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TokenDetails {
    #[serde(default)]
    pub cached_tokens: i32,
    #[serde(default)]
    pub text_tokens: i32,
    #[serde(default)]
    pub audio_tokens: i32,
    #[serde(default)]
    pub cached_tokens_details: Option<CachedTokensDetails>,
}

/// Cached token details.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct CachedTokensDetails {
    #[serde(default)]
    pub text_tokens: i32,
    #[serde(default)]
    pub audio_tokens: i32,
}

/// Rate limit information.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct RateLimit {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub limit: i32,
    #[serde(default)]
    pub remaining: i32,
    #[serde(default)]
    pub reset_seconds: f64,
}
