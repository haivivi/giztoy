//! Device state types.

use giztoy_jsontime::Milli;
use serde::{Deserialize, Serialize};
use std::fmt;

/// Represents the state of a device.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default)]
pub enum GearState {
    #[default]
    Unknown,
    ShuttingDown,
    Sleeping,
    Resetting,
    Ready,
    Recording,
    WaitingForResponse,
    Streaming,
    Calling,
    Interrupted,
}

impl GearState {
    /// Returns true if the device is in an active (non-idle) state.
    pub fn is_active(&self) -> bool {
        matches!(
            self,
            GearState::Recording
                | GearState::WaitingForResponse
                | GearState::Streaming
                | GearState::Calling
        )
    }

    /// Returns true if the device can start recording in this state.
    pub fn can_record(&self) -> bool {
        matches!(self, GearState::Ready | GearState::Streaming)
    }

    /// Returns the string representation of the state.
    pub fn as_str(&self) -> &'static str {
        match self {
            GearState::Unknown => "unknown",
            GearState::ShuttingDown => "shutting_down",
            GearState::Sleeping => "sleeping",
            GearState::Resetting => "resetting",
            GearState::Ready => "ready",
            GearState::Recording => "recording",
            GearState::WaitingForResponse => "waiting_for_response",
            GearState::Streaming => "streaming",
            GearState::Calling => "calling",
            GearState::Interrupted => "interrupted",
        }
    }

    /// Parses a state from a string.
    pub fn from_str(s: &str) -> Self {
        match s {
            "shutting_down" => GearState::ShuttingDown,
            "sleeping" => GearState::Sleeping,
            "resetting" => GearState::Resetting,
            "ready" => GearState::Ready,
            "recording" => GearState::Recording,
            "waiting_for_response" => GearState::WaitingForResponse,
            "streaming" => GearState::Streaming,
            "calling" => GearState::Calling,
            "interrupted" => GearState::Interrupted,
            _ => GearState::Unknown,
        }
    }
}

impl fmt::Display for GearState {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.as_str())
    }
}

impl Serialize for GearState {
    fn serialize<S: serde::Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        serializer.serialize_str(self.as_str())
    }
}

impl<'de> Deserialize<'de> for GearState {
    fn deserialize<D: serde::Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        let s = String::deserialize(deserializer)?;
        Ok(GearState::from_str(&s))
    }
}

/// Provides additional context for why a state changed.
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
pub struct GearStateChangeCause {
    #[serde(default, skip_serializing_if = "is_false")]
    pub calling_initiated: bool,
    #[serde(default, skip_serializing_if = "is_false")]
    pub calling_resume: bool,
}

fn is_false(b: &bool) -> bool {
    !*b
}

/// Represents a state change event from the device.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct GearStateEvent {
    /// Protocol version.
    #[serde(rename = "v")]
    pub version: i32,

    /// Event timestamp.
    #[serde(rename = "t")]
    pub time: Milli,

    /// Current state.
    #[serde(rename = "s")]
    pub state: GearState,

    /// Optional cause for the state change.
    #[serde(rename = "c", skip_serializing_if = "Option::is_none")]
    pub cause: Option<GearStateChangeCause>,

    /// When the state was last updated on the device.
    #[serde(rename = "ut")]
    pub update_at: Milli,
}

impl GearStateEvent {
    /// Creates a new GearStateEvent.
    pub fn new(state: GearState) -> Self {
        let now = Milli::now();
        Self {
            version: 1,
            time: now,
            state,
            cause: None,
            update_at: now,
        }
    }

    /// Creates a new GearStateEvent with a specific update time.
    pub fn with_update_at(state: GearState, update_at: Milli) -> Self {
        Self {
            version: 1,
            time: Milli::now(),
            state,
            cause: None,
            update_at,
        }
    }

    /// Returns a deep copy of the event.
    pub fn clone_deep(&self) -> Self {
        Self {
            version: self.version,
            time: self.time,
            state: self.state,
            cause: self.cause.clone(),
            update_at: self.update_at,
        }
    }

    /// Merges another event into this one.
    /// Returns true if the state changed.
    pub fn merge_with(&mut self, other: &GearStateEvent) -> bool {
        if other.version != 1 {
            return false;
        }
        if other.time.before(&self.time) {
            return false;
        }

        self.time = other.time;
        self.update_at = other.update_at;
        self.cause = other.cause.clone();

        if self.state != other.state {
            self.state = other.state;
            return true;
        }
        false
    }
}

impl Default for GearStateEvent {
    fn default() -> Self {
        Self::new(GearState::Unknown)
    }
}

#[cfg(test)]
mod state_tests {
    use super::*;

    #[test]
    fn test_gear_state_string() {
        assert_eq!(GearState::Unknown.to_string(), "unknown");
        assert_eq!(GearState::ShuttingDown.to_string(), "shutting_down");
        assert_eq!(GearState::Ready.to_string(), "ready");
        assert_eq!(GearState::Recording.to_string(), "recording");
        assert_eq!(GearState::Streaming.to_string(), "streaming");
    }

    #[test]
    fn test_gear_state_from_str() {
        assert_eq!(GearState::from_str("ready"), GearState::Ready);
        assert_eq!(GearState::from_str("recording"), GearState::Recording);
        assert_eq!(GearState::from_str("invalid"), GearState::Unknown);
    }

    #[test]
    fn test_gear_state_is_active() {
        assert!(!GearState::Ready.is_active());
        assert!(GearState::Recording.is_active());
        assert!(GearState::WaitingForResponse.is_active());
        assert!(GearState::Streaming.is_active());
        assert!(GearState::Calling.is_active());
    }

    #[test]
    fn test_gear_state_can_record() {
        assert!(GearState::Ready.can_record());
        assert!(GearState::Streaming.can_record());
        assert!(!GearState::Recording.can_record());
        assert!(!GearState::Sleeping.can_record());
    }

    #[test]
    fn test_gear_state_serialize() {
        let state = GearState::Ready;
        let json = serde_json::to_string(&state).unwrap();
        assert_eq!(json, r#""ready""#);

        let restored: GearState = serde_json::from_str(&json).unwrap();
        assert_eq!(restored, state);
    }

    #[test]
    fn test_gear_state_event_serialize() {
        let event = GearStateEvent::new(GearState::Ready);
        let json = serde_json::to_string(&event).unwrap();
        
        let restored: GearStateEvent = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.state, GearState::Ready);
        assert_eq!(restored.version, 1);
    }

    #[test]
    fn test_gear_state_event_merge() {
        let mut event1 = GearStateEvent::new(GearState::Ready);
        let event2 = GearStateEvent::new(GearState::Recording);

        // Sleep a tiny bit to ensure time difference
        std::thread::sleep(std::time::Duration::from_millis(1));
        let event2 = GearStateEvent::new(GearState::Recording);

        let changed = event1.merge_with(&event2);
        assert!(changed);
        assert_eq!(event1.state, GearState::Recording);
    }

    #[test]
    fn test_gear_state_change_cause() {
        let cause = GearStateChangeCause {
            calling_initiated: true,
            calling_resume: false,
        };

        let json = serde_json::to_string(&cause).unwrap();
        assert!(json.contains("calling_initiated"));
        assert!(!json.contains("calling_resume")); // false values are skipped

        let restored: GearStateChangeCause = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.calling_initiated, true);
        assert_eq!(restored.calling_resume, false);
    }
}
