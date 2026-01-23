//! Session trait for OpenAI Realtime API.

use async_trait::async_trait;

use crate::error::Result;
use crate::event::ServerEvent;
use crate::types::*;

/// Common interface for OpenAI Realtime sessions.
/// Both WebSocket and WebRTC implementations satisfy this trait.
#[async_trait]
pub trait Session: Send + Sync {
    // === Session Management ===

    /// Updates the session configuration.
    /// This should be called after receiving session.created event.
    async fn update_session(&self, config: &SessionConfig) -> Result<()>;

    /// Closes the session connection.
    async fn close(&self) -> Result<()>;

    /// Returns the session ID assigned by the server.
    /// Returns None if session.created has not been received yet.
    fn session_id(&self) -> Option<String>;

    // === Audio Input ===

    /// Appends PCM audio data to the input audio buffer.
    /// Audio should be 24kHz, 16-bit, mono PCM (little-endian).
    async fn append_audio(&self, audio: &[u8]) -> Result<()>;

    /// Appends base64-encoded audio data to the input buffer.
    async fn append_audio_base64(&self, audio_base64: &str) -> Result<()>;

    /// Commits the audio buffer and creates a user message.
    /// In server_vad mode, this is called automatically after VAD detects end of speech.
    /// In manual mode (turn_detection: null), call this to indicate end of user input.
    async fn commit_input(&self) -> Result<()>;

    /// Clears the input audio buffer without creating a message.
    async fn clear_input(&self) -> Result<()>;

    // === Conversation Management ===

    /// Adds a user text message to the conversation.
    async fn add_user_message(&self, text: &str) -> Result<()>;

    /// Adds a user audio message to the conversation.
    /// Audio should be base64 encoded. Transcript is optional.
    async fn add_user_audio(&self, audio_base64: &str, transcript: Option<&str>) -> Result<()>;

    /// Adds an assistant text message to the conversation.
    async fn add_assistant_message(&self, text: &str) -> Result<()>;

    /// Adds a function call output to the conversation.
    async fn add_function_call_output(&self, call_id: &str, output: &str) -> Result<()>;

    /// Truncates a conversation item (assistant audio).
    /// content_index is the index of the content part to truncate.
    /// audio_end_ms is the audio end time in milliseconds.
    async fn truncate_item(&self, item_id: &str, content_index: i32, audio_end_ms: i32)
        -> Result<()>;

    /// Deletes a conversation item.
    async fn delete_item(&self, item_id: &str) -> Result<()>;

    // === Response Control ===

    /// Requests the model to generate a response.
    /// In server_vad mode, this is called automatically by the server.
    /// In manual mode, call this after commit_input to trigger response generation.
    /// Pass None for default options.
    async fn create_response(&self, opts: Option<&ResponseCreateOptions>) -> Result<()>;

    /// Cancels the current response generation.
    async fn cancel_response(&self) -> Result<()>;

    // === Event Reception ===

    /// Receives the next event from the server.
    /// Returns None when the session is closed.
    async fn recv(&mut self) -> Option<Result<ServerEvent>>;

    // === Raw Operations ===

    /// Sends a raw JSON event to the server.
    /// Use this for events not covered by helper methods.
    async fn send_raw(&self, event: serde_json::Value) -> Result<()>;
}
