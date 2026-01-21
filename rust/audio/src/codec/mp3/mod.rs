//! MP3 audio codec.
//!
//! This module provides MP3 encoding using LAME and decoding using minimp3.

mod ffi;
mod encoder;
mod decoder;

pub use encoder::*;
pub use decoder::*;
