//! Error types for the Doubao Speech API client.

use thiserror::Error;

/// API error status codes.
pub mod status_code {
    /// Success
    pub const SUCCESS: i32 = 3000;
    /// Parameter error
    pub const PARAM_ERROR: i32 = 3001;
    /// Authentication error
    pub const AUTH_ERROR: i32 = 3002;
    /// Rate limit exceeded
    pub const RATE_LIMIT: i32 = 3003;
    /// Quota exceeded
    pub const QUOTA_EXCEED: i32 = 3004;
    /// Server error
    pub const SERVER_ERROR: i32 = 3005;
    /// ASR success
    pub const ASR_SUCCESS: i32 = 1000;
}

/// Result type alias for Doubao Speech operations.
pub type Result<T> = std::result::Result<T, Error>;

/// Error type for Doubao Speech API operations.
#[derive(Error, Debug)]
pub enum Error {
    /// API error returned by Doubao Speech.
    #[error("doubaospeech: {message} (code={code}, req_id={req_id}, log_id={log_id})")]
    Api {
        code: i32,
        message: String,
        req_id: String,
        log_id: String,
        trace_id: String,
        http_status: u16,
    },

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

    /// WebSocket error.
    #[error("websocket error: {0}")]
    WebSocket(#[from] tokio_tungstenite::tungstenite::Error),

    /// Invalid configuration.
    #[error("invalid configuration: {0}")]
    Config(String),

    /// Task failed.
    #[error("task failed: {0}")]
    TaskFailed(String),

    /// Other error.
    #[error("{0}")]
    Other(String),
}

impl Error {
    /// Creates a new API error.
    pub fn api(code: i32, message: impl Into<String>, http_status: u16) -> Self {
        Error::Api {
            code,
            message: message.into(),
            req_id: String::new(),
            log_id: String::new(),
            trace_id: String::new(),
            http_status,
        }
    }

    /// Creates a new API error with request ID.
    pub fn api_with_req_id(
        code: i32,
        message: impl Into<String>,
        req_id: impl Into<String>,
        http_status: u16,
    ) -> Self {
        Error::Api {
            code,
            message: message.into(),
            req_id: req_id.into(),
            log_id: String::new(),
            trace_id: String::new(),
            http_status,
        }
    }

    /// Creates a new API error with log ID.
    pub fn api_with_log_id(
        code: i32,
        message: impl Into<String>,
        log_id: impl Into<String>,
        http_status: u16,
    ) -> Self {
        Error::Api {
            code,
            message: message.into(),
            req_id: String::new(),
            log_id: log_id.into(),
            trace_id: String::new(),
            http_status,
        }
    }

    /// Returns true if this is an authentication error.
    pub fn is_auth_error(&self) -> bool {
        match self {
            Error::Api {
                code, http_status, ..
            } => *code == status_code::AUTH_ERROR || *http_status == 401 || *http_status == 403,
            _ => false,
        }
    }

    /// Returns true if this is a rate limit error.
    pub fn is_rate_limit(&self) -> bool {
        match self {
            Error::Api {
                code, http_status, ..
            } => *code == status_code::RATE_LIMIT || *http_status == 429,
            _ => false,
        }
    }

    /// Returns true if this is a quota exceeded error.
    pub fn is_quota_exceeded(&self) -> bool {
        match self {
            Error::Api {
                code, http_status, ..
            } => *code == status_code::QUOTA_EXCEED || *http_status == 402,
            _ => false,
        }
    }

    /// Returns true if this is an invalid parameter error.
    pub fn is_invalid_param(&self) -> bool {
        match self {
            Error::Api {
                code, http_status, ..
            } => *code == status_code::PARAM_ERROR || *http_status == 400,
            _ => false,
        }
    }

    /// Returns true if this is a server-side error.
    pub fn is_server_error(&self) -> bool {
        match self {
            Error::Api {
                code, http_status, ..
            } => *code == status_code::SERVER_ERROR || *http_status >= 500,
            _ => false,
        }
    }

    /// Returns true if the request can be retried.
    pub fn is_retryable(&self) -> bool {
        self.is_rate_limit() || self.is_server_error()
    }
}
