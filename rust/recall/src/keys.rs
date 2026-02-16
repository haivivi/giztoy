use chrono::{TimeZone, Utc};

/// Build the KV key for a segment.
/// Format: `{prefix}:seg:{YYYYMMDD}:{ts_ns}`
pub fn segment_key(prefix: &str, ts: i64) -> String {
    let dt = Utc.timestamp_nanos(ts);
    let date = dt.format("%Y%m%d").to_string();
    format!("{prefix}:seg:{date}:{ts}")
}

/// Return the KV prefix for listing all segments.
/// Format: `{prefix}:seg:`
pub fn segment_prefix(prefix: &str) -> String {
    format!("{prefix}:seg:")
}

/// Return the KV key for the segment-ID reverse index.
/// Format: `{prefix}:sid:{id}`
pub fn sid_key(prefix: &str, id: &str) -> String {
    format!("{prefix}:sid:{id}")
}

/// Return the KV prefix for the graph sub-store.
/// Format: `{prefix}:g`
pub fn graph_prefix(prefix: &str) -> String {
    format!("{prefix}:g")
}
