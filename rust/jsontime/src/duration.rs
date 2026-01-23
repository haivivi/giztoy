//! Duration type with flexible JSON serialization.

use serde::{Deserialize, Deserializer, Serialize, Serializer};
use std::fmt;
use std::time::Duration as StdDuration;

/// A duration that serializes to string (e.g., "1h30m") and deserializes from
/// either a string or nanoseconds integer.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Default)]
pub struct Duration(StdDuration);

impl Duration {
    /// Creates a new Duration from a std::time::Duration.
    pub fn new(d: StdDuration) -> Self {
        Self(d)
    }

    /// Creates a Duration from seconds.
    pub fn from_secs(secs: u64) -> Self {
        Self(StdDuration::from_secs(secs))
    }

    /// Creates a Duration from milliseconds.
    pub fn from_millis(ms: u64) -> Self {
        Self(StdDuration::from_millis(ms))
    }

    /// Creates a Duration from nanoseconds.
    pub fn from_nanos(nanos: u64) -> Self {
        Self(StdDuration::from_nanos(nanos))
    }

    /// Returns the underlying std::time::Duration.
    pub fn as_std(&self) -> StdDuration {
        self.0
    }

    /// Returns the duration as seconds (floating point).
    pub fn as_secs_f64(&self) -> f64 {
        self.0.as_secs_f64()
    }

    /// Returns the duration as whole seconds.
    pub fn as_secs(&self) -> u64 {
        self.0.as_secs()
    }

    /// Returns the duration as milliseconds.
    pub fn as_millis(&self) -> u128 {
        self.0.as_millis()
    }

    /// Returns the duration as nanoseconds.
    pub fn as_nanos(&self) -> u128 {
        self.0.as_nanos()
    }

    /// Returns true if this duration is zero.
    pub fn is_zero(&self) -> bool {
        self.0.is_zero()
    }
}

impl fmt::Display for Duration {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let total_secs = self.0.as_secs();
        let hours = total_secs / 3600;
        let mins = (total_secs % 3600) / 60;
        let secs = total_secs % 60;

        if hours > 0 {
            write!(f, "{}h{}m{}s", hours, mins, secs)
        } else if mins > 0 {
            write!(f, "{}m{}s", mins, secs)
        } else {
            write!(f, "{}s", secs)
        }
    }
}

impl Serialize for Duration {
    fn serialize<S: Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        serializer.serialize_str(&self.to_string())
    }
}

impl<'de> Deserialize<'de> for Duration {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        struct DurationVisitor;

        impl<'de> serde::de::Visitor<'de> for DurationVisitor {
            type Value = Duration;

            fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
                formatter.write_str("a duration string (e.g., '1h30m') or nanoseconds integer")
            }

            fn visit_str<E: serde::de::Error>(self, v: &str) -> Result<Self::Value, E> {
                parse_duration(v).map_err(serde::de::Error::custom)
            }

            fn visit_i64<E: serde::de::Error>(self, v: i64) -> Result<Self::Value, E> {
                if v < 0 {
                    return Err(serde::de::Error::custom("negative duration"));
                }
                Ok(Duration::from_nanos(v as u64))
            }

            fn visit_u64<E: serde::de::Error>(self, v: u64) -> Result<Self::Value, E> {
                Ok(Duration::from_nanos(v))
            }

            fn visit_unit<E: serde::de::Error>(self) -> Result<Self::Value, E> {
                Ok(Duration::default())
            }
        }

        deserializer.deserialize_any(DurationVisitor)
    }
}

/// Parses a duration string like "1h30m", "5m", "30s", "1h30m45s".
fn parse_duration(s: &str) -> Result<Duration, String> {
    if s.is_empty() {
        return Ok(Duration::default());
    }

    let mut total_secs: u64 = 0;
    let mut current_num = String::new();

    for c in s.chars() {
        if c.is_ascii_digit() {
            current_num.push(c);
        } else {
            let num: u64 = current_num
                .parse()
                .map_err(|_| format!("invalid number in duration: {}", s))?;
            current_num.clear();

            match c {
                'h' => total_secs += num * 3600,
                'm' => total_secs += num * 60,
                's' => total_secs += num,
                _ => return Err(format!("invalid duration unit '{}' in: {}", c, s)),
            }
        }
    }

    // Handle case where string ends with a number (no unit = seconds)
    if !current_num.is_empty() {
        let num: u64 = current_num
            .parse()
            .map_err(|_| format!("invalid number in duration: {}", s))?;
        total_secs += num;
    }

    Ok(Duration::from_secs(total_secs))
}

impl From<StdDuration> for Duration {
    fn from(d: StdDuration) -> Self {
        Self(d)
    }
}

impl From<Duration> for StdDuration {
    fn from(d: Duration) -> Self {
        d.0
    }
}

/// Creates a Duration pointer from a std::time::Duration.
pub fn from_duration(d: StdDuration) -> Duration {
    Duration(d)
}

#[cfg(test)]
mod parse_tests {
    use super::*;

    #[test]
    fn test_parse_duration() {
        assert_eq!(parse_duration("1h").unwrap().as_secs(), 3600);
        assert_eq!(parse_duration("30m").unwrap().as_secs(), 1800);
        assert_eq!(parse_duration("45s").unwrap().as_secs(), 45);
        assert_eq!(parse_duration("1h30m").unwrap().as_secs(), 5400);
        assert_eq!(parse_duration("1h30m45s").unwrap().as_secs(), 5445);
        assert_eq!(parse_duration("").unwrap().as_secs(), 0);
    }

    #[test]
    fn test_display() {
        assert_eq!(Duration::from_secs(3600).to_string(), "1h0m0s");
        assert_eq!(Duration::from_secs(5400).to_string(), "1h30m0s");
        assert_eq!(Duration::from_secs(90).to_string(), "1m30s");
        assert_eq!(Duration::from_secs(30).to_string(), "30s");
    }
}
