//! Time builtin implementation.

use crate::runtime::Runtime;
use giztoy_luau::{Error, LuaStackOps};
use std::time::{SystemTime, UNIX_EPOCH};

impl Runtime {
    /// Register time builtins
    pub fn register_time(&mut self) -> Result<(), Error> {
        // __builtin.time() -> number
        self.state.register_func("__builtin_time", |state| {
            let now = SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map(|d| d.as_secs_f64())
                .unwrap_or(0.0);
            state.push_number(now);
            1
        })?;

        self.state.get_global("__builtin_time")?;
        self.state.set_field(-2, "time")?;
        self.state.push_nil();
        self.state.set_global("__builtin_time")?;

        // __builtin.parse_time(iso_string) -> number | nil
        self.state.register_func("__builtin_parse_time", |state| {
            let iso_str = state.to_string(1).unwrap_or_default();
            if iso_str.is_empty() {
                state.push_nil();
                return 1;
            }

            // Try to parse ISO 8601 date
            if let Some(ts) = parse_iso8601(&iso_str) {
                state.push_number(ts);
            } else {
                state.push_nil();
            }
            1
        })?;

        self.state.get_global("__builtin_parse_time")?;
        self.state.set_field(-2, "parse_time")?;
        self.state.push_nil();
        self.state.set_global("__builtin_parse_time")?;

        Ok(())
    }
}

/// Parse ISO 8601 date string to Unix timestamp (seconds).
/// Supports formats like:
/// - 2026-01-27T00:00:00Z
/// - 2026-01-27T00:00:00.000Z
/// - 2026-01-27T00:00:00+08:00
fn parse_iso8601(s: &str) -> Option<f64> {
    // Simple ISO 8601 parser without external dependencies
    // Format: YYYY-MM-DDTHH:MM:SS[.sss][Z|±HH:MM]
    
    let s = s.trim();
    if s.len() < 10 {
        return None;
    }

    // Parse date part: YYYY-MM-DD
    let year: i32 = s.get(0..4)?.parse().ok()?;
    if s.get(4..5)? != "-" {
        return None;
    }
    let month: u32 = s.get(5..7)?.parse().ok()?;
    if s.get(7..8)? != "-" {
        return None;
    }
    let day: u32 = s.get(8..10)?.parse().ok()?;

    // Default time to midnight
    let mut hour: u32 = 0;
    let mut minute: u32 = 0;
    let mut second: u32 = 0;
    let mut millis: u32 = 0;
    let mut tz_offset_seconds: i64 = 0;

    // Parse time part if present
    let rest = s.get(10..)?;
    if !rest.is_empty() {
        let rest = if rest.starts_with('T') || rest.starts_with(' ') {
            &rest[1..]
        } else {
            return None;
        };

        if rest.len() >= 8 {
            hour = rest.get(0..2)?.parse().ok()?;
            if rest.get(2..3)? != ":" {
                return None;
            }
            minute = rest.get(3..5)?.parse().ok()?;
            if rest.get(5..6)? != ":" {
                return None;
            }
            second = rest.get(6..8)?.parse().ok()?;

            let mut idx = 8;

            // Parse milliseconds if present
            if rest.get(idx..idx + 1) == Some(".") {
                idx += 1;
                let ms_start = idx;
                while idx < rest.len() && rest.chars().nth(idx).map(|c| c.is_ascii_digit()).unwrap_or(false) {
                    idx += 1;
                }
                let ms_str = rest.get(ms_start..idx)?;
                millis = match ms_str.len() {
                    1 => ms_str.parse::<u32>().ok()? * 100,
                    2 => ms_str.parse::<u32>().ok()? * 10,
                    _ => ms_str.get(0..3)?.parse().ok()?,
                };
            }

            // Parse timezone
            let tz_part = rest.get(idx..)?;
            if tz_part == "Z" || tz_part.is_empty() {
                // UTC
                tz_offset_seconds = 0;
            } else if tz_part.starts_with('+') || tz_part.starts_with('-') {
                let sign = if tz_part.starts_with('+') { 1 } else { -1 };
                let tz_str = &tz_part[1..];
                if tz_str.len() >= 5 && tz_str.get(2..3) == Some(":") {
                    let tz_hour: i64 = tz_str.get(0..2)?.parse().ok()?;
                    let tz_min: i64 = tz_str.get(3..5)?.parse().ok()?;
                    tz_offset_seconds = sign * (tz_hour * 3600 + tz_min * 60);
                } else if tz_str.len() >= 4 {
                    let tz_hour: i64 = tz_str.get(0..2)?.parse().ok()?;
                    let tz_min: i64 = tz_str.get(2..4)?.parse().ok()?;
                    tz_offset_seconds = sign * (tz_hour * 3600 + tz_min * 60);
                }
            }
        }
    }

    // Calculate days since Unix epoch (1970-01-01)
    let days = days_since_epoch(year, month, day)?;
    
    // Calculate total seconds
    let total_seconds = days as i64 * 86400
        + hour as i64 * 3600
        + minute as i64 * 60
        + second as i64
        - tz_offset_seconds;

    Some(total_seconds as f64 + millis as f64 / 1000.0)
}

/// Calculate days since Unix epoch (1970-01-01).
fn days_since_epoch(year: i32, month: u32, day: u32) -> Option<i64> {
    if month < 1 || month > 12 || day < 1 || day > 31 {
        return None;
    }

    // Days in each month (non-leap year)
    let days_in_month = [31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
    
    let is_leap = |y: i32| (y % 4 == 0 && y % 100 != 0) || (y % 400 == 0);
    
    // Days from year 1970 to start of given year
    let mut days: i64 = 0;
    if year >= 1970 {
        for y in 1970..year {
            days += if is_leap(y) { 366 } else { 365 };
        }
    } else {
        for y in year..1970 {
            days -= if is_leap(y) { 366 } else { 365 };
        }
    }
    
    // Days from start of year to start of given month
    for m in 1..month {
        days += days_in_month[(m - 1) as usize] as i64;
        if m == 2 && is_leap(year) {
            days += 1;
        }
    }
    
    // Add day of month (1-indexed)
    days += (day - 1) as i64;
    
    Some(days)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_iso8601() {
        // UTC
        assert!((parse_iso8601("2026-01-27T00:00:00Z").unwrap() - 1769472000.0).abs() < 1.0);
        
        // With milliseconds
        assert!((parse_iso8601("2026-01-27T00:00:00.500Z").unwrap() - 1769472000.5).abs() < 0.01);
        
        // Date only
        assert!((parse_iso8601("2026-01-27").unwrap() - 1769472000.0).abs() < 1.0);
    }
}
