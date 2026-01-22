//! Error types for mqtt0.
//!
//! This module provides error types that work in both `std` and `no_std` environments.

#[cfg(feature = "alloc")]
use alloc::string::String;

use core::fmt;

/// Result type alias for mqtt0.
pub type Result<T> = core::result::Result<T, Error>;

/// Error type for mqtt0 operations.
#[derive(Debug)]
pub enum Error {
    /// Protocol error.
    Protocol(ProtocolError),

    /// Buffer too small.
    BufferTooSmall {
        required: usize,
        available: usize,
    },

    /// Invalid packet type.
    InvalidPacketType(u8),

    /// Invalid remaining length encoding.
    InvalidRemainingLength,

    /// Packet too large.
    PacketTooLarge {
        size: usize,
        max: usize,
    },

    /// Incomplete packet (need more data).
    Incomplete {
        needed: usize,
    },

    /// Invalid UTF-8 string.
    InvalidUtf8,

    /// Invalid QoS value.
    InvalidQoS(u8),

    /// Invalid protocol version.
    InvalidProtocolVersion(u8),

    /// Wildcard '#' must be last segment.
    WildcardNotLast,

    /// Invalid topic filter.
    #[cfg(feature = "alloc")]
    InvalidTopicFilter(String),

    /// Invalid topic filter (no_std version).
    #[cfg(not(feature = "alloc"))]
    InvalidTopicFilter,
}

/// Protocol-specific errors.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ProtocolError {
    /// Malformed packet.
    MalformedPacket,
    /// Unsupported protocol level.
    UnsupportedProtocolLevel,
    /// Invalid connect flags.
    InvalidConnectFlags,
    /// Invalid property identifier.
    InvalidPropertyId(u8),
    /// Missing required field.
    MissingRequiredField,
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Error::Protocol(e) => write!(f, "protocol error: {:?}", e),
            Error::BufferTooSmall { required, available } => {
                write!(f, "buffer too small: need {} bytes, have {}", required, available)
            }
            Error::InvalidPacketType(t) => write!(f, "invalid packet type: {}", t),
            Error::InvalidRemainingLength => write!(f, "invalid remaining length encoding"),
            Error::PacketTooLarge { size, max } => {
                write!(f, "packet too large: {} bytes, max {}", size, max)
            }
            Error::Incomplete { needed } => write!(f, "incomplete packet, need {} more bytes", needed),
            Error::InvalidUtf8 => write!(f, "invalid UTF-8 string"),
            Error::InvalidQoS(q) => write!(f, "invalid QoS value: {}", q),
            Error::InvalidProtocolVersion(v) => write!(f, "invalid protocol version: {}", v),
            Error::WildcardNotLast => write!(f, "wildcard '#' must be last segment"),
            #[cfg(feature = "alloc")]
            Error::InvalidTopicFilter(s) => write!(f, "invalid topic filter: {}", s),
            #[cfg(not(feature = "alloc"))]
            Error::InvalidTopicFilter => write!(f, "invalid topic filter"),
        }
    }
}

#[cfg(feature = "std")]
impl std::error::Error for Error {}

impl From<ProtocolError> for Error {
    fn from(e: ProtocolError) -> Self {
        Error::Protocol(e)
    }
}
