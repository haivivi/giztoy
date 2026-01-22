//! Lightweight QoS 0 MQTT client.
//!
//! This crate provides an MQTT client that supports both `tokio` and `embassy` runtimes.
//!
//! ## Features
//!
//! - `tokio` (default): Use tokio async runtime
//! - `embassy`: Use embassy async runtime (for embedded systems)
//!
//! ## Example (tokio)
//!
//! ```ignore
//! use mqtt0_client::{Client, ClientConfig};
//!
//! #[tokio::main]
//! async fn main() -> mqtt0_client::Result<()> {
//!     let client = Client::connect(
//!         ClientConfig::new("127.0.0.1:1883", "my-client")
//!     ).await?;
//!
//!     client.subscribe(&["test/topic"]).await?;
//!     client.publish("test/topic", b"hello").await?;
//!
//!     let msg = client.recv().await?;
//!     println!("Received: {:?}", msg);
//!
//!     Ok(())
//! }
//! ```

#![cfg_attr(not(feature = "tokio"), no_std)]

#[cfg(feature = "tokio")]
extern crate std;

#[cfg(all(not(feature = "tokio"), feature = "embassy"))]
extern crate alloc;

mod config;
mod error;

#[cfg(feature = "tokio")]
mod tokio_client;

#[cfg(feature = "embassy")]
mod embassy_client;

pub use config::ClientConfig;
pub use error::{Error, Result};
pub use mqtt0::types::{Message, ProtocolVersion, QoS};

// Re-export the appropriate client based on features
#[cfg(feature = "tokio")]
pub use tokio_client::Client;

#[cfg(all(not(feature = "tokio"), feature = "embassy"))]
pub use embassy_client::Client;
