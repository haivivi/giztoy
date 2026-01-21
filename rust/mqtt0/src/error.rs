//! Error types for mqtt0.

use std::io;

/// Result type alias for mqtt0.
pub type Result<T> = std::result::Result<T, Error>;

/// Error type for mqtt0 operations.
#[derive(Debug, thiserror::Error)]
pub enum Error {
    /// IO error.
    #[error("io error: {0}")]
    Io(#[from] io::Error),

    /// Protocol error.
    #[error("protocol error: {0}")]
    Protocol(String),

    /// Connection refused by broker.
    #[error("connection refused: {0}")]
    ConnectionRefused(String),

    /// Authentication failed.
    #[error("authentication failed")]
    AuthenticationFailed,

    /// ACL check failed (publish/subscribe denied).
    #[error("acl denied: {0}")]
    AclDenied(String),

    /// Connection closed by peer.
    #[error("connection closed")]
    ConnectionClosed,

    /// Unexpected packet received.
    #[error("unexpected packet: expected {expected}, got {got}")]
    UnexpectedPacket { expected: String, got: String },

    /// Timeout error.
    #[error("timeout: {0}")]
    Timeout(String),

    /// Invalid configuration.
    #[error("invalid config: {0}")]
    InvalidConfig(String),

    /// Broker is already running.
    #[error("broker already running")]
    AlreadyRunning,

    /// Broker is shutting down.
    #[error("broker shutting down")]
    ShuttingDown,
}

impl From<rumqttc::mqttbytes::Error> for Error {
    fn from(e: rumqttc::mqttbytes::Error) -> Self {
        Error::Protocol(e.to_string())
    }
}
