//! Lightweight QoS 0 MQTT client and broker with full ACL support.
//!
//! This crate provides a minimal MQTT implementation that only supports QoS 0,
//! but with complete control over authentication and ACL at every step:
//!
//! - **Connect**: Authenticate client credentials
//! - **Publish**: Check write permission for topic
//! - **Subscribe**: Check read permission for topic
//!
//! ## Components
//!
//! - [`Client`]: QoS 0 MQTT client (mqtt0c)
//! - [`Broker`]: QoS 0 MQTT broker (mqtt0d)
//!
//! ## Example
//!
//! ```no_run
//! use mqtt0::{Broker, BrokerConfig, Client, ClientConfig};
//!
//! #[tokio::main]
//! async fn main() -> mqtt0::Result<()> {
//!     // Start broker
//!     let broker = Broker::new(BrokerConfig::new("127.0.0.1:1883"));
//!     tokio::spawn(async move { broker.serve().await });
//!
//!     // Connect client
//!     let mut client = Client::connect(ClientConfig::new("127.0.0.1:1883", "client-1")).await?;
//!
//!     // Subscribe and publish
//!     client.subscribe(&["test/topic"]).await?;
//!     client.publish("test/topic", b"hello").await?;
//!
//!     // Receive message
//!     let msg = client.recv().await?;
//!     println!("Received: {:?}", msg);
//!
//!     Ok(())
//! }
//! ```

mod broker;
mod client;
mod error;
mod protocol;
pub mod transport;
pub mod trie;
mod types;

pub use broker::{Broker, BrokerConfig, BrokerBuilder};
pub use client::{Client, ClientConfig};
pub use error::{Error, Result};
pub use transport::TransportType;
pub use types::{Authenticator, Handler, Message, ProtocolVersion, QoS};

#[cfg(feature = "tls")]
pub use transport::tls::TlsConfig;

#[cfg(test)]
mod tests;
