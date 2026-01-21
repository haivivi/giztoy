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

    /// Generate a sine wave as i16 PCM samples.
    fn generate_sine_wave(sample_rate: i32, frequency: f64, duration_ms: u32) -> Vec<i16> {
        let num_samples = (sample_rate as u64 * duration_ms as u64 / 1000) as usize;
        let mut samples = Vec::with_capacity(num_samples);
        
        for i in 0..num_samples {
            let t = i as f64 / sample_rate as f64;
            let value = (2.0 * std::f64::consts::PI * frequency * t).sin();
            // Use 0.8 amplitude to avoid clipping
            samples.push((value * 0.8 * i16::MAX as f64) as i16);
        }
        
        samples
    }

    /// Convert i16 samples to bytes (little-endian).
    fn samples_to_bytes(samples: &[i16]) -> Vec<u8> {
        let mut bytes = Vec::with_capacity(samples.len() * 2);
        for &sample in samples {
            bytes.extend_from_slice(&sample.to_le_bytes());
        }
        bytes
    }

    /// Convert bytes to i16 samples (little-endian).
    fn bytes_to_samples(bytes: &[u8]) -> Vec<i16> {
        bytes.chunks_exact(2)
            .map(|chunk| i16::from_le_bytes([chunk[0], chunk[1]]))
            .collect()
    }

    /// Calculate RMS (Root Mean Square) energy of samples.
    fn calculate_rms(samples: &[i16]) -> f64 {
        if samples.is_empty() {
            return 0.0;
        }
        let sum: f64 = samples.iter().map(|&s| (s as f64).powi(2)).sum();
        (sum / samples.len() as f64).sqrt()
    }

    /// Find the dominant frequency using zero-crossing rate.
    /// Returns approximate frequency in Hz.
    fn estimate_frequency(samples: &[i16], sample_rate: i32) -> f64 {
        if samples.len() < 4 {
            return 0.0;
        }

        let mut zero_crossings = 0;
        for i in 1..samples.len() {
            if (samples[i-1] >= 0 && samples[i] < 0) || (samples[i-1] < 0 && samples[i] >= 0) {
                zero_crossings += 1;
            }
        }

        // Each complete cycle has 2 zero crossings
        let duration_sec = samples.len() as f64 / sample_rate as f64;
        zero_crossings as f64 / (2.0 * duration_sec)
    }

    /// Find best alignment offset using cross-correlation on a subset.
    fn find_best_offset(original: &[i16], decoded: &[i16], max_offset: usize) -> usize {
        let chunk_size = 2000.min(original.len() / 2);
        if chunk_size < 100 || decoded.len() < chunk_size + max_offset {
            return 0;
        }

        let orig_chunk = &original[..chunk_size];
        let mut best_offset = 0;
        let mut best_corr = f64::NEG_INFINITY;

        for offset in 0..max_offset.min(decoded.len() - chunk_size) {
            let dec_chunk = &decoded[offset..offset + chunk_size];
            
            // Calculate dot product as correlation proxy
            let mut dot: f64 = 0.0;
            for i in 0..chunk_size {
                dot += orig_chunk[i] as f64 * dec_chunk[i] as f64;
            }
            
            if dot > best_corr {
                best_corr = dot;
                best_offset = offset;
            }
        }

        best_offset
    }

    #[test]
    fn test_encode_decode_roundtrip() {
        use super::super::encoder::{Mp3Encoder, EncoderOptions, Quality};

        let sample_rate = 44100;
        let channels = 1;
        let frequency = 440.0; // A4 note
        let duration_ms = 500; // 500ms of audio

        // Step 1: Generate sine wave PCM
        let original_samples = generate_sine_wave(sample_rate, frequency, duration_ms);
        let pcm_bytes = samples_to_bytes(&original_samples);
        
        // Step 2: Encode to MP3
        let mut mp3_data = Vec::new();
        {
            let mut encoder = Mp3Encoder::new(
                &mut mp3_data,
                sample_rate,
                channels,
                EncoderOptions::default().with_quality(Quality::High),
            ).expect("Failed to create encoder");
            
            encoder.write(&pcm_bytes).expect("Failed to encode");
            encoder.flush().expect("Failed to flush");
        }

        // Verify MP3 data was generated
        assert!(!mp3_data.is_empty(), "MP3 data should not be empty");
        assert!(mp3_data.len() < pcm_bytes.len(), "MP3 should be smaller than raw PCM");

        // Step 3: Decode MP3 back to PCM
        let mut decoder = Mp3Decoder::new(Cursor::new(mp3_data));
        let mut decoded_bytes = vec![0u8; pcm_bytes.len() * 2]; // Extra space for padding
        let mut total_decoded = 0;

        loop {
            let n = decoder.read(&mut decoded_bytes[total_decoded..]).expect("Failed to decode");
            if n == 0 {
                break;
            }
            total_decoded += n;
        }

        decoded_bytes.truncate(total_decoded);
        let decoded_samples = bytes_to_samples(&decoded_bytes);

        // Step 4: Verify decoded audio properties
        assert_eq!(decoder.sample_rate(), sample_rate, "Sample rate should match");
        assert_eq!(decoder.channels(), channels, "Channels should match");

        // Decoded length should be similar (MP3 adds padding, typically up to 2304 samples)
        let length_ratio = decoded_samples.len() as f64 / original_samples.len() as f64;
        assert!(
            length_ratio > 0.8 && length_ratio < 1.3,
            "Decoded length ratio {} is out of expected range (0.8-1.3)",
            length_ratio
        );

        // Step 5: Verify audio content quality
        
        // 5a: Decoded audio should have significant energy (not silence)
        let original_rms = calculate_rms(&original_samples);
        let decoded_rms = calculate_rms(&decoded_samples);
        
        assert!(decoded_rms > 0.0, "Decoded audio should not be silent");
        
        // RMS should be within 50% of original (lossy compression affects amplitude)
        let rms_ratio = decoded_rms / original_rms;
        assert!(
            rms_ratio > 0.5 && rms_ratio < 2.0,
            "RMS ratio {} is out of expected range (0.5-2.0)",
            rms_ratio
        );

        // 5b: Verify the dominant frequency is preserved
        // Find best alignment to account for encoder delay
        let offset = find_best_offset(&original_samples, &decoded_samples, 3000);
        
        // Skip encoder delay and use middle portion
        let analyze_start = offset + 1000;
        let analyze_len = 8000.min(decoded_samples.len().saturating_sub(analyze_start + 1000));
        
        if analyze_len > 2000 {
            let orig_freq = estimate_frequency(
                &original_samples[1000..1000 + analyze_len],
                sample_rate
            );
            let dec_freq = estimate_frequency(
                &decoded_samples[analyze_start..analyze_start + analyze_len],
                sample_rate
            );

            // Frequency should be preserved within 10%
            let freq_ratio = dec_freq / orig_freq;
            assert!(
                freq_ratio > 0.9 && freq_ratio < 1.1,
                "Frequency {} Hz differs too much from original {} Hz (ratio: {})",
                dec_freq, orig_freq, freq_ratio
            );
        }
    }

    #[test]
    fn test_encode_decode_stereo() {
        use super::super::encoder::{Mp3Encoder, EncoderOptions, Quality};

        let sample_rate = 44100;
        let channels = 2;
        let duration_ms = 200;

        // Generate stereo sine wave (different frequencies for L/R)
        let num_samples = (sample_rate as u64 * duration_ms as u64 / 1000) as usize;
        let mut stereo_samples = Vec::with_capacity(num_samples * 2);
        
        for i in 0..num_samples {
            let t = i as f64 / sample_rate as f64;
            // Left channel: 440 Hz
            let left = ((2.0 * std::f64::consts::PI * 440.0 * t).sin() * 0.7 * i16::MAX as f64) as i16;
            // Right channel: 880 Hz
            let right = ((2.0 * std::f64::consts::PI * 880.0 * t).sin() * 0.7 * i16::MAX as f64) as i16;
            stereo_samples.push(left);
            stereo_samples.push(right);
        }

        let pcm_bytes = samples_to_bytes(&stereo_samples);

        // Encode
        let mut mp3_data = Vec::new();
        {
            let mut encoder = Mp3Encoder::new(
                &mut mp3_data,
                sample_rate,
                channels,
                EncoderOptions::default().with_quality(Quality::Medium),
            ).expect("Failed to create encoder");
            
            encoder.write(&pcm_bytes).expect("Failed to encode");
            encoder.flush().expect("Failed to flush");
        }

        // Decode
        let mut decoder = Mp3Decoder::new(Cursor::new(mp3_data));
        let mut decoded_bytes = vec![0u8; pcm_bytes.len() * 2];
        let mut total = 0;

        loop {
            let n = decoder.read(&mut decoded_bytes[total..]).expect("Decode failed");
            if n == 0 { break; }
            total += n;
        }

        // Verify stereo
        assert_eq!(decoder.channels(), 2, "Should decode as stereo");
        assert!(total > 0, "Should have decoded some audio");
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
