//! Error types for mqtt0-client.

use core::fmt;

/// Result type alias.
pub type Result<T> = core::result::Result<T, Error>;

/// Client error type.
#[derive(Debug)]
pub enum Error {
    /// Protocol error.
    Protocol(mqtt0::Error),

    /// IO error.
    #[cfg(feature = "tokio")]
    Io(std::io::Error),

    /// Connection refused by broker.
    ConnectionRefused(ConnectionRefusedReason),

    /// Authentication failed.
    AuthenticationFailed,

    /// ACL check failed.
    AclDenied,

    /// Connection closed.
    ConnectionClosed,

    /// Unexpected packet received.
    UnexpectedPacket,

    /// Timeout.
    Timeout,

    /// Invalid configuration.
    InvalidConfig,
}

/// Connection refused reasons.
#[derive(Debug, Clone, Copy)]
pub enum ConnectionRefusedReason {
    UnacceptableProtocolVersion,
    IdentifierRejected,
    ServerUnavailable,
    BadUsernamePassword,
    NotAuthorized,
    Other(u8),
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Error::Protocol(e) => write!(f, "protocol error: {}", e),
            #[cfg(feature = "tokio")]
            Error::Io(e) => write!(f, "io error: {}", e),
            Error::ConnectionRefused(r) => write!(f, "connection refused: {:?}", r),
            Error::AuthenticationFailed => write!(f, "authentication failed"),
            Error::AclDenied => write!(f, "ACL denied"),
            Error::ConnectionClosed => write!(f, "connection closed"),
            Error::UnexpectedPacket => write!(f, "unexpected packet"),
            Error::Timeout => write!(f, "timeout"),
            Error::InvalidConfig => write!(f, "invalid configuration"),
        }
    }
}

#[cfg(feature = "tokio")]
impl std::error::Error for Error {}

impl From<mqtt0::Error> for Error {
    fn from(e: mqtt0::Error) -> Self {
        Error::Protocol(e)
    }
}

#[cfg(feature = "tokio")]
impl From<std::io::Error> for Error {
    fn from(e: std::io::Error) -> Self {
        Error::Io(e)
    }
}
