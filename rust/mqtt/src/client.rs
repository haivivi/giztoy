//! MQTT client implementation using rumqttc.
//!
//! Provides a client for connecting to MQTT brokers with:
//! - Automatic reconnection
//! - Message routing via ServeMux
//! - Subscribe and publish operations

use crate::error::{Error, Result};
use crate::serve_mux::{Message, ServeMux};
use crate::types::QoS;
use bytes::Bytes;
use parking_lot::Mutex;
use rumqttc::{AsyncClient, Event, EventLoop, MqttOptions, Packet};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::broadcast;
use tracing::{debug, error, info, warn};
use uuid::Uuid;

/// Options for dialing an MQTT connection.
pub enum DialOption {
    /// Set username and password.
    WithUser { username: String, password: String },
    /// Set keep-alive interval.
    WithKeepAlive(Duration),
    /// Set clean session flag.
    WithCleanSession(bool),
}

/// Options for publishing a message.
pub enum WriteOption {
    /// Set QoS level.
    Qos(QoS),
    /// Set retain flag.
    Retain,
    /// Set packet ID.
    PacketId(u16),
}

/// Options for subscribing to a topic.
pub enum SubscribeOption {
    /// Set QoS level.
    Qos(QoS),
    /// Set shared group.
    SharedGroup(String),
    /// Auto resubscribe on reconnect.
    AutoResubscribe,
}

/// MQTT client dialer.
///
/// Contains all options to establish and maintain an MQTT connection.
#[derive(Default)]
pub struct Dialer {
    /// Keep-alive interval in seconds.
    pub keep_alive: Option<u16>,
    /// Session expiry interval in seconds.
    pub session_expiry_interval: Option<u32>,
    /// Connection retry delay.
    pub connect_retry_delay: Option<Duration>,
    /// Connection timeout.
    pub connect_timeout: Option<Duration>,
    /// Client ID (defaults to random UUID).
    pub id: Option<String>,
    /// Message handler.
    pub serve_mux: Option<Arc<ServeMux>>,
    /// Callback on connection error.
    pub on_connect_error: Option<Box<dyn Fn(&Error) + Send + Sync>>,
    /// Callback on connection up.
    pub on_connection_up: Option<Box<dyn Fn() + Send + Sync>>,
}

impl Dialer {
    /// Create a new dialer with default options.
    pub fn new() -> Self {
        Self::default()
    }

    /// Set the client ID.
    pub fn with_id(mut self, id: impl Into<String>) -> Self {
        self.id = Some(id.into());
        self
    }

    /// Set the keep-alive interval.
    pub fn with_keep_alive(mut self, seconds: u16) -> Self {
        self.keep_alive = Some(seconds);
        self
    }

    /// Set the message handler.
    pub fn with_serve_mux(mut self, mux: Arc<ServeMux>) -> Self {
        self.serve_mux = Some(mux);
        self
    }

    /// Set the connection retry delay.
    pub fn with_connect_retry_delay(mut self, delay: Duration) -> Self {
        self.connect_retry_delay = Some(delay);
        self
    }

    /// Set the connection timeout.
    pub fn with_connect_timeout(mut self, timeout: Duration) -> Self {
        self.connect_timeout = Some(timeout);
        self
    }

    /// Set the on_connect_error callback.
    pub fn with_on_connect_error<F>(mut self, f: F) -> Self
    where
        F: Fn(&Error) + Send + Sync + 'static,
    {
        self.on_connect_error = Some(Box::new(f));
        self
    }

    /// Set the on_connection_up callback.
    pub fn with_on_connection_up<F>(mut self, f: F) -> Self
    where
        F: Fn() + Send + Sync + 'static,
    {
        self.on_connection_up = Some(Box::new(f));
        self
    }

    /// Connect to the MQTT broker at the given address.
    ///
    /// Address format: `mqtt://[user:pass@]host:port`
    ///
    /// # Example
    ///
    /// ```no_run
    /// use giztoy_mqtt::Dialer;
    ///
    /// #[tokio::main]
    /// async fn main() -> anyhow::Result<()> {
    ///     let conn = Dialer::new().dial("mqtt://127.0.0.1:1883").await?;
    ///     conn.subscribe("test/topic").await?;
    ///     Ok(())
    /// }
    /// ```
    pub async fn dial(self, addr: &str) -> Result<Conn> {
        self.dial_with_opts(addr, &[]).await
    }

    /// Connect with additional options.
    pub async fn dial_with_opts(self, addr: &str, opts: &[DialOption]) -> Result<Conn> {
        let url = url::Url::parse(addr).map_err(|e| Error::Connection(e.to_string()))?;

        let host = url.host_str().unwrap_or("127.0.0.1");
        let port = url.port().unwrap_or(1883);

        let id = self.id.unwrap_or_else(|| Uuid::new_v4().to_string());
        let keep_alive = self.keep_alive.unwrap_or(20);

        let mut mqtt_options = MqttOptions::new(&id, host, port);
        mqtt_options.set_keep_alive(Duration::from_secs(keep_alive as u64));

        // Set credentials from URL
        if let Some(password) = url.password() {
            mqtt_options.set_credentials(url.username(), password);
        }

        // Apply dial options
        for opt in opts {
            match opt {
                DialOption::WithUser { username, password } => {
                    mqtt_options.set_credentials(username, password);
                }
                DialOption::WithKeepAlive(duration) => {
                    mqtt_options.set_keep_alive(*duration);
                }
                DialOption::WithCleanSession(clean) => {
                    mqtt_options.set_clean_session(*clean);
                }
            }
        }

        let (client, event_loop) = AsyncClient::new(mqtt_options, 100);

        let mux = self.serve_mux.unwrap_or_else(|| Arc::new(ServeMux::new()));
        let (shutdown_tx, _) = broadcast::channel(1);

        let conn = Conn {
            client,
            serve_mux: mux.clone(),
            subscriptions: Mutex::new(Vec::new()),
            connected: AtomicBool::new(false),
            shutdown_tx,
        };

        // Spawn event loop handler
        let conn_arc = Arc::new(conn);
        let conn_clone = conn_arc.clone();

        tokio::spawn(async move {
            conn_clone.run_event_loop(event_loop, mux).await;
        });

        // Wait for initial connection
        let timeout = self.connect_timeout.unwrap_or(Duration::from_secs(10));
        let start = std::time::Instant::now();

        while !conn_arc.connected.load(Ordering::SeqCst) {
            if start.elapsed() > timeout {
                return Err(Error::Connection("Connection timeout".to_string()));
            }
            tokio::time::sleep(Duration::from_millis(50)).await;
        }

        // Extract from Arc - we know there's only one reference at this point
        // since the spawned task only holds a clone
        Ok(Arc::try_unwrap(conn_arc).unwrap_or_else(|arc| {
            // If we can't unwrap, create a new Conn that shares the client
            Conn {
                client: arc.client.clone(),
                serve_mux: arc.serve_mux.clone(),
                subscriptions: Mutex::new(arc.subscriptions.lock().clone()),
                connected: AtomicBool::new(arc.connected.load(Ordering::SeqCst)),
                shutdown_tx: arc.shutdown_tx.clone(),
            }
        }))
    }
}

/// MQTT connection.
pub struct Conn {
    client: AsyncClient,
    serve_mux: Arc<ServeMux>,
    subscriptions: Mutex<Vec<(String, QoS)>>,
    connected: AtomicBool,
    shutdown_tx: broadcast::Sender<()>,
}

impl Conn {
    async fn run_event_loop(self: &Arc<Self>, mut event_loop: EventLoop, mux: Arc<ServeMux>) {
        let mut shutdown_rx = self.shutdown_tx.subscribe();

        loop {
            tokio::select! {
                _ = shutdown_rx.recv() => {
                    info!("Connection shutdown requested");
                    break;
                }
                event = event_loop.poll() => {
                    match event {
                        Ok(Event::Incoming(Packet::ConnAck(_))) => {
                            info!("Connected to MQTT broker");
                            self.connected.store(true, Ordering::SeqCst);

                            // Resubscribe to all topics
                            let subs = self.subscriptions.lock().clone();
                            for (topic, qos) in subs {
                                if let Err(e) = self.client.subscribe(&topic, qos.into()).await {
                                    error!("Resubscribe error: {}", e);
                                }
                            }
                        }
                        Ok(Event::Incoming(Packet::Publish(publish))) => {
                            debug!("Received message on topic: {}", publish.topic);

                            let msg = Message {
                                topic: publish.topic.clone(),
                                payload: Bytes::from(publish.payload.to_vec()),
                                qos: publish.qos as u8,
                                retain: publish.retain,
                                packet_id: Some(publish.pkid),
                                user_properties: Vec::new(),
                                client_id: None,
                            };

                            if let Err(e) = mux.handle_message(&msg) {
                                debug!("Handler error: {}", e);
                            }
                        }
                        Ok(Event::Incoming(Packet::SubAck(suback))) => {
                            debug!("Subscription acknowledged: {:?}", suback);
                        }
                        Ok(Event::Incoming(Packet::PubAck(puback))) => {
                            debug!("Publish acknowledged: {}", puback.pkid);
                        }
                        Ok(Event::Incoming(Packet::Disconnect)) => {
                            warn!("Disconnected from broker");
                            self.connected.store(false, Ordering::SeqCst);
                        }
                        Ok(_) => {}
                        Err(e) => {
                            error!("Event loop error: {}", e);
                            self.connected.store(false, Ordering::SeqCst);

                            // Retry connection
                            tokio::time::sleep(Duration::from_secs(3)).await;
                        }
                    }
                }
            }
        }
    }

    /// Close the connection.
    pub async fn close(&self) -> Result<()> {
        let _ = self.shutdown_tx.send(());
        self.client
            .disconnect()
            .await
            .map_err(|e| Error::Connection(e.to_string()))?;
        Ok(())
    }

    /// Subscribe to a topic.
    pub async fn subscribe(&self, topic: &str) -> Result<()> {
        self.subscribe_with_opts(topic, &[]).await
    }

    /// Subscribe to a topic with options.
    pub async fn subscribe_with_opts(
        &self,
        topic: &str,
        opts: &[SubscribeOption],
    ) -> Result<()> {
        let mut qos = QoS::AtMostOnce;
        let mut actual_topic = topic.to_string();
        let mut auto_resub = false;

        for opt in opts {
            match opt {
                SubscribeOption::Qos(q) => qos = *q,
                SubscribeOption::SharedGroup(group) => {
                    actual_topic = format!("$share/{}/{}", group, topic);
                }
                SubscribeOption::AutoResubscribe => {
                    auto_resub = true;
                }
            }
        }

        if auto_resub {
            self.subscriptions
                .lock()
                .push((actual_topic.clone(), qos));
        }

        self.client
            .subscribe(&actual_topic, qos.into())
            .await
            .map_err(|e| Error::Subscribe(e.to_string()))?;

        Ok(())
    }

    /// Subscribe to multiple topics.
    pub async fn subscribe_all(&self, topics: &[&str]) -> Result<()> {
        for topic in topics {
            self.subscribe(topic).await?;
        }
        Ok(())
    }

    /// Unsubscribe from a topic.
    pub async fn unsubscribe(&self, topic: &str) -> Result<()> {
        self.client
            .unsubscribe(topic)
            .await
            .map_err(|e| Error::Subscribe(e.to_string()))?;

        // Remove from auto-resubscribe list
        self.subscriptions.lock().retain(|(t, _)| t != topic);

        Ok(())
    }

    /// Publish a message to a topic.
    pub async fn write_to_topic(&self, payload: &[u8], topic: &str) -> Result<()> {
        self.write_to_topic_with_opts(payload, topic, &[]).await
    }

    /// Publish a message to a topic with options.
    pub async fn write_to_topic_with_opts(
        &self,
        payload: &[u8],
        topic: &str,
        opts: &[WriteOption],
    ) -> Result<()> {
        let mut qos = QoS::AtMostOnce;
        let mut retain = false;

        for opt in opts {
            match opt {
                WriteOption::Qos(q) => qos = *q,
                WriteOption::Retain => retain = true,
                WriteOption::PacketId(_) => {} // Handled by rumqttc internally
            }
        }

        self.client
            .publish(topic, qos.into(), retain, payload)
            .await
            .map_err(|e| Error::Publish(e.to_string()))?;

        Ok(())
    }

    /// Check if connected.
    pub fn is_connected(&self) -> bool {
        self.connected.load(Ordering::SeqCst)
    }

    /// Get the serve mux.
    pub fn serve_mux(&self) -> &Arc<ServeMux> {
        &self.serve_mux
    }
}

/// Connect to an MQTT broker with default options.
///
/// This is a convenience function equivalent to `Dialer::new().dial(addr)`.
pub async fn dial(addr: &str) -> Result<Conn> {
    Dialer::new().dial(addr).await
}

/// Connect to an MQTT broker with a serve mux.
pub async fn dial_with_mux(addr: &str, mux: Arc<ServeMux>) -> Result<Conn> {
    Dialer::new().with_serve_mux(mux).dial(addr).await
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_dialer_builder() {
        let mux = Arc::new(ServeMux::new());

        let dialer = Dialer::new()
            .with_id("test-client")
            .with_keep_alive(30)
            .with_serve_mux(mux)
            .with_connect_timeout(Duration::from_secs(5));

        assert_eq!(dialer.id, Some("test-client".to_string()));
        assert_eq!(dialer.keep_alive, Some(30));
        assert!(dialer.serve_mux.is_some());
    }

    #[test]
    fn test_qos_conversion() {
        assert_eq!(rumqttc::QoS::from(QoS::AtMostOnce), rumqttc::QoS::AtMostOnce);
        assert_eq!(
            rumqttc::QoS::from(QoS::AtLeastOnce),
            rumqttc::QoS::AtLeastOnce
        );
        assert_eq!(
            rumqttc::QoS::from(QoS::ExactlyOnce),
            rumqttc::QoS::ExactlyOnce
        );
    }
}
