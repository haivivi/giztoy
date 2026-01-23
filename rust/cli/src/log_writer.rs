//! Log writer utilities for TUI display.

use giztoy_buffer::RingBuffer;
use std::io::{self, Write};
use tokio::sync::mpsc;

/// A thread-safe log buffer with max size using ring buffer.
pub type LogBuffer = RingBuffer<String>;

/// Creates a new buffer with the given max size.
pub fn new_log_buffer(max_size: usize) -> LogBuffer {
    RingBuffer::new(max_size)
}

/// Implements io::Write and captures log output for TUI display.
/// It stores lines in a buffer and notifies via a channel.
pub struct LogWriter {
    buf: LogBuffer,
    tx: mpsc::Sender<String>,
    rx: Option<mpsc::Receiver<String>>,
}

impl LogWriter {
    /// Creates a new log writer with the given max lines.
    pub fn new(max_lines: usize) -> Self {
        let (tx, rx) = mpsc::channel(100);
        Self {
            buf: new_log_buffer(max_lines),
            tx,
            rx: Some(rx),
        }
    }

    /// Takes the receiver channel. Can only be called once.
    pub fn take_receiver(&mut self) -> Option<mpsc::Receiver<String>> {
        self.rx.take()
    }

    /// Returns all buffered lines.
    pub fn lines(&self) -> Vec<String> {
        self.buf.to_vec()
    }

    /// Returns the sender for external use.
    pub fn sender(&self) -> mpsc::Sender<String> {
        self.tx.clone()
    }
}

impl Write for LogWriter {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        let text = String::from_utf8_lossy(buf);
        let text = text.trim_end_matches('\n');

        for line in text.split('\n') {
            let line = line.to_string();
            let _ = self.buf.add(line.clone());

            // Non-blocking send to channel
            let _ = self.tx.try_send(line);
        }

        Ok(buf.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

/// A sync-compatible log writer that can be used with tracing.
pub struct SyncLogWriter {
    buf: std::sync::Arc<std::sync::Mutex<LogBuffer>>,
    tx: mpsc::Sender<String>,
}

impl SyncLogWriter {
    /// Creates a new sync log writer with the given max lines.
    pub fn new(max_lines: usize) -> (Self, mpsc::Receiver<String>) {
        let (tx, rx) = mpsc::channel(100);
        let writer = Self {
            buf: std::sync::Arc::new(std::sync::Mutex::new(new_log_buffer(max_lines))),
            tx,
        };
        (writer, rx)
    }

    /// Returns all buffered lines.
    pub fn lines(&self) -> Vec<String> {
        self.buf.lock().unwrap().to_vec()
    }
}

impl Write for SyncLogWriter {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        let text = String::from_utf8_lossy(buf);
        let text = text.trim_end_matches('\n');

        for line in text.split('\n') {
            let line = line.to_string();
            if let Ok(guard) = self.buf.lock() {
                let _ = guard.add(line.clone());
            }
            let _ = self.tx.try_send(line);
        }

        Ok(buf.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

impl Clone for SyncLogWriter {
    fn clone(&self) -> Self {
        Self {
            buf: self.buf.clone(),
            tx: self.tx.clone(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_log_buffer() {
        let buf = new_log_buffer(3);
        buf.add("line1".to_string()).unwrap();
        buf.add("line2".to_string()).unwrap();
        buf.add("line3".to_string()).unwrap();
        buf.add("line4".to_string()).unwrap(); // Should evict line1

        let lines = buf.to_vec();
        assert_eq!(lines.len(), 3);
        assert_eq!(lines[0], "line2");
        assert_eq!(lines[2], "line4");
    }

    #[test]
    fn test_log_writer() {
        let mut writer = LogWriter::new(10);

        writeln!(writer, "Hello").unwrap();
        writeln!(writer, "World").unwrap();

        let lines = writer.lines();
        assert_eq!(lines.len(), 2);
        assert_eq!(lines[0], "Hello");
        assert_eq!(lines[1], "World");
    }

    #[test]
    fn test_log_writer_multiline() {
        let mut writer = LogWriter::new(10);

        write!(writer, "Line1\nLine2\nLine3").unwrap();

        let lines = writer.lines();
        assert_eq!(lines.len(), 3);
    }

    #[tokio::test]
    async fn test_log_writer_channel() {
        let mut writer = LogWriter::new(10);
        let mut rx = writer.take_receiver().unwrap();

        writeln!(writer, "Test").unwrap();

        let received = rx.recv().await.unwrap();
        assert_eq!(received, "Test");
    }
}
