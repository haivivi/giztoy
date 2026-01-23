//! Unix seconds timestamp type.

use chrono::{DateTime, TimeZone, Utc};
use serde::{Deserialize, Deserializer, Serialize, Serializer};
use std::fmt;
use std::time::Duration;

/// A timestamp that serializes to/from Unix seconds in JSON.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Default)]
pub struct Unix(DateTime<Utc>);

impl Unix {
    /// Creates a new Unix from a DateTime<Utc>.
    pub fn new(dt: DateTime<Utc>) -> Self {
        Self(dt)
    }

    /// Returns the current time as Unix.
    pub fn now() -> Self {
        Self(Utc::now())
    }

    /// Creates a Unix from Unix seconds.
    pub fn from_secs(secs: i64) -> Self {
        Self(Utc.timestamp_opt(secs, 0).single().unwrap_or_default())
    }

    /// Returns the Unix seconds value.
    pub fn as_secs(&self) -> i64 {
        self.0.timestamp()
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
        self.0.timestamp() == 0
    }

    /// Returns the duration between this and another time.
    pub fn sub(&self, other: &Self) -> Duration {
        let diff = self.0.signed_duration_since(other.0);
        Duration::from_secs(diff.num_seconds().unsigned_abs())
    }

    /// Returns the time plus the given duration.
    pub fn add(&self, d: Duration) -> Self {
        Self(self.0 + chrono::Duration::seconds(d.as_secs() as i64))
    }
}

impl fmt::Display for Unix {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl Serialize for Unix {
    fn serialize<S: Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        serializer.serialize_i64(self.0.timestamp())
    }
}

impl<'de> Deserialize<'de> for Unix {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        let secs = i64::deserialize(deserializer)?;
        Ok(Self::from_secs(secs))
    }
}

impl From<DateTime<Utc>> for Unix {
    fn from(dt: DateTime<Utc>) -> Self {
        Self(dt)
    }
}

impl From<Unix> for DateTime<Utc> {
    fn from(u: Unix) -> Self {
        u.0
    }
}

impl From<i64> for Unix {
    fn from(secs: i64) -> Self {
        Self::from_secs(secs)
    }
}
