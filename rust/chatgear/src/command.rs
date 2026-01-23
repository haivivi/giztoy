//! Device command types.

use giztoy_jsontime::Milli;
use serde::{Deserialize, Serialize};
use std::fmt;

/// The interface for device commands.
pub trait SessionCommand: fmt::Debug + Send + Sync {
    /// Returns the command type string.
    fn command_type(&self) -> &'static str;
}

/// Wraps a command with metadata.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SessionCommandEvent {
    /// Command type identifier.
    #[serde(rename = "type")]
    pub cmd_type: String,

    /// Event timestamp.
    pub time: Milli,

    /// Command payload (serialized).
    #[serde(rename = "pld")]
    pub payload: serde_json::Value,

    /// When the command was issued.
    pub issue_at: Milli,
}

impl SessionCommandEvent {
    /// Creates a new command event.
    pub fn new<C: SessionCommand + Serialize>(cmd: &C) -> Self {
        let now = Milli::now();
        Self {
            cmd_type: cmd.command_type().to_string(),
            time: now,
            payload: serde_json::to_value(cmd).unwrap_or(serde_json::Value::Null),
            issue_at: now,
        }
    }

    /// Creates a new command event with a specific issue time.
    pub fn with_issue_at<C: SessionCommand + Serialize>(cmd: &C, issue_at: Milli) -> Self {
        Self {
            cmd_type: cmd.command_type().to_string(),
            time: Milli::now(),
            payload: serde_json::to_value(cmd).unwrap_or(serde_json::Value::Null),
            issue_at,
        }
    }

    /// Attempts to deserialize the payload into a specific command type.
    pub fn parse_payload<C: for<'de> Deserialize<'de>>(&self) -> Result<C, serde_json::Error> {
        serde_json::from_value(self.payload.clone())
    }
}

// ============================================================================
// Command Types
// ============================================================================

/// Command to start/stop audio streaming.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(transparent)]
pub struct Streaming(pub bool);

impl Streaming {
    pub fn new(enabled: bool) -> Self {
        Self(enabled)
    }
}

impl SessionCommand for Streaming {
    fn command_type(&self) -> &'static str {
        "streaming"
    }
}

/// Command to reset the device.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct Reset {
    #[serde(default, skip_serializing_if = "is_false")]
    pub unpair: bool,
}

fn is_false(b: &bool) -> bool {
    !*b
}

impl Reset {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn with_unpair() -> Self {
        Self { unpair: true }
    }
}

impl SessionCommand for Reset {
    fn command_type(&self) -> &'static str {
        "reset"
    }
}

/// Command to raise an event (e.g., start a call).
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct Raise {
    #[serde(default)]
    pub call: bool,
}

impl Raise {
    pub fn call() -> Self {
        Self { call: true }
    }
}

impl SessionCommand for Raise {
    fn command_type(&self) -> &'static str {
        "raise"
    }
}

/// Command to halt device operation.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct Halt {
    #[serde(default, skip_serializing_if = "is_false")]
    pub sleep: bool,
    #[serde(default, skip_serializing_if = "is_false")]
    pub shutdown: bool,
    #[serde(default, skip_serializing_if = "is_false")]
    pub interrupt: bool,
}

impl Halt {
    pub fn sleep() -> Self {
        Self {
            sleep: true,
            ..Default::default()
        }
    }

    pub fn shutdown() -> Self {
        Self {
            shutdown: true,
            ..Default::default()
        }
    }

    pub fn interrupt() -> Self {
        Self {
            interrupt: true,
            ..Default::default()
        }
    }
}

impl SessionCommand for Halt {
    fn command_type(&self) -> &'static str {
        "halt"
    }
}

/// Command to set audio volume.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(transparent)]
pub struct SetVolume(pub i32);

impl SetVolume {
    pub fn new(volume: i32) -> Self {
        Self(volume)
    }
}

impl SessionCommand for SetVolume {
    fn command_type(&self) -> &'static str {
        "set_volume"
    }
}

/// Command to set display brightness.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(transparent)]
pub struct SetBrightness(pub i32);

impl SetBrightness {
    pub fn new(brightness: i32) -> Self {
        Self(brightness)
    }
}

impl SessionCommand for SetBrightness {
    fn command_type(&self) -> &'static str {
        "set_brightness"
    }
}

/// Command to set light mode.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(transparent)]
pub struct SetLightMode(pub String);

impl SetLightMode {
    pub fn new(mode: impl Into<String>) -> Self {
        Self(mode.into())
    }
}

impl SessionCommand for SetLightMode {
    fn command_type(&self) -> &'static str {
        "set_light_mode"
    }
}

/// Command to configure WiFi.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SetWifi {
    pub ssid: String,
    pub security: String,
    pub password: String,
}

impl SetWifi {
    pub fn new(ssid: impl Into<String>, security: impl Into<String>, password: impl Into<String>) -> Self {
        Self {
            ssid: ssid.into(),
            security: security.into(),
            password: password.into(),
        }
    }
}

impl SessionCommand for SetWifi {
    fn command_type(&self) -> &'static str {
        "set_wifi"
    }
}

/// Command to delete a stored WiFi network.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(transparent)]
pub struct DeleteWifi(pub String);

impl DeleteWifi {
    pub fn new(ssid: impl Into<String>) -> Self {
        Self(ssid.into())
    }
}

impl SessionCommand for DeleteWifi {
    fn command_type(&self) -> &'static str {
        "delete_wifi"
    }
}

/// OTA info for a component.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct ComponentOTA {
    pub name: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub version: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub image_url: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub image_md5: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub data_file_url: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub data_file_md5: String,
}

/// Command to initiate firmware upgrade.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct OTA {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub version: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub image_url: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub image_md5: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub data_file_url: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub data_file_md5: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub components: Vec<ComponentOTA>,
}

impl SessionCommand for OTA {
    fn command_type(&self) -> &'static str {
        "ota_upgrade"
    }
}

/// Enum wrapper for all command types (for deserialization).
#[derive(Debug, Clone)]
pub enum Command {
    Streaming(Streaming),
    Reset(Reset),
    Raise(Raise),
    Halt(Halt),
    SetVolume(SetVolume),
    SetBrightness(SetBrightness),
    SetLightMode(SetLightMode),
    SetWifi(SetWifi),
    DeleteWifi(DeleteWifi),
    OTA(OTA),
}

impl Command {
    /// Parses a command from a type string and JSON payload.
    pub fn parse(cmd_type: &str, payload: &serde_json::Value) -> Result<Self, serde_json::Error> {
        match cmd_type {
            "streaming" => Ok(Command::Streaming(serde_json::from_value(payload.clone())?)),
            "reset" => Ok(Command::Reset(serde_json::from_value(payload.clone())?)),
            "raise" => Ok(Command::Raise(serde_json::from_value(payload.clone())?)),
            "halt" => Ok(Command::Halt(serde_json::from_value(payload.clone())?)),
            "set_volume" => Ok(Command::SetVolume(serde_json::from_value(payload.clone())?)),
            "set_brightness" => Ok(Command::SetBrightness(serde_json::from_value(payload.clone())?)),
            "set_light_mode" => Ok(Command::SetLightMode(serde_json::from_value(payload.clone())?)),
            "set_wifi" => Ok(Command::SetWifi(serde_json::from_value(payload.clone())?)),
            "delete_wifi" => Ok(Command::DeleteWifi(serde_json::from_value(payload.clone())?)),
            "ota_upgrade" => Ok(Command::OTA(serde_json::from_value(payload.clone())?)),
            _ => Err(serde::de::Error::custom(format!("unknown command type: {}", cmd_type))),
        }
    }
}

#[cfg(test)]
mod command_tests {
    use super::*;

    #[test]
    fn test_streaming_command() {
        let cmd = Streaming::new(true);
        assert_eq!(cmd.command_type(), "streaming");

        let json = serde_json::to_string(&cmd).unwrap();
        assert_eq!(json, "true");

        let restored: Streaming = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.0, true);
    }

    #[test]
    fn test_set_volume_command() {
        let cmd = SetVolume::new(75);
        assert_eq!(cmd.command_type(), "set_volume");

        let json = serde_json::to_string(&cmd).unwrap();
        assert_eq!(json, "75");

        let restored: SetVolume = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.0, 75);
    }

    #[test]
    fn test_reset_command() {
        let cmd = Reset::new();
        let json = serde_json::to_string(&cmd).unwrap();
        assert_eq!(json, "{}");

        let cmd = Reset::with_unpair();
        let json = serde_json::to_string(&cmd).unwrap();
        assert!(json.contains("unpair"));
    }

    #[test]
    fn test_halt_command() {
        let cmd = Halt::sleep();
        let json = serde_json::to_string(&cmd).unwrap();
        assert!(json.contains("sleep"));
        assert!(!json.contains("shutdown"));

        let cmd = Halt::shutdown();
        let json = serde_json::to_string(&cmd).unwrap();
        assert!(json.contains("shutdown"));
    }

    #[test]
    fn test_set_wifi_command() {
        let cmd = SetWifi::new("MyNetwork", "WPA2", "password123");
        assert_eq!(cmd.command_type(), "set_wifi");

        let json = serde_json::to_string(&cmd).unwrap();
        assert!(json.contains("MyNetwork"));
        assert!(json.contains("WPA2"));
        assert!(json.contains("password123"));
    }

    #[test]
    fn test_ota_command() {
        let cmd = OTA {
            version: "1.2.3".to_string(),
            image_url: "https://example.com/firmware.bin".to_string(),
            ..Default::default()
        };
        assert_eq!(cmd.command_type(), "ota_upgrade");

        let json = serde_json::to_string(&cmd).unwrap();
        assert!(json.contains("1.2.3"));
        assert!(json.contains("firmware.bin"));
    }

    #[test]
    fn test_session_command_event() {
        let cmd = SetVolume::new(50);
        let event = SessionCommandEvent::new(&cmd);

        assert_eq!(event.cmd_type, "set_volume");
        assert_eq!(event.payload, serde_json::json!(50));
    }

    #[test]
    fn test_command_parse() {
        let payload = serde_json::json!(true);
        let cmd = Command::parse("streaming", &payload).unwrap();
        match cmd {
            Command::Streaming(s) => assert_eq!(s.0, true),
            _ => panic!("expected Streaming"),
        }

        let payload = serde_json::json!({"ssid": "test", "security": "WPA2", "password": "pass"});
        let cmd = Command::parse("set_wifi", &payload).unwrap();
        match cmd {
            Command::SetWifi(w) => {
                assert_eq!(w.ssid, "test");
                assert_eq!(w.security, "WPA2");
            }
            _ => panic!("expected SetWifi"),
        }
    }
}
