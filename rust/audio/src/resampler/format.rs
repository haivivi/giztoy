//! Audio format definitions.
//!
//! This module provides a unified `Format` type for describing audio formats,
//! supporting both mono and stereo configurations at various sample rates.

use std::time::Duration;

/// Describes the audio format for PCM audio data.
/// Currently only supports 16-bit signed integer samples (L16).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct Format {
    /// Sample rate in Hz (e.g., 16000, 44100, 48000).
    pub sample_rate: u32,
    /// True for stereo (2 channels), false for mono (1 channel).
    pub stereo: bool,
}

impl Format {
    /// Creates a new format with the given sample rate and mono audio.
    pub const fn mono(sample_rate: u32) -> Self {
        Self { sample_rate, stereo: false }
    }

    /// Creates a new format with the given sample rate and stereo audio.
    pub const fn stereo(sample_rate: u32) -> Self {
        Self { sample_rate, stereo: true }
    }

    /// Returns the sample rate in Hz for this format.
    #[inline]
    pub const fn sample_rate(&self) -> u32 {
        self.sample_rate
    }

    /// Returns the number of channels (1 for mono, 2 for stereo).
    #[inline]
    pub const fn channels(&self) -> u32 {
        if self.stereo { 2 } else { 1 }
    }

    /// Returns the bit depth for this format (always 16 for L16).
    #[inline]
    pub const fn depth(&self) -> u32 {
        16
    }

    /// Returns the number of bytes per sample frame.
    /// For 16-bit audio: 2 bytes for mono, 4 bytes for stereo.
    #[inline]
    pub const fn sample_bytes(&self) -> usize {
        if self.stereo { 4 } else { 2 }
    }

    /// Returns the number of samples in the given number of bytes.
    #[inline]
    pub const fn samples(&self, bytes: u64) -> u64 {
        bytes * 8 / (self.channels() as u64) / (self.depth() as u64)
    }

    /// Returns the number of samples in the given duration.
    #[inline]
    pub fn samples_in_duration(&self, duration: Duration) -> u64 {
        (self.sample_rate as u64) * duration.as_micros() as u64 / 1_000_000
    }

    /// Returns the number of bytes in the given duration.
    #[inline]
    pub fn bytes_in_duration(&self, duration: Duration) -> u64 {
        self.samples_in_duration(duration) * (self.channels() as u64) * (self.depth() as u64) / 8
    }

    /// Returns the duration of the given number of bytes.
    #[inline]
    pub fn duration(&self, bytes: u64) -> Duration {
        let samples = self.samples(bytes);
        Duration::from_micros(samples * 1_000_000 / (self.sample_rate as u64))
    }

    /// Returns the bit rate of the audio data in bits per second.
    #[inline]
    pub const fn bits_rate(&self) -> u32 {
        self.sample_rate * self.channels() * self.depth()
    }

    /// Returns the byte rate of the audio data in bytes per second.
    #[inline]
    pub const fn bytes_rate(&self) -> u32 {
        self.bits_rate() / 8
    }
}

// ============================================================================
// Format Presets - L16 naming convention
// ============================================================================

#[allow(non_upper_case_globals)]
impl Format {
    // Mono formats (L16Mono*)
    
    /// 16-bit mono at 16000 Hz (audio/L16; rate=16000; channels=1)
    pub const L16Mono16K: Format = Format::mono(16000);
    /// 16-bit mono at 24000 Hz (audio/L16; rate=24000; channels=1)
    pub const L16Mono24K: Format = Format::mono(24000);
    /// 16-bit mono at 44100 Hz (audio/L16; rate=44100; channels=1)
    pub const L16Mono44K: Format = Format::mono(44100);
    /// 16-bit mono at 48000 Hz (audio/L16; rate=48000; channels=1)
    pub const L16Mono48K: Format = Format::mono(48000);

    // Stereo formats (L16Stereo*)
    
    /// 16-bit stereo at 44100 Hz (audio/L16; rate=44100; channels=2)
    pub const L16Stereo44K: Format = Format::stereo(44100);
    /// 16-bit stereo at 48000 Hz (audio/L16; rate=48000; channels=2)
    pub const L16Stereo48K: Format = Format::stereo(48000);
}

impl std::fmt::Display for Format {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "audio/L16; rate={}; channels={}", self.sample_rate, self.channels())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_format_mono() {
        let fmt = Format::mono(16000);
        assert_eq!(fmt.sample_rate, 16000);
        assert!(!fmt.stereo);
        assert_eq!(fmt.channels(), 1);
        assert_eq!(fmt.sample_bytes(), 2);
    }

    #[test]
    fn test_format_stereo() {
        let fmt = Format::stereo(48000);
        assert_eq!(fmt.sample_rate, 48000);
        assert!(fmt.stereo);
        assert_eq!(fmt.channels(), 2);
        assert_eq!(fmt.sample_bytes(), 4);
    }

    #[test]
    fn test_format_properties() {
        let format = Format::L16Mono16K;
        assert_eq!(format.sample_rate(), 16000);
        assert_eq!(format.channels(), 1);
        assert_eq!(format.depth(), 16);
        assert_eq!(format.bits_rate(), 256000);
        assert_eq!(format.bytes_rate(), 32000);
    }

    #[test]
    fn test_bytes_in_duration() {
        let format = Format::L16Mono16K;
        // 1 second at 16kHz mono 16-bit = 16000 samples * 2 bytes = 32000 bytes
        assert_eq!(format.bytes_in_duration(Duration::from_secs(1)), 32000);
        // 100ms = 1600 samples * 2 bytes = 3200 bytes
        assert_eq!(format.bytes_in_duration(Duration::from_millis(100)), 3200);
    }

    #[test]
    fn test_duration() {
        let format = Format::L16Mono16K;
        // 32000 bytes = 1 second
        assert_eq!(format.duration(32000), Duration::from_secs(1));
        // 3200 bytes = 100ms
        assert_eq!(format.duration(3200), Duration::from_millis(100));
    }

    #[test]
    fn test_samples() {
        let format = Format::L16Mono16K;
        // 32000 bytes = 16000 samples (2 bytes per sample)
        assert_eq!(format.samples(32000), 16000);
    }

    #[test]
    fn test_format_presets_l16() {
        // Mono presets
        assert_eq!(Format::L16Mono16K.sample_rate, 16000);
        assert!(!Format::L16Mono16K.stereo);
        
        assert_eq!(Format::L16Mono24K.sample_rate, 24000);
        assert!(!Format::L16Mono24K.stereo);
        
        assert_eq!(Format::L16Mono44K.sample_rate, 44100);
        assert!(!Format::L16Mono44K.stereo);
        
        assert_eq!(Format::L16Mono48K.sample_rate, 48000);
        assert!(!Format::L16Mono48K.stereo);

        // Stereo presets
        assert_eq!(Format::L16Stereo44K.sample_rate, 44100);
        assert!(Format::L16Stereo44K.stereo);
        
        assert_eq!(Format::L16Stereo48K.sample_rate, 48000);
        assert!(Format::L16Stereo48K.stereo);
    }

    #[test]
    fn test_format_display() {
        assert_eq!(format!("{}", Format::L16Mono16K), "audio/L16; rate=16000; channels=1");
        assert_eq!(format!("{}", Format::L16Stereo48K), "audio/L16; rate=48000; channels=2");
    }

    #[test]
    fn test_format_clone() {
        let fmt = Format::mono(16000);
        let cloned = fmt.clone();
        assert_eq!(fmt, cloned);
    }

    #[test]
    fn test_format_copy() {
        let fmt = Format::mono(16000);
        let copied = fmt;
        assert_eq!(fmt, copied);
    }

    #[test]
    fn test_format_eq() {
        let fmt1 = Format::mono(16000);
        let fmt2 = Format::mono(16000);
        let fmt3 = Format::mono(24000);
        let fmt4 = Format::stereo(16000);

        assert_eq!(fmt1, fmt2);
        assert_ne!(fmt1, fmt3);
        assert_ne!(fmt1, fmt4);
    }

    #[test]
    fn test_stereo_bytes_in_duration() {
        let format = Format::L16Stereo44K;
        // 1 second at 44.1kHz stereo 16-bit = 44100 * 2 * 2 = 176400 bytes
        assert_eq!(format.bytes_in_duration(Duration::from_secs(1)), 176400);
    }
}
