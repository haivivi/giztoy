//! Utility functions for speech processing.

use crate::{Speech, SpeechError, SpeechSegment, SpeechStream};
use async_trait::async_trait;
use std::sync::Arc;
use tokio::sync::Mutex;

/// Collects all speeches from a SpeechStream into a single Speech.
pub struct SpeechCollector {
    stream: Arc<Mutex<Box<dyn SpeechStream>>>,
    current: Arc<Mutex<Option<Box<dyn Speech>>>>,
    done: bool,
}

impl SpeechCollector {
    /// Creates a new speech collector from a stream.
    pub fn new(stream: Box<dyn SpeechStream>) -> Self {
        Self {
            stream: Arc::new(Mutex::new(stream)),
            current: Arc::new(Mutex::new(None)),
            done: false,
        }
    }
}

#[async_trait]
impl Speech for SpeechCollector {
    async fn next(&mut self) -> Result<Box<dyn SpeechSegment>, SpeechError> {
        if self.done {
            return Err(SpeechError::Done);
        }

        loop {
            // Try to get segment from current speech
            {
                let mut current = self.current.lock().await;
                if let Some(ref mut speech) = *current {
                    match speech.next().await {
                        Ok(seg) => return Ok(seg),
                        Err(SpeechError::Done) => {
                            // Current speech exhausted, close it
                            let _ = speech.close().await;
                            *current = None;
                        }
                        Err(e) => {
                            self.done = true;
                            return Err(e);
                        }
                    }
                }
            }

            // Get next speech from stream
            let mut stream = self.stream.lock().await;
            match stream.next().await {
                Ok(speech) => {
                    let mut current = self.current.lock().await;
                    *current = Some(speech);
                }
                Err(SpeechError::Done) => {
                    self.done = true;
                    return Err(SpeechError::Done);
                }
                Err(e) => {
                    self.done = true;
                    return Err(e);
                }
            }
        }
    }

    async fn close(&mut self) -> Result<(), SpeechError> {
        self.done = true;

        // Close current speech if any
        {
            let mut current = self.current.lock().await;
            if let Some(ref mut speech) = *current {
                let _ = speech.close().await;
            }
            *current = None;
        }

        // Close stream
        let mut stream = self.stream.lock().await;
        stream.close().await
    }
}

/// Iterator adapter for Speech that yields (segment, error) pairs.
pub struct SpeechIter<'a> {
    speech: &'a mut dyn Speech,
    done: bool,
}

impl<'a> SpeechIter<'a> {
    /// Creates a new iterator over speech segments.
    pub fn new(speech: &'a mut dyn Speech) -> Self {
        Self { speech, done: false }
    }
}

/// Async iterator implementation for SpeechIter.
impl<'a> SpeechIter<'a> {
    /// Returns the next segment, or None if done.
    pub async fn next_segment(&mut self) -> Option<Result<Box<dyn SpeechSegment>, SpeechError>> {
        if self.done {
            return None;
        }

        match self.speech.next().await {
            Ok(seg) => Some(Ok(seg)),
            Err(SpeechError::Done) => {
                self.done = true;
                None
            }
            Err(e) => {
                self.done = true;
                Some(Err(e))
            }
        }
    }
}

#[cfg(test)]
mod util_tests {
    use super::*;

    // Mock implementations for testing
    struct MockSpeechStream {
        count: usize,
    }

    #[async_trait]
    impl SpeechStream for MockSpeechStream {
        async fn next(&mut self) -> Result<Box<dyn Speech>, SpeechError> {
            if self.count > 0 {
                self.count -= 1;
                Ok(Box::new(MockSpeech { segments: 2 }))
            } else {
                Err(SpeechError::Done)
            }
        }

        async fn close(&mut self) -> Result<(), SpeechError> {
            Ok(())
        }
    }

    struct MockSpeech {
        segments: usize,
    }

    #[async_trait]
    impl Speech for MockSpeech {
        async fn next(&mut self) -> Result<Box<dyn SpeechSegment>, SpeechError> {
            if self.segments > 0 {
                self.segments -= 1;
                Ok(Box::new(MockSegment))
            } else {
                Err(SpeechError::Done)
            }
        }

        async fn close(&mut self) -> Result<(), SpeechError> {
            Ok(())
        }
    }

    struct MockSegment;

    #[async_trait]
    impl SpeechSegment for MockSegment {
        fn decode(&self, _best: giztoy_audio::pcm::Format) -> Box<dyn crate::VoiceSegment> {
            unimplemented!()
        }

        fn transcribe(&self) -> Box<dyn tokio::io::AsyncRead + Send + Unpin> {
            Box::new(tokio::io::empty())
        }

        async fn close(&mut self) -> Result<(), SpeechError> {
            Ok(())
        }
    }

    #[tokio::test]
    async fn test_speech_collector() {
        let stream = MockSpeechStream { count: 2 };
        let mut collector = SpeechCollector::new(Box::new(stream));

        // Should get 4 segments total (2 speeches × 2 segments each)
        let mut count = 0;
        loop {
            match collector.next().await {
                Ok(_) => count += 1,
                Err(SpeechError::Done) => break,
                Err(e) => panic!("unexpected error: {}", e),
            }
        }
        assert_eq!(count, 4);
    }
}
