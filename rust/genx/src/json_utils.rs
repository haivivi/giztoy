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
    let mut string_quote: Option<char> = None; // which quote opened the current string
    let mut escape_next = false;
    let chars: Vec<char> = s.chars().collect();

    for i in 0..chars.len() {
        let ch = chars[i];
        let in_string = string_quote.is_some();

        if escape_next {
            escape_next = false;
            if string_quote == Some('\'') && ch == '\'' {
                // \' inside single-quoted string: single quotes don't need
                // escaping in double-quoted JSON, so just emit '
                // (the backslash was not pushed — see below)
                result.push('\'');
            } else if string_quote == Some('\'') {
                // other escapes inside single-quoted string: emit backslash + char
                result.push('\\');
                result.push(ch);
            } else {
                result.push(ch);
            }
            continue;
        }

        if ch == '\\' && in_string {
            if string_quote == Some('\'') {
                // In single-quoted string: defer backslash to escape_next handler
                escape_next = true;
            } else {
                result.push(ch);
                escape_next = true;
            }
            continue;
        }

        if ch == '"' && string_quote == Some('"') {
            string_quote = None;
            result.push(ch);
            continue;
        }

        if ch == '"' && !in_string {
            string_quote = Some('"');
            result.push(ch);
            continue;
        }

        if ch == '\'' && string_quote == Some('\'') {
            string_quote = None;
            result.push('"');
            continue;
        }

        if ch == '\'' && !in_string {
            string_quote = Some('\'');
            result.push('"');
            continue;
        }

        if !in_string && ch == ',' {
            let is_trailing = chars[i + 1..]
                .iter()
                .find(|c| !c.is_ascii_whitespace())
                .is_some_and(|c| *c == '}' || *c == ']');
            if is_trailing {
                continue;
            }
        }

        if string_quote == Some('\'') && ch == '"' {
            result.push('\\');
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
        let _ = result;
    }

    #[test]
    fn t4_6_single_quote_with_comma_in_value() {
        #[derive(Deserialize, Debug, PartialEq)]
        struct T {
            k: String,
        }
        let result: T = unmarshal_json(b"{'k': 'a, }'}").unwrap();
        assert_eq!(result.k, "a, }");
    }

    #[test]
    fn t4_7_double_quote_with_single_inside() {
        #[derive(Deserialize, Debug, PartialEq)]
        struct T {
            k: String,
        }
        let result: T = unmarshal_json(br#"{"k": "it's fine"}"#).unwrap();
        assert_eq!(result.k, "it's fine");
    }

    #[test]
    fn t4_9_escaped_single_quote_in_single_quoted_string() {
        #[derive(Deserialize, Debug, PartialEq)]
        struct T {
            k: String,
        }
        // {'k': 'it\'s fine'} → {"k": "it's fine"}
        let result: T = unmarshal_json(b"{'k': 'it\\'s fine'}").unwrap();
        assert_eq!(result.k, "it's fine");
    }

    #[test]
    fn t4_10_escaped_single_quote_apostrophe() {
        #[derive(Deserialize, Debug, PartialEq)]
        struct T {
            name: String,
        }
        let result: T = unmarshal_json(b"{'name': 'O\\'Brien'}").unwrap();
        assert_eq!(result.name, "O'Brien");
    }

    #[test]
    fn t4_8_double_quote_inside_single_quoted_string() {
        #[derive(Deserialize, Debug, PartialEq)]
        struct T {
            k: String,
        }
        // {'k': 'he said "hi"'} should become {"k": "he said \"hi\""}
        let result: T = unmarshal_json(b"{'k': 'he said \"hi\"'}").unwrap();
        assert_eq!(result.k, r#"he said "hi""#);
    }
}
