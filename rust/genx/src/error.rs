//! Error types and stream status for GenX.

use serde::{Deserialize, Serialize};
use std::fmt;
use thiserror::Error;

/// Status of a streaming response.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Status {
    /// Normal chunk, more coming
    Ok,
    /// Stream completed successfully
    Done,
    /// Response was truncated (max tokens reached)
    Truncated,
    /// Response was blocked (safety filter, etc.)
    Blocked,
    /// An error occurred
    Error,
}

impl fmt::Display for Status {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Status::Ok => write!(f, "ok"),
            Status::Done => write!(f, "done"),
            Status::Truncated => write!(f, "truncated"),
            Status::Blocked => write!(f, "blocked"),
            Status::Error => write!(f, "error"),
        }
    }
}

/// Token usage statistics.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct Usage {
    /// Number of tokens in the prompt
    pub prompt_token_count: i64,
    /// Number of tokens from cached content
    pub cached_content_token_count: i64,
    /// Number of tokens generated
    pub generated_token_count: i64,
}

impl Usage {
    /// Create a new Usage with all zero counts.
    pub fn new() -> Self {
        Self::default()
    }

    /// Create a new Usage with the given counts.
    pub fn with_counts(prompt: i64, cached: i64, generated: i64) -> Self {
        Self {
            prompt_token_count: prompt,
            cached_content_token_count: cached,
            generated_token_count: generated,
        }
    }

    /// Total tokens used (prompt + generated).
    pub fn total(&self) -> i64 {
        self.prompt_token_count + self.generated_token_count
    }
}

impl fmt::Display for Usage {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "Usage(prompt={}, cached={}, generated={})",
            self.prompt_token_count, self.cached_content_token_count, self.generated_token_count
        )
    }
}

/// Error type for GenX operations.
#[derive(Error, Debug)]
pub enum GenxError {
    /// Stream completed normally
    #[error("stream done")]
    Done(Usage),

    /// Response was truncated
    #[error("response truncated (max tokens)")]
    Truncated(Usage),

    /// Response was blocked
    #[error("response blocked: {reason}")]
    Blocked { usage: Usage, reason: String },

    /// Generation error
    #[error("generation error: {message}")]
    Generation { usage: Usage, message: String },

    /// Tool not found
    #[error("tool not found: {name}")]
    ToolNotFound { name: String },

    /// Invalid arguments
    #[error("invalid arguments: {message}")]
    InvalidArguments { message: String },

    /// Serialization error
    #[error("serialization error: {0}")]
    Serialization(#[from] serde_json::Error),

    /// Other error
    #[error("{0}")]
    Other(#[from] anyhow::Error),
}

impl GenxError {
    /// Get the usage statistics if available.
    pub fn usage(&self) -> Option<&Usage> {
        match self {
            GenxError::Done(u) => Some(u),
            GenxError::Truncated(u) => Some(u),
            GenxError::Blocked { usage, .. } => Some(usage),
            GenxError::Generation { usage, .. } => Some(usage),
            _ => None,
        }
    }

    /// Get the status corresponding to this error.
    pub fn status(&self) -> Status {
        match self {
            GenxError::Done(_) => Status::Done,
            GenxError::Truncated(_) => Status::Truncated,
            GenxError::Blocked { .. } => Status::Blocked,
            GenxError::Generation { .. } => Status::Error,
            _ => Status::Error,
        }
    }

    /// Check if this error indicates normal completion.
    pub fn is_done(&self) -> bool {
        matches!(self, GenxError::Done(_))
    }
}

/// A state that wraps error with additional context.
///
/// This is similar to Go's State type that implements the error interface.
#[derive(Debug)]
pub struct State {
    usage: Usage,
    status: Status,
    error: GenxError,
}

impl State {
    /// Create a "done" state.
    pub fn done(usage: Usage) -> Self {
        Self {
            usage: usage.clone(),
            status: Status::Done,
            error: GenxError::Done(usage),
        }
    }

    /// Create a "truncated" state.
    pub fn truncated(usage: Usage) -> Self {
        Self {
            usage: usage.clone(),
            status: Status::Truncated,
            error: GenxError::Truncated(usage),
        }
    }

    /// Create a "blocked" state.
    pub fn blocked(usage: Usage, reason: impl Into<String>) -> Self {
        let reason = reason.into();
        Self {
            usage: usage.clone(),
            status: Status::Blocked,
            error: GenxError::Blocked { usage, reason },
        }
    }

    /// Create an "error" state.
    pub fn error(usage: Usage, message: impl Into<String>) -> Self {
        let message = message.into();
        Self {
            usage: usage.clone(),
            status: Status::Error,
            error: GenxError::Generation { usage, message },
        }
    }

    /// Get the usage statistics.
    pub fn usage(&self) -> &Usage {
        &self.usage
    }

    /// Get the status.
    pub fn status(&self) -> Status {
        self.status
    }

    /// Convert to the underlying error.
    pub fn into_error(self) -> GenxError {
        self.error
    }
}

impl fmt::Display for State {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.error)
    }
}

impl std::error::Error for State {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        Some(&self.error)
    }
}

impl From<State> for GenxError {
    fn from(state: State) -> Self {
        state.error
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_status_display() {
        assert_eq!(Status::Ok.to_string(), "ok");
        assert_eq!(Status::Done.to_string(), "done");
        assert_eq!(Status::Truncated.to_string(), "truncated");
        assert_eq!(Status::Blocked.to_string(), "blocked");
        assert_eq!(Status::Error.to_string(), "error");
    }

    #[test]
    fn test_usage() {
        let usage = Usage::with_counts(100, 10, 50);
        assert_eq!(usage.total(), 150);
        assert_eq!(usage.prompt_token_count, 100);
        assert_eq!(usage.cached_content_token_count, 10);
        assert_eq!(usage.generated_token_count, 50);
    }

    #[test]
    fn test_state_done() {
        let usage = Usage::with_counts(100, 0, 50);
        let state = State::done(usage.clone());

        assert_eq!(state.status(), Status::Done);
        assert_eq!(state.usage().prompt_token_count, 100);

        let err = state.into_error();
        assert!(err.is_done());
    }

    #[test]
    fn test_state_blocked() {
        let usage = Usage::new();
        let state = State::blocked(usage, "safety filter triggered");

        assert_eq!(state.status(), Status::Blocked);
        assert!(state.to_string().contains("blocked"));
    }

    #[test]
    fn test_genx_error_status() {
        let err = GenxError::Done(Usage::new());
        assert_eq!(err.status(), Status::Done);

        let err = GenxError::Truncated(Usage::new());
        assert_eq!(err.status(), Status::Truncated);

        let err = GenxError::ToolNotFound {
            name: "test".to_string(),
        };
        assert_eq!(err.status(), Status::Error);
    }

    #[test]
    fn t_err_truncated() {
        let usage = Usage::with_counts(50, 0, 100);
        let state = State::truncated(usage.clone());
        assert_eq!(state.status(), Status::Truncated);
        assert_eq!(state.usage().generated_token_count, 100);
        let err = state.into_error();
        assert!(matches!(err, GenxError::Truncated(_)));
    }

    #[test]
    fn t_err_unexpected_status() {
        let usage = Usage::new();
        let state = State::error(usage, "unexpected failure");
        assert_eq!(state.status(), Status::Error);
        assert!(state.to_string().contains("unexpected failure"));
    }

    #[test]
    fn t_err_done() {
        let err = GenxError::Done(Usage::with_counts(10, 0, 5));
        assert!(err.is_done());
        assert_eq!(err.status(), Status::Done);
        assert_eq!(err.usage().unwrap().prompt_token_count, 10);
    }

    #[test]
    fn t_usage_cached_content() {
        let usage = Usage::with_counts(200, 150, 50);
        assert_eq!(usage.cached_content_token_count, 150);
        assert_eq!(usage.total(), 250);
        let display = usage.to_string();
        assert!(display.contains("150"));
    }
}
