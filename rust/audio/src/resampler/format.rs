//! Audio format for resampling.

/// Describes the audio format for resampling.
/// Currently only supports 16-bit signed integer samples.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Format {
    /// Sample rate in Hz (e.g., 44100, 48000).
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

    /// Returns the number of channels (1 for mono, 2 for stereo).
    pub fn channels(&self) -> u32 {
        if self.stereo { 2 } else { 1 }
    }

    /// Returns the number of bytes per sample frame.
    /// For 16-bit audio: 2 bytes for mono, 4 bytes for stereo.
    pub fn sample_bytes(&self) -> usize {
        if self.stereo { 4 } else { 2 }
    }
}

// Common format presets
impl Format {
    /// 16kHz mono (common for TTS)
    pub const MONO_16K: Format = Format::mono(16000);
    /// 24kHz mono
    pub const MONO_24K: Format = Format::mono(24000);
    /// 44.1kHz mono (CD quality mono)
    pub const MONO_44K: Format = Format::mono(44100);
    /// 48kHz mono
    pub const MONO_48K: Format = Format::mono(48000);
    /// 44.1kHz stereo (CD quality)
    pub const STEREO_44K: Format = Format::stereo(44100);
    /// 48kHz stereo
    pub const STEREO_48K: Format = Format::stereo(48000);
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_format_mono() {
        let fmt = Format::mono(16000);
        assert_eq!(fmt.sample_rate, 16000);
        assert!(!fmt.stereo);
    }

    #[test]
    fn test_format_stereo() {
        let fmt = Format::stereo(48000);
        assert_eq!(fmt.sample_rate, 48000);
        assert!(fmt.stereo);
    }

    #[test]
    fn test_format_channels() {
        let mono = Format::mono(16000);
        assert_eq!(mono.channels(), 1);

        let stereo = Format::stereo(48000);
        assert_eq!(stereo.channels(), 2);
    }

    #[test]
    fn test_format_sample_bytes() {
        let mono = Format::mono(16000);
        assert_eq!(mono.sample_bytes(), 2); // 16-bit mono = 2 bytes

        let stereo = Format::stereo(48000);
        assert_eq!(stereo.sample_bytes(), 4); // 16-bit stereo = 4 bytes
    }

    #[test]
    fn test_format_presets_mono() {
        assert_eq!(Format::MONO_16K.sample_rate, 16000);
        assert!(!Format::MONO_16K.stereo);

        assert_eq!(Format::MONO_24K.sample_rate, 24000);
        assert!(!Format::MONO_24K.stereo);

        assert_eq!(Format::MONO_44K.sample_rate, 44100);
        assert!(!Format::MONO_44K.stereo);

        assert_eq!(Format::MONO_48K.sample_rate, 48000);
        assert!(!Format::MONO_48K.stereo);
    }

    #[test]
    fn test_format_presets_stereo() {
        assert_eq!(Format::STEREO_44K.sample_rate, 44100);
        assert!(Format::STEREO_44K.stereo);

        assert_eq!(Format::STEREO_48K.sample_rate, 48000);
        assert!(Format::STEREO_48K.stereo);
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
    fn test_format_debug() {
        let fmt = Format::mono(16000);
        let debug_str = format!("{:?}", fmt);
        assert!(debug_str.contains("Format"));
        assert!(debug_str.contains("16000"));
    }
}
