//! Audio codec implementations.
//!
//! This module provides encoding and decoding for various audio formats:
//!
//! - `opus`: Opus audio codec (RFC 6716)
//! - `ogg`: Ogg container format (RFC 3533)
//! - `mp3`: MP3 encoding/decoding

pub mod opus;
pub mod ogg;
pub mod mp3;
