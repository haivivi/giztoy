//! QoS 0 MQTT broker (mqtt0d).
//!
//! A lightweight MQTT broker that supports both MQTT 3.1.1 (v4) and MQTT 5.0 (v5),
//! with full control over authentication and ACL.
//!
//! The broker automatically detects the protocol version from the CONNECT packet.

use bytes::{Bytes, BytesMut};
use parking_lot::RwLock;
use rumqttc::mqttbytes::QoS;
use std::collections::HashMap;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use tokio::io::{AsyncReadExt, ReadHalf, WriteHalf};
use tokio::net::{TcpListener, TcpStream};
use tokio::sync::mpsc;
use tracing::{debug, info, trace, warn};

use crate::error::{Error, Result};
use crate::protocol::{self, MAX_PACKET_SIZE};
use crate::trie::Trie;
use crate::types::{AllowAll, Authenticator, Handler, Message, ProtocolVersion};

/// Callback type alias.
type Callback = Arc<dyn Fn(&str) + Send + Sync>;

/// Broker configuration.
#[derive(Debug, Clone)]
pub struct BrokerConfig {
    /// Listen address (host:port).
    pub addr: String,
    /// Maximum packet size.
    pub max_packet_size: usize,
}

impl BrokerConfig {
    /// Create a new broker config.
    pub fn new(addr: impl Into<String>) -> Self {
        Self {
            addr: addr.into(),
            max_packet_size: MAX_PACKET_SIZE,
        }
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
            running: Arc::new(AtomicBool::new(false)),
        }
    }
}

/// Client handle for message delivery.
#[derive(Clone)]
struct ClientHandle {
    client_id: String,
    tx: mpsc::Sender<Message>,
}

impl PartialEq for ClientHandle {
    fn eq(&self, other: &Self) -> bool {
        self.client_id == other.client_id
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
    clients: Arc<RwLock<HashMap<String, mpsc::Sender<Message>>>>,
    /// Track subscriptions per client for efficient cleanup on disconnect.
    client_subscriptions: Arc<RwLock<HashMap<String, Vec<String>>>>,
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
                max_packet_size: self.config.max_packet_size,
            };

            tokio::spawn(async move {
                if let Err(e) = broker.handle_connection(stream).await {
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
    clients: Arc<RwLock<HashMap<String, mpsc::Sender<Message>>>>,
    /// Track subscriptions per client for efficient cleanup on disconnect.
    client_subscriptions: Arc<RwLock<HashMap<String, Vec<String>>>>,
    max_packet_size: usize,
}

impl BrokerContext {
    async fn handle_connection(&self, stream: TcpStream) -> Result<()> {
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
            ProtocolVersion::V4 => self.handle_connection_v4(reader, writer, read_buf).await,
            ProtocolVersion::V5 => self.handle_connection_v5(reader, writer, read_buf).await,
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
    ) -> Result<()> {
        use crate::protocol::v4::{ConnectReturnCode, Packet};

        // Read CONNECT packet
        let packet = protocol::v4::read_packet(&mut reader, &mut read_buf, self.max_packet_size).await?;

        let (client_id, keep_alive) = match packet {
            Packet::Connect(connect) => {
                let client_id = connect.client_id.clone();
                let keep_alive = connect.keep_alive;
                let username = connect.login.as_ref().map(|l| l.username.as_str()).unwrap_or("");
                let password = connect.login.as_ref().map(|l| l.password.as_bytes()).unwrap_or(&[]);

                if !self.authenticator.authenticate(&client_id, username, password) {
                    warn!("Authentication failed for {} (v4)", client_id);
                    let connack = protocol::v4::create_connack(false, ConnectReturnCode::NotAuthorized);
                    protocol::v4::write_packet(&mut writer, connack).await?;
                    return Err(Error::AuthenticationFailed);
                }

                debug!("Client {} authenticated (v4), keep_alive={}s", client_id, keep_alive);

                let connack = protocol::v4::create_connack(false, ConnectReturnCode::Success);
                protocol::v4::write_packet(&mut writer, connack).await?;

                (client_id, keep_alive)
            }
            other => {
                return Err(Error::UnexpectedPacket {
                    expected: "Connect".to_string(),
                    got: format!("{:?}", other),
                });
            }
        };

        self.run_client_v4(&client_id, keep_alive, reader, writer, read_buf).await
    }

    async fn handle_connection_v5(
        &self,
        mut reader: ReadHalf<TcpStream>,
        mut writer: WriteHalf<TcpStream>,
        mut read_buf: BytesMut,
    ) -> Result<()> {
        use crate::protocol::v5::{ConnectReturnCode, Packet};

        // Read CONNECT packet
        let packet = protocol::v5::read_packet(&mut reader, &mut read_buf, self.max_packet_size).await?;

        let (client_id, keep_alive) = match packet {
            Packet::Connect(connect, _, login) => {
                let client_id = connect.client_id.clone();
                let keep_alive = connect.keep_alive;
                let username = login.as_ref().map(|l| l.username.as_str()).unwrap_or("");
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

                if !self.authenticator.authenticate(&client_id, username, password) {
                    warn!("Authentication failed for {} (v5)", client_id);
                    let connack = protocol::v5::create_connack(false, ConnectReturnCode::NotAuthorized);
                    protocol::v5::write_packet(&mut writer, connack).await?;
                    return Err(Error::AuthenticationFailed);
                }

                debug!("Client {} authenticated (v5), keep_alive={}s", client_id, keep_alive);

                let connack = protocol::v5::create_connack(false, ConnectReturnCode::Success);
                protocol::v5::write_packet(&mut writer, connack).await?;

                (client_id, keep_alive)
            }
            other => {
                return Err(Error::UnexpectedPacket {
                    expected: "Connect".to_string(),
                    got: format!("{:?}", other),
                });
            }
        };

        self.run_client_v5(&client_id, keep_alive, reader, writer, read_buf).await
    }

    async fn run_client_v4(
        &self,
        client_id: &str,
        keep_alive: u16,
        reader: ReadHalf<TcpStream>,
        writer: WriteHalf<TcpStream>,
        read_buf: BytesMut,
    ) -> Result<()> {
        let (tx, rx) = mpsc::channel::<Message>(100);

        {
            let mut clients = self.clients.write();
            clients.insert(client_id.to_string(), tx.clone());
        }

        if let Some(ref on_connect) = self.on_connect {
            on_connect(client_id);
        }

        info!("Client {} connected (MQTT 3.1.1)", client_id);

        let client_handle = ClientHandle {
            client_id: client_id.to_string(),
            tx,
        };

        let result = self
            .client_loop_v4(client_id, keep_alive, &client_handle, reader, writer, read_buf, rx)
            .await;

        self.cleanup_client(client_id);

        if let Some(ref on_disconnect) = self.on_disconnect {
            on_disconnect(client_id);
        }

        info!("Client {} disconnected", client_id);
        result
    }

    async fn run_client_v5(
        &self,
        client_id: &str,
        keep_alive: u16,
        reader: ReadHalf<TcpStream>,
        writer: WriteHalf<TcpStream>,
        read_buf: BytesMut,
    ) -> Result<()> {
        let (tx, rx) = mpsc::channel::<Message>(100);

        {
            let mut clients = self.clients.write();
            clients.insert(client_id.to_string(), tx.clone());
        }

        if let Some(ref on_connect) = self.on_connect {
            on_connect(client_id);
        }

        info!("Client {} connected (MQTT 5.0)", client_id);

        let client_handle = ClientHandle {
            client_id: client_id.to_string(),
            tx,
        };

        let result = self
            .client_loop_v5(client_id, keep_alive, &client_handle, reader, writer, read_buf, rx)
            .await;

        self.cleanup_client(client_id);

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
                                self.handle_publish_v5(client_id, publish).await?;
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

    async fn handle_publish_v5(
        &self,
        client_id: &str,
        publish: crate::protocol::v5::Publish,
    ) -> Result<()> {
        let topic = String::from_utf8_lossy(&publish.topic).to_string();

        if !self.authenticator.acl(client_id, &topic, true) {
            warn!("ACL denied publish from {} to {}", client_id, topic);
            return Ok(());
        }

        trace!("Client {} published to {} (v5)", client_id, topic);

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
        let subscribers = {
            let subs = self.subscriptions.read();
            subs.get(topic)
        };

        for handle in subscribers {
            if let Err(e) = handle.tx.send(msg.clone()).await {
                warn!("Failed to send to {}: {}", handle.client_id, e);
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

            if !self.authenticator.acl(client_id, topic, false) {
                warn!("ACL denied subscribe from {} to {}", client_id, topic);
                return_codes.push(SubscribeReasonCode::Failure);
                continue;
            }

            // Insert into trie and handle errors
            let insert_result = {
                let subs = self.subscriptions.read();
                subs.insert(topic, client_handle.clone())
            };

            if let Err(e) = insert_result {
                warn!("Failed to insert subscription {} for {}: {}", topic, client_id, e);
                return_codes.push(SubscribeReasonCode::Failure);
                continue;
            }

            // Track subscription for cleanup on disconnect
            {
                let mut client_subs = self.client_subscriptions.write();
                client_subs
                    .entry(client_id.to_string())
                    .or_default()
                    .push(topic.to_string());
            }

            debug!("Client {} subscribed to {} (v4)", client_id, topic);
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

            if !self.authenticator.acl(client_id, topic, false) {
                warn!("ACL denied subscribe from {} to {}", client_id, topic);
                return_codes.push(crate::protocol::v5::SubscribeReasonCode::NotAuthorized);
                continue;
            }

            // Insert into trie and handle errors
            let insert_result = {
                let subs = self.subscriptions.read();
                subs.insert(topic, client_handle.clone())
            };

            if let Err(e) = insert_result {
                warn!("Failed to insert subscription {} for {}: {}", topic, client_id, e);
                return_codes.push(crate::protocol::v5::SubscribeReasonCode::Unspecified);
                continue;
            }

            // Track subscription for cleanup on disconnect
            {
                let mut client_subs = self.client_subscriptions.write();
                client_subs
                    .entry(client_id.to_string())
                    .or_default()
                    .push(topic.to_string());
            }

            debug!("Client {} subscribed to {} (v5)", client_id, topic);
            return_codes.push(crate::protocol::v5::SubscribeReasonCode::Success(crate::protocol::v5::QoS::AtMostOnce));
        }

        return_codes
    }

    fn handle_unsubscribe(&self, client_id: &str, topics: &[String]) {
        let subs = self.subscriptions.write();

        for topic in topics {
            subs.with_mut(|root| {
                root.remove(topic, |h| h.client_id == client_id);
            });
            debug!("Client {} unsubscribed from {}", client_id, topic);
        }

        // Remove from tracking
        let mut client_subs = self.client_subscriptions.write();
        if let Some(subs_list) = client_subs.get_mut(client_id) {
            subs_list.retain(|t| !topics.contains(t));
        }
    }

    fn cleanup_client(&self, client_id: &str) {
        // Remove from clients map
        self.clients.write().remove(client_id);

        // Remove all subscriptions for this client
        let topics = self.client_subscriptions.write().remove(client_id);
        if let Some(topics) = topics {
            let topic_count = topics.len();
            let subs = self.subscriptions.write();
            for topic in &topics {
                subs.with_mut(|root| {
                    root.remove(topic, |h| h.client_id == client_id);
                });
            }
            debug!("Cleaned up {} subscriptions for client {}", topic_count, client_id);
        }
    }
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
