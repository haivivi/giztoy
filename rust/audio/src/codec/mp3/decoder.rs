//! MP3 decoder using minimp3.

use std::io::{self, Read};
use super::ffi::{Mp3Dec, Mp3FrameInfo, MINIMP3_MAX_SAMPLES_PER_FRAME};

/// MP3 decoder error.
#[derive(Debug)]
pub enum DecoderError {
    /// Failed to create decoder.
    CreateFailed,
    /// Decoder is closed.
    Closed,
    /// I/O error.
    Io(io::Error),
}

impl std::fmt::Display for DecoderError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::CreateFailed => write!(f, "mp3: failed to create decoder"),
            Self::Closed => write!(f, "mp3: decoder is closed"),
            Self::Io(e) => write!(f, "mp3: io error: {}", e),
        }
    }
}

impl std::error::Error for DecoderError {}

impl From<io::Error> for DecoderError {
    fn from(e: io::Error) -> Self {
        Self::Io(e)
    }
}

/// MP3 decoder.
pub struct Mp3Decoder<R: Read> {
    reader: R,
    dec: Mp3Dec,
    info: Mp3FrameInfo,
    pcm: Vec<i16>,
    pcm_pos: usize,
    pcm_len: usize,
    input_buf: Vec<u8>,
    input_pos: usize,
    input_len: usize,
    closed: bool,
}

impl<R: Read> Mp3Decoder<R> {
    /// Creates a new MP3 decoder.
    pub fn new(reader: R) -> Self {
        Self {
            reader,
            dec: Mp3Dec::new(),
            info: Mp3FrameInfo {
                frame_bytes: 0,
                frame_offset: 0,
                channels: 0,
                hz: 0,
                layer: 0,
                bitrate_kbps: 0,
            },
            pcm: vec![0i16; MINIMP3_MAX_SAMPLES_PER_FRAME],
            pcm_pos: 0,
            pcm_len: 0,
            input_buf: vec![0u8; 16 * 1024],
            input_pos: 0,
            input_len: 0,
            closed: false,
        }
    }

    /// Returns the sample rate (0 if not yet determined).
    pub fn sample_rate(&self) -> i32 {
        self.info.hz
    }

    /// Returns the number of channels (0 if not yet determined).
    pub fn channels(&self) -> i32 {
        self.info.channels
    }

    /// Returns the bitrate in kbps of the last decoded frame.
    pub fn bitrate(&self) -> i32 {
        self.info.bitrate_kbps
    }

    /// Closes the decoder.
    pub fn close(&mut self) {
        self.closed = true;
    }

    /// Refills the input buffer from the reader.
    fn refill_input(&mut self) -> io::Result<usize> {
        self.input_pos = 0;
        self.input_len = self.reader.read(&mut self.input_buf)?;
        Ok(self.input_len)
    }

    /// Shifts remaining input data to the front and reads more.
    fn shift_and_refill(&mut self) -> io::Result<usize> {
        let remaining = self.input_len - self.input_pos;
        if remaining > 0 && self.input_pos > 0 {
            self.input_buf.copy_within(self.input_pos..self.input_len, 0);
        }
        self.input_len = remaining;
        self.input_pos = 0;

        let n = self.reader.read(&mut self.input_buf[self.input_len..])?;
        self.input_len += n;
        Ok(n)
    }

    /// Decodes the next frame.
    fn decode_frame(&mut self) -> i32 {
        let samples = unsafe {
            super::ffi::mp3dec_decode_frame(
                &mut self.dec,
                self.input_buf[self.input_pos..self.input_len].as_ptr(),
                (self.input_len - self.input_pos) as i32,
                self.pcm.as_mut_ptr(),
                &mut self.info,
            )
        };

        self.input_pos += self.info.frame_bytes as usize;

        if samples > 0 {
            self.pcm_pos = 0;
            self.pcm_len = samples as usize;
        }

        samples
    }

    /// Copies decoded PCM data to output buffer.
    fn copy_pcm(&mut self, buf: &mut [u8], offset: usize) -> usize {
        if self.pcm_pos >= self.pcm_len {
            return 0;
        }

        let channels = if self.info.channels > 0 { self.info.channels } else { 1 };
        let sample_size = (channels as usize) * 2; // bytes per sample
        let available_samples = self.pcm_len - self.pcm_pos;
        let available_bytes = available_samples * sample_size;
        
        // Only copy complete samples to avoid alignment issues and infinite loops
        let max_bytes = buf.len() - offset;
        let to_copy = max_bytes.min(available_bytes);
        let samples_to_copy = to_copy / sample_size;
        
        if samples_to_copy == 0 {
            return 0;
        }
        
        let bytes_to_copy = samples_to_copy * sample_size;

        // Copy PCM data as bytes
        let pcm_byte_offset = self.pcm_pos * sample_size;
        let pcm_bytes: &[u8] = unsafe {
            std::slice::from_raw_parts(
                self.pcm.as_ptr() as *const u8,
                self.pcm.len() * 2,
            )
        };

        buf[offset..offset + bytes_to_copy]
            .copy_from_slice(&pcm_bytes[pcm_byte_offset..pcm_byte_offset + bytes_to_copy]);

        self.pcm_pos += samples_to_copy;
        bytes_to_copy
    }
}

impl<R: Read> Read for Mp3Decoder<R> {
    fn read(&mut self, buf: &mut [u8]) -> io::Result<usize> {
        if self.closed {
            return Err(io::Error::new(io::ErrorKind::Other, "decoder is closed"));
        }

        let mut total_read = 0;

        while total_read < buf.len() {
            // Try to copy from decoded PCM buffer
            let copied = self.copy_pcm(buf, total_read);
            if copied > 0 {
                total_read += copied;
                continue;
            }

            // Need to decode more frames
            // Ensure we have input data
            if self.input_pos >= self.input_len {
                let n = self.refill_input()?;
                if n == 0 {
                    return Ok(total_read);
                }
            }

            // Decode next frame
            let samples = self.decode_frame();

            if samples <= 0 && self.info.frame_bytes == 0 {
                // Need more data
                let n = self.shift_and_refill()?;
                if n == 0 {
                    return Ok(total_read);
                }
            }
        }

        Ok(total_read)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Cursor;

    #[test]
    fn test_decoder_new() {
        let data = Vec::<u8>::new();
        let cursor = Cursor::new(data);
        let decoder = Mp3Decoder::new(cursor);
        
        assert_eq!(decoder.sample_rate(), 0); // Not yet determined
        assert_eq!(decoder.channels(), 0);
        assert_eq!(decoder.bitrate(), 0);
    }

    #[test]
    fn test_decoder_close() {
        let data = Vec::<u8>::new();
        let cursor = Cursor::new(data);
        let mut decoder = Mp3Decoder::new(cursor);
        
        decoder.close();
        assert!(decoder.closed);
    }

    #[test]
    fn test_decoder_read_after_close() {
        let data = Vec::<u8>::new();
        let cursor = Cursor::new(data);
        let mut decoder = Mp3Decoder::new(cursor);
        
        decoder.close();
        
        let mut buf = [0u8; 100];
        let result = decoder.read(&mut buf);
        assert!(result.is_err());
    }

    #[test]
    fn test_decoder_read_empty() {
        let data = Vec::<u8>::new();
        let cursor = Cursor::new(data);
        let mut decoder = Mp3Decoder::new(cursor);
        
        let mut buf = [0u8; 100];
        let n = decoder.read(&mut buf).unwrap();
        assert_eq!(n, 0);
    }

    #[test]
    fn test_decoder_error_display() {
        let err = DecoderError::CreateFailed;
        assert!(format!("{}", err).contains("create"));

        let err = DecoderError::Closed;
        assert!(format!("{}", err).contains("closed"));

        let err = DecoderError::Io(io::Error::new(io::ErrorKind::Other, "test"));
        assert!(format!("{}", err).contains("io error"));
    }

    #[test]
    fn test_decoder_error_from_io() {
        let io_err = io::Error::new(io::ErrorKind::Other, "test");
        let dec_err: DecoderError = io_err.into();
        assert!(matches!(dec_err, DecoderError::Io(_)));
    }

    #[test]
    fn test_decoder_initial_state() {
        let data = Vec::<u8>::new();
        let cursor = Cursor::new(data);
        let decoder = Mp3Decoder::new(cursor);
        
        assert!(!decoder.closed);
        assert_eq!(decoder.pcm_pos, 0);
        assert_eq!(decoder.pcm_len, 0);
        assert_eq!(decoder.input_pos, 0);
        assert_eq!(decoder.input_len, 0);
    }

    #[test]
    fn test_decoder_accessors() {
        let data = Vec::<u8>::new();
        let cursor = Cursor::new(data);
        let mut decoder = Mp3Decoder::new(cursor);
        
        // Before decoding, all info is 0
        assert_eq!(decoder.sample_rate(), 0);
        assert_eq!(decoder.channels(), 0);
        assert_eq!(decoder.bitrate(), 0);
        
        // Close and verify
        decoder.close();
        assert!(decoder.closed);
    }
}
