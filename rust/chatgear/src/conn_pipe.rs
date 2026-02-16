//! In-memory pipe connection for testing.
//!
//! This module provides a bidirectional pipe for testing client-server communication
//! without actual network connections.

use crate::{
    ConnError, DownlinkRx, DownlinkTx, GearStateEvent, GearStatsEvent, SessionCommandEvent,
    StampedOpusFrame, UplinkRx, UplinkTx,
};
use async_trait::async_trait;
use std::sync::Arc;
use tokio::sync::{mpsc, Mutex, RwLock};

/// Creates a connected pair of server and client connections using channels.
/// This is useful for testing and in-process communication.
pub fn new_pipe() -> (PipeServerConn, PipeClientConn) {
    // Uplink channels (client -> server)
    let (uplink_opus_tx, uplink_opus_rx) = mpsc::channel::<StampedOpusFrame>(1024);
    let (uplink_states_tx, uplink_states_rx) = mpsc::channel(32);
    let (uplink_stats_tx, uplink_stats_rx) = mpsc::channel(32);

    // Downlink channels (server -> client)
    let (downlink_opus_tx, downlink_opus_rx) = mpsc::channel::<StampedOpusFrame>(1024);
    let (downlink_cmds_tx, downlink_cmds_rx) = mpsc::channel(32);

    // Shared error state
    let shared = Arc::new(RwLock::new(PipeSharedState::default()));

    let server = PipeServerConn {
        uplink_opus: Arc::new(Mutex::new(uplink_opus_rx)),
        uplink_states: Arc::new(Mutex::new(uplink_states_rx)),
        uplink_stats: Arc::new(Mutex::new(uplink_stats_rx)),
        downlink_opus: downlink_opus_tx,
        downlink_cmds: downlink_cmds_tx,
        shared: shared.clone(),
        latest_stats: Arc::new(Mutex::new(None)),
        closed: Arc::new(Mutex::new(false)),
    };

    let client = PipeClientConn {
        uplink_opus: uplink_opus_tx,
        uplink_states: uplink_states_tx,
        uplink_stats: uplink_stats_tx,
        downlink_opus: Arc::new(Mutex::new(downlink_opus_rx)),
        downlink_cmds: Arc::new(Mutex::new(downlink_cmds_rx)),
        shared,
        closed: Arc::new(Mutex::new(false)),
    };

    (server, client)
}

/// Shared state between server and client connections.
#[derive(Default)]
struct PipeSharedState {
    server_err: Option<String>,
    client_err: Option<String>,
}

/// Server side of a pipe connection.
/// Implements UplinkRx (receive from client) and DownlinkTx (send to client).
pub struct PipeServerConn {
    // Uplink channels (receive from client)
    uplink_opus: Arc<Mutex<mpsc::Receiver<StampedOpusFrame>>>,
    uplink_states: Arc<Mutex<mpsc::Receiver<GearStateEvent>>>,
    uplink_stats: Arc<Mutex<mpsc::Receiver<GearStatsEvent>>>,

    // Downlink channels (send to client)
    downlink_opus: mpsc::Sender<StampedOpusFrame>,
    downlink_cmds: mpsc::Sender<SessionCommandEvent>,

    shared: Arc<RwLock<PipeSharedState>>,
    latest_stats: Arc<Mutex<Option<GearStatsEvent>>>,
    closed: Arc<Mutex<bool>>,
}

impl PipeServerConn {
    /// Returns the uplink opus receiver for direct access.
    pub fn uplink_opus(&self) -> Arc<Mutex<mpsc::Receiver<StampedOpusFrame>>> {
        self.uplink_opus.clone()
    }

    /// Returns the uplink states receiver for direct access.
    pub fn uplink_states(&self) -> Arc<Mutex<mpsc::Receiver<GearStateEvent>>> {
        self.uplink_states.clone()
    }

    /// Returns the uplink stats receiver for direct access.
    pub fn uplink_stats(&self) -> Arc<Mutex<mpsc::Receiver<GearStatsEvent>>> {
        self.uplink_stats.clone()
    }

    /// Closes the connection with an optional error.
    pub async fn close_with_error(&self, err: Option<String>) -> Result<(), ConnError> {
        let mut closed = self.closed.lock().await;
        if *closed {
            return Ok(());
        }
        *closed = true;

        // Set server error in shared state
        let mut shared = self.shared.write().await;
        shared.server_err = err;

        Ok(())
    }
}

#[async_trait]
impl UplinkRx for PipeServerConn {
    async fn recv_opus_frame(&self) -> Result<Option<StampedOpusFrame>, ConnError> {
        let mut rx = self.uplink_opus.lock().await;
        match rx.recv().await {
            Some(frame) => Ok(Some(frame)),
            None => {
                let shared = self.shared.read().await;
                if let Some(ref err) = shared.client_err {
                    Err(ConnError::ReceiveFailed(err.clone()))
                } else {
                    Ok(None)
                }
            }
        }
    }

    async fn recv_state(&self) -> Result<Option<GearStateEvent>, ConnError> {
        let mut rx = self.uplink_states.lock().await;
        match rx.recv().await {
            Some(state) => Ok(Some(state)),
            None => {
                let shared = self.shared.read().await;
                if let Some(ref err) = shared.client_err {
                    Err(ConnError::ReceiveFailed(err.clone()))
                } else {
                    Ok(None)
                }
            }
        }
    }

    async fn recv_stats(&self) -> Result<Option<GearStatsEvent>, ConnError> {
        let mut rx = self.uplink_stats.lock().await;
        match rx.recv().await {
            Some(stats) => {
                *self.latest_stats.lock().await = Some(stats.clone());
                Ok(Some(stats))
            }
            None => {
                let shared = self.shared.read().await;
                if let Some(ref err) = shared.client_err {
                    Err(ConnError::ReceiveFailed(err.clone()))
                } else {
                    Ok(None)
                }
            }
        }
    }

    async fn latest_stats(&self) -> Option<GearStatsEvent> {
        self.latest_stats.lock().await.clone()
    }

    async fn close(&self) -> Result<(), ConnError> {
        self.close_with_error(None).await
    }
}

#[async_trait]
impl DownlinkTx for PipeServerConn {
    async fn send_opus_frame(&self, frame: &StampedOpusFrame) -> Result<(), ConnError> {
        self.downlink_opus
            .send(frame.clone())
            .await
            .map_err(|e| ConnError::SendFailed(e.to_string()))
    }

    async fn send_command(&self, cmd: &SessionCommandEvent) -> Result<(), ConnError> {
        self.downlink_cmds
            .send(cmd.clone())
            .await
            .map_err(|e| ConnError::SendFailed(e.to_string()))
    }

    async fn close(&self) -> Result<(), ConnError> {
        self.close_with_error(None).await
    }
}

/// Client side of a pipe connection.
/// Implements UplinkTx (send to server) and DownlinkRx (receive from server).
pub struct PipeClientConn {
    // Uplink channels (send to server)
    uplink_opus: mpsc::Sender<StampedOpusFrame>,
    uplink_states: mpsc::Sender<GearStateEvent>,
    uplink_stats: mpsc::Sender<GearStatsEvent>,

    // Downlink channels (receive from server)
    downlink_opus: Arc<Mutex<mpsc::Receiver<StampedOpusFrame>>>,
    downlink_cmds: Arc<Mutex<mpsc::Receiver<SessionCommandEvent>>>,

    shared: Arc<RwLock<PipeSharedState>>,
    closed: Arc<Mutex<bool>>,
}

impl PipeClientConn {
    /// Returns the downlink opus receiver for direct access.
    pub fn downlink_opus(&self) -> Arc<Mutex<mpsc::Receiver<StampedOpusFrame>>> {
        self.downlink_opus.clone()
    }

    /// Returns the downlink commands receiver for direct access.
    pub fn downlink_cmds(&self) -> Arc<Mutex<mpsc::Receiver<SessionCommandEvent>>> {
        self.downlink_cmds.clone()
    }

    /// Closes the connection with an optional error.
    pub async fn close_with_error(&self, err: Option<String>) -> Result<(), ConnError> {
        let mut closed = self.closed.lock().await;
        if *closed {
            return Ok(());
        }
        *closed = true;

        // Set client error in shared state
        let mut shared = self.shared.write().await;
        shared.client_err = err;

        Ok(())
    }
}

#[async_trait]
impl UplinkTx for PipeClientConn {
    async fn send_opus_frame(&self, frame: &StampedOpusFrame) -> Result<(), ConnError> {
        self.uplink_opus
            .send(frame.clone())
            .await
            .map_err(|e| ConnError::SendFailed(e.to_string()))
    }

    async fn send_state(&self, state: &GearStateEvent) -> Result<(), ConnError> {
        self.uplink_states
            .send(state.clone())
            .await
            .map_err(|e| ConnError::SendFailed(e.to_string()))
    }

    async fn send_stats(&self, stats: &GearStatsEvent) -> Result<(), ConnError> {
        self.uplink_stats
            .send(stats.clone())
            .await
            .map_err(|e| ConnError::SendFailed(e.to_string()))
    }

    async fn close(&self) -> Result<(), ConnError> {
        self.close_with_error(None).await
    }
}

#[async_trait]
impl DownlinkRx for PipeClientConn {
    async fn recv_opus_frame(&self) -> Result<Option<StampedOpusFrame>, ConnError> {
        let mut rx = self.downlink_opus.lock().await;
        match rx.recv().await {
            Some(frame) => Ok(Some(frame)),
            None => {
                let shared = self.shared.read().await;
                if let Some(ref err) = shared.server_err {
                    Err(ConnError::ReceiveFailed(err.clone()))
                } else {
                    Ok(None)
                }
            }
        }
    }

    async fn recv_command(&self) -> Result<Option<SessionCommandEvent>, ConnError> {
        let mut rx = self.downlink_cmds.lock().await;
        match rx.recv().await {
            Some(cmd) => Ok(Some(cmd)),
            None => {
                let shared = self.shared.read().await;
                if let Some(ref err) = shared.server_err {
                    Err(ConnError::ReceiveFailed(err.clone()))
                } else {
                    Ok(None)
                }
            }
        }
    }

    async fn close(&self) -> Result<(), ConnError> {
        self.close_with_error(None).await
    }
}

#[cfg(test)]
mod pipe_tests {
    use super::*;
    use crate::{GearState, SetVolume, UplinkRx, UplinkTx, DownlinkRx, DownlinkTx};

    #[tokio::test]
    async fn test_pipe_creation() {
        let (server, client) = new_pipe();
        // Just test that they can be created
        drop(server);
        drop(client);
    }

    #[tokio::test]
    async fn test_pipe_opus_client_to_server() {
        let (server, client) = new_pipe();

        // Client sends stamped opus frame
        let frame = StampedOpusFrame::now(vec![0xFC, 0x01, 0x02, 0x03]);
        client.send_opus_frame(&frame).await.unwrap();

        // Server receives it
        let received = server.recv_opus_frame().await.unwrap().unwrap();
        assert_eq!(received.frame, frame.frame);
    }

    #[tokio::test]
    async fn test_pipe_opus_server_to_client() {
        let (server, client) = new_pipe();

        // Server sends stamped opus frame
        let frame = StampedOpusFrame::now(vec![0xFC, 0x04, 0x05, 0x06]);
        server.send_opus_frame(&frame).await.unwrap();

        // Client receives it
        let received = client.recv_opus_frame().await.unwrap().unwrap();
        assert_eq!(received.frame, frame.frame);
    }

    #[tokio::test]
    async fn test_pipe_state_event() {
        let (server, client) = new_pipe();

        // Client sends state
        let state = GearStateEvent::new(GearState::Recording);
        client.send_state(&state).await.unwrap();

        // Server receives it
        let received = server.recv_state().await.unwrap().unwrap();
        assert_eq!(received.state, GearState::Recording);
    }

    #[tokio::test]
    async fn test_pipe_stats_event() {
        let (server, client) = new_pipe();

        // Client sends stats
        let mut stats = GearStatsEvent::new();
        stats.volume = Some(crate::Volume {
            percentage: 75.0,
            update_at: giztoy_jsontime::Milli::now(),
        });
        client.send_stats(&stats).await.unwrap();

        // Server receives it
        let received = server.recv_stats().await.unwrap().unwrap();
        assert_eq!(received.volume.unwrap().percentage, 75.0);
    }

    #[tokio::test]
    async fn test_pipe_command() {
        let (server, client) = new_pipe();

        // Server sends command
        let cmd = SetVolume::new(50);
        let event = SessionCommandEvent::new(&cmd);
        server.send_command(&event).await.unwrap();

        // Client receives it
        let received = client.recv_command().await.unwrap().unwrap();
        assert_eq!(received.cmd_type, "set_volume");
    }

    #[tokio::test]
    async fn test_pipe_close() {
        let (server, client) = new_pipe();

        // Close client
        client.close_with_error(None).await.unwrap();

        // Close server
        server.close_with_error(None).await.unwrap();

        // Both should be marked as closed
        // Note: In Rust, recv() only returns None when all senders are dropped,
        // which doesn't happen until the struct is dropped.
    }

    #[tokio::test]
    async fn test_pipe_close_with_error() {
        let (_server, client) = new_pipe();

        // Close client with error
        client
            .close_with_error(Some("test error".to_string()))
            .await
            .unwrap();

        // Verify double close is idempotent
        client.close_with_error(None).await.unwrap();
    }

    #[tokio::test]
    async fn test_pipe_bidirectional() {
        let (server, client) = new_pipe();

        // Simultaneous communication
        let client_frame = StampedOpusFrame::now(vec![0x01, 0x02]);
        let server_frame = StampedOpusFrame::now(vec![0x03, 0x04]);

        client.send_opus_frame(&client_frame).await.unwrap();
        server.send_opus_frame(&server_frame).await.unwrap();

        let from_client = server.recv_opus_frame().await.unwrap().unwrap();
        let from_server = client.recv_opus_frame().await.unwrap().unwrap();

        assert_eq!(from_client.frame, client_frame.frame);
        assert_eq!(from_server.frame, server_frame.frame);
    }
}
