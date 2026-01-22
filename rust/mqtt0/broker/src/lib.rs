//! Lightweight QoS 0 MQTT broker with full ACL support.
//!
//! This crate provides an MQTT broker that supports both MQTT 3.1.1 and MQTT 5.0,
//! with complete control over authentication and ACL.
//!
//! ## Features
//!
//! - `tokio` (default): Use tokio async runtime
//! - `tls`: TLS support
//! - `websocket`: WebSocket support
//!
//! ## Example
//!
//! ```ignore
//! use mqtt0_broker::{Broker, BrokerConfig};
//!
//! #[tokio::main]
//! async fn main() -> mqtt0_broker::Result<()> {
//!     let broker = Broker::new(BrokerConfig::new("127.0.0.1:1883"));
//!     broker.serve().await
//! }
//! ```

mod broker;
mod error;
mod trie;
mod types;

pub use broker::{Broker, BrokerBuilder, BrokerConfig};
pub use error::{Error, Result};
pub use trie::{Trie, TrieNode};
pub use types::{AllowAll, Authenticator, Handler};

// Re-export Message from mqtt0
pub use mqtt0::types::Message;
