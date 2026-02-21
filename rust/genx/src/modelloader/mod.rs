//! Model configuration loading and registration.
//!
//! Loads YAML/JSON config files and registers generators, segmentors,
//! and profilers to their respective Mux instances.

mod config;

pub use config::*;
