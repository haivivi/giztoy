//! QoS 0 MQTT broker (mqtt0d).
//!
//! A lightweight MQTT broker that supports both MQTT 3.1.1 (v4) and MQTT 5.0 (v5),
//! with full control over authentication and ACL.
//!
//! The broker automatically detects the protocol version from the CONNECT packet.
//!
//! ## Features
//!
//! - **$SYS Events**: Publishes client connect/disconnect events to `$SYS/brokers/{clientid}/connected`
//! - **Shared Subscriptions**: Supports `$share/{group}/{topic}` for load balancing (MQTT 5.0 feature, also works with v4)
//! - **Topic Alias**: MQTT 5.0 topic alias support for reduced bandwidth (with configurable limit)

use bytes::{Bytes, BytesMut};
use parking_lot::RwLock;
use rumqttc::mqttbytes::QoS;
use serde::Serialize;
use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};
use tokio::io::{AsyncReadExt, ReadHalf, WriteHalf};
use tokio::net::{TcpListener, TcpStream};
use tokio::sync::mpsc;
use tracing::{debug, info, trace, warn};

use crate::error::{Error, Result};
use crate::protocol::{self, MAX_PACKET_SIZE};
use crate::trie::Trie;
use crate::types::{AllowAll, Authenticator, Handler, Message, ProtocolVersion};

/// Default maximum topic aliases per client (MQTT 5.0).
pub const DEFAULT_MAX_TOPIC_ALIAS: u16 = 65535;

/// Callback type alias.
type Callback = Arc<dyn Fn(&str) + Send + Sync>;

/// Shared subscription group for load balancing.
#[derive(Clone)]
struct SharedGroup {
    /// Subscribers in this group.
    subscribers: Vec<ClientHandle>,
    /// Round-robin index.
    next_index: Arc<AtomicUsize>,
}

impl SharedGroup {
    fn new() -> Self {
        Self {
            subscribers: Vec::new(),
            next_index: Arc::new(AtomicUsize::new(0)),
        }
    }

    fn add(&mut self, handle: ClientHandle) {
        if !self.subscribers.iter().any(|h| h.client_id == handle.client_id) {
            self.subscribers.push(handle);
        }
    }

    fn remove(&mut self, client_id: &str) {
        self.subscribers.retain(|h| h.client_id.as_ref() != client_id);
    }

    fn is_empty(&self) -> bool {
        self.subscribers.is_empty()
    }

    /// Get next subscriber using round-robin.
    fn next_subscriber(&self) -> Option<&ClientHandle> {
        if self.subscribers.is_empty() {
            return None;
        }
        let idx = self.next_index.fetch_add(1, Ordering::Acquire) % self.subscribers.len();
        self.subscribers.get(idx)
    }
}

/// Entry in shared subscription trie: (group_name, SharedGroup).
#[derive(Clone)]
struct SharedEntry {
    group_name: String,
    group: SharedGroup,
}

/// Broker configuration.
#[derive(Debug, Clone)]
pub struct BrokerConfig {
    /// Listen address (host:port).
    pub addr: String,
    /// Maximum packet size.
    pub max_packet_size: usize,
    /// Enable $SYS event publishing.
    pub sys_events_enabled: bool,
    /// Maximum topic aliases per client (MQTT 5.0).
    pub max_topic_alias: u16,
}

impl BrokerConfig {
    /// Create a new broker config.
    pub fn new(addr: impl Into<String>) -> Self {
        Self {
            addr: addr.into(),
            max_packet_size: MAX_PACKET_SIZE,
            sys_events_enabled: true,
            max_topic_alias: DEFAULT_MAX_TOPIC_ALIAS,
        }
    }

    /// Enable or disable $SYS events.
    pub fn sys_events(mut self, enabled: bool) -> Self {
        self.sys_events_enabled = enabled;
        self
    }

    /// Set maximum topic aliases per client (MQTT 5.0).
    pub fn max_topic_alias(mut self, max: u16) -> Self {
        self.max_topic_alias = max;
        self
    }
}

/// Builder for Broker.
pub struct BrokerBuilder {
    config: BrokerConfig,
    authenticator: Option<Arc<dyn Authenticator>>,
    handler: Option<Arc<dyn Handler>>,
    on_connect: Option<Callback>,
    on_disconnect: Option<Callback>,
}

impl BrokerBuilder {
    /// Create a new broker builder.
    pub fn new(config: BrokerConfig) -> Self {
        Self {
            config,
            authenticator: None,
            handler: None,
            on_connect: None,
            on_disconnect: None,
        }
    }

    /// Set the authenticator.
    pub fn authenticator<A: Authenticator + 'static>(mut self, auth: A) -> Self {
        self.authenticator = Some(Arc::new(auth));
        self
    }

    /// Set the message handler.
    pub fn handler<H: Handler + 'static>(mut self, handler: H) -> Self {
        self.handler = Some(Arc::new(handler));
        self
    }

    /// Set the on_connect callback.
    pub fn on_connect<F: Fn(&str) + Send + Sync + 'static>(mut self, f: F) -> Self {
        self.on_connect = Some(Arc::new(f));
        self
    }

    /// Set the on_disconnect callback.
    pub fn on_disconnect<F: Fn(&str) + Send + Sync + 'static>(mut self, f: F) -> Self {
        self.on_disconnect = Some(Arc::new(f));
        self
    }

    /// Build the broker.
    pub fn build(self) -> Broker {
        Broker {
            config: self.config,
            authenticator: self.authenticator.unwrap_or_else(|| Arc::new(AllowAll)),
            handler: self.handler,
            on_connect: self.on_connect,
            on_disconnect: self.on_disconnect,
            subscriptions: Arc::new(RwLock::new(Trie::new())),
            clients: Arc::new(RwLock::new(HashMap::new())),
            client_subscriptions: Arc::new(RwLock::new(HashMap::new())),
            shared_trie: Arc::new(RwLock::new(Trie::new())),
            running: Arc::new(AtomicBool::new(false)),
        }
    }
}

/// Client handle for message delivery.
/// Uses Arc<str> for client_id and Arc<Sender> for tx to enable:
/// - Cheap cloning (O(1) ref count increment)
/// - Pointer comparison for race condition prevention in cleanup
#[derive(Clone)]
struct ClientHandle {
    client_id: Arc<str>,
    tx: Arc<mpsc::Sender<Message>>,
}

impl PartialEq for ClientHandle {
    fn eq(&self, other: &Self) -> bool {
        self.client_id == other.client_id
    }
}

impl ClientHandle {
    /// Check if this handle's sender is the same instance as another.
    /// Used for race condition prevention during cleanup.
    fn same_sender(&self, other: &Arc<mpsc::Sender<Message>>) -> bool {
        Arc::ptr_eq(&self.tx, other)
    }
}

/// QoS 0 MQTT broker supporting both v4 and v5.
pub struct Broker {
    config: BrokerConfig,
    authenticator: Arc<dyn Authenticator>,
    handler: Option<Arc<dyn Handler>>,
    on_connect: Option<Callback>,
    on_disconnect: Option<Callback>,
    subscriptions: Arc<RwLock<Trie<ClientHandle>>>,
    /// Maps client_id -> Arc<Sender> for pointer comparison during cleanup.
    clients: Arc<RwLock<HashMap<String, Arc<mpsc::Sender<Message>>>>>,
    /// Track subscriptions per client for efficient cleanup on disconnect.
    client_subscriptions: Arc<RwLock<HashMap<String, Vec<String>>>>,
    /// Shared subscriptions trie for O(topic_length) lookup ($share/group/topic).
    shared_trie: Arc<RwLock<Trie<SharedEntry>>>,
    running: Arc<AtomicBool>,
}

impl Broker {
    /// Create a new broker with the given config.
    pub fn new(config: BrokerConfig) -> Self {
        BrokerBuilder::new(config).build()
    }

    /// Create a builder for this broker.
    pub fn builder(config: BrokerConfig) -> BrokerBuilder {
        BrokerBuilder::new(config)
    }

    /// Start the broker.
    pub async fn serve(&self) -> Result<()> {
        if self.running.swap(true, Ordering::SeqCst) {
            return Err(Error::AlreadyRunning);
        }

        let listener = TcpListener::bind(&self.config.addr).await?;
        info!("Broker listening on {}", self.config.addr);

        loop {
            let (stream, addr) = listener.accept().await?;
            debug!("Accepted connection from {}", addr);

            let broker = BrokerContext {
                authenticator: Arc::clone(&self.authenticator),
                handler: self.handler.clone(),
                on_connect: self.on_connect.clone(),
                on_disconnect: self.on_disconnect.clone(),
                subscriptions: Arc::clone(&self.subscriptions),
                clients: Arc::clone(&self.clients),
                client_subscriptions: Arc::clone(&self.client_subscriptions),
                shared_trie: Arc::clone(&self.shared_trie),
                max_packet_size: self.config.max_packet_size,
                sys_events_enabled: self.config.sys_events_enabled,
                max_topic_alias: self.config.max_topic_alias,
            };

            tokio::spawn(async move {
                if let Err(e) = broker.handle_connection(stream, addr).await {
                    debug!("Connection error: {}", e);
                }
            });
        }
    }

    /// Publish a message from the broker.
    pub async fn publish(&self, topic: &str, payload: &[u8]) -> Result<()> {
        let msg = Message::new(topic, Bytes::copy_from_slice(payload));
        self.route_message(&msg).await;
        Ok(())
    }

    async fn route_message(&self, msg: &Message) {
        let subscribers = {
            let subs = self.subscriptions.read();
            subs.get(&msg.topic)
        };

        for handle in subscribers {
            if let Err(e) = handle.tx.send(msg.clone()).await {
                warn!("Failed to send to {}: {}", handle.client_id, e);
            }
        }
    }
}

/// Internal broker context for handling connections.
struct BrokerContext {
    authenticator: Arc<dyn Authenticator>,
    handler: Option<Arc<dyn Handler>>,
    on_connect: Option<Callback>,
    on_disconnect: Option<Callback>,
    subscriptions: Arc<RwLock<Trie<ClientHandle>>>,
    /// Maps client_id -> Arc<Sender> for pointer comparison during cleanup.
    clients: Arc<RwLock<HashMap<String, Arc<mpsc::Sender<Message>>>>>,
    /// Track subscriptions per client for efficient cleanup on disconnect.
    client_subscriptions: Arc<RwLock<HashMap<String, Vec<String>>>>,
    /// Shared subscriptions trie for O(topic_length) lookup.
    shared_trie: Arc<RwLock<Trie<SharedEntry>>>,
    max_packet_size: usize,
    /// Enable $SYS event publishing.
    sys_events_enabled: bool,
    /// Maximum topic aliases per client (MQTT 5.0).
    max_topic_alias: u16,
}

impl BrokerContext {
    async fn handle_connection(&self, stream: TcpStream, addr: SocketAddr) -> Result<()> {
        let (mut reader, writer) = tokio::io::split(stream);
        let mut read_buf = BytesMut::with_capacity(4096);

        // Read initial bytes to detect protocol version
        let mut peek_buf = [0u8; 16];
        let n = reader.read(&mut peek_buf).await?;
        if n == 0 {
            return Err(Error::ConnectionClosed);
        }
        read_buf.extend_from_slice(&peek_buf[..n]);

        // Detect protocol version from CONNECT packet
        let protocol_version = self.detect_protocol_version(&read_buf)?;
        debug!("Detected protocol version: {}", protocol_version);

        match protocol_version {
            ProtocolVersion::V4 => self.handle_connection_v4(reader, writer, read_buf, addr).await,
            ProtocolVersion::V5 => self.handle_connection_v5(reader, writer, read_buf, addr).await,
        }
    }

    /// Detect protocol version from CONNECT packet.
    ///
    /// CONNECT packet structure:
    /// - Fixed header: 1+ bytes (0x10 for CONNECT)
    /// - Remaining length: 1-4 bytes
    /// - Protocol Name Length: 2 bytes
    /// - Protocol Name: "MQTT" (4 bytes)
    /// - Protocol Level: 1 byte (4 for v3.1.1, 5 for v5.0)
    fn detect_protocol_version(&self, buf: &[u8]) -> Result<ProtocolVersion> {
        if buf.len() < 2 {
            return Err(Error::Protocol("insufficient data".to_string()));
        }

        // Check if it's a CONNECT packet (0x10)
        if buf[0] != 0x10 {
            return Err(Error::Protocol("expected CONNECT packet".to_string()));
        }

        // Parse remaining length (variable length encoding)
        let mut multiplier = 1usize;
        let mut header_len = 1usize;

        for &byte in buf.iter().skip(1) {
            header_len += 1;
            if byte & 0x80 == 0 {
                break;
            }
            multiplier *= 128;
            if multiplier > 128 * 128 * 128 * 128 {
                return Err(Error::Protocol("malformed remaining length".to_string()));
            }
        }

        // Protocol level offset: header_len + 2 (name length) + 4 (name "MQTT")
        let protocol_level_offset = header_len + 2 + 4;
        if buf.len() <= protocol_level_offset {
            // Not enough data yet, assume v4 (will be corrected when full packet arrives)
            return Ok(ProtocolVersion::V4);
        }

        let protocol_level = buf[protocol_level_offset];
        match protocol_level {
            4 => Ok(ProtocolVersion::V4),
            5 => Ok(ProtocolVersion::V5),
            _ => Err(Error::Protocol(format!(
                "unsupported protocol level: {}",
                protocol_level
            ))),
        }
    }

    async fn handle_connection_v4(
        &self,
        mut reader: ReadHalf<TcpStream>,
        mut writer: WriteHalf<TcpStream>,
        mut read_buf: BytesMut,
        addr: SocketAddr,
    ) -> Result<()> {
        use crate::protocol::v4::{ConnectReturnCode, Packet};

        // Read CONNECT packet
        let packet = protocol::v4::read_packet(&mut reader, &mut read_buf, self.max_packet_size).await?;

        let (client_id, keep_alive, username) = match packet {
            Packet::Connect(connect) => {
                let client_id = connect.client_id.clone();
                let keep_alive = connect.keep_alive;
                let username = connect.login.as_ref().map(|l| l.username.clone()).unwrap_or_default();
                let password = connect.login.as_ref().map(|l| l.password.as_bytes()).unwrap_or(&[]);

                if !self.authenticator.authenticate(&client_id, &username, password) {
                    warn!("Authentication failed for {} (v4)", client_id);
                    let connack = protocol::v4::create_connack(false, ConnectReturnCode::NotAuthorized);
                    protocol::v4::write_packet(&mut writer, connack).await?;
                    return Err(Error::AuthenticationFailed);
                }

                debug!("Client {} authenticated (v4), keep_alive={}s", client_id, keep_alive);

                let connack = protocol::v4::create_connack(false, ConnectReturnCode::Success);
                protocol::v4::write_packet(&mut writer, connack).await?;

                (client_id, keep_alive, username)
            }
            other => {
                return Err(Error::UnexpectedPacket {
                    expected: "Connect".to_string(),
                    got: format!("{:?}", other),
                });
            }
        };

        self.run_client_v4(&client_id, &username, keep_alive, addr, ProtocolVersion::V4, reader, writer, read_buf).await
    }

    async fn handle_connection_v5(
        &self,
        mut reader: ReadHalf<TcpStream>,
        mut writer: WriteHalf<TcpStream>,
        mut read_buf: BytesMut,
        addr: SocketAddr,
    ) -> Result<()> {
        use crate::protocol::v5::{ConnectReturnCode, Packet};

        // Read CONNECT packet
        let packet = protocol::v5::read_packet(&mut reader, &mut read_buf, self.max_packet_size).await?;

        let (client_id, keep_alive, username) = match packet {
            Packet::Connect(connect, _, login) => {
                let client_id = connect.client_id.clone();
                let keep_alive = connect.keep_alive;
                let username = login.as_ref().map(|l| l.username.clone()).unwrap_or_default();
                let password = login.as_ref().map(|l| l.password.as_bytes()).unwrap_or(&[]);

                // Log session expiry if present
                if let Some(ref props) = connect.properties {
                    if let Some(expiry) = props.session_expiry_interval {
                        debug!(
                            "Client {} requesting session_expiry={}s",
                            client_id, expiry
                        );
                    }
                }

                if !self.authenticator.authenticate(&client_id, &username, password) {
                    warn!("Authentication failed for {} (v5)", client_id);
                    let connack = protocol::v5::create_connack(false, ConnectReturnCode::NotAuthorized);
                    protocol::v5::write_packet(&mut writer, connack).await?;
                    return Err(Error::AuthenticationFailed);
                }

                debug!("Client {} authenticated (v5), keep_alive={}s", client_id, keep_alive);

                let connack = protocol::v5::create_connack(false, ConnectReturnCode::Success);
                protocol::v5::write_packet(&mut writer, connack).await?;

                (client_id, keep_alive, username)
            }
            other => {
                return Err(Error::UnexpectedPacket {
                    expected: "Connect".to_string(),
                    got: format!("{:?}", other),
                });
            }
        };

        self.run_client_v5(&client_id, &username, keep_alive, addr, ProtocolVersion::V5, reader, writer, read_buf).await
    }

    async fn run_client_v4(
        &self,
        client_id: &str,
        username: &str,
        keep_alive: u16,
        addr: SocketAddr,
        proto_ver: ProtocolVersion,
        reader: ReadHalf<TcpStream>,
        writer: WriteHalf<TcpStream>,
        read_buf: BytesMut,
    ) -> Result<()> {
        let (tx, rx) = mpsc::channel::<Message>(100);
        let tx = Arc::new(tx);

        {
            let mut clients = self.clients.write();
            clients.insert(client_id.to_string(), Arc::clone(&tx));
        }

        if let Some(ref on_connect) = self.on_connect {
            on_connect(client_id);
        }

        // Publish $SYS connected event
        self.publish_sys_connected(client_id, username, addr, proto_ver, keep_alive).await;

        info!("Client {} connected (MQTT 3.1.1)", client_id);

        let client_handle = ClientHandle {
            client_id: Arc::from(client_id),
            tx: Arc::clone(&tx),
        };

        let result = self
            .client_loop_v4(client_id, keep_alive, &client_handle, reader, writer, read_buf, rx)
            .await;

        // Pass tx for pointer comparison to prevent race conditions
        self.cleanup_client(client_id, username, &tx).await;

        if let Some(ref on_disconnect) = self.on_disconnect {
            on_disconnect(client_id);
        }

        info!("Client {} disconnected", client_id);
        result
    }

    async fn run_client_v5(
        &self,
        client_id: &str,
        username: &str,
        keep_alive: u16,
        addr: SocketAddr,
        proto_ver: ProtocolVersion,
        reader: ReadHalf<TcpStream>,
        writer: WriteHalf<TcpStream>,
        read_buf: BytesMut,
    ) -> Result<()> {
        let (tx, rx) = mpsc::channel::<Message>(100);
        let tx = Arc::new(tx);

        {
            let mut clients = self.clients.write();
            clients.insert(client_id.to_string(), Arc::clone(&tx));
        }

        if let Some(ref on_connect) = self.on_connect {
            on_connect(client_id);
        }

        // Publish $SYS connected event
        self.publish_sys_connected(client_id, username, addr, proto_ver, keep_alive).await;

        info!("Client {} connected (MQTT 5.0)", client_id);

        let client_handle = ClientHandle {
            client_id: Arc::from(client_id),
            tx: Arc::clone(&tx),
        };

        let result = self
            .client_loop_v5(client_id, keep_alive, &client_handle, reader, writer, read_buf, rx)
            .await;

        // Pass tx for pointer comparison to prevent race conditions
        self.cleanup_client(client_id, username, &tx).await;

        if let Some(ref on_disconnect) = self.on_disconnect {
            on_disconnect(client_id);
        }

        info!("Client {} disconnected", client_id);
        result
    }

    async fn client_loop_v4(
        &self,
        client_id: &str,
        keep_alive: u16,
        client_handle: &ClientHandle,
        mut reader: ReadHalf<TcpStream>,
        mut writer: WriteHalf<TcpStream>,
        mut read_buf: BytesMut,
        mut rx: mpsc::Receiver<Message>,
    ) -> Result<()> {
        use crate::protocol::v4::Packet;
        use std::time::Duration;

        // MQTT spec: disconnect if no packet received within 1.5 × keep_alive
        // If keep_alive is 0, no timeout (client disabled keep-alive)
        let timeout_duration = if keep_alive > 0 {
            Some(Duration::from_secs((keep_alive as u64 * 3) / 2))
        } else {
            None
        };

        loop {
            let select_future = async {
                tokio::select! {
                    msg = rx.recv() => {
                        match msg {
                            Some(msg) => {
                                let publish = protocol::v4::create_publish(&msg.topic, &msg.payload, msg.retain);
                                protocol::v4::write_packet(&mut writer, publish).await?;
                                Ok::<bool, Error>(false) // continue loop
                            }
                            None => Ok::<bool, Error>(true), // exit loop
                        }
                    }

                    result = protocol::v4::read_packet(&mut reader, &mut read_buf, self.max_packet_size) => {
                        let packet = result?;

                        match packet {
                            Packet::Publish(publish) => {
                                self.handle_publish_v4(client_id, publish).await?;
                            }
                            Packet::Subscribe(subscribe) => {
                                let return_codes = self.handle_subscribe_v4(client_id, client_handle, &subscribe.filters).await;
                                let suback = protocol::v4::create_suback(subscribe.pkid, return_codes);
                                protocol::v4::write_packet(&mut writer, suback).await?;
                            }
                            Packet::Unsubscribe(unsubscribe) => {
                                self.handle_unsubscribe(client_id, &unsubscribe.topics);
                                let unsuback = protocol::v4::create_unsuback(unsubscribe.pkid);
                                protocol::v4::write_packet(&mut writer, unsuback).await?;
                            }
                            Packet::PingReq => {
                                protocol::v4::write_packet(&mut writer, protocol::v4::create_pingresp()).await?;
                            }
                            Packet::Disconnect => {
                                return Ok::<bool, Error>(true); // exit loop
                            }
                            _ => {
                                trace!("Ignoring packet from {}: {:?}", client_id, packet);
                            }
                        }
                        Ok::<bool, Error>(false) // continue loop
                    }
                }
            };

            let should_exit = if let Some(timeout) = timeout_duration {
                match tokio::time::timeout(timeout, select_future).await {
                    Ok(result) => result?,
                    Err(_) => {
                        warn!("Client {} keep-alive timeout ({}s), disconnecting", client_id, keep_alive);
                        return Err(Error::Protocol("keep-alive timeout".to_string()));
                    }
                }
            } else {
                select_future.await?
            };

            if should_exit {
                return Ok(());
            }
        }
    }

    async fn client_loop_v5(
        &self,
        client_id: &str,
        keep_alive: u16,
        client_handle: &ClientHandle,
        mut reader: ReadHalf<TcpStream>,
        mut writer: WriteHalf<TcpStream>,
        mut read_buf: BytesMut,
        mut rx: mpsc::Receiver<Message>,
    ) -> Result<()> {
        use crate::protocol::v5::Packet;
        use std::time::Duration;

        // MQTT spec: disconnect if no packet received within 1.5 × keep_alive
        // If keep_alive is 0, no timeout (client disabled keep-alive)
        let timeout_duration = if keep_alive > 0 {
            Some(Duration::from_secs((keep_alive as u64 * 3) / 2))
        } else {
            None
        };

        // Topic Alias mapping for this client (MQTT 5.0 feature)
        // Maps alias (u16) -> topic name (String)
        let mut topic_aliases: HashMap<u16, String> = HashMap::new();

        loop {
            let select_future = async {
                tokio::select! {
                    msg = rx.recv() => {
                        match msg {
                            Some(msg) => {
                                let publish = protocol::v5::create_publish(&msg.topic, &msg.payload, msg.retain);
                                protocol::v5::write_packet(&mut writer, publish).await?;
                                Ok::<bool, Error>(false) // continue loop
                            }
                            None => Ok::<bool, Error>(true), // exit loop
                        }
                    }

                    result = protocol::v5::read_packet(&mut reader, &mut read_buf, self.max_packet_size) => {
                        let packet = result?;

                        match packet {
                            Packet::Publish(publish) => {
                                self.handle_publish_v5_with_alias(client_id, publish, &mut topic_aliases).await?;
                            }
                            Packet::Subscribe(subscribe) => {
                                let return_codes = self.handle_subscribe_v5(client_id, client_handle, &subscribe.filters).await;
                                let suback = protocol::v5::create_suback(subscribe.pkid, return_codes);
                                protocol::v5::write_packet(&mut writer, suback).await?;
                            }
                            Packet::Unsubscribe(unsubscribe) => {
                                let topics: Vec<String> = unsubscribe.filters.clone();
                                self.handle_unsubscribe(client_id, &topics);
                                let unsuback = protocol::v5::create_unsuback(unsubscribe.pkid);
                                protocol::v5::write_packet(&mut writer, unsuback).await?;
                            }
                            Packet::PingReq(_) => {
                                protocol::v5::write_packet(&mut writer, protocol::v5::create_pingresp()).await?;
                            }
                            Packet::Disconnect(_) => {
                                return Ok::<bool, Error>(true); // exit loop
                            }
                            _ => {
                                trace!("Ignoring packet from {}: {:?}", client_id, packet);
                            }
                        }
                        Ok::<bool, Error>(false) // continue loop
                    }
                }
            };

            let should_exit = if let Some(timeout) = timeout_duration {
                match tokio::time::timeout(timeout, select_future).await {
                    Ok(result) => result?,
                    Err(_) => {
                        warn!("Client {} keep-alive timeout ({}s), disconnecting", client_id, keep_alive);
                        return Err(Error::Protocol("keep-alive timeout".to_string()));
                    }
                }
            } else {
                select_future.await?
            };

            if should_exit {
                return Ok(());
            }
        }
    }

    async fn handle_publish_v4(
        &self,
        client_id: &str,
        publish: crate::protocol::v4::Publish,
    ) -> Result<()> {
        let topic = publish.topic.to_string();

        if !self.authenticator.acl(client_id, &topic, true) {
            warn!("ACL denied publish from {} to {}", client_id, topic);
            return Ok(());
        }

        trace!("Client {} published to {} (v4)", client_id, topic);

        let msg = Message {
            topic: topic.clone(),
            payload: publish.payload,
            retain: publish.retain,
        };

        if let Some(ref handler) = self.handler {
            handler.handle(client_id, &msg);
        }

        self.route_to_subscribers(&topic, &msg).await;
        Ok(())
    }

    /// Handle MQTT v5 PUBLISH with Topic Alias support.
    ///
    /// Topic Alias (MQTT 5.0 feature):
    /// - If topic is non-empty and alias is present: update alias mapping, use topic
    /// - If topic is empty and alias is present: use topic from alias mapping
    /// - If topic is non-empty and alias is absent: use topic directly
    ///
    /// DoS Protection:
    /// - Enforces max_topic_alias limit to prevent memory exhaustion
    async fn handle_publish_v5_with_alias(
        &self,
        client_id: &str,
        publish: crate::protocol::v5::Publish,
        topic_aliases: &mut HashMap<u16, String>,
    ) -> Result<()> {
        let topic_from_packet = String::from_utf8_lossy(&publish.topic).to_string();

        // Extract topic alias from properties
        let topic_alias = publish
            .properties
            .as_ref()
            .and_then(|p| p.topic_alias);

        // Resolve the actual topic
        let topic = if let Some(alias) = topic_alias {
            // DoS protection: enforce alias limit
            if alias > self.max_topic_alias {
                warn!(
                    "Client {} used topic alias {} exceeding limit {}, ignoring publish",
                    client_id, alias, self.max_topic_alias
                );
                return Ok(());
            }

            if !topic_from_packet.is_empty() {
                // Topic is provided with alias - update the mapping
                topic_aliases.insert(alias, topic_from_packet.clone());
                trace!(
                    "Client {} set topic alias {} = '{}'",
                    client_id, alias, topic_from_packet
                );
                topic_from_packet
            } else {
                // Topic is empty - look up from alias mapping
                match topic_aliases.get(&alias) {
                    Some(resolved) => {
                        trace!(
                            "Client {} resolved topic alias {} to '{}'",
                            client_id, alias, resolved
                        );
                        resolved.clone()
                    }
                    None => {
                        warn!(
                            "Client {} used unknown topic alias {}, ignoring publish",
                            client_id, alias
                        );
                        return Ok(());
                    }
                }
            }
        } else {
            // No alias - use topic directly
            if topic_from_packet.is_empty() {
                warn!("Client {} sent PUBLISH with empty topic and no alias", client_id);
                return Ok(());
            }
            topic_from_packet
        };

        if !self.authenticator.acl(client_id, &topic, true) {
            warn!("ACL denied publish from {} to {}", client_id, topic);
            return Ok(());
        }

        trace!("Client {} published to {} (v5, alias={:?})", client_id, topic, topic_alias);

        let msg = Message {
            topic: topic.clone(),
            payload: publish.payload,
            retain: publish.retain,
        };

        if let Some(ref handler) = self.handler {
            handler.handle(client_id, &msg);
        }

        self.route_to_subscribers(&topic, &msg).await;
        Ok(())
    }

    async fn route_to_subscribers(&self, topic: &str, msg: &Message) {
        // Route to normal subscribers
        let subscribers = {
            let subs = self.subscriptions.read();
            subs.get(topic)
        };

        for handle in subscribers {
            if let Err(e) = handle.tx.send(msg.clone()).await {
                warn!("Failed to send to {}: {}", handle.client_id, e);
            }
        }

        // Route to shared subscription groups (round-robin) using Trie lookup - O(topic_length)
        let shared_subscribers: Vec<(String, Option<ClientHandle>)> = {
            let shared_trie = self.shared_trie.read();
            shared_trie
                .get(topic)
                .into_iter()
                .map(|entry| (entry.group_name.clone(), entry.group.next_subscriber().cloned()))
                .collect()
        };

        for (group_name, subscriber) in shared_subscribers {
            if let Some(handle) = subscriber {
                if let Err(e) = handle.tx.send(msg.clone()).await {
                    warn!(
                        "Failed to send to shared subscriber {} (group {}): {}",
                        handle.client_id, group_name, e
                    );
                }
            }
        }
    }


    async fn handle_subscribe_v4(
        &self,
        client_id: &str,
        client_handle: &ClientHandle,
        filters: &[crate::protocol::v4::SubscribeFilter],
    ) -> Vec<crate::protocol::v4::SubscribeReasonCode> {
        use crate::protocol::v4::SubscribeReasonCode;

        let mut return_codes = Vec::with_capacity(filters.len());

        for filter in filters {
            let topic = &filter.path;

            // For shared subscriptions, check ACL on the actual topic
            let acl_topic = if let Some((_, actual_topic)) = parse_shared_topic(topic) {
                actual_topic
            } else {
                topic.as_str()
            };

            if !self.authenticator.acl(client_id, acl_topic, false) {
                warn!("ACL denied subscribe from {} to {}", client_id, topic);
                return_codes.push(SubscribeReasonCode::Failure);
                continue;
            }

            // Handle shared subscriptions
            if let Some((group, actual_topic)) = parse_shared_topic(topic) {
                // Use Trie for O(topic_length) lookup
                let shared_trie = self.shared_trie.write();
                shared_trie.with_mut(|root| {
                    let _ = root.set(actual_topic, |node| {
                        // Find existing group or create new one
                        let values = node.values_mut();
                        if let Some(entry) = values.iter_mut().find(|e| e.group_name == group) {
                            entry.group.add(client_handle.clone());
                        } else {
                            let mut new_group = SharedGroup::new();
                            new_group.add(client_handle.clone());
                            values.push(SharedEntry {
                                group_name: group.to_string(),
                                group: new_group,
                            });
                        }
                    });
                });
                debug!(
                    "Client {} subscribed to shared group '{}' topic '{}' (v4)",
                    client_id, group, actual_topic
                );
            } else {
                // Normal subscription - insert into trie
            let insert_result = {
                let subs = self.subscriptions.read();
                subs.insert(topic, client_handle.clone())
            };

            if let Err(e) = insert_result {
                warn!("Failed to insert subscription {} for {}: {}", topic, client_id, e);
                return_codes.push(SubscribeReasonCode::Failure);
                continue;
                }
                debug!("Client {} subscribed to {} (v4)", client_id, topic);
            }

            // Track subscription for cleanup on disconnect
            {
                let mut client_subs = self.client_subscriptions.write();
                client_subs
                    .entry(client_id.to_string())
                    .or_default()
                    .push(topic.to_string());
            }

            return_codes.push(SubscribeReasonCode::Success(QoS::AtMostOnce));
        }

        return_codes
    }

    async fn handle_subscribe_v5(
        &self,
        client_id: &str,
        client_handle: &ClientHandle,
        filters: &[crate::protocol::v5::Filter],
    ) -> Vec<crate::protocol::v5::SubscribeReasonCode> {
        let mut return_codes = Vec::with_capacity(filters.len());

        for filter in filters {
            let topic = &filter.path;

            // For shared subscriptions, check ACL on the actual topic
            let acl_topic = if let Some((_, actual_topic)) = parse_shared_topic(topic) {
                actual_topic
            } else {
                topic.as_str()
            };

            if !self.authenticator.acl(client_id, acl_topic, false) {
                warn!("ACL denied subscribe from {} to {}", client_id, topic);
                return_codes.push(crate::protocol::v5::SubscribeReasonCode::NotAuthorized);
                continue;
            }

            // Handle shared subscriptions
            if let Some((group, actual_topic)) = parse_shared_topic(topic) {
                // Use Trie for O(topic_length) lookup
                let shared_trie = self.shared_trie.write();
                shared_trie.with_mut(|root| {
                    let _ = root.set(actual_topic, |node| {
                        // Find existing group or create new one
                        let values = node.values_mut();
                        if let Some(entry) = values.iter_mut().find(|e| e.group_name == group) {
                            entry.group.add(client_handle.clone());
                        } else {
                            let mut new_group = SharedGroup::new();
                            new_group.add(client_handle.clone());
                            values.push(SharedEntry {
                                group_name: group.to_string(),
                                group: new_group,
                            });
                        }
                    });
                });
                debug!(
                    "Client {} subscribed to shared group '{}' topic '{}' (v5)",
                    client_id, group, actual_topic
                );
            } else {
                // Normal subscription - insert into trie
            let insert_result = {
                let subs = self.subscriptions.read();
                subs.insert(topic, client_handle.clone())
            };

            if let Err(e) = insert_result {
                warn!("Failed to insert subscription {} for {}: {}", topic, client_id, e);
                return_codes.push(crate::protocol::v5::SubscribeReasonCode::Unspecified);
                continue;
                }
                debug!("Client {} subscribed to {} (v5)", client_id, topic);
            }

            // Track subscription for cleanup on disconnect
            {
                let mut client_subs = self.client_subscriptions.write();
                client_subs
                    .entry(client_id.to_string())
                    .or_default()
                    .push(topic.to_string());
            }

            return_codes.push(crate::protocol::v5::SubscribeReasonCode::Success(crate::protocol::v5::QoS::AtMostOnce));
        }

        return_codes
    }

    fn handle_unsubscribe(&self, client_id: &str, topics: &[String]) {
        for topic in topics {
            // Handle shared subscriptions
            if let Some((group, actual_topic)) = parse_shared_topic(topic) {
                // Use Trie for unsubscribe
                let shared_trie = self.shared_trie.write();
                shared_trie.with_mut(|root| {
                    let _ = root.set(actual_topic, |node| {
                        let values = node.values_mut();
                        if let Some(entry) = values.iter_mut().find(|e| e.group_name == group) {
                            entry.group.remove(client_id);
                        }
                        // Remove empty groups
                        values.retain(|e| !e.group.is_empty());
                    });
                });
                debug!(
                    "Client {} unsubscribed from shared group '{}' topic '{}'",
                    client_id, group, actual_topic
                );
            } else {
                // Normal subscription
                let subs = self.subscriptions.write();
            subs.with_mut(|root| {
                root.remove(topic, |h| h.client_id.as_ref() == client_id);
            });
            debug!("Client {} unsubscribed from {}", client_id, topic);
            }
        }

        // Remove from tracking
        let mut client_subs = self.client_subscriptions.write();
        if let Some(subs_list) = client_subs.get_mut(client_id) {
            subs_list.retain(|t| !topics.contains(t));
        }
    }

    /// Cleanup client state on disconnect.
    /// The `tx` parameter is used for pointer comparison to prevent race conditions
    /// where a new client with the same client_id replaces an old one before cleanup.
    async fn cleanup_client(&self, client_id: &str, username: &str, tx: &Arc<mpsc::Sender<Message>>) {
        // Remove from clients map only if the current sender matches (pointer comparison)
        // This prevents removing a new client that connected with the same client_id
        {
            let mut clients = self.clients.write();
            if let Some(current) = clients.get(client_id) {
                if Arc::ptr_eq(current, tx) {
                    clients.remove(client_id);
                }
            }
        }

        // Remove all subscriptions for this client
        let topics = self.client_subscriptions.write().remove(client_id);
        if let Some(topics) = topics {
            let topic_count = topics.len();
            let subs = self.subscriptions.write();
            for topic in &topics {
                // Check if it's a shared subscription
                if let Some((group, actual_topic)) = parse_shared_topic(topic) {
                    // Use Trie for cleanup - use pointer comparison
                    let shared_trie = self.shared_trie.write();
                    shared_trie.with_mut(|root| {
                        let _ = root.set(actual_topic, |node| {
                            let values = node.values_mut();
                            if let Some(entry) = values.iter_mut().find(|e| e.group_name == group) {
                                // Use pointer comparison via same_sender
                                entry.group.subscribers.retain(|h| !h.same_sender(tx));
                            }
                            // Remove empty groups
                            values.retain(|e| !e.group.is_empty());
                        });
                    });
                } else {
                    // Use pointer comparison for normal subscriptions
                    subs.with_mut(|root| {
                        root.remove(topic, |h| h.same_sender(tx));
                    });
                }
            }
            debug!("Cleaned up {} subscriptions for client {}", topic_count, client_id);
        }

        // Publish $SYS disconnected event
        self.publish_sys_disconnected(client_id, username).await;
    }

    /// Publish $SYS client connected event.
    /// Topic: $SYS/brokers/{clientid}/connected
    /// Format compatible with EMQX: https://docs.emqx.com/en/emqx/latest/observability/mqtt-system-topics.html
    async fn publish_sys_connected(
        &self,
        client_id: &str,
        username: &str,
        addr: SocketAddr,
        proto_ver: ProtocolVersion,
        keepalive: u16,
    ) {
        if !self.sys_events_enabled {
            return;
        }

        let topic = format!("$SYS/brokers/{}/connected", client_id);

        let connected_at = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|d| d.as_secs())
            .unwrap_or(0);

        let proto_ver_num: u8 = match proto_ver {
            ProtocolVersion::V4 => 4,
            ProtocolVersion::V5 => 5,
        };

        // Use serde_json for safe JSON serialization (prevents JSON injection)
        let event = SysConnectedEvent {
            clientid: client_id,
            username,
            ipaddress: addr.ip().to_string(),
            proto_ver: proto_ver_num,
            keepalive,
            connected_at,
        };
        let payload = match serde_json::to_string(&event) {
            Ok(s) => s,
            Err(e) => {
                warn!("Failed to serialize $SYS connected event for {}: {}", client_id, e);
                return;
            }
        };

        let msg = Message::new(topic.clone(), Bytes::from(payload));
        self.route_to_subscribers(&topic, &msg).await;
    }

    /// Publish $SYS client disconnected event.
    /// Topic: $SYS/brokers/{clientid}/disconnected
    /// Format compatible with EMQX: https://docs.emqx.com/en/emqx/latest/observability/mqtt-system-topics.html
    async fn publish_sys_disconnected(&self, client_id: &str, username: &str) {
        if !self.sys_events_enabled {
            return;
        }

        let topic = format!("$SYS/brokers/{}/disconnected", client_id);

        let disconnected_at = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|d| d.as_secs())
            .unwrap_or(0);

        // Use serde_json for safe JSON serialization (prevents JSON injection)
        let event = SysDisconnectedEvent {
            clientid: client_id,
            username,
            reason: "normal",
            disconnected_at,
        };
        let payload = match serde_json::to_string(&event) {
            Ok(s) => s,
            Err(e) => {
                warn!("Failed to serialize $SYS disconnected event for {}: {}", client_id, e);
                return;
            }
        };

        let msg = Message::new(topic.clone(), Bytes::from(payload));
        self.route_to_subscribers(&topic, &msg).await;
    }
}

/// $SYS client connected event payload.
#[derive(Serialize)]
struct SysConnectedEvent<'a> {
    clientid: &'a str,
    username: &'a str,
    ipaddress: String,
    proto_ver: u8,
    keepalive: u16,
    connected_at: u64,
}

/// $SYS client disconnected event payload.
#[derive(Serialize)]
struct SysDisconnectedEvent<'a> {
    clientid: &'a str,
    username: &'a str,
    reason: &'a str,
    disconnected_at: u64,
}

/// Parse shared subscription topic.
/// Format: $share/{group}/{topic}
/// Returns: Some((group, actual_topic)) or None
pub fn parse_shared_topic(topic: &str) -> Option<(&str, &str)> {
    if !topic.starts_with("$share/") {
        return None;
    }
    let rest = &topic[7..]; // Skip "$share/"
    let slash_pos = rest.find('/')?;
    let group = &rest[..slash_pos];
    let actual_topic = &rest[slash_pos + 1..];
    if group.is_empty() || actual_topic.is_empty() {
        return None;
    }
    Some((group, actual_topic))
}

/// Check if a subscription pattern matches a topic.
/// Supports MQTT wildcards: + (single level) and # (multi level).
///
/// MQTT spec compliance (Section 4.7.2):
/// - Wildcards (#, +) at the beginning of a pattern MUST NOT match topics starting with $
/// - To receive $SYS messages, clients must explicitly subscribe to patterns starting with $SYS
pub fn topic_matches(pattern: &str, topic: &str) -> bool {
    let pattern_parts: Vec<&str> = pattern.split('/').collect();
    let topic_parts: Vec<&str> = topic.split('/').collect();

    // MQTT spec: wildcards should not match $ topics unless pattern also starts with $
    // e.g., "#" should not match "$SYS/foo", but "$SYS/#" should match "$SYS/foo"
    if !topic_parts.is_empty() && topic_parts[0].starts_with('$') {
        if pattern_parts.is_empty() {
            return false;
        }
        // If topic starts with $ but pattern's first part is a wildcard, no match
        let first_pattern = pattern_parts[0];
        if first_pattern == "#" || first_pattern == "+" {
            return false;
        }
    }

    let mut p_idx = 0;
    let mut t_idx = 0;

    while p_idx < pattern_parts.len() {
        let p = pattern_parts[p_idx];

        if p == "#" {
            // # matches everything remaining
            return true;
        }

        if t_idx >= topic_parts.len() {
            return false;
        }

        if p == "+" {
            // + matches exactly one level
            p_idx += 1;
            t_idx += 1;
        } else if p == topic_parts[t_idx] {
            // Exact match
            p_idx += 1;
            t_idx += 1;
        } else {
            return false;
        }
    }

    // Both should be exhausted for exact match
    p_idx == pattern_parts.len() && t_idx == topic_parts.len()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_broker_config() {
        let config = BrokerConfig::new("127.0.0.1:1883");
        assert_eq!(config.addr, "127.0.0.1:1883");
    }

    #[test]
    fn test_broker_builder() {
        struct TestAuth;
        impl Authenticator for TestAuth {
            fn authenticate(&self, _: &str, _: &str, _: &[u8]) -> bool {
                true
            }
            fn acl(&self, _: &str, _: &str, _: bool) -> bool {
                true
            }
        }

        let broker = Broker::builder(BrokerConfig::new("127.0.0.1:1883"))
            .authenticator(TestAuth)
            .on_connect(|id| println!("Connected: {}", id))
            .on_disconnect(|id| println!("Disconnected: {}", id))
            .build();

        assert_eq!(broker.config.addr, "127.0.0.1:1883");
    }
}
