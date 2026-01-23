//! Device statistics types.

use giztoy_encoding::StdBase64Data;
use giztoy_jsontime::Milli;
use serde::{Deserialize, Serialize};

/// Contains device statistics.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct GearStatsEvent {
    /// Event timestamp.
    pub time: Milli,

    /// Last device reset time.
    #[serde(default, skip_serializing_if = "Milli::is_zero")]
    pub last_reset_at: Milli,

    /// Battery status.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub battery: Option<Battery>,

    /// System version information.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub system_version: Option<SystemVersion>,

    /// Audio volume settings.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub volume: Option<Volume>,

    /// Display brightness settings.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub brightness: Option<Brightness>,

    /// Light mode settings.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub light_mode: Option<LightMode>,

    /// Cellular network information.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cellular_network: Option<ConnectedCellular>,

    /// WiFi network information.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub wifi_network: Option<ConnectedWifi>,

    /// Stored WiFi list.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub wifi_store: Option<StoredWifiList>,

    /// NFC tag read data.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub read_nfc_tag: Option<ReadNFCTag>,

    /// Device pairing status.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub pair_status: Option<PairStatus>,

    /// Device shaking detection data.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub shaking: Option<Shaking>,
}

impl GearStatsEvent {
    /// Creates a new empty stats event.
    pub fn new() -> Self {
        Self {
            time: Milli::now(),
            ..Default::default()
        }
    }

    /// Returns a deep copy of the stats event.
    pub fn clone_deep(&self) -> Self {
        self.clone()
    }

    /// Merges another stats event into this one.
    /// Returns the changes if any fields were updated.
    pub fn merge_with(&mut self, other: &GearStatsEvent) -> Option<GearStatsChanges> {
        if other.time.before(&self.time) {
            return None;
        }
        self.time = other.time;

        let mut diff = GearStatsChanges::default();
        let mut has_changes = false;

        // Last reset at
        if other.last_reset_at.after(&self.last_reset_at) {
            self.last_reset_at = other.last_reset_at;
            diff.last_reset_at = Some(other.last_reset_at);
            has_changes = true;
        }

        // System version
        if let Some(ref other_sv) = other.system_version {
            if self.system_version.is_none()
                || !other_sv.update_at.before(&self.system_version.as_ref().unwrap().update_at)
            {
                self.system_version = Some(other_sv.clone());
                diff.system_version = Some(other_sv.clone());
                has_changes = true;
            }
        }

        // Volume
        if let Some(ref other_vol) = other.volume {
            if self.volume.is_none()
                || !other_vol.update_at.before(&self.volume.as_ref().unwrap().update_at)
            {
                self.volume = Some(other_vol.clone());
                diff.volume = Some(other_vol.clone());
                has_changes = true;
            }
        }

        // Brightness
        if let Some(ref other_br) = other.brightness {
            if self.brightness.is_none()
                || !other_br.update_at.before(&self.brightness.as_ref().unwrap().update_at)
            {
                self.brightness = Some(other_br.clone());
                diff.brightness = Some(other_br.clone());
                has_changes = true;
            }
        }

        // Light mode
        if let Some(ref other_lm) = other.light_mode {
            if self.light_mode.is_none()
                || !other_lm.update_at.before(&self.light_mode.as_ref().unwrap().update_at)
            {
                self.light_mode = Some(other_lm.clone());
                diff.light_mode = Some(other_lm.clone());
                has_changes = true;
            }
        }

        // Pair status
        if let Some(ref other_ps) = other.pair_status {
            if self.pair_status.is_none()
                || !other_ps.update_at.before(&self.pair_status.as_ref().unwrap().update_at)
            {
                self.pair_status = Some(other_ps.clone());
                diff.pair_status = Some(other_ps.clone());
                has_changes = true;
            }
        }

        // Battery
        if let Some(ref other_bat) = other.battery {
            if self.battery.as_ref() != Some(other_bat) {
                self.battery = Some(other_bat.clone());
                diff.battery = Some(other_bat.clone());
                has_changes = true;
            }
        }

        // WiFi store
        if let Some(ref other_ws) = other.wifi_store {
            if self.wifi_store.is_none()
                || !other_ws.update_at.before(&self.wifi_store.as_ref().unwrap().update_at)
            {
                self.wifi_store = Some(other_ws.clone());
                diff.wifi_store = Some(other_ws.clone());
                has_changes = true;
            }
        }

        // Cellular
        if let Some(ref other_cell) = other.cellular_network {
            if self.cellular_network.as_ref() != Some(other_cell) {
                self.cellular_network = Some(other_cell.clone());
                diff.cellular = Some(other_cell.clone());
                has_changes = true;
            }
        }

        // WiFi network
        if let Some(ref other_wifi) = other.wifi_network {
            if self.wifi_network.as_ref() != Some(other_wifi) {
                self.wifi_network = Some(other_wifi.clone());
                diff.wifi_network = Some(other_wifi.clone());
                has_changes = true;
            }
        }

        // NFC tag
        if let Some(ref other_nfc) = other.read_nfc_tag {
            if self.read_nfc_tag.as_ref() != Some(other_nfc) {
                self.read_nfc_tag = Some(other_nfc.clone());
                diff.read_nfc_tag = Some(other_nfc.clone());
                has_changes = true;
            }
        }

        // Shaking
        if let Some(ref other_shake) = other.shaking {
            if self.shaking.as_ref() != Some(other_shake) {
                self.shaking = Some(other_shake.clone());
                diff.shaking = Some(other_shake.clone());
                has_changes = true;
            }
        }

        if has_changes {
            diff.time = self.time;
            Some(diff)
        } else {
            None
        }
    }
}

/// Represents changes to device statistics.
#[derive(Debug, Clone, Default)]
pub struct GearStatsChanges {
    pub time: Milli,
    pub last_reset_at: Option<Milli>,
    pub battery: Option<Battery>,
    pub system_version: Option<SystemVersion>,
    pub volume: Option<Volume>,
    pub brightness: Option<Brightness>,
    pub light_mode: Option<LightMode>,
    pub cellular: Option<ConnectedCellular>,
    pub wifi_network: Option<ConnectedWifi>,
    pub wifi_store: Option<StoredWifiList>,
    pub read_nfc_tag: Option<ReadNFCTag>,
    pub pair_status: Option<PairStatus>,
    pub shaking: Option<Shaking>,
}

impl GearStatsChanges {
    /// Converts changes to a full stats event.
    pub fn to_event(&self) -> GearStatsEvent {
        GearStatsEvent {
            time: self.time,
            last_reset_at: self.last_reset_at.unwrap_or_default(),
            battery: self.battery.clone(),
            system_version: self.system_version.clone(),
            volume: self.volume.clone(),
            brightness: self.brightness.clone(),
            light_mode: self.light_mode.clone(),
            cellular_network: self.cellular.clone(),
            wifi_network: self.wifi_network.clone(),
            wifi_store: self.wifi_store.clone(),
            read_nfc_tag: self.read_nfc_tag.clone(),
            pair_status: self.pair_status.clone(),
            shaking: self.shaking.clone(),
        }
    }
}

// ============================================================================
// Statistics Types
// ============================================================================

/// Contains system version information.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct SystemVersion {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub current_version: String,
    #[serde(default, skip_serializing_if = "is_zero_f64")]
    pub installing_percentage: f64,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub installing_version: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub components: Vec<ComponentVersion>,
    #[serde(default, skip_serializing_if = "Milli::is_zero")]
    pub update_at: Milli,
}

fn is_zero_f64(v: &f64) -> bool {
    *v == 0.0
}

/// Contains version info for a system component.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct ComponentVersion {
    pub name: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub current_version: String,
}

/// Contains display brightness settings.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct Brightness {
    pub percentage: f64,
    pub update_at: Milli,
}

/// Contains light mode settings.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct LightMode {
    pub mode: String,
    pub update_at: Milli,
}

/// Contains audio volume settings.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct Volume {
    pub percentage: f64,
    pub update_at: Milli,
}

/// Contains network ping statistics.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct Ping {
    pub recv_at: Milli,
    pub send_at: Milli,
    pub timestamp: Milli,
    pub latency: f64,
}

/// Contains cellular network information.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct ConnectedCellular {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub dns: Vec<String>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub gateway: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub ip: String,
    #[serde(default, rename = "mask", skip_serializing_if = "String::is_empty")]
    pub net_mask: String,
    #[serde(default, skip_serializing_if = "is_zero_f64")]
    pub rssi: f64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ping: Option<Ping>,
}

/// Contains WiFi network information.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct ConnectedWifi {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub bssid: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub dns: Vec<String>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub gateway: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub ip: String,
    #[serde(default, rename = "mask", skip_serializing_if = "String::is_empty")]
    pub net_mask: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub mac: String,
    #[serde(default, skip_serializing_if = "is_zero_f64")]
    pub rssi: f64,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub security: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub ssid: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ping: Option<Ping>,
}

/// Contains information about a stored WiFi network.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct WifiStoreItem {
    pub last_connect_at: Milli,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub security: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub ssid: String,
}

/// Contains a list of stored WiFi networks.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct StoredWifiList {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub list: Vec<WifiStoreItem>,
    pub update_at: Milli,
}

/// Contains battery status information.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct Battery {
    #[serde(default, skip_serializing_if = "is_zero_f64")]
    pub current_capacity: f64,
    #[serde(default, skip_serializing_if = "is_zero_f64")]
    pub cycle_count: f64,
    #[serde(default)]
    pub is_charging: bool,
    #[serde(default, skip_serializing_if = "is_zero_f64")]
    pub original_capacity: f64,
    #[serde(default, skip_serializing_if = "is_zero_f64")]
    pub percentage: f64,
    #[serde(default, skip_serializing_if = "is_zero_f64")]
    pub temperature: f64,
    #[serde(default, skip_serializing_if = "is_zero_f64")]
    pub voltage: f64,
}

/// Contains NFC tag read data.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(transparent)]
pub struct ReadNFCTag {
    pub tags: Vec<NFCTag>,
}

/// Contains information about an NFC tag.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct NFCTag {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub uid: String,
    #[serde(default, rename = "type", skip_serializing_if = "String::is_empty")]
    pub tag_type: String,
    #[serde(default, skip_serializing_if = "StdBase64Data::is_empty")]
    pub raw_data: StdBase64Data,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub data_format: String,
    #[serde(default, skip_serializing_if = "is_zero_f32")]
    pub rssi: f32,
    #[serde(default, skip_serializing_if = "Milli::is_zero")]
    pub update_at: Milli,
}

fn is_zero_f32(v: &f32) -> bool {
    *v == 0.0
}

/// Contains device pairing status.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct PairStatus {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub pair_with: String,
    #[serde(default, skip_serializing_if = "Milli::is_zero")]
    pub update_at: Milli,
}

/// Contains device shaking detection data.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct Shaking {
    pub level: f64,
}

#[cfg(test)]
mod stats_tests {
    use super::*;

    #[test]
    fn test_gear_stats_event_new() {
        let event = GearStatsEvent::new();
        assert!(!event.time.is_zero());
    }

    #[test]
    fn test_gear_stats_event_serialize() {
        let mut event = GearStatsEvent::new();
        event.battery = Some(Battery {
            percentage: 75.0,
            is_charging: true,
            ..Default::default()
        });

        let json = serde_json::to_string(&event).unwrap();
        assert!(json.contains("battery"));
        assert!(json.contains("75"));

        let restored: GearStatsEvent = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.battery.as_ref().unwrap().percentage, 75.0);
    }

    #[test]
    fn test_volume_serialize() {
        let vol = Volume {
            percentage: 50.0,
            update_at: Milli::now(),
        };

        let json = serde_json::to_string(&vol).unwrap();
        let restored: Volume = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.percentage, 50.0);
    }

    #[test]
    fn test_wifi_serialize() {
        let wifi = ConnectedWifi {
            ssid: "TestNetwork".to_string(),
            rssi: -45.0,
            ip: "192.168.1.100".to_string(),
            ..Default::default()
        };

        let json = serde_json::to_string(&wifi).unwrap();
        assert!(json.contains("TestNetwork"));
        assert!(json.contains("-45"));

        let restored: ConnectedWifi = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.ssid, "TestNetwork");
    }

    #[test]
    fn test_nfc_tag_serialize() {
        let tag = NFCTag {
            uid: "04:AB:CD:EF".to_string(),
            tag_type: "NTAG215".to_string(),
            raw_data: StdBase64Data::from(vec![0x01, 0x02, 0x03]),
            ..Default::default()
        };

        let json = serde_json::to_string(&tag).unwrap();
        assert!(json.contains("04:AB:CD:EF"));

        let restored: NFCTag = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.uid, "04:AB:CD:EF");
    }

    #[test]
    fn test_stats_merge() {
        let mut event1 = GearStatsEvent::new();
        event1.battery = Some(Battery {
            percentage: 50.0,
            ..Default::default()
        });

        std::thread::sleep(std::time::Duration::from_millis(1));

        let mut event2 = GearStatsEvent::new();
        event2.battery = Some(Battery {
            percentage: 75.0,
            ..Default::default()
        });

        let changes = event1.merge_with(&event2);
        assert!(changes.is_some());
        assert_eq!(event1.battery.as_ref().unwrap().percentage, 75.0);
    }
}
