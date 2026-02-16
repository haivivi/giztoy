//! PCM (Pulse Code Modulation) audio format handling.
//!
//! This module provides types and utilities for working with PCM audio data,
//! including format definitions, audio chunks, and a multi-track mixer.
//!
//! # Key Types
//!
//! - [`Format`]: Represents audio format (sample rate, channels, bit depth)
//! - [`Chunk`]: Trait for audio data chunks
//! - [`DataChunk`]: Concrete implementation of Chunk for raw audio data
//! - [`SilenceChunk`]: Chunk that produces silence of a specified duration
//! - [`Mixer`]: Multi-track audio mixer with gain control
//!
//! # Example
//!
//! ```rust
//! use giztoy_audio::pcm::{Format, Mixer, MixerOptions};
//! use std::time::Duration;
//!
//! // Create a mixer with 16kHz output
//! let mut mixer = Mixer::new(Format::L16Mono16K, MixerOptions::default());
//!
//! // Create tracks and write audio data
//! let (track, ctrl) = mixer.create_track(None).unwrap();
//! ```

mod format;
mod chunk;
pub mod io;
mod mixer;
mod atomic;
pub(crate) mod track;

pub use format::{Format, FormatExt};
pub use chunk::{Chunk, DataChunk, SilenceChunk};
pub use mixer::{Mixer, MixerOptions, Track, TrackCtrl, TrackOptions, TrackWriter};
pub use atomic::AtomicF32;
