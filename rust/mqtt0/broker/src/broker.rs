//! MQTT broker implementation.

use bytes::BytesMut;
use parking_lot::RwLock;
use std::collections::HashMap;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::Duration;
use tokio::io::{AsyncReadExt, AsyncWriteExt, ReadHalf, WriteHalf};
use tokio::net::{TcpListener, TcpStream};
use tokio::sync::mpsc;
use tracing::{debug, info, trace, warn};

use mqtt0::protocol::{v4, MAX_PACKET_SIZE};
use mqtt0::types::{ConnectReturnCodeV4, Message, QoS};

use crate::error::{Error, Result};
use crate::trie::Trie;
use crate::types::{AllowAll, Authenticator, Handler};

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
    client_id: Arc<str>,
    tx: mpsc::Sender<Message>,
}

impl PartialEq for ClientHandle {
    fn eq(&self, other: &Self) -> bool {
        self.client_id == other.client_id
    }
}

/// QoS 0 MQTT broker.
pub struct Broker {
    config: BrokerConfig,
    authenticator: Arc<dyn Authenticator>,
    handler: Option<Arc<dyn Handler>>,
    on_connect: Option<Callback>,
    on_disconnect: Option<Callback>,
    subscriptions: Arc<RwLock<Trie<ClientHandle>>>,
    clients: Arc<RwLock<HashMap<String, mpsc::Sender<Message>>>>,
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

            let ctx = BrokerContext {
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
                if let Err(e) = ctx.handle_connection(stream).await {
                    debug!("Connection error: {}", e);
                }
            });
        }
    }

    /// Publish a message from the broker.
    pub async fn publish(&self, topic: &str, payload: &[u8]) -> Result<()> {
        let msg = Message::new(topic, bytes::Bytes::copy_from_slice(payload));
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

        // For now, assume MQTT 3.1.1 (v4)
        self.handle_connection_v4(reader, writer, read_buf).await
    }

    async fn handle_connection_v4(
        &self,
        mut reader: ReadHalf<TcpStream>,
        mut writer: WriteHalf<TcpStream>,
        mut read_buf: BytesMut,
    ) -> Result<()> {
        // Read more data if needed for CONNECT packet
        loop {
            match v4::Packet::read(&read_buf, self.max_packet_size) {
                Ok((packet, consumed)) => {
                    let _ = read_buf.split_to(consumed);
                    
                    let (client_id, keep_alive) = match packet {
                        v4::Packet::Connect(connect) => {
                            let client_id = connect.client_id.clone();
                            let keep_alive = connect.keep_alive;
                            let username = connect.username.as_deref().unwrap_or("");
                            let password = connect.password.as_deref().unwrap_or(&[]);

                            if !self.authenticator.authenticate(&client_id, username, password) {
                                warn!("Authentication failed for {}", client_id);
                                let connack = v4::create_connack(false, ConnectReturnCodeV4::NotAuthorized);
                                let mut buf = [0u8; 10];
                                let written = connack.write(&mut buf).map_err(Error::from)?;
                                writer.write_all(&buf[..written]).await?;
                                return Err(Error::AuthenticationFailed);
                            }

                            debug!("Client {} authenticated, keep_alive={}s", client_id, keep_alive);

                            let connack = v4::create_connack(false, ConnectReturnCodeV4::Success);
                            let mut buf = [0u8; 10];
                            let written = connack.write(&mut buf).map_err(Error::from)?;
                            writer.write_all(&buf[..written]).await?;
                            writer.flush().await?;

                            (client_id, keep_alive)
                        }
                        _ => {
                            return Err(Error::UnexpectedPacket {
                                expected: "Connect".to_string(),
                                got: "other".to_string(),
                            });
                        }
                    };

                    return self.run_client_v4(&client_id, keep_alive, reader, writer, read_buf).await;
                }
                Err(mqtt0::Error::Incomplete { .. }) => {
                    // Need more data
                    let mut tmp = [0u8; 4096];
                    let n = reader.read(&mut tmp).await?;
                    if n == 0 {
                        return Err(Error::ConnectionClosed);
                    }
                    read_buf.extend_from_slice(&tmp[..n]);
                }
                Err(e) => return Err(e.into()),
            }
        }
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
            client_id: Arc::from(client_id),
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
                                let packet = v4::create_publish(&msg.topic, &msg.payload, msg.retain);
                                let mut buf = vec![0u8; packet.size() + 10];
                                let written = packet.write(&mut buf).map_err(Error::from)?;
                                writer.write_all(&buf[..written]).await?;
                                writer.flush().await?;
                                Ok::<bool, Error>(false)
                            }
                            None => Ok(true),
                        }
                    }

                    result = async {
                        // Try to read from buffer first
                        loop {
                            match v4::Packet::read(&read_buf, self.max_packet_size) {
                                Ok((packet, consumed)) => {
                                    let _ = read_buf.split_to(consumed);
                                    return Ok(packet);
                                }
                                Err(mqtt0::Error::Incomplete { .. }) => {
                                    let mut tmp = [0u8; 4096];
                                    let n = reader.read(&mut tmp).await?;
                                    if n == 0 {
                                        return Err(Error::ConnectionClosed);
                                    }
                                    read_buf.extend_from_slice(&tmp[..n]);
                                }
                                Err(e) => return Err(e.into()),
                            }
                        }
                    } => {
                        let packet = result?;

                        match packet {
                            v4::Packet::Publish(publish) => {
                                self.handle_publish_v4(client_id, publish).await?;
                            }
                            v4::Packet::Subscribe(subscribe) => {
                                let return_codes = self.handle_subscribe_v4(client_id, client_handle, &subscribe.filters).await;
                                let suback = v4::create_suback(subscribe.pkid, return_codes);
                                let mut buf = vec![0u8; suback.size() + 10];
                                let written = suback.write(&mut buf).map_err(Error::from)?;
                                writer.write_all(&buf[..written]).await?;
                                writer.flush().await?;
                            }
                            v4::Packet::Unsubscribe(unsubscribe) => {
                                self.handle_unsubscribe(client_id, &unsubscribe.topics);
                                let unsuback = v4::create_unsuback(unsubscribe.pkid);
                                let mut buf = [0u8; 10];
                                let written = unsuback.write(&mut buf).map_err(Error::from)?;
                                writer.write_all(&buf[..written]).await?;
                                writer.flush().await?;
                            }
                            v4::Packet::PingReq => {
                                // Send PINGRESP
                                let mut buf = [0u8; 2];
                                buf[0] = 0xD0;
                                buf[1] = 0x00;
                                writer.write_all(&buf).await?;
                                writer.flush().await?;
                            }
                            v4::Packet::Disconnect => {
                                return Ok(true);
                            }
                            _ => {
                                trace!("Ignoring packet from {}", client_id);
                            }
                        }
                        Ok::<bool, Error>(false)
                    }
                }
            };

            let should_exit = if let Some(timeout) = timeout_duration {
                match tokio::time::timeout(timeout, select_future).await {
                    Ok(result) => result?,
                    Err(_) => {
                        warn!("Client {} keep-alive timeout", client_id);
                        return Ok(());
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
        publish: v4::Publish,
    ) -> Result<()> {
        let topic = &publish.topic;

        if !self.authenticator.acl(client_id, topic, true) {
            warn!("ACL denied publish from {} to {}", client_id, topic);
            return Ok(());
        }

        trace!("Client {} published to {}", client_id, topic);

        let msg = Message {
            topic: topic.clone(),
            payload: publish.payload,
            retain: publish.retain,
        };

        if let Some(ref handler) = self.handler {
            handler.handle(client_id, &msg);
        }

        self.route_to_subscribers(topic, &msg).await;
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
        filters: &[v4::SubscribeFilter],
    ) -> Vec<v4::SubscribeReasonCode> {
        let mut return_codes = Vec::with_capacity(filters.len());

        for filter in filters {
            let topic = &filter.path;

            if !self.authenticator.acl(client_id, topic, false) {
                warn!("ACL denied subscribe from {} to {}", client_id, topic);
                return_codes.push(v4::SubscribeReasonCode::Failure);
                continue;
            }

            if let Err(e) = self.subscriptions.read().insert(topic, client_handle.clone()) {
                warn!("Failed to insert subscription: {}", e);
                return_codes.push(v4::SubscribeReasonCode::Failure);
                continue;
            }

            {
                let mut client_subs = self.client_subscriptions.write();
                client_subs
                    .entry(client_id.to_string())
                    .or_default()
                    .push(topic.to_string());
            }

            debug!("Client {} subscribed to {}", client_id, topic);
            return_codes.push(v4::SubscribeReasonCode::Success(QoS::AtMostOnce));
        }

        return_codes
    }

    fn handle_unsubscribe(&self, client_id: &str, topics: &[String]) {
        let subs = self.subscriptions.write();

        for topic in topics {
            subs.with_mut(|root| {
                root.remove(topic, |h| h.client_id.as_ref() == client_id);
            });
            debug!("Client {} unsubscribed from {}", client_id, topic);
        }

        let mut client_subs = self.client_subscriptions.write();
        if let Some(subs_list) = client_subs.get_mut(client_id) {
            subs_list.retain(|t| !topics.contains(t));
        }
    }

    fn cleanup_client(&self, client_id: &str) {
        self.clients.write().remove(client_id);

        let topics = self.client_subscriptions.write().remove(client_id);
        if let Some(topics) = topics {
            let topic_count = topics.len();
            let subs = self.subscriptions.write();
            for topic in &topics {
                subs.with_mut(|root| {
                    root.remove(topic, |h| h.client_id.as_ref() == client_id);
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
}
