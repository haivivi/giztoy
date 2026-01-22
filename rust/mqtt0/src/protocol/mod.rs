//! MQTT protocol encoding and decoding.
//!
//! This module provides packet encoding and decoding for both
//! MQTT 3.1.1 (v4) and MQTT 5.0 (v5).

pub mod codec;
pub mod v4;
pub mod v5;

pub use codec::*;

/// Maximum packet size (1MB default).
pub const MAX_PACKET_SIZE: usize = 1024 * 1024;

/// Protocol name for MQTT.
pub const PROTOCOL_NAME: &[u8] = b"MQTT";
