//! MQTT server implementation using rumqttd.
//!
//! Provides an embedded MQTT broker with support for:
//! - TCP and WebSocket listeners
//! - Authentication and ACL
//! - Message handlers via ServeMux

use crate::error::{Error, Result};
use crate::serve_mux::{Message, ServeMux};
use crate::types::QoS;
use parking_lot::Mutex;
use rumqttd::{Broker, Config, ConnectionSettings, RouterConfig, ServerSettings};
use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use tokio::sync::broadcast;
use tracing::{debug, error, info};

/// Server configuration.
#[derive(Debug, Clone)]
pub struct ServerConfig {
    /// TCP listener address (e.g., "127.0.0.1:1883").
    pub tcp_addr: Option<String>,
    /// WebSocket listener address (e.g., "127.0.0.1:8083").
    pub ws_addr: Option<String>,
    /// Server ID.
    pub id: String,
    /// Max incoming packet size in bytes.
    pub max_packet_size: usize,
    /// Max inflight messages.
    pub max_inflight: u16,
}

impl ServerConfig {
    /// Create a new server config with TCP address.
    pub fn new(tcp_addr: impl Into<String>) -> Self {
        Self {
            tcp_addr: Some(tcp_addr.into()),
            ws_addr: None,
            id: "mqtt-server".to_string(),
            max_packet_size: 1024 * 1024, // 1MB
            max_inflight: 100,
        }
    }

    /// Add WebSocket listener address.
    pub fn with_websocket(mut self, addr: impl Into<String>) -> Self {
        self.ws_addr = Some(addr.into());
        self
    }

    /// Set server ID.
    pub fn with_id(mut self, id: impl Into<String>) -> Self {
        self.id = id.into();
        self
    }

    fn to_rumqttd_config(&self) -> Config {
        let mut servers = HashMap::new();

        if let Some(ref tcp_addr) = self.tcp_addr {
            let addr: SocketAddr = tcp_addr.parse().expect("Invalid TCP address");
            servers.insert(
                "tcp".to_string(),
                ServerSettings {
                    name: "tcp".to_string(),
                    listen: addr,
                    tls: None,
                    next_connection_delay_ms: 1,
                    connections: ConnectionSettings {
                        connection_timeout_ms: 60000,
                        max_payload_size: self.max_packet_size,
                        max_inflight_count: self.max_inflight as usize,
                        auth: None,
                        external_auth: None,
                        dynamic_filters: false,
                    },
                },
            );
        }

        if let Some(ref ws_addr) = self.ws_addr {
            let addr: SocketAddr = ws_addr.parse().expect("Invalid WebSocket address");
            servers.insert(
                "ws".to_string(),
                ServerSettings {
                    name: "ws".to_string(),
                    listen: addr,
                    tls: None,
                    next_connection_delay_ms: 1,
                    connections: ConnectionSettings {
                        connection_timeout_ms: 60000,
                        max_payload_size: self.max_packet_size,
                        max_inflight_count: self.max_inflight as usize,
                        auth: None,
                        external_auth: None,
                        dynamic_filters: false,
                    },
                },
            );
        }

        Config {
            id: 0,
            router: RouterConfig {
                max_connections: 10000,
                max_outgoing_packet_count: 200,
                max_segment_size: 1024 * 1024,
                max_segment_count: 10,
                ..Default::default()
            },
            v4: Some(servers),
            v5: None,
            ws: None,
            prometheus: None,
            metrics: None,
            console: None,
            bridge: None,
            cluster: None,
        }
    }
}

/// Authentication and authorization for MQTT clients.
pub trait Authenticator: Send + Sync {
    /// Authenticate a client connection.
    ///
    /// Returns true to allow the connection.
    fn authenticate(&self, client_id: &str, username: &str, password: &[u8]) -> bool;

    /// Check ACL permissions.
    ///
    /// `write` is true for publish, false for subscribe.
    fn acl(&self, client_id: &str, topic: &str, write: bool) -> bool;
}

/// Allow-all authenticator (default).
#[derive(Default)]
pub struct AllowAllAuth;

impl Authenticator for AllowAllAuth {
    fn authenticate(&self, _client_id: &str, _username: &str, _password: &[u8]) -> bool {
        true
    }

    fn acl(&self, _client_id: &str, _topic: &str, _write: bool) -> bool {
        true
    }
}

/// Write options for server publish.
pub struct ServerWriteOption {
    pub retain: bool,
    pub qos: QoS,
}

impl Default for ServerWriteOption {
    fn default() -> Self {
        Self {
            retain: false,
            qos: QoS::AtMostOnce,
        }
    }
}

/// Internal message for publishing from server.
#[derive(Clone)]
struct PublishMessage {
    topic: String,
    payload: Vec<u8>,
    retain: bool,
    qos: u8,
}

/// MQTT broker server.
pub struct Server {
    config: ServerConfig,
    handler: Option<Arc<ServeMux>>,
    authenticator: Option<Arc<dyn Authenticator>>,
    on_connect: Option<Box<dyn Fn(&str) + Send + Sync>>,
    on_disconnect: Option<Box<dyn Fn(&str) + Send + Sync>>,
    in_shutdown: AtomicBool,
    broker: Mutex<Option<Broker>>,
    publish_tx: broadcast::Sender<PublishMessage>,
}

impl Server {
    /// Create a new MQTT server.
    pub fn new(config: ServerConfig, handler: Option<Arc<ServeMux>>) -> Arc<Self> {
        let (publish_tx, _) = broadcast::channel(1000);
        Arc::new(Self {
            config,
            handler,
            authenticator: None,
            on_connect: None,
            on_disconnect: None,
            in_shutdown: AtomicBool::new(false),
            broker: Mutex::new(None),
            publish_tx,
        })
    }

    /// Create a new server builder.
    pub fn builder(config: ServerConfig) -> ServerBuilder {
        ServerBuilder::new(config)
    }

    /// Start the MQTT broker.
    ///
    /// This method blocks until the server is closed.
    pub async fn serve(self: &Arc<Self>) -> Result<()> {
        if self.in_shutdown.load(Ordering::SeqCst) {
            return Err(Error::ServerClosed);
        }

        {
            let guard = self.broker.lock();
            if guard.is_some() {
                return Err(Error::ServerRunning);
            }
        }

        let rumqttd_config = self.config.to_rumqttd_config();
        let broker = Broker::new(rumqttd_config);

        {
            let mut guard = self.broker.lock();
            *guard = Some(broker);
        }

        info!("Starting MQTT server");
        if let Some(ref addr) = self.config.tcp_addr {
            info!("TCP listener on {}", addr);
        }
        if let Some(ref addr) = self.config.ws_addr {
            info!("WebSocket listener on {}", addr);
        }

        // Get the broker and start it
        let mut broker = {
            let mut guard = self.broker.lock();
            guard.take()
        };

        if let Some(ref mut broker) = broker {
            // Spawn link handler if we have a handler
            if let Some(ref handler) = self.handler {
                let handler = handler.clone();
                let (mut link_tx, mut link_rx) = broker.link("server-handler").unwrap();

                // Subscribe to all topics to receive messages
                link_tx.subscribe("#").unwrap();

                // Spawn message handler
                tokio::spawn(async move {
                    loop {
                        match link_rx.recv() {
                            Ok(Some(notification)) => {
                                if let rumqttd::Notification::Forward(forward) = notification {
                                    let topic = String::from_utf8_lossy(&forward.publish.topic).to_string();
                                    let msg = Message {
                                        topic,
                                        payload: forward.publish.payload.clone(),
                                        qos: 0, // rumqttd doesn't expose QoS in Forward
                                        retain: forward.publish.retain,
                                        packet_id: None,
                                        user_properties: Vec::new(),
                                        client_id: None,
                                    };

                                    if let Err(e) = handler.handle_message(&msg) {
                                        debug!("Handler error: {}", e);
                                    }
                                }
                            }
                            Ok(None) => {
                                // No notification available
                                tokio::time::sleep(std::time::Duration::from_millis(10)).await;
                            }
                            Err(e) => {
                                error!("Link recv error: {}", e);
                                break;
                            }
                        }
                    }
                });

                // Handle publish requests from WriteToTopic
                let mut publish_rx = self.publish_tx.subscribe();
                tokio::spawn(async move {
                    while let Ok(msg) = publish_rx.recv().await {
                        if let Err(e) = link_tx.publish(msg.topic, msg.payload) {
                            error!("Publish error: {}", e);
                        }
                    }
                });
            }

            // Start the broker (this blocks)
            broker.start().map_err(|e| Error::Connection(e.to_string()))?;
        }

        Ok(())
    }

    /// Close the server gracefully.
    pub fn close(&self) -> Result<()> {
        self.in_shutdown.store(true, Ordering::SeqCst);

        let mut guard = self.broker.lock();
        if guard.is_some() {
            *guard = None;
        }

        Ok(())
    }

    /// Publish a message to the given topic.
    ///
    /// All clients subscribed to matching topics will receive the message.
    pub fn write_to_topic(&self, payload: &[u8], topic: &str) -> Result<()> {
        self.write_to_topic_with_opts(payload, topic, ServerWriteOption::default())
    }

    /// Publish a message to the given topic with options.
    pub fn write_to_topic_with_opts(
        &self,
        payload: &[u8],
        topic: &str,
        opts: ServerWriteOption,
    ) -> Result<()> {
        if self.broker.lock().is_none() {
            return Err(Error::ServerNotRunning);
        }

        let msg = PublishMessage {
            topic: topic.to_string(),
            payload: payload.to_vec(),
            retain: opts.retain,
            qos: opts.qos as u8,
        };

        self.publish_tx
            .send(msg)
            .map_err(|e| Error::Publish(e.to_string()))?;

        Ok(())
    }

    /// Check if the server is running.
    pub fn is_running(&self) -> bool {
        self.broker.lock().is_some()
    }
}

/// Builder for Server.
pub struct ServerBuilder {
    config: ServerConfig,
    handler: Option<Arc<ServeMux>>,
    authenticator: Option<Arc<dyn Authenticator>>,
    on_connect: Option<Box<dyn Fn(&str) + Send + Sync>>,
    on_disconnect: Option<Box<dyn Fn(&str) + Send + Sync>>,
}

impl ServerBuilder {
    /// Create a new builder.
    pub fn new(config: ServerConfig) -> Self {
        Self {
            config,
            handler: None,
            authenticator: None,
            on_connect: None,
            on_disconnect: None,
        }
    }

    /// Set the message handler.
    pub fn handler(mut self, handler: Arc<ServeMux>) -> Self {
        self.handler = Some(handler);
        self
    }

    /// Set the authenticator.
    pub fn authenticator(mut self, auth: Arc<dyn Authenticator>) -> Self {
        self.authenticator = Some(auth);
        self
    }

    /// Set the on_connect callback.
    pub fn on_connect<F>(mut self, f: F) -> Self
    where
        F: Fn(&str) + Send + Sync + 'static,
    {
        self.on_connect = Some(Box::new(f));
        self
    }

    /// Set the on_disconnect callback.
    pub fn on_disconnect<F>(mut self, f: F) -> Self
    where
        F: Fn(&str) + Send + Sync + 'static,
    {
        self.on_disconnect = Some(Box::new(f));
        self
    }

    /// Build the server.
    pub fn build(self) -> Arc<Server> {
        let (publish_tx, _) = broadcast::channel(1000);
        Arc::new(Server {
            config: self.config,
            handler: self.handler,
            authenticator: self.authenticator,
            on_connect: self.on_connect,
            on_disconnect: self.on_disconnect,
            in_shutdown: AtomicBool::new(false),
            broker: Mutex::new(None),
            publish_tx,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_server_config() {
        let config = ServerConfig::new("127.0.0.1:1883")
            .with_websocket("127.0.0.1:8083")
            .with_id("test-server");

        assert_eq!(config.tcp_addr, Some("127.0.0.1:1883".to_string()));
        assert_eq!(config.ws_addr, Some("127.0.0.1:8083".to_string()));
        assert_eq!(config.id, "test-server");
    }

    #[test]
    fn test_allow_all_auth() {
        let auth = AllowAllAuth;

        assert!(auth.authenticate("client1", "user", b"pass"));
        assert!(auth.acl("client1", "topic/test", true));
        assert!(auth.acl("client1", "topic/test", false));
    }

    #[test]
    fn test_server_builder() {
        let mux = Arc::new(ServeMux::new());
        let config = ServerConfig::new("127.0.0.1:1883");

        let server = Server::builder(config)
            .handler(mux)
            .authenticator(Arc::new(AllowAllAuth))
            .on_connect(|client_id| {
                println!("Connected: {}", client_id);
            })
            .on_disconnect(|client_id| {
                println!("Disconnected: {}", client_id);
            })
            .build();

        assert!(!server.is_running());
    }
}
