//! Error types for mqtt0-broker.

use std::fmt;
use std::io;

/// Result type alias.
pub type Result<T> = std::result::Result<T, Error>;

/// Broker error type.
#[derive(Debug)]
pub enum Error {
    /// Protocol error.
    Protocol(mqtt0::Error),

    /// IO error.
    Io(io::Error),

    /// Connection refused.
    ConnectionRefused(String),

    /// Authentication failed.
    AuthenticationFailed,

    /// ACL check failed.
    AclDenied(String),

    /// Connection closed.
    ConnectionClosed,

    /// Unexpected packet received.
    UnexpectedPacket { expected: String, got: String },

    /// Timeout.
    Timeout(String),

    /// Invalid configuration.
    InvalidConfig(String),

    /// Broker already running.
    AlreadyRunning,

    /// Broker shutting down.
    ShuttingDown,
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Error::Protocol(e) => write!(f, "protocol error: {}", e),
            Error::Io(e) => write!(f, "io error: {}", e),
            Error::ConnectionRefused(r) => write!(f, "connection refused: {}", r),
            Error::AuthenticationFailed => write!(f, "authentication failed"),
            Error::AclDenied(r) => write!(f, "ACL denied: {}", r),
            Error::ConnectionClosed => write!(f, "connection closed"),
            Error::UnexpectedPacket { expected, got } => {
                write!(f, "unexpected packet: expected {}, got {}", expected, got)
            }
            Error::Timeout(r) => write!(f, "timeout: {}", r),
            Error::InvalidConfig(r) => write!(f, "invalid config: {}", r),
            Error::AlreadyRunning => write!(f, "broker already running"),
            Error::ShuttingDown => write!(f, "broker shutting down"),
        }
    }
}

impl std::error::Error for Error {}

impl From<mqtt0::Error> for Error {
    fn from(e: mqtt0::Error) -> Self {
        Error::Protocol(e)
    }
}

impl From<io::Error> for Error {
    fn from(e: io::Error) -> Self {
        Error::Io(e)
    }
}
