//! CLI commands module.

mod config;
mod omni;
mod util;

pub use config::ConfigCommand;
pub use omni::OmniCommand;

// Re-export utils for use in commands
pub(crate) use util::*;
