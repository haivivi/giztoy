//! Tests for jsontime types.

use super::*;
use chrono::{TimeZone, Utc};
use std::time::Duration as StdDuration;

// ============================================================================
// Milli tests
// ============================================================================

#[test]
fn test_milli_marshal_json() {
    let dt = Utc.with_ymd_and_hms(2024, 1, 15, 10, 30, 0).unwrap();
    let ep = Milli::new(dt);

    let data = serde_json::to_string(&ep).unwrap();
    let expected = dt.timestamp_millis();
    let got: i64 = serde_json::from_str(&data).unwrap();

    assert_eq!(got, expected);
}

#[test]
fn test_milli_unmarshal_json() {
    let ms: i64 = 1705315800000; // 2024-01-15 10:30:00 UTC
    let data = serde_json::to_string(&ms).unwrap();

    let ep: Milli = serde_json::from_str(&data).unwrap();
    let expected = Utc.timestamp_millis_opt(ms).unwrap();

    assert_eq!(ep.datetime(), expected);
}

#[test]
fn test_milli_round_trip() {
    let original = Milli::now();
    let data = serde_json::to_string(&original).unwrap();
    let restored: Milli = serde_json::from_str(&data).unwrap();

    // Compare at millisecond level
    assert_eq!(original.as_millis(), restored.as_millis());
}

#[test]
fn test_milli_comparisons() {
    let t1 = Milli::new(Utc.with_ymd_and_hms(2024, 1, 1, 0, 0, 0).unwrap());
    let t2 = Milli::new(Utc.with_ymd_and_hms(2024, 1, 2, 0, 0, 0).unwrap());

    assert!(t1.before(&t2));
    assert!(t2.after(&t1));
    assert_ne!(t1, t2);
    assert_eq!(t1, t1);
}

#[test]
fn test_milli_methods() {
    let ep = Milli::now();

    // Test Display
    assert!(!ep.to_string().is_empty());

    // Test datetime - now() should return non-zero timestamp
    assert!(ep.datetime().timestamp_millis() != 0);
    assert!(!ep.is_zero());

    // Test is_zero
    let zero = Milli::default();
    assert!(zero.is_zero());

    // Test add/sub
    let added = ep.add(StdDuration::from_secs(3600));
    let diff = added.sub(&ep);
    assert_eq!(diff.as_secs(), 3600);
}

#[test]
fn test_milli_from_conversions() {
    let ms: i64 = 1705315800000;
    let m1 = Milli::from(ms);
    let m2 = Milli::from_millis(ms);
    assert_eq!(m1, m2);

    let dt = Utc.timestamp_millis_opt(ms).unwrap();
    let m3 = Milli::from(dt);
    assert_eq!(m1, m3);
}

// ============================================================================
// Unix tests
// ============================================================================

#[test]
fn test_unix_marshal_json() {
    let dt = Utc.with_ymd_and_hms(2024, 1, 15, 10, 30, 0).unwrap();
    let ep = Unix::new(dt);

    let data = serde_json::to_string(&ep).unwrap();
    let expected = dt.timestamp();
    let got: i64 = serde_json::from_str(&data).unwrap();

    assert_eq!(got, expected);
}

#[test]
fn test_unix_unmarshal_json() {
    let secs: i64 = 1705315800; // 2024-01-15 10:30:00 UTC
    let data = serde_json::to_string(&secs).unwrap();

    let ep: Unix = serde_json::from_str(&data).unwrap();
    let expected = Utc.timestamp_opt(secs, 0).unwrap();

    assert_eq!(ep.datetime(), expected);
}

#[test]
fn test_unix_round_trip() {
    let original = Unix::now();
    let data = serde_json::to_string(&original).unwrap();
    let restored: Unix = serde_json::from_str(&data).unwrap();

    // Compare at second level
    assert_eq!(original.as_secs(), restored.as_secs());
}

#[test]
fn test_unix_comparisons() {
    let t1 = Unix::new(Utc.with_ymd_and_hms(2024, 1, 1, 0, 0, 0).unwrap());
    let t2 = Unix::new(Utc.with_ymd_and_hms(2024, 1, 2, 0, 0, 0).unwrap());

    assert!(t1.before(&t2));
    assert!(t2.after(&t1));
    assert_ne!(t1, t2);
    assert_eq!(t1, t1);
}

#[test]
fn test_unix_methods() {
    let ep = Unix::now();

    // Test Display
    assert!(!ep.to_string().is_empty());

    // Test is_zero
    let zero = Unix::default();
    assert!(zero.is_zero());

    // Test add/sub
    let added = ep.add(StdDuration::from_secs(3600));
    let diff = added.sub(&ep);
    assert_eq!(diff.as_secs(), 3600);
}

#[test]
fn test_unix_from_conversions() {
    let secs: i64 = 1705315800;
    let u1 = Unix::from(secs);
    let u2 = Unix::from_secs(secs);
    assert_eq!(u1, u2);

    let dt = Utc.timestamp_opt(secs, 0).unwrap();
    let u3 = Unix::from(dt);
    assert_eq!(u1, u3);
}

// ============================================================================
// Duration tests
// ============================================================================

#[test]
fn test_duration_marshal_json() {
    let d = Duration::from_secs(5400); // 1h30m

    let data = serde_json::to_string(&d).unwrap();
    let got: String = serde_json::from_str(&data).unwrap();

    assert_eq!(got, "1h30m0s");
}

#[test]
fn test_duration_unmarshal_json_string() {
    let data = r#""2h30m""#;

    let d: Duration = serde_json::from_str(data).unwrap();
    let expected = StdDuration::from_secs(2 * 3600 + 30 * 60);

    assert_eq!(d.as_std(), expected);
}

#[test]
fn test_duration_unmarshal_json_int() {
    let ns: u64 = 5_000_000_000; // 5 seconds in nanoseconds
    let data = serde_json::to_string(&ns).unwrap();

    let d: Duration = serde_json::from_str(&data).unwrap();

    assert_eq!(d.as_secs(), 5);
}

#[test]
fn test_duration_unmarshal_json_null() {
    let data = "null";

    let d: Duration = serde_json::from_str(data).unwrap();

    assert_eq!(d.as_secs(), 0);
}

#[test]
fn test_duration_round_trip() {
    let original = Duration::from_secs(3 * 3600 + 45 * 60 + 30);

    let data = serde_json::to_string(&original).unwrap();
    let restored: Duration = serde_json::from_str(&data).unwrap();

    assert_eq!(original, restored);
}

#[test]
fn test_duration_methods() {
    let d = Duration::from_secs(5400); // 1h30m

    // Test as_std
    assert_eq!(d.as_std(), StdDuration::from_secs(5400));

    // Test as_secs_f64
    assert!((d.as_secs_f64() - 5400.0).abs() < 0.001);

    // Test as_secs
    assert_eq!(d.as_secs(), 5400);

    // Test as_millis
    assert_eq!(d.as_millis(), 5400000);

    // Test Display
    assert_eq!(d.to_string(), "1h30m0s");

    // Test is_zero
    assert!(!d.is_zero());
    assert!(Duration::default().is_zero());
}

#[test]
fn test_duration_from_conversions() {
    let std_dur = StdDuration::from_secs(60);
    let d = Duration::from(std_dur);
    assert_eq!(d.as_secs(), 60);

    let back: StdDuration = d.into();
    assert_eq!(back, std_dur);
}

#[test]
fn test_from_duration_helper() {
    let d = Duration::from(StdDuration::from_secs(120));
    assert_eq!(d.as_secs(), 120);
}

#[test]
fn test_duration_in_struct() {
    #[derive(serde::Serialize, serde::Deserialize, PartialEq, Debug)]
    struct Config {
        timeout: Duration,
        #[serde(skip_serializing_if = "Option::is_none")]
        interval: Option<Duration>,
    }

    let cfg = Config {
        timeout: Duration::from_secs(30),
        interval: Some(Duration::from_secs(300)),
    };

    let data = serde_json::to_string(&cfg).unwrap();
    let restored: Config = serde_json::from_str(&data).unwrap();

    assert_eq!(restored.timeout, cfg.timeout);
    assert_eq!(restored.interval, cfg.interval);
}

// ============================================================================
// Edge cases
// ============================================================================

#[test]
fn test_milli_zero() {
    let zero = Milli::from_millis(0);
    assert!(zero.is_zero());

    let data = serde_json::to_string(&zero).unwrap();
    assert_eq!(data, "0");

    let restored: Milli = serde_json::from_str(&data).unwrap();
    assert!(restored.is_zero());
}

#[test]
fn test_unix_zero() {
    let zero = Unix::from_secs(0);
    assert!(zero.is_zero());

    let data = serde_json::to_string(&zero).unwrap();
    assert_eq!(data, "0");

    let restored: Unix = serde_json::from_str(&data).unwrap();
    assert!(restored.is_zero());
}

#[test]
fn test_duration_various_formats() {
    // Hours only
    let d: Duration = serde_json::from_str(r#""2h""#).unwrap();
    assert_eq!(d.as_secs(), 7200);

    // Minutes only
    let d: Duration = serde_json::from_str(r#""45m""#).unwrap();
    assert_eq!(d.as_secs(), 2700);

    // Seconds only
    let d: Duration = serde_json::from_str(r#""30s""#).unwrap();
    assert_eq!(d.as_secs(), 30);

    // Combined
    let d: Duration = serde_json::from_str(r#""1h30m45s""#).unwrap();
    assert_eq!(d.as_secs(), 5445);
}

#[test]
fn test_negative_timestamp() {
    // Negative milliseconds (before Unix epoch)
    let ms: i64 = -1000;
    let m = Milli::from_millis(ms);
    assert_eq!(m.as_millis(), ms);

    // Round trip
    let data = serde_json::to_string(&m).unwrap();
    let restored: Milli = serde_json::from_str(&data).unwrap();
    assert_eq!(restored.as_millis(), ms);
}

#[test]
fn test_large_timestamp() {
    // Year 3000
    let ms: i64 = 32503680000000;
    let m = Milli::from_millis(ms);
    
    let data = serde_json::to_string(&m).unwrap();
    let restored: Milli = serde_json::from_str(&data).unwrap();
    assert_eq!(restored.as_millis(), ms);
}
