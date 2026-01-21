//! Audio chunk types.

use super::Format;
use std::io::{self, Write};
use std::time::Duration;

/// A chunk of audio data.
pub trait Chunk: Send + Sync {
    /// Returns the length of the audio data in bytes.
    fn len(&self) -> u64;

    /// Returns true if the chunk is empty.
    fn is_empty(&self) -> bool {
        self.len() == 0
    }

    /// Returns the audio format of this chunk.
    fn format(&self) -> Format;

    /// Writes the audio data to the writer.
    fn write_to(&self, w: &mut dyn Write) -> io::Result<u64>;

    /// Returns the audio data as a byte slice, if available.
    fn as_bytes(&self) -> Option<&[u8]> {
        None
    }
}

/// A chunk of raw audio data.
#[derive(Debug, Clone)]
pub struct DataChunk {
    data: Vec<u8>,
    format: Format,
}

impl DataChunk {
    /// Creates a new data chunk with the given format and data.
    pub fn new(format: Format, data: Vec<u8>) -> Self {
        Self { data, format }
    }

    /// Returns a mutable reference to the underlying data.
    pub fn data_mut(&mut self) -> &mut Vec<u8> {
        &mut self.data
    }

    /// Consumes the chunk and returns the underlying data.
    pub fn into_data(self) -> Vec<u8> {
        self.data
    }

    /// Returns the audio samples as i16 values.
    pub fn samples(&self) -> Vec<i16> {
        self.data
            .chunks_exact(2)
            .map(|bytes| i16::from_le_bytes([bytes[0], bytes[1]]))
            .collect()
    }
}

impl Chunk for DataChunk {
    fn len(&self) -> u64 {
        self.data.len() as u64
    }

    fn format(&self) -> Format {
        self.format
    }

    fn write_to(&self, w: &mut dyn Write) -> io::Result<u64> {
        w.write_all(&self.data)?;
        Ok(self.data.len() as u64)
    }

    fn as_bytes(&self) -> Option<&[u8]> {
        Some(&self.data)
    }
}

/// A chunk that produces silence of a specified duration.
#[derive(Debug, Clone)]
pub struct SilenceChunk {
    duration: Duration,
    len: u64,
    format: Format,
}

impl SilenceChunk {
    /// Creates a new silence chunk with the given format and duration.
    pub fn new(format: Format, duration: Duration) -> Self {
        let len = format.bytes_in_duration(duration);
        Self {
            duration,
            len,
            format,
        }
    }

    /// Returns the duration of the silence.
    pub fn duration(&self) -> Duration {
        self.duration
    }
}

/// Static buffer of zeros for writing silence efficiently.
static EMPTY_BYTES: [u8; 32000] = [0u8; 32000];

impl Chunk for SilenceChunk {
    fn len(&self) -> u64 {
        self.len
    }

    fn format(&self) -> Format {
        self.format
    }

    fn write_to(&self, w: &mut dyn Write) -> io::Result<u64> {
        let mut remaining = self.len;
        let mut written = 0u64;

        while remaining > 0 {
            let to_write = remaining.min(EMPTY_BYTES.len() as u64) as usize;
            w.write_all(&EMPTY_BYTES[..to_write])?;
            written += to_write as u64;
            remaining -= to_write as u64;
        }

        Ok(written)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_data_chunk() {
        let format = Format::L16Mono16K;
        let samples = vec![1000i16, 2000, -1000, -2000];
        let chunk = format.data_chunk_from_samples(&samples);

        assert_eq!(chunk.len(), 8);
        assert_eq!(chunk.format(), format);
        assert_eq!(chunk.samples(), samples);
    }

    #[test]
    fn test_silence_chunk() {
        let format = Format::L16Mono16K;
        let chunk = format.silence_chunk(Duration::from_millis(100));

        assert_eq!(chunk.len(), 3200); // 100ms at 16kHz = 3200 bytes
        assert_eq!(chunk.format(), format);
        assert_eq!(chunk.duration(), Duration::from_millis(100));

        let mut buf = Vec::new();
        chunk.write_to(&mut buf).unwrap();
        assert_eq!(buf.len(), 3200);
        assert!(buf.iter().all(|&b| b == 0));
    }

    #[test]
    fn test_data_chunk_write_to() {
        let format = Format::L16Mono16K;
        let data = vec![0x01, 0x02, 0x03, 0x04];
        let chunk = format.data_chunk(data.clone());

        let mut buf = Vec::new();
        let written = chunk.write_to(&mut buf).unwrap();

        assert_eq!(written, 4);
        assert_eq!(buf, data);
    }
}
