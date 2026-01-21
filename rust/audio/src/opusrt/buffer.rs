//! Jitter buffer for reordering Opus frames.

use std::collections::BinaryHeap;
use std::io;
use std::sync::Mutex;
use std::time::Duration;
use std::cmp::Ordering;
use super::frame::{Frame, from_stamped};
use super::timestamp::EpochMillis;

/// Tolerance for timestamp comparisons (milliseconds).
/// 2ms is smaller than the shortest Opus frame (2.5ms).
const TIMESTAMP_EPSILON: i64 = 2;

/// Default buffer duration (2 minutes).
const DEFAULT_BUFFER_DURATION: i64 = 2 * 60 * 1000;

/// Buffered frame with timestamp.
struct BufferedFrame {
    stamp: EpochMillis,
    frame: Frame,
}

impl BufferedFrame {
    fn end_stamp(&self) -> EpochMillis {
        self.stamp + self.frame.duration()
    }
}

impl PartialEq for BufferedFrame {
    fn eq(&self, other: &Self) -> bool {
        self.stamp == other.stamp
    }
}

impl Eq for BufferedFrame {}

impl PartialOrd for BufferedFrame {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

// Reverse ordering for min-heap
impl Ord for BufferedFrame {
    fn cmp(&self, other: &Self) -> Ordering {
        other.stamp.0.cmp(&self.stamp.0)
    }
}

/// Error for disordered packets.
#[derive(Debug)]
pub struct DisorderedPacketError;

impl std::fmt::Display for DisorderedPacketError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "opusrt: disordered packet")
    }
}

impl std::error::Error for DisorderedPacketError {}

/// Jitter buffer that reorders out-of-order Opus frames.
pub struct Buffer {
    inner: Mutex<BufferInner>,
}

struct BufferInner {
    /// Maximum buffered duration.
    max_duration: i64,
    /// Min-heap of frames sorted by timestamp.
    heap: BinaryHeap<BufferedFrame>,
    /// Timestamp of the last returned frame's end.
    tail: EpochMillis,
    /// Total buffered duration in milliseconds.
    buffered: i64,
}

impl Buffer {
    /// Creates a new jitter buffer with the specified maximum duration.
    pub fn new(max_duration: Duration) -> Self {
        Self {
            inner: Mutex::new(BufferInner {
                max_duration: max_duration.as_millis() as i64,
                heap: BinaryHeap::new(),
                tail: EpochMillis::from_millis(0),
                buffered: 0,
            }),
        }
    }

    /// Creates a buffer with default duration (2 minutes).
    pub fn default_duration() -> Self {
        Self::new(Duration::from_millis(DEFAULT_BUFFER_DURATION as u64))
    }

    /// Returns the next frame in timestamp order.
    ///
    /// Returns:
    /// - `Ok((Some(frame), Duration::ZERO))` - Normal frame
    /// - `Ok((None, loss_duration))` - Packet loss detected
    /// - `Err(io::ErrorKind::UnexpectedEof)` - Buffer empty
    pub fn frame(&self) -> io::Result<(Option<Frame>, Duration)> {
        let mut inner = self.inner.lock().unwrap();

        if inner.heap.is_empty() {
            return Err(io::Error::new(io::ErrorKind::UnexpectedEof, "buffer empty"));
        }

        // Peek at the first frame
        let first = inner.heap.peek().unwrap();
        let first_stamp = first.stamp;
        let tail = inner.tail;

        // Check for gap (packet loss)
        if tail.0 > 0 {
            let gap = first_stamp.0 - tail.0;
            if gap > TIMESTAMP_EPSILON {
                inner.tail = first_stamp;
                return Ok((None, Duration::from_millis(gap as u64)));
            }
        }

        // Pop the frame
        let frame = inner.heap.pop().unwrap();
        inner.tail = frame.end_stamp();
        inner.buffered -= frame.frame.duration().as_millis() as i64;

        Ok((Some(frame.frame), Duration::ZERO))
    }

    /// Appends a frame with its timestamp to the buffer.
    pub fn append(&self, frame: Frame, stamp: EpochMillis) -> Result<(), DisorderedPacketError> {
        let mut inner = self.inner.lock().unwrap();

        // Check if frame is too old
        if stamp.0 < inner.tail.0 {
            return Err(DisorderedPacketError);
        }

        let frame_dur = frame.duration().as_millis() as i64;
        inner.heap.push(BufferedFrame { stamp, frame });
        inner.buffered += frame_dur;

        // Trim buffer if it exceeds max duration
        let max_dur = if inner.max_duration > 0 {
            inner.max_duration
        } else {
            DEFAULT_BUFFER_DURATION
        };

        while inner.buffered > max_dur {
            if let Some(old) = inner.heap.pop() {
                inner.buffered -= old.frame.duration().as_millis() as i64;
            }
        }

        Ok(())
    }

    /// Writes stamped frame data.
    pub fn write(&self, stamped: &[u8]) -> io::Result<usize> {
        let (frame, stamp) = from_stamped(stamped)
            .ok_or_else(|| io::Error::new(io::ErrorKind::InvalidData, "invalid stamped frame"))?;

        self.append(frame, stamp)
            .map_err(|_| io::Error::new(io::ErrorKind::InvalidData, "disordered packet"))?;

        Ok(stamped.len())
    }

    /// Returns the number of frames in the buffer.
    pub fn len(&self) -> usize {
        self.inner.lock().unwrap().heap.len()
    }

    /// Returns true if the buffer is empty.
    pub fn is_empty(&self) -> bool {
        self.inner.lock().unwrap().heap.is_empty()
    }

    /// Resets the buffer, clearing all frames.
    pub fn reset(&self) {
        let mut inner = self.inner.lock().unwrap();
        inner.heap.clear();
        inner.tail = EpochMillis::from_millis(0);
        inner.buffered = 0;
    }

    /// Returns the total buffered duration.
    pub fn buffered(&self) -> Duration {
        Duration::from_millis(self.inner.lock().unwrap().buffered.max(0) as u64)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_frame() -> Frame {
        // Create a fake opus frame with 20ms duration
        // Config 1 = SILK NB 20ms, mono
        Frame::new(vec![0x08, 0x00])
    }

    #[test]
    fn test_buffer_in_order() {
        let buf = Buffer::new(Duration::from_secs(10));
        
        let f1 = make_frame();
        let f2 = make_frame();
        
        buf.append(f1.clone(), EpochMillis::from_millis(0)).unwrap();
        buf.append(f2.clone(), EpochMillis::from_millis(20)).unwrap();
        
        assert_eq!(buf.len(), 2);
        
        let (frame1, loss1) = buf.frame().unwrap();
        assert!(frame1.is_some());
        assert_eq!(loss1, Duration::ZERO);
        
        let (frame2, loss2) = buf.frame().unwrap();
        assert!(frame2.is_some());
        assert_eq!(loss2, Duration::ZERO);
    }

    #[test]
    fn test_buffer_out_of_order() {
        let buf = Buffer::new(Duration::from_secs(10));
        
        let f1 = make_frame();
        let f2 = make_frame();
        
        // Add frames out of order
        buf.append(f2.clone(), EpochMillis::from_millis(20)).unwrap();
        buf.append(f1.clone(), EpochMillis::from_millis(0)).unwrap();
        
        // Should get f1 first (lower timestamp)
        let (frame1, _) = buf.frame().unwrap();
        assert!(frame1.is_some());
    }

    #[test]
    fn test_buffer_disordered_rejection() {
        let buf = Buffer::new(Duration::from_secs(10));
        
        let f1 = make_frame();
        let f2 = make_frame();
        
        buf.append(f1, EpochMillis::from_millis(0)).unwrap();
        let _ = buf.frame().unwrap(); // Consume f1, tail is now 20
        
        // Try to add frame with earlier timestamp
        let result = buf.append(f2, EpochMillis::from_millis(10));
        assert!(result.is_err());
    }
}
