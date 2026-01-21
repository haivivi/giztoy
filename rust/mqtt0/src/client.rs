//! QoS 0 MQTT client (mqtt0c).
//!
//! A lightweight MQTT client that supports both MQTT 3.1.1 (v4) and MQTT 5.0 (v5).

use bytes::BytesMut;
use std::sync::atomic::{AtomicBool, AtomicU16, Ordering};
use std::sync::Arc;
use std::time::Duration;
use tokio::io::{AsyncWriteExt, ReadHalf, WriteHalf};
use tokio::net::TcpStream;
use tokio::sync::Mutex;
use tracing::{debug, trace, warn};

use crate::error::{Error, Result};
use crate::protocol::{self, MAX_PACKET_SIZE};
use crate::types::{Message, ProtocolVersion};

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
    /// - None: use broker default
    /// - Some(0): session ends immediately on disconnect
    /// - Some(n): session persists for n seconds after disconnect
    /// - Some(0xFFFFFFFF): session never expires
    pub session_expiry: Option<u32>,
    /// Enable automatic keep-alive (sends PINGREQ at keep_alive/2 intervals).
    /// Default: true (like Go's autopaho).
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
            auto_keepalive: true, // Default: auto ping like Go's autopaho
        }
    }

    /// Set credentials.
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
    ///
    /// - `0`: session ends immediately on disconnect
    /// - `n`: session persists for n seconds after disconnect
    /// - `0xFFFFFFFF`: session never expires
    pub fn with_session_expiry(mut self, seconds: u32) -> Self {
        self.session_expiry = Some(seconds);
        self
    }

    /// Enable or disable automatic keep-alive.
    ///
    /// When enabled (default), the client will automatically send PINGREQ
    /// packets at `keep_alive / 2` intervals to prevent the broker from
    /// disconnecting the client due to inactivity.
    pub fn with_auto_keepalive(mut self, enabled: bool) -> Self {
        self.auto_keepalive = enabled;
        self
    }
}

/// Internal client state for different protocol versions.
#[derive(Clone, Copy)]
enum ClientState {
    V4,
    V5,
}

/// Shared state for keepalive task.
struct KeepaliveState {
    writer: Arc<Mutex<WriteHalf<TcpStream>>>,
    running: Arc<AtomicBool>,
    interval: Duration,
    state: ClientState,
}

/// QoS 0 MQTT client supporting both v4 and v5.
pub struct Client {
    reader: Mutex<ReadHalf<TcpStream>>,
    writer: Arc<Mutex<WriteHalf<TcpStream>>>,
    read_buf: Mutex<BytesMut>,
    client_id: String,
    next_pkid: AtomicU16,
    max_packet_size: usize,
    state: ClientState,
    /// Flag to stop the keepalive task.
    running: Arc<AtomicBool>,
}

impl Client {
    /// Connect to an MQTT broker.
    pub async fn connect(config: ClientConfig) -> Result<Self> {
        match config.protocol_version {
            ProtocolVersion::V4 => Self::connect_v4(config).await,
            ProtocolVersion::V5 => Self::connect_v5(config).await,
        }
    }

    /// Connect using MQTT 3.1.1 (v4).
    async fn connect_v4(config: ClientConfig) -> Result<Self> {
        use crate::protocol::v4::{ConnectReturnCode, Packet};

        debug!("Connecting to {} as {} (MQTT 3.1.1)", config.addr, config.client_id);

        let stream = TcpStream::connect(&config.addr).await?;
        let (reader, mut writer) = tokio::io::split(stream);

        // Send CONNECT packet
        let connect_packet = protocol::v4::create_connect(
            &config.client_id,
            config.username.as_deref(),
            config.password.as_deref(),
            config.keep_alive,
            config.clean_session,
        );

        let mut buf = BytesMut::with_capacity(connect_packet.size());
        connect_packet
            .write(&mut buf, config.max_packet_size)
            .map_err(|e| Error::Protocol(e.to_string()))?;
        writer.write_all(&buf).await?;
        writer.flush().await?;

        // Wait for CONNACK
        let mut read_buf = BytesMut::with_capacity(1024);
        let mut reader = reader;

        let packet = protocol::v4::read_packet(&mut reader, &mut read_buf, config.max_packet_size).await?;

        match packet {
            Packet::ConnAck(connack) => {
                if connack.code != ConnectReturnCode::Success {
                    return Err(Error::ConnectionRefused(format!("{:?}", connack.code)));
                }
                debug!("Connected successfully (v4), session_present={}", connack.session_present);
            }
            other => {
                return Err(Error::UnexpectedPacket {
                    expected: "ConnAck".to_string(),
                    got: format!("{:?}", other),
                });
            }
        }

        let running = Arc::new(AtomicBool::new(true));
        let writer = Arc::new(Mutex::new(writer));

        // Start keepalive task if enabled
        if config.auto_keepalive && config.keep_alive > 0 {
            let keepalive_state = KeepaliveState {
                writer: Arc::clone(&writer),
                running: Arc::clone(&running),
                interval: Duration::from_secs((config.keep_alive / 2).max(1) as u64),
                state: ClientState::V4,
            };
            tokio::spawn(Self::keepalive_task(keepalive_state));
            debug!("Started auto keepalive task (interval={}s)", config.keep_alive / 2);
        }

        Ok(Self {
            reader: Mutex::new(reader),
            writer,
            read_buf: Mutex::new(read_buf),
            client_id: config.client_id,
            next_pkid: AtomicU16::new(1),
            max_packet_size: config.max_packet_size,
            state: ClientState::V4,
            running,
        })
    }

    /// Connect using MQTT 5.0 (v5).
    async fn connect_v5(config: ClientConfig) -> Result<Self> {
        use crate::protocol::v5::{ConnectReturnCode, Packet};

        debug!(
            "Connecting to {} as {} (MQTT 5.0, session_expiry={:?})",
            config.addr, config.client_id, config.session_expiry
        );

        let stream = TcpStream::connect(&config.addr).await?;
        let (reader, mut writer) = tokio::io::split(stream);

        // Send CONNECT packet
        let connect_packet = protocol::v5::create_connect(
            &config.client_id,
            config.username.as_deref(),
            config.password.as_deref(),
            config.keep_alive,
            config.clean_session,
            config.session_expiry,
        );

        let max_size_opt = Some(config.max_packet_size as u32);
        let mut buf = BytesMut::with_capacity(connect_packet.size());
        connect_packet
            .write(&mut buf, max_size_opt)
            .map_err(|e| Error::Protocol(e.to_string()))?;
        writer.write_all(&buf).await?;
        writer.flush().await?;

        // Wait for CONNACK
        let mut read_buf = BytesMut::with_capacity(1024);
        let mut reader = reader;

        let packet = protocol::v5::read_packet(&mut reader, &mut read_buf, config.max_packet_size).await?;

        match packet {
            Packet::ConnAck(connack) => {
                if connack.code != ConnectReturnCode::Success {
                    return Err(Error::ConnectionRefused(format!("{:?}", connack.code)));
                }
                debug!("Connected successfully (v5), session_present={}", connack.session_present);
            }
            other => {
                return Err(Error::UnexpectedPacket {
                    expected: "ConnAck".to_string(),
                    got: format!("{:?}", other),
                });
            }
        }

        let running = Arc::new(AtomicBool::new(true));
        let writer = Arc::new(Mutex::new(writer));

        // Start keepalive task if enabled
        if config.auto_keepalive && config.keep_alive > 0 {
            let keepalive_state = KeepaliveState {
                writer: Arc::clone(&writer),
                running: Arc::clone(&running),
                interval: Duration::from_secs((config.keep_alive / 2).max(1) as u64),
                state: ClientState::V5,
            };
            tokio::spawn(Self::keepalive_task(keepalive_state));
            debug!("Started auto keepalive task (interval={}s)", config.keep_alive / 2);
        }

        Ok(Self {
            reader: Mutex::new(reader),
            writer,
            read_buf: Mutex::new(read_buf),
            client_id: config.client_id,
            next_pkid: AtomicU16::new(1),
            max_packet_size: config.max_packet_size,
            state: ClientState::V5,
            running,
        })
    }

    /// Get the client ID.
    pub fn client_id(&self) -> &str {
        &self.client_id
    }

    /// Publish a message (QoS 0, fire and forget).
    pub async fn publish(&self, topic: &str, payload: &[u8]) -> Result<()> {
        self.publish_with_retain(topic, payload, false).await
    }

    /// Publish a message with retain flag.
    pub async fn publish_with_retain(&self, topic: &str, payload: &[u8], retain: bool) -> Result<()> {
        trace!("Publishing to {} ({} bytes)", topic, payload.len());

        match &self.state {
            ClientState::V4 => {
                let packet = protocol::v4::create_publish(topic, payload, retain);
                let mut writer = self.writer.lock().await;
                protocol::v4::write_packet(&mut *writer, packet).await
            }
            ClientState::V5 => {
                let packet = protocol::v5::create_publish(topic, payload, retain);
                let mut writer = self.writer.lock().await;
                protocol::v5::write_packet(&mut *writer, packet).await
            }
        }
    }

    /// Subscribe to topics.
    pub async fn subscribe(&self, topics: &[&str]) -> Result<()> {
        if topics.is_empty() {
            return Ok(());
        }

        let pkid = self.next_pkid.fetch_add(1, Ordering::SeqCst);
        debug!("Subscribing to {:?} with pkid={}", topics, pkid);

        match &self.state {
            ClientState::V4 => self.subscribe_v4(pkid, topics).await,
            ClientState::V5 => self.subscribe_v5(pkid, topics).await,
        }
    }

    async fn subscribe_v4(&self, pkid: u16, topics: &[&str]) -> Result<()> {
        use crate::protocol::v4::Packet;

        let packet = protocol::v4::create_subscribe(pkid, topics);

        {
            let mut writer = self.writer.lock().await;
            protocol::v4::write_packet(&mut *writer, packet).await?;
        }

        let packet = {
            let mut reader = self.reader.lock().await;
            let mut read_buf = self.read_buf.lock().await;
            protocol::v4::read_packet(&mut *reader, &mut *read_buf, self.max_packet_size).await?
        };

        match packet {
            Packet::SubAck(suback) => {
                debug!("Received SubAck for pkid={}: {:?}", suback.pkid, suback.return_codes);
                for code in &suback.return_codes {
                    if *code == crate::protocol::v4::SubscribeReasonCode::Failure {
                        return Err(Error::AclDenied("subscription denied".to_string()));
                    }
                }
                Ok(())
            }
            other => Err(Error::UnexpectedPacket {
                expected: "SubAck".to_string(),
                got: format!("{:?}", other),
            }),
        }
    }

    async fn subscribe_v5(&self, pkid: u16, topics: &[&str]) -> Result<()> {
        use crate::protocol::v5::Packet;

        let packet = protocol::v5::create_subscribe(pkid, topics);

        {
            let mut writer = self.writer.lock().await;
            protocol::v5::write_packet(&mut *writer, packet).await?;
        }

        let packet = {
            let mut reader = self.reader.lock().await;
            let mut read_buf = self.read_buf.lock().await;
            protocol::v5::read_packet(&mut *reader, &mut *read_buf, self.max_packet_size).await?
        };

        match packet {
            Packet::SubAck(suback) => {
                debug!("Received SubAck for pkid={}: {:?}", suback.pkid, suback.return_codes);
                for code in &suback.return_codes {
                    match code {
                        crate::protocol::v5::SubscribeReasonCode::Success(_) => {}
                        _ => return Err(Error::AclDenied("subscription denied".to_string())),
                    }
                }
                Ok(())
            }
            other => Err(Error::UnexpectedPacket {
                expected: "SubAck".to_string(),
                got: format!("{:?}", other),
            }),
        }
    }

    /// Unsubscribe from topics.
    pub async fn unsubscribe(&self, topics: &[&str]) -> Result<()> {
        if topics.is_empty() {
            return Ok(());
        }

        let pkid = self.next_pkid.fetch_add(1, Ordering::SeqCst);
        debug!("Unsubscribing from {:?} with pkid={}", topics, pkid);

        match &self.state {
            ClientState::V4 => self.unsubscribe_v4(pkid, topics).await,
            ClientState::V5 => self.unsubscribe_v5(pkid, topics).await,
        }
    }

    async fn unsubscribe_v4(&self, pkid: u16, topics: &[&str]) -> Result<()> {
        use crate::protocol::v4::Packet;

        let packet = protocol::v4::create_unsubscribe(pkid, topics);

        {
            let mut writer = self.writer.lock().await;
            protocol::v4::write_packet(&mut *writer, packet).await?;
        }

        let packet = {
            let mut reader = self.reader.lock().await;
            let mut read_buf = self.read_buf.lock().await;
            protocol::v4::read_packet(&mut *reader, &mut *read_buf, self.max_packet_size).await?
        };

        match packet {
            Packet::UnsubAck(_) => Ok(()),
            other => Err(Error::UnexpectedPacket {
                expected: "UnsubAck".to_string(),
                got: format!("{:?}", other),
            }),
        }
    }

    async fn unsubscribe_v5(&self, pkid: u16, topics: &[&str]) -> Result<()> {
        use crate::protocol::v5::Packet;

        let packet = protocol::v5::create_unsubscribe(pkid, topics);

        {
            let mut writer = self.writer.lock().await;
            protocol::v5::write_packet(&mut *writer, packet).await?;
        }

        let packet = {
            let mut reader = self.reader.lock().await;
            let mut read_buf = self.read_buf.lock().await;
            protocol::v5::read_packet(&mut *reader, &mut *read_buf, self.max_packet_size).await?
        };

        match packet {
            Packet::UnsubAck(_) => Ok(()),
            other => Err(Error::UnexpectedPacket {
                expected: "UnsubAck".to_string(),
                got: format!("{:?}", other),
            }),
        }
    }

    /// Receive the next message.
    pub async fn recv(&self) -> Result<Message> {
        match &self.state {
            ClientState::V4 => self.recv_v4().await,
            ClientState::V5 => self.recv_v5().await,
        }
    }

    async fn recv_v4(&self) -> Result<Message> {
        use crate::protocol::v4::Packet;

        loop {
            let packet = {
                let mut reader = self.reader.lock().await;
                let mut read_buf = self.read_buf.lock().await;
                protocol::v4::read_packet(&mut *reader, &mut *read_buf, self.max_packet_size).await?
            };

            match packet {
                Packet::Publish(publish) => {
                    trace!("Received message on {}", publish.topic);
                    return Ok(Message {
                        topic: publish.topic.to_string(),
                        payload: publish.payload,
                        retain: publish.retain,
                    });
                }
                Packet::PingResp => {
                    trace!("Received PingResp");
                    continue;
                }
                Packet::Disconnect => {
                    return Err(Error::ConnectionClosed);
                }
                other => {
                    trace!("Ignoring packet: {:?}", other);
                    continue;
                }
            }
        }
    }

    async fn recv_v5(&self) -> Result<Message> {
        use crate::protocol::v5::Packet;

        loop {
            let packet = {
                let mut reader = self.reader.lock().await;
                let mut read_buf = self.read_buf.lock().await;
                protocol::v5::read_packet(&mut *reader, &mut *read_buf, self.max_packet_size).await?
            };

            match packet {
                Packet::Publish(publish) => {
                    let topic = String::from_utf8_lossy(&publish.topic).to_string();
                    trace!("Received message on {}", topic);
                    return Ok(Message {
                        topic,
                        payload: publish.payload,
                        retain: publish.retain,
                    });
                }
                Packet::PingResp(_) => {
                    trace!("Received PingResp");
                    continue;
                }
                Packet::Disconnect(_) => {
                    return Err(Error::ConnectionClosed);
                }
                other => {
                    trace!("Ignoring packet: {:?}", other);
                    continue;
                }
            }
        }
    }

    /// Receive a message with timeout.
    pub async fn recv_timeout(&self, timeout: Duration) -> Result<Option<Message>> {
        match tokio::time::timeout(timeout, self.recv()).await {
            Ok(result) => result.map(Some),
            Err(_) => Ok(None),
        }
    }

    /// Send a ping request.
    pub async fn ping(&self) -> Result<()> {
        trace!("Sending PingReq");
        match &self.state {
            ClientState::V4 => {
                let mut writer = self.writer.lock().await;
                protocol::v4::write_packet(&mut *writer, protocol::v4::create_pingreq()).await
            }
            ClientState::V5 => {
                let mut writer = self.writer.lock().await;
                protocol::v5::write_packet(&mut *writer, protocol::v5::create_pingreq()).await
            }
        }
    }

    /// Disconnect from the broker.
    pub async fn disconnect(&self) -> Result<()> {
        debug!("Disconnecting");

        // Stop keepalive task
        self.running.store(false, Ordering::SeqCst);

        match &self.state {
            ClientState::V4 => {
                let mut writer = self.writer.lock().await;
                protocol::v4::write_packet(&mut *writer, protocol::v4::create_disconnect()).await
            }
            ClientState::V5 => {
                let mut writer = self.writer.lock().await;
                protocol::v5::write_packet(&mut *writer, protocol::v5::create_disconnect()).await
            }
        }
    }

    /// Check if the client is still running (not disconnected).
    pub fn is_running(&self) -> bool {
        self.running.load(Ordering::SeqCst)
    }

    /// Background task for automatic keep-alive.
    async fn keepalive_task(state: KeepaliveState) {
        loop {
            tokio::time::sleep(state.interval).await;

            if !state.running.load(Ordering::SeqCst) {
                trace!("Keepalive task stopping");
                break;
            }

            let result = {
                let mut writer = state.writer.lock().await;
                match state.state {
                    ClientState::V4 => {
                        protocol::v4::write_packet(&mut *writer, protocol::v4::create_pingreq()).await
                    }
                    ClientState::V5 => {
                        protocol::v5::write_packet(&mut *writer, protocol::v5::create_pingreq()).await
                    }
                }
            };

            if let Err(e) = result {
                warn!("Keepalive ping failed: {}", e);
                state.running.store(false, Ordering::SeqCst);
                break;
            }

            trace!("Keepalive ping sent");
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_client_config_v4() {
        let config = ClientConfig::new("127.0.0.1:1883", "test-client")
            .with_credentials("user", b"pass".to_vec())
            .with_keep_alive(30)
            .with_clean_session(false);

        assert_eq!(config.addr, "127.0.0.1:1883");
        assert_eq!(config.client_id, "test-client");
        assert_eq!(config.username, Some("user".to_string()));
        assert_eq!(config.password, Some(b"pass".to_vec()));
        assert_eq!(config.keep_alive, 30);
        assert!(!config.clean_session);
        assert_eq!(config.protocol_version, ProtocolVersion::V4);
    }

    #[test]
    fn test_client_config_v5() {
        let config = ClientConfig::new("127.0.0.1:1883", "test-client")
            .with_protocol(ProtocolVersion::V5)
            .with_session_expiry(3600)
            .with_credentials("user", b"pass".to_vec());

        assert_eq!(config.protocol_version, ProtocolVersion::V5);
        assert_eq!(config.session_expiry, Some(3600));
    }
}
