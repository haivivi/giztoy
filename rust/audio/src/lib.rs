//! Audio processing utilities.
//!
//! This crate provides audio processing utilities including:
//!
//! - `pcm`: PCM (Pulse Code Modulation) audio format handling and mixing
//! - `songs`: Built-in melodies for testing audio playback
//!
//! # Example
//!
//! ```rust
//! use giztoy_audio::pcm::{Format, Chunk};
//! use std::time::Duration;
//!
//! // Create a 16kHz mono format
//! let format = Format::L16Mono16K;
//!
//! // Calculate bytes needed for 20ms of audio
//! let bytes = format.bytes_in_duration(Duration::from_millis(20));
//!
//! // Create a silence chunk
//! let silence = format.silence_chunk(Duration::from_millis(100));
//!
//! // Create a data chunk
//! let data = vec![0i16; 1600]; // 100ms at 16kHz
//! let chunk = format.data_chunk_from_samples(&data);
//! ```

pub mod codec;
pub mod opusrt;
pub mod pcm;
pub mod resampler;
pub mod songs;

pub use pcm::Format;
