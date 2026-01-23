//! Unix milliseconds timestamp type.

use chrono::{DateTime, TimeZone, Utc};
use serde::{Deserialize, Deserializer, Serialize, Serializer};
use std::fmt;
use std::time::Duration;

/// A timestamp that serializes to/from Unix milliseconds in JSON.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Default)]
pub struct Milli(DateTime<Utc>);

impl Milli {
    /// Creates a new Milli from a DateTime<Utc>.
    pub fn new(dt: DateTime<Utc>) -> Self {
        Self(dt)
    }

    /// Returns the current time as Milli.
    pub fn now() -> Self {
        Self(Utc::now())
    }

    /// Creates a Milli from Unix milliseconds.
    pub fn from_millis(ms: i64) -> Self {
        Self(Utc.timestamp_millis_opt(ms).single().unwrap_or_default())
    }

    /// Returns the Unix milliseconds value.
    pub fn as_millis(&self) -> i64 {
        self.0.timestamp_millis()
    }

    /// Returns the underlying DateTime<Utc>.
    pub fn datetime(&self) -> DateTime<Utc> {
        self.0
    }

    /// Reports whether this time is before the other.
    pub fn before(&self, other: &Self) -> bool {
        self.0 < other.0
    }

    /// Reports whether this time is after the other.
    pub fn after(&self, other: &Self) -> bool {
        self.0 > other.0
    }

    /// Reports whether this represents the zero time instant.
    pub fn is_zero(&self) -> bool {
        self.0.timestamp_millis() == 0
    }

    /// Returns the duration between this and another time.
    pub fn sub(&self, other: &Self) -> Duration {
        let diff = self.0.signed_duration_since(other.0);
        Duration::from_millis(diff.num_milliseconds().unsigned_abs())
    }

    /// Returns the time plus the given duration.
    pub fn add(&self, d: Duration) -> Self {
        Self(self.0 + chrono::Duration::milliseconds(d.as_millis() as i64))
    }
}

impl fmt::Display for Milli {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl Serialize for Milli {
    fn serialize<S: Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        serializer.serialize_i64(self.0.timestamp_millis())
    }
}

impl<'de> Deserialize<'de> for Milli {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        let ms = i64::deserialize(deserializer)?;
        Ok(Self::from_millis(ms))
    }
}

impl From<DateTime<Utc>> for Milli {
    fn from(dt: DateTime<Utc>) -> Self {
        Self(dt)
    }
}

impl From<Milli> for DateTime<Utc> {
    fn from(m: Milli) -> Self {
        m.0
    }
}

impl From<i64> for Milli {
    fn from(ms: i64) -> Self {
        Self::from_millis(ms)
    }
}
