//! MQTT client and server library.
//!
//! This crate provides an MQTT client (using rumqttc) and server (using rumqttd)
//! with a consistent API similar to the Go implementation.
//!
//! # Example - Server
//!
//! ```no_run
//! use giztoy_mqtt::{Server, ServerConfig, ServeMux};
//!
//! #[tokio::main]
//! async fn main() -> anyhow::Result<()> {
//!     let mux = ServeMux::new();
//!     mux.handle_func("test/+/data", |msg| {
//!         println!("Received: {:?}", msg.payload);
//!         Ok(())
//!     });
//!
//!     let config = ServerConfig::new("127.0.0.1:1883");
//!     let server = Server::new(config, Some(mux));
//!     server.serve().await?;
//!     Ok(())
//! }
//! ```
//!
//! # Example - Client
//!
//! ```no_run
//! use giztoy_mqtt::{Dialer, ServeMux};
//!
//! #[tokio::main]
//! async fn main() -> anyhow::Result<()> {
//!     let mux = ServeMux::new();
//!     mux.handle_func("response/#", |msg| {
//!         println!("Received: {:?}", msg.payload);
//!         Ok(())
//!     });
//!
//!     let dialer = Dialer::new().with_serve_mux(mux);
//!     let conn = dialer.dial("mqtt://127.0.0.1:1883").await?;
//!
//!     conn.subscribe("response/#").await?;
//!     conn.write_to_topic(b"hello", "test/topic").await?;
//!
//!     conn.close().await?;
//!     Ok(())
//! }
//! ```

mod client;
mod error;
mod serve_mux;
mod server;
#[cfg(test)]
mod tests;
mod topic;
mod trie;
mod types;

pub use client::{Conn, Dialer, DialOption, SubscribeOption, WriteOption};
pub use error::{Error, Result};
pub use serve_mux::{Handler, HandlerFunc, Message, ServeMux};
pub use server::{Authenticator, Server, ServerConfig};
pub use topic::{TopicSubscriber, TopicWriter};
pub use trie::Trie;
pub use types::QoS;

/// Re-export commonly used items
pub mod prelude {
    pub use crate::{
        Authenticator, Conn, Dialer, Error, Handler, HandlerFunc, Message, QoS, Result,
        ServeMux, Server, ServerConfig, TopicSubscriber, TopicWriter,
    };
}
