//! Track system for the mixer with independent ring buffers.
//!
//! Each track has a queue of `TrackInput`s, each with its own ring buffer
//! (and optional resampler). The mixer pulls fixed-size chunks from each
//! track; if a track has insufficient data, the remainder is zero-filled.
//!
//! This matches the Go implementation in `go/pkg/audio/pcm/track.go`.

use super::format::Format;
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

    pub fn is_eof(&self) -> bool {
        let inner = self.mu.lock().unwrap();
        inner.close_write && (inner.tail - inner.head) == 0
    }
}

pub(crate) enum ReadResult {
    Data(usize),
    Empty,
    Eof,
    Error(String),
}

// ============================================================================
// TrackInput — a single input writer with its own ring buffer
// ============================================================================

/// A single audio input source for a track, with its own ring buffer.
pub(crate) struct TrackInput {
    pub format: Format,
    pub rb: Arc<TrackRingBuf>,
}

// ============================================================================
// InternalTrack — the track as seen by the mixer (read side)
// ============================================================================

/// Internal track state managed by the mixer.
///
/// Holds a queue of TrackInputs. The mixer reads from the head input;
/// when it returns EOF, the next input is activated.
pub(crate) struct InternalTrack {
    pub inputs: Mutex<Vec<Arc<TrackRingBuf>>>,
    pub close_write: AtomicBool,
    pub close_err: Mutex<Option<String>>,
    pub notify_mixer: Arc<Condvar>,
}

impl InternalTrack {
    pub fn new(notify_mixer: Arc<Condvar>) -> Self {
        Self {
            inputs: Mutex::new(Vec::new()),
            close_write: AtomicBool::new(false),
            close_err: Mutex::new(None),
            notify_mixer,
        }
    }

    /// Adds a new input ring buffer to the track.
    pub fn add_input(&self, rb: Arc<TrackRingBuf>) {
        self.inputs.lock().unwrap().push(rb);
    }

    /// Reads a full chunk from the track. Zero-fills if not enough data.
    ///
    /// This is the key function: it reads from the head input's ring buffer.
    /// If the head input returns EOF, it moves to the next one.
    /// If no data is available from any input, the buffer is left zeroed.
    ///
    /// Returns (bytes_read, is_done):
    /// - bytes_read > 0: data was read (possibly zero-filled at the end)
    /// - is_done: true if all inputs are exhausted and write is closed
    pub fn read_full(&self, buf: &mut [u8]) -> (usize, bool) {
        // Pre-zero the buffer for zero-fill semantics
        buf.fill(0);

        if let Some(ref e) = *self.close_err.lock().unwrap() {
            return (0, true);
        }

        let mut inputs = self.inputs.lock().unwrap();
        let mut offset = 0;

        while offset < buf.len() && !inputs.is_empty() {
            let rb = &inputs[0];
            match rb.read(&mut buf[offset..]) {
                ReadResult::Data(n) => {
                    offset += n;
                    // Got some data, return what we have (rest is zero-filled)
                    if offset > 0 {
                        return (buf.len(), false);
                    }
                }
                ReadResult::Empty => {
                    // No data available from this input yet
                    // Return zeros (the buffer is already zeroed)
                    if offset > 0 {
                        return (buf.len(), false);
                    }
                    return (0, false);
                }
                ReadResult::Eof => {
                    // This input is done, move to next
                    inputs.remove(0);
                    continue;
                }
                ReadResult::Error(_) => {
                    inputs.remove(0);
                    continue;
                }
            }
        }

        if inputs.is_empty() && self.close_write.load(Ordering::SeqCst) {
            return (offset, true);
        }

        if offset > 0 {
            (buf.len(), false) // Zero-filled remainder
        } else {
            (0, false) // No data yet
        }
    }

    pub fn close_write(&self) {
        self.close_write.store(true, Ordering::SeqCst);
        let inputs = self.inputs.lock().unwrap();
        if let Some(last) = inputs.last() {
            last.close_write();
        }
        self.notify_mixer.notify_all();
    }

    pub fn close_with_error(&self, err: String) {
        *self.close_err.lock().unwrap() = Some(err.clone());
        self.close_write.store(true, Ordering::SeqCst);
        let inputs = self.inputs.lock().unwrap();
        for rb in inputs.iter() {
            rb.close_with_error(err.clone());
        }
        self.notify_mixer.notify_all();
    }

    pub fn is_done(&self) -> bool {
        if self.close_err.lock().unwrap().is_some() {
            return true;
        }
        if !self.close_write.load(Ordering::SeqCst) {
            return false;
        }
        let inputs = self.inputs.lock().unwrap();
        inputs.iter().all(|rb| rb.is_eof())
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
        let track = InternalTrack::new(notify.clone());

        let rb = Arc::new(TrackRingBuf::new(1000, notify));
        // Write only 4 bytes (2 samples)
        rb.write(&[0x10, 0x00, 0x20, 0x00]).unwrap();
        track.add_input(rb);

        // Read 10 bytes — should get 4 real + 6 zeros
        let mut buf = vec![0xFFu8; 10];
        let (n, done) = track.read_full(&mut buf);
        assert_eq!(n, 10, "read_full should return full buffer size");
        assert!(!done);
        // First 4 bytes are real data
        assert_eq!(buf[0], 0x10);
        assert_eq!(buf[1], 0x00);
        assert_eq!(buf[2], 0x20);
        assert_eq!(buf[3], 0x00);
        // Rest should be zero-filled
        for &b in &buf[4..] {
            assert_eq!(b, 0, "remaining bytes should be zero-filled");
        }
    }

    #[test]
    fn test_internal_track_sequential_inputs() {
        let notify = Arc::new(Condvar::new());
        let track = InternalTrack::new(notify.clone());

        // Input 1: writes [1,0, 2,0]
        let rb1 = Arc::new(TrackRingBuf::new(100, notify.clone()));
        rb1.write(&[1, 0, 2, 0]).unwrap();
        rb1.close_write();
        track.add_input(rb1);

        // Input 2: writes [3,0, 4,0]
        let rb2 = Arc::new(TrackRingBuf::new(100, notify.clone()));
        rb2.write(&[3, 0, 4, 0]).unwrap();
        rb2.close_write();
        track.add_input(rb2);

        track.close_write();

        // First read should get input 1
        let mut buf = [0u8; 4];
        let (n, done) = track.read_full(&mut buf);
        assert_eq!(n, 4);
        assert!(!done);
        assert_eq!(buf, [1, 0, 2, 0]);

        // Input 1 is EOF, should switch to input 2
        let (n, done) = track.read_full(&mut buf);
        assert_eq!(n, 4);
        assert_eq!(buf, [3, 0, 4, 0]);

        // Both inputs exhausted
        let (_, done) = track.read_full(&mut buf);
        assert!(done);
    }
}
