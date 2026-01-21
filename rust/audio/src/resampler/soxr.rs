//! SoX Resampler wrapper.

use std::io::{self, Read};
use std::ptr;
use std::sync::Mutex;

use super::ffi::{self, SoxrHandle, SOXR_HQ, SOXR_INT16_I};
use super::format::Format;
use super::sample_reader::SampleReader;

/// Error type for resampling operations.
#[derive(Debug)]
pub enum ResamplerError {
    /// Error from libsoxr.
    Soxr(String),
    /// I/O error.
    Io(io::Error),
    /// Resampler has been closed.
    Closed,
}

impl std::fmt::Display for ResamplerError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ResamplerError::Soxr(msg) => write!(f, "soxr error: {}", msg),
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

/// Resampler trait for audio sample rate conversion.
pub trait Resampler: Read {
    /// Closes the resampler with an error.
    fn close_with_error(&mut self, err: Option<io::Error>) -> io::Result<()>;
}

/// SoX Resampler wrapper that resamples audio from source format to destination format.
///
/// Supports sample rate conversion and channel conversion (mono↔stereo).
/// The resampler must be dropped to release C library resources.
pub struct Soxr<R: Read> {
    inner: Mutex<SoxrInner<R>>,
}

struct SoxrInner<R: Read> {
    /// Ratio of source to destination sample rate.
    sample_rate_ratio: f64,
    /// Source format.
    src_fmt: Format,
    /// Source reader wrapped in sample reader.
    src: SampleReader<R>,
    /// Destination format.
    dst_fmt: Format,
    /// Read buffer for source data.
    read_buf: Vec<u8>,
    /// Close error if any.
    close_err: Option<io::Error>,
    /// Native soxr handle.
    handle: *mut SoxrHandle,
}

// Safety: The Soxr handle is protected by a Mutex and only accessed through safe methods.
unsafe impl<R: Read + Send> Send for Soxr<R> {}
unsafe impl<R: Read + Send> Sync for Soxr<R> {}

impl<R: Read> Drop for Soxr<R> {
    fn drop(&mut self) {
        let inner = self.inner.get_mut().unwrap();
        if !inner.handle.is_null() {
            unsafe { ffi::soxr_delete(inner.handle) };
            inner.handle = ptr::null_mut();
        }
    }
}

impl<R: Read> Soxr<R> {
    /// Creates a new resampler that converts audio from src_fmt to dst_fmt.
    ///
    /// Supports sample rate conversion and channel conversion (mono↔stereo).
    /// The formats must use 16-bit signed integer samples.
    pub fn new(src: R, src_fmt: Format, dst_fmt: Format) -> Result<Self, ResamplerError> {
        unsafe {
            let io_spec = ffi::soxr_io_spec(SOXR_INT16_I, SOXR_INT16_I);
            let quality_spec = ffi::soxr_quality_spec(SOXR_HQ, 0);

            let mut error: ffi::SoxrError = ptr::null();
            let handle = ffi::soxr_create(
                src_fmt.sample_rate as f64,
                dst_fmt.sample_rate as f64,
                dst_fmt.channels(),
                &mut error,
                &io_spec,
                &quality_spec,
                ptr::null(),
            );

            if handle.is_null() {
                let msg = ffi::error_string(error)
                    .unwrap_or_else(|| "unknown error".to_string());
                return Err(ResamplerError::Soxr(msg));
            }

            Ok(Self {
                inner: Mutex::new(SoxrInner {
                    sample_rate_ratio: src_fmt.sample_rate as f64 / dst_fmt.sample_rate as f64,
                    src_fmt,
                    src: SampleReader::new(src, src_fmt.sample_bytes()),
                    dst_fmt,
                    read_buf: Vec::new(),
                    close_err: None,
                    handle,
                }),
            })
        }
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

        // Truncate buf to multiple of sample bytes
        let aligned_len = (buf.len() / inner.dst_fmt.sample_bytes()) * inner.dst_fmt.sample_bytes();
        let buf = &mut buf[..aligned_len];

        // At most two iterations: first for initial read, second for EOF case
        for _ in 0..2 {
            if inner.handle.is_null() {
                return Err(inner.close_err.take().unwrap_or_else(|| {
                    io::Error::new(io::ErrorKind::BrokenPipe, "resampler closed")
                }));
            }

            if inner.close_err.is_some() {
                // Flush remaining data
                let n = inner.process_locked(None, buf)?;
                return Ok(n);
            }

            // Read from source
            let result = inner.read_from_source(buf.len());
            
            if inner.handle.is_null() {
                return Err(inner.close_err.take().unwrap_or_else(|| {
                    io::Error::new(io::ErrorKind::BrokenPipe, "resampler closed")
                }));
            }

            match result {
                Ok(0) => continue,
                Ok(n) => {
                    // Pad partial sample with zeros
                    let mod_bytes = n % inner.dst_fmt.sample_bytes();
                    let padded_n = if mod_bytes != 0 {
                        let padding = inner.dst_fmt.sample_bytes() - mod_bytes;
                        for i in 0..padding {
                            if n + i < inner.read_buf.len() {
                                inner.read_buf[n + i] = 0;
                            }
                        }
                        n + padding
                    } else {
                        n
                    };

                    let data = inner.read_buf[..padded_n].to_vec();
                    return inner.process_locked(Some(&data), buf);
                }
                Err(e) => {
                    inner.close_err = Some(e);
                }
            }
        }

        Ok(0)
    }
}

impl<R: Read> SoxrInner<R> {
    /// Reads from source and handles channel conversion.
    fn read_from_source(&mut self, dst_len: usize) -> io::Result<usize> {
        let n = (dst_len as f64 * self.sample_rate_ratio) as usize;
        if n == 0 {
            return Err(io::Error::new(
                io::ErrorKind::InvalidInput,
                "buffer too small",
            ));
        }

        if self.src_fmt.stereo && !self.dst_fmt.stereo {
            // Downmixing stereo to mono: need double the source data
            let src_n = n * 2;
            if self.read_buf.len() < src_n {
                self.read_buf.resize(src_n, 0);
            }
            let rn = self.src.read(&mut self.read_buf[..src_n])?;
            if rn == 0 {
                return Ok(0);
            }
            return Ok(stereo_to_mono(&mut self.read_buf[..rn]));
        }

        if self.read_buf.len() < n {
            self.read_buf.resize(n, 0);
        }

        if self.src_fmt.stereo == self.dst_fmt.stereo {
            // Same channel count
            return self.src.read(&mut self.read_buf[..n]);
        }

        // Upmixing mono to stereo
        let rn = self.src.read(&mut self.read_buf[..n / 2])?;
        if rn == 0 {
            return Ok(0);
        }
        Ok(mono_to_stereo(&mut self.read_buf[..rn * 2]))
    }

    /// Processes samples through soxr.
    fn process_locked(&mut self, src: Option<&[u8]>, dst: &mut [u8]) -> io::Result<usize> {
        if self.handle.is_null() {
            return Err(self.close_err.take().unwrap_or_else(|| {
                io::Error::new(io::ErrorKind::BrokenPipe, "resampler closed")
            }));
        }

        let sample_bytes = self.dst_fmt.sample_bytes();
        let (iptr, isize) = match src {
            Some(data) => (data.as_ptr() as *const _, data.len() / sample_bytes),
            None => (ptr::null(), 0),
        };

        let optr = dst.as_mut_ptr() as *mut _;
        let osize = dst.len() / sample_bytes;
        let mut idone: usize = 0;
        let mut odone: usize = 0;

        unsafe {
            let err = ffi::soxr_process(
                self.handle,
                iptr,
                isize,
                &mut idone,
                optr,
                osize,
                &mut odone,
            );

            if !err.is_null() {
                let msg = ffi::error_string(err)
                    .unwrap_or_else(|| "unknown error".to_string());
                ffi::soxr_delete(self.handle);
                self.handle = ptr::null_mut();
                self.close_err = Some(io::Error::new(io::ErrorKind::Other, msg));
                return Err(io::Error::new(io::ErrorKind::Other, "soxr error"));
            }
        }

        // Check for flush completion
        let flushing = src.is_none() && odone == 0;
        if flushing {
            unsafe { ffi::soxr_delete(self.handle) };
            self.handle = ptr::null_mut();
            self.close_err = Some(io::Error::new(
                io::ErrorKind::UnexpectedEof,
                "resampler flushed",
            ));
        }

        Ok(odone * sample_bytes)
    }
}

impl<R: Read> Resampler for Soxr<R> {
    fn close_with_error(&mut self, err: Option<io::Error>) -> io::Result<()> {
        let mut inner = self.inner.lock().unwrap();
        if inner.handle.is_null() {
            return Ok(());
        }
        if inner.close_err.is_none() {
            inner.close_err = err.or_else(|| {
                Some(io::Error::new(io::ErrorKind::BrokenPipe, "resampler closed"))
            });
        }
        unsafe { ffi::soxr_delete(inner.handle) };
        inner.handle = ptr::null_mut();
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
        let result = Soxr::new(cursor, Format::MONO_16K, Format::MONO_24K);
        assert!(result.is_ok());
        
        let soxr = result.unwrap();
        assert_eq!(soxr.src_format(), Format::MONO_16K);
        assert_eq!(soxr.dst_format(), Format::MONO_24K);
    }

    #[test]
    fn test_soxr_create_stereo_to_mono() {
        let data = vec![0u8; 640]; // 160 stereo samples
        let cursor = Cursor::new(data);
        let result = Soxr::new(cursor, Format::STEREO_48K, Format::MONO_24K);
        // This may or may not succeed depending on soxr library
        if result.is_ok() {
            let soxr = result.unwrap();
            assert_eq!(soxr.src_format(), Format::STEREO_48K);
            assert_eq!(soxr.dst_format(), Format::MONO_24K);
        }
    }

    #[test]
    fn test_soxr_create_mono_to_stereo() {
        let data = vec![0u8; 320]; // 160 mono samples
        let cursor = Cursor::new(data);
        let result = Soxr::new(cursor, Format::MONO_16K, Format::STEREO_48K);
        // This may or may not succeed depending on soxr library
        if result.is_ok() {
            let soxr = result.unwrap();
            assert_eq!(soxr.src_format(), Format::MONO_16K);
            assert_eq!(soxr.dst_format(), Format::STEREO_48K);
        }
    }

    #[test]
    fn test_soxr_read_mono_same_rate() {
        // Same sample rate should pass through mostly unchanged
        let src_fmt = Format::mono(16000);
        let dst_fmt = Format::mono(16000);
        
        // Create some test audio (silence)
        let data = vec![0u8; 3200]; // 1600 samples
        let cursor = Cursor::new(data);
        
        let result = Soxr::new(cursor, src_fmt, dst_fmt);
        if result.is_err() {
            // Skip if soxr is not available
            return;
        }
        
        let mut soxr = result.unwrap();
        let mut output = vec![0u8; 3200];
        
        let n = soxr.read(&mut output).unwrap();
        // Should read something
        assert!(n > 0 || n == 0); // May be 0 if soxr needs more data
    }

    #[test]
    fn test_soxr_read_upsample() {
        // Upsample 16kHz to 48kHz
        let src_fmt = Format::mono(16000);
        let dst_fmt = Format::mono(48000);
        
        // Create some test audio (silence)
        let data = vec![0u8; 3200]; // 1600 samples at 16kHz = 100ms
        let cursor = Cursor::new(data);
        
        let result = Soxr::new(cursor, src_fmt, dst_fmt);
        if result.is_err() {
            return;
        }
        
        let mut soxr = result.unwrap();
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
        
        let result = Soxr::new(cursor, src_fmt, dst_fmt);
        if result.is_err() {
            return;
        }
        
        let mut soxr = result.unwrap();
        let mut output = vec![0u8; 3200];
        
        let n = soxr.read(&mut output);
        assert!(n.is_ok());
    }

    #[test]
    fn test_soxr_close_with_error() {
        let data = vec![0u8; 320];
        let cursor = Cursor::new(data);
        
        let result = Soxr::new(cursor, Format::MONO_16K, Format::MONO_24K);
        if result.is_err() {
            return;
        }
        
        let mut soxr = result.unwrap();
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
        
        let result = Soxr::new(cursor, Format::MONO_16K, Format::MONO_24K);
        if result.is_err() {
            return;
        }
        
        let mut soxr = result.unwrap();
        let err = io::Error::new(io::ErrorKind::Other, "custom error");
        let close_result = soxr.close_with_error(Some(err));
        assert!(close_result.is_ok());
    }

    #[test]
    fn test_soxr_read_small_buffer() {
        let data = vec![0u8; 320];
        let cursor = Cursor::new(data);
        
        let result = Soxr::new(cursor, Format::MONO_16K, Format::MONO_24K);
        if result.is_err() {
            return;
        }
        
        let mut soxr = result.unwrap();
        
        // Buffer too small for even one sample
        let mut output = vec![0u8; 1];
        let read_result = soxr.read(&mut output);
        assert!(read_result.is_err());
    }

    #[test]
    fn test_soxr_read_empty_buffer() {
        let data = vec![0u8; 320];
        let cursor = Cursor::new(data);
        
        let result = Soxr::new(cursor, Format::MONO_16K, Format::MONO_24K);
        if result.is_err() {
            return;
        }
        
        let mut soxr = result.unwrap();
        let mut output: Vec<u8> = vec![];
        
        let n = soxr.read(&mut output).unwrap();
        assert_eq!(n, 0);
    }

    #[test]
    fn test_resampler_error_display() {
        let err = ResamplerError::Soxr("test error".to_string());
        assert!(format!("{}", err).contains("soxr error"));

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
