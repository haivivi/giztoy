//! Port traits for client/server audio and command handling.

use crate::{
    ConnectedCellular, ConnectedWifi, GearStateEvent, GearStatsChanges, GearStatsEvent,
    ReadNFCTag, SessionCommandEvent, StoredWifiList, OTA,
};
use async_trait::async_trait;
use tokio::sync::mpsc;

/// Error type for port operations.
#[derive(Debug, thiserror::Error)]
pub enum PortError {
    #[error("port closed")]
    Closed,
    #[error("send failed: {0}")]
    SendFailed(String),
    #[error("receive failed: {0}")]
    ReceiveFailed(String),
    #[error("command failed: {0}")]
    CommandFailed(String),
    #[error("track creation failed: {0}")]
    TrackCreationFailed(String),
    #[error("other error: {0}")]
    Other(String),
}

// =============================================================================
// Client Port Interfaces
// =============================================================================

/// Transmit side of a client port (client to server).
/// Sends audio frames, state events, and stats events to the server.
#[async_trait]
pub trait ClientPortTx: Send + Sync {
    /// Sends a stamped opus frame to the server.
    async fn send_opus_frames(&self, stamped_opus_frame: &[u8]) -> Result<(), PortError>;

    /// Sends a state event to the server.
    async fn send_state(&self, state: &GearStateEvent) -> Result<(), PortError>;

    /// Sends a stats event to the server.
    async fn send_stats(&self, stats: &GearStatsEvent) -> Result<(), PortError>;
}

/// Receive side of a client port (server to client).
/// Receives audio frames and commands from the server.
pub trait ClientPortRx: Send + Sync {
    /// Returns a receiver for opus frames from the server.
    fn opus_frames(&self) -> &mpsc::Receiver<Vec<u8>>;

    /// Returns a receiver for commands from the server.
    fn commands(&self) -> &mpsc::Receiver<SessionCommandEvent>;
}

// =============================================================================
// Server Port Interfaces
// =============================================================================

/// Track handle for audio output.
pub trait AudioTrack: Send + Sync {
    /// Writes PCM samples to the track.
    fn write(&self, samples: &[i16]) -> Result<usize, PortError>;

    /// Returns true if the track is active.
    fn is_active(&self) -> bool;

    /// Stops the track.
    fn stop(&self);
}

/// Track controller for managing audio tracks.
pub trait AudioTrackCtrl: Send + Sync {
    /// Pauses the track.
    fn pause(&self);

    /// Resumes the track.
    fn resume(&self);

    /// Stops the track.
    fn stop(&self);

    /// Returns true if the track is playing.
    fn is_playing(&self) -> bool;
}

/// Transmit side of a server port (server to client).
/// Sends audio frames and commands to the client.
#[async_trait]
pub trait ServerPortTx: Send + Sync {
    // --- Audio Output ---

    /// Creates a new background audio track.
    fn new_background_track(&self) -> Result<(Box<dyn AudioTrack>, Box<dyn AudioTrackCtrl>), PortError>;

    /// Creates a new foreground audio track.
    fn new_foreground_track(&self) -> Result<(Box<dyn AudioTrack>, Box<dyn AudioTrackCtrl>), PortError>;

    /// Creates a new overlay audio track.
    fn new_overlay_track(&self) -> Result<(Box<dyn AudioTrack>, Box<dyn AudioTrackCtrl>), PortError>;

    /// Returns the current background track controller.
    fn background_track_ctrl(&self) -> Option<&dyn AudioTrackCtrl>;

    /// Returns the current foreground track controller.
    fn foreground_track_ctrl(&self) -> Option<&dyn AudioTrackCtrl>;

    /// Returns the current overlay track controller.
    fn overlay_track_ctrl(&self) -> Option<&dyn AudioTrackCtrl>;

    /// Stops all output tracks immediately.
    fn interrupt(&self);

    // --- Device Commands ---

    /// Sets the volume of the device.
    async fn set_volume(&self, volume: i32) -> Result<(), PortError>;

    /// Sets the light mode of the device.
    async fn set_light_mode(&self, mode: &str) -> Result<(), PortError>;

    /// Sets the brightness of the device.
    async fn set_brightness(&self, brightness: i32) -> Result<(), PortError>;

    /// Sets the WiFi network of the device.
    async fn set_wifi(&self, ssid: &str, password: &str) -> Result<(), PortError>;

    /// Deletes a stored WiFi network.
    async fn delete_wifi(&self, ssid: &str) -> Result<(), PortError>;

    /// Resets the device.
    async fn reset(&self) -> Result<(), PortError>;

    /// Unpairs the device.
    async fn unpair(&self) -> Result<(), PortError>;

    /// Puts the device to sleep.
    async fn sleep(&self) -> Result<(), PortError>;

    /// Shuts down the device.
    async fn shutdown(&self) -> Result<(), PortError>;

    /// Raises a call on the device.
    async fn raise_call(&self) -> Result<(), PortError>;

    /// Initiates an OTA firmware upgrade.
    async fn upgrade_firmware(&self, ota: OTA) -> Result<(), PortError>;
}

/// Receive side of a server port (client to server).
/// Receives audio frames, state events, and stats changes from the client.
pub trait ServerPortRx: Send + Sync {
    /// Returns a receiver for opus frames from the client.
    fn opus_frames(&self) -> &mpsc::Receiver<Vec<u8>>;

    /// Returns a receiver for state events from the client.
    fn state_events(&self) -> &mpsc::Receiver<GearStateEvent>;

    /// Returns a receiver for stats change events from the client.
    fn stats_changes(&self) -> &mpsc::Receiver<GearStatsChanges>;

    /// Returns the current gear state.
    fn gear_state(&self) -> Option<GearStateEvent>;

    /// Returns the current gear stats.
    fn gear_stats(&self) -> Option<GearStatsEvent>;

    /// Returns the current volume percentage.
    fn volume(&self) -> Option<i32>;

    /// Returns the current light mode.
    fn light_mode(&self) -> Option<String>;

    /// Returns the current brightness percentage.
    fn brightness(&self) -> Option<i32>;

    /// Returns the current connected WiFi network.
    fn wifi_network(&self) -> Option<ConnectedWifi>;

    /// Returns the stored WiFi list.
    fn wifi_store(&self) -> Option<StoredWifiList>;

    /// Returns the current battery status.
    fn battery(&self) -> Option<(i32, bool)>; // (percentage, is_charging)

    /// Returns the current system version.
    fn system_version(&self) -> Option<String>;

    /// Returns the current cellular network.
    fn cellular(&self) -> Option<ConnectedCellular>;

    /// Returns the current pair status.
    fn pair_status(&self) -> Option<String>;

    /// Returns the last read NFC tags.
    fn read_nfc_tag(&self) -> Option<ReadNFCTag>;

    /// Returns the current shaking level.
    fn shaking(&self) -> Option<f64>;
}

// =============================================================================
// Combined Port Traits
// =============================================================================

/// A complete client port (both tx and rx sides).
pub trait ClientPort: ClientPortTx + ClientPortRx {}

/// A complete server port (both tx and rx sides).
pub trait ServerPort: ServerPortTx + ServerPortRx {}

#[cfg(test)]
mod port_tests {
    use super::*;

    #[test]
    fn test_port_error_display() {
        let err = PortError::Closed;
        assert_eq!(err.to_string(), "port closed");

        let err = PortError::CommandFailed("test".to_string());
        assert!(err.to_string().contains("test"));
    }
}
