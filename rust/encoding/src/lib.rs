//! JSON-serializable encoding types.
//!
//! This crate provides byte slice types that serialize to/from specific encodings in JSON:
//!
//! - [`StdBase64Data`]: Standard Base64 encoding
//! - [`HexData`]: Hexadecimal encoding
//!
//! # Example
//!
//! ```rust
//! use giztoy_encoding::{StdBase64Data, HexData};
//!
//! // StdBase64Data serializes to base64
//! let data = StdBase64Data::from(b"hello world".as_slice());
//! let json = serde_json::to_string(&data).unwrap();
//! assert_eq!(json, r#""aGVsbG8gd29ybGQ=""#);
//!
//! // HexData serializes to hex
//! let data = HexData::from(vec![0xde, 0xad, 0xbe, 0xef]);
//! let json = serde_json::to_string(&data).unwrap();
//! assert_eq!(json, r#""deadbeef""#);
//! ```

mod base64_data;
mod hex_data;

pub use base64_data::StdBase64Data;
pub use hex_data::HexData;

#[cfg(test)]
mod tests;
