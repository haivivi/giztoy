//! Client port implementation.
//!
//! ClientPort manages client-side audio input/output and command handling.

use crate::{
    ClientPortRx, ClientPortTx, DownlinkRx, GearStateEvent, GearStatsEvent,
    PortError, SessionCommandEvent, StampedOpusFrame, UplinkTx,
};
use async_trait::async_trait;
use std::sync::Arc;
use tokio::sync::{mpsc, Mutex, RwLock};
use tokio_util::sync::CancellationToken;
use tracing::warn;

/// Client port for managing audio input/output and commands.
pub struct ClientPort<T: UplinkTx> {
    tx: Arc<T>,
    cancel: CancellationToken,

    // Output - audio from server (ClientPortRx)
    opus_frames_rx: Arc<Mutex<mpsc::Receiver<StampedOpusFrame>>>,
    opus_frames_tx: mpsc::Sender<StampedOpusFrame>,
    commands_rx: Arc<Mutex<mpsc::Receiver<SessionCommandEvent>>>,
    commands_tx: mpsc::Sender<SessionCommandEvent>,

    closed: Arc<RwLock<bool>>,
}

impl<T: UplinkTx + 'static> ClientPort<T> {
    /// Creates a new ClientPort.
    pub fn new(tx: T) -> Self {
        let (opus_tx, opus_rx) = mpsc::channel(1024);
        let (cmd_tx, cmd_rx) = mpsc::channel(32);

        Self {
            tx: Arc::new(tx),
            cancel: CancellationToken::new(),
            opus_frames_rx: Arc::new(Mutex::new(opus_rx)),
            opus_frames_tx: opus_tx,
            commands_rx: Arc::new(Mutex::new(cmd_rx)),
            commands_tx: cmd_tx,
            closed: Arc::new(RwLock::new(false)),
        }
    }

    /// Returns the cancellation token.
    pub fn cancellation_token(&self) -> CancellationToken {
        self.cancel.clone()
    }

    /// Handles incoming opus frames from the server.
    pub async fn handle_opus_frame(&self, frame: StampedOpusFrame) {
        if let Err(e) = self.opus_frames_tx.send(frame).await {
            warn!("failed to buffer opus frame: {}", e);
        }
    }

    /// Handles incoming commands from the server.
    pub async fn handle_command(&self, cmd: SessionCommandEvent) {
        let closed = self.closed.read().await;
        if *closed {
            return;
        }

        if let Err(e) = self.commands_tx.send(cmd).await {
            warn!("commands channel full, dropping command: {}", e);
        }
    }

    /// Receives data from the given DownlinkRx until closed.
    /// This spawns background tasks that will process incoming data.
    pub async fn recv_from<R: DownlinkRx + 'static>(&self, rx: Arc<R>) {
        let cancel = self.cancel.clone();
        let opus_tx = self.opus_frames_tx.clone();
        let cmd_tx = self.commands_tx.clone();
        let closed = self.closed.clone();

        // Spawn opus frames receiver
        let rx_opus = rx.clone();
        let cancel_opus = cancel.clone();
        tokio::spawn(async move {
            loop {
                tokio::select! {
                    _ = cancel_opus.cancelled() => break,
                    result = rx_opus.recv_opus_frame() => {
                        match result {
                            Ok(Some(frame)) => {
                                if opus_tx.send(frame).await.is_err() {
                                    break;
                                }
                            }
                            Ok(None) => break,
                            Err(_) => break,
                        }
                    }
                }
            }
        });

        // Spawn commands receiver
        let rx_cmd = rx.clone();
        let cancel_cmd = cancel.clone();
        tokio::spawn(async move {
            loop {
                tokio::select! {
                    _ = cancel_cmd.cancelled() => break,
                    result = rx_cmd.recv_command() => {
                        match result {
                            Ok(Some(cmd)) => {
                                let is_closed = *closed.read().await;
                                if is_closed {
                                    break;
                                }
                                if cmd_tx.send(cmd).await.is_err() {
                                    break;
                                }
                            }
                            Ok(None) => break,
                            Err(_) => break,
                        }
                    }
                }
            }
        });
    }

    /// Closes the port.
    pub async fn close(&self) -> Result<(), PortError> {
        self.cancel.cancel();
        *self.closed.write().await = true;
        self.tx.close().await.map_err(|e| PortError::Other(e.to_string()))
    }
}

#[async_trait]
impl<T: UplinkTx + 'static> ClientPortTx for ClientPort<T> {
    async fn send_opus_frame(&self, frame: &StampedOpusFrame) -> Result<(), PortError> {
        self.tx
            .send_opus_frame(frame)
            .await
            .map_err(|e| PortError::SendFailed(e.to_string()))
    }

    async fn send_state(&self, state: &GearStateEvent) -> Result<(), PortError> {
        self.tx
            .send_state(state)
            .await
            .map_err(|e| PortError::SendFailed(e.to_string()))
    }

    async fn send_stats(&self, stats: &GearStatsEvent) -> Result<(), PortError> {
        self.tx
            .send_stats(stats)
            .await
            .map_err(|e| PortError::SendFailed(e.to_string()))
    }
}

#[async_trait]
impl<T: UplinkTx + 'static> ClientPortRx for ClientPort<T> {
    async fn recv_opus_frame(&self) -> Option<StampedOpusFrame> {
        self.opus_frames_rx.lock().await.recv().await
    }

    async fn recv_command(&self) -> Option<SessionCommandEvent> {
        self.commands_rx.lock().await.recv().await
    }
}

#[cfg(test)]
mod client_port_tests {
    use super::*;
    use crate::{conn_pipe::new_pipe, GearState, UplinkRx};

    #[tokio::test]
    async fn test_client_port_send_state() {
        let (server, client) = new_pipe();
        let port = ClientPort::new(client);

        let state = GearStateEvent::new(GearState::Recording);
        port.send_state(&state).await.unwrap();

        let received = server.recv_state().await.unwrap().unwrap();
        assert_eq!(received.state, GearState::Recording);
    }

    #[tokio::test]
    async fn test_client_port_send_stats() {
        let (server, client) = new_pipe();
        let port = ClientPort::new(client);

        let mut stats = GearStatsEvent::new();
        stats.volume = Some(crate::Volume {
            percentage: 50.0,
            update_at: giztoy_jsontime::Milli::now(),
        });
        port.send_stats(&stats).await.unwrap();

        let received = server.recv_stats().await.unwrap().unwrap();
        assert_eq!(received.volume.unwrap().percentage, 50.0);
    }

    #[tokio::test]
    async fn test_client_port_send_opus() {
        let (server, client) = new_pipe();
        let port = ClientPort::new(client);

        let frame = StampedOpusFrame::now(vec![0x01, 0x02, 0x03]);
        port.send_opus_frame(&frame).await.unwrap();

        let received = server.recv_opus_frame().await.unwrap().unwrap();
        assert_eq!(received.frame, frame.frame);
    }

    #[tokio::test]
    async fn test_client_port_handle_command() {
        let (_server, client) = new_pipe();
        let port = ClientPort::new(client);

        // Manually handle a command
        let cmd = crate::SetVolume::new(75);
        let event = crate::SessionCommandEvent::new(&cmd);
        port.handle_command(event.clone()).await;

        // Use the async trait method
        let received = port.recv_command().await.unwrap();
        assert_eq!(received.cmd_type, "set_volume");

        port.close().await.unwrap();
    }

    #[tokio::test]
    async fn test_client_port_close() {
        let (_server, client) = new_pipe();
        let port = ClientPort::new(client);
        port.close().await.unwrap();
    }
}
