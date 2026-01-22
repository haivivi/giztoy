//! Lightweight QoS 0 MQTT protocol library.
//!
//! This crate provides MQTT protocol types and encoding/decoding utilities
//! that are compatible with both `std` and `no_std` environments.
//!
//! ## Features
//!
//! - `std` (default): Enable standard library support
//! - `alloc`: Enable heap allocation (required for most operations)
//! - `rumqttc-compat`: Use rumqttc's mqttbytes for parsing (std only)
//!
//! ## no_std Support
//!
//! To use this crate in a `no_std` environment:
//!
//! ```toml
//! [dependencies]
//! mqtt0 = { version = "0.1", default-features = false, features = ["alloc"] }
//! ```

#![cfg_attr(not(feature = "std"), no_std)]

#[cfg(feature = "alloc")]
extern crate alloc;

pub mod error;
pub mod protocol;
pub mod types;

pub use error::{Error, Result};
pub use types::{Message, ProtocolVersion, QoS};

// Re-export protocol modules
pub use protocol::{v4, v5};
