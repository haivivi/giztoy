//! Opus TOC (Table of Contents) parsing.
//!
//! Implements RFC 6716 Section 3.1.

use std::time::Duration;

/// TOC byte from an Opus packet header.
///
/// Layout:
/// ```text
///          0 1 2 3 4 5 6 7
///         +-+-+-+-+-+-+-+-+
///         | config  |s| c |
///         +-+-+-+-+-+-+-+-+
/// ```
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct TOC(pub u8);

impl TOC {
    /// Creates a TOC from a byte.
    pub const fn new(byte: u8) -> Self {
        Self(byte)
    }

    /// Returns the configuration number (0-31).
    pub fn configuration(&self) -> Configuration {
        Configuration((self.0 >> 3) as u8)
    }

    /// Returns true if the TOC indicates stereo audio.
    pub fn is_stereo(&self) -> bool {
        (self.0 & 0b00000100) != 0
    }

    /// Returns the frame code (number of frames per packet).
    pub fn frame_code(&self) -> FrameCode {
        FrameCode::from_bits(self.0 & 0b00000011)
    }
}

impl std::fmt::Display for TOC {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(
            f,
            "opus_toc: stereo={}, mode={}, bw={}, {}, {}",
            self.is_stereo(),
            self.configuration().mode(),
            self.configuration().bandwidth(),
            self.frame_code(),
            self.configuration().frame_duration(),
        )
    }
}

/// Opus configuration number (0-31).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Configuration(pub u8);

impl Configuration {
    /// Returns the configuration mode (SILK, CELT, or Hybrid).
    pub fn mode(&self) -> ConfigurationMode {
        match self.0 {
            0..=11 => ConfigurationMode::Silk,
            12..=15 => ConfigurationMode::Hybrid,
            16..=31 => ConfigurationMode::CELT,
            _ => ConfigurationMode::Silk, // Invalid, default to Silk
        }
    }

    /// Returns the audio bandwidth.
    pub fn bandwidth(&self) -> Bandwidth {
        match self.0 {
            0..=3 => Bandwidth::NB,
            4..=7 => Bandwidth::MB,
            8..=11 => Bandwidth::WB,
            12..=13 => Bandwidth::SWB,
            14..=15 => Bandwidth::FB,
            16..=19 => Bandwidth::NB,
            20..=23 => Bandwidth::WB,
            24..=27 => Bandwidth::SWB,
            28..=31 => Bandwidth::FB,
            _ => Bandwidth::NB,
        }
    }

    /// Returns the frame duration.
    pub fn frame_duration(&self) -> FrameDuration {
        match self.0 {
            16 | 20 | 24 | 28 => FrameDuration::Duration2500us,
            17 | 21 | 25 | 29 => FrameDuration::Duration5ms,
            0 | 4 | 8 | 12 | 14 | 18 | 22 | 26 | 30 => FrameDuration::Duration10ms,
            1 | 5 | 9 | 13 | 15 | 19 | 23 | 27 | 31 => FrameDuration::Duration20ms,
            2 | 6 | 10 => FrameDuration::Duration40ms,
            3 | 7 | 11 => FrameDuration::Duration60ms,
            _ => FrameDuration::Duration20ms,
        }
    }

    /// Returns the number of samples for this configuration.
    pub fn samples(&self) -> i32 {
        const SAMPLES: [i32; 32] = [
            // Silk   NB   0...3
            80, 160, 320, 480,
            // Silk   MB   4...7
            120, 240, 480, 720,
            // Silk   WB   8..11
            160, 320, 640, 960,
            // Hybrid SWB 12..13
            240, 480,
            // Hybrid FB  14..15
            480, 960,
            // CELT   NB  16..19
            20, 40, 80, 120,
            // CELT   WB  20..23
            40, 80, 160, 240,
            // CELT   SWB 24..27
            60, 120, 240, 480,
            // CELT   SWB 28..31
            120, 240, 480, 960,
        ];
        if self.0 > 31 {
            return 0;
        }
        SAMPLES[self.0 as usize]
    }

    /// Returns the granule position increment for OGG pages.
    pub fn page_granule_increment(&self) -> i32 {
        match self.0 {
            16 | 20 | 24 | 28 => 120,
            17 | 21 | 25 | 29 => 240,
            0 | 4 | 8 | 12 | 14 | 18 | 22 | 26 | 30 => 480,
            1 | 5 | 9 | 13 | 15 | 19 | 23 | 27 | 31 => 960,
            2 | 6 | 10 => 1920,
            3 | 7 | 11 => 2880,
            _ => 0,
        }
    }
}

/// Configuration mode (SILK, CELT, or Hybrid).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ConfigurationMode {
    Silk,
    CELT,
    Hybrid,
}

impl std::fmt::Display for ConfigurationMode {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Silk => write!(f, "Silk"),
            Self::CELT => write!(f, "CELT"),
            Self::Hybrid => write!(f, "Hybrid"),
        }
    }
}

/// Frame code indicating number of frames per packet.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FrameCode {
    /// One frame in the packet.
    OneFrame,
    /// Two frames with equal compressed size.
    TwoEqualFrames,
    /// Two frames with different compressed sizes.
    TwoDifferentFrames,
    /// Arbitrary number of frames.
    ArbitraryFrames,
}

impl FrameCode {
    fn from_bits(bits: u8) -> Self {
        match bits & 0b11 {
            0 => Self::OneFrame,
            1 => Self::TwoEqualFrames,
            2 => Self::TwoDifferentFrames,
            _ => Self::ArbitraryFrames,
        }
    }
}

impl std::fmt::Display for FrameCode {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::OneFrame => write!(f, "One Frame"),
            Self::TwoEqualFrames => write!(f, "Two Equal Frames"),
            Self::TwoDifferentFrames => write!(f, "Two Different Frames"),
            Self::ArbitraryFrames => write!(f, "Arbitrary Frames"),
        }
    }
}

/// Frame duration.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FrameDuration {
    Duration2500us,
    Duration5ms,
    Duration10ms,
    Duration20ms,
    Duration40ms,
    Duration60ms,
}

impl FrameDuration {
    /// Returns the duration in milliseconds.
    pub fn millis(&self) -> i64 {
        match self {
            Self::Duration2500us => 2,
            Self::Duration5ms => 5,
            Self::Duration10ms => 10,
            Self::Duration20ms => 20,
            Self::Duration40ms => 40,
            Self::Duration60ms => 60,
        }
    }

    /// Returns the duration as a Duration.
    pub fn duration(&self) -> Duration {
        match self {
            Self::Duration2500us => Duration::from_micros(2500),
            Self::Duration5ms => Duration::from_millis(5),
            Self::Duration10ms => Duration::from_millis(10),
            Self::Duration20ms => Duration::from_millis(20),
            Self::Duration40ms => Duration::from_millis(40),
            Self::Duration60ms => Duration::from_millis(60),
        }
    }
}

impl std::fmt::Display for FrameDuration {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Duration2500us => write!(f, "2.5ms"),
            Self::Duration5ms => write!(f, "5ms"),
            Self::Duration10ms => write!(f, "10ms"),
            Self::Duration20ms => write!(f, "20ms"),
            Self::Duration40ms => write!(f, "40ms"),
            Self::Duration60ms => write!(f, "60ms"),
        }
    }
}

/// Audio bandwidth.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Bandwidth {
    /// Narrowband (4 kHz audio, 8 kHz sample rate)
    NB,
    /// Medium-band (6 kHz audio, 12 kHz sample rate)
    MB,
    /// Wideband (8 kHz audio, 16 kHz sample rate)
    WB,
    /// Super-wideband (12 kHz audio, 24 kHz sample rate)
    SWB,
    /// Fullband (20 kHz audio, 48 kHz sample rate)
    FB,
}

impl Bandwidth {
    /// Returns the effective sample rate for this bandwidth.
    pub fn sample_rate(&self) -> u32 {
        match self {
            Self::NB => 8000,
            Self::MB => 12000,
            Self::WB => 16000,
            Self::SWB => 24000,
            Self::FB => 48000,
        }
    }
}

impl std::fmt::Display for Bandwidth {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::NB => write!(f, "Narrowband"),
            Self::MB => write!(f, "Mediumband"),
            Self::WB => write!(f, "Wideband"),
            Self::SWB => write!(f, "Superwideband"),
            Self::FB => write!(f, "Fullband"),
        }
    }
}

/// Parses the frame count byte following the TOC byte for packets with
/// arbitrary frame counts (code 3).
///
/// Returns (is_vbr, has_padding, frame_count).
pub fn parse_frame_count_byte(byte: u8) -> (bool, bool, u8) {
    let is_vbr = (byte & 0b10000000) != 0;
    let has_padding = (byte & 0b01000000) != 0;
    let frame_count = byte & 0b00111111;
    (is_vbr, has_padding, frame_count)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_toc_parsing() {
        // Config 9 (WB SILK 20ms), mono, one frame
        let toc = TOC::new(0b01001000);
        assert_eq!(toc.configuration().0, 9);
        assert!(!toc.is_stereo());
        assert_eq!(toc.frame_code(), FrameCode::OneFrame);
        assert_eq!(toc.configuration().mode(), ConfigurationMode::Silk);
        assert_eq!(toc.configuration().bandwidth(), Bandwidth::WB);
        assert_eq!(toc.configuration().frame_duration(), FrameDuration::Duration20ms);
    }

    #[test]
    fn test_frame_count_byte() {
        let (is_vbr, has_padding, count) = parse_frame_count_byte(0b11000011);
        assert!(is_vbr);
        assert!(has_padding);
        assert_eq!(count, 3);
    }

    #[test]
    fn test_toc_stereo_flag() {
        // Stereo flag is bit 2
        let mono_toc = TOC::new(0b01001000);
        assert!(!mono_toc.is_stereo());

        let stereo_toc = TOC::new(0b01001100);
        assert!(stereo_toc.is_stereo());
    }

    #[test]
    fn test_toc_frame_codes() {
        // Frame code is bottom 2 bits
        let toc0 = TOC::new(0b00000000);
        assert_eq!(toc0.frame_code(), FrameCode::OneFrame);

        let toc1 = TOC::new(0b00000001);
        assert_eq!(toc1.frame_code(), FrameCode::TwoEqualFrames);

        let toc2 = TOC::new(0b00000010);
        assert_eq!(toc2.frame_code(), FrameCode::TwoDifferentFrames);

        let toc3 = TOC::new(0b00000011);
        assert_eq!(toc3.frame_code(), FrameCode::ArbitraryFrames);
    }

    #[test]
    fn test_configuration_silk_modes() {
        // Config 0-11 are SILK
        for i in 0..=11 {
            let config = Configuration(i);
            assert_eq!(config.mode(), ConfigurationMode::Silk);
        }
    }

    #[test]
    fn test_configuration_hybrid_modes() {
        // Config 12-15 are Hybrid
        for i in 12..=15 {
            let config = Configuration(i);
            assert_eq!(config.mode(), ConfigurationMode::Hybrid);
        }
    }

    #[test]
    fn test_configuration_celt_modes() {
        // Config 16-31 are CELT
        for i in 16..=31 {
            let config = Configuration(i);
            assert_eq!(config.mode(), ConfigurationMode::CELT);
        }
    }

    #[test]
    fn test_configuration_bandwidth() {
        // NB: 0-3, 16-19
        assert_eq!(Configuration(0).bandwidth(), Bandwidth::NB);
        assert_eq!(Configuration(16).bandwidth(), Bandwidth::NB);

        // MB: 4-7
        assert_eq!(Configuration(4).bandwidth(), Bandwidth::MB);

        // WB: 8-11, 20-23
        assert_eq!(Configuration(8).bandwidth(), Bandwidth::WB);
        assert_eq!(Configuration(20).bandwidth(), Bandwidth::WB);

        // SWB: 12-13, 24-27
        assert_eq!(Configuration(12).bandwidth(), Bandwidth::SWB);
        assert_eq!(Configuration(24).bandwidth(), Bandwidth::SWB);

        // FB: 14-15, 28-31
        assert_eq!(Configuration(14).bandwidth(), Bandwidth::FB);
        assert_eq!(Configuration(28).bandwidth(), Bandwidth::FB);
    }

    #[test]
    fn test_configuration_frame_duration() {
        // 2.5ms: 16, 20, 24, 28
        assert_eq!(Configuration(16).frame_duration(), FrameDuration::Duration2500us);
        assert_eq!(Configuration(20).frame_duration(), FrameDuration::Duration2500us);

        // 5ms: 17, 21, 25, 29
        assert_eq!(Configuration(17).frame_duration(), FrameDuration::Duration5ms);

        // 10ms: 0, 4, 8, etc.
        assert_eq!(Configuration(0).frame_duration(), FrameDuration::Duration10ms);
        assert_eq!(Configuration(4).frame_duration(), FrameDuration::Duration10ms);

        // 20ms: 1, 5, 9, etc.
        assert_eq!(Configuration(1).frame_duration(), FrameDuration::Duration20ms);
        assert_eq!(Configuration(5).frame_duration(), FrameDuration::Duration20ms);

        // 40ms: 2, 6, 10
        assert_eq!(Configuration(2).frame_duration(), FrameDuration::Duration40ms);

        // 60ms: 3, 7, 11
        assert_eq!(Configuration(3).frame_duration(), FrameDuration::Duration60ms);
    }

    #[test]
    fn test_configuration_samples() {
        // Test some known sample counts
        assert_eq!(Configuration(0).samples(), 80);  // NB 10ms
        assert_eq!(Configuration(1).samples(), 160); // NB 20ms
        assert_eq!(Configuration(8).samples(), 160); // WB 10ms
        assert_eq!(Configuration(9).samples(), 320); // WB 20ms
        
        // Invalid config should return 0
        assert_eq!(Configuration(255).samples(), 0);
    }

    #[test]
    fn test_configuration_page_granule_increment() {
        // Test some known values
        assert_eq!(Configuration(16).page_granule_increment(), 120); // 2.5ms
        assert_eq!(Configuration(17).page_granule_increment(), 240); // 5ms
        assert_eq!(Configuration(0).page_granule_increment(), 480);  // 10ms
        assert_eq!(Configuration(1).page_granule_increment(), 960);  // 20ms
        assert_eq!(Configuration(2).page_granule_increment(), 1920); // 40ms
        assert_eq!(Configuration(3).page_granule_increment(), 2880); // 60ms
    }

    #[test]
    fn test_frame_duration_millis() {
        assert_eq!(FrameDuration::Duration2500us.millis(), 2);
        assert_eq!(FrameDuration::Duration5ms.millis(), 5);
        assert_eq!(FrameDuration::Duration10ms.millis(), 10);
        assert_eq!(FrameDuration::Duration20ms.millis(), 20);
        assert_eq!(FrameDuration::Duration40ms.millis(), 40);
        assert_eq!(FrameDuration::Duration60ms.millis(), 60);
    }

    #[test]
    fn test_frame_duration_duration() {
        assert_eq!(FrameDuration::Duration2500us.duration(), Duration::from_micros(2500));
        assert_eq!(FrameDuration::Duration5ms.duration(), Duration::from_millis(5));
        assert_eq!(FrameDuration::Duration10ms.duration(), Duration::from_millis(10));
        assert_eq!(FrameDuration::Duration20ms.duration(), Duration::from_millis(20));
        assert_eq!(FrameDuration::Duration40ms.duration(), Duration::from_millis(40));
        assert_eq!(FrameDuration::Duration60ms.duration(), Duration::from_millis(60));
    }

    #[test]
    fn test_bandwidth_sample_rate() {
        assert_eq!(Bandwidth::NB.sample_rate(), 8000);
        assert_eq!(Bandwidth::MB.sample_rate(), 12000);
        assert_eq!(Bandwidth::WB.sample_rate(), 16000);
        assert_eq!(Bandwidth::SWB.sample_rate(), 24000);
        assert_eq!(Bandwidth::FB.sample_rate(), 48000);
    }

    #[test]
    fn test_toc_display() {
        let toc = TOC::new(0b01001000);
        let s = format!("{}", toc);
        assert!(s.contains("stereo=false"));
        assert!(s.contains("Silk"));
        assert!(s.contains("Wideband"));
    }

    #[test]
    fn test_configuration_mode_display() {
        assert_eq!(format!("{}", ConfigurationMode::Silk), "Silk");
        assert_eq!(format!("{}", ConfigurationMode::CELT), "CELT");
        assert_eq!(format!("{}", ConfigurationMode::Hybrid), "Hybrid");
    }

    #[test]
    fn test_frame_code_display() {
        assert_eq!(format!("{}", FrameCode::OneFrame), "One Frame");
        assert_eq!(format!("{}", FrameCode::TwoEqualFrames), "Two Equal Frames");
        assert_eq!(format!("{}", FrameCode::TwoDifferentFrames), "Two Different Frames");
        assert_eq!(format!("{}", FrameCode::ArbitraryFrames), "Arbitrary Frames");
    }

    #[test]
    fn test_frame_duration_display() {
        assert_eq!(format!("{}", FrameDuration::Duration2500us), "2.5ms");
        assert_eq!(format!("{}", FrameDuration::Duration5ms), "5ms");
        assert_eq!(format!("{}", FrameDuration::Duration10ms), "10ms");
        assert_eq!(format!("{}", FrameDuration::Duration20ms), "20ms");
        assert_eq!(format!("{}", FrameDuration::Duration40ms), "40ms");
        assert_eq!(format!("{}", FrameDuration::Duration60ms), "60ms");
    }

    #[test]
    fn test_bandwidth_display() {
        assert_eq!(format!("{}", Bandwidth::NB), "Narrowband");
        assert_eq!(format!("{}", Bandwidth::MB), "Mediumband");
        assert_eq!(format!("{}", Bandwidth::WB), "Wideband");
        assert_eq!(format!("{}", Bandwidth::SWB), "Superwideband");
        assert_eq!(format!("{}", Bandwidth::FB), "Fullband");
    }

    #[test]
    fn test_frame_count_byte_no_vbr_no_padding() {
        let (is_vbr, has_padding, count) = parse_frame_count_byte(0b00000101);
        assert!(!is_vbr);
        assert!(!has_padding);
        assert_eq!(count, 5);
    }
}
