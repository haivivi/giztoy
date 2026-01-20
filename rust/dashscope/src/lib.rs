//! DashScope (Aliyun Model Studio) API SDK for Rust.
//!
//! This crate provides a client for interacting with the DashScope API,
//! including the Qwen-Omni-Realtime multimodal conversation API.
//!
//! # Example
//!
//! ```rust,no_run
//! use giztoy_dashscope::{Client, RealtimeConfig, ModelQwenOmniTurboRealtimeLatest};
//!
//! #[tokio::main]
//! async fn main() -> Result<(), Box<dyn std::error::Error>> {
//!     let client = Client::new("your-api-key")?;
//!     
//!     let session = client.realtime().connect(&RealtimeConfig {
//!         model: ModelQwenOmniTurboRealtimeLatest.to_string(),
//!     }).await?;
//!     
//!     // Send audio, receive events...
//!     Ok(())
//! }
//! ```

mod client;
mod error;
mod event;
mod realtime;
mod types;

pub use client::{Client, ClientBuilder, DEFAULT_REALTIME_URL, DEFAULT_HTTP_BASE_URL};
pub use error::{Error, Result};
pub use event::*;
pub use realtime::{RealtimeService, RealtimeSession, RealtimeConfig, ResponseCreateOptions, SimpleMessage};
pub use types::*;
