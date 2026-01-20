//! Error types for the DashScope API client.

use thiserror::Error;

/// Common error codes from DashScope.
pub mod error_code {
    /// Authentication errors
    pub const INVALID_API_KEY: &str = "InvalidApiKey";
    pub const ACCESS_DENIED: &str = "AccessDenied";
    pub const WORKSPACE_NOT_FOUND: &str = "WorkspaceNotFound";

    /// Rate limiting
    pub const RATE_LIMIT_EXCEEDED: &str = "RateLimitExceeded";
    pub const QUOTA_EXCEEDED: &str = "QuotaExceeded";

    /// Request errors
    pub const INVALID_PARAMETER: &str = "InvalidParameter";
    pub const MODEL_NOT_FOUND: &str = "ModelNotFound";

    /// Server errors
    pub const INTERNAL_ERROR: &str = "InternalError";
    pub const SERVICE_BUSY: &str = "ServiceBusy";
}

/// Result type alias for DashScope operations.
pub type Result<T> = std::result::Result<T, Error>;

/// Error type for DashScope API operations.
#[derive(Error, Debug)]
pub enum Error {
    /// API error returned by DashScope.
    #[error("dashscope: {code} - {message} (request_id={request_id}, http_status={http_status})")]
    Api {
        code: String,
        message: String,
        request_id: String,
        http_status: u16,
    },

    /// WebSocket error.
    #[error("websocket error: {0}")]
    WebSocket(#[from] tokio_tungstenite::tungstenite::Error),

    /// HTTP request error.
    #[error("http error: {0}")]
    Http(#[from] reqwest::Error),

    /// JSON serialization/deserialization error.
    #[error("json error: {0}")]
    Json(#[from] serde_json::Error),

    /// Base64 decoding error.
    #[error("base64 decode error: {0}")]
    Base64Decode(#[from] base64::DecodeError),

    /// IO error.
    #[error("io error: {0}")]
    Io(#[from] std::io::Error),

    /// Invalid configuration.
    #[error("invalid configuration: {0}")]
    Config(String),

    /// Connection error.
    #[error("connection error: {0}")]
    Connection(String),

    /// Session closed.
    #[error("session closed")]
    SessionClosed,

    /// Other error.
    #[error("{0}")]
    Other(String),
}

impl Error {
    /// Creates a new API error.
    pub fn api(code: impl Into<String>, message: impl Into<String>, http_status: u16) -> Self {
        Error::Api {
            code: code.into(),
            message: message.into(),
            request_id: String::new(),
            http_status,
        }
    }

    /// Creates a new API error with request ID.
    pub fn api_with_request_id(
        code: impl Into<String>,
        message: impl Into<String>,
        request_id: impl Into<String>,
        http_status: u16,
    ) -> Self {
        Error::Api {
            code: code.into(),
            message: message.into(),
            request_id: request_id.into(),
            http_status,
        }
    }

    /// Returns true if this is a rate limit error.
    pub fn is_rate_limit(&self) -> bool {
        match self {
            Error::Api { code, http_status, .. } => {
                code == error_code::RATE_LIMIT_EXCEEDED
                    || code == error_code::QUOTA_EXCEEDED
                    || *http_status == 429
            }
            _ => false,
        }
    }

    /// Returns true if this is an authentication error.
    pub fn is_auth(&self) -> bool {
        match self {
            Error::Api { code, http_status, .. } => {
                code == error_code::INVALID_API_KEY
                    || code == error_code::ACCESS_DENIED
                    || *http_status == 401
            }
            _ => false,
        }
    }

    /// Returns true if this is a server-side error.
    pub fn is_server_error(&self) -> bool {
        match self {
            Error::Api { code, http_status, .. } => {
                code == error_code::INTERNAL_ERROR
                    || code == error_code::SERVICE_BUSY
                    || *http_status >= 500
            }
            _ => false,
        }
    }

    /// Returns true if the request can be retried.
    pub fn is_retryable(&self) -> bool {
        self.is_rate_limit() || self.is_server_error()
    }
}
