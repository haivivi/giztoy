//! Tokio-based MQTT client implementation.

use bytes::BytesMut;
use std::sync::atomic::{AtomicBool, AtomicU16, Ordering};
use std::sync::Arc;
use std::time::Duration;
use tokio::io::{AsyncReadExt, AsyncWriteExt, ReadHalf, WriteHalf};
use tokio::net::TcpStream;
use tokio::sync::Mutex;
use tracing::{debug, trace, warn};

use mqtt0::protocol::{v4, v5};
use mqtt0::types::{ConnectReturnCodeV4, Message, ProtocolVersion};

use crate::config::ClientConfig;
use crate::error::{ConnectionRefusedReason, Error, Result};

/// Protocol state.
#[derive(Clone, Copy)]
#[allow(dead_code)]
enum ClientState {
    V4,
    V5,
}

/// Keepalive task state.
struct KeepaliveState {
    writer: Arc<Mutex<WriteHalf<TcpStream>>>,
    running: Arc<AtomicBool>,
    interval: Duration,
    #[allow(dead_code)]
    state: ClientState,
}

/// QoS 0 MQTT client.
pub struct Client {
    reader: Mutex<ReadHalf<TcpStream>>,
    writer: Arc<Mutex<WriteHalf<TcpStream>>>,
    read_buf: Mutex<BytesMut>,
    client_id: String,
    next_pkid: AtomicU16,
    max_packet_size: usize,
    state: ClientState,
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

    async fn connect_v4(config: ClientConfig) -> Result<Self> {
        debug!("Connecting to {} as {} (MQTT 3.1.1)", config.addr, config.client_id);

        let stream = TcpStream::connect(&config.addr).await?;
        let (reader, mut writer) = tokio::io::split(stream);

        // Create and send CONNECT packet
        let connect = v4::Connect {
            client_id: config.client_id.clone(),
            keep_alive: config.keep_alive,
            clean_session: config.clean_session,
            username: config.username.clone(),
            password: config.password.clone(),
            will: None,
        };

        let mut buf = vec![0u8; connect.size() + 10];
        let written = connect.write(&mut buf)?;
        writer.write_all(&buf[..written]).await?;
        writer.flush().await?;

        // Read CONNACK
        let mut read_buf = BytesMut::with_capacity(1024);
        let mut reader = reader;

        loop {
            let mut tmp = [0u8; 256];
            let n = reader.read(&mut tmp).await?;
            if n == 0 {
                return Err(Error::ConnectionClosed);
            }
            read_buf.extend_from_slice(&tmp[..n]);

            match v4::Packet::read(&read_buf, config.max_packet_size) {
                Ok((packet, consumed)) => {
                    let _ = read_buf.split_to(consumed);
                    match packet {
                        v4::Packet::ConnAck(connack) => {
                            if connack.code != ConnectReturnCodeV4::Success {
                                return Err(Error::ConnectionRefused(
                                    ConnectionRefusedReason::Other(connack.code as u8)
                                ));
                            }
                            debug!("Connected successfully (v4), session_present={}", connack.session_present);
                            break;
                        }
                        _ => return Err(Error::UnexpectedPacket),
                    }
                }
                Err(mqtt0::Error::Incomplete { .. }) => continue,
                Err(e) => return Err(e.into()),
            }
        }

        let running = Arc::new(AtomicBool::new(true));
        let writer = Arc::new(Mutex::new(writer));

        // Start keepalive task
        if config.auto_keepalive && config.keep_alive > 0 {
            let keepalive_state = KeepaliveState {
                writer: Arc::clone(&writer),
                running: Arc::clone(&running),
                interval: Duration::from_secs((config.keep_alive / 2).max(1) as u64),
                state: ClientState::V4,
            };
            tokio::spawn(keepalive_task(keepalive_state));
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

    async fn connect_v5(config: ClientConfig) -> Result<Self> {
        debug!("Connecting to {} as {} (MQTT 5.0)", config.addr, config.client_id);

        let _stream = TcpStream::connect(&config.addr).await?;

        // Create CONNECT packet using helper (for future use)
        let _packet = v5::create_connect(
            &config.client_id,
            config.username.as_deref(),
            config.password.as_deref(),
            config.keep_alive,
            config.clean_session,
            config.session_expiry,
        );

        // TODO: Implement proper v5 packet writing
        // For now, fallback to v4

        // For now, fallback to v4 behavior
        return Self::connect_v4(ClientConfig {
            protocol_version: ProtocolVersion::V4,
            ..config
        }).await;
    }

    /// Get the client ID.
    pub fn client_id(&self) -> &str {
        &self.client_id
    }

    /// Publish a message (QoS 0).
    pub async fn publish(&self, topic: &str, payload: &[u8]) -> Result<()> {
        self.publish_with_retain(topic, payload, false).await
    }

    /// Publish a message with retain flag.
    pub async fn publish_with_retain(&self, topic: &str, payload: &[u8], retain: bool) -> Result<()> {
        trace!("Publishing to {} ({} bytes)", topic, payload.len());

        let packet = match self.state {
            ClientState::V4 => v4::create_publish(topic, payload, retain),
            ClientState::V5 => {
                // Use v4 for now
                v4::create_publish(topic, payload, retain)
            }
        };

        let mut buf = vec![0u8; packet.size() + 10];
        let written = packet.write(&mut buf)?;

        let mut writer = self.writer.lock().await;
        writer.write_all(&buf[..written]).await?;
        writer.flush().await?;

        Ok(())
    }

    /// Subscribe to topics.
    pub async fn subscribe(&self, topics: &[&str]) -> Result<()> {
        if topics.is_empty() {
            return Ok(());
        }

        let pkid = self.next_pkid.fetch_add(1, Ordering::SeqCst);
        debug!("Subscribing to {:?} with pkid={}", topics, pkid);

        let packet = v4::create_subscribe(pkid, topics);
        let mut buf = vec![0u8; packet.size() + 10];
        let written = packet.write(&mut buf)?;

        {
            let mut writer = self.writer.lock().await;
            writer.write_all(&buf[..written]).await?;
            writer.flush().await?;
        }

        // Wait for SUBACK
        loop {
            let mut reader = self.reader.lock().await;
            let mut read_buf = self.read_buf.lock().await;

            let mut tmp = [0u8; 256];
            let n = reader.read(&mut tmp).await?;
            if n == 0 {
                return Err(Error::ConnectionClosed);
            }
            read_buf.extend_from_slice(&tmp[..n]);

            match v4::Packet::read(&read_buf, self.max_packet_size) {
                Ok((packet, consumed)) => {
                    let _ = read_buf.split_to(consumed);
                    match packet {
                        v4::Packet::SubAck(suback) => {
                            debug!("Received SubAck for pkid={}: {:?}", suback.pkid, suback.return_codes);
                            for code in &suback.return_codes {
                                if *code == v4::SubscribeReasonCode::Failure {
                                    return Err(Error::AclDenied);
                                }
                            }
                            return Ok(());
                        }
                        _ => continue,
                    }
                }
                Err(mqtt0::Error::Incomplete { .. }) => continue,
                Err(e) => return Err(e.into()),
            }
        }
    }

    /// Receive the next message.
    pub async fn recv(&self) -> Result<Message> {
        loop {
            let mut reader = self.reader.lock().await;
            let mut read_buf = self.read_buf.lock().await;

            // Try to parse from existing buffer first
            if !read_buf.is_empty() {
                match v4::Packet::read(&read_buf, self.max_packet_size) {
                    Ok((packet, consumed)) => {
                        let _ = read_buf.split_to(consumed);
                        match packet {
                            v4::Packet::Publish(publish) => {
                                trace!("Received message on {}", publish.topic);
                                return Ok(Message {
                                    topic: publish.topic,
                                    payload: publish.payload,
                                    retain: publish.retain,
                                });
                            }
                            v4::Packet::PingResp => {
                                trace!("Received PingResp");
                                continue;
                            }
                            v4::Packet::Disconnect => {
                                return Err(Error::ConnectionClosed);
                            }
                            _ => continue,
                        }
                    }
                    Err(mqtt0::Error::Incomplete { .. }) => {}
                    Err(e) => return Err(e.into()),
                }
            }

            // Need more data
            let mut tmp = [0u8; 4096];
            let n = reader.read(&mut tmp).await?;
            if n == 0 {
                return Err(Error::ConnectionClosed);
            }
            read_buf.extend_from_slice(&tmp[..n]);
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

        let mut buf = [0u8; 2];
        buf[0] = 0xC0; // PINGREQ
        buf[1] = 0x00;

        let mut writer = self.writer.lock().await;
        writer.write_all(&buf).await?;
        writer.flush().await?;

        Ok(())
    }

    /// Disconnect from the broker.
    pub async fn disconnect(&self) -> Result<()> {
        debug!("Disconnecting");
        self.running.store(false, Ordering::SeqCst);

        let mut buf = [0u8; 2];
        buf[0] = 0xE0; // DISCONNECT
        buf[1] = 0x00;

        let mut writer = self.writer.lock().await;
        writer.write_all(&buf).await?;
        writer.flush().await?;

        Ok(())
    }

    /// Check if the client is still running.
    pub fn is_running(&self) -> bool {
        self.running.load(Ordering::SeqCst)
    }
}

/// Background keepalive task.
async fn keepalive_task(state: KeepaliveState) {
    loop {
        tokio::time::sleep(state.interval).await;

        if !state.running.load(Ordering::SeqCst) {
            trace!("Keepalive task stopping");
            break;
        }

        let mut buf = [0u8; 2];
        buf[0] = 0xC0; // PINGREQ
        buf[1] = 0x00;

        let result = {
            let mut writer = state.writer.lock().await;
            writer.write_all(&buf).await.and_then(|_| {
                // Can't call flush in same expression due to borrow
                Ok(())
            })
        };

        // Flush separately
        if result.is_ok() {
            let mut writer = state.writer.lock().await;
            if let Err(e) = writer.flush().await {
                warn!("Keepalive flush failed: {}", e);
                state.running.store(false, Ordering::SeqCst);
                break;
            }
        }

        if let Err(e) = result {
            warn!("Keepalive ping failed: {}", e);
            state.running.store(false, Ordering::SeqCst);
            break;
        }

        trace!("Keepalive ping sent");
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_client_config() {
        let config = ClientConfig::new("127.0.0.1:1883", "test-client")
            .with_credentials("user", b"pass".to_vec())
            .with_keep_alive(30)
            .with_clean_session(false);

        assert_eq!(config.addr, "127.0.0.1:1883");
        assert_eq!(config.client_id, "test-client");
        assert_eq!(config.username, Some("user".to_string()));
        assert_eq!(config.keep_alive, 30);
        assert!(!config.clean_session);
    }
}
