//! Opus frame types and stamped frame wire format.
//!
//! Wire format: `[Version(1B) | Timestamp(7B big-endian ms) | OpusFrameData(N)]`

use std::time::Duration;

use giztoy_audio::codec::opus::Frame;

use super::{InputError, Timestamped};

/// A raw Opus frame.
#[derive(Debug, Clone, PartialEq)]
pub struct OpusFrame(pub Vec<u8>);

impl OpusFrame {
    pub fn new(data: Vec<u8>) -> Self {
        Self(data)
    }

    pub fn data(&self) -> &[u8] {
        &self.0
    }

    pub fn is_empty(&self) -> bool {
        self.0.is_empty()
    }

    pub fn len(&self) -> usize {
        self.0.len()
    }

    /// 根据 Opus TOC 计算帧时长。
    pub fn duration(&self) -> Duration {
        Frame::from_slice(self.data()).duration()
    }
}

/// Timestamp in milliseconds since Unix epoch.
pub type EpochMillis = i64;

/// An Opus frame with its timestamp.
#[derive(Debug, Clone, PartialEq)]
pub struct StampedFrame {
    pub frame: OpusFrame,
    pub stamp: EpochMillis,
}

impl StampedFrame {
    pub fn new(frame: OpusFrame, stamp: EpochMillis) -> Self {
        Self { frame, stamp }
    }
}

impl Timestamped<EpochMillis> for StampedFrame {
    fn timestamp(&self) -> EpochMillis {
        self.stamp
    }
}

/// Current stamped frame format version.
const FRAME_VERSION: u8 = 1;

/// Size of the stamped frame header (1 byte version + 7 bytes timestamp).
const STAMPED_HEADER_SIZE: usize = 8;

/// Parse a stamped frame from wire data.
pub fn parse_stamped(data: &[u8]) -> Result<(OpusFrame, EpochMillis), InputError> {
    if data.len() < STAMPED_HEADER_SIZE {
        return Err(InputError::TruncatedFrame);
    }
    if data[0] != FRAME_VERSION {
        return Err(InputError::UnsupportedVersion(data[0]));
    }

    let mut buf = [0u8; 8];
    buf[1..].copy_from_slice(&data[1..8]);
    let ts = i64::from_be_bytes(buf);

    let frame_data = &data[STAMPED_HEADER_SIZE..];
    if frame_data.is_empty() {
        return Err(InputError::InvalidStampedFrame(
            "empty frame payload".to_string(),
        ));
    }

    Ok((OpusFrame(frame_data.to_vec()), ts))
}

/// Create stamped wire data from a frame and timestamp.
pub fn make_stamped(frame: &OpusFrame, stamp: EpochMillis) -> Vec<u8> {
    let ts_bytes = (stamp as u64).to_be_bytes();
    let mut buf = Vec::with_capacity(STAMPED_HEADER_SIZE + frame.len());
    buf.push(FRAME_VERSION);
    buf.extend_from_slice(&ts_bytes[1..]);
    buf.extend_from_slice(frame.data());
    buf
}

/// Opus silence frame (20ms, mono, fullband, CELT-only).
pub const OPUS_SILENCE_20MS: [u8; 3] = [0xf8, 0xff, 0xfe];

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn t19_1_roundtrip() {
        let frame = OpusFrame(vec![0xf8, 0xff, 0xfe, 0x01, 0x02]);
        let stamp: EpochMillis = 1700000000000;
        let wire = make_stamped(&frame, stamp);
        let (parsed_frame, parsed_stamp) = parse_stamped(&wire).expect("parse stamped roundtrip");
        assert_eq!(parsed_frame, frame);
        assert_eq!(parsed_stamp, stamp);
    }

    #[test]
    fn t19_2_corrupted_data() {
        assert!(matches!(
            parse_stamped(&[0x00; 8]),
            Err(InputError::UnsupportedVersion(_))
        ));
        assert!(matches!(
            parse_stamped(&[FRAME_VERSION, 0, 0, 0, 0, 0, 0, 0]),
            Err(InputError::InvalidStampedFrame(_))
        ));
        // no frame data
    }

    #[test]
    fn t19_3_empty_data() {
        assert!(matches!(
            parse_stamped(&[]),
            Err(InputError::TruncatedFrame)
        ));
        assert!(matches!(
            parse_stamped(&[1, 2, 3]),
            Err(InputError::TruncatedFrame)
        )); // too short
    }

    #[test]
    fn t19_4_timestamp_boundaries() {
        let frame = OpusFrame(vec![0xAA]);

        // Zero timestamp
        let wire = make_stamped(&frame, 0);
        let (_, ts) = parse_stamped(&wire).expect("parse zero ts");
        assert_eq!(ts, 0);

        // Large timestamp (but fits in 7 bytes = 56 bits max)
        let large_ts: EpochMillis = (1i64 << 55) - 1;
        let wire = make_stamped(&frame, large_ts);
        let (_, ts) = parse_stamped(&wire).expect("parse large ts");
        assert_eq!(ts, large_ts);
    }

    #[test]
    fn t19_5_wire_format_version_byte() {
        let frame = OpusFrame(vec![0x01]);
        let wire = make_stamped(&frame, 12345);
        assert_eq!(wire[0], FRAME_VERSION);
        assert_eq!(wire.len(), STAMPED_HEADER_SIZE + 1);
    }

    #[test]
    fn t19_6_silence_frame() {
        assert_eq!(OPUS_SILENCE_20MS.len(), 3);
        let frame = OpusFrame(OPUS_SILENCE_20MS.to_vec());
        let wire = make_stamped(&frame, 1000);
        let (parsed, _) = parse_stamped(&wire).expect("parse silence frame");
        assert_eq!(parsed.data(), &OPUS_SILENCE_20MS);
    }
}
