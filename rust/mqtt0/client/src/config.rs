//! Client configuration.

#[cfg(feature = "tokio")]
use std::string::String;

#[cfg(all(not(feature = "tokio"), feature = "embassy"))]
use alloc::string::String;
#[cfg(all(not(feature = "tokio"), feature = "embassy"))]
use alloc::vec::Vec;

use mqtt0::protocol::MAX_PACKET_SIZE;
use mqtt0::types::ProtocolVersion;

/// Client configuration.
#[derive(Debug, Clone)]
pub struct ClientConfig {
    /// Broker address (host:port).
    pub addr: String,
    /// Client ID.
    pub client_id: String,
    /// Username for authentication.
    pub username: Option<String>,
    /// Password for authentication.
    #[cfg(feature = "tokio")]
    pub password: Option<std::vec::Vec<u8>>,
    #[cfg(all(not(feature = "tokio"), feature = "embassy"))]
    pub password: Option<Vec<u8>>,
    /// Keep alive interval in seconds.
    pub keep_alive: u16,
    /// Clean session flag (v4) / Clean start flag (v5).
    pub clean_session: bool,
    /// Maximum packet size.
    pub max_packet_size: usize,
    /// Protocol version.
    pub protocol_version: ProtocolVersion,
    /// Session expiry interval in seconds (v5 only).
    pub session_expiry: Option<u32>,
    /// Enable automatic keep-alive.
    pub auto_keepalive: bool,
}

impl ClientConfig {
    /// Create a new client config (defaults to MQTT 3.1.1).
    pub fn new(addr: impl Into<String>, client_id: impl Into<String>) -> Self {
        Self {
            addr: addr.into(),
            client_id: client_id.into(),
            username: None,
            password: None,
            keep_alive: 60,
            clean_session: true,
            max_packet_size: MAX_PACKET_SIZE,
            protocol_version: ProtocolVersion::V4,
            session_expiry: None,
            auto_keepalive: true,
        }
    }

    /// Set credentials.
    #[cfg(feature = "tokio")]
    pub fn with_credentials(mut self, username: impl Into<String>, password: impl Into<std::vec::Vec<u8>>) -> Self {
        self.username = Some(username.into());
        self.password = Some(password.into());
        self
    }

    /// Set credentials.
    #[cfg(all(not(feature = "tokio"), feature = "embassy"))]
    pub fn with_credentials(mut self, username: impl Into<String>, password: impl Into<Vec<u8>>) -> Self {
        self.username = Some(username.into());
        self.password = Some(password.into());
        self
    }

    /// Set keep alive interval.
    pub fn with_keep_alive(mut self, seconds: u16) -> Self {
        self.keep_alive = seconds;
        self
    }

    /// Set clean session flag.
    pub fn with_clean_session(mut self, clean: bool) -> Self {
        self.clean_session = clean;
        self
    }

    /// Set protocol version.
    pub fn with_protocol(mut self, version: ProtocolVersion) -> Self {
        self.protocol_version = version;
        self
    }

    /// Set session expiry interval (MQTT 5.0 only).
    pub fn with_session_expiry(mut self, seconds: u32) -> Self {
        self.session_expiry = Some(seconds);
        self
    }

    /// Enable or disable automatic keep-alive.
    pub fn with_auto_keepalive(mut self, enabled: bool) -> Self {
        self.auto_keepalive = enabled;
        self
    }
}
