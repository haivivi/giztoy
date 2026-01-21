//! Epoch millisecond timestamp utilities.

use std::time::{Duration, SystemTime, UNIX_EPOCH};

/// Timestamp in milliseconds since Unix epoch.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Default)]
pub struct EpochMillis(pub i64);

impl EpochMillis {
    /// Creates a new EpochMillis from milliseconds.
    pub const fn from_millis(ms: i64) -> Self {
        Self(ms)
    }

    /// Returns the current time as EpochMillis.
    pub fn now() -> Self {
        let duration = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or(Duration::ZERO);
        Self(duration.as_millis() as i64)
    }

    /// Converts to milliseconds.
    pub const fn as_millis(&self) -> i64 {
        self.0
    }

    /// Converts to Duration.
    pub fn to_duration(&self) -> Duration {
        Duration::from_millis(self.0.max(0) as u64)
    }

    /// Creates from Duration.
    pub fn from_duration(d: Duration) -> Self {
        Self(d.as_millis() as i64)
    }

    /// Adds a Duration.
    pub fn add(&self, d: Duration) -> Self {
        Self(self.0 + d.as_millis() as i64)
    }

    /// Subtracts another EpochMillis, returning the difference as Duration.
    pub fn sub(&self, other: EpochMillis) -> Duration {
        let diff = self.0 - other.0;
        if diff >= 0 {
            Duration::from_millis(diff as u64)
        } else {
            Duration::ZERO
        }
    }

    /// Returns the difference in milliseconds.
    pub fn diff(&self, other: EpochMillis) -> i64 {
        self.0 - other.0
    }
}

impl std::ops::Add<Duration> for EpochMillis {
    type Output = Self;
    fn add(self, rhs: Duration) -> Self::Output {
        Self(self.0 + rhs.as_millis() as i64)
    }
}

impl std::ops::AddAssign<Duration> for EpochMillis {
    fn add_assign(&mut self, rhs: Duration) {
        self.0 += rhs.as_millis() as i64;
    }
}

impl std::ops::Sub<EpochMillis> for EpochMillis {
    type Output = i64;
    fn sub(self, rhs: EpochMillis) -> Self::Output {
        self.0 - rhs.0
    }
}

impl std::ops::Add<i64> for EpochMillis {
    type Output = Self;
    fn add(self, rhs: i64) -> Self::Output {
        Self(self.0 + rhs)
    }
}

impl std::ops::Sub<i64> for EpochMillis {
    type Output = Self;
    fn sub(self, rhs: i64) -> Self::Output {
        Self(self.0 - rhs)
    }
}

impl From<i64> for EpochMillis {
    fn from(ms: i64) -> Self {
        Self(ms)
    }
}

impl From<EpochMillis> for i64 {
    fn from(ms: EpochMillis) -> Self {
        ms.0
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_epoch_millis() {
        let t1 = EpochMillis::from_millis(1000);
        let t2 = t1.add(Duration::from_millis(500));
        assert_eq!(t2.as_millis(), 1500);
        assert_eq!(t2.sub(t1), Duration::from_millis(500));
    }

    #[test]
    fn test_now() {
        let t = EpochMillis::now();
        assert!(t.as_millis() > 0);
    }
}
