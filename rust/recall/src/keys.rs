/// Build the KV key for a segment.
/// Format: `{prefix}:seg:{bucket}:{ts_ns_20d}`
///
/// Timestamp is zero-padded to 20 decimal digits for stable lexicographic
/// ordering in KV scans.
pub fn segment_key(prefix: &str, bucket: &str, ts: i64) -> String {
    if ts >= 0 {
        format!("{prefix}:seg:{bucket}:{ts:020}")
    } else {
        // Negative timestamps are not expected in normal usage, but keep the
        // encoding parseable for robustness.
        format!("{prefix}:seg:{bucket}:{ts}")
    }
}

/// Return the KV prefix for listing all segments across all buckets.
/// Format: `{prefix}:seg:`
pub fn segment_prefix(prefix: &str) -> String {
    format!("{prefix}:seg:")
}

/// Return the KV prefix for listing segments in a specific bucket.
/// Format: `{prefix}:seg:{bucket}:`
pub fn bucket_prefix(prefix: &str, bucket: &str) -> String {
    format!("{prefix}:seg:{bucket}:")
}

/// Return the KV key for the segment-ID reverse index.
/// Format: `{prefix}:sid:{id}`
pub fn sid_key(prefix: &str, id: &str) -> String {
    format!("{prefix}:sid:{id}")
}

/// Encode bucket and timestamp into the sid reverse index value.
/// Format: `{bucket}:{ts_ns}`
pub fn sid_value(bucket: &str, ts: i64) -> String {
    format!("{bucket}:{ts}")
}

/// Decode a sid reverse index value into (bucket, timestamp).
/// Handles legacy format (just timestamp) by defaulting to "1h".
pub fn parse_sid_value(data: &[u8]) -> Result<(String, i64), String> {
    let s = std::str::from_utf8(data).map_err(|e| e.to_string())?;
    if let Some(idx) = s.find(':') {
        let bucket = s[..idx].to_string();
        let ts: i64 = s[idx + 1..]
            .parse()
            .map_err(|e: std::num::ParseIntError| e.to_string())?;
        Ok((bucket, ts))
    } else {
        let ts: i64 = s
            .parse()
            .map_err(|e: std::num::ParseIntError| e.to_string())?;
        Ok(("1h".to_string(), ts))
    }
}

/// Return the KV prefix for the graph sub-store.
/// Format: `{prefix}:g`
pub fn graph_prefix(prefix: &str) -> String {
    format!("{prefix}:g")
}

#[cfg(test)]
mod tests {
    use super::segment_key;

    #[test]
    fn segment_key_is_zero_padded() {
        let k = segment_key("mem:p1", "1h", 123);
        assert!(k.ends_with(":00000000000000000123"));
    }

    #[test]
    fn segment_key_keeps_lex_order_for_timestamps() {
        let k9 = segment_key("mem:p1", "1h", 9);
        let k10 = segment_key("mem:p1", "1h", 10);
        assert!(k9 < k10);
    }
}
