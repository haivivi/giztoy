//! MQTT client connection for chatgear.
//!
//! Implements `UplinkTx` (send to server) and `DownlinkRx` (receive from server)
//! over an MQTT broker, matching Go's `MQTTClientConn`.
//!
//! Wire format for stamped Opus frames:
//!
//! ```text
//! +--------+------------------+------------------+
//! | Version| Timestamp (7B)   | Opus Frame Data  |
//! | (1B)   | Big-endian ms    |                  |
//! +--------+------------------+------------------+
//! ```
//! Total header: 8 bytes

use crate::{
    ConnError, DownlinkRx, GearStateEvent, GearStatsEvent, SessionCommandEvent,
    StampedOpusFrame, UplinkTx,
    logger::{self, Logger},
};
use async_trait::async_trait;
use std::sync::Arc;
use tokio::sync::{mpsc, Mutex};
use tokio_util::sync::CancellationToken;

const FRAME_VERSION: u8 = 1;
const STAMPED_HEADER_SIZE: usize = 8;

/// Encode a StampedOpusFrame into the wire format.
pub fn stamp_frame(frame: &StampedOpusFrame) -> Vec<u8> {
    let stamp = frame.timestamp_ms as u64;
    let mut buf = stamp.to_be_bytes();
    buf[0] = FRAME_VERSION;
    let mut result = Vec::with_capacity(STAMPED_HEADER_SIZE + frame.frame.len());
    result.extend_from_slice(&buf);
    result.extend_from_slice(&frame.frame);
    result
}

/// Decode wire format into a StampedOpusFrame.
/// Returns None if the data is invalid.
pub fn unstamp_frame(data: &[u8]) -> Option<StampedOpusFrame> {
    if data.len() < STAMPED_HEADER_SIZE + 1 {
        return None;
    }
    if data[0] != FRAME_VERSION {
        return None;
    }
    let mut buf = [0u8; 8];
    buf[0] = 0; // Clear version byte
    buf[1..8].copy_from_slice(&data[1..8]);
    let stamp = i64::from_be_bytes(buf);
    let frame = data[STAMPED_HEADER_SIZE..].to_vec();
    Some(StampedOpusFrame::new(stamp, frame))
}

/// Configuration for MQTT client connection.
#[derive(Debug, Clone)]
pub struct MQTTClientConfig {
    /// MQTT broker address (e.g., "127.0.0.1:1883").
    pub addr: String,
    /// Topic prefix (e.g., "palr/cn").
    pub scope: String,
    /// Device identifier.
    pub gear_id: String,
    /// MQTT client identifier. Auto-generated if empty.
    pub client_id: String,
    /// Keep-alive interval in seconds. Default: 60.
    pub keep_alive: u16,
}

/// MQTT client connection.
///
/// Implements `UplinkTx` (send audio/state/stats to server) and
/// `DownlinkRx` (receive audio/commands from server).
pub struct MQTTClientConn {
    client: Arc<giztoy_mqtt0::Client>,
    cancel: CancellationToken,
    gear_id: String,
    scope: String,
    logger: Arc<dyn Logger>,

    opus_rx: Arc<Mutex<mpsc::Receiver<StampedOpusFrame>>>,
    commands_rx: Arc<Mutex<mpsc::Receiver<SessionCommandEvent>>>,

    closed: Arc<Mutex<bool>>,
}

impl MQTTClientConn {
    /// Returns the gear ID for this connection.
    pub fn gear_id(&self) -> &str {
        &self.gear_id
    }
}

/// Connects to an MQTT broker and returns a client connection.
pub async fn dial_mqtt(cfg: MQTTClientConfig, log: Arc<dyn Logger>) -> Result<MQTTClientConn, ConnError> {
    let scope = if cfg.scope.is_empty() || cfg.scope.ends_with('/') {
        cfg.scope.clone()
    } else {
        format!("{}/", cfg.scope)
    };

    let client_id = if cfg.client_id.is_empty() {
        format!("chatgear-{}-{}", cfg.gear_id, std::time::SystemTime::now()
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

    let audio_topic = format!("{}device/{}/output_audio_stream", scope, cfg.gear_id);
    let cmd_topic = format!("{}device/{}/command", scope, cfg.gear_id);

    client.subscribe(&[&audio_topic, &cmd_topic]).await
        .map_err(|e| ConnError::Other(format!("mqtt subscribe: {}", e)))?;

    log.info(&format!("subscribed to MQTT topics: audio={}, command={}", audio_topic, cmd_topic));

    let (opus_tx, opus_rx) = mpsc::channel(1024);
    let (cmd_tx, commands_rx) = mpsc::channel(32);
    let cancel = CancellationToken::new();

    let conn = MQTTClientConn {
        client: client.clone(),
        cancel: cancel.clone(),
        gear_id: cfg.gear_id.clone(),
        scope: scope.clone(),
        logger: log.clone(),
        opus_rx: Arc::new(Mutex::new(opus_rx)),
        commands_rx: Arc::new(Mutex::new(commands_rx)),
        closed: Arc::new(Mutex::new(false)),
    };

    // Spawn receive loop
    let recv_client = client.clone();
    let recv_cancel = cancel.clone();
    let recv_scope = scope.clone();
    let recv_gear_id = cfg.gear_id.clone();
    let recv_log = log.clone();
    tokio::spawn(async move {
        receive_loop(recv_client, recv_cancel, &recv_scope, &recv_gear_id, recv_log, opus_tx, cmd_tx).await;
    });

    Ok(conn)
}

async fn receive_loop(
    client: Arc<giztoy_mqtt0::Client>,
    cancel: CancellationToken,
    scope: &str,
    gear_id: &str,
    log: Arc<dyn Logger>,
    opus_tx: mpsc::Sender<StampedOpusFrame>,
    cmd_tx: mpsc::Sender<SessionCommandEvent>,
) {
    let audio_topic = format!("{}device/{}/output_audio_stream", scope, gear_id);
    let cmd_topic = format!("{}device/{}/command", scope, gear_id);

    loop {
        tokio::select! {
            _ = cancel.cancelled() => {
                log.info("receiveLoop: cancelled");
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
                        } else if msg.topic == cmd_topic {
                            match serde_json::from_slice::<SessionCommandEvent>(&msg.payload) {
                                Ok(evt) => { let _ = cmd_tx.try_send(evt); }
                                Err(e) => log.warn(&format!("failed to unmarshal command: {}", e)),
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
impl UplinkTx for MQTTClientConn {
    async fn send_opus_frame(&self, frame: &StampedOpusFrame) -> Result<(), ConnError> {
        let topic = format!("{}device/{}/input_audio_stream", self.scope, self.gear_id);
        let stamped = stamp_frame(frame);
        self.client.publish(&topic, &stamped).await
            .map_err(|e| ConnError::SendFailed(e.to_string()))
    }

    async fn send_state(&self, state: &GearStateEvent) -> Result<(), ConnError> {
        let topic = format!("{}device/{}/state", self.scope, self.gear_id);
        let data = serde_json::to_vec(state)
            .map_err(|e| ConnError::SendFailed(e.to_string()))?;
        self.client.publish(&topic, &data).await
            .map_err(|e| ConnError::SendFailed(e.to_string()))
    }

    async fn send_stats(&self, stats: &GearStatsEvent) -> Result<(), ConnError> {
        let topic = format!("{}device/{}/stats", self.scope, self.gear_id);
        let data = serde_json::to_vec(stats)
            .map_err(|e| ConnError::SendFailed(e.to_string()))?;
        self.client.publish(&topic, &data).await
            .map_err(|e| ConnError::SendFailed(e.to_string()))
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
impl DownlinkRx for MQTTClientConn {
    async fn recv_opus_frame(&self) -> Result<Option<StampedOpusFrame>, ConnError> {
        let mut rx = self.opus_rx.lock().await;
        Ok(rx.recv().await)
    }

    async fn recv_command(&self) -> Result<Option<SessionCommandEvent>, ConnError> {
        let mut rx = self.commands_rx.lock().await;
        Ok(rx.recv().await)
    }

    async fn close(&self) -> Result<(), ConnError> {
        UplinkTx::close(self).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_stamp_unstamp_roundtrip() {
        let frame = StampedOpusFrame::new(1700000000000, vec![0xFC, 0x01, 0x02, 0x03]);
        let stamped = stamp_frame(&frame);
        assert_eq!(stamped.len(), STAMPED_HEADER_SIZE + 4);

        let decoded = unstamp_frame(&stamped).unwrap();
        assert_eq!(decoded.timestamp_ms, frame.timestamp_ms);
        assert_eq!(decoded.frame, frame.frame);
    }

    #[test]
    fn test_unstamp_invalid() {
        // Too short
        assert!(unstamp_frame(&[]).is_none());
        assert!(unstamp_frame(&[1, 2, 3]).is_none());

        // Wrong version
        let mut data = vec![0u8; 12];
        data[0] = 99;
        assert!(unstamp_frame(&data).is_none());

        // Valid header but no frame data
        let mut data = vec![0u8; STAMPED_HEADER_SIZE];
        data[0] = FRAME_VERSION;
        assert!(unstamp_frame(&data).is_none());
    }
}
