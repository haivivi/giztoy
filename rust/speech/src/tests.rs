//! Integration tests for speech module.

use super::*;
use async_trait::async_trait;
use giztoy_audio::opusrt::{Frame, FrameReader};
use giztoy_audio::pcm::Format;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::Arc;
use std::time::Duration;
use tokio::io::AsyncRead;

// ============================================================================
// Test Error Types
// ============================================================================

#[test]
fn test_all_error_types() {
    // Voice errors
    let _ = VoiceError::Done.to_string();
    let _ = VoiceError::Io(std::io::Error::new(std::io::ErrorKind::Other, "test")).to_string();
    let _ = VoiceError::Decode("test".to_string()).to_string();
    let _ = VoiceError::Other("test".to_string()).to_string();

    // Speech errors
    let _ = SpeechError::Done.to_string();
    let _ = SpeechError::Io(std::io::Error::new(std::io::ErrorKind::Other, "test")).to_string();
    let _ = SpeechError::Transcription("test".to_string()).to_string();
    let _ = SpeechError::Other("test".to_string()).to_string();

    // TTS errors
    let _ = TTSError::NotFound("test".to_string()).to_string();
    let _ = TTSError::SynthesisFailed("test".to_string()).to_string();
    let _ = TTSError::Pattern("test".to_string()).to_string();
    let _ = TTSError::Other("test".to_string()).to_string();

    // ASR errors
    let _ = ASRError::NotFound("test".to_string()).to_string();
    let _ = ASRError::TranscriptionFailed("test".to_string()).to_string();
    let _ = ASRError::Pattern("test".to_string()).to_string();
    let _ = ASRError::Other("test".to_string()).to_string();

    // Segment errors
    let _ = SegmentError::Done.to_string();
    let _ = SegmentError::Closed.to_string();
}

// ============================================================================
// Mock Implementations
// ============================================================================

struct MockSynthesizer {
    call_count: Arc<AtomicUsize>,
    name_pattern: String,
}

impl MockSynthesizer {
    fn new(name_pattern: &str) -> Self {
        Self {
            call_count: Arc::new(AtomicUsize::new(0)),
            name_pattern: name_pattern.to_string(),
        }
    }
    
    fn call_count(&self) -> usize {
        self.call_count.load(Ordering::SeqCst)
    }
}

#[async_trait]
impl Synthesizer for MockSynthesizer {
    async fn synthesize(
        &self,
        name: &str,
        _text_stream: Box<dyn AsyncRead + Send + Unpin>,
        _format: Format,
    ) -> Result<Box<dyn Speech>, TTSError> {
        self.call_count.fetch_add(1, Ordering::SeqCst);
        Ok(Box::new(MockSpeech::new(&format!(
            "synthesized from {} using pattern {}",
            name, self.name_pattern
        ))))
    }
}

struct MockFrameReader {
    frames: Vec<Option<Frame>>,
    index: usize,
}

impl MockFrameReader {
    fn new(frame_count: usize) -> Self {
        let frames = (0..frame_count)
            .map(|_| Some(Frame::new(vec![0u8; 100])))
            .chain(std::iter::once(None))
            .collect();
        Self { frames, index: 0 }
    }
}

impl FrameReader for MockFrameReader {
    fn next_frame(&mut self) -> Result<(Option<Frame>, Duration), std::io::Error> {
        if self.index < self.frames.len() {
            let frame = self.frames[self.index].take();
            self.index += 1;
            Ok((frame, Duration::from_millis(20)))
        } else {
            Ok((None, Duration::ZERO))
        }
    }
}

struct MockStreamTranscriber {
    call_count: Arc<AtomicUsize>,
    model_pattern: String,
}

impl MockStreamTranscriber {
    fn new(model_pattern: &str) -> Self {
        Self {
            call_count: Arc::new(AtomicUsize::new(0)),
            model_pattern: model_pattern.to_string(),
        }
    }
    
    fn call_count(&self) -> usize {
        self.call_count.load(Ordering::SeqCst)
    }
}

#[async_trait]
impl StreamTranscriber for MockStreamTranscriber {
    async fn transcribe_stream(
        &self,
        _model: &str,
        _opus: Box<dyn FrameReader + Send>,
    ) -> Result<Box<dyn SpeechStream>, ASRError> {
        self.call_count.fetch_add(1, Ordering::SeqCst);
        Ok(Box::new(MockSpeechStream {
            pattern: self.model_pattern.clone(),
            count: 2, // Return 2 speeches
        }))
    }
}

struct MockSpeechStream {
    pattern: String,
    count: usize,
}

#[async_trait]
impl SpeechStream for MockSpeechStream {
    async fn next(&mut self) -> Result<Box<dyn Speech>, SpeechError> {
        if self.count > 0 {
            self.count -= 1;
            Ok(Box::new(MockSpeech::new(&self.pattern)))
        } else {
            Err(SpeechError::Done)
        }
    }

    async fn close(&mut self) -> Result<(), SpeechError> {
        Ok(())
    }
}

struct MockSpeech {
    text: String,
    segments: usize,
}

impl MockSpeech {
    fn new(text: &str) -> Self {
        Self {
            text: text.to_string(),
            segments: 3, // Default 3 segments
        }
    }
    
    fn with_segments(text: &str, segments: usize) -> Self {
        Self {
            text: text.to_string(),
            segments,
        }
    }
}

#[async_trait]
impl Speech for MockSpeech {
    async fn next(&mut self) -> Result<Box<dyn SpeechSegment>, SpeechError> {
        if self.segments > 0 {
            self.segments -= 1;
            Ok(Box::new(MockSpeechSegment {
                text: format!("{} segment {}", self.text, self.segments),
            }))
        } else {
            Err(SpeechError::Done)
        }
    }

    async fn close(&mut self) -> Result<(), SpeechError> {
        Ok(())
    }
}

struct MockSpeechSegment {
    text: String,
}

#[async_trait]
impl SpeechSegment for MockSpeechSegment {
    fn decode(&self, _best: Format) -> Box<dyn VoiceSegment> {
        Box::new(MockVoiceSegment {
            text: self.text.clone(),
            bytes_remaining: 1024,
        })
    }

    fn transcribe(&self) -> Box<dyn tokio::io::AsyncRead + Send + Unpin> {
        Box::new(std::io::Cursor::new(self.text.as_bytes().to_vec()))
    }

    async fn close(&mut self) -> Result<(), SpeechError> {
        Ok(())
    }
}

struct MockVoiceSegment {
    text: String,
    bytes_remaining: usize,
}

#[async_trait]
impl VoiceSegment for MockVoiceSegment {
    fn format(&self) -> Format {
        Format::L16Mono16K
    }

    async fn read(&mut self, buf: &mut [u8]) -> Result<usize, VoiceError> {
        if self.bytes_remaining == 0 {
            return Err(VoiceError::Done);
        }
        let to_read = buf.len().min(self.bytes_remaining);
        buf[..to_read].fill(0);
        self.bytes_remaining -= to_read;
        Ok(to_read)
    }

    async fn close(&mut self) -> Result<(), VoiceError> {
        Ok(())
    }
}

// ============================================================================
// TTS Tests
// ============================================================================

#[tokio::test]
async fn test_tts_mux() {
    let tts = TTS::new();

    // Test not found
    let result = tts
        .synthesize(
            "voice/en",
            Box::new(tokio::io::empty()),
            Format::L16Mono16K,
        )
        .await;
    assert!(matches!(result, Err(TTSError::NotFound(_))));
}

#[tokio::test]
async fn test_tts_handle_and_synthesize() {
    let tts = TTS::new();
    let synth = Arc::new(MockSynthesizer::new("voice/en-US"));

    // Register handler
    tts.handle("voice/en-US", synth.clone()).await.unwrap();

    // Test exact match
    let result = tts
        .synthesize(
            "voice/en-US",
            Box::new(tokio::io::empty()),
            Format::L16Mono16K,
        )
        .await;
    assert!(result.is_ok());
    assert_eq!(synth.call_count(), 1);
}

#[tokio::test]
async fn test_tts_wildcard_pattern() {
    let tts = TTS::new();
    let synth = Arc::new(MockSynthesizer::new("voice/+"));

    // Register wildcard pattern
    tts.handle("voice/+", synth.clone()).await.unwrap();

    // Test wildcard match
    let result = tts
        .synthesize(
            "voice/en-US",
            Box::new(tokio::io::empty()),
            Format::L16Mono16K,
        )
        .await;
    assert!(result.is_ok());
    assert_eq!(synth.call_count(), 1);

    // Another wildcard match
    let result = tts
        .synthesize(
            "voice/zh-CN",
            Box::new(tokio::io::empty()),
            Format::L16Mono16K,
        )
        .await;
    assert!(result.is_ok());
    assert_eq!(synth.call_count(), 2);

    // Non-matching pattern
    let result = tts
        .synthesize(
            "other/path",
            Box::new(tokio::io::empty()),
            Format::L16Mono16K,
        )
        .await;
    assert!(matches!(result, Err(TTSError::NotFound(_))));
}

#[tokio::test]
async fn test_tts_multi_level_wildcard() {
    let tts = TTS::new();
    let synth = Arc::new(MockSynthesizer::new("voice/#"));

    // Register multi-level wildcard
    tts.handle("voice/#", synth.clone()).await.unwrap();

    // Test various depths
    for path in &["voice/en", "voice/en/US", "voice/en/US/male", "voice/zh/CN/female/standard"] {
        let result = tts
            .synthesize(path, Box::new(tokio::io::empty()), Format::L16Mono16K)
            .await;
        assert!(result.is_ok(), "path {} should match", path);
    }
    assert_eq!(synth.call_count(), 4);
}

#[tokio::test]
async fn test_tts_multiple_handlers() {
    let tts = TTS::new();
    let synth_en = Arc::new(MockSynthesizer::new("voice/en"));
    let synth_zh = Arc::new(MockSynthesizer::new("voice/zh"));

    // Register multiple handlers
    tts.handle("voice/en", synth_en.clone()).await.unwrap();
    tts.handle("voice/zh", synth_zh.clone()).await.unwrap();

    // Test routing to correct handler
    tts.synthesize("voice/en", Box::new(tokio::io::empty()), Format::L16Mono16K)
        .await
        .unwrap();
    assert_eq!(synth_en.call_count(), 1);
    assert_eq!(synth_zh.call_count(), 0);

    tts.synthesize("voice/zh", Box::new(tokio::io::empty()), Format::L16Mono16K)
        .await
        .unwrap();
    assert_eq!(synth_en.call_count(), 1);
    assert_eq!(synth_zh.call_count(), 1);
}

// ============================================================================
// ASR Tests
// ============================================================================

#[tokio::test]
async fn test_asr_mux() {
    let asr = ASR::new();

    // Test not found
    let result = asr
        .transcribe_stream("model/en", Box::new(MockFrameReader::new(0)))
        .await;
    assert!(matches!(result, Err(ASRError::NotFound(_))));
}

#[tokio::test]
async fn test_asr_handle_and_transcribe() {
    let asr = ASR::new();
    let transcriber = Arc::new(MockStreamTranscriber::new("model/en-US"));

    // Register handler
    asr.handle("model/en-US", transcriber.clone()).await.unwrap();

    // Test exact match
    let result = asr
        .transcribe_stream("model/en-US", Box::new(MockFrameReader::new(5)))
        .await;
    assert!(result.is_ok());
    assert_eq!(transcriber.call_count(), 1);
}

#[tokio::test]
async fn test_asr_wildcard_pattern() {
    let asr = ASR::new();
    let transcriber = Arc::new(MockStreamTranscriber::new("model/+"));

    // Register wildcard pattern
    asr.handle("model/+", transcriber.clone()).await.unwrap();

    // Test wildcard match
    let result = asr
        .transcribe_stream("model/en-US", Box::new(MockFrameReader::new(5)))
        .await;
    assert!(result.is_ok());
    assert_eq!(transcriber.call_count(), 1);

    // Another wildcard match
    let result = asr
        .transcribe_stream("model/zh-CN", Box::new(MockFrameReader::new(5)))
        .await;
    assert!(result.is_ok());
    assert_eq!(transcriber.call_count(), 2);
}

#[tokio::test]
async fn test_asr_transcribe_with_collector() {
    let asr = ASR::new();
    let transcriber = Arc::new(MockStreamTranscriber::new("model/en"));

    asr.handle("model/en", transcriber.clone()).await.unwrap();

    // transcribe() should use SpeechCollector internally
    let mut speech = asr
        .transcribe("model/en", Box::new(MockFrameReader::new(5)))
        .await
        .unwrap();

    // Should get all segments from the stream
    let mut count = 0;
    loop {
        match speech.next().await {
            Ok(_) => count += 1,
            Err(SpeechError::Done) => break,
            Err(e) => panic!("unexpected error: {}", e),
        }
    }
    // 2 speeches × 3 segments each = 6 total
    assert_eq!(count, 6);
}

// ============================================================================
// Segmenter Tests - Table Driven
// ============================================================================

#[test]
fn test_default_segmenter_config() {
    let seg = DefaultSentenceSegmenter::default();
    assert_eq!(seg.max_chars_per_segment, 256);

    let seg = DefaultSentenceSegmenter::new(100);
    assert_eq!(seg.max_chars_per_segment, 100);
}

struct SegmentTestCase {
    name: &'static str,
    input: &'static str,
    expected: Vec<&'static str>,
}

#[tokio::test]
async fn test_segmenter_table_driven() {
    let cases = vec![
        SegmentTestCase {
            name: "simple sentences",
            input: "Hello. World. How are you?",
            expected: vec!["Hello.", " World.", " How are you?"],
        },
        SegmentTestCase {
            name: "chinese punctuation",
            input: "你好。世界！再见？",
            expected: vec!["你好。", "世界！", "再见？"],
        },
        SegmentTestCase {
            name: "preserve decimals",
            input: "The value is 3.14. Please check.",
            expected: vec!["The value is 3.14.", " Please check."],
        },
        SegmentTestCase {
            name: "preserve time",
            input: "Meeting at 10:30. See you there.",
            expected: vec!["Meeting at 10:30.", " See you there."],
        },
        SegmentTestCase {
            name: "newlines",
            input: "Line one\nLine two\nLine three",
            expected: vec!["Line one\n", "Line two\n", "Line three"],
        },
        SegmentTestCase {
            name: "mixed punctuation",
            input: "Hi! How are you? I'm fine。Thanks～",
            expected: vec!["Hi!", " How are you?", " I'm fine。", "Thanks～"],
        },
        SegmentTestCase {
            name: "semicolon",
            input: "First; second; third",
            expected: vec!["First;", " second;", " third"],
        },
        SegmentTestCase {
            name: "ellipsis",
            input: "Wait… really?",
            expected: vec!["Wait…", " really?"],
        },
        SegmentTestCase {
            name: "empty input",
            input: "",
            expected: vec![],
        },
        SegmentTestCase {
            name: "no boundaries",
            input: "Hello world",
            expected: vec!["Hello world"],
        },
        SegmentTestCase {
            name: "chinese comma",
            input: "我喜欢苹果，香蕉，橙子。",
            expected: vec!["我喜欢苹果，", "香蕉，", "橙子。"],
        },
    ];

    for case in cases {
        let segmenter = DefaultSentenceSegmenter::default();
        let reader = std::io::Cursor::new(case.input.as_bytes().to_vec());
        let mut iter = segmenter.segment(Box::new(reader)).await.unwrap();

        let mut results = Vec::new();
        loop {
            match iter.next().await {
                Ok(s) => results.push(s),
                Err(SegmentError::Done) => break,
                Err(e) => panic!("unexpected error in case '{}': {}", case.name, e),
            }
        }

        assert_eq!(
            results, case.expected,
            "case '{}' failed:\n  input: {:?}\n  expected: {:?}\n  got: {:?}",
            case.name, case.input, case.expected, results
        );
    }
}

#[tokio::test]
async fn test_segmenter_max_chars() {
    let segmenter = DefaultSentenceSegmenter::new(10);
    let long_text = "ThisIsAVeryLongTextWithoutAnyPunctuationMarks";
    let reader = std::io::Cursor::new(long_text.as_bytes().to_vec());
    let mut iter = segmenter.segment(Box::new(reader)).await.unwrap();

    let mut results = Vec::new();
    loop {
        match iter.next().await {
            Ok(s) => results.push(s),
            Err(SegmentError::Done) => break,
            Err(e) => panic!("unexpected error: {}", e),
        }
    }

    // Each segment should be at most 10 characters
    for (i, seg) in results.iter().enumerate() {
        assert!(
            seg.chars().count() <= 10,
            "segment {} exceeds max chars: {:?} (len {})",
            i,
            seg,
            seg.chars().count()
        );
    }

    // All segments together should equal the original
    let combined: String = results.iter().map(|s| s.as_str()).collect();
    assert_eq!(combined, long_text);
}

// ============================================================================
// SpeechCollector Tests
// ============================================================================

#[tokio::test]
async fn test_speech_collector_empty_stream() {
    struct EmptyStream;

    #[async_trait]
    impl SpeechStream for EmptyStream {
        async fn next(&mut self) -> Result<Box<dyn Speech>, SpeechError> {
            Err(SpeechError::Done)
        }

        async fn close(&mut self) -> Result<(), SpeechError> {
            Ok(())
        }
    }

    let mut collector = SpeechCollector::new(Box::new(EmptyStream));
    let result = collector.next().await;
    assert!(matches!(result, Err(SpeechError::Done)));
}

#[tokio::test]
async fn test_speech_collector_single_speech() {
    struct SingleSpeechStream {
        done: bool,
    }

    #[async_trait]
    impl SpeechStream for SingleSpeechStream {
        async fn next(&mut self) -> Result<Box<dyn Speech>, SpeechError> {
            if self.done {
                Err(SpeechError::Done)
            } else {
                self.done = true;
                Ok(Box::new(MockSpeech::with_segments("single", 2)))
            }
        }

        async fn close(&mut self) -> Result<(), SpeechError> {
            Ok(())
        }
    }

    let mut collector = SpeechCollector::new(Box::new(SingleSpeechStream { done: false }));

    // Should get 2 segments
    collector.next().await.unwrap();
    collector.next().await.unwrap();

    // Should be done
    let result = collector.next().await;
    assert!(matches!(result, Err(SpeechError::Done)));
}

#[tokio::test]
async fn test_speech_collector_error_propagation() {
    struct ErrorStream {
        count: usize,
    }

    #[async_trait]
    impl SpeechStream for ErrorStream {
        async fn next(&mut self) -> Result<Box<dyn Speech>, SpeechError> {
            if self.count > 0 {
                self.count -= 1;
                Ok(Box::new(MockSpeech::with_segments("ok", 1)))
            } else {
                Err(SpeechError::Other("stream error".to_string()))
            }
        }

        async fn close(&mut self) -> Result<(), SpeechError> {
            Ok(())
        }
    }

    let mut collector = SpeechCollector::new(Box::new(ErrorStream { count: 1 }));

    // First segment should succeed
    collector.next().await.unwrap();

    // After first speech exhausts, error from stream should propagate
    let result = collector.next().await;
    assert!(matches!(result, Err(SpeechError::Other(_))));
}

// ============================================================================
// SpeechIter Tests
// ============================================================================

#[tokio::test]
async fn test_speech_iter() {
    let mut speech = MockSpeech::with_segments("test", 3);
    let mut iter = SpeechIter::new(&mut speech);

    // Should get 3 segments
    let mut count = 0;
    while let Some(result) = iter.next_segment().await {
        result.unwrap();
        count += 1;
    }
    assert_eq!(count, 3);

    // Subsequent calls should return None
    assert!(iter.next_segment().await.is_none());
}

#[tokio::test]
async fn test_speech_iter_empty() {
    let mut speech = MockSpeech::with_segments("empty", 0);
    let mut iter = SpeechIter::new(&mut speech);

    // Should immediately return None
    assert!(iter.next_segment().await.is_none());
}

// ============================================================================
// VoiceSegment Tests
// ============================================================================

#[tokio::test]
async fn test_voice_segment_read() {
    let mut segment = MockVoiceSegment {
        text: "test".to_string(),
        bytes_remaining: 100,
    };

    let mut buf = [0u8; 50];

    // First read
    let n = segment.read(&mut buf).await.unwrap();
    assert_eq!(n, 50);

    // Second read
    let n = segment.read(&mut buf).await.unwrap();
    assert_eq!(n, 50);

    // Third read should return Done
    let result = segment.read(&mut buf).await;
    assert!(matches!(result, Err(VoiceError::Done)));
}

#[tokio::test]
async fn test_speech_segment_transcribe() {
    let segment = MockSpeechSegment {
        text: "Hello World".to_string(),
    };

    let mut reader = segment.transcribe();
    let mut buf = Vec::new();
    tokio::io::AsyncReadExt::read_to_end(&mut reader, &mut buf)
        .await
        .unwrap();

    assert_eq!(String::from_utf8(buf).unwrap(), "Hello World");
}

// ============================================================================
// Voice Tests
// ============================================================================

struct MockVoice {
    segments: usize,
}

#[async_trait]
impl Voice for MockVoice {
    async fn next(&mut self) -> Result<Box<dyn VoiceSegment>, VoiceError> {
        if self.segments > 0 {
            self.segments -= 1;
            Ok(Box::new(MockVoiceSegment {
                text: "segment".to_string(),
                bytes_remaining: 100,
            }))
        } else {
            Err(VoiceError::Done)
        }
    }

    async fn close(&mut self) -> Result<(), VoiceError> {
        Ok(())
    }
}

#[tokio::test]
async fn test_voice_iteration() {
    let mut voice = MockVoice { segments: 3 };

    let mut count = 0;
    loop {
        match voice.next().await {
            Ok(_) => count += 1,
            Err(VoiceError::Done) => break,
            Err(e) => panic!("unexpected error: {}", e),
        }
    }
    assert_eq!(count, 3);
}

// ============================================================================
// Format Tests
// ============================================================================

#[test]
fn test_speech_segment_decode_format() {
    let segment = MockSpeechSegment {
        text: "test".to_string(),
    };

    let voice_segment = segment.decode(Format::L16Mono16K);
    assert_eq!(voice_segment.format(), Format::L16Mono16K);
}
