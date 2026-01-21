//! Opus encoder.

use std::ptr;
use super::ffi::{self, OpusEncoder as OpusEncoderHandle};
use super::frame::Frame;
use super::toc::FrameDuration;

/// Opus application type.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Application {
    /// Best quality for voice signals.
    VoIP,
    /// Best quality for non-voice signals.
    Audio,
    /// Minimum possible coding delay.
    RestrictedLowdelay,
}

impl Application {
    fn to_ffi(&self) -> i32 {
        match self {
            Self::VoIP => ffi::OPUS_APPLICATION_VOIP,
            Self::Audio => ffi::OPUS_APPLICATION_AUDIO,
            Self::RestrictedLowdelay => ffi::OPUS_APPLICATION_RESTRICTED_LOWDELAY,
        }
    }
}

/// Opus encoder error.
#[derive(Debug)]
pub enum EncoderError {
    /// Failed to create encoder.
    CreateFailed(String),
    /// Encoder is closed.
    Closed,
    /// Encoding failed.
    EncodeFailed(String),
    /// Failed to set option.
    SetOptionFailed(String),
}

impl std::fmt::Display for EncoderError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::CreateFailed(msg) => write!(f, "opus: encoder create failed: {}", msg),
            Self::Closed => write!(f, "opus: encoder is closed"),
            Self::EncodeFailed(msg) => write!(f, "opus: encode failed: {}", msg),
            Self::SetOptionFailed(msg) => write!(f, "opus: set option failed: {}", msg),
        }
    }
}

impl std::error::Error for EncoderError {}

/// Opus encoder.
pub struct Encoder {
    sample_rate: i32,
    channels: i32,
    handle: *mut OpusEncoderHandle,
}

// Safety: The encoder handle is not shared across threads.
unsafe impl Send for Encoder {}

impl Drop for Encoder {
    fn drop(&mut self) {
        if !self.handle.is_null() {
            unsafe { ffi::opus_encoder_destroy(self.handle) };
            self.handle = ptr::null_mut();
        }
    }
}

impl Encoder {
    /// Creates a new Opus encoder.
    ///
    /// # Parameters
    /// - `sample_rate`: Sample rate (8000, 12000, 16000, 24000, or 48000)
    /// - `channels`: Number of channels (1 or 2)
    /// - `application`: Intended application type
    pub fn new(sample_rate: i32, channels: i32, application: Application) -> Result<Self, EncoderError> {
        let mut error: i32 = 0;
        let handle = unsafe {
            ffi::opus_encoder_create(
                sample_rate,
                channels,
                application.to_ffi(),
                &mut error,
            )
        };

        if handle.is_null() || error != ffi::OPUS_OK {
            return Err(EncoderError::CreateFailed(ffi::error_string(error)));
        }

        Ok(Self {
            sample_rate,
            channels,
            handle,
        })
    }

    /// Creates a new VoIP encoder.
    pub fn new_voip(sample_rate: i32, channels: i32) -> Result<Self, EncoderError> {
        Self::new(sample_rate, channels, Application::VoIP)
    }

    /// Creates a new audio encoder.
    pub fn new_audio(sample_rate: i32, channels: i32) -> Result<Self, EncoderError> {
        Self::new(sample_rate, channels, Application::Audio)
    }

    /// Returns the sample rate.
    pub fn sample_rate(&self) -> i32 {
        self.sample_rate
    }

    /// Returns the number of channels.
    pub fn channels(&self) -> i32 {
        self.channels
    }

    /// Encodes PCM samples to an Opus frame.
    ///
    /// # Parameters
    /// - `pcm`: Input PCM samples (frame_size * channels samples)
    /// - `frame_size`: Number of samples per channel
    pub fn encode(&mut self, pcm: &[i16], frame_size: i32) -> Result<Frame, EncoderError> {
        if self.handle.is_null() {
            return Err(EncoderError::Closed);
        }

        let mut buf = vec![0u8; 4000]; // Max Opus frame size
        let n = unsafe {
            ffi::opus_encode(
                self.handle,
                pcm.as_ptr(),
                frame_size,
                buf.as_mut_ptr(),
                buf.len() as i32,
            )
        };

        if n < 0 {
            return Err(EncoderError::EncodeFailed(ffi::error_string(n)));
        }

        buf.truncate(n as usize);
        Ok(Frame::new(buf))
    }

    /// Encodes PCM samples from bytes (little-endian i16).
    pub fn encode_bytes(&mut self, pcm: &[u8], frame_size: i32) -> Result<Frame, EncoderError> {
        // Reinterpret bytes as i16 samples
        let samples: &[i16] = unsafe {
            std::slice::from_raw_parts(
                pcm.as_ptr() as *const i16,
                pcm.len() / 2,
            )
        };
        self.encode(samples, frame_size)
    }

    /// Encodes to a provided buffer. Returns number of bytes written.
    pub fn encode_to(&mut self, pcm: &[i16], frame_size: i32, buf: &mut [u8]) -> Result<usize, EncoderError> {
        if self.handle.is_null() {
            return Err(EncoderError::Closed);
        }

        let n = unsafe {
            ffi::opus_encode(
                self.handle,
                pcm.as_ptr(),
                frame_size,
                buf.as_mut_ptr(),
                buf.len() as i32,
            )
        };

        if n < 0 {
            return Err(EncoderError::EncodeFailed(ffi::error_string(n)));
        }

        Ok(n as usize)
    }

    /// Sets the target bitrate in bits per second.
    pub fn set_bitrate(&mut self, bitrate: i32) -> Result<(), EncoderError> {
        if self.handle.is_null() {
            return Err(EncoderError::Closed);
        }

        let ret = unsafe {
            ffi::opus_encoder_ctl(
                self.handle,
                ffi::OPUS_SET_BITRATE_REQUEST,
                bitrate,
            )
        };

        if ret != ffi::OPUS_OK {
            return Err(EncoderError::SetOptionFailed(ffi::error_string(ret)));
        }

        Ok(())
    }

    /// Sets the encoder complexity (0-10).
    pub fn set_complexity(&mut self, complexity: i32) -> Result<(), EncoderError> {
        if self.handle.is_null() {
            return Err(EncoderError::Closed);
        }

        let ret = unsafe {
            ffi::opus_encoder_ctl(
                self.handle,
                ffi::OPUS_SET_COMPLEXITY_REQUEST,
                complexity,
            )
        };

        if ret != ffi::OPUS_OK {
            return Err(EncoderError::SetOptionFailed(ffi::error_string(ret)));
        }

        Ok(())
    }

    /// Returns the frame size for a given duration.
    pub fn frame_size_for_duration(&self, fd: FrameDuration) -> i32 {
        self.sample_rate * fd.millis() as i32 / 1000
    }

    /// Returns the frame size for 20ms frames (recommended default).
    pub fn frame_size_20ms(&self) -> i32 {
        self.sample_rate * 20 / 1000
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_encoder_create() {
        let encoder = Encoder::new_voip(16000, 1);
        assert!(encoder.is_ok());
        let enc = encoder.unwrap();
        assert_eq!(enc.sample_rate(), 16000);
        assert_eq!(enc.channels(), 1);
        assert_eq!(enc.frame_size_20ms(), 320);
    }

    #[test]
    fn test_encode() {
        let mut encoder = Encoder::new_voip(16000, 1).unwrap();
        let pcm = vec![0i16; 320]; // 20ms silence
        let frame = encoder.encode(&pcm, 320);
        assert!(frame.is_ok());
        let f = frame.unwrap();
        assert!(!f.is_empty());
    }

    #[test]
    fn test_encoder_create_audio() {
        let encoder = Encoder::new_audio(48000, 2);
        assert!(encoder.is_ok());
        let enc = encoder.unwrap();
        assert_eq!(enc.sample_rate(), 48000);
        assert_eq!(enc.channels(), 2);
    }

    #[test]
    fn test_encoder_create_with_application() {
        // Test VoIP
        let enc = Encoder::new(16000, 1, Application::VoIP);
        assert!(enc.is_ok());

        // Test Audio
        let enc = Encoder::new(48000, 2, Application::Audio);
        assert!(enc.is_ok());

        // Test RestrictedLowdelay
        let enc = Encoder::new(48000, 1, Application::RestrictedLowdelay);
        assert!(enc.is_ok());
    }

    #[test]
    fn test_encoder_different_sample_rates() {
        // 8000 Hz
        let enc = Encoder::new_voip(8000, 1);
        assert!(enc.is_ok());

        // 12000 Hz
        let enc = Encoder::new_voip(12000, 1);
        assert!(enc.is_ok());

        // 24000 Hz
        let enc = Encoder::new_voip(24000, 1);
        assert!(enc.is_ok());

        // 48000 Hz
        let enc = Encoder::new_voip(48000, 1);
        assert!(enc.is_ok());
    }

    #[test]
    fn test_encoder_stereo() {
        let mut encoder = Encoder::new_voip(48000, 2).unwrap();
        let pcm = vec![0i16; 960 * 2]; // 20ms stereo at 48kHz
        let frame = encoder.encode(&pcm, 960);
        assert!(frame.is_ok());
    }

    #[test]
    fn test_encode_bytes() {
        let mut encoder = Encoder::new_voip(16000, 1).unwrap();
        // 320 samples = 640 bytes
        let pcm_bytes = vec![0u8; 640];
        let frame = encoder.encode_bytes(&pcm_bytes, 320);
        assert!(frame.is_ok());
    }

    #[test]
    fn test_encode_to() {
        let mut encoder = Encoder::new_voip(16000, 1).unwrap();
        let pcm = vec![0i16; 320];
        let mut buf = vec![0u8; 4000];
        let result = encoder.encode_to(&pcm, 320, &mut buf);
        assert!(result.is_ok());
        let n = result.unwrap();
        assert!(n > 0 && n <= buf.len());
    }

    #[test]
    fn test_set_bitrate() {
        let mut encoder = Encoder::new_voip(16000, 1).unwrap();
        let result = encoder.set_bitrate(32000);
        assert!(result.is_ok());
    }

    #[test]
    fn test_set_complexity() {
        let mut encoder = Encoder::new_voip(16000, 1).unwrap();
        let result = encoder.set_complexity(5);
        assert!(result.is_ok());
    }

    #[test]
    fn test_frame_size_for_duration() {
        let encoder = Encoder::new_voip(16000, 1).unwrap();
        
        // 10ms at 16kHz = 160 samples
        assert_eq!(encoder.frame_size_for_duration(FrameDuration::Duration10ms), 160);
        
        // 20ms at 16kHz = 320 samples
        assert_eq!(encoder.frame_size_for_duration(FrameDuration::Duration20ms), 320);
        
        // 40ms at 16kHz = 640 samples
        assert_eq!(encoder.frame_size_for_duration(FrameDuration::Duration40ms), 640);
    }

    #[test]
    fn test_encoder_error_display() {
        let err = EncoderError::CreateFailed("test error".to_string());
        assert!(format!("{}", err).contains("create failed"));

        let err = EncoderError::Closed;
        assert!(format!("{}", err).contains("closed"));

        let err = EncoderError::EncodeFailed("test".to_string());
        assert!(format!("{}", err).contains("encode failed"));

        let err = EncoderError::SetOptionFailed("test".to_string());
        assert!(format!("{}", err).contains("set option failed"));
    }

    #[test]
    fn test_encode_non_silence() {
        let mut encoder = Encoder::new_voip(16000, 1).unwrap();
        // Generate a simple sine wave
        let frame_size = 320;
        let mut pcm = Vec::with_capacity(frame_size);
        for i in 0..frame_size {
            let sample = ((i as f32 * 440.0 * 2.0 * std::f32::consts::PI / 16000.0).sin() * 10000.0) as i16;
            pcm.push(sample);
        }
        
        let frame = encoder.encode(&pcm, frame_size as i32);
        assert!(frame.is_ok());
        let f = frame.unwrap();
        assert!(!f.is_empty());
    }

    #[test]
    fn test_encoder_multiple_frames() {
        let mut encoder = Encoder::new_voip(16000, 1).unwrap();
        let pcm = vec![0i16; 320];
        
        // Encode multiple frames
        for _ in 0..10 {
            let frame = encoder.encode(&pcm, 320);
            assert!(frame.is_ok());
        }
    }
}
