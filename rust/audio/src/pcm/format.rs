//! PCM audio format definitions.
//!
//! Re-exports `Format` from the resampler module and extends it with
//! chunk creation methods for PCM audio processing.

// Re-export Format from resampler module
pub use crate::resampler::format::Format;

/// Extension trait for Format that adds chunk creation methods.
pub trait FormatExt {
    /// Creates a silence chunk of the given duration.
    fn silence_chunk(&self, duration: std::time::Duration) -> super::SilenceChunk;

    /// Creates a data chunk from raw bytes.
    fn data_chunk(&self, data: Vec<u8>) -> super::DataChunk;

    /// Creates a data chunk from i16 samples.
    fn data_chunk_from_samples(&self, samples: &[i16]) -> super::DataChunk;
}

impl FormatExt for Format {
    #[inline]
    fn silence_chunk(&self, duration: std::time::Duration) -> super::SilenceChunk {
        super::SilenceChunk::new(*self, duration)
    }

    #[inline]
    fn data_chunk(&self, data: Vec<u8>) -> super::DataChunk {
        super::DataChunk::new(*self, data)
    }

    fn data_chunk_from_samples(&self, samples: &[i16]) -> super::DataChunk {
        let mut data = Vec::with_capacity(samples.len() * 2);
        for sample in samples {
            data.extend_from_slice(&sample.to_le_bytes());
        }
        super::DataChunk::new(*self, data)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Duration;

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
