//! CLI utilities for giztoy.
//!
//! This crate provides common utilities for CLI applications:
//!
//! - [`Config`]: Configuration management
//! - [`Output`]: Output formatting (JSON/YAML)
//! - [`Paths`]: Directory structure management
//! - [`load_request`]: Request loading from YAML/JSON files
//! - [`LogWriter`]: Log capture for TUI display

pub mod config;
pub mod output;
pub mod paths;
pub mod request;
pub mod log_writer;

pub use config::{Config, Context, ClientCredentials, ConsoleCredentials};
pub use output::{Output, OutputFormat, print_verbose, guess_extension};
pub use paths::{Paths, DEFAULT_BASE_DIR, DEFAULT_CONFIG_FILE};
pub use request::{load_request, parse_request, load_request_from_stdin, must_load_request, RequestError};
pub use log_writer::{LogWriter, LogBuffer, SyncLogWriter, new_log_buffer};