//! JSON-serializable time types.
//!
//! This crate provides time types that serialize to/from numeric values in JSON:
//!
//! - [`Milli`]: Unix milliseconds timestamp
//! - [`Unix`]: Unix seconds timestamp
//! - [`Duration`]: Duration that serializes to string (e.g., "1h30m") or nanoseconds
//!
//! # Example
//!
//! ```rust
//! use giztoy_jsontime::{Milli, Unix, Duration};
//! use std::time::Duration as StdDuration;
//!
//! // Milli serializes to milliseconds
//! let now = Milli::now();
//! let json = serde_json::to_string(&now).unwrap();
//! // => "1705315800000"
//!
//! // Unix serializes to seconds
//! let now = Unix::now();
//! let json = serde_json::to_string(&now).unwrap();
//! // => "1705315800"
//!
//! // Duration serializes to string
//! let dur = Duration::from(StdDuration::from_secs(5400));
//! let json = serde_json::to_string(&dur).unwrap();
//! // => "\"1h30m0s\""
//! ```

mod milli;
mod unix;
mod duration;

pub use milli::Milli;
pub use unix::Unix;
pub use duration::Duration;

#[cfg(test)]
mod tests;
