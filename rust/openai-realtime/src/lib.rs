//! OpenAI Realtime API client for Rust.
//!
//! This crate provides a client for the OpenAI Realtime API, supporting both
//! WebSocket connections. It enables real-time audio and text conversations
//! with OpenAI's GPT-4o models.
//!
//! # Features
//!
//! - WebSocket-based realtime sessions
//! - Audio input/output support (PCM16 at 24kHz)
//! - Voice activity detection (VAD) modes
//! - Function calling support
//! - Streaming text and audio responses
//!
//! # Example
//!
//! ```rust,no_run
//! use giztoy_openai_realtime::{Client, SessionConfig, Session};
//!
//! #[tokio::main]
//! async fn main() -> Result<(), Box<dyn std::error::Error>> {
//!     // Create client
//!     let client = Client::new("your-api-key")?;
//!
//!     // Connect via WebSocket
//!     let mut session = client.connect_websocket(None).await?;
//!
//!     // Wait for session.created
//!     while let Some(result) = session.recv().await {
//!         let event = result?;
//!         if event.event_type == "session.created" {
//!             break;
//!         }
//!     }
//!
//!     // Configure session (disable VAD for manual mode)
//!     let config = SessionConfig {
//!         turn_detection_disabled: true,
//!         modalities: vec!["text".to_string(), "audio".to_string()],
//!         voice: Some("alloy".to_string()),
//!         ..Default::default()
//!     };
//!     session.update_session(&config).await?;
//!
//!     // Add a user message
//!     session.add_user_message("Hello!").await?;
//!
//!     // Request a response
//!     session.create_response(None).await?;
//!
//!     // Process events
//!     while let Some(result) = session.recv().await {
//!         let event = result?;
//!         match event.event_type.as_str() {
//!             "response.text.delta" => {
//!                 if let Some(delta) = event.delta {
//!                     print!("{}", delta);
//!                 }
//!             }
//!             "response.done" => break,
//!             _ => {}
//!         }
//!     }
//!
//!     session.close().await?;
//!     Ok(())
//! }
//! ```

pub mod client;
pub mod error;
pub mod event;
pub mod session;
pub mod types;
pub mod websocket;

// Re-export main types
pub use client::{Client, ClientBuilder};
pub use error::{ApiError, Error, Result};
pub use event::{EventError, ServerEvent};
pub use session::Session;
pub use types::*;
pub use websocket::WebSocketSession;

// Re-export event type constants
pub use event::{
    EVENT_TYPE_CONVERSATION_CREATED, EVENT_TYPE_CONVERSATION_ITEM_CREATE,
    EVENT_TYPE_CONVERSATION_ITEM_CREATED, EVENT_TYPE_CONVERSATION_ITEM_DELETE,
    EVENT_TYPE_CONVERSATION_ITEM_DELETED,
    EVENT_TYPE_CONVERSATION_ITEM_INPUT_AUDIO_TRANSCRIPTION_COMPLETED,
    EVENT_TYPE_CONVERSATION_ITEM_INPUT_AUDIO_TRANSCRIPTION_FAILED,
    EVENT_TYPE_CONVERSATION_ITEM_TRUNCATE, EVENT_TYPE_CONVERSATION_ITEM_TRUNCATED,
    EVENT_TYPE_ERROR, EVENT_TYPE_INPUT_AUDIO_BUFFER_APPEND, EVENT_TYPE_INPUT_AUDIO_BUFFER_CLEAR,
    EVENT_TYPE_INPUT_AUDIO_BUFFER_CLEARED, EVENT_TYPE_INPUT_AUDIO_BUFFER_COMMIT,
    EVENT_TYPE_INPUT_AUDIO_BUFFER_COMMITTED, EVENT_TYPE_INPUT_AUDIO_BUFFER_SPEECH_STARTED,
    EVENT_TYPE_INPUT_AUDIO_BUFFER_SPEECH_STOPPED, EVENT_TYPE_RATE_LIMITS_UPDATED,
    EVENT_TYPE_RESPONSE_AUDIO_DELTA, EVENT_TYPE_RESPONSE_AUDIO_DONE,
    EVENT_TYPE_RESPONSE_AUDIO_TRANSCRIPT_DELTA, EVENT_TYPE_RESPONSE_AUDIO_TRANSCRIPT_DONE,
    EVENT_TYPE_RESPONSE_CANCEL, EVENT_TYPE_RESPONSE_CONTENT_PART_ADDED,
    EVENT_TYPE_RESPONSE_CONTENT_PART_DONE, EVENT_TYPE_RESPONSE_CREATE, EVENT_TYPE_RESPONSE_CREATED,
    EVENT_TYPE_RESPONSE_DONE, EVENT_TYPE_RESPONSE_FUNCTION_CALL_ARGUMENTS_DELTA,
    EVENT_TYPE_RESPONSE_FUNCTION_CALL_ARGUMENTS_DONE, EVENT_TYPE_RESPONSE_OUTPUT_ITEM_ADDED,
    EVENT_TYPE_RESPONSE_OUTPUT_ITEM_DONE, EVENT_TYPE_RESPONSE_TEXT_DELTA,
    EVENT_TYPE_RESPONSE_TEXT_DONE, EVENT_TYPE_SESSION_CREATED, EVENT_TYPE_SESSION_UPDATE,
    EVENT_TYPE_SESSION_UPDATED,
};
