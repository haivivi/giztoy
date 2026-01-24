//! Event types for OpenAI Realtime API.

use serde::{Deserialize, Serialize};

use crate::types::*;

// ============================================================================
// Client Event Types (sent from client to server)
// ============================================================================

/// Session update event.
pub const EVENT_TYPE_SESSION_UPDATE: &str = "session.update";

/// Input audio buffer events.
pub const EVENT_TYPE_INPUT_AUDIO_BUFFER_APPEND: &str = "input_audio_buffer.append";
pub const EVENT_TYPE_INPUT_AUDIO_BUFFER_COMMIT: &str = "input_audio_buffer.commit";
pub const EVENT_TYPE_INPUT_AUDIO_BUFFER_CLEAR: &str = "input_audio_buffer.clear";

/// Conversation item events.
pub const EVENT_TYPE_CONVERSATION_ITEM_CREATE: &str = "conversation.item.create";
pub const EVENT_TYPE_CONVERSATION_ITEM_TRUNCATE: &str = "conversation.item.truncate";
pub const EVENT_TYPE_CONVERSATION_ITEM_DELETE: &str = "conversation.item.delete";

/// Response events.
pub const EVENT_TYPE_RESPONSE_CREATE: &str = "response.create";
pub const EVENT_TYPE_RESPONSE_CANCEL: &str = "response.cancel";

// ============================================================================
// Server Event Types (sent from server to client)
// ============================================================================

/// Error event.
pub const EVENT_TYPE_ERROR: &str = "error";

/// Session events.
pub const EVENT_TYPE_SESSION_CREATED: &str = "session.created";
pub const EVENT_TYPE_SESSION_UPDATED: &str = "session.updated";

/// Conversation events.
pub const EVENT_TYPE_CONVERSATION_CREATED: &str = "conversation.created";
pub const EVENT_TYPE_CONVERSATION_ITEM_CREATED: &str = "conversation.item.created";
pub const EVENT_TYPE_CONVERSATION_ITEM_INPUT_AUDIO_TRANSCRIPTION_COMPLETED: &str =
    "conversation.item.input_audio_transcription.completed";
pub const EVENT_TYPE_CONVERSATION_ITEM_INPUT_AUDIO_TRANSCRIPTION_FAILED: &str =
    "conversation.item.input_audio_transcription.failed";
pub const EVENT_TYPE_CONVERSATION_ITEM_TRUNCATED: &str = "conversation.item.truncated";
pub const EVENT_TYPE_CONVERSATION_ITEM_DELETED: &str = "conversation.item.deleted";

/// Input audio buffer events.
pub const EVENT_TYPE_INPUT_AUDIO_BUFFER_COMMITTED: &str = "input_audio_buffer.committed";
pub const EVENT_TYPE_INPUT_AUDIO_BUFFER_CLEARED: &str = "input_audio_buffer.cleared";
pub const EVENT_TYPE_INPUT_AUDIO_BUFFER_SPEECH_STARTED: &str = "input_audio_buffer.speech_started";
pub const EVENT_TYPE_INPUT_AUDIO_BUFFER_SPEECH_STOPPED: &str = "input_audio_buffer.speech_stopped";

/// Response events.
pub const EVENT_TYPE_RESPONSE_CREATED: &str = "response.created";
pub const EVENT_TYPE_RESPONSE_DONE: &str = "response.done";
pub const EVENT_TYPE_RESPONSE_OUTPUT_ITEM_ADDED: &str = "response.output_item.added";
pub const EVENT_TYPE_RESPONSE_OUTPUT_ITEM_DONE: &str = "response.output_item.done";
pub const EVENT_TYPE_RESPONSE_CONTENT_PART_ADDED: &str = "response.content_part.added";
pub const EVENT_TYPE_RESPONSE_CONTENT_PART_DONE: &str = "response.content_part.done";

/// Response text events.
pub const EVENT_TYPE_RESPONSE_TEXT_DELTA: &str = "response.text.delta";
pub const EVENT_TYPE_RESPONSE_TEXT_DONE: &str = "response.text.done";

/// Response audio events.
pub const EVENT_TYPE_RESPONSE_AUDIO_DELTA: &str = "response.audio.delta";
pub const EVENT_TYPE_RESPONSE_AUDIO_DONE: &str = "response.audio.done";

/// Response audio transcript events.
pub const EVENT_TYPE_RESPONSE_AUDIO_TRANSCRIPT_DELTA: &str = "response.audio_transcript.delta";
pub const EVENT_TYPE_RESPONSE_AUDIO_TRANSCRIPT_DONE: &str = "response.audio_transcript.done";

/// Response function call events.
pub const EVENT_TYPE_RESPONSE_FUNCTION_CALL_ARGUMENTS_DELTA: &str =
    "response.function_call_arguments.delta";
pub const EVENT_TYPE_RESPONSE_FUNCTION_CALL_ARGUMENTS_DONE: &str =
    "response.function_call_arguments.done";

/// Rate limits event.
pub const EVENT_TYPE_RATE_LIMITS_UPDATED: &str = "rate_limits.updated";

// ============================================================================
// Server Event
// ============================================================================

/// Server event received from the Realtime API.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ServerEvent {
    /// Event type.
    #[serde(rename = "type", default)]
    pub event_type: String,

    /// Unique event identifier.
    #[serde(default)]
    pub event_id: Option<String>,

    // === Session events ===
    /// Session information (for session.created, session.updated).
    #[serde(default)]
    pub session: Option<SessionResource>,

    // === Conversation events ===
    /// Conversation information (for conversation.created).
    #[serde(default)]
    pub conversation: Option<ConversationResource>,

    /// Conversation item (for conversation.item.* events).
    #[serde(default)]
    pub item: Option<ConversationItem>,

    // === Input audio buffer events ===
    /// Previous item ID (for input_audio_buffer.committed).
    #[serde(default)]
    pub previous_item_id: Option<String>,

    /// Item ID (for various events).
    #[serde(default)]
    pub item_id: Option<String>,

    /// Audio start time in milliseconds (for speech_started).
    #[serde(default)]
    pub audio_start_ms: Option<i32>,

    /// Audio end time in milliseconds (for speech_stopped, truncated).
    #[serde(default)]
    pub audio_end_ms: Option<i32>,

    // === Transcription events ===
    /// Transcription text.
    #[serde(default)]
    pub transcript: Option<String>,

    /// Content part index.
    #[serde(default)]
    pub content_index: Option<i32>,

    // === Response events ===
    /// Response information (for response.* events).
    #[serde(default)]
    pub response: Option<ResponseResource>,

    /// Response identifier.
    #[serde(default)]
    pub response_id: Option<String>,

    /// Output item index.
    #[serde(default)]
    pub output_index: Option<i32>,

    /// Content part information.
    #[serde(default)]
    pub part: Option<ContentPart>,

    // === Delta events ===
    /// Incremental text/arguments (for *.delta events).
    #[serde(default)]
    pub delta: Option<String>,

    // === Function call events ===
    /// Function call ID.
    #[serde(default)]
    pub call_id: Option<String>,

    /// Function name.
    #[serde(default)]
    pub name: Option<String>,

    /// Function arguments (complete, for done event).
    #[serde(default)]
    pub arguments: Option<String>,

    // === Rate limits event ===
    /// Rate limit information.
    #[serde(default)]
    pub rate_limits: Option<Vec<RateLimit>>,

    // === Error event ===
    /// Error information.
    #[serde(default)]
    pub error: Option<EventError>,

    // === Parsed data (not from JSON) ===
    /// Decoded audio data (populated after parsing audio delta).
    #[serde(skip)]
    pub audio: Option<Vec<u8>>,

    /// Base64 audio data (from delta field for audio events).
    #[serde(skip)]
    pub audio_base64: Option<String>,

    /// Raw JSON message.
    #[serde(skip)]
    pub raw: Option<Vec<u8>>,
}

impl ServerEvent {
    /// Returns true if this is an error event.
    pub fn is_error(&self) -> bool {
        self.event_type == EVENT_TYPE_ERROR
    }

    /// Returns true if this is a response done event.
    pub fn is_response_done(&self) -> bool {
        self.event_type == EVENT_TYPE_RESPONSE_DONE
    }

    /// Returns true if this is an audio delta event.
    pub fn is_audio_delta(&self) -> bool {
        self.event_type == EVENT_TYPE_RESPONSE_AUDIO_DELTA
    }

    /// Returns true if this is a text delta event.
    pub fn is_text_delta(&self) -> bool {
        self.event_type == EVENT_TYPE_RESPONSE_TEXT_DELTA
    }

    /// Returns true if this is a transcript delta event.
    pub fn is_transcript_delta(&self) -> bool {
        self.event_type == EVENT_TYPE_RESPONSE_AUDIO_TRANSCRIPT_DELTA
    }
}

/// Error information from error events.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct EventError {
    #[serde(rename = "type", default)]
    pub error_type: Option<String>,
    #[serde(default)]
    pub code: Option<String>,
    #[serde(default)]
    pub message: String,
    #[serde(default)]
    pub param: Option<String>,
    #[serde(default)]
    pub event_id: Option<String>,
}

impl EventError {
    /// Converts to an API error.
    pub fn to_api_error(&self) -> crate::error::ApiError {
        crate::error::ApiError::from_event(
            self.error_type.clone(),
            self.code.clone(),
            self.message.clone(),
            self.param.clone(),
            self.event_id.clone(),
        )
    }
}
