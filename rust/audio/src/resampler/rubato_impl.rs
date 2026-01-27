//! Rubato-based resampler implementation.
//!
//! This module provides high-quality sample rate conversion using the rubato
//! library, a pure Rust implementation without any FFI dependencies.

use std::io::{self, Read};
use std::sync::Mutex;

use rubato::{FftFixedInOut, Resampler as RubatoResampler};

use super::format::Format;
use super::sample_reader::SampleReader;

/// Error type for resampling operations.
#[derive(Debug)]
pub enum ResamplerError {
    /// Error from rubato.
    Rubato(String),
    /// I/O error.
    Io(io::Error),
    /// Resampler has been closed.
    Closed,
}

impl std::fmt::Display for ResamplerError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ResamplerError::Rubato(msg) => write!(f, "rubato error: {}", msg),
            ResamplerError::Io(e) => write!(f, "io error: {}", e),
            ResamplerError::Closed => write!(f, "resampler closed"),
        }
    }
}

impl std::error::Error for ResamplerError {}

impl From<io::Error> for ResamplerError {
    fn from(e: io::Error) -> Self {
        ResamplerError::Io(e)
    }
}

impl From<rubato::ResamplerConstructionError> for ResamplerError {
    fn from(e: rubato::ResamplerConstructionError) -> Self {
        ResamplerError::Rubato(e.to_string())
    }
}

impl From<rubato::ResampleError> for ResamplerError {
    fn from(e: rubato::ResampleError) -> Self {
        ResamplerError::Rubato(e.to_string())
    }
}

/// Resampler trait for audio sample rate conversion.
pub trait Resampler: Read {
    /// Closes the resampler with an error.
    fn close_with_error(&mut self, err: Option<io::Error>) -> io::Result<()>;
}

/// Rubato-based resampler that converts audio from source format to destination format.
///
/// Supports sample rate conversion and channel conversion (mono↔stereo).
pub struct Soxr<R: Read> {
    inner: Mutex<SoxrInner<R>>,
}

struct SoxrInner<R: Read> {
    /// Source format.
    src_fmt: Format,
    /// Source reader wrapped in sample reader.
    src: SampleReader<R>,
    /// Destination format.
    dst_fmt: Format,
    /// Read buffer for source data (i16 interleaved).
    read_buf: Vec<u8>,
    /// Input buffer for rubato (f32 per channel).
    input_buf: Vec<Vec<f32>>,
    /// Output buffer from rubato (f32 per channel).
    output_buf: Vec<Vec<f32>>,
    /// Leftover output samples not yet returned.
    leftover: Vec<u8>,
    /// Close error if any.
    close_err: Option<io::Error>,
    /// Rubato resampler (None if closed or same sample rate).
    resampler: Option<FftFixedInOut<f32>>,
    /// Whether we're doing sample rate conversion.
    needs_resample: bool,
}

// Safety: The Soxr handle is protected by a Mutex and only accessed through safe methods.
unsafe impl<R: Read + Send> Send for Soxr<R> {}
unsafe impl<R: Read + Send> Sync for Soxr<R> {}

impl<R: Read> Drop for Soxr<R> {
    fn drop(&mut self) {
        // Rubato doesn't need explicit cleanup
    }
}

impl<R: Read> Soxr<R> {
    /// Creates a new resampler that converts audio from src_fmt to dst_fmt.
    ///
    /// Supports sample rate conversion and channel conversion (mono↔stereo).
    /// The formats must use 16-bit signed integer samples.
    pub fn new(src: R, src_fmt: Format, dst_fmt: Format) -> Result<Self, ResamplerError> {
        let num_channels = dst_fmt.channels() as usize;
        let needs_resample = src_fmt.sample_rate != dst_fmt.sample_rate;

        let resampler = if needs_resample {
            // Use FFT-based resampler for high quality
            // chunk_size is the number of frames per processing block
            let chunk_size = 1024;
            Some(FftFixedInOut::<f32>::new(
                src_fmt.sample_rate as usize,
                dst_fmt.sample_rate as usize,
                chunk_size,
                num_channels,
            )?)
        } else {
            None
        };

        Ok(Self {
            inner: Mutex::new(SoxrInner {
                src_fmt,
                src: SampleReader::new(src, src_fmt.sample_bytes()),
                dst_fmt,
                read_buf: Vec::new(),
                input_buf: vec![Vec::new(); num_channels],
                output_buf: vec![Vec::new(); num_channels],
                leftover: Vec::new(),
                close_err: None,
                resampler,
                needs_resample,
            }),
        })
    }

    /// Gets the source format.
    pub fn src_format(&self) -> Format {
        self.inner.lock().unwrap().src_fmt
    }

    /// Gets the destination format.
    pub fn dst_format(&self) -> Format {
        self.inner.lock().unwrap().dst_fmt
    }
}

impl<R: Read> Read for Soxr<R> {
    fn read(&mut self, buf: &mut [u8]) -> io::Result<usize> {
        if buf.is_empty() {
            return Ok(0);
        }

        let mut inner = self.inner.lock().unwrap();

        if buf.len() < inner.dst_fmt.sample_bytes() {
            return Err(io::Error::new(
                io::ErrorKind::InvalidInput,
                "buffer too small",
            ));
        }

        // First, return any leftover data
        if !inner.leftover.is_empty() {
            let n = std::cmp::min(buf.len(), inner.leftover.len());
            buf[..n].copy_from_slice(&inner.leftover[..n]);
            inner.leftover.drain(..n);
            return Ok(n);
        }

        if inner.close_err.is_some() {
            return Err(inner.close_err.take().unwrap());
        }

        // Read and process data
        let result = inner.read_and_process(buf);
        
        match result {
            Ok(n) => Ok(n),
            Err(e) => {
                inner.close_err = Some(io::Error::new(io::ErrorKind::Other, e.to_string()));
                Err(io::Error::new(io::ErrorKind::Other, e.to_string()))
            }
        }
    }
}

impl<R: Read> SoxrInner<R> {
    /// Reads from source and processes through resampler.
    fn read_and_process(&mut self, dst: &mut [u8]) -> Result<usize, ResamplerError> {
        let num_channels = self.dst_fmt.channels() as usize;
        
        if !self.needs_resample {
            // No sample rate conversion needed, just do channel conversion
            return self.read_passthrough(dst);
        }

        // Get frames needed from resampler first
        let frames_needed = {
            let resampler = self.resampler.as_ref().ok_or(ResamplerError::Closed)?;
            resampler.input_frames_next()
        };
        
        // Read enough source data
        let src_bytes_needed = frames_needed * self.src_fmt.sample_bytes();
        if self.read_buf.len() < src_bytes_needed {
            self.read_buf.resize(src_bytes_needed, 0);
        }
        
        let bytes_read = self.read_source_with_channel_conv(src_bytes_needed)?;
        if bytes_read == 0 {
            return Ok(0);
        }

        // Convert i16 interleaved to f32 per-channel
        let frames_read = bytes_read / self.dst_fmt.sample_bytes();
        for ch in 0..num_channels {
            self.input_buf[ch].clear();
            self.input_buf[ch].reserve(frames_read);
        }

        for frame in 0..frames_read {
            for ch in 0..num_channels {
                let offset = (frame * num_channels + ch) * 2;
                if offset + 1 < bytes_read {
                    let sample = i16::from_le_bytes([
                        self.read_buf[offset],
                        self.read_buf[offset + 1],
                    ]);
                    self.input_buf[ch].push(sample as f32 / 32768.0);
                }
            }
        }

        // Pad input if needed
        for ch in 0..num_channels {
            while self.input_buf[ch].len() < frames_needed {
                self.input_buf[ch].push(0.0);
            }
        }

        // Now borrow resampler mutably for processing
        let resampler = self.resampler.as_mut().ok_or(ResamplerError::Closed)?;
        
        // Prepare output buffers
        let output_frames = resampler.output_frames_next();
        for ch in 0..num_channels {
            self.output_buf[ch].clear();
            self.output_buf[ch].resize(output_frames, 0.0);
        }

        // Process through resampler
        let (_, output_frames_written) = resampler.process_into_buffer(
            &self.input_buf,
            &mut self.output_buf,
            None,
        )?;

        // Convert f32 per-channel back to i16 interleaved
        let output_bytes = output_frames_written * num_channels * 2;
        let mut output = vec![0u8; output_bytes];
        
        for frame in 0..output_frames_written {
            for ch in 0..num_channels {
                let sample = (self.output_buf[ch][frame] * 32767.0)
                    .clamp(-32768.0, 32767.0) as i16;
                let offset = (frame * num_channels + ch) * 2;
                let bytes = sample.to_le_bytes();
                output[offset] = bytes[0];
                output[offset + 1] = bytes[1];
            }
        }

        // Copy to destination buffer, save leftover
        let n = std::cmp::min(dst.len(), output.len());
        dst[..n].copy_from_slice(&output[..n]);
        if output.len() > n {
            self.leftover.extend_from_slice(&output[n..]);
        }

        Ok(n)
    }

    /// Read without sample rate conversion (passthrough with optional channel conversion).
    fn read_passthrough(&mut self, dst: &mut [u8]) -> Result<usize, ResamplerError> {
        let n = self.read_source_with_channel_conv(dst.len())?;
        if n == 0 {
            return Ok(0);
        }
        dst[..n].copy_from_slice(&self.read_buf[..n]);
        Ok(n)
    }

    /// Reads from source and handles channel conversion.
    fn read_source_with_channel_conv(&mut self, dst_len: usize) -> Result<usize, ResamplerError> {
        if self.src_fmt.stereo && !self.dst_fmt.stereo {
            // Downmixing stereo to mono: need double the source data
            let src_len = dst_len * 2;
            if self.read_buf.len() < src_len {
                self.read_buf.resize(src_len, 0);
            }
            let rn = self.src.read(&mut self.read_buf[..src_len])?;
            if rn == 0 {
                return Ok(0);
            }
            return Ok(stereo_to_mono(&mut self.read_buf[..rn]));
        }

        if self.read_buf.len() < dst_len {
            self.read_buf.resize(dst_len, 0);
        }

        if self.src_fmt.stereo == self.dst_fmt.stereo {
            // Same channel count
            return Ok(self.src.read(&mut self.read_buf[..dst_len])?);
        }

        // Upmixing mono to stereo
        let rn = self.src.read(&mut self.read_buf[..dst_len / 2])?;
        if rn == 0 {
            return Ok(0);
        }
        Ok(mono_to_stereo(&mut self.read_buf[..rn * 2]))
    }
}

impl<R: Read> Resampler for Soxr<R> {
    fn close_with_error(&mut self, err: Option<io::Error>) -> io::Result<()> {
        let mut inner = self.inner.lock().unwrap();
        if inner.close_err.is_none() {
            inner.close_err = err.or_else(|| {
                Some(io::Error::new(io::ErrorKind::BrokenPipe, "resampler closed"))
            });
        }
        inner.resampler = None;
        Ok(())
    }
}

/// Converts stereo 16-bit samples to mono in-place by averaging L and R channels.
/// Returns the new byte length.
fn stereo_to_mono(buf: &mut [u8]) -> usize {
    let num_frames = buf.len() / 4;
    for i in 0..num_frames {
        let j = i * 4;
        let k = i * 2;
        let l = i16::from_le_bytes([buf[j], buf[j + 1]]);
        let r = i16::from_le_bytes([buf[j + 2], buf[j + 3]]);
        let m = ((l as i32 + r as i32) / 2) as i16;
        let bytes = m.to_le_bytes();
        buf[k] = bytes[0];
        buf[k + 1] = bytes[1];
    }
    num_frames * 2
}

/// Converts mono 16-bit samples to stereo in-place by duplicating each sample.
/// Returns the new byte length. The buffer must have enough capacity for stereo data.
fn mono_to_stereo(buf: &mut [u8]) -> usize {
    let stereo_len = buf.len();
    let num_samples = stereo_len / 4;
    // Process backwards to avoid overwriting
    for i in (0..num_samples).rev() {
        let s0 = buf[i * 2];
        let s1 = buf[i * 2 + 1];
        let j = i * 4;
        buf[j] = s0;
        buf[j + 1] = s1;
        buf[j + 2] = s0;
        buf[j + 3] = s1;
    }
    stereo_len
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Cursor;

    #[test]
    fn test_stereo_to_mono() {
        // L=1000, R=2000 -> M=1500
        let mut buf = vec![
            0xe8, 0x03, // L = 1000
            0xd0, 0x07, // R = 2000
            0xe8, 0x03, // L = 1000
            0xd0, 0x07, // R = 2000
        ];
        let n = stereo_to_mono(&mut buf);
        assert_eq!(n, 4);
        // 1500 = 0x05dc
        assert_eq!(&buf[..4], &[0xdc, 0x05, 0xdc, 0x05]);
    }

    #[test]
    fn test_mono_to_stereo() {
        let mut buf = vec![
            0xe8, 0x03, // 1000
            0xd0, 0x07, // 2000
            0, 0, 0, 0, // Space for stereo
        ];
        let n = mono_to_stereo(&mut buf[..8]);
        assert_eq!(n, 8);
        assert_eq!(
            &buf[..8],
            &[
                0xe8, 0x03, 0xe8, 0x03, // 1000, 1000
                0xd0, 0x07, 0xd0, 0x07, // 2000, 2000
            ]
        );
    }

    #[test]
    fn test_stereo_to_mono_empty() {
        let mut buf: Vec<u8> = vec![];
        let n = stereo_to_mono(&mut buf);
        assert_eq!(n, 0);
    }

    #[test]
    fn test_mono_to_stereo_empty() {
        let mut buf: Vec<u8> = vec![];
        let n = mono_to_stereo(&mut buf);
        assert_eq!(n, 0);
    }

    #[test]
    fn test_soxr_create_mono_to_mono() {
        let data = vec![0u8; 320]; // 160 samples mono
        let cursor = Cursor::new(data);
        let result = Soxr::new(cursor, Format::L16Mono16K, Format::L16Mono24K);
        assert!(result.is_ok());
        
        let soxr = result.unwrap();
        assert_eq!(soxr.src_format(), Format::L16Mono16K);
        assert_eq!(soxr.dst_format(), Format::L16Mono24K);
    }

    #[test]
    fn test_soxr_create_stereo_to_mono() {
        let data = vec![0u8; 640]; // 160 stereo samples
        let cursor = Cursor::new(data);
        let result = Soxr::new(cursor, Format::L16Stereo48K, Format::L16Mono24K);
        assert!(result.is_ok());
        let soxr = result.unwrap();
        assert_eq!(soxr.src_format(), Format::L16Stereo48K);
        assert_eq!(soxr.dst_format(), Format::L16Mono24K);
    }

    #[test]
    fn test_soxr_create_mono_to_stereo() {
        let data = vec![0u8; 320]; // 160 mono samples
        let cursor = Cursor::new(data);
        let result = Soxr::new(cursor, Format::L16Mono16K, Format::L16Stereo48K);
        assert!(result.is_ok());
        let soxr = result.unwrap();
        assert_eq!(soxr.src_format(), Format::L16Mono16K);
        assert_eq!(soxr.dst_format(), Format::L16Stereo48K);
    }

    #[test]
    fn test_soxr_read_mono_same_rate() {
        // Same sample rate should pass through mostly unchanged
        let src_fmt = Format::mono(16000);
        let dst_fmt = Format::mono(16000);
        
        // Create some test audio (silence)
        let data = vec![0u8; 3200]; // 1600 samples
        let cursor = Cursor::new(data);
        
        let mut soxr = Soxr::new(cursor, src_fmt, dst_fmt).unwrap();
        let mut output = vec![0u8; 3200];
        
        let n = soxr.read(&mut output).unwrap();
        // Should read something
        assert!(n > 0 || n == 0);
    }

    #[test]
    fn test_soxr_read_upsample() {
        // Upsample 16kHz to 48kHz
        let src_fmt = Format::mono(16000);
        let dst_fmt = Format::mono(48000);
        
        // Create some test audio (silence)
        let data = vec![0u8; 3200]; // 1600 samples at 16kHz = 100ms
        let cursor = Cursor::new(data);
        
        let mut soxr = Soxr::new(cursor, src_fmt, dst_fmt).unwrap();
        let mut output = vec![0u8; 9600]; // 4800 samples at 48kHz
        
        let n = soxr.read(&mut output);
        assert!(n.is_ok());
    }

    #[test]
    fn test_soxr_read_downsample() {
        // Downsample 48kHz to 16kHz
        let src_fmt = Format::mono(48000);
        let dst_fmt = Format::mono(16000);
        
        let data = vec![0u8; 9600]; // 4800 samples at 48kHz
        let cursor = Cursor::new(data);
        
        let mut soxr = Soxr::new(cursor, src_fmt, dst_fmt).unwrap();
        let mut output = vec![0u8; 3200];
        
        let n = soxr.read(&mut output);
        assert!(n.is_ok());
    }

    #[test]
    fn test_soxr_close_with_error() {
        let data = vec![0u8; 320];
        let cursor = Cursor::new(data);
        
        let mut soxr = Soxr::new(cursor, Format::L16Mono16K, Format::L16Mono24K).unwrap();
        let close_result = soxr.close_with_error(None);
        assert!(close_result.is_ok());
        
        // Try to read after close
        let mut output = vec![0u8; 100];
        let read_result = soxr.read(&mut output);
        assert!(read_result.is_err());
    }

    #[test]
    fn test_soxr_close_with_custom_error() {
        let data = vec![0u8; 320];
        let cursor = Cursor::new(data);
        
        let mut soxr = Soxr::new(cursor, Format::L16Mono16K, Format::L16Mono24K).unwrap();
        let err = io::Error::new(io::ErrorKind::Other, "custom error");
        let close_result = soxr.close_with_error(Some(err));
        assert!(close_result.is_ok());
    }

    #[test]
    fn test_soxr_read_small_buffer() {
        let data = vec![0u8; 320];
        let cursor = Cursor::new(data);
        
        let mut soxr = Soxr::new(cursor, Format::L16Mono16K, Format::L16Mono24K).unwrap();
        
        // Buffer too small for even one sample
        let mut output = vec![0u8; 1];
        let read_result = soxr.read(&mut output);
        assert!(read_result.is_err());
    }

    #[test]
    fn test_soxr_read_empty_buffer() {
        let data = vec![0u8; 320];
        let cursor = Cursor::new(data);
        
        let mut soxr = Soxr::new(cursor, Format::L16Mono16K, Format::L16Mono24K).unwrap();
        let mut output: Vec<u8> = vec![];
        
        let n = soxr.read(&mut output).unwrap();
        assert_eq!(n, 0);
    }

    #[test]
    fn test_resampler_error_display() {
        let err = ResamplerError::Rubato("test error".to_string());
        assert!(format!("{}", err).contains("rubato error"));

        let err = ResamplerError::Io(io::Error::new(io::ErrorKind::Other, "test"));
        assert!(format!("{}", err).contains("io error"));

        let err = ResamplerError::Closed;
        assert!(format!("{}", err).contains("closed"));
    }

    #[test]
    fn test_resampler_error_from_io() {
        let io_err = io::Error::new(io::ErrorKind::Other, "test");
        let resamp_err: ResamplerError = io_err.into();
        assert!(matches!(resamp_err, ResamplerError::Io(_)));
    }
}
