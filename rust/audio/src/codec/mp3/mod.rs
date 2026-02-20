//! MP3 audio codec.
//!
//! This module provides MP3 encoding using LAME and decoding using minimp3.
//!
//! # Convenience Functions
//!
//! - [`encode_pcm_stream`]: Encode an entire PCM stream to MP3
//! - [`decode_full`]: Decode an entire MP3 stream to PCM

mod ffi;
mod encoder;
mod decoder;

pub use encoder::*;
pub use decoder::*;

use std::io::{self, Read, Write};

/// Encodes an entire PCM stream to MP3.
///
/// Reads PCM data from `pcm` and writes encoded MP3 to `w`.
/// Returns the total number of PCM input bytes consumed.
pub fn encode_pcm_stream(
    w: &mut dyn Write,
    pcm: &mut dyn Read,
    sample_rate: i32,
    channels: i32,
    opts: Option<EncoderOptions>,
) -> Result<u64, EncoderError> {
    let opts = opts.unwrap_or_default();
    let mut encoder = Mp3Encoder::new(w, sample_rate, channels, opts)?;

    let mut buf = vec![0u8; 4096];
    let mut total = 0u64;
    loop {
        let n = pcm.read(&mut buf).map_err(EncoderError::Io)?;
        if n == 0 {
            break;
        }
        encoder.write_all(&buf[..n]).map_err(EncoderError::Io)?;
        total += n as u64;
    }

    encoder.close()?;
    Ok(total)
}

/// Decodes an entire MP3 stream to PCM.
///
/// Returns `(pcm_bytes, sample_rate, channels)`.
pub fn decode_full(r: impl Read) -> Result<(Vec<u8>, i32, i32), DecoderError> {
    let mut decoder = Mp3Decoder::new(r);
    let mut pcm = Vec::new();
    let mut buf = [0u8; 8192];

    loop {
        match decoder.read(&mut buf) {
            Ok(0) => break,
            Ok(n) => pcm.extend_from_slice(&buf[..n]),
            Err(e) if e.kind() == io::ErrorKind::UnexpectedEof => break,
            Err(e) => return Err(DecoderError::Io(e)),
        }
    }

    let sample_rate = decoder.sample_rate();
    let channels = decoder.channels();

    Ok((pcm, sample_rate, channels))
}
