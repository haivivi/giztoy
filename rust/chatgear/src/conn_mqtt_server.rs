//! MQTT server connection for chatgear.
//!
//! Implements `UplinkRx` (receive from client) and `DownlinkTx` (send to client)
//! over an MQTT broker, matching Go's `MQTTServerConn`.

use crate::{
    ConnError, DownlinkTx, GearStateEvent, GearStatsEvent, SessionCommandEvent,
    StampedOpusFrame, UplinkRx,
    conn_mqtt::{stamp_frame, unstamp_frame},
    logger::Logger,
};
use async_trait::async_trait;
use std::sync::Arc;
use tokio::sync::{mpsc, Mutex};
use tokio_util::sync::CancellationToken;

/// Configuration for MQTT server connection.
#[derive(Debug, Clone)]
pub struct MQTTServerConfig {
    /// MQTT broker address (e.g., "127.0.0.1:1883").
    pub addr: String,
    /// Topic prefix (e.g., "palr/cn").
    pub scope: String,
    /// Device identifier to listen for.
    pub gear_id: String,
    /// MQTT client identifier. Auto-generated if empty.
    pub client_id: String,
    /// Keep-alive interval in seconds. Default: 60.
    pub keep_alive: u16,
}

/// MQTT server connection.
///
/// Connects to an MQTT broker as a subscriber and implements both
/// `UplinkRx` (receive audio/state/stats from client) and
/// `DownlinkTx` (send audio/commands to client).
pub struct MQTTServerConn {
    client: Arc<BrokerOrClient>,
    cancel: CancellationToken,
    scope: String,
    gear_id: String,
    logger: Arc<dyn Logger>,

    opus_rx: Arc<Mutex<mpsc::Receiver<StampedOpusFrame>>>,
    states_rx: Arc<Mutex<mpsc::Receiver<GearStateEvent>>>,
    stats_rx: Arc<Mutex<mpsc::Receiver<GearStatsEvent>>>,
    latest_stats: Arc<Mutex<Option<GearStatsEvent>>>,

    closed: Arc<Mutex<bool>>,
}

impl MQTTServerConn {
    /// Returns the gear ID for this connection.
    pub fn gear_id(&self) -> &str {
        &self.gear_id
    }
}

/// Connects to an MQTT broker and returns a server connection.
pub async fn dial_mqtt_server(cfg: MQTTServerConfig, log: Arc<dyn Logger>) -> Result<MQTTServerConn, ConnError> {
    let scope = if cfg.scope.is_empty() || cfg.scope.ends_with('/') {
        cfg.scope.clone()
    } else {
        format!("{}/", cfg.scope)
    };

    let client_id = if cfg.client_id.is_empty() {
        format!("chatgear-server-{}-{}", cfg.gear_id, std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_millis() % 10000)
    } else {
        cfg.client_id.clone()
    };

    let keep_alive = if cfg.keep_alive == 0 { 60 } else { cfg.keep_alive };

    let mut mqtt_cfg = giztoy_mqtt0::ClientConfig::new(&cfg.addr, &client_id);
    mqtt_cfg.keep_alive = keep_alive;
    let client = giztoy_mqtt0::Client::connect(mqtt_cfg)
        .await.map_err(|e| ConnError::Other(format!("mqtt connect: {}", e)))?;

    let client = Arc::new(client);

    // Subscribe to uplink topics (from client)
    let audio_topic = format!("{}device/{}/input_audio_stream", scope, cfg.gear_id);
    let state_topic = format!("{}device/{}/state", scope, cfg.gear_id);
    let stats_topic = format!("{}device/{}/stats", scope, cfg.gear_id);

    client.subscribe(&[&audio_topic, &state_topic, &stats_topic]).await
        .map_err(|e| ConnError::Other(format!("mqtt subscribe: {}", e)))?;

    log.info(&format!(
        "subscribed to MQTT topics: audio={}, state={}, stats={}",
        audio_topic, state_topic, stats_topic
    ));

    let (opus_tx, opus_rx) = mpsc::channel(1024);
    let (states_tx, states_rx) = mpsc::channel(32);
    let (stats_tx, stats_rx) = mpsc::channel(32);
    let cancel = CancellationToken::new();
    let latest_stats = Arc::new(Mutex::new(None));

    let conn = MQTTServerConn {
        client: Arc::new(BrokerOrClient::Client(Arc::clone(&client))),
        cancel: cancel.clone(),
        scope: scope.clone(),
        gear_id: cfg.gear_id.clone(),
        logger: log.clone(),
        opus_rx: Arc::new(Mutex::new(opus_rx)),
        states_rx: Arc::new(Mutex::new(states_rx)),
        stats_rx: Arc::new(Mutex::new(stats_rx)),
        latest_stats: latest_stats.clone(),
        closed: Arc::new(Mutex::new(false)),
    };

    // Spawn receive loop
    let recv_client = client.clone();
    let recv_cancel = cancel.clone();
    let recv_scope = scope.clone();
    let recv_gear_id = cfg.gear_id.clone();
    let recv_log = log.clone();
    let recv_latest_stats = latest_stats.clone();
    tokio::spawn(async move {
        server_receive_loop(
            recv_client, recv_cancel, &recv_scope, &recv_gear_id, recv_log,
            opus_tx, states_tx, stats_tx, recv_latest_stats,
        ).await;
    });

    Ok(conn)
}

/// Starts an embedded MQTT broker and returns a server connection.
///
/// The server handles messages internally without network overhead for the
/// server side. Clients connect to `cfg.addr` to communicate.
/// Matches Go's `ListenMQTTServer`.
pub async fn listen_mqtt_server(cfg: MQTTServerConfig, log: Arc<dyn Logger>) -> Result<MQTTServerConn, ConnError> {
    let scope = if cfg.scope.is_empty() || cfg.scope.ends_with('/') {
        cfg.scope.clone()
    } else {
        format!("{}/", cfg.scope)
    };

    let (opus_tx, opus_rx) = mpsc::channel(1024);
    let (states_tx, states_rx) = mpsc::channel(32);
    let (stats_tx, stats_rx) = mpsc::channel(32);
    let cancel = CancellationToken::new();
    let latest_stats = Arc::new(Mutex::new(None));

    let handler_scope = scope.clone();
    let handler_gear_id = cfg.gear_id.clone();
    let handler_log = log.clone();
    let handler_latest_stats = latest_stats.clone();

    let broker_cfg = giztoy_mqtt0::BrokerConfig::new(&cfg.addr).sys_events(false);
    let broker = std::sync::Arc::new(giztoy_mqtt0::Broker::builder(broker_cfg)
        .handler(BrokerMessageHandler {
            scope: handler_scope,
            gear_id: handler_gear_id,
            logger: handler_log,
            opus_tx,
            states_tx,
            stats_tx,
            latest_stats: handler_latest_stats,
        })
        .build());

    let broker_for_serve = broker.clone();
    let broker_cancel = cancel.clone();
    let serve_log = log.clone();
    tokio::spawn(async move {
        tokio::select! {
            result = broker_for_serve.serve() => {
                if let Err(e) = result {
                    serve_log.error(&format!("broker serve error: {}", e));
                }
            }
            _ = broker_cancel.cancelled() => {
                serve_log.info("listen_mqtt_server: shutting down");
            }
        }
    });

    log.info(&format!("MQTT broker listening on {} for gear {}", cfg.addr, cfg.gear_id));

    Ok(MQTTServerConn {
        client: std::sync::Arc::new(BrokerOrClient::Broker(broker)),
        cancel,
        scope,
        gear_id: cfg.gear_id,
        logger: log,
        opus_rx: Arc::new(Mutex::new(opus_rx)),
        states_rx: Arc::new(Mutex::new(states_rx)),
        stats_rx: Arc::new(Mutex::new(stats_rx)),
        latest_stats,
        closed: Arc::new(Mutex::new(false)),
    })
}

/// Message handler for embedded broker mode.
struct BrokerMessageHandler {
    scope: String,
    gear_id: String,
    logger: Arc<dyn Logger>,
    opus_tx: mpsc::Sender<StampedOpusFrame>,
    states_tx: mpsc::Sender<GearStateEvent>,
    stats_tx: mpsc::Sender<GearStatsEvent>,
    latest_stats: Arc<Mutex<Option<GearStatsEvent>>>,
}

impl giztoy_mqtt0::Handler for BrokerMessageHandler {
    fn handle(&self, _client_id: &str, msg: &giztoy_mqtt0::Message) {
        let audio_topic = format!("{}device/{}/input_audio_stream", self.scope, self.gear_id);
        let state_topic = format!("{}device/{}/state", self.scope, self.gear_id);
        let stats_topic = format!("{}device/{}/stats", self.scope, self.gear_id);

        if msg.topic == audio_topic {
            if let Some(frame) = unstamp_frame(&msg.payload) {
                let _ = self.opus_tx.try_send(frame);
            }
        } else if msg.topic == state_topic {
            if let Ok(evt) = serde_json::from_slice::<GearStateEvent>(&msg.payload) {
                let _ = self.states_tx.try_send(evt);
            }
        } else if msg.topic == stats_topic {
            if let Ok(evt) = serde_json::from_slice::<GearStatsEvent>(&msg.payload) {
                // Use try_lock since this is synchronous handler context
                if let Ok(mut guard) = self.latest_stats.try_lock() {
                    *guard = Some(evt.clone());
                }
                let _ = self.stats_tx.try_send(evt);
            }
        }
    }
}

/// Enum to hold either a client or broker for publishing.
enum BrokerOrClient {
    Client(Arc<giztoy_mqtt0::Client>),
    Broker(std::sync::Arc<giztoy_mqtt0::Broker>),
}

impl BrokerOrClient {
    async fn publish(&self, topic: &str, payload: &[u8]) -> Result<(), ConnError> {
        match self {
            BrokerOrClient::Client(client) => {
                client.publish(topic, payload).await
                    .map_err(|e| ConnError::SendFailed(e.to_string()))
            }
            BrokerOrClient::Broker(broker) => {
                broker.publish(topic, payload)
                    .map_err(|e| ConnError::SendFailed(e.to_string()))
            }
        }
    }
}

async fn server_receive_loop(
    client: Arc<giztoy_mqtt0::Client>,
    cancel: CancellationToken,
    scope: &str,
    gear_id: &str,
    log: Arc<dyn Logger>,
    opus_tx: mpsc::Sender<StampedOpusFrame>,
    states_tx: mpsc::Sender<GearStateEvent>,
    stats_tx: mpsc::Sender<GearStatsEvent>,
    latest_stats: Arc<Mutex<Option<GearStatsEvent>>>,
) {
    let audio_topic = format!("{}device/{}/input_audio_stream", scope, gear_id);
    let state_topic = format!("{}device/{}/state", scope, gear_id);
    let stats_topic = format!("{}device/{}/stats", scope, gear_id);

    loop {
        tokio::select! {
            _ = cancel.cancelled() => {
                log.info("server receiveLoop: cancelled");
                return;
            }
            result = client.recv() => {
                match result {
                    Ok(msg) => {
                        if msg.topic == audio_topic {
                            if let Some(frame) = unstamp_frame(&msg.payload) {
                                let _ = opus_tx.try_send(frame);
                            } else {
                                log.warn("invalid stamped frame received");
                            }
                        } else if msg.topic == state_topic {
                            match serde_json::from_slice::<GearStateEvent>(&msg.payload) {
                                Ok(evt) => { let _ = states_tx.try_send(evt); }
                                Err(e) => log.warn(&format!("failed to unmarshal state: {}", e)),
                            }
                        } else if msg.topic == stats_topic {
                            match serde_json::from_slice::<GearStatsEvent>(&msg.payload) {
                                Ok(evt) => {
                                    *latest_stats.lock().await = Some(evt.clone());
                                    let _ = stats_tx.try_send(evt);
                                }
                                Err(e) => log.warn(&format!("failed to unmarshal stats: {}", e)),
                            }
                        }
                    }
                    Err(e) => {
                        log.warn(&format!("mqtt recv error: {}", e));
                        return;
                    }
                }
            }
        }
    }
}

#[async_trait]
impl UplinkRx for MQTTServerConn {
    async fn recv_opus_frame(&self) -> Result<Option<StampedOpusFrame>, ConnError> {
        let mut rx = self.opus_rx.lock().await;
        Ok(rx.recv().await)
    }

    async fn recv_state(&self) -> Result<Option<GearStateEvent>, ConnError> {
        let mut rx = self.states_rx.lock().await;
        Ok(rx.recv().await)
    }

    async fn recv_stats(&self) -> Result<Option<GearStatsEvent>, ConnError> {
        let mut rx = self.stats_rx.lock().await;
        Ok(rx.recv().await)
    }

    async fn latest_stats(&self) -> Option<GearStatsEvent> {
        self.latest_stats.lock().await.clone()
    }

    async fn close(&self) -> Result<(), ConnError> {
        let mut closed = self.closed.lock().await;
        if *closed {
            return Ok(());
        }
        *closed = true;
        self.cancel.cancel();
        Ok(())
    }
}

#[async_trait]
impl DownlinkTx for MQTTServerConn {
    async fn send_opus_frame(&self, frame: &StampedOpusFrame) -> Result<(), ConnError> {
        let topic = format!("{}device/{}/output_audio_stream", self.scope, self.gear_id);
        let stamped = stamp_frame(frame);
        self.client.publish(&topic, &stamped).await
            .map_err(|e| ConnError::SendFailed(e.to_string()))
    }

    async fn send_command(&self, cmd: &SessionCommandEvent) -> Result<(), ConnError> {
        let topic = format!("{}device/{}/command", self.scope, self.gear_id);
        let data = serde_json::to_vec(cmd)
            .map_err(|e| ConnError::SendFailed(e.to_string()))?;
        self.client.publish(&topic, &data).await
            .map_err(|e| ConnError::SendFailed(e.to_string()))
    }

    async fn close(&self) -> Result<(), ConnError> {
        UplinkRx::close(self).await
    }
}
