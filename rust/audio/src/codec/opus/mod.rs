//! Opus audio codec.
//!
//! This module implements the Opus codec specification as defined in RFC 6716.
//! It provides TOC (Table of Contents) parsing, frame handling, and encoding/decoding
//! capabilities using libopus via FFI.
//!
//! # Example
//!
//! ```ignore
//! use giztoy_audio::codec::opus::{Encoder, Decoder, Application};
//!
//! // Create an encoder
//! let mut encoder = Encoder::new(16000, 1, Application::VoIP)?;
//! encoder.set_bitrate(24000)?;
//!
//! // Encode PCM samples
//! let pcm: Vec<i16> = vec![0i16; 320]; // 20ms at 16kHz
//! let frame = encoder.encode(&pcm, 320)?;
//!
//! // Create a decoder
//! let mut decoder = Decoder::new(16000, 1)?;
//! let decoded = decoder.decode(&frame)?;
//! ```

mod ffi;
mod encoder;
mod decoder;
mod frame;
mod toc;

pub use encoder::*;
pub use decoder::*;
pub use frame::*;
pub use toc::*;
