//! MP3 encoder using LAME.

use std::io::{self, Write};
use std::ptr;
use std::sync::Mutex;
use super::ffi::{self, LameGlobalFlags};

/// Quality presets for VBR encoding.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Quality {
    /// Best quality (~245 kbps)
    Best = 0,
    /// High quality (~190 kbps)
    High = 2,
    /// Medium quality (~130 kbps)
    Medium = 5,
    /// Low quality (~100 kbps)
    Low = 7,
    /// Worst quality (~65 kbps)
    Worst = 9,
}

/// MP3 encoder options.
#[derive(Debug, Clone)]
pub struct EncoderOptions {
    /// VBR quality (0 = best, 9 = worst).
    pub quality: Quality,
    /// Constant bitrate in kbps. If set, VBR is disabled.
    pub bitrate: Option<i32>,
}

impl Default for EncoderOptions {
    fn default() -> Self {
        Self {
            quality: Quality::Medium,
            bitrate: None,
        }
    }
}

impl EncoderOptions {
    /// Sets VBR quality.
    pub fn with_quality(mut self, quality: Quality) -> Self {
        self.quality = quality;
        self
    }

    /// Sets constant bitrate mode (in kbps).
    pub fn with_bitrate(mut self, kbps: i32) -> Self {
        self.bitrate = Some(kbps);
        self
    }
}

/// MP3 encoder error.
#[derive(Debug)]
pub enum EncoderError {
    /// Invalid parameters.
    InvalidParams(String),
    /// LAME initialization failed.
    InitFailed,
    /// Encoding failed.
    EncodeFailed,
    /// Encoder is closed.
    Closed,
    /// I/O error.
    Io(io::Error),
    /// Input data is not properly aligned.
    AlignmentError,
}

impl std::fmt::Display for EncoderError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::InvalidParams(msg) => write!(f, "mp3: invalid params: {}", msg),
            Self::InitFailed => write!(f, "mp3: failed to initialize LAME"),
            Self::EncodeFailed => write!(f, "mp3: encode failed"),
            Self::Closed => write!(f, "mp3: encoder is closed"),
            Self::Io(e) => write!(f, "mp3: io error: {}", e),
            Self::AlignmentError => write!(f, "mp3: input data is not 2-byte aligned for i16 samples"),
        }
    }
}

impl std::error::Error for EncoderError {}

impl From<io::Error> for EncoderError {
    fn from(e: io::Error) -> Self {
        Self::Io(e)
    }
}

/// MP3 encoder.
pub struct Mp3Encoder<W: Write> {
    inner: Mutex<EncoderInner<W>>,
}

struct EncoderInner<W: Write> {
    writer: W,
    sample_rate: i32,
    channels: i32,
    options: EncoderOptions,
    lame: *mut LameGlobalFlags,
    mp3buf: Vec<u8>,
    inited: bool,
    closed: bool,
}

// Safety: The LAME handle is protected by a mutex.
unsafe impl<W: Write + Send> Send for Mp3Encoder<W> {}
unsafe impl<W: Write + Send> Sync for Mp3Encoder<W> {}

impl<W: Write> Drop for Mp3Encoder<W> {
    fn drop(&mut self) {
        let inner = self.inner.get_mut().unwrap();
        if !inner.lame.is_null() {
            unsafe { ffi::lame_close(inner.lame) };
            inner.lame = ptr::null_mut();
        }
    }
}

impl<W: Write> Mp3Encoder<W> {
    /// Creates a new MP3 encoder.
    pub fn new(
        writer: W,
        sample_rate: i32,
        channels: i32,
        options: EncoderOptions,
    ) -> Result<Self, EncoderError> {
        if channels != 1 && channels != 2 {
            return Err(EncoderError::InvalidParams(
                "channels must be 1 or 2".to_string(),
            ));
        }

        Ok(Self {
            inner: Mutex::new(EncoderInner {
                writer,
                sample_rate,
                channels,
                options,
                lame: ptr::null_mut(),
                mp3buf: vec![0u8; 8192],
                inited: false,
                closed: false,
            }),
        })
    }

    /// Initializes LAME encoder (called on first write).
    fn init(inner: &mut EncoderInner<W>) -> Result<(), EncoderError> {
        if inner.inited {
            return Ok(());
        }

        let lame = unsafe { ffi::lame_init() };
        if lame.is_null() {
            return Err(EncoderError::InitFailed);
        }

        unsafe {
            ffi::lame_set_in_samplerate(lame, inner.sample_rate);
            ffi::lame_set_num_channels(lame, inner.channels);

            if inner.channels == 1 {
                ffi::lame_set_mode(lame, ffi::MONO);
            } else {
                ffi::lame_set_mode(lame, ffi::JOINT_STEREO);
            }

            if let Some(bitrate) = inner.options.bitrate {
                ffi::lame_set_VBR(lame, ffi::VBR_OFF);
                ffi::lame_set_brate(lame, bitrate);
            } else {
                ffi::lame_set_VBR(lame, ffi::VBR_DEFAULT);
                ffi::lame_set_VBR_quality(lame, inner.options.quality as i32 as f32);
            }

            if ffi::lame_init_params(lame) < 0 {
                ffi::lame_close(lame);
                return Err(EncoderError::InitFailed);
            }
        }

        inner.lame = lame;
        inner.inited = true;
        Ok(())
    }

    /// Writes PCM samples to the encoder.
    /// Input should be interleaved i16 samples (little-endian bytes).
    pub fn write(&mut self, pcm: &[u8]) -> Result<(), EncoderError> {
        let mut inner = self.inner.lock().unwrap();

        if inner.closed {
            return Err(EncoderError::Closed);
        }

        Self::init(&mut inner)?;

        // Safely reinterpret bytes as i16 samples with alignment check
        let (prefix, samples, suffix) = unsafe { pcm.align_to::<i16>() };
        if !prefix.is_empty() || !suffix.is_empty() {
            return Err(EncoderError::AlignmentError);
        }

        // Calculate samples per channel
        let num_samples = samples.len() / inner.channels as usize;
        if num_samples == 0 {
            return Ok(());
        }

        // Ensure buffer is large enough (LAME recommends 1.25*num_samples + 7200)
        let required_size = num_samples * 5 / 4 + 7200;
        if inner.mp3buf.len() < required_size {
            inner.mp3buf.resize(required_size, 0);
        }

        let lame = inner.lame;
        let channels = inner.channels;

        let encoded = unsafe {
            if channels == 2 {
                ffi::lame_encode_buffer_interleaved(
                    lame,
                    samples.as_ptr(),
                    num_samples as i32,
                    inner.mp3buf.as_mut_ptr(),
                    inner.mp3buf.len() as i32,
                )
            } else {
                ffi::lame_encode_buffer(
                    lame,
                    samples.as_ptr(),
                    ptr::null(),
                    num_samples as i32,
                    inner.mp3buf.as_mut_ptr(),
                    inner.mp3buf.len() as i32,
                )
            }
        };

        if encoded < 0 {
            return Err(EncoderError::EncodeFailed);
        }

        if encoded > 0 {
            // Split borrow: get slice reference before borrowing writer
            let EncoderInner { ref mp3buf, ref mut writer, .. } = *inner;
            writer.write_all(&mp3buf[..encoded as usize])?;
        }

        Ok(())
    }

    /// Flushes remaining encoded data.
    pub fn flush(&mut self) -> Result<(), EncoderError> {
        let mut inner = self.inner.lock().unwrap();

        if !inner.inited || inner.closed {
            return Ok(());
        }

        let lame = inner.lame;

        let encoded = unsafe {
            ffi::lame_encode_flush(
                lame,
                inner.mp3buf.as_mut_ptr(),
                inner.mp3buf.len() as i32,
            )
        };

        if encoded > 0 {
            // Split borrow: get slice reference before borrowing writer
            let EncoderInner { ref mp3buf, ref mut writer, .. } = *inner;
            writer.write_all(&mp3buf[..encoded as usize])?;
        }

        Ok(())
    }

    /// Closes the encoder, flushing any remaining buffered frames first.
    ///
    /// Flush and close are done under a single mutex acquisition to avoid
    /// any gap between the two operations.
    pub fn close(&mut self) -> Result<(), EncoderError> {
        let mut inner = self.inner.lock().unwrap();

        if inner.closed {
            return Ok(());
        }

        // Flush remaining buffered frames before closing.
        if inner.inited && !inner.lame.is_null() {
            let encoded = unsafe {
                ffi::lame_encode_flush(
                    inner.lame,
                    inner.mp3buf.as_mut_ptr(),
                    inner.mp3buf.len() as i32,
                )
            };
            if encoded > 0 {
                let flush_data = inner.mp3buf[..encoded as usize].to_vec();
                inner
                    .writer
                    .write_all(&flush_data)
                    .map_err(EncoderError::Io)?;
            }
        }

        inner.closed = true;

        if !inner.lame.is_null() {
            unsafe { ffi::lame_close(inner.lame) };
            inner.lame = ptr::null_mut();
        }

        Ok(())
    }
}

impl<W: Write> Write for Mp3Encoder<W> {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        Mp3Encoder::write(self, buf).map_err(|e| io::Error::new(io::ErrorKind::Other, e))?;
        Ok(buf.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        Mp3Encoder::flush(self).map_err(|e| io::Error::new(io::ErrorKind::Other, e))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_encoder_options_default() {
        let opts = EncoderOptions::default();
        assert_eq!(opts.quality, Quality::Medium);
        assert!(opts.bitrate.is_none());
    }

    #[test]
    fn test_encoder_options_with_quality() {
        let opts = EncoderOptions::default().with_quality(Quality::Best);
        assert_eq!(opts.quality, Quality::Best);
    }

    #[test]
    fn test_encoder_options_with_bitrate() {
        let opts = EncoderOptions::default().with_bitrate(128);
        assert_eq!(opts.bitrate, Some(128));
    }

    #[test]
    fn test_quality_values() {
        assert_eq!(Quality::Best as i32, 0);
        assert_eq!(Quality::High as i32, 2);
        assert_eq!(Quality::Medium as i32, 5);
        assert_eq!(Quality::Low as i32, 7);
        assert_eq!(Quality::Worst as i32, 9);
    }

    #[test]
    fn test_encoder_new_mono() {
        let buf = Vec::new();
        let encoder = Mp3Encoder::new(buf, 44100, 1, EncoderOptions::default());
        assert!(encoder.is_ok());
    }

    #[test]
    fn test_encoder_new_stereo() {
        let buf = Vec::new();
        let encoder = Mp3Encoder::new(buf, 44100, 2, EncoderOptions::default());
        assert!(encoder.is_ok());
    }

    #[test]
    fn test_encoder_new_invalid_channels() {
        let buf = Vec::new();
        let encoder = Mp3Encoder::new(buf, 44100, 3, EncoderOptions::default());
        assert!(encoder.is_err());
    }

    #[test]
    fn test_encoder_write_mono() {
        let buf = Vec::new();
        let mut encoder = Mp3Encoder::new(buf, 44100, 1, EncoderOptions::default()).unwrap();
        
        // 1152 samples (standard MP3 frame size) * 2 bytes = 2304 bytes
        let pcm = vec![0u8; 2304];
        let result = encoder.write(&pcm);
        assert!(result.is_ok());
    }

    #[test]
    fn test_encoder_write_stereo() {
        let buf = Vec::new();
        let mut encoder = Mp3Encoder::new(buf, 44100, 2, EncoderOptions::default()).unwrap();
        
        // 1152 samples * 2 channels * 2 bytes = 4608 bytes
        let pcm = vec![0u8; 4608];
        let result = encoder.write(&pcm);
        assert!(result.is_ok());
    }

    #[test]
    fn test_encoder_flush() {
        let buf = Vec::new();
        let mut encoder = Mp3Encoder::new(buf, 44100, 1, EncoderOptions::default()).unwrap();
        
        let pcm = vec![0u8; 2304];
        encoder.write(&pcm).unwrap();
        
        let result = encoder.flush();
        assert!(result.is_ok());
    }

    #[test]
    fn test_encoder_close() {
        let buf = Vec::new();
        let mut encoder = Mp3Encoder::new(buf, 44100, 1, EncoderOptions::default()).unwrap();
        
        let result = encoder.close();
        assert!(result.is_ok());
    }

    #[test]
    fn test_encoder_close_twice() {
        let buf = Vec::new();
        let mut encoder = Mp3Encoder::new(buf, 44100, 1, EncoderOptions::default()).unwrap();
        
        encoder.close().unwrap();
        let result = encoder.close();
        assert!(result.is_ok());
    }

    #[test]
    fn test_encoder_write_after_close() {
        let buf = Vec::new();
        let mut encoder = Mp3Encoder::new(buf, 44100, 1, EncoderOptions::default()).unwrap();
        
        encoder.close().unwrap();
        
        let pcm = vec![0u8; 2304];
        let result = encoder.write(&pcm);
        assert!(result.is_err());
    }

    #[test]
    fn test_encoder_with_cbr() {
        let buf = Vec::new();
        let opts = EncoderOptions::default().with_bitrate(128);
        let mut encoder = Mp3Encoder::new(buf, 44100, 2, opts).unwrap();
        
        let pcm = vec![0u8; 4608];
        let result = encoder.write(&pcm);
        assert!(result.is_ok());
    }

    #[test]
    fn test_encoder_empty_write() {
        let buf = Vec::new();
        let mut encoder = Mp3Encoder::new(buf, 44100, 1, EncoderOptions::default()).unwrap();
        
        let pcm: Vec<u8> = vec![];
        let result = encoder.write(&pcm);
        assert!(result.is_ok());
    }

    #[test]
    fn test_encoder_multiple_writes() {
        let buf = Vec::new();
        let mut encoder = Mp3Encoder::new(buf, 44100, 1, EncoderOptions::default()).unwrap();
        
        let pcm = vec![0u8; 2304];
        for _ in 0..10 {
            encoder.write(&pcm).unwrap();
        }
        
        encoder.flush().unwrap();
    }

    #[test]
    fn test_encoder_io_write_trait() {
        use std::io::Write;
        
        let buf = Vec::new();
        let mut encoder = Mp3Encoder::new(buf, 44100, 1, EncoderOptions::default()).unwrap();
        
        let pcm = vec![0u8; 2304];
        let n = Write::write(&mut encoder, &pcm).unwrap();
        assert_eq!(n, pcm.len());
        
        Write::flush(&mut encoder).unwrap();
    }

    #[test]
    fn test_encoder_error_display() {
        let err = EncoderError::InvalidParams("test".to_string());
        assert!(format!("{}", err).contains("invalid params"));

        let err = EncoderError::InitFailed;
        assert!(format!("{}", err).contains("initialize"));

        let err = EncoderError::EncodeFailed;
        assert!(format!("{}", err).contains("encode failed"));

        let err = EncoderError::Closed;
        assert!(format!("{}", err).contains("closed"));

        let err = EncoderError::Io(io::Error::new(io::ErrorKind::Other, "test"));
        assert!(format!("{}", err).contains("io error"));
    }

    #[test]
    fn test_encoder_error_from_io() {
        let io_err = io::Error::new(io::ErrorKind::Other, "test");
        let enc_err: EncoderError = io_err.into();
        assert!(matches!(enc_err, EncoderError::Io(_)));
    }
}
