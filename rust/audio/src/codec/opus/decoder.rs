//! Opus decoder.

use std::ptr;
use super::ffi::{self, OpusDecoder as OpusDecoderHandle};
use super::frame::Frame;

/// Opus decoder error.
#[derive(Debug)]
pub enum DecoderError {
    /// Failed to create decoder.
    CreateFailed(String),
    /// Decoder is closed.
    Closed,
    /// Decoding failed.
    DecodeFailed(String),
}

impl std::fmt::Display for DecoderError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::CreateFailed(msg) => write!(f, "opus: decoder create failed: {}", msg),
            Self::Closed => write!(f, "opus: decoder is closed"),
            Self::DecodeFailed(msg) => write!(f, "opus: decode failed: {}", msg),
        }
    }
}

impl std::error::Error for DecoderError {}

/// Opus decoder.
pub struct Decoder {
    sample_rate: i32,
    channels: i32,
    handle: *mut OpusDecoderHandle,
}

// Safety: The decoder handle is not shared across threads.
unsafe impl Send for Decoder {}

impl Drop for Decoder {
    fn drop(&mut self) {
        if !self.handle.is_null() {
            unsafe { ffi::opus_decoder_destroy(self.handle) };
            self.handle = ptr::null_mut();
        }
    }
}

impl Decoder {
    /// Creates a new Opus decoder.
    ///
    /// # Parameters
    /// - `sample_rate`: Sample rate to decode at (8000, 12000, 16000, 24000, or 48000)
    /// - `channels`: Number of channels (1 or 2)
    pub fn new(sample_rate: i32, channels: i32) -> Result<Self, DecoderError> {
        let mut error: i32 = 0;
        let handle = unsafe {
            ffi::opus_decoder_create(sample_rate, channels, &mut error)
        };

        if handle.is_null() || error != ffi::OPUS_OK {
            return Err(DecoderError::CreateFailed(ffi::error_string(error)));
        }

        Ok(Self {
            sample_rate,
            channels,
            handle,
        })
    }

    /// Returns the sample rate.
    pub fn sample_rate(&self) -> i32 {
        self.sample_rate
    }

    /// Returns the number of channels.
    pub fn channels(&self) -> i32 {
        self.channels
    }

    /// Decodes an Opus frame to PCM samples.
    /// Returns the decoded PCM data as bytes (i16 samples, little-endian).
    pub fn decode(&mut self, frame: &Frame) -> Result<Vec<u8>, DecoderError> {
        if self.handle.is_null() {
            return Err(DecoderError::Closed);
        }

        // Max frame size: 120ms at 48kHz stereo = 5760 samples * 2 channels
        let max_samples = 5760 * self.channels;
        let mut buf = vec![0i16; max_samples as usize];

        let (data_ptr, data_len) = if frame.is_empty() {
            (ptr::null(), 0)
        } else {
            (frame.as_bytes().as_ptr(), frame.len() as i32)
        };

        let n = unsafe {
            ffi::opus_decode(
                self.handle,
                data_ptr,
                data_len,
                buf.as_mut_ptr(),
                max_samples / self.channels,
                0, // decode_fec
            )
        };

        if n < 0 {
            return Err(DecoderError::DecodeFailed(ffi::error_string(n)));
        }

        // Convert i16 samples to bytes
        let byte_len = (n as usize) * (self.channels as usize) * 2;
        let bytes = unsafe {
            std::slice::from_raw_parts(buf.as_ptr() as *const u8, byte_len)
        };

        Ok(bytes.to_vec())
    }

    /// Decodes to a provided buffer. Returns number of samples per channel.
    pub fn decode_to(&mut self, frame: &Frame, buf: &mut [i16]) -> Result<i32, DecoderError> {
        if self.handle.is_null() {
            return Err(DecoderError::Closed);
        }

        let (data_ptr, data_len) = if frame.is_empty() {
            (ptr::null(), 0)
        } else {
            (frame.as_bytes().as_ptr(), frame.len() as i32)
        };

        let n = unsafe {
            ffi::opus_decode(
                self.handle,
                data_ptr,
                data_len,
                buf.as_mut_ptr(),
                (buf.len() / self.channels as usize) as i32,
                0,
            )
        };

        if n < 0 {
            return Err(DecoderError::DecodeFailed(ffi::error_string(n)));
        }

        Ok(n)
    }

    /// Performs packet loss concealment (PLC) to generate samples when a packet is lost.
    pub fn decode_plc(&mut self, samples: i32) -> Result<Vec<u8>, DecoderError> {
        if self.handle.is_null() {
            return Err(DecoderError::Closed);
        }

        let mut buf = vec![0i16; (samples * self.channels) as usize];

        let n = unsafe {
            ffi::opus_decode(
                self.handle,
                ptr::null(),
                0,
                buf.as_mut_ptr(),
                samples,
                0,
            )
        };

        if n < 0 {
            return Err(DecoderError::DecodeFailed(ffi::error_string(n)));
        }

        let byte_len = (n as usize) * (self.channels as usize) * 2;
        let bytes = unsafe {
            std::slice::from_raw_parts(buf.as_ptr() as *const u8, byte_len)
        };

        Ok(bytes.to_vec())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use super::super::encoder::Encoder;

    #[test]
    fn test_decoder_create() {
        let decoder = Decoder::new(16000, 1);
        assert!(decoder.is_ok());
        let dec = decoder.unwrap();
        assert_eq!(dec.sample_rate(), 16000);
        assert_eq!(dec.channels(), 1);
    }

    #[test]
    fn test_encode_decode_roundtrip() {
        let mut encoder = Encoder::new_voip(16000, 1).unwrap();
        let mut decoder = Decoder::new(16000, 1).unwrap();

        // Generate test signal
        let pcm: Vec<i16> = (0..320).map(|i| (i * 100 % 32768) as i16).collect();
        
        let frame = encoder.encode(&pcm, 320).unwrap();
        let decoded = decoder.decode(&frame).unwrap();

        // Should decode to approximately same length
        assert_eq!(decoded.len(), 320 * 2); // 320 samples * 2 bytes
    }
}
