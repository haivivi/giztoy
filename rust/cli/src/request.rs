//! Request loading utilities.

use serde::de::DeserializeOwned;
use std::fs;
use std::io::{self, Read};
use std::path::Path;
use thiserror::Error;

/// Error type for request loading.
#[derive(Debug, Error)]
pub enum RequestError {
    #[error("failed to read file: {0}")]
    ReadFile(#[from] io::Error),
    #[error("failed to parse YAML: {0}")]
    ParseYaml(#[from] serde_yaml::Error),
    #[error("failed to parse JSON: {0}")]
    ParseJson(#[from] serde_json::Error),
    #[error("failed to parse file (tried YAML and JSON)")]
    ParseFailed,
}

/// Loads a request from a YAML or JSON file into the provided type.
pub fn load_request<T: DeserializeOwned>(path: impl AsRef<Path>) -> Result<T, RequestError> {
    let data = fs::read(path.as_ref())?;
    parse_request(&data, path.as_ref())
}

/// Parses request data based on file extension or content.
pub fn parse_request<T: DeserializeOwned>(data: &[u8], path: impl AsRef<Path>) -> Result<T, RequestError> {
    let ext = path
        .as_ref()
        .extension()
        .and_then(|e| e.to_str())
        .map(|e| e.to_lowercase());

    match ext.as_deref() {
        Some("yaml") | Some("yml") => {
            Ok(serde_yaml::from_slice(data)?)
        }
        Some("json") => {
            Ok(serde_json::from_slice(data)?)
        }
        _ => {
            // Try YAML first, then JSON
            if let Ok(v) = serde_yaml::from_slice(data) {
                return Ok(v);
            }
            if let Ok(v) = serde_json::from_slice(data) {
                return Ok(v);
            }
            Err(RequestError::ParseFailed)
        }
    }
}

/// Loads a request from stdin.
pub fn load_request_from_stdin<T: DeserializeOwned>() -> Result<T, RequestError> {
    let mut data = Vec::new();
    io::stdin().read_to_end(&mut data)?;

    // Try JSON first for stdin, then YAML
    if let Ok(v) = serde_json::from_slice(&data) {
        return Ok(v);
    }
    if let Ok(v) = serde_yaml::from_slice(&data) {
        return Ok(v);
    }
    Err(RequestError::ParseFailed)
}

/// Loads a request or panics with error message.
pub fn must_load_request<T: DeserializeOwned>(path: impl AsRef<Path>) -> T {
    match load_request(path) {
        Ok(v) => v,
        Err(e) => {
            eprintln!("Failed to load request: {}", e);
            std::process::exit(1);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde::Deserialize;
    use std::io::Write;
    use tempfile::NamedTempFile;

    #[derive(Debug, Deserialize, PartialEq)]
    struct TestRequest {
        name: String,
        value: i32,
    }

    #[test]
    fn test_load_yaml() {
        let mut file = NamedTempFile::with_suffix(".yaml").unwrap();
        writeln!(file, "name: test\nvalue: 42").unwrap();

        let req: TestRequest = load_request(file.path()).unwrap();
        assert_eq!(req.name, "test");
        assert_eq!(req.value, 42);
    }

    #[test]
    fn test_load_json() {
        let mut file = NamedTempFile::with_suffix(".json").unwrap();
        writeln!(file, r#"{{"name": "test", "value": 42}}"#).unwrap();

        let req: TestRequest = load_request(file.path()).unwrap();
        assert_eq!(req.name, "test");
        assert_eq!(req.value, 42);
    }

    #[test]
    fn test_parse_unknown_extension() {
        let data = b"name: test\nvalue: 42";
        let req: TestRequest = parse_request(data, "file.txt").unwrap();
        assert_eq!(req.name, "test");
        assert_eq!(req.value, 42);
    }

    #[test]
    fn test_parse_invalid() {
        let data = b"invalid data {{{{";
        let result: Result<TestRequest, _> = parse_request(data, "file.txt");
        assert!(matches!(result, Err(RequestError::ParseFailed)));
    }
}
