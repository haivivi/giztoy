//! Track system for the mixer with independent ring buffers and read-side resampling.
//!
//! Each track has a queue of `TrackInput`s, each with its own ring buffer
//! and optional `SincFixedOut` resampler. The ring buffer stores raw input-format
//! data; when the mixer reads, data is pulled through the resampler (if present)
//! to produce output-format samples on demand (lazy read-side resample).
//!
//! This matches the Go implementation in `go/pkg/audio/pcm/track.go`.

use super::format::Format;
use rubato::{Resampler, SincFixedOut, SincInterpolationParameters, SincInterpolationType, WindowFunction};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Condvar, Mutex};

// ============================================================================
// TrackRingBuf — per-input circular buffer
// ============================================================================

/// Thread-safe circular buffer for audio data.
///
/// - Write side: blocks if buffer is full (waits for read_notify).
/// - Read side: non-blocking, returns 0 if empty. Output buffer is
///   pre-zeroed by caller for zero-fill semantics.
pub(crate) struct TrackRingBuf {
    notify_mixer: Arc<Condvar>,
    read_notify: Condvar,

    mu: Mutex<RingBufInner>,
}

struct RingBufInner {
    buf: Vec<u8>,
    head: usize,
    tail: usize,
    close_write: bool,
    close_err: Option<String>,
}

impl TrackRingBuf {
    pub fn new(capacity: usize, notify_mixer: Arc<Condvar>) -> Self {
        Self {
            notify_mixer,
            read_notify: Condvar::new(),
            mu: Mutex::new(RingBufInner {
                buf: vec![0u8; capacity],
                head: 0,
                tail: 0,
                close_write: false,
                close_err: None,
            }),
        }
    }

    /// Writes data into the ring buffer. Blocks if full.
    pub fn write(&self, mut data: &[u8]) -> Result<usize, String> {
        let total = data.len();
        let mut inner = self.mu.lock().unwrap();

        while !data.is_empty() {
            if let Some(ref e) = inner.close_err {
                return Err(e.clone());
            }
            if inner.close_write {
                return Err("write closed".to_string());
            }

            let cap = inner.buf.len();
            let used = inner.tail - inner.head;
            let free = cap - used;

            if free == 0 {
                // Buffer full — drop lock and wait for reader to consume
                inner = self.read_notify.wait(inner).unwrap();
                continue;
            }

            let n = data.len().min(free);
            let write_pos = inner.tail % cap;
            let first_chunk = n.min(cap - write_pos);
            inner.buf[write_pos..write_pos + first_chunk].copy_from_slice(&data[..first_chunk]);
            if first_chunk < n {
                inner.buf[..n - first_chunk].copy_from_slice(&data[first_chunk..n]);
            }
            inner.tail += n;
            data = &data[n..];

            // Notify mixer that data is available
            self.notify_mixer.notify_all();
        }

        Ok(total)
    }

    /// Non-blocking read. Reads up to `buf.len()` bytes.
    /// Returns 0 if no data available (caller should zero-fill).
    /// The output buffer should be pre-zeroed by the caller.
    pub fn read(&self, buf: &mut [u8]) -> ReadResult {
        let mut inner = self.mu.lock().unwrap();

        if let Some(ref e) = inner.close_err {
            return ReadResult::Error(e.clone());
        }

        let available = inner.tail - inner.head;
        if available == 0 {
            if inner.close_write {
                return ReadResult::Eof;
            }
            return ReadResult::Empty;
        }

        let cap = inner.buf.len();
        let n = buf.len().min(available);
        let read_pos = inner.head % cap;
        let first_chunk = n.min(cap - read_pos);
        buf[..first_chunk].copy_from_slice(&inner.buf[read_pos..read_pos + first_chunk]);
        if first_chunk < n {
            buf[first_chunk..n].copy_from_slice(&inner.buf[..n - first_chunk]);
        }
        inner.head += n;

        // Notify writer that space is available
        if !inner.close_write {
            self.read_notify.notify_one();
        }

        ReadResult::Data(n)
    }

    pub fn close_write(&self) {
        let mut inner = self.mu.lock().unwrap();
        if !inner.close_write {
            inner.close_write = true;
            self.read_notify.notify_all(); // Wake blocked writers
            self.notify_mixer.notify_all();
        }
    }

    pub fn close_with_error(&self, err: String) {
        let mut inner = self.mu.lock().unwrap();
        inner.close_err = Some(err);
        inner.close_write = true;
        self.read_notify.notify_all();
        self.notify_mixer.notify_all();
    }

}

pub(crate) enum ReadResult {
    Data(usize),
    Empty,
    Eof,
    Error(String),
}

// ============================================================================
// TrackInput — a single input writer with its own ring buffer + resampler
// ============================================================================

const SINC_LEN: usize = 128;
const OVERSAMPLING_FACTOR: usize = 128;
const RESAMPLE_CHUNK_SIZE: usize = 480;

fn make_sinc_params() -> SincInterpolationParameters {
    SincInterpolationParameters {
        sinc_len: SINC_LEN,
        f_cutoff: rubato::calculate_cutoff(SINC_LEN, WindowFunction::BlackmanHarris2),
        interpolation: SincInterpolationType::Cubic,
        oversampling_factor: OVERSAMPLING_FACTOR,
        window: WindowFunction::BlackmanHarris2,
    }
}

/// A single audio input source for a track, with its own ring buffer
/// and optional read-side resampler.
pub(crate) struct TrackInput {
    pub format: Format,
    pub output_format: Format,
    pub rb: Arc<TrackRingBuf>,
    resampler: Option<SincFixedOut<f32>>,
    /// Accumulated input frames (f32 per-channel) waiting for the resampler.
    accum: Vec<Vec<f32>>,
    /// Reusable byte buffer for reading raw PCM from ring buffer.
    raw_buf: Vec<u8>,
    /// Whether the ring buffer has reached EOF.
    rb_eof: bool,
    /// Whether the resampler's internal delay has been flushed.
    delay_flushed: bool,
}

impl TrackInput {
    pub fn new(format: Format, output_format: Format, rb: Arc<TrackRingBuf>) -> Result<Self, String> {
        let needs_resample = format.sample_rate != output_format.sample_rate;
        let in_channels = format.channels() as usize;
        let resampler = if needs_resample {
            let ratio = output_format.sample_rate as f64 / format.sample_rate as f64;
            Some(SincFixedOut::<f32>::new(
                ratio,
                2.0,
                make_sinc_params(),
                RESAMPLE_CHUNK_SIZE,
                in_channels,
            ).map_err(|e| format!("failed to create resampler ({}Hz → {}Hz): {}",
                format.sample_rate, output_format.sample_rate, e))?)
        } else {
            None
        };

        Ok(Self {
            format,
            output_format,
            rb,
            resampler,
            accum: vec![Vec::new(); in_channels],
            raw_buf: Vec::new(),
            rb_eof: false,
            delay_flushed: false,
        })
    }

    /// Reads data from this input, resampling if necessary.
    /// The output buffer `buf` must be in output_format (pre-zeroed by caller).
    ///
    /// Returns ReadResult indicating data availability.
    pub fn read(&mut self, buf: &mut [u8]) -> ReadResult {
        if self.resampler.is_some() {
            self.read_resampled(buf)
        } else if self.format.channels() != self.output_format.channels() {
            self.read_channel_convert(buf)
        } else {
            self.rb.read(buf)
        }
    }

    /// Read with sample rate conversion via SincFixedOut.
    ///
    /// Accumulates input frames from the ring buffer until we have enough
    /// for the resampler, then processes and returns output. Never zero-pads
    /// real data — if not enough input is available, returns Empty.
    fn read_resampled(&mut self, buf: &mut [u8]) -> ReadResult {
        let in_channels = self.format.channels() as usize;
        let out_channels = self.output_format.channels() as usize;

        // Pull data from ring buffer into accumulator
        if !self.rb_eof {
            self.pull_from_ringbuf();
        }

        let frames_needed = self.resampler.as_ref().unwrap().input_frames_next();
        let accum_frames = if self.accum.is_empty() { 0 } else { self.accum[0].len() };

        if accum_frames >= frames_needed {
            // Have enough — extract exactly frames_needed and process
            let mut input: Vec<Vec<f32>> = Vec::with_capacity(in_channels);
            for ch in 0..in_channels {
                let rest = self.accum[ch].split_off(frames_needed);
                input.push(std::mem::replace(&mut self.accum[ch], rest));
            }

            let resampler = self.resampler.as_mut().unwrap();
            let output = match resampler.process(&input, None) {
                Ok(out) => out,
                Err(e) => return ReadResult::Error(format!("resampler process: {}", e)),
            };

            let output_frames = if output.is_empty() { 0 } else { output[0].len() };
            if output_frames == 0 {
                return ReadResult::Empty;
            }
            return self.write_output_to_buf(&output, output_frames, out_channels, buf);
        }

        if self.rb_eof {
            if self.delay_flushed {
                return ReadResult::Eof;
            }

            // Not enough frames and ring buffer is done — flush resampler
            if accum_frames > 0 {
                let input = std::mem::take(&mut self.accum);
                let resampler = self.resampler.as_mut().unwrap();
                let output = match resampler.process_partial(Some(&input), None) {
                    Ok(out) => out,
                    Err(_) => {
                        self.delay_flushed = true;
                        return ReadResult::Eof;
                    }
                };
                let output_frames = if output.is_empty() { 0 } else { output[0].len() };
                if output_frames > 0 {
                    return self.write_output_to_buf(&output, output_frames, out_channels, buf);
                }
            }

            // Flush resampler's internal delay (once)
            let resampler = self.resampler.as_mut().unwrap();
            let output = match resampler.process_partial::<Vec<f32>>(None, None) {
                Ok(out) => out,
                Err(_) => {
                    self.delay_flushed = true;
                    return ReadResult::Eof;
                }
            };
            self.delay_flushed = true;
            let output_frames = if output.is_empty() { 0 } else { output[0].len() };
            if output_frames > 0 {
                return match self.write_output_to_buf(&output, output_frames, out_channels, buf) {
                    ReadResult::Data(n) => ReadResult::Data(n),
                    _ => ReadResult::Eof,
                };
            }
            return ReadResult::Eof;
        }

        // Not enough data yet and ring buffer still open
        ReadResult::Empty
    }

    /// Pull available data from ring buffer into the f32 accumulator.
    fn pull_from_ringbuf(&mut self) {
        let in_channels = self.format.channels() as usize;
        let read_size = self.format.bytes_rate() as usize / 10; // ~100ms worth
        if self.raw_buf.len() < read_size {
            self.raw_buf.resize(read_size, 0);
        }

        loop {
            match self.rb.read(&mut self.raw_buf[..read_size]) {
                ReadResult::Data(n) => {
                    let frame_bytes = in_channels * 2;
                    let frames_read = n / frame_bytes;
                    for frame in 0..frames_read {
                        for ch in 0..in_channels {
                            let offset = (frame * in_channels + ch) * 2;
                            let sample = i16::from_le_bytes([
                                self.raw_buf[offset],
                                self.raw_buf[offset + 1],
                            ]);
                            self.accum[ch].push(sample as f32 / 32768.0);
                        }
                    }
                }
                ReadResult::Eof => {
                    self.rb_eof = true;
                    break;
                }
                ReadResult::Empty | ReadResult::Error(_) => {
                    break;
                }
            }
        }
    }

    /// Convert resampler f32 per-channel output to i16 LE bytes in buf.
    /// Handles channel conversion (mono↔stereo) between resampler output
    /// and mixer output format.
    fn write_output_to_buf(
        &self,
        output: &[Vec<f32>],
        output_frames: usize,
        out_channels: usize,
        buf: &mut [u8],
    ) -> ReadResult {
        let resampler_channels = output.len();
        let max_frames = buf.len() / (out_channels * 2);
        let frames_to_write = output_frames.min(max_frames);

        let mut written = 0;
        for frame in 0..frames_to_write {
            for ch in 0..out_channels {
                let sample_f32 = if resampler_channels == 1 && out_channels == 2 {
                    // Mono → stereo: duplicate
                    output[0][frame]
                } else if resampler_channels == 2 && out_channels == 1 {
                    // Stereo → mono: average
                    (output[0][frame] + output[1][frame]) * 0.5
                } else {
                    output[ch.min(resampler_channels - 1)][frame]
                };

                let sample = (sample_f32 * 32767.0).clamp(-32768.0, 32767.0) as i16;
                let offset = written;
                if offset + 1 < buf.len() {
                    let bytes = sample.to_le_bytes();
                    buf[offset] = bytes[0];
                    buf[offset + 1] = bytes[1];
                }
                written += 2;
            }
        }

        if written > 0 {
            ReadResult::Data(written)
        } else {
            ReadResult::Empty
        }
    }

    /// Read with channel conversion only (no sample rate change).
    fn read_channel_convert(&mut self, buf: &mut [u8]) -> ReadResult {
        let in_ch = self.format.channels() as usize;
        let out_ch = self.output_format.channels() as usize;

        if in_ch == 2 && out_ch == 1 {
            // Stereo → mono: read double, downmix
            let stereo_len = buf.len() * 2;
            if self.raw_buf.len() < stereo_len {
                self.raw_buf.resize(stereo_len, 0);
            }
            match self.rb.read(&mut self.raw_buf[..stereo_len]) {
                ReadResult::Data(n) => {
                    let frames = n / 4;
                    for i in 0..frames {
                        let j = i * 4;
                        let l = i16::from_le_bytes([self.raw_buf[j], self.raw_buf[j + 1]]);
                        let r = i16::from_le_bytes([self.raw_buf[j + 2], self.raw_buf[j + 3]]);
                        let m = ((l as i32 + r as i32) / 2) as i16;
                        let bytes = m.to_le_bytes();
                        buf[i * 2] = bytes[0];
                        buf[i * 2 + 1] = bytes[1];
                    }
                    ReadResult::Data(frames * 2)
                }
                other => other,
            }
        } else if in_ch == 1 && out_ch == 2 {
            // Mono → stereo: read half, duplicate
            let mono_len = buf.len() / 2;
            if self.raw_buf.len() < mono_len {
                self.raw_buf.resize(mono_len, 0);
            }
            match self.rb.read(&mut self.raw_buf[..mono_len]) {
                ReadResult::Data(n) => {
                    let samples = n / 2;
                    for i in (0..samples).rev() {
                        let s0 = self.raw_buf[i * 2];
                        let s1 = self.raw_buf[i * 2 + 1];
                        let j = i * 4;
                        buf[j] = s0;
                        buf[j + 1] = s1;
                        buf[j + 2] = s0;
                        buf[j + 3] = s1;
                    }
                    ReadResult::Data(samples * 4)
                }
                other => other,
            }
        } else {
            self.rb.read(buf)
        }
    }

}

// ============================================================================
// InternalTrack — the track as seen by the mixer (read side)
// ============================================================================

/// Internal track state managed by the mixer.
///
/// Holds a queue of TrackInputs. The mixer reads from the head input;
/// when it returns EOF, the next input is activated. Each input may have
/// its own resampler for read-side lazy sample rate conversion.
pub(crate) struct InternalTrack {
    pub output_format: Format,
    pub inputs: Mutex<Vec<TrackInput>>,
    pub close_write: AtomicBool,
    pub close_err: Mutex<Option<String>>,
    pub notify_mixer: Arc<Condvar>,
}

impl InternalTrack {
    pub fn new(output_format: Format, notify_mixer: Arc<Condvar>) -> Self {
        Self {
            output_format,
            inputs: Mutex::new(Vec::new()),
            close_write: AtomicBool::new(false),
            close_err: Mutex::new(None),
            notify_mixer,
        }
    }

    /// Adds a new input with its format and ring buffer.
    /// A resampler is created automatically if the input format differs
    /// from the track's output format. Returns an error if the resampler
    /// cannot be created (e.g. invalid sample rate).
    pub fn add_input(&self, format: Format, rb: Arc<TrackRingBuf>) -> Result<(), String> {
        let input = TrackInput::new(format, self.output_format, rb)?;
        self.inputs.lock().unwrap().push(input);
        self.notify_mixer.notify_all();
        Ok(())
    }

    /// Reads a full chunk from the track in the output format.
    /// Zero-fills if not enough data.
    ///
    /// Data is pulled through the head input's resampler (if present)
    /// to convert from input format to output format on demand.
    /// Loops to fill the entire buffer, matching Go's readFull semantics.
    ///
    /// Returns (bytes_read, is_done):
    /// - bytes_read > 0: data was read (possibly zero-filled at the end)
    /// - is_done: true if all inputs are exhausted and write is closed
    pub fn read_full(&self, buf: &mut [u8]) -> (usize, bool) {
        buf.fill(0);

        if self.close_err.lock().unwrap().is_some() {
            return (0, true);
        }

        let mut inputs = self.inputs.lock().unwrap();
        let mut filled = 0usize;

        while !inputs.is_empty() {
            if filled >= buf.len() {
                break;
            }

            match inputs[0].read(&mut buf[filled..]) {
                ReadResult::Data(n) => {
                    filled += n;
                    if filled >= buf.len() {
                        return (buf.len(), false);
                    }
                }
                ReadResult::Empty => {
                    if filled > 0 {
                        return (buf.len(), false);
                    }
                    return (0, false);
                }
                ReadResult::Eof => {
                    inputs.remove(0);
                    continue;
                }
                ReadResult::Error(_) => {
                    inputs.remove(0);
                    continue;
                }
            }
        }

        if filled > 0 {
            return (buf.len(), false);
        }

        if self.close_write.load(Ordering::SeqCst) {
            (0, true)
        } else {
            (0, false)
        }
    }

    pub fn close_write(&self) {
        self.close_write.store(true, Ordering::SeqCst);
        let inputs = self.inputs.lock().unwrap();
        if let Some(last) = inputs.last() {
            last.rb.close_write();
        }
        self.notify_mixer.notify_all();
    }

    pub fn close_with_error(&self, err: String) {
        *self.close_err.lock().unwrap() = Some(err.clone());
        self.close_write.store(true, Ordering::SeqCst);
        let inputs = self.inputs.lock().unwrap();
        for input in inputs.iter() {
            input.rb.close_with_error(err.clone());
        }
        self.notify_mixer.notify_all();
    }

}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_ring_buf_write_read() {
        let notify = Arc::new(Condvar::new());
        let rb = TrackRingBuf::new(100, notify);

        // Write
        assert!(matches!(rb.write(&[1, 2, 3, 4, 5]), Ok(5)));

        // Read
        let mut buf = [0u8; 5];
        assert!(matches!(rb.read(&mut buf), ReadResult::Data(5)));
        assert_eq!(buf, [1, 2, 3, 4, 5]);

        // Read empty
        assert!(matches!(rb.read(&mut buf), ReadResult::Empty));
    }

    #[test]
    fn test_ring_buf_eof() {
        let notify = Arc::new(Condvar::new());
        let rb = TrackRingBuf::new(100, notify);

        rb.write(&[1, 2, 3]).unwrap();
        rb.close_write();

        let mut buf = [0u8; 10];
        assert!(matches!(rb.read(&mut buf), ReadResult::Data(3)));
        assert!(matches!(rb.read(&mut buf), ReadResult::Eof));
    }

    #[test]
    fn test_ring_buf_wrap_around() {
        let notify = Arc::new(Condvar::new());
        let rb = TrackRingBuf::new(8, notify);

        // Fill most of the buffer
        rb.write(&[1, 2, 3, 4, 5, 6]).unwrap();

        // Read some to create wrap-around space
        let mut buf = [0u8; 4];
        assert!(matches!(rb.read(&mut buf), ReadResult::Data(4)));

        // Write more (wraps around)
        rb.write(&[7, 8, 9, 10]).unwrap();

        // Read all
        let mut buf = [0u8; 6];
        assert!(matches!(rb.read(&mut buf), ReadResult::Data(6)));
        assert_eq!(buf, [5, 6, 7, 8, 9, 10]);
    }

    #[test]
    fn test_internal_track_read_full_zero_fill() {
        let notify = Arc::new(Condvar::new());
        let track = InternalTrack::new(Format::L16Mono16K, notify.clone());

        let rb = Arc::new(TrackRingBuf::new(1000, notify));
        rb.write(&[0x10, 0x00, 0x20, 0x00]).unwrap();
        track.add_input(Format::L16Mono16K, rb);

        let mut buf = vec![0xFFu8; 10];
        let (n, done) = track.read_full(&mut buf);
        assert_eq!(n, 10, "read_full should return full buffer size");
        assert!(!done);
        assert_eq!(buf[0], 0x10);
        assert_eq!(buf[1], 0x00);
        assert_eq!(buf[2], 0x20);
        assert_eq!(buf[3], 0x00);
        for &b in &buf[4..] {
            assert_eq!(b, 0, "remaining bytes should be zero-filled");
        }
    }

    #[test]
    fn test_internal_track_sequential_inputs() {
        let notify = Arc::new(Condvar::new());
        let track = InternalTrack::new(Format::L16Mono16K, notify.clone());

        let rb1 = Arc::new(TrackRingBuf::new(100, notify.clone()));
        rb1.write(&[1, 0, 2, 0]).unwrap();
        rb1.close_write();
        track.add_input(Format::L16Mono16K, rb1);

        let rb2 = Arc::new(TrackRingBuf::new(100, notify.clone()));
        rb2.write(&[3, 0, 4, 0]).unwrap();
        rb2.close_write();
        track.add_input(Format::L16Mono16K, rb2);

        track.close_write();

        let mut buf = [0u8; 4];
        let (n, done) = track.read_full(&mut buf);
        assert_eq!(n, 4);
        assert!(!done);
        assert_eq!(buf, [1, 0, 2, 0]);

        let (n, done) = track.read_full(&mut buf);
        assert_eq!(n, 4);
        assert_eq!(buf, [3, 0, 4, 0]);

        let (_, done) = track.read_full(&mut buf);
        assert!(done);
    }
}
