//! Core types for device state, statistics, and commands.
//!
//! This crate provides the fundamental types used for communication between
//! devices (gears) and servers in the chatgear protocol:
//!
//! - [`GearState`] and [`GearStateEvent`]: Device state management
//! - [`SessionCommand`] and command types: Device control commands
//! - [`GearStatsEvent`]: Device statistics and telemetry
//! - Connection traits ([`UplinkTx`], [`UplinkRx`], [`DownlinkTx`], [`DownlinkRx`])
//! - Port traits ([`ClientPortTx`], [`ClientPortRx`], [`ServerPortTx`], [`ServerPortRx`])
//! - Port implementations ([`ClientPort`], [`ServerPort`])
//! - Pipe for testing ([`new_pipe`])
//!
//! # Example
//!
//! ```rust
//! use giztoy_chatgear::{GearState, GearStateEvent};
//!
//! let event = GearStateEvent::new(GearState::Ready);
//! assert!(event.state.can_record());
//! ```

mod state;
mod command;
mod stats;
mod conn;
pub mod logger;
mod port;
pub mod conn_mqtt;
pub mod conn_mqtt_server;
pub mod conn_pipe;
pub mod listener;
mod client_port;
mod server_port;

pub use state::*;
pub use command::*;
pub use stats::*;
pub use conn::*;
pub use port::*;
pub use conn_pipe::new_pipe;
pub use client_port::ClientPort;
pub use server_port::ServerPort;

#[cfg(test)]
mod tests;
