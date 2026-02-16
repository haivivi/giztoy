//! Device discovery and connection acceptance via MQTT.
//!
//! The `Listener` follows the `net::TcpListener` pattern: it starts an embedded
//! MQTT broker and yields `AcceptedPort` instances when new devices connect.
//!
//! # Example
//!
//! ```ignore
//! let listener = listen_mqtt0(ListenerConfig {
//!     addr: "127.0.0.1:1883".to_string(),
//!     scope: "palr/cn".to_string(),
//!     ..Default::default()
//! }, logger).await?;
//!
//! loop {
//!     let accepted = listener.accept().await?;
//!     tokio::spawn(handle_device(accepted.gear_id, accepted.port));
//! }
//! ```

use crate::{
    ConnError, GearStateEvent, GearStatsEvent, StampedOpusFrame,
    conn_mqtt::unstamp_frame,
    logger::Logger,
};
use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::sync::{mpsc, Mutex, RwLock};
use tokio_util::sync::CancellationToken;

/// Configuration for the Listener.
#[derive(Debug, Clone)]
pub struct ListenerConfig {
    /// Address to listen on (e.g., "127.0.0.1:1883").
    pub addr: String,
    /// Topic prefix (e.g., "palr/cn").
    pub scope: String,
    /// Inactivity timeout for device connections. Default: 30s.
    pub timeout: Duration,
}

impl Default for ListenerConfig {
    fn default() -> Self {
        Self {
            addr: "127.0.0.1:1883".to_string(),
            scope: String::new(),
            timeout: Duration::from_secs(30),
        }
    }
}

/// A newly connected device.
#[derive(Debug)]
pub struct AcceptedPort {
    /// Device identifier.
    pub gear_id: String,
    /// Receiver for opus frames from this device.
    pub opus_rx: mpsc::Receiver<StampedOpusFrame>,
    /// Receiver for state events from this device.
    pub state_rx: mpsc::Receiver<GearStateEvent>,
    /// Receiver for stats events from this device.
    pub stats_rx: mpsc::Receiver<GearStatsEvent>,
}

/// Tracks a managed device port.
struct ManagedPort {
    gear_id: String,
    opus_tx: mpsc::Sender<StampedOpusFrame>,
    state_tx: mpsc::Sender<GearStateEvent>,
    stats_tx: mpsc::Sender<GearStatsEvent>,
    last_active: Instant,
}

/// Listener for device connections via MQTT.
///
/// Starts an embedded MQTT broker and routes incoming device messages
/// to per-device channels. New devices are announced via `accept()`.
pub struct Listener {
    accept_rx: Mutex<mpsc::Receiver<AcceptedPort>>,
    cancel: CancellationToken,
    logger: Arc<dyn Logger>,
}

impl Listener {
    /// Blocks until a new device connects.
    pub async fn accept(&self) -> Result<AcceptedPort, ConnError> {
        let mut rx = self.accept_rx.lock().await;
        match rx.recv().await {
            Some(port) => Ok(port),
            None => Err(ConnError::Closed),
        }
    }

    /// Closes the listener.
    pub fn close(&self) {
        self.cancel.cancel();
    }
}

/// Creates a new Listener that accepts device connections via MQTT.
///
/// Starts an embedded MQTT broker on the given address. Devices that publish
/// to `{scope}device/{gear_id}/input_audio_stream` are automatically discovered
/// and their data routed to the `AcceptedPort` returned by `accept()`.
pub async fn listen_mqtt0(
    cfg: ListenerConfig,
    log: Arc<dyn Logger>,
) -> Result<Listener, ConnError> {
    let scope = if cfg.scope.is_empty() || cfg.scope.ends_with('/') {
        cfg.scope.clone()
    } else {
        format!("{}/", cfg.scope)
    };

    let timeout = if cfg.timeout == Duration::ZERO {
        Duration::from_secs(30)
    } else {
        cfg.timeout
    };

    let (accept_tx, accept_rx) = mpsc::channel(32);
    let cancel = CancellationToken::new();
    let ports: Arc<RwLock<HashMap<String, ManagedPort>>> = Arc::new(RwLock::new(HashMap::new()));

    // Create broker with message handler
    let handler_ports = ports.clone();
    let handler_scope = scope.clone();
    let handler_accept_tx = accept_tx.clone();
    let handler_log = log.clone();
    let handler_timeout = timeout;

    let broker_cfg = giztoy_mqtt0::BrokerConfig::new(&cfg.addr).sys_events(false);
    let broker = giztoy_mqtt0::Broker::builder(broker_cfg)
        .handler(MessageHandler {
            ports: handler_ports.clone(),
            scope: handler_scope.clone(),
            accept_tx: handler_accept_tx,
            logger: handler_log.clone(),
        })
        .build();

    // Start broker
    let broker_cancel = cancel.clone();
    let broker_log = log.clone();
    tokio::spawn(async move {
        tokio::select! {
            result = broker.serve() => {
                if let Err(e) = result {
                    broker_log.error(&format!("broker serve error: {}", e));
                }
            }
            _ = broker_cancel.cancelled() => {
                broker_log.info("listener broker shutting down");
            }
        }
    });

    // Start timeout checker
    let timeout_ports = ports.clone();
    let timeout_cancel = cancel.clone();
    let timeout_log = log.clone();
    tokio::spawn(async move {
        loop {
            tokio::select! {
                _ = timeout_cancel.cancelled() => return,
                _ = tokio::time::sleep(Duration::from_secs(5)) => {
                    let mut ports = timeout_ports.write().await;
                    let now = Instant::now();
                    ports.retain(|gear_id, port| {
                        if now.duration_since(port.last_active) > handler_timeout {
                            timeout_log.info(&format!("device {} timed out", gear_id));
                            false
                        } else {
                            true
                        }
                    });
                }
            }
        }
    });

    log.info(&format!("listener started on {}", cfg.addr));

    Ok(Listener {
        accept_rx: Mutex::new(accept_rx),
        cancel,
        logger: log,
    })
}

/// MQTT message handler that routes messages to per-device channels.
struct MessageHandler {
    ports: Arc<RwLock<HashMap<String, ManagedPort>>>,
    scope: String,
    accept_tx: mpsc::Sender<AcceptedPort>,
    logger: Arc<dyn Logger>,
}

impl giztoy_mqtt0::Handler for MessageHandler {
    fn handle(&self, _client_id: &str, msg: &giztoy_mqtt0::Message) {
        let topic = &msg.topic;
        let payload = &msg.payload;

        // Parse topic: {scope}device/{gear_id}/{type}
        let prefix = format!("{}device/", self.scope);
        let rest = match topic.strip_prefix(&prefix) {
            Some(r) => r,
            None => return,
        };

        let (gear_id, msg_type) = match rest.find('/') {
            Some(idx) => (&rest[..idx], &rest[idx + 1..]),
            None => return,
        };

        if gear_id.is_empty() {
            return;
        }

        // Use a blocking approach for the async lock since Handler::handle is sync
        let ports = self.ports.clone();
        let accept_tx = self.accept_tx.clone();
        let gear_id = gear_id.to_string();
        let msg_type = msg_type.to_string();
        let payload = payload.to_vec();
        let logger = self.logger.clone();

        tokio::spawn(async move {
            let mut ports_guard = ports.write().await;

            // Get or create port for this gear
            if !ports_guard.contains_key(&gear_id) {
                let (opus_tx, opus_rx) = mpsc::channel(1024);
                let (state_tx, state_rx) = mpsc::channel(32);
                let (stats_tx, stats_rx) = mpsc::channel(32);

                ports_guard.insert(gear_id.clone(), ManagedPort {
                    gear_id: gear_id.clone(),
                    opus_tx,
                    state_tx,
                    stats_tx,
                    last_active: Instant::now(),
                });

                // Notify listener of new device
                let _ = accept_tx.try_send(AcceptedPort {
                    gear_id: gear_id.clone(),
                    opus_rx,
                    state_rx,
                    stats_rx,
                });

                logger.info(&format!("new device connected: {}", gear_id));
            }

            if let Some(port) = ports_guard.get_mut(&gear_id) {
                port.last_active = Instant::now();

                match msg_type.as_str() {
                    "input_audio_stream" => {
                        if let Some(frame) = unstamp_frame(&payload) {
                            let _ = port.opus_tx.try_send(frame);
                        }
                    }
                    "state" => {
                        if let Ok(evt) = serde_json::from_slice::<GearStateEvent>(&payload) {
                            let _ = port.state_tx.try_send(evt);
                        }
                    }
                    "stats" => {
                        if let Ok(evt) = serde_json::from_slice::<GearStatsEvent>(&payload) {
                            let _ = port.stats_tx.try_send(evt);
                        }
                    }
                    _ => {}
                }
            }
        });
    }
}
