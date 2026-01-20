//! Event types for realtime communication.

use serde::{Deserialize, Serialize};

use crate::types::{SessionInfo, UsageStats};

/// Client events
pub const EVENT_TYPE_SESSION_UPDATE: &str = "session.update";
pub const EVENT_TYPE_INPUT_AUDIO_APPEND: &str = "input_audio_buffer.append";
pub const EVENT_TYPE_INPUT_AUDIO_COMMIT: &str = "input_audio_buffer.commit";
pub const EVENT_TYPE_INPUT_AUDIO_CLEAR: &str = "input_audio_buffer.clear";
pub const EVENT_TYPE_INPUT_IMAGE_APPEND: &str = "input_image_buffer.append";
pub const EVENT_TYPE_RESPONSE_CREATE: &str = "response.create";
pub const EVENT_TYPE_RESPONSE_CANCEL: &str = "response.cancel";
pub const EVENT_TYPE_SESSION_FINISH: &str = "session.finish";

/// Server events
pub const EVENT_TYPE_SESSION_CREATED: &str = "session.created";
pub const EVENT_TYPE_SESSION_UPDATED: &str = "session.updated";
pub const EVENT_TYPE_INPUT_AUDIO_COMMITTED: &str = "input_audio_buffer.committed";
pub const EVENT_TYPE_INPUT_AUDIO_CLEARED: &str = "input_audio_buffer.cleared";
pub const EVENT_TYPE_INPUT_SPEECH_STARTED: &str = "input_audio_buffer.speech_started";
pub const EVENT_TYPE_INPUT_SPEECH_STOPPED: &str = "input_audio_buffer.speech_stopped";
pub const EVENT_TYPE_RESPONSE_CREATED: &str = "response.created";
pub const EVENT_TYPE_RESPONSE_DONE: &str = "response.done";
pub const EVENT_TYPE_RESPONSE_OUTPUT_ADDED: &str = "response.output_item.added";
pub const EVENT_TYPE_RESPONSE_OUTPUT_DONE: &str = "response.output_item.done";
pub const EVENT_TYPE_RESPONSE_CONTENT_ADDED: &str = "response.content_part.added";
pub const EVENT_TYPE_RESPONSE_CONTENT_DONE: &str = "response.content_part.done";
pub const EVENT_TYPE_RESPONSE_TEXT_DELTA: &str = "response.text.delta";
pub const EVENT_TYPE_RESPONSE_TEXT_DONE: &str = "response.text.done";
pub const EVENT_TYPE_RESPONSE_AUDIO_DELTA: &str = "response.audio.delta";
pub const EVENT_TYPE_RESPONSE_AUDIO_DONE: &str = "response.audio.done";
pub const EVENT_TYPE_RESPONSE_TRANSCRIPT_DELTA: &str = "response.audio_transcript.delta";
pub const EVENT_TYPE_RESPONSE_TRANSCRIPT_DONE: &str = "response.audio_transcript.done";
pub const EVENT_TYPE_INPUT_AUDIO_TRANSCRIPTION_COMPLETED: &str = "conversation.item.input_audio_transcription.completed";
pub const EVENT_TYPE_ERROR: &str = "error";

/// DashScope-specific: "choices" format response
pub const EVENT_TYPE_CHOICES_RESPONSE: &str = "choices";

/// Realtime event from the server.
#[derive(Debug, Clone, Default)]
pub struct RealtimeEvent {
    /// Event type.
    pub event_type: String,

    /// Unique event identifier.
    pub event_id: Option<String>,

    /// Session information (for session.* events).
    pub session: Option<SessionInfo>,

    /// Response information (for response.* events).
    pub response: Option<ResponseInfo>,

    /// Response identifier (for response.created).
    pub response_id: Option<String>,

    /// Incremental text content (for *.delta events).
    pub delta: Option<String>,

    /// Decoded audio data (for response.audio.delta and choices responses).
    pub audio: Option<Vec<u8>>,

    /// Raw base64 audio from JSON.
    pub audio_base64: Option<String>,

    /// Transcript text (for transcript completion events).
    pub transcript: Option<String>,

    /// Finish reason (for choices responses).
    pub finish_reason: Option<String>,

    /// Item identifier (for item events).
    pub item_id: Option<String>,

    /// Output index (for content events).
    pub output_index: Option<i32>,

    /// Content index (for content events).
    pub content_index: Option<i32>,

    /// Error information (for error events).
    pub error: Option<EventError>,

    /// Usage statistics (for response.done).
    pub usage: Option<UsageStats>,
}

/// Response information.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ResponseInfo {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status_detail: Option<StatusDetail>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output: Option<Vec<OutputItem>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub usage: Option<UsageStats>,
}

/// Status detail information.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct StatusDetail {
    #[serde(rename = "type", skip_serializing_if = "Option::is_none")]
    pub detail_type: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<EventError>,
}

/// Output item in a response.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct OutputItem {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
    #[serde(rename = "type", skip_serializing_if = "Option::is_none")]
    pub item_type: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub role: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub content: Option<Vec<ContentPart>>,
}

/// Content part.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ContentPart {
    #[serde(rename = "type", skip_serializing_if = "Option::is_none")]
    pub content_type: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub text: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub transcript: Option<String>,
}

/// Error information from error events.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct EventError {
    #[serde(rename = "type", skip_serializing_if = "Option::is_none")]
    pub error_type: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub code: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub param: Option<String>,
}

impl RealtimeEvent {
    /// Returns true if this is a session.created event.
    pub fn is_session_created(&self) -> bool {
        self.event_type == EVENT_TYPE_SESSION_CREATED
    }

    /// Returns true if this is a session.updated event.
    pub fn is_session_updated(&self) -> bool {
        self.event_type == EVENT_TYPE_SESSION_UPDATED
    }

    /// Returns true if this is a response.done event.
    pub fn is_response_done(&self) -> bool {
        self.event_type == EVENT_TYPE_RESPONSE_DONE
    }

    /// Returns true if this is an error event.
    pub fn is_error(&self) -> bool {
        self.event_type == EVENT_TYPE_ERROR
    }

    /// Returns true if this is a choices response (DashScope-specific).
    pub fn is_choices_response(&self) -> bool {
        self.event_type == EVENT_TYPE_CHOICES_RESPONSE
    }

    /// Returns true if this event contains audio data.
    pub fn has_audio(&self) -> bool {
        self.audio.is_some() || self.audio_base64.is_some()
    }

    /// Returns true if this event contains text delta.
    pub fn has_text_delta(&self) -> bool {
        self.delta.is_some()
    }
}
