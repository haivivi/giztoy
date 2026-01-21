//! Audio resampling using libsoxr.
//!
//! This module provides high-quality sample rate conversion using the SoX
//! Resampler library (libsoxr).
//!
//! # Example
//!
//! ```ignore
//! use giztoy_audio::resampler::{Format, Soxr};
//! use std::io::Read;
//!
//! let src_fmt = Format { sample_rate: 44100, stereo: false };
//! let dst_fmt = Format { sample_rate: 16000, stereo: false };
//!
//! let input = std::io::Cursor::new(audio_data);
//! let mut resampler = Soxr::new(input, src_fmt, dst_fmt)?;
//!
//! let mut output = vec![0u8; 4096];
//! let n = resampler.read(&mut output)?;
//! ```

mod ffi;
pub mod format;
mod soxr;
mod sample_reader;

pub use format::*;
pub use soxr::*;
