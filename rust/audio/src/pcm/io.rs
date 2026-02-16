//! Audio I/O adapters for converting between PCM chunk writers and byte-level I/O.
//!
//! This module provides adapters to bridge the gap between the
//! chunk-based `Writer` trait and Rust's standard `io::Write`/`io::Read`.

use super::{Chunk, DataChunk, Format, FormatExt};
use std::io::{self, Read};
use std::time::Duration;

/// A writer for chunks of audio data.
pub trait Writer: Send + Sync {
    /// Writes a chunk of audio data.
    fn write(&self, chunk: &dyn Chunk) -> io::Result<()>;
}

/// A function that implements the Writer trait.
pub struct WriteFunc<F>(pub F);

impl<F> Writer for WriteFunc<F>
where
    F: Fn(&dyn Chunk) -> io::Result<()> + Send + Sync,
{
    fn write(&self, chunk: &dyn Chunk) -> io::Result<()> {
        (self.0)(chunk)
    }
}

/// A Writer that discards all written chunks.
pub struct Discard;

impl Writer for Discard {
    fn write(&self, _chunk: &dyn Chunk) -> io::Result<()> {
        Ok(())
    }
}

/// Wraps a pcm `Writer` to provide an `io::Write` interface.
///
/// All bytes written are converted to `DataChunk` with the specified format.
pub struct IOWriter<W: Writer> {
    writer: W,
    format: Format,
}

/// Creates an `io::Write` adapter from a pcm `Writer` and format.
pub fn io_writer<W: Writer>(writer: W, format: Format) -> IOWriter<W> {
    IOWriter { writer, format }
}

impl<W: Writer> io::Write for IOWriter<W> {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        let chunk = DataChunk::new(self.format, buf.to_vec());
        self.writer.write(&chunk)?;
        Ok(buf.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

/// Wraps an `io::Write` to provide a pcm `Writer` interface.
///
/// All chunks are written to the underlying writer using `write_to`.
/// Uses interior mutability via `Mutex` since `Writer::write` takes `&self`.
pub struct ChunkWriter<W: io::Write> {
    writer: std::sync::Mutex<W>,
}

/// Creates a pcm `Writer` adapter from an `io::Write`.
pub fn chunk_writer<W: io::Write + Send + Sync>(writer: W) -> ChunkWriter<W> {
    ChunkWriter {
        writer: std::sync::Mutex::new(writer),
    }
}

impl<W: io::Write + Send + Sync> Writer for ChunkWriter<W> {
    fn write(&self, chunk: &dyn Chunk) -> io::Result<()> {
        let mut w = self.writer.lock().map_err(|e| {
            io::Error::new(io::ErrorKind::Other, e.to_string())
        })?;
        chunk.write_to(&mut *w)?;
        Ok(())
    }
}

/// Copies audio data from reader `r` to writer `w` using the specified format.
///
/// Reads data in chunks of at least 20ms duration and writes them as DataChunks.
/// Returns `Ok(())` on EOF, or any other error encountered.
pub fn copy(w: &dyn Writer, r: &mut dyn Read, format: Format) -> io::Result<()> {
    let min_chunk = format.bytes_in_duration(Duration::from_millis(20)) as usize;
    let buf_size = 10 * min_chunk;
    let mut buf = vec![0u8; buf_size];

    loop {
        let mut total_read = 0;

        // Read at least min_chunk bytes
        while total_read < min_chunk {
            match r.read(&mut buf[total_read..]) {
                Ok(0) => {
                    // EOF: write any remaining data
                    if total_read > 0 {
                        let chunk = DataChunk::new(format, buf[..total_read].to_vec());
                        w.write(&chunk)?;
                    }
                    return Ok(());
                }
                Ok(n) => total_read += n,
                Err(e) if e.kind() == io::ErrorKind::Interrupted => continue,
                Err(e) => return Err(e),
            }
        }

        let chunk = DataChunk::new(format, buf[..total_read].to_vec());
        w.write(&chunk)?;
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::{Arc, Mutex};

    #[test]
    fn test_discard() {
        let format = Format::L16Mono16K;
        let chunk = DataChunk::new(format, vec![1, 2, 3, 4]);
        Discard.write(&chunk).unwrap();
    }

    #[test]
    fn test_write_func() {
        let count = Arc::new(Mutex::new(0usize));
        let count_clone = count.clone();

        let writer = WriteFunc(move |_chunk: &dyn Chunk| {
            *count_clone.lock().unwrap() += 1;
            Ok(())
        });

        let format = Format::L16Mono16K;
        writer.write(&DataChunk::new(format, vec![1, 2])).unwrap();
        writer.write(&DataChunk::new(format, vec![3, 4])).unwrap();

        assert_eq!(*count.lock().unwrap(), 2);
    }

    #[test]
    fn test_io_writer() {
        use std::io::Write;

        let written = Arc::new(Mutex::new(Vec::<Vec<u8>>::new()));
        let written_clone = written.clone();

        let pcm_writer = WriteFunc(move |chunk: &dyn Chunk| {
            if let Some(bytes) = chunk.as_bytes() {
                written_clone.lock().unwrap().push(bytes.to_vec());
            }
            Ok(())
        });

        let mut adapter = io_writer(pcm_writer, Format::L16Mono16K);
        adapter.write_all(&[1, 2, 3, 4]).unwrap();
        adapter.write_all(&[5, 6]).unwrap();

        let chunks = written.lock().unwrap();
        assert_eq!(chunks.len(), 2);
        assert_eq!(chunks[0], vec![1, 2, 3, 4]);
        assert_eq!(chunks[1], vec![5, 6]);
    }

    #[test]
    fn test_copy() {
        let written = Arc::new(Mutex::new(Vec::<Vec<u8>>::new()));
        let written_clone = written.clone();

        let writer = WriteFunc(move |chunk: &dyn Chunk| {
            if let Some(bytes) = chunk.as_bytes() {
                written_clone.lock().unwrap().push(bytes.to_vec());
            }
            Ok(())
        });

        let format = Format::L16Mono16K;
        let min_bytes = format.bytes_in_duration(Duration::from_millis(20)) as usize;

        // Create input data larger than min_chunk
        let input_data = vec![42u8; min_bytes * 3];
        let mut cursor = io::Cursor::new(input_data.clone());

        copy(&writer, &mut cursor, format).unwrap();

        let chunks = written.lock().unwrap();
        assert!(!chunks.is_empty());

        // Verify all data was written
        let total: Vec<u8> = chunks.iter().flat_map(|c| c.iter().copied()).collect();
        assert_eq!(total.len(), input_data.len());
    }
}
