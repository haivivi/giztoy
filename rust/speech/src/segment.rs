//! Sentence segmentation utilities.

use std::sync::Arc;
use tokio::io::{AsyncRead, AsyncReadExt};
use tokio::sync::{mpsc, Mutex};

/// Error type for segmentation operations.
#[derive(Debug, thiserror::Error)]
pub enum SegmentError {
    #[error("end of stream")]
    Done,
    #[error("io error: {0}")]
    Io(#[from] std::io::Error),
    #[error("closed")]
    Closed,
}

/// Interface for iterating over sentences.
#[async_trait::async_trait]
pub trait SentenceIterator: Send + Sync {
    /// Returns the next sentence.
    async fn next(&mut self) -> Result<String, SegmentError>;

    /// Closes the iterator.
    fn close(&mut self);
}

/// Interface for segmenting text into sentences.
#[async_trait::async_trait]
pub trait SentenceSegmenter: Send + Sync {
    /// Segments the text from the reader into sentences.
    async fn segment(
        &self,
        reader: Box<dyn AsyncRead + Send + Unpin>,
    ) -> Result<Box<dyn SentenceIterator>, SegmentError>;
}

/// Default sentence segmenter that splits on punctuation.
pub struct DefaultSentenceSegmenter {
    /// Maximum number of characters allowed in each segment.
    /// If the text exceeds this limit, it will be split into multiple segments.
    /// Defaults to 256.
    pub max_chars_per_segment: usize,
}

impl Default for DefaultSentenceSegmenter {
    fn default() -> Self {
        Self {
            max_chars_per_segment: 256,
        }
    }
}

impl DefaultSentenceSegmenter {
    /// Creates a new segmenter with the specified max chars per segment.
    pub fn new(max_chars: usize) -> Self {
        Self {
            max_chars_per_segment: max_chars,
        }
    }
}

#[async_trait::async_trait]
impl SentenceSegmenter for DefaultSentenceSegmenter {
    async fn segment(
        &self,
        reader: Box<dyn AsyncRead + Send + Unpin>,
    ) -> Result<Box<dyn SentenceIterator>, SegmentError> {
        let (tx, rx) = mpsc::channel(16);
        let max_chars = self.max_chars_per_segment;

        tokio::spawn(async move {
            let mut reader = reader;
            let mut buf = String::new();
            let mut temp = [0u8; 1024];

            loop {
                match reader.read(&mut temp).await {
                    Ok(0) => {
                        // EOF - send remaining buffer
                        if !buf.is_empty() {
                            let _ = tx.send(Ok(std::mem::take(&mut buf))).await;
                        }
                        break;
                    }
                    Ok(n) => {
                        if let Ok(s) = std::str::from_utf8(&temp[..n]) {
                            buf.push_str(s);

                            // Try to find sentence boundaries
                            while let Some(idx) = find_sentence_boundary(&buf, max_chars) {
                                if idx > 0 {
                                    let sentence: String = buf.drain(..idx).collect();
                                    if tx.send(Ok(sentence)).await.is_err() {
                                        return;
                                    }
                                } else {
                                    break;
                                }
                            }
                        }
                    }
                    Err(e) => {
                        let _ = tx.send(Err(SegmentError::Io(e))).await;
                        break;
                    }
                }
            }
        });

        Ok(Box::new(DefaultSentenceIterator {
            rx: Arc::new(Mutex::new(rx)),
            closed: false,
        }))
    }
}

/// Finds the index of the first sentence boundary in the string.
/// Returns None if no boundary is found.
fn find_sentence_boundary(s: &str, max_chars: usize) -> Option<usize> {
    let chars: Vec<char> = s.chars().collect();

    // If we've exceeded max chars, force a split
    if chars.len() >= max_chars {
        // Try to find a boundary first
        for (i, &c) in chars.iter().enumerate().take(max_chars) {
            if is_sentence_boundary(c, i, &chars) {
                return Some(s.char_indices().nth(i + 1).map(|(idx, _)| idx).unwrap_or(s.len()));
            }
        }
        // No boundary found, force split at max_chars
        return Some(s.char_indices().nth(max_chars).map(|(idx, _)| idx).unwrap_or(s.len()));
    }

    // Look for natural boundaries
    for (i, &c) in chars.iter().enumerate() {
        if is_sentence_boundary(c, i, &chars) {
            return Some(s.char_indices().nth(i + 1).map(|(idx, _)| idx).unwrap_or(s.len()));
        }
    }

    None
}

/// Checks if a character is a sentence boundary.
fn is_sentence_boundary(c: char, idx: usize, chars: &[char]) -> bool {
    let prev = if idx > 0 { chars[idx - 1] } else { ' ' };
    let next = if idx + 1 < chars.len() { chars[idx + 1] } else { ' ' };

    match c {
        // Handle decimal numbers and times (9.9, 10:15)
        '.' | ':' | ',' | '：' => {
            !(next.is_ascii_digit() && prev.is_ascii_digit())
        }
        // Definite sentence boundaries
        '，' | '；' | '。' | '？' | '！' | '…' | '～' | '?' | '!' | '¿' | '¡' | ';' | '~'
        | '\r' | '\n' | '„' | '・' => true,
        _ => false,
    }
}

struct DefaultSentenceIterator {
    rx: Arc<Mutex<mpsc::Receiver<Result<String, SegmentError>>>>,
    closed: bool,
}

#[async_trait::async_trait]
impl SentenceIterator for DefaultSentenceIterator {
    async fn next(&mut self) -> Result<String, SegmentError> {
        if self.closed {
            return Err(SegmentError::Closed);
        }

        let mut rx = self.rx.lock().await;
        match rx.recv().await {
            Some(result) => result,
            None => Err(SegmentError::Done),
        }
    }

    fn close(&mut self) {
        self.closed = true;
    }
}

#[cfg(test)]
mod segment_tests {
    use super::*;

    #[test]
    fn test_is_sentence_boundary() {
        let chars: Vec<char> = "Hello. World".chars().collect();
        assert!(is_sentence_boundary('.', 5, &chars));

        let chars: Vec<char> = "3.14".chars().collect();
        assert!(!is_sentence_boundary('.', 1, &chars)); // Decimal number

        let chars: Vec<char> = "10:30".chars().collect();
        assert!(!is_sentence_boundary(':', 2, &chars)); // Time
    }

    #[test]
    fn test_find_sentence_boundary() {
        assert_eq!(find_sentence_boundary("Hello. World", 256), Some(6)); // After the period
        assert_eq!(find_sentence_boundary("你好。世界", 256), Some(9)); // 3 bytes per char, after 。
        assert_eq!(find_sentence_boundary("No boundary here", 256), None);
    }

    #[tokio::test]
    async fn test_default_segmenter() {
        let segmenter = DefaultSentenceSegmenter::default();
        let text = "Hello. World!";

        // Use tokio's cursor directly (it implements AsyncRead)
        let async_reader = std::io::Cursor::new(text.as_bytes().to_vec());
        let mut iter = segmenter.segment(Box::new(async_reader)).await.unwrap();

        let s1 = iter.next().await.unwrap();
        assert!(s1.contains("Hello"));
    }
}
