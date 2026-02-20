//! Connection traits for client-server communication.

use crate::{GearStateEvent, GearStatsEvent, SessionCommandEvent};
use async_trait::async_trait;

/// Error type for connection operations.
#[derive(Debug, Clone, thiserror::Error)]
pub enum ConnError {
    #[error("connection closed")]
    Closed,
    #[error("send failed: {0}")]
    SendFailed(String),
    #[error("receive failed: {0}")]
    ReceiveFailed(String),
    #[error("timeout")]
    Timeout,
    #[error("io error: {0}")]
    Io(String),
    #[error("other error: {0}")]
    Other(String),
}

impl From<std::io::Error> for ConnError {
    fn from(e: std::io::Error) -> Self {
        ConnError::Io(e.to_string())
    }
}

/// An opus frame with its associated timestamp.
#[derive(Debug, Clone)]
pub struct StampedOpusFrame {
    /// Timestamp of the frame (milliseconds since epoch).
    pub timestamp_ms: i64,
    /// Raw opus frame bytes.
    pub frame: Vec<u8>,
}

impl StampedOpusFrame {
    /// Creates a new stamped opus frame.
    pub fn new(timestamp_ms: i64, frame: Vec<u8>) -> Self {
        Self { timestamp_ms, frame }
    }

    /// Creates a stamped opus frame with the current time.
    pub fn now(frame: Vec<u8>) -> Self {
        let timestamp_ms = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_millis() as i64;
        Self { timestamp_ms, frame }
    }
}

// =============================================================================
// Uplink: Client -> Server
// =============================================================================

/// Client-side transmitter for sending data to the server.
#[async_trait]
pub trait UplinkTx: Send + Sync {
    /// Sends a stamped opus frame to the server.
    async fn send_opus_frame(&self, frame: &StampedOpusFrame) -> Result<(), ConnError>;

    /// Sends a state event to the server.
    async fn send_state(&self, state: &GearStateEvent) -> Result<(), ConnError>;

    /// Sends a stats event to the server.
    async fn send_stats(&self, stats: &GearStatsEvent) -> Result<(), ConnError>;

    /// Closes the uplink.
    async fn close(&self) -> Result<(), ConnError>;
}

/// Server-side receiver for data from the client.
#[async_trait]
pub trait UplinkRx: Send + Sync {
    /// Receives the next opus frame from the client.
    /// Returns Ok(None) when the connection is closed normally.
    async fn recv_opus_frame(&self) -> Result<Option<StampedOpusFrame>, ConnError>;

    /// Receives the next state event from the client.
    /// Returns Ok(None) when the connection is closed normally.
    async fn recv_state(&self) -> Result<Option<GearStateEvent>, ConnError>;

    /// Receives the next stats event from the client.
    /// Returns Ok(None) when the connection is closed normally.
    async fn recv_stats(&self) -> Result<Option<GearStatsEvent>, ConnError>;

    /// Returns the latest stats event from the client, if any.
    async fn latest_stats(&self) -> Option<GearStatsEvent>;

    /// Closes the receiver.
    async fn close(&self) -> Result<(), ConnError>;
}

// =============================================================================
// Downlink: Server -> Client
// =============================================================================

/// Opus encoding options for the connection.
#[derive(Debug, Clone, Default)]
pub struct OpusEncodeOptions {
    pub sample_rate: u32,
    pub channels: u8,
    pub frame_duration_ms: u32,
}

/// Server-side transmitter for sending data to the client.
#[async_trait]
pub trait DownlinkTx: Send + Sync {
    /// Sends a stamped opus frame to the client.
    async fn send_opus_frame(&self, frame: &StampedOpusFrame) -> Result<(), ConnError>;

    /// Sends a command event to the client.
    async fn send_command(&self, cmd: &SessionCommandEvent) -> Result<(), ConnError>;

    /// Closes the downlink.
    async fn close(&self) -> Result<(), ConnError>;
}

/// Client-side receiver for data from the server.
#[async_trait]
pub trait DownlinkRx: Send + Sync {
    /// Receives the next opus frame from the server.
    /// Returns Ok(None) when the connection is closed normally.
    async fn recv_opus_frame(&self) -> Result<Option<StampedOpusFrame>, ConnError>;

    /// Receives the next command from the server.
    /// Returns Ok(None) when the connection is closed normally.
    async fn recv_command(&self) -> Result<Option<SessionCommandEvent>, ConnError>;

    /// Closes the receiver.
    async fn close(&self) -> Result<(), ConnError>;
}

// =============================================================================
// Connection Pair Traits
// =============================================================================

/// A server-side connection (receives uplink, sends downlink).
pub trait ServerConn: UplinkRx + DownlinkTx {}
impl<T: UplinkRx + DownlinkTx> ServerConn for T {}

/// A client-side connection (sends uplink, receives downlink).
pub trait ClientConn: UplinkTx + DownlinkRx {}
impl<T: UplinkTx + DownlinkRx> ClientConn for T {}

#[cfg(test)]
mod conn_tests {
    use super::*;

    #[test]
    fn test_conn_error_display() {
        let err = ConnError::Closed;
        assert_eq!(err.to_string(), "connection closed");

        let err = ConnError::SendFailed("test".to_string());
        assert!(err.to_string().contains("test"));
    }

    #[test]
    fn test_opus_encode_options_default() {
        let opts = OpusEncodeOptions::default();
        assert_eq!(opts.sample_rate, 0);
        assert_eq!(opts.channels, 0);
    }
}
