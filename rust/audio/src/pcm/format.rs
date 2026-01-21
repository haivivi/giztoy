//! PCM audio format definitions.

use std::time::Duration;

/// PCM audio format configuration.
///
/// Represents common audio format configurations including sample rate,
/// number of channels, and bit depth.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Format {
    /// 16-bit mono at 16000 Hz (audio/L16; rate=16000; channels=1)
    L16Mono16K,
    /// 16-bit mono at 24000 Hz (audio/L16; rate=24000; channels=1)
    L16Mono24K,
    /// 16-bit mono at 48000 Hz (audio/L16; rate=48000; channels=1)
    L16Mono48K,
}

impl Format {
    /// Returns the sample rate in Hz for this format.
    #[inline]
    pub const fn sample_rate(&self) -> u32 {
        match self {
            Format::L16Mono16K => 16000,
            Format::L16Mono24K => 24000,
            Format::L16Mono48K => 48000,
        }
    }

    /// Returns the number of audio channels for this format.
    #[inline]
    pub const fn channels(&self) -> u32 {
        match self {
            Format::L16Mono16K | Format::L16Mono24K | Format::L16Mono48K => 1,
        }
    }

    /// Returns the bit depth for this format.
    #[inline]
    pub const fn depth(&self) -> u32 {
        match self {
            Format::L16Mono16K | Format::L16Mono24K | Format::L16Mono48K => 16,
        }
    }

    /// Returns the number of samples in the given number of bytes.
    #[inline]
    pub const fn samples(&self, bytes: u64) -> u64 {
        bytes * 8 / (self.channels() as u64) / (self.depth() as u64)
    }

    /// Returns the number of samples in the given duration.
    #[inline]
    pub fn samples_in_duration(&self, duration: Duration) -> u64 {
        (self.sample_rate() as u64) * duration.as_micros() as u64 / 1_000_000
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
        Duration::from_micros(samples * 1_000_000 / (self.sample_rate() as u64))
    }

    /// Returns the bit rate of the audio data in bits per second.
    #[inline]
    pub const fn bits_rate(&self) -> u32 {
        self.sample_rate() * self.channels() * self.depth()
    }

    /// Returns the byte rate of the audio data in bytes per second.
    #[inline]
    pub const fn bytes_rate(&self) -> u32 {
        self.bits_rate() / 8
    }

    /// Creates a silence chunk of the given duration.
    #[inline]
    pub fn silence_chunk(&self, duration: Duration) -> super::SilenceChunk {
        super::SilenceChunk::new(*self, duration)
    }

    /// Creates a data chunk from raw bytes.
    ///
    /// The bytes should be in little-endian 16-bit PCM format.
    #[inline]
    pub fn data_chunk(&self, data: Vec<u8>) -> super::DataChunk {
        super::DataChunk::new(*self, data)
    }

    /// Creates a data chunk from i16 samples.
    ///
    /// Samples are converted to little-endian bytes.
    pub fn data_chunk_from_samples(&self, samples: &[i16]) -> super::DataChunk {
        let mut data = Vec::with_capacity(samples.len() * 2);
        for sample in samples {
            data.extend_from_slice(&sample.to_le_bytes());
        }
        super::DataChunk::new(*self, data)
    }
}

impl std::fmt::Display for Format {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Format::L16Mono16K => write!(f, "audio/L16; rate=16000; channels=1"),
            Format::L16Mono24K => write!(f, "audio/L16; rate=24000; channels=1"),
            Format::L16Mono48K => write!(f, "audio/L16; rate=48000; channels=1"),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

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
}
