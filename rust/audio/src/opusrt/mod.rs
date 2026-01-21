//! Real-time Opus streaming utilities.
//!
//! This module provides utilities for real-time Opus audio streaming:
//!
//! - `Frame`: Opus frame with duration calculation from TOC
//! - `EpochMillis`: Millisecond timestamp helper
//! - `StampedFrame`: Frame with embedded timestamp
//! - `Buffer`: Jitter buffer for reordering out-of-order frames
//!
//! # Example
//!
//! ```ignore
//! use giztoy_audio::opusrt::{Buffer, Frame, EpochMillis};
//! use std::time::Duration;
//!
//! // Create a jitter buffer with 2 second capacity
//! let mut buffer = Buffer::new(Duration::from_secs(2));
//!
//! // Append frames with timestamps
//! buffer.append(frame1, EpochMillis::from_millis(0));
//! buffer.append(frame2, EpochMillis::from_millis(20));
//!
//! // Read frames in order
//! while let Ok((frame, loss)) = buffer.frame() {
//!     if loss.as_millis() > 0 {
//!         // Packet loss detected
//!     }
//! }
//! ```

mod timestamp;
mod frame;
mod buffer;

pub use timestamp::*;
pub use frame::*;
pub use buffer::*;
