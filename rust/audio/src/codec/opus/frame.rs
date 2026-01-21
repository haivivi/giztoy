//! Opus frame type.

use std::time::Duration;
use super::toc::{TOC, FrameCode, Configuration, ConfigurationMode, Bandwidth, FrameDuration};

/// Raw Opus encoded frame.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Frame(pub Vec<u8>);

impl Frame {
    /// Creates a new frame from bytes.
    pub fn new(data: Vec<u8>) -> Self {
        Self(data)
    }

    /// Creates a frame from a byte slice.
    pub fn from_slice(data: &[u8]) -> Self {
        Self(data.to_vec())
    }

    /// Returns the raw bytes.
    pub fn as_bytes(&self) -> &[u8] {
        &self.0
    }

    /// Returns the length in bytes.
    pub fn len(&self) -> usize {
        self.0.len()
    }

    /// Returns true if the frame is empty.
    pub fn is_empty(&self) -> bool {
        self.0.is_empty()
    }

    /// Returns the total duration of audio in this frame.
    pub fn duration(&self) -> Duration {
        if self.0.is_empty() {
            return Duration::ZERO;
        }

        let toc = self.toc();
        let fd = toc.configuration().frame_duration();

        match toc.frame_code() {
            FrameCode::OneFrame => fd.duration(),
            FrameCode::TwoEqualFrames => fd.duration() * 2,
            FrameCode::TwoDifferentFrames => fd.duration() * 2,
            FrameCode::ArbitraryFrames => {
                if self.0.len() < 2 {
                    Duration::ZERO
                } else {
                    let frame_count = self.0[1] & 0b00111111;
                    fd.duration() * frame_count as u32
                }
            }
        }
    }

    /// Returns the TOC byte of this frame.
    pub fn toc(&self) -> TOC {
        if self.0.is_empty() {
            return TOC::new(0);
        }
        TOC::new(self.0[0])
    }

    /// Returns true if this frame contains stereo audio.
    pub fn is_stereo(&self) -> bool {
        self.toc().is_stereo()
    }

    /// Returns the configuration of this frame.
    pub fn configuration(&self) -> Configuration {
        self.toc().configuration()
    }

    /// Returns the mode (SILK, CELT, or Hybrid) of this frame.
    pub fn mode(&self) -> ConfigurationMode {
        self.configuration().mode()
    }

    /// Returns the bandwidth of this frame.
    pub fn bandwidth(&self) -> Bandwidth {
        self.configuration().bandwidth()
    }

    /// Returns the frame duration type.
    pub fn frame_duration(&self) -> FrameDuration {
        self.configuration().frame_duration()
    }

    /// Returns the number of samples in this frame.
    pub fn samples(&self) -> i32 {
        self.configuration().samples()
    }
}

impl AsRef<[u8]> for Frame {
    fn as_ref(&self) -> &[u8] {
        &self.0
    }
}

impl From<Vec<u8>> for Frame {
    fn from(data: Vec<u8>) -> Self {
        Self(data)
    }
}

impl From<&[u8]> for Frame {
    fn from(data: &[u8]) -> Self {
        Self(data.to_vec())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_empty_frame() {
        let frame = Frame::new(vec![]);
        assert!(frame.is_empty());
        assert_eq!(frame.duration(), Duration::ZERO);
    }

    #[test]
    fn test_frame_new() {
        let data = vec![0x01, 0x02, 0x03];
        let frame = Frame::new(data.clone());
        assert_eq!(frame.len(), 3);
        assert!(!frame.is_empty());
        assert_eq!(frame.as_bytes(), &data[..]);
    }

    #[test]
    fn test_frame_from_slice() {
        let data = [0x48, 0x01, 0x02];
        let frame = Frame::from_slice(&data);
        assert_eq!(frame.len(), 3);
    }

    #[test]
    fn test_frame_toc() {
        // TOC byte 0x48 = config 9, mono, one frame
        let frame = Frame::new(vec![0x48, 0x00, 0x00]);
        let toc = frame.toc();
        assert_eq!(toc.configuration().0, 9);
        assert!(!toc.is_stereo());
        assert_eq!(toc.frame_code(), FrameCode::OneFrame);
    }

    #[test]
    fn test_frame_is_stereo() {
        // Mono frame
        let mono_frame = Frame::new(vec![0x48]);
        assert!(!mono_frame.is_stereo());

        // Stereo frame (bit 2 set)
        let stereo_frame = Frame::new(vec![0x4C]);
        assert!(stereo_frame.is_stereo());
    }

    #[test]
    fn test_frame_configuration() {
        let frame = Frame::new(vec![0x48]); // Config 9
        let config = frame.configuration();
        assert_eq!(config.0, 9);
    }

    #[test]
    fn test_frame_mode() {
        // SILK mode (config 0-11)
        let silk_frame = Frame::new(vec![0x48]); // Config 9
        assert_eq!(silk_frame.mode(), ConfigurationMode::Silk);

        // CELT mode (config 16-31)
        let celt_frame = Frame::new(vec![0x80]); // Config 16
        assert_eq!(celt_frame.mode(), ConfigurationMode::CELT);
    }

    #[test]
    fn test_frame_bandwidth() {
        let frame = Frame::new(vec![0x48]); // Config 9 = WB
        assert_eq!(frame.bandwidth(), Bandwidth::WB);
    }

    #[test]
    fn test_frame_frame_duration() {
        let frame = Frame::new(vec![0x48]); // Config 9 = 20ms
        assert_eq!(frame.frame_duration(), FrameDuration::Duration20ms);
    }

    #[test]
    fn test_frame_samples() {
        let frame = Frame::new(vec![0x48]); // Config 9 = WB 20ms = 320 samples
        assert_eq!(frame.samples(), 320);
    }

    #[test]
    fn test_frame_duration_one_frame() {
        // TOC with one frame (code 0)
        let frame = Frame::new(vec![0x48]); // Config 9 (20ms), mono, one frame
        assert_eq!(frame.duration(), Duration::from_millis(20));
    }

    #[test]
    fn test_frame_duration_two_equal_frames() {
        // TOC with two equal frames (code 1)
        let frame = Frame::new(vec![0x49]); // Config 9 (20ms), mono, two equal frames
        assert_eq!(frame.duration(), Duration::from_millis(40));
    }

    #[test]
    fn test_frame_duration_two_different_frames() {
        // TOC with two different frames (code 2)
        let frame = Frame::new(vec![0x4A]); // Config 9 (20ms), mono, two different frames
        assert_eq!(frame.duration(), Duration::from_millis(40));
    }

    #[test]
    fn test_frame_duration_arbitrary_frames() {
        // TOC with arbitrary frames (code 3)
        // Second byte indicates frame count (lower 6 bits)
        let frame = Frame::new(vec![0x4B, 0x03]); // Config 9 (20ms), 3 frames
        assert_eq!(frame.duration(), Duration::from_millis(60));
    }

    #[test]
    fn test_frame_duration_arbitrary_frames_too_short() {
        // Frame with arbitrary code but missing second byte
        let frame = Frame::new(vec![0x4B]);
        assert_eq!(frame.duration(), Duration::ZERO);
    }

    #[test]
    fn test_frame_as_ref() {
        let data = vec![0x48, 0x01, 0x02];
        let frame = Frame::new(data.clone());
        let slice: &[u8] = frame.as_ref();
        assert_eq!(slice, &data[..]);
    }

    #[test]
    fn test_frame_from_vec() {
        let data = vec![0x48, 0x01, 0x02];
        let frame: Frame = data.clone().into();
        assert_eq!(frame.as_bytes(), &data[..]);
    }

    #[test]
    fn test_frame_from_slice_trait() {
        let data: &[u8] = &[0x48, 0x01, 0x02];
        let frame: Frame = data.into();
        assert_eq!(frame.as_bytes(), data);
    }

    #[test]
    fn test_frame_clone() {
        let frame1 = Frame::new(vec![0x48, 0x01]);
        let frame2 = frame1.clone();
        assert_eq!(frame1, frame2);
    }

    #[test]
    fn test_frame_debug() {
        let frame = Frame::new(vec![0x48]);
        let debug_str = format!("{:?}", frame);
        assert!(debug_str.contains("Frame"));
    }
}
