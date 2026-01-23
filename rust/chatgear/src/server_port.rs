//! Server port implementation.
//!
//! ServerPort manages server-side audio input/output, state/stats tracking, and device commands.

use crate::{
    ConnectedCellular, ConnectedWifi, DownlinkTx, GearStateEvent, GearStatsChanges,
    GearStatsEvent, PortError, ReadNFCTag, ServerPortRx, ServerPortTx, SessionCommand,
    SessionCommandEvent, StoredWifiList, UplinkRx, OTA,
    DeleteWifi, Halt, Raise, Reset, SetBrightness, SetLightMode, SetVolume, SetWifi,
};
use serde::Serialize;
use crate::port::AudioTrack;
use crate::port::AudioTrackCtrl;
use async_trait::async_trait;
use std::sync::Arc;
use tokio::sync::{mpsc, Mutex, RwLock};
use tokio_util::sync::CancellationToken;
use tracing::warn;

/// Server port for managing audio input/output and device state.
pub struct ServerPort<T: DownlinkTx> {
    tx: Arc<T>,
    cancel: CancellationToken,

    // Input - audio from device
    opus_frames_rx: Arc<Mutex<mpsc::Receiver<Vec<u8>>>>,
    opus_frames_tx: mpsc::Sender<Vec<u8>>,

    // Stats & State
    gear_stats: Arc<RwLock<Option<GearStatsEvent>>>,
    gear_state: Arc<RwLock<Option<GearStateEvent>>>,

    // Events
    stats_changes_rx: Arc<Mutex<mpsc::Receiver<GearStatsChanges>>>,
    stats_changes_tx: mpsc::Sender<GearStatsChanges>,
    state_events_rx: Arc<Mutex<mpsc::Receiver<GearStateEvent>>>,
    state_events_tx: mpsc::Sender<GearStateEvent>,

    closed: Arc<RwLock<bool>>,
}

impl<T: DownlinkTx + 'static> ServerPort<T> {
    /// Creates a new ServerPort.
    pub fn new(tx: T) -> Self {
        let (opus_tx, opus_rx) = mpsc::channel(1024);
        let (stats_tx, stats_rx) = mpsc::channel(32);
        let (state_tx, state_rx) = mpsc::channel(32);

        Self {
            tx: Arc::new(tx),
            cancel: CancellationToken::new(),
            opus_frames_rx: Arc::new(Mutex::new(opus_rx)),
            opus_frames_tx: opus_tx,
            gear_stats: Arc::new(RwLock::new(None)),
            gear_state: Arc::new(RwLock::new(None)),
            stats_changes_rx: Arc::new(Mutex::new(stats_rx)),
            stats_changes_tx: stats_tx,
            state_events_rx: Arc::new(Mutex::new(state_rx)),
            state_events_tx: state_tx,
            closed: Arc::new(RwLock::new(false)),
        }
    }

    /// Returns the cancellation token.
    pub fn cancellation_token(&self) -> CancellationToken {
        self.cancel.clone()
    }

    /// Returns the opus frames receiver.
    pub fn opus_frames_receiver(&self) -> Arc<Mutex<mpsc::Receiver<Vec<u8>>>> {
        self.opus_frames_rx.clone()
    }

    /// Returns the stats changes receiver.
    pub fn stats_changes_receiver(&self) -> Arc<Mutex<mpsc::Receiver<GearStatsChanges>>> {
        self.stats_changes_rx.clone()
    }

    /// Returns the state events receiver.
    pub fn state_events_receiver(&self) -> Arc<Mutex<mpsc::Receiver<GearStateEvent>>> {
        self.state_events_rx.clone()
    }

    /// Handles incoming opus frames from the device.
    pub async fn handle_opus_frames(&self, stamped_frame: Vec<u8>) {
        if let Err(e) = self.opus_frames_tx.send(stamped_frame).await {
            warn!("failed to buffer opus frame: {}", e);
        }
    }

    /// Handles incoming state events from the device.
    pub async fn handle_state_event(&self, event: GearStateEvent) {
        let closed = self.closed.read().await;
        if *closed {
            return;
        }

        // Filter out-of-order events
        {
            let current = self.gear_state.read().await;
            if let Some(ref current_state) = *current {
                if event.time < current_state.time {
                    return;
                }
            }
        }

        // Update state
        *self.gear_state.write().await = Some(event.clone());

        // Send event
        if let Err(e) = self.state_events_tx.send(event).await {
            warn!("state events channel full: {}", e);
        }
    }

    /// Handles incoming stats events from the device.
    pub async fn handle_stats_event(&self, event: GearStatsEvent) {
        let closed = self.closed.read().await;
        if *closed {
            return;
        }

        let changes = {
            let mut current = self.gear_stats.write().await;
            if let Some(ref mut current_stats) = *current {
                current_stats.merge_with(&event)
            } else {
                *current = Some(event);
                None
            }
        };

        if let Some(changes) = changes {
            if let Err(e) = self.stats_changes_tx.send(changes).await {
                warn!("stats changes channel full: {}", e);
            }
        }
    }

    /// Receives data from the given UplinkRx until closed.
    pub async fn recv_from<R: UplinkRx + 'static>(&self, rx: Arc<R>) {
        let cancel = self.cancel.clone();

        // Spawn opus frames receiver
        let rx_opus = rx.clone();
        let cancel_opus = cancel.clone();
        let opus_tx = self.opus_frames_tx.clone();
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

        // Spawn state receiver
        let rx_state = rx.clone();
        let cancel_state = cancel.clone();
        let gear_state = self.gear_state.clone();
        let state_tx = self.state_events_tx.clone();
        let closed = self.closed.clone();
        tokio::spawn(async move {
            loop {
                tokio::select! {
                    _ = cancel_state.cancelled() => break,
                    result = rx_state.recv_state() => {
                        match result {
                            Ok(Some(event)) => {
                                if *closed.read().await {
                                    break;
                                }
                                // Filter out-of-order
                                {
                                    let current = gear_state.read().await;
                                    if let Some(ref c) = *current {
                                        if event.time < c.time {
                                            continue;
                                        }
                                    }
                                }
                                *gear_state.write().await = Some(event.clone());
                                if state_tx.send(event).await.is_err() {
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

        // Spawn stats receiver
        let rx_stats = rx.clone();
        let cancel_stats = cancel.clone();
        let gear_stats = self.gear_stats.clone();
        let stats_tx = self.stats_changes_tx.clone();
        let closed = self.closed.clone();
        tokio::spawn(async move {
            loop {
                tokio::select! {
                    _ = cancel_stats.cancelled() => break,
                    result = rx_stats.recv_stats() => {
                        match result {
                            Ok(Some(event)) => {
                                if *closed.read().await {
                                    break;
                                }
                                let changes = {
                                    let mut current = gear_stats.write().await;
                                    if let Some(ref mut c) = *current {
                                        c.merge_with(&event)
                                    } else {
                                        *current = Some(event);
                                        None
                                    }
                                };
                                if let Some(changes) = changes {
                                    if stats_tx.send(changes).await.is_err() {
                                        break;
                                    }
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

    /// Sends a command to the device.
    async fn send_command<C: SessionCommand + Serialize>(&self, cmd: &C) -> Result<(), PortError> {
        let event = SessionCommandEvent::new(cmd);
        self.tx
            .send_command(&event)
            .await
            .map_err(|e| PortError::CommandFailed(e.to_string()))
    }

    /// Closes the port.
    pub async fn close(&self) -> Result<(), PortError> {
        self.cancel.cancel();
        *self.closed.write().await = true;
        self.tx.close().await.map_err(|e| PortError::Other(e.to_string()))
    }
}

#[async_trait]
impl<T: DownlinkTx + 'static> ServerPortTx for ServerPort<T> {
    fn new_background_track(&self) -> Result<(Box<dyn AudioTrack>, Box<dyn AudioTrackCtrl>), PortError> {
        // Simplified - in real implementation, this would create mixer tracks
        Err(PortError::TrackCreationFailed("not implemented".to_string()))
    }

    fn new_foreground_track(&self) -> Result<(Box<dyn AudioTrack>, Box<dyn AudioTrackCtrl>), PortError> {
        Err(PortError::TrackCreationFailed("not implemented".to_string()))
    }

    fn new_overlay_track(&self) -> Result<(Box<dyn AudioTrack>, Box<dyn AudioTrackCtrl>), PortError> {
        Err(PortError::TrackCreationFailed("not implemented".to_string()))
    }

    fn background_track_ctrl(&self) -> Option<&dyn AudioTrackCtrl> {
        None
    }

    fn foreground_track_ctrl(&self) -> Option<&dyn AudioTrackCtrl> {
        None
    }

    fn overlay_track_ctrl(&self) -> Option<&dyn AudioTrackCtrl> {
        None
    }

    fn interrupt(&self) {
        // No-op in simplified implementation
    }

    async fn set_volume(&self, volume: i32) -> Result<(), PortError> {
        let cmd = SetVolume::new(volume);
        self.send_command(&cmd).await
    }

    async fn set_light_mode(&self, mode: &str) -> Result<(), PortError> {
        let cmd = SetLightMode::new(mode);
        self.send_command(&cmd).await
    }

    async fn set_brightness(&self, brightness: i32) -> Result<(), PortError> {
        let cmd = SetBrightness::new(brightness);
        self.send_command(&cmd).await
    }

    async fn set_wifi(&self, ssid: &str, password: &str) -> Result<(), PortError> {
        let cmd = SetWifi::new(ssid, "WPA2", password);
        self.send_command(&cmd).await
    }

    async fn delete_wifi(&self, ssid: &str) -> Result<(), PortError> {
        let cmd = DeleteWifi::new(ssid);
        self.send_command(&cmd).await
    }

    async fn reset(&self) -> Result<(), PortError> {
        let cmd = Reset::new();
        self.send_command(&cmd).await
    }

    async fn unpair(&self) -> Result<(), PortError> {
        let cmd = Reset::with_unpair();
        self.send_command(&cmd).await
    }

    async fn sleep(&self) -> Result<(), PortError> {
        let cmd = Halt::sleep();
        self.send_command(&cmd).await
    }

    async fn shutdown(&self) -> Result<(), PortError> {
        let cmd = Halt::shutdown();
        self.send_command(&cmd).await
    }

    async fn raise_call(&self) -> Result<(), PortError> {
        let cmd = Raise::call();
        self.send_command(&cmd).await
    }

    async fn upgrade_firmware(&self, ota: OTA) -> Result<(), PortError> {
        self.send_command(&ota).await
    }
}

impl<T: DownlinkTx + 'static> ServerPortRx for ServerPort<T> {
    fn opus_frames(&self) -> &mpsc::Receiver<Vec<u8>> {
        unimplemented!("use opus_frames_receiver() instead")
    }

    fn state_events(&self) -> &mpsc::Receiver<GearStateEvent> {
        unimplemented!("use state_events_receiver() instead")
    }

    fn stats_changes(&self) -> &mpsc::Receiver<GearStatsChanges> {
        unimplemented!("use stats_changes_receiver() instead")
    }

    fn gear_state(&self) -> Option<GearStateEvent> {
        // Blocking in async context - use gear_state_async in async code
        None
    }

    fn gear_stats(&self) -> Option<GearStatsEvent> {
        None
    }

    fn volume(&self) -> Option<i32> {
        None
    }

    fn light_mode(&self) -> Option<String> {
        None
    }

    fn brightness(&self) -> Option<i32> {
        None
    }

    fn wifi_network(&self) -> Option<ConnectedWifi> {
        None
    }

    fn wifi_store(&self) -> Option<StoredWifiList> {
        None
    }

    fn battery(&self) -> Option<(i32, bool)> {
        None
    }

    fn system_version(&self) -> Option<String> {
        None
    }

    fn cellular(&self) -> Option<ConnectedCellular> {
        None
    }

    fn pair_status(&self) -> Option<String> {
        None
    }

    fn read_nfc_tag(&self) -> Option<ReadNFCTag> {
        None
    }

    fn shaking(&self) -> Option<f64> {
        None
    }
}

// Async getters for ServerPort
impl<T: DownlinkTx + 'static> ServerPort<T> {
    /// Returns the current gear state (async version).
    pub async fn gear_state_async(&self) -> Option<GearStateEvent> {
        self.gear_state.read().await.clone()
    }

    /// Returns the current gear stats (async version).
    pub async fn gear_stats_async(&self) -> Option<GearStatsEvent> {
        self.gear_stats.read().await.clone()
    }

    /// Returns the current volume percentage.
    pub async fn volume_async(&self) -> Option<i32> {
        let stats = self.gear_stats.read().await;
        stats.as_ref()?.volume.as_ref().map(|v| v.percentage as i32)
    }

    /// Returns the current light mode.
    pub async fn light_mode_async(&self) -> Option<String> {
        let stats = self.gear_stats.read().await;
        stats.as_ref()?.light_mode.as_ref().map(|l| l.mode.clone())
    }

    /// Returns the current brightness percentage.
    pub async fn brightness_async(&self) -> Option<i32> {
        let stats = self.gear_stats.read().await;
        stats.as_ref()?.brightness.as_ref().map(|b| b.percentage as i32)
    }

    /// Returns the current battery status (percentage, is_charging).
    pub async fn battery_async(&self) -> Option<(i32, bool)> {
        let stats = self.gear_stats.read().await;
        stats.as_ref()?.battery.as_ref().map(|b| (b.percentage as i32, b.is_charging))
    }

    /// Returns the current WiFi network.
    pub async fn wifi_network_async(&self) -> Option<ConnectedWifi> {
        let stats = self.gear_stats.read().await;
        stats.as_ref()?.wifi_network.clone()
    }

    /// Returns the shaking level.
    pub async fn shaking_async(&self) -> Option<f64> {
        let stats = self.gear_stats.read().await;
        stats.as_ref()?.shaking.as_ref().map(|s| s.level)
    }
}

#[cfg(test)]
mod server_port_tests {
    use super::*;
    use crate::conn_pipe::new_pipe;
    use crate::{GearState, UplinkTx, DownlinkRx};

    #[tokio::test]
    async fn test_server_port_send_command() {
        let (server, client) = new_pipe();
        let port = ServerPort::new(server);

        // Send set_volume command
        port.set_volume(75).await.unwrap();

        // Client receives it
        let received = client.recv_command().await.unwrap().unwrap();
        assert_eq!(received.cmd_type, "set_volume");
    }

    #[tokio::test]
    async fn test_server_port_receive_state() {
        let (server, client) = new_pipe();
        let port = ServerPort::new(server);

        // Client sends state
        let state = GearStateEvent::new(GearState::Recording);
        client.send_state(&state).await.unwrap();

        // Use handle_state_event directly
        port.handle_state_event(state.clone()).await;

        // Check state
        let current = port.gear_state_async().await.unwrap();
        assert_eq!(current.state, GearState::Recording);
    }

    #[tokio::test]
    async fn test_server_port_receive_stats() {
        let (server, _client) = new_pipe();
        let port = ServerPort::new(server);

        // Send stats
        let mut stats = GearStatsEvent::new();
        stats.volume = Some(crate::Volume {
            percentage: 80.0,
            update_at: giztoy_jsontime::Milli::now(),
        });

        port.handle_stats_event(stats).await;

        // Check volume
        let vol = port.volume_async().await.unwrap();
        assert_eq!(vol, 80);
    }

    #[tokio::test]
    async fn test_server_port_stats_merge() {
        let (server, _client) = new_pipe();
        let port = ServerPort::new(server);

        // First stats
        let mut stats1 = GearStatsEvent::new();
        stats1.volume = Some(crate::Volume {
            percentage: 50.0,
            update_at: giztoy_jsontime::Milli::now(),
        });
        port.handle_stats_event(stats1).await;

        // Second stats with battery
        let mut stats2 = GearStatsEvent::new();
        stats2.battery = Some(crate::Battery {
            percentage: 90.0,
            is_charging: true,
            ..Default::default()
        });
        port.handle_stats_event(stats2).await;

        // Both should be present
        let vol = port.volume_async().await.unwrap();
        assert_eq!(vol, 50);
        let (bat, charging) = port.battery_async().await.unwrap();
        assert_eq!(bat, 90);
        assert!(charging);
    }

    #[tokio::test]
    async fn test_server_port_all_commands() {
        let (server, client) = new_pipe();
        let port = ServerPort::new(server);

        // Test all command methods
        port.set_volume(50).await.unwrap();
        let cmd = client.recv_command().await.unwrap().unwrap();
        assert_eq!(cmd.cmd_type, "set_volume");

        port.set_brightness(80).await.unwrap();
        let cmd = client.recv_command().await.unwrap().unwrap();
        assert_eq!(cmd.cmd_type, "set_brightness");

        port.set_light_mode("dark").await.unwrap();
        let cmd = client.recv_command().await.unwrap().unwrap();
        assert_eq!(cmd.cmd_type, "set_light_mode");

        port.set_wifi("TestSSID", "password").await.unwrap();
        let cmd = client.recv_command().await.unwrap().unwrap();
        assert_eq!(cmd.cmd_type, "set_wifi");

        port.delete_wifi("TestSSID").await.unwrap();
        let cmd = client.recv_command().await.unwrap().unwrap();
        assert_eq!(cmd.cmd_type, "delete_wifi");

        port.reset().await.unwrap();
        let cmd = client.recv_command().await.unwrap().unwrap();
        assert_eq!(cmd.cmd_type, "reset");

        port.sleep().await.unwrap();
        let cmd = client.recv_command().await.unwrap().unwrap();
        assert_eq!(cmd.cmd_type, "halt");

        port.shutdown().await.unwrap();
        let cmd = client.recv_command().await.unwrap().unwrap();
        assert_eq!(cmd.cmd_type, "halt");

        port.raise_call().await.unwrap();
        let cmd = client.recv_command().await.unwrap().unwrap();
        assert_eq!(cmd.cmd_type, "raise");
    }

    #[tokio::test]
    async fn test_server_port_close() {
        let (server, _client) = new_pipe();
        let port = ServerPort::new(server);
        port.close().await.unwrap();
    }
}
