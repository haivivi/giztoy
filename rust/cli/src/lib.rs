//! CLI utilities for giztoy.
//!
//! This crate provides common utilities for CLI applications.

pub mod config;
pub mod output;

pub use config::{Config, Context, ClientCredentials, ConsoleCredentials};
pub use output::{Output, OutputFormat, print_verbose, guess_extension};
