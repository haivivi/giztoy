//! JSON utility functions.

/// Deserialize JSON with basic repair for common malformations.
///
/// If the initial parse fails, attempts to fix trailing commas and
/// single quotes before retrying.
pub fn unmarshal_json<T: serde::de::DeserializeOwned>(data: &[u8]) -> Result<T, serde_json::Error> {
    match serde_json::from_slice(data) {
        Ok(v) => Ok(v),
        Err(e) => {
            let s = String::from_utf8_lossy(data);
            let fixed = repair_json(&s);
            match serde_json::from_str(&fixed) {
                Ok(v) => Ok(v),
                Err(_) => Err(e),
            }
        }
    }
}

fn repair_json(s: &str) -> String {
    let mut result = String::with_capacity(s.len());
    let mut in_string = false;
    let mut escape_next = false;
    let chars: Vec<char> = s.chars().collect();

    for i in 0..chars.len() {
        let ch = chars[i];

        if escape_next {
            result.push(ch);
            escape_next = false;
            continue;
        }

        if ch == '\\' && in_string {
            result.push(ch);
            escape_next = true;
            continue;
        }

        if ch == '"' {
            in_string = !in_string;
            result.push(ch);
            continue;
        }

        if !in_string && ch == '\'' {
            result.push('"');
            continue;
        }

        if !in_string && ch == ',' {
            // Skip trailing commas before } or ]
            let rest = chars[i + 1..].iter().collect::<String>();
            let trimmed = rest.trim_start();
            if trimmed.starts_with('}') || trimmed.starts_with(']') {
                continue;
            }
        }

        result.push(ch);
    }

    result
}

/// Generate a random hex string of the given byte length (output is 2*n chars).
pub fn hex_string(n: usize) -> String {
    let mut buf = vec![0u8; n];
    getrandom::fill(&mut buf).expect("getrandom failed");
    hex::encode(buf)
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde::Deserialize;

    #[test]
    fn t4_1_hex_string_length() {
        let s = hex_string(8);
        assert_eq!(s.len(), 16);
        assert!(s.chars().all(|c| c.is_ascii_hexdigit()));
    }

    #[test]
    fn t4_2_normal_json() {
        #[derive(Deserialize, Debug, PartialEq)]
        struct T {
            name: String,
        }
        let result: T = unmarshal_json(br#"{"name":"hello"}"#).unwrap();
        assert_eq!(result.name, "hello");
    }

    #[test]
    fn t4_3_trailing_comma() {
        #[derive(Deserialize, Debug, PartialEq)]
        struct T {
            a: i32,
            b: i32,
        }
        let result: T = unmarshal_json(br#"{"a": 1, "b": 2,}"#).unwrap();
        assert_eq!(result.a, 1);
        assert_eq!(result.b, 2);
    }

    #[test]
    fn t4_4_single_quotes() {
        #[derive(Deserialize, Debug, PartialEq)]
        struct T {
            name: String,
        }
        let result: T = unmarshal_json(b"{'name': 'world'}").unwrap();
        assert_eq!(result.name, "world");
    }

    #[test]
    fn t4_5_truncated_json() {
        let result: Result<serde_json::Value, _> = unmarshal_json(br#"{"name": "hel"#);
        // Truncated JSON may or may not be repairable; we just ensure no panic
        let _ = result;
    }
}
