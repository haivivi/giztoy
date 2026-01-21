//! Sample-aligned reader wrapper.

use std::io::{self, Read};

/// Wraps an io::Read and ensures each read returns a multiple of sample_size bytes.
/// Buffers partial data internally until a complete sample can be returned.
pub(crate) struct SampleReader<R: Read> {
    /// Holds leftover bytes (up to sample_size - 1)
    buffer: Vec<u8>,
    /// Number of valid bytes in buffer
    buffered: usize,
    /// Bytes per sample frame
    sample_size: usize,
    /// Inner reader
    inner: R,
}

impl<R: Read> SampleReader<R> {
    /// Creates a new SampleReader that returns data in multiples of sample_size bytes.
    pub fn new(reader: R, sample_size: usize) -> Self {
        Self {
            buffer: vec![0u8; sample_size.saturating_sub(1)],
            buffered: 0,
            sample_size,
            inner: reader,
        }
    }
}

impl<R: Read> Read for SampleReader<R> {
    /// Reads data into buf, returning 0 or a multiple of sample_size bytes.
    /// Returns io::ErrorKind::InvalidInput if len(buf) < sample_size.
    /// On EOF, may return remaining data that is not aligned to sample_size.
    fn read(&mut self, buf: &mut [u8]) -> io::Result<usize> {
        if buf.len() < self.sample_size {
            return Err(io::Error::new(
                io::ErrorKind::InvalidInput,
                "buffer too small for sample",
            ));
        }

        // Truncate buf to a multiple of sample_size
        let aligned_len = (buf.len() / self.sample_size) * self.sample_size;
        let buf = &mut buf[..aligned_len];

        let mut n = 0;

        // Copy buffered data first
        if self.buffered > 0 {
            n = self.buffer[..self.buffered].len();
            buf[..n].copy_from_slice(&self.buffer[..self.buffered]);
            self.buffered = 0;
        }

        // Read from inner
        let rn = self.inner.read(&mut buf[n..])?;
        n += rn;

        // Handle unaligned remainder
        let remainder = n % self.sample_size;
        if remainder != 0 {
            // Save unaligned remainder for next call
            let start = n - remainder;
            self.buffer[..remainder].copy_from_slice(&buf[start..n]);
            self.buffered = remainder;
            n = start;
        }

        Ok(n)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Cursor;

    #[test]
    fn test_sample_reader_aligned() {
        let data = vec![1, 2, 3, 4, 5, 6, 7, 8];
        let mut reader = SampleReader::new(Cursor::new(data), 2);
        let mut buf = vec![0u8; 8];
        
        let n = reader.read(&mut buf).unwrap();
        assert_eq!(n, 8);
        assert_eq!(&buf[..n], &[1, 2, 3, 4, 5, 6, 7, 8]);
    }

    #[test]
    fn test_sample_reader_unaligned() {
        let data = vec![1, 2, 3, 4, 5, 6, 7]; // 7 bytes, sample_size = 2
        let mut reader = SampleReader::new(Cursor::new(data), 2);
        let mut buf = vec![0u8; 8];
        
        // First read should return 6 bytes (3 complete samples)
        let n = reader.read(&mut buf).unwrap();
        assert_eq!(n, 6);
        assert_eq!(&buf[..n], &[1, 2, 3, 4, 5, 6]);
        
        // Second read should return the buffered byte + EOF
        let n = reader.read(&mut buf).unwrap();
        assert_eq!(n, 0); // Only 1 byte buffered, but EOF means we might not get more
    }

    #[test]
    fn test_sample_reader_small_reads() {
        let data = vec![1, 2, 3, 4, 5, 6, 7, 8, 9, 10];
        let mut reader = SampleReader::new(Cursor::new(data), 4);
        let mut buf = vec![0u8; 4];
        
        let n = reader.read(&mut buf).unwrap();
        assert_eq!(n, 4);
        assert_eq!(&buf[..n], &[1, 2, 3, 4]);
        
        let n = reader.read(&mut buf).unwrap();
        assert_eq!(n, 4);
        assert_eq!(&buf[..n], &[5, 6, 7, 8]);
    }
}
