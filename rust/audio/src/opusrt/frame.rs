//! Opus frame types for real-time streaming.

use std::time::Duration;
use super::timestamp::EpochMillis;
use crate::codec::opus::{TOC, FrameCode};

/// Raw Opus frame data.
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

    /// Returns the TOC byte.
    pub fn toc(&self) -> TOC {
        if self.0.is_empty() {
            return TOC::new(0);
        }
        TOC::new(self.0[0])
    }

    /// Returns true if this frame is stereo.
    pub fn is_stereo(&self) -> bool {
        self.toc().is_stereo()
    }

    /// Returns the duration of this frame based on its TOC byte.
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

/// Stamped frame format version.
pub const FRAME_VERSION: u8 = 1;

/// Size of the stamped frame header.
pub const STAMPED_HEADER_SIZE: usize = 8;

/// StampedFrame format:
/// ```text
/// +--------+------------------+------------------+
/// | Version| Timestamp (7B)   | Opus Frame Data  |
/// | (1B)   | Big-endian ms    |                  |
/// +--------+------------------+------------------+
/// ```
#[derive(Debug, Clone)]
pub struct StampedFrame(pub Vec<u8>);

impl StampedFrame {
    /// Creates a stamped frame from a frame and timestamp.
    pub fn new(frame: &Frame, stamp: EpochMillis) -> Self {
        let mut buf = [0u8; 8];
        let stamp_bytes = (stamp.as_millis() as u64).to_be_bytes();
        buf[0] = FRAME_VERSION;
        buf[1..8].copy_from_slice(&stamp_bytes[1..8]);
        
        let mut data = Vec::with_capacity(STAMPED_HEADER_SIZE + frame.len());
        data.extend_from_slice(&buf);
        data.extend_from_slice(frame.as_bytes());
        Self(data)
    }

    /// Returns the frame data (without header).
    pub fn frame(&self) -> Option<Frame> {
        if self.0.len() < STAMPED_HEADER_SIZE {
            return None;
        }
        Some(Frame::new(self.0[STAMPED_HEADER_SIZE..].to_vec()))
    }

    /// Returns the version byte.
    pub fn version(&self) -> u8 {
        if self.0.is_empty() {
            return 0;
        }
        self.0[0]
    }

    /// Returns the embedded timestamp.
    pub fn stamp(&self) -> EpochMillis {
        if self.0.len() < STAMPED_HEADER_SIZE {
            return EpochMillis::from_millis(0);
        }
        let mut buf = [0u8; 8];
        buf[1..8].copy_from_slice(&self.0[1..8]);
        EpochMillis::from_millis(u64::from_be_bytes(buf) as i64)
    }

    /// Returns the duration of the embedded frame.
    pub fn duration(&self) -> Duration {
        self.frame().map(|f| f.duration()).unwrap_or(Duration::ZERO)
    }

    /// Returns the raw bytes.
    pub fn as_bytes(&self) -> &[u8] {
        &self.0
    }
}

/// Extracts frame and timestamp from stamped data.
pub fn from_stamped(data: &[u8]) -> Option<(Frame, EpochMillis)> {
    if data.len() < STAMPED_HEADER_SIZE {
        return None;
    }
    if data[0] != FRAME_VERSION {
        return None;
    }
    
    let mut buf = [0u8; 8];
    buf[1..8].copy_from_slice(&data[1..8]);
    let stamp = EpochMillis::from_millis(u64::from_be_bytes(buf) as i64);
    
    let frame_data = &data[STAMPED_HEADER_SIZE..];
    if frame_data.is_empty() {
        return None;
    }
    
    Some((Frame::new(frame_data.to_vec()), stamp))
}

/// Creates a stamped frame from a frame and timestamp.
pub fn stamp(frame: &Frame, stamp: EpochMillis) -> Vec<u8> {
    let mut buf = [0u8; 8];
    let stamp_bytes = (stamp.as_millis() as u64).to_be_bytes();
    buf[0] = FRAME_VERSION;
    buf[1..8].copy_from_slice(&stamp_bytes[1..8]);
    
    let mut data = Vec::with_capacity(STAMPED_HEADER_SIZE + frame.len());
    data.extend_from_slice(&buf);
    data.extend_from_slice(frame.as_bytes());
    data
}

/// FrameReader trait for reading Opus frames.
pub trait FrameReader {
    /// Returns the next frame and loss duration.
    /// If loss > 0, frame is None and loss indicates lost data duration.
    fn next_frame(&mut self) -> Result<(Option<Frame>, Duration), std::io::Error>;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_stamped_frame() {
        let frame = Frame::new(vec![0x48, 0x00, 0x01, 0x02]); // Example opus data
        let stamp = EpochMillis::from_millis(12345678);
        
        let stamped = StampedFrame::new(&frame, stamp);
        assert_eq!(stamped.version(), FRAME_VERSION);
        assert_eq!(stamped.stamp().as_millis(), 12345678);
        assert_eq!(stamped.frame().unwrap().as_bytes(), frame.as_bytes());
    }

    #[test]
    fn test_from_stamped() {
        let frame = Frame::new(vec![0x48, 0x00]);
        let ts = EpochMillis::from_millis(1000);
        let data = stamp(&frame, ts);
        
        let (parsed_frame, parsed_stamp) = from_stamped(&data).unwrap();
        assert_eq!(parsed_frame.as_bytes(), frame.as_bytes());
        assert_eq!(parsed_stamp.as_millis(), 1000);
    }
}
