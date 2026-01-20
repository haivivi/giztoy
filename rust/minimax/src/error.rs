//! Error types for the MiniMax API client.

use thiserror::Error;

/// API error status codes.
pub mod status_code {
    pub const INVALID_API_KEY: i32 = 1001;
    pub const RATE_LIMIT: i32 = 1002;
    pub const INSUFFICIENT_QUOTA: i32 = 1003;
    pub const INVALID_REQUEST_MIN: i32 = 2000;
    pub const INVALID_REQUEST_MAX: i32 = 2999;
    pub const SERVER_ERROR_MIN: i32 = 5000;
}

/// Result type alias for MiniMax operations.
pub type Result<T> = std::result::Result<T, Error>;

/// Error type for MiniMax API operations.
#[derive(Error, Debug)]
pub enum Error {
    /// API error returned by MiniMax.
    #[error("minimax: {status_msg} (code={status_code}, trace={trace_id})")]
    Api {
        status_code: i32,
        status_msg: String,
        trace_id: String,
        http_status: u16,
    },

    /// HTTP request error.
    #[error("http error: {0}")]
    Http(#[from] reqwest::Error),

    /// JSON serialization/deserialization error.
    #[error("json error: {0}")]
    Json(#[from] serde_json::Error),

    /// Hex decoding error.
    #[error("hex decode error: {0}")]
    HexDecode(#[from] hex::FromHexError),

    /// IO error.
    #[error("io error: {0}")]
    Io(#[from] std::io::Error),

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
    pub fn api(status_code: i32, status_msg: impl Into<String>, http_status: u16) -> Self {
        Error::Api {
            status_code,
            status_msg: status_msg.into(),
            trace_id: String::new(),
            http_status,
        }
    }

    /// Creates a new API error with trace ID.
    pub fn api_with_trace(
        status_code: i32,
        status_msg: impl Into<String>,
        trace_id: impl Into<String>,
        http_status: u16,
    ) -> Self {
        Error::Api {
            status_code,
            status_msg: status_msg.into(),
            trace_id: trace_id.into(),
            http_status,
        }
    }

    /// Returns true if this is a rate limit error.
    pub fn is_rate_limit(&self) -> bool {
        match self {
            Error::Api {
                status_code,
                http_status,
                ..
            } => *status_code == status_code::RATE_LIMIT || *http_status == 429,
            _ => false,
        }
    }

    /// Returns true if this is an invalid API key error.
    pub fn is_invalid_api_key(&self) -> bool {
        match self {
            Error::Api {
                status_code,
                http_status,
                ..
            } => *status_code == status_code::INVALID_API_KEY || *http_status == 401,
            _ => false,
        }
    }

    /// Returns true if this is an insufficient quota error.
    pub fn is_insufficient_quota(&self) -> bool {
        match self {
            Error::Api { status_code, .. } => *status_code == status_code::INSUFFICIENT_QUOTA,
            _ => false,
        }
    }

    /// Returns true if this is an invalid request error.
    pub fn is_invalid_request(&self) -> bool {
        match self {
            Error::Api { status_code, .. } => {
                *status_code >= status_code::INVALID_REQUEST_MIN
                    && *status_code <= status_code::INVALID_REQUEST_MAX
            }
            _ => false,
        }
    }

    /// Returns true if this is a server-side error.
    pub fn is_server_error(&self) -> bool {
        match self {
            Error::Api {
                status_code,
                http_status,
                ..
            } => *status_code >= status_code::SERVER_ERROR_MIN || *http_status >= 500,
            _ => false,
        }
    }

    /// Returns true if the request can be retried.
    pub fn is_retryable(&self) -> bool {
        self.is_rate_limit() || self.is_server_error()
    }
}
