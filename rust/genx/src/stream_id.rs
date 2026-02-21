//! Stream ID generation.
//!
//! Format: base62(seconds_since_2025) + base62(random_6bytes)
//! Length: ~14 characters (6 for time + 8 for random)

use std::time::{SystemTime, UNIX_EPOCH};

const EPOCH_2025: u64 = 1735689600;

const BASE62_CHARS: &[u8] = b"0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz";

/// Generate a short unique stream identifier.
///
/// The time component ensures IDs are roughly time-ordered,
/// reducing collision probability in long-running systems.
pub fn new_stream_id() -> String {
    let secs = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs()
        .saturating_sub(EPOCH_2025) as u32;
    let time_part = base62_encode_u32(secs);

    let mut random_bytes = [0u8; 6];
    getrandom::fill(&mut random_bytes).expect("getrandom failed");
    let random_part = base62_encode_bytes(&random_bytes);

    format!("{}{}", time_part, random_part)
}

fn base62_encode_u32(mut n: u32) -> String {
    if n == 0 {
        return "0".into();
    }
    let mut result = Vec::new();
    while n > 0 {
        result.push(BASE62_CHARS[(n % 62) as usize]);
        n /= 62;
    }
    result.reverse();
    String::from_utf8(result).unwrap()
}

fn base62_encode_bytes(data: &[u8]) -> String {
    if data.is_empty() {
        return String::new();
    }
    let mut n: u64 = 0;
    for &b in data {
        n = n * 256 + b as u64;
    }
    if n == 0 {
        return "0".into();
    }
    let mut result = Vec::new();
    while n > 0 {
        result.push(BASE62_CHARS[(n % 62) as usize]);
        n /= 62;
    }
    result.reverse();
    String::from_utf8(result).unwrap()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashSet;

    #[test]
    fn t2_1_non_empty() {
        let id = new_stream_id();
        assert!(!id.is_empty());
    }

    #[test]
    fn t2_2_reasonable_length() {
        let id = new_stream_id();
        assert!(id.len() >= 10 && id.len() <= 20, "len={}", id.len());
    }

    #[test]
    fn t2_3_no_duplicates() {
        let mut seen = HashSet::new();
        for _ in 0..1000 {
            let id = new_stream_id();
            assert!(seen.insert(id.clone()), "duplicate: {}", id);
        }
    }

    #[test]
    fn t2_4_only_base62_chars() {
        for _ in 0..100 {
            let id = new_stream_id();
            for ch in id.chars() {
                assert!(
                    ch.is_ascii_alphanumeric(),
                    "non-base62 char '{}' in '{}'",
                    ch,
                    id
                );
            }
        }
    }

    #[test]
    fn t2_base62_encode_u32_zero() {
        assert_eq!(base62_encode_u32(0), "0");
    }

    #[test]
    fn t2_base62_encode_u32_small() {
        assert_eq!(base62_encode_u32(61), "z");
        assert_eq!(base62_encode_u32(62), "10");
    }
}
