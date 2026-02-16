//! Multi-track audio mixer.
//!
//! The mixer allows mixing multiple audio tracks into a single output stream.
//! Each track can have independent gain (volume) control.

use super::{AtomicF32, Chunk, Format, FormatExt};
use std::collections::VecDeque;
use std::io::{self, Read};
use std::sync::atomic::{AtomicBool, AtomicI64, Ordering};
use std::sync::{Arc, Condvar, Mutex};
use std::time::Duration;

/// Callback type for mixer events.
type MixerCallback = Arc<dyn Fn() + Send + Sync>;

/// Options for configuring a Mixer.
pub struct MixerOptions {
    /// Automatically close writing when all tracks are removed.
    pub auto_close: bool,
    /// Duration of silence after which the mixer will close (0 = disabled).
    pub silence_gap: Duration,
    /// Called when a new track is created.
    on_track_created: Option<MixerCallback>,
    /// Called when a track is closed/removed.
    on_track_closed: Option<MixerCallback>,
}

impl Default for MixerOptions {
    fn default() -> Self {
        Self {
            auto_close: false,
            silence_gap: Duration::ZERO,
            on_track_created: None,
            on_track_closed: None,
        }
    }
}

impl std::fmt::Debug for MixerOptions {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("MixerOptions")
            .field("auto_close", &self.auto_close)
            .field("silence_gap", &self.silence_gap)
            .field("on_track_created", &self.on_track_created.is_some())
            .field("on_track_closed", &self.on_track_closed.is_some())
            .finish()
    }
}

impl MixerOptions {
    /// Creates options with auto_close enabled.
    pub fn with_auto_close(mut self) -> Self {
        self.auto_close = true;
        self
    }

    /// Sets the silence gap duration.
    pub fn with_silence_gap(mut self, gap: Duration) -> Self {
        self.silence_gap = gap;
        self
    }

    /// Sets a callback that fires when a new track is created.
    pub fn with_on_track_created(mut self, f: impl Fn() + Send + Sync + 'static) -> Self {
        self.on_track_created = Some(Arc::new(f));
        self
    }

    /// Sets a callback that fires when a track is closed/removed.
    pub fn with_on_track_closed(mut self, f: impl Fn() + Send + Sync + 'static) -> Self {
        self.on_track_closed = Some(Arc::new(f));
        self
    }
}

/// Options for configuring a Track.
#[derive(Debug, Clone, Default)]
pub struct TrackOptions {
    /// Optional label for the track.
    pub label: Option<String>,
}

impl TrackOptions {
    /// Creates options with a label.
    pub fn with_label(label: impl Into<String>) -> Self {
        Self {
            label: Some(label.into()),
        }
    }
}

/// Internal state of the mixer.
struct MixerState {
    tracks: Vec<Arc<TrackCtrlInner>>,
    close_err: Option<String>,
    close_write: bool,
    running_silence: Duration,
}

/// A multi-track audio mixer.
///
/// The mixer mixes multiple audio tracks into a single output stream.
/// It implements `Read` to provide the mixed audio data.
pub struct Mixer {
    output: Format,
    read_chunk: usize,
    auto_close: bool,
    silence_gap: Duration,

    state: Mutex<MixerState>,
    notify: Condvar,

    // Mixing buffer (reused across reads)
    mix_buf: Mutex<Vec<f32>>,

    // Callbacks
    on_track_created: Option<MixerCallback>,
    on_track_closed: Option<MixerCallback>,
}

impl Mixer {
    /// Creates a new Mixer with the specified output format and options.
    pub fn new(output: Format, opts: MixerOptions) -> Arc<Self> {
        let read_chunk = output.bytes_in_duration(Duration::from_millis(60)) as usize;
        let initial_silence = if opts.silence_gap > Duration::ZERO {
            opts.silence_gap
        } else {
            Duration::ZERO
        };

        Arc::new(Self {
            output,
            read_chunk,
            auto_close: opts.auto_close,
            silence_gap: opts.silence_gap,
            state: Mutex::new(MixerState {
                tracks: Vec::new(),
                close_err: None,
                close_write: false,
                running_silence: initial_silence,
            }),
            notify: Condvar::new(),
            mix_buf: Mutex::new(Vec::new()),
            on_track_created: opts.on_track_created,
            on_track_closed: opts.on_track_closed,
        })
    }

    /// Returns the output format of the mixer.
    pub fn output(&self) -> Format {
        self.output
    }

    /// Creates a new writable track in the mixer.
    ///
    /// Returns the Track for writing audio chunks and a TrackCtrl for
    /// controlling the track (gain, close, etc.).
    pub fn create_track(
        self: &Arc<Self>,
        opts: Option<TrackOptions>,
    ) -> io::Result<(Track, TrackCtrl)> {
        let opts = opts.unwrap_or_default();

        let mut state = self.state.lock().unwrap();

        if let Some(ref err) = state.close_err {
            return Err(io::Error::new(io::ErrorKind::BrokenPipe, err.clone()));
        }

        if state.close_write {
            return Err(io::Error::new(
                io::ErrorKind::BrokenPipe,
                "cannot create track after CloseWrite",
            ));
        }

        let inner = Arc::new(TrackCtrlInner::new(self.clone(), opts.label));
        state.tracks.push(inner.clone());

        // Notify readers that a new track is available
        self.notify.notify_all();

        // Fire on_track_created callback (outside the lock scope by dropping state first)
        drop(state);
        if let Some(ref cb) = self.on_track_created {
            cb();
        }

        Ok((
            Track {
                inner: inner.clone(),
            },
            TrackCtrl { inner },
        ))
    }

    /// Closes writing to the mixer.
    ///
    /// After calling this, no new tracks can be added, and Read will return
    /// EOF once all tracks are finished.
    pub fn close_write(&self) -> io::Result<()> {
        let mut state = self.state.lock().unwrap();
        self.close_write_locked(&mut state)
    }

    /// Closes the mixer with an error.
    ///
    /// All tracks will be closed with the same error.
    pub fn close_with_error(&self, err: io::Error) -> io::Result<()> {
        let mut state = self.state.lock().unwrap();

        if state.close_err.is_some() {
            return Ok(());
        }

        state.close_err = Some(err.to_string());

        if !state.close_write {
            state.close_write = true;
        }

        for track in &state.tracks {
            track.close_with_error(err.to_string());
        }

        self.notify.notify_all();

        Ok(())
    }

    /// Closes the mixer.
    pub fn close(&self) -> io::Result<()> {
        self.close_with_error(io::Error::new(io::ErrorKind::BrokenPipe, "mixer closed"))
    }

    fn close_write_locked(&self, state: &mut MixerState) -> io::Result<()> {
        if state.close_err.is_some() || state.close_write {
            return Ok(());
        }

        state.close_write = true;

        for track in &state.tracks {
            track.close_write();
        }

        self.notify.notify_all();

        Ok(())
    }

    fn notify_data_available(&self) {
        self.notify.notify_all();
    }

    /// Reads mixed audio data from the mixer.
    fn read_mixed(&self, buf: &mut [u8]) -> io::Result<usize> {
        let len = buf.len().min(self.read_chunk);
        let buf = &mut buf[..len];

        // Ensure buffer length is even (16-bit samples)
        let len = len & !1;
        if len == 0 {
            return Ok(0);
        }
        let buf = &mut buf[..len];

        let sample_count = len / 2;

        // Get or resize mixing buffer
        let mut mix_buf = self.mix_buf.lock().unwrap();
        if mix_buf.len() < sample_count {
            mix_buf.resize(sample_count, 0.0);
        }

        let mut state = self.state.lock().unwrap();

        loop {
            // Check for error
            if let Some(ref err) = state.close_err {
                return Err(io::Error::new(io::ErrorKind::BrokenPipe, err.clone()));
            }

            // Try to read and mix data
            let (peak, read, should_return_silence, should_eof) =
                self.try_read_and_mix(&mut state, &mut mix_buf[..sample_count]);

            if should_eof {
                return Err(io::Error::new(io::ErrorKind::UnexpectedEof, "EOF"));
            }

            if read || should_return_silence {
                // Update running silence counter
                if read {
                    state.running_silence = Duration::ZERO;
                } else {
                    state.running_silence += self.output.duration(len as u64);
                }

                // Convert mixed float32 samples to int16 and write to buffer byte-by-byte
                // (avoids alignment issues with from_raw_parts_mut)
                if peak == 0.0 {
                    // Output silence
                    buf[..len].fill(0);
                } else {
                    // Convert mixed float32 samples to int16
                    for i in 0..sample_count {
                        let mut t = mix_buf[i];
                        // Clip to prevent overflow
                        t = t.clamp(-1.0, 1.0);
                        // Convert to int16
                        let sample_i16 = if t >= 0.0 {
                            (t * 32767.0) as i16
                        } else {
                            (t * 32768.0) as i16
                        };
                        let bytes = sample_i16.to_le_bytes();
                        buf[i * 2] = bytes[0];
                        buf[i * 2 + 1] = bytes[1];
                    }
                }

                return Ok(len);
            }

            // No data available, wait for notification
            // Use wait_timeout to avoid infinite blocking
            let (new_state, _timeout) = self
                .notify
                .wait_timeout(state, Duration::from_millis(100))
                .unwrap();
            state = new_state;
        }
    }

    /// Try to read and mix audio data from all tracks.
    /// Returns (peak, has_data, should_return_silence, should_eof)
    fn try_read_and_mix(
        &self,
        state: &mut MixerState,
        mix_buf: &mut [f32],
    ) -> (f32, bool, bool, bool) {
        // Check if we should EOF
        if state.tracks.is_empty() {
            if state.close_write {
                return (0.0, false, false, true); // EOF
            }

            if self.auto_close {
                let _ = self.close_write_locked(state);
                return (0.0, false, false, true); // EOF
            }

            // No tracks, return silence if within silence gap
            if state.running_silence < self.silence_gap {
                return (0.0, false, true, false); // silence
            }

            // No data, no silence, caller should wait
            return (0.0, false, false, false);
        }

        // Clear mixing buffer
        for sample in mix_buf.iter_mut() {
            *sample = 0.0;
        }

        let mut peak: f32 = 0.0;
        let mut has_data = false;

        // Create temporary buffer for reading track data
        let bytes_needed = mix_buf.len() * 2;
        let mut track_buf = vec![0u8; bytes_needed];

        let initial_track_count = state.tracks.len();

        // Try to read from all tracks
        state.tracks.retain(|track| {
            if track.is_closed() {
                return false; // Remove closed tracks
            }

            let n = track.try_read(&mut track_buf);
            if n > 0 {
                has_data = true;
                let gain = track.gain.load(Ordering::Relaxed);

                // Mix this track's audio into the buffer
                let sample_count = n / 2;
                for i in 0..sample_count.min(mix_buf.len()) {
                    let sample =
                        i16::from_le_bytes([track_buf[i * 2], track_buf[i * 2 + 1]]);
                    if sample != 0 {
                        // Convert int16 to float32 in range [-1.0, 1.0]
                        let s = if sample >= 0 {
                            sample as f32 / 32767.0
                        } else {
                            sample as f32 / 32768.0
                        };
                        // Apply track gain
                        let s = s * gain;
                        // Track peak amplitude
                        peak = peak.max(s.abs());
                        // Accumulate into mixing buffer
                        mix_buf[i] += s;
                    }
                }
            }

            !track.is_done() // Keep track if not done
        });

        // Fire on_track_closed for each removed track
        let removed = initial_track_count - state.tracks.len();
        if removed > 0 {
            if let Some(ref cb) = self.on_track_closed {
                for _ in 0..removed {
                    cb();
                }
            }
        }

        (peak, has_data, false, false)
    }
}

impl Read for Mixer {
    fn read(&mut self, buf: &mut [u8]) -> io::Result<usize> {
        self.read_mixed(buf)
    }
}

// Also implement for Arc<Mixer> so it can be used with shared ownership
impl Read for &Mixer {
    fn read(&mut self, buf: &mut [u8]) -> io::Result<usize> {
        self.read_mixed(buf)
    }
}

/// Internal track controller state.
struct TrackCtrlInner {
    mixer: Arc<Mixer>,
    label: Option<String>,
    gain: AtomicF32,
    read_bytes: AtomicI64,
    fade_out_ms: AtomicI64,

    buffer: Mutex<TrackBuffer>,
    buffer_notify: Condvar, // Notifies when buffer space becomes available
    closed: AtomicBool,
    write_closed: AtomicBool,
    error: Mutex<Option<String>>,
}

impl TrackCtrlInner {
    fn new(mixer: Arc<Mixer>, label: Option<String>) -> Self {
        let buffer_size = mixer.output.bytes_rate() as usize * 10; // 10 seconds
        Self {
            mixer,
            label,
            gain: AtomicF32::new(1.0),
            read_bytes: AtomicI64::new(0),
            fade_out_ms: AtomicI64::new(0),
            buffer: Mutex::new(TrackBuffer::new(buffer_size)),
            buffer_notify: Condvar::new(),
            closed: AtomicBool::new(false),
            write_closed: AtomicBool::new(false),
            error: Mutex::new(None),
        }
    }

    fn write(&self, data: &[u8]) -> io::Result<usize> {
        if data.is_empty() {
            return Ok(0);
        }

        if self.closed.load(Ordering::SeqCst) {
            return Err(io::Error::new(io::ErrorKind::BrokenPipe, "track closed"));
        }

        if self.write_closed.load(Ordering::SeqCst) {
            return Err(io::Error::new(
                io::ErrorKind::BrokenPipe,
                "track write closed",
            ));
        }

        let mut written = 0;
        let mut remaining = data;

        let mut buffer = self.buffer.lock().unwrap();

        while !remaining.is_empty() {
            // Try to write to buffer
            let n = buffer.write(remaining);
            if n > 0 {
                written += n;
                remaining = &remaining[n..];
                // Notify mixer that data is available
                self.mixer.notify_data_available();
            }

            if remaining.is_empty() {
                break;
            }

            // Buffer full, wait for space to become available
            // Use wait_timeout to periodically check for close signals
            let (new_buffer, _timeout) = self
                .buffer_notify
                .wait_timeout(buffer, Duration::from_millis(100))
                .unwrap();
            buffer = new_buffer;

            // Check if we should stop
            if self.closed.load(Ordering::SeqCst) {
                return Err(io::Error::new(io::ErrorKind::BrokenPipe, "track closed"));
            }

            if self.write_closed.load(Ordering::SeqCst) {
                return Err(io::Error::new(
                    io::ErrorKind::BrokenPipe,
                    "track write closed",
                ));
            }
        }

        Ok(written)
    }

    /// Try to read data from the buffer (non-blocking).
    fn try_read(&self, buf: &mut [u8]) -> usize {
        let mut buffer = self.buffer.lock().unwrap();
        let n = buffer.read(buf);
        if n > 0 {
            self.read_bytes.fetch_add(n as i64, Ordering::Relaxed);
            // Notify writers that buffer space is available
            self.buffer_notify.notify_one();
        }
        n
    }

    fn is_closed(&self) -> bool {
        self.closed.load(Ordering::SeqCst)
    }

    fn is_done(&self) -> bool {
        if self.closed.load(Ordering::SeqCst) {
            return true;
        }
        // Done if write is closed and buffer is empty
        if self.write_closed.load(Ordering::SeqCst) {
            let buffer = self.buffer.lock().unwrap();
            if buffer.is_empty() {
                return true;
            }
        }
        false
    }

    fn close_write(&self) {
        self.write_closed.store(true, Ordering::SeqCst);
        self.buffer_notify.notify_all(); // Wake up any waiting writers
        self.mixer.notify_data_available();
    }

    fn close_with_error(&self, err: String) {
        *self.error.lock().unwrap() = Some(err);
        self.closed.store(true, Ordering::SeqCst);
        self.write_closed.store(true, Ordering::SeqCst);
        self.buffer_notify.notify_all(); // Wake up any waiting writers
        self.mixer.notify_data_available();
    }
}

/// Ring buffer for track audio data.
struct TrackBuffer {
    data: VecDeque<u8>,
    capacity: usize,
}

impl TrackBuffer {
    fn new(capacity: usize) -> Self {
        Self {
            data: VecDeque::with_capacity(capacity),
            capacity,
        }
    }

    fn write(&mut self, data: &[u8]) -> usize {
        let available = self.capacity - self.data.len();
        let to_write = data.len().min(available);
        self.data.extend(&data[..to_write]);
        to_write
    }

    fn read(&mut self, buf: &mut [u8]) -> usize {
        let to_read = buf.len().min(self.data.len());
        for (i, byte) in self.data.drain(..to_read).enumerate() {
            buf[i] = byte;
        }
        to_read
    }

    fn is_empty(&self) -> bool {
        self.data.is_empty()
    }
}

/// A writable audio track in a Mixer.
pub struct Track {
    inner: Arc<TrackCtrlInner>,
}

impl Track {
    /// Writes an audio chunk to the track.
    pub fn write(&self, chunk: &dyn Chunk) -> io::Result<()> {
        if let Some(bytes) = chunk.as_bytes() {
            self.inner.write(bytes)?;
        } else {
            // For chunks that don't have direct byte access, use write_to
            let mut buf = Vec::new();
            chunk.write_to(&mut buf)?;
            self.inner.write(&buf)?;
        }
        Ok(())
    }

    /// Writes raw bytes to the track.
    pub fn write_bytes(&self, data: &[u8]) -> io::Result<usize> {
        self.inner.write(data)
    }
}

/// Controller for a track in the mixer.
///
/// Provides control over gain (volume), fade-out, and track lifecycle.
pub struct TrackCtrl {
    inner: Arc<TrackCtrlInner>,
}

impl TrackCtrl {
    /// Returns the label of the track, if set.
    pub fn label(&self) -> Option<&str> {
        self.inner.label.as_deref()
    }

    /// Returns the current gain (volume) of the track.
    pub fn gain(&self) -> f32 {
        self.inner.gain.load(Ordering::Relaxed)
    }

    /// Sets the gain (volume) of the track.
    ///
    /// The gain is a linear multiplier where 1.0 is full volume,
    /// 0.0 is silence, and values greater than 1.0 may cause clipping.
    pub fn set_gain(&self, gain: f32) {
        self.inner.gain.store(gain, Ordering::Relaxed);
    }

    /// Linearly fades the track's gain from the current value to the target
    /// value over the specified duration.
    ///
    /// This method blocks until the fade is complete.
    pub fn set_gain_linear_to(&self, to: f32, duration: Duration) {
        let from = self.gain();
        let steps = (duration.as_millis() / 10) as usize;
        if steps == 0 {
            self.set_gain(to);
            return;
        }

        for i in 0..steps {
            std::thread::sleep(Duration::from_millis(10));
            let progress = i as f32 / steps as f32;
            self.set_gain(from + (to - from) * progress);
        }
        self.set_gain(to);
    }

    /// Sets the fade-out duration for the track.
    ///
    /// When the track is closed, it will automatically fade out over this
    /// duration before actually closing.
    pub fn set_fade_out_duration(&self, duration: Duration) {
        self.inner
            .fade_out_ms
            .store(duration.as_millis() as i64, Ordering::Relaxed);
    }

    /// Returns the total number of bytes read from this track.
    pub fn read_bytes(&self) -> i64 {
        self.inner.read_bytes.load(Ordering::Relaxed)
    }

    /// Closes writing to the track.
    pub fn close_write(&self) {
        self.inner.close_write();
    }

    /// Closes writing to the track after appending silence.
    pub fn close_write_with_silence(&self, silence: Duration) -> io::Result<()> {
        let chunk = self.inner.mixer.output.silence_chunk(silence);
        let mut buf = Vec::new();
        chunk.write_to(&mut buf)?;
        self.inner.write(&buf)?;
        self.close_write();
        Ok(())
    }

    /// Closes the track.
    ///
    /// If a fade-out duration has been set, the track will fade out before
    /// closing.
    pub fn close(&self) {
        let fade_ms = self.inner.fade_out_ms.load(Ordering::Relaxed);
        if fade_ms > 0 {
            let inner = self.inner.clone();
            std::thread::spawn(move || {
                // Fade out
                let from = inner.gain.load(Ordering::Relaxed);
                let steps = (fade_ms / 10) as usize;
                for i in 0..steps {
                    std::thread::sleep(Duration::from_millis(10));
                    let progress = i as f32 / steps as f32;
                    inner.gain.store(from * (1.0 - progress), Ordering::Relaxed);
                }
                inner.closed.store(true, Ordering::SeqCst);
                inner.write_closed.store(true, Ordering::SeqCst);
                inner.mixer.notify_data_available();
            });
            self.inner.close_write();
        } else {
            self.inner.closed.store(true, Ordering::SeqCst);
            self.inner.write_closed.store(true, Ordering::SeqCst);
            self.inner.mixer.notify_data_available();
        }
    }

    /// Closes the track with an error.
    pub fn close_with_error(&self, err: io::Error) {
        let fade_ms = self.inner.fade_out_ms.load(Ordering::Relaxed);
        if fade_ms > 0 {
            let inner = self.inner.clone();
            let err_msg = err.to_string();
            std::thread::spawn(move || {
                // Fade out
                let from = inner.gain.load(Ordering::Relaxed);
                let steps = (fade_ms / 10) as usize;
                for i in 0..steps {
                    std::thread::sleep(Duration::from_millis(10));
                    let progress = i as f32 / steps as f32;
                    inner.gain.store(from * (1.0 - progress), Ordering::Relaxed);
                }
                inner.close_with_error(err_msg);
            });
            self.inner.close_write();
        } else {
            self.inner.close_with_error(err.to_string());
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::f64::consts::PI;

    /// Generates a sine wave as i16 samples.
    fn generate_sine_wave(freq: f64, sample_rate: u32, duration_ms: u32) -> Vec<u8> {
        let samples = (sample_rate * duration_ms / 1000) as usize;
        let mut data = Vec::with_capacity(samples * 2);
        for i in 0..samples {
            let t = i as f64 / sample_rate as f64;
            let value = (2.0 * PI * freq * t).sin();
            let sample = (value * 16000.0) as i16;
            data.extend_from_slice(&sample.to_le_bytes());
        }
        data
    }

    #[test]
    fn test_mixer_creates_track() {
        let mixer = Mixer::new(Format::L16Mono16K, MixerOptions::default());
        let result = mixer.create_track(None);
        assert!(result.is_ok());
    }

    #[test]
    fn test_mixer_mixes_two_tracks() {
        let mixer = Mixer::new(Format::L16Mono16K, MixerOptions::default().with_auto_close());

        // Create two tracks
        let (track1, ctrl1) = mixer
            .create_track(Some(TrackOptions::with_label("440Hz")))
            .unwrap();
        let (track2, ctrl2) = mixer
            .create_track(Some(TrackOptions::with_label("880Hz")))
            .unwrap();

        // Generate 100ms of audio for each track
        let wave1 = generate_sine_wave(440.0, 16000, 100);
        let wave2 = generate_sine_wave(880.0, 16000, 100);

        // Write to tracks in threads
        let h1 = std::thread::spawn(move || {
            track1.write_bytes(&wave1).unwrap();
            ctrl1.close_write();
        });

        let h2 = std::thread::spawn(move || {
            track2.write_bytes(&wave2).unwrap();
            ctrl2.close_write();
        });

        // Read mixed output
        let mut mixed = Vec::new();
        let mut buf = [0u8; 1024];
        loop {
            match (&*mixer).read(&mut buf) {
                Ok(0) => break,
                Ok(n) => mixed.extend_from_slice(&buf[..n]),
                Err(e) if e.kind() == io::ErrorKind::UnexpectedEof => break,
                Err(e) => panic!("read error: {}", e),
            }
        }

        h1.join().unwrap();
        h2.join().unwrap();

        // Verify we got some output
        assert!(!mixed.is_empty(), "Mixed output should not be empty");

        // Convert to samples and analyze
        let samples: Vec<i16> = mixed
            .chunks_exact(2)
            .map(|b| i16::from_le_bytes([b[0], b[1]]))
            .collect();

        // Count non-zero samples
        let non_zero = samples.iter().filter(|&&s| s != 0).count();
        assert!(non_zero > 0, "Should have non-zero samples");
    }

    #[test]
    fn test_mixer_gain_control() {
        let mixer = Mixer::new(Format::L16Mono16K, MixerOptions::default().with_auto_close());

        let (track, ctrl) = mixer.create_track(None).unwrap();

        // Set gain to 0.5
        ctrl.set_gain(0.5);
        assert!((ctrl.gain() - 0.5).abs() < 0.001);

        // Write constant value
        let data: Vec<u8> = (0..100).flat_map(|_| 10000i16.to_le_bytes()).collect();

        let h = std::thread::spawn(move || {
            track.write_bytes(&data).unwrap();
            ctrl.close_write();
        });

        // Read and verify gain was applied
        let mut buf = [0u8; 400];
        let n = (&*mixer).read(&mut buf).unwrap_or(0);

        h.join().unwrap();

        if n > 0 {
            let samples: Vec<i16> = buf[..n]
                .chunks_exact(2)
                .map(|b| i16::from_le_bytes([b[0], b[1]]))
                .collect();

            // With 0.5 gain, 10000 should become ~5000
            for &sample in &samples {
                if sample != 0 {
                    // Allow some tolerance for float conversion
                    assert!(
                        sample.abs() < 6000,
                        "Sample {} should be reduced by gain",
                        sample
                    );
                }
            }
        }
    }

    #[test]
    fn test_track_buffer() {
        let mut buf = TrackBuffer::new(100);

        // Write data
        let data = [1u8, 2, 3, 4, 5];
        assert_eq!(buf.write(&data), 5);

        // Read data
        let mut out = [0u8; 5];
        assert_eq!(buf.read(&mut out), 5);
        assert_eq!(out, data);

        // Buffer should be empty
        assert!(buf.is_empty());
    }

    #[test]
    fn test_mixer_concurrent_write() {
        let mixer = Mixer::new(Format::L16Mono16K, MixerOptions::default().with_auto_close());

        let (track1, ctrl1) = mixer
            .create_track(Some(TrackOptions::with_label("A")))
            .unwrap();
        let (track2, ctrl2) = mixer
            .create_track(Some(TrackOptions::with_label("B")))
            .unwrap();

        // Track A: all 1000
        // Track B: all 2000
        // Mixed should be ~3000
        let data_a: Vec<u8> = (0..1600).flat_map(|_| 1000i16.to_le_bytes()).collect();
        let data_b: Vec<u8> = (0..1600).flat_map(|_| 2000i16.to_le_bytes()).collect();

        let h1 = std::thread::spawn(move || {
            track1.write_bytes(&data_a).unwrap();
            ctrl1.close_write();
        });

        let h2 = std::thread::spawn(move || {
            track2.write_bytes(&data_b).unwrap();
            ctrl2.close_write();
        });

        // Read mixed output
        let mut mixed = Vec::new();
        let mut buf = [0u8; 1024];
        loop {
            match (&*mixer).read(&mut buf) {
                Ok(0) => break,
                Ok(n) => mixed.extend_from_slice(&buf[..n]),
                Err(e) if e.kind() == io::ErrorKind::UnexpectedEof => break,
                Err(e) => panic!("read error: {}", e),
            }
        }

        h1.join().unwrap();
        h2.join().unwrap();

        // Analyze output
        let samples: Vec<i16> = mixed
            .chunks_exact(2)
            .map(|b| i16::from_le_bytes([b[0], b[1]]))
            .collect();

        let mut count_1000 = 0;
        let mut count_2000 = 0;
        let mut count_3000 = 0;

        for &s in &samples {
            if (s - 1000).abs() < 100 {
                count_1000 += 1;
            } else if (s - 2000).abs() < 100 {
                count_2000 += 1;
            } else if (s - 3000).abs() < 100 {
                count_3000 += 1;
            }
        }

        // We should have at least some audio
        assert!(
            count_1000 + count_2000 + count_3000 > 0,
            "Should have audio data"
        );
    }
}
