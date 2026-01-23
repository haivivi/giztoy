//! Error types for OpenAI Realtime API.

use thiserror::Error;

/// Result type for OpenAI Realtime operations.
pub type Result<T> = std::result::Result<T, Error>;

/// Errors that can occur when using the OpenAI Realtime API.
#[derive(Error, Debug)]
pub enum Error {
    /// Connection error.
    #[error("connection error: {0}")]
    Connection(String),

    /// WebSocket error.
    #[error("websocket error: {0}")]
    WebSocket(#[from] tokio_tungstenite::tungstenite::Error),

    /// HTTP error.
    #[error("http error: {0}")]
    Http(#[from] reqwest::Error),

    /// JSON serialization/deserialization error.
    #[error("json error: {0}")]
    Json(#[from] serde_json::Error),

    /// API error returned by OpenAI.
    #[error("api error: {}: {}", .0.code.as_deref().or(.0.error_type.as_deref()).unwrap_or("unknown"), .0.message)]
    Api(ApiError),

    /// Session is closed.
    #[error("session closed")]
    SessionClosed,

    /// Invalid configuration.
    #[error("invalid configuration: {0}")]
    InvalidConfig(String),

    /// Timeout error.
    #[error("timeout: {0}")]
    Timeout(String),
}

/// API error from OpenAI Realtime.
#[derive(Debug, Clone, Default)]
pub struct ApiError {
    /// Error type (e.g., "invalid_request_error").
    pub error_type: Option<String>,
    /// Error code (e.g., "invalid_value").
    pub code: Option<String>,
    /// Human-readable error message.
    pub message: String,
    /// Parameter that caused the error.
    pub param: Option<String>,
    /// Event ID that caused the error.
    pub event_id: Option<String>,
    /// HTTP status code, if applicable.
    pub http_status: Option<u16>,
}

impl ApiError {
    /// Creates a new API error from an event error.
    pub fn from_event(
        error_type: Option<String>,
        code: Option<String>,
        message: String,
        param: Option<String>,
        event_id: Option<String>,
    ) -> Self {
        Self {
            error_type,
            code,
            message,
            param,
            event_id,
            http_status: None,
        }
    }
}

impl std::fmt::Display for ApiError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        if let Some(ref code) = self.code {
            write!(f, "{}: {}", code, self.message)
        } else if let Some(ref error_type) = self.error_type {
            write!(f, "{}: {}", error_type, self.message)
        } else {
            write!(f, "{}", self.message)
        }
    }
}
