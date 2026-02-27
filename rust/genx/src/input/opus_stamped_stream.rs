use std::collections::VecDeque;
use std::io;
use std::time::Duration;

use anyhow::anyhow;
use async_trait::async_trait;

use crate::error::{GenxError, Usage};
use crate::stream::{Stream, StreamResult};
use crate::types::{MessageChunk, Role};

use super::InputError;
use super::jitter_buffer::JitterBuffer;
use super::opus::{EpochMillis, OPUS_SILENCE_20MS, OpusFrame, StampedFrame, parse_stamped};

/// Stamped Opus 输入读取接口。
pub trait StampedOpusReader: Send + Sync {
    fn read_stamped(&mut self) -> io::Result<Vec<u8>>;
}

/// 实时流配置。
#[derive(Debug, Clone)]
pub struct RealtimeConfig {
    pub role: Role,
    pub name: String,
    /// 小 gap 的补静音上限。
    pub max_loss: Duration,
    /// 抖动缓冲容量（按帧数）。
    pub jitter_buffer_size: usize,
    /// 迟到窗口：`stamp + late_window < last_emitted_end` 的帧将被丢弃。
    pub late_window: Duration,
    /// 启动缓冲帧数：在非 EOF 情况下，jitter 至少积累该数量帧才出队。
    pub min_ready_frames: usize,
}

impl Default for RealtimeConfig {
    fn default() -> Self {
        Self {
            role: Role::User,
            name: String::new(),
            max_loss: Duration::from_secs(5),
            jitter_buffer_size: 100,
            late_window: Duration::from_millis(100),
            min_ready_frames: 1,
        }
    }
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct StampedOpusStats {
    pub invalid_frames: u64,
    pub dropped_late_frames: u64,
    pub inserted_silence_frames: u64,
    pub unsupported_version_frames: u64,
    pub truncated_frames: u64,
}

/// 基于 stamped 输入的 Opus 流。
pub struct StampedOpusStream<R: StampedOpusReader> {
    reader: R,
    cfg: RealtimeConfig,
    jitter: JitterBuffer<EpochMillis, StampedFrame>,
    out: VecDeque<MessageChunk>,
    stats: StampedOpusStats,
    last_emitted_end: Option<EpochMillis>,
    eof: bool,
    closed: bool,
    close_error: Option<GenxError>,
}

impl<R: StampedOpusReader> StampedOpusStream<R> {
    pub fn new(reader: R, cfg: RealtimeConfig) -> Self {
        let cfg = RealtimeConfig {
            role: cfg.role,
            name: cfg.name,
            max_loss: if cfg.max_loss.is_zero() {
                Duration::from_secs(5)
            } else {
                cfg.max_loss
            },
            jitter_buffer_size: if cfg.jitter_buffer_size == 0 {
                100
            } else {
                cfg.jitter_buffer_size
            },
            late_window: if cfg.late_window.is_zero() {
                Duration::from_millis(100)
            } else {
                cfg.late_window
            },
            min_ready_frames: cfg.min_ready_frames.max(1),
        };
        Self {
            jitter: JitterBuffer::new(cfg.jitter_buffer_size),
            reader,
            cfg,
            out: VecDeque::new(),
            stats: StampedOpusStats::default(),
            last_emitted_end: None,
            eof: false,
            closed: false,
            close_error: None,
        }
    }

    pub fn stats(&self) -> &StampedOpusStats {
        &self.stats
    }

    fn ensure_open(&self) -> Result<(), GenxError> {
        if let Some(err) = &self.close_error {
            return Err(GenxError::Other(anyhow!(err.to_string())));
        }
        if self.closed {
            return Err(GenxError::Other(anyhow!(
                InputError::StreamClosed.to_string()
            )));
        }
        Ok(())
    }

    fn late_window_ms(&self) -> i64 {
        self.cfg.late_window.as_millis() as i64
    }

    fn is_late_frame(&self, stamp: EpochMillis) -> bool {
        if let Some(last_end) = self.last_emitted_end {
            stamp + self.late_window_ms() < last_end
        } else {
            false
        }
    }

    fn read_one(&mut self) -> Result<(), GenxError> {
        match self.reader.read_stamped() {
            Ok(data) => match parse_stamped(&data) {
                Ok((frame, stamp)) => {
                    if self.is_late_frame(stamp) {
                        self.stats.dropped_late_frames =
                            self.stats.dropped_late_frames.saturating_add(1);
                        return Ok(());
                    }
                    self.jitter.push(StampedFrame { frame, stamp });
                    Ok(())
                }
                Err(err) => {
                    self.stats.invalid_frames = self.stats.invalid_frames.saturating_add(1);
                    match err {
                        InputError::UnsupportedVersion(_) => {
                            self.stats.unsupported_version_frames =
                                self.stats.unsupported_version_frames.saturating_add(1);
                        }
                        InputError::TruncatedFrame => {
                            self.stats.truncated_frames =
                                self.stats.truncated_frames.saturating_add(1);
                        }
                        _ => {}
                    }
                    Ok(())
                }
            },
            Err(e) if e.kind() == io::ErrorKind::UnexpectedEof => {
                self.eof = true;
                Ok(())
            }
            Err(e) => Err(GenxError::Other(anyhow!(e))),
        }
    }

    fn try_emit_one(&mut self) -> bool {
        if self.jitter.is_empty() {
            return false;
        }
        // 增量策略：未 EOF 时按可配置 startup buffer 出队。
        if !self.eof && self.jitter.len() < self.cfg.min_ready_frames {
            return false;
        }

        if let Some(sf) = self.jitter.pop() {
            self.emit_with_gap(sf);
            return true;
        }
        false
    }

    fn emit_with_gap(&mut self, sf: StampedFrame) {
        if let Some(prev_end) = self.last_emitted_end {
            let gap_ms = sf.stamp - prev_end;
            if gap_ms > 0 {
                let gap = Duration::from_millis(gap_ms as u64);
                if gap <= self.cfg.max_loss {
                    self.emit_silence(gap);
                }
            }
        }

        let dur = sf.frame.duration();
        self.out.push_back(self.frame_to_chunk(sf.frame));
        self.last_emitted_end = Some(sf.stamp + dur.as_millis() as i64);
    }

    fn emit_silence(&mut self, mut gap: Duration) {
        let silence_dur = Duration::from_millis(20);
        while gap >= silence_dur {
            self.stats.inserted_silence_frames =
                self.stats.inserted_silence_frames.saturating_add(1);
            self.out
                .push_back(self.frame_to_chunk(OpusFrame(OPUS_SILENCE_20MS.to_vec())));
            gap -= silence_dur;
        }
    }

    fn frame_to_chunk(&self, frame: OpusFrame) -> MessageChunk {
        MessageChunk {
            role: self.cfg.role,
            name: if self.cfg.name.is_empty() {
                None
            } else {
                Some(self.cfg.name.clone())
            },
            part: Some(crate::types::Part::blob("audio/opus", frame.0)),
            tool_call: None,
            ctrl: None,
        }
    }
}

#[async_trait]
impl<R: StampedOpusReader> Stream for StampedOpusStream<R> {
    async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
        self.ensure_open()?;

        loop {
            if let Some(chunk) = self.out.pop_front() {
                return Ok(Some(chunk));
            }

            if self.try_emit_one() {
                continue;
            }

            if self.eof {
                return Ok(None);
            }

            self.read_one()?;
        }
    }

    fn result(&self) -> Option<StreamResult> {
        (self.eof && self.jitter.is_empty() && self.out.is_empty())
            .then(|| StreamResult::done(Usage::default()))
    }

    async fn close(&mut self) -> Result<(), GenxError> {
        self.closed = true;
        Ok(())
    }

    async fn close_with_error(&mut self, error: GenxError) -> Result<(), GenxError> {
        self.closed = true;
        self.close_error = Some(error);
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use std::fs;
    use std::path::{Path, PathBuf};

    use super::super::opus::make_stamped;
    use super::*;
    use serde::Deserialize;

    struct MockStampedReader {
        items: VecDeque<io::Result<Vec<u8>>>,
        read_calls: usize,
    }

    impl MockStampedReader {
        fn from_frames(frames: Vec<Vec<u8>>) -> Self {
            let mut items = VecDeque::new();
            for f in frames {
                items.push_back(Ok(f));
            }
            items.push_back(Err(io::Error::new(io::ErrorKind::UnexpectedEof, "eof")));
            Self {
                items,
                read_calls: 0,
            }
        }
    }

    impl StampedOpusReader for MockStampedReader {
        fn read_stamped(&mut self) -> io::Result<Vec<u8>> {
            self.read_calls += 1;
            self.items
                .pop_front()
                .unwrap_or_else(|| Err(io::Error::new(io::ErrorKind::UnexpectedEof, "eof")))
        }
    }

    async fn collect_payloads<R: StampedOpusReader>(
        stream: &mut StampedOpusStream<R>,
    ) -> Vec<Vec<u8>> {
        let mut out = Vec::new();
        while let Some(chunk) = stream.next().await.unwrap() {
            let blob = chunk.part.as_ref().unwrap().as_blob().unwrap();
            out.push(blob.data.clone());
        }
        out
    }

    #[tokio::test]
    async fn t22_incremental_output_not_wait_eof() {
        let base = 1_700_000_000_000i64;
        let frames = vec![
            make_stamped(&OpusFrame(vec![0xf8, 0x11]), base),
            make_stamped(&OpusFrame(vec![0xf8, 0x22]), base + 20),
            make_stamped(&OpusFrame(vec![0xf8, 0x33]), base + 40),
        ];
        let reader = MockStampedReader::from_frames(frames);
        let mut s = StampedOpusStream::new(
            reader,
            RealtimeConfig {
                min_ready_frames: 2,
                ..RealtimeConfig::default()
            },
        );

        let first = s.next().await.unwrap();
        assert!(first.is_some());
    }

    #[tokio::test]
    async fn t22_stamped_out_of_order_reorder() {
        let base = 1_700_000_000_000i64;
        let frames = vec![
            make_stamped(&OpusFrame(vec![0xf8, 0x33]), base + 40),
            make_stamped(&OpusFrame(vec![0xf8, 0x11]), base),
            make_stamped(&OpusFrame(vec![0xf8, 0x22]), base + 20),
        ];
        let reader = MockStampedReader::from_frames(frames);
        let mut s = StampedOpusStream::new(
            reader,
            RealtimeConfig {
                min_ready_frames: 2,
                ..RealtimeConfig::default()
            },
        );
        let got = collect_payloads(&mut s).await;
        assert_eq!(
            got,
            vec![vec![0xf8, 0x11], vec![0xf8, 0x22], vec![0xf8, 0x33]]
        );
    }

    #[tokio::test]
    async fn t22_duplicate_timestamp_keep_arrival_order() {
        let base = 1_700_000_000_000i64;
        let frames = vec![
            make_stamped(&OpusFrame(vec![0xf8, 0x01]), base),
            make_stamped(&OpusFrame(vec![0xf8, 0x02]), base),
            make_stamped(&OpusFrame(vec![0xf8, 0x03]), base + 20),
        ];
        let reader = MockStampedReader::from_frames(frames);
        let mut s = StampedOpusStream::new(reader, RealtimeConfig::default());
        let got = collect_payloads(&mut s).await;
        assert_eq!(got[0], vec![0xf8, 0x01]);
        assert_eq!(got[1], vec![0xf8, 0x02]);
    }

    #[tokio::test]
    async fn t22_stamped_emit_silence_for_small_gap() {
        let base = 1_700_000_000_000i64;
        let frame = OpusFrame(OPUS_SILENCE_20MS.to_vec());
        let frames = vec![make_stamped(&frame, base), make_stamped(&frame, base + 60)];
        let reader = MockStampedReader::from_frames(frames);
        let mut s = StampedOpusStream::new(reader, RealtimeConfig::default());
        let got = collect_payloads(&mut s).await;
        assert_eq!(got.len(), 4);
        assert_eq!(s.stats().inserted_silence_frames, 2);
    }

    #[tokio::test]
    async fn t22_stamped_resync_for_large_gap() {
        let base = 1_700_000_000_000i64;
        let frame = OpusFrame(OPUS_SILENCE_20MS.to_vec());
        let frames = vec![make_stamped(&frame, base), make_stamped(&frame, base + 220)];
        let reader = MockStampedReader::from_frames(frames);
        let mut s = StampedOpusStream::new(
            reader,
            RealtimeConfig {
                max_loss: Duration::from_millis(100),
                ..RealtimeConfig::default()
            },
        );
        let got = collect_payloads(&mut s).await;
        assert_eq!(got.len(), 2);
        assert_eq!(s.stats().inserted_silence_frames, 0);
    }

    #[tokio::test]
    async fn t22_drop_late_frame_with_window() {
        let base = 1_700_000_000_000i64;
        let frames = vec![
            make_stamped(&OpusFrame(vec![0xf8, 0x10]), base + 100),
            make_stamped(&OpusFrame(vec![0xf8, 0x20]), base + 120),
            make_stamped(&OpusFrame(vec![0xf8, 0x99]), base + 80),
        ];
        let reader = MockStampedReader::from_frames(frames);
        let mut s = StampedOpusStream::new(
            reader,
            RealtimeConfig {
                late_window: Duration::from_millis(10),
                ..RealtimeConfig::default()
            },
        );

        let got = collect_payloads(&mut s).await;
        assert_eq!(got, vec![vec![0xf8, 0x10], vec![0xf8, 0x20]]);
        assert_eq!(s.stats().dropped_late_frames, 1);
    }

    #[tokio::test]
    async fn t22_skip_invalid_frame_and_classify() {
        let base = 1_700_000_000_000i64;
        let frame = OpusFrame(OPUS_SILENCE_20MS.to_vec());
        let mut bad_version = make_stamped(&frame, base + 20);
        bad_version[0] = 0x7f;
        let frames = vec![
            make_stamped(&frame, base),
            vec![0x01, 0x02],
            bad_version,
            make_stamped(&frame, base + 40),
        ];
        let reader = MockStampedReader::from_frames(frames);
        let mut s = StampedOpusStream::new(reader, RealtimeConfig::default());
        let got = collect_payloads(&mut s).await;
        assert_eq!(got.len(), 3);
        assert_eq!(s.stats().invalid_frames, 2);
        assert_eq!(s.stats().truncated_frames, 1);
        assert_eq!(s.stats().unsupported_version_frames, 1);
    }

    #[tokio::test]
    async fn t22_u56_timestamp_boundary_integration() {
        let max_u56 = ((1u64 << 56) - 1) as i64;
        let frame = OpusFrame(OPUS_SILENCE_20MS.to_vec());
        let frames = vec![
            make_stamped(&frame, max_u56 - 20),
            make_stamped(&frame, max_u56),
        ];
        let reader = MockStampedReader::from_frames(frames);
        let mut s = StampedOpusStream::new(reader, RealtimeConfig::default());
        let got = collect_payloads(&mut s).await;
        assert_eq!(got.len(), 2);
    }

    #[test]
    fn t22_default_config_values() {
        let cfg = RealtimeConfig::default();
        assert_eq!(cfg.role, Role::User);
        assert_eq!(cfg.max_loss, Duration::from_secs(5));
        assert_eq!(cfg.jitter_buffer_size, 100);
        assert_eq!(cfg.late_window, Duration::from_millis(100));
        assert_eq!(cfg.min_ready_frames, 1);
    }

    #[derive(Debug, Deserialize)]
    struct ParityFixture {
        cases: Vec<ParityCase>,
    }

    #[derive(Debug, Deserialize)]
    struct ParityCase {
        name: String,
        max_loss_ms: i64,
        inputs: Vec<ParityInput>,
        expected_opus_hex: Vec<String>,
    }

    #[derive(Debug, Deserialize)]
    struct ParityInput {
        kind: String,
        stamp_ms: Option<i64>,
        opus_hex: Option<String>,
        raw_hex: Option<String>,
    }

    #[derive(Debug)]
    struct ParsedFrame {
        seq: usize,
        stamp: i64,
        frame: OpusFrame,
    }

    #[test]
    fn t22_parity_fixture_rust_reference() {
        let text = load_parity_fixture();
        let fixture: ParityFixture =
            serde_json::from_str(&text).expect("parse parity fixture json");

        for case in fixture.cases {
            let mut raw_inputs: Vec<Vec<u8>> = Vec::new();
            for input in case.inputs {
                match input.kind.as_str() {
                    "stamped" => {
                        let frame_hex = input.opus_hex.expect("stamped requires opus_hex");
                        let stamp = input.stamp_ms.expect("stamped requires stamp_ms");
                        let frame =
                            OpusFrame(hex::decode(frame_hex).expect("decode stamped opus_hex"));
                        raw_inputs.push(make_stamped(&frame, stamp));
                    }
                    "raw" => {
                        let raw_hex = input.raw_hex.expect("raw requires raw_hex");
                        raw_inputs.push(hex::decode(raw_hex).expect("decode raw_hex"));
                    }
                    other => panic!("unknown input kind: {other}"),
                }
            }

            let got =
                run_reference_pipeline(&raw_inputs, Duration::from_millis(case.max_loss_ms as u64));
            let want: Vec<Vec<u8>> = case
                .expected_opus_hex
                .iter()
                .map(|s| hex::decode(s).expect("decode expected_opus_hex"))
                .collect();

            assert_eq!(got, want, "parity fixture mismatch on case {}", case.name);
        }
    }

    fn load_parity_fixture() -> String {
        let mut candidates: Vec<PathBuf> = Vec::new();

        if let (Ok(srcdir), Ok(workspace)) = (
            std::env::var("TEST_SRCDIR"),
            std::env::var("TEST_WORKSPACE"),
        ) {
            candidates.push(
                Path::new(&srcdir)
                    .join(&workspace)
                    .join("go/pkg/genx/input/opus/testdata/parity_cases.json"),
            );
            candidates
                .push(Path::new(&srcdir).join("go/pkg/genx/input/opus/testdata/parity_cases.json"));
        }

        candidates.push(
            Path::new(env!("CARGO_MANIFEST_DIR"))
                .join("../../go/pkg/genx/input/opus/testdata/parity_cases.json"),
        );

        for path in &candidates {
            if let Ok(text) = fs::read_to_string(path) {
                return text;
            }
        }

        panic!(
            "cannot load parity fixture; checked paths: {:?}",
            candidates
        );
    }

    fn run_reference_pipeline(raw: &[Vec<u8>], max_loss: Duration) -> Vec<Vec<u8>> {
        let mut parsed: Vec<ParsedFrame> = Vec::new();
        for (seq, item) in raw.iter().enumerate() {
            if let Ok((frame, stamp)) = parse_stamped(item) {
                parsed.push(ParsedFrame { seq, stamp, frame });
            }
        }

        parsed.sort_by(|a, b| a.stamp.cmp(&b.stamp).then(a.seq.cmp(&b.seq)));

        let mut out: Vec<Vec<u8>> = Vec::new();
        let mut last_end: Option<i64> = None;

        for pf in parsed {
            if let Some(prev_end) = last_end {
                let gap_ms = pf.stamp - prev_end;
                if gap_ms > 0 {
                    let mut gap = Duration::from_millis(gap_ms as u64);
                    if gap <= max_loss {
                        while gap >= Duration::from_millis(20) {
                            out.push(OPUS_SILENCE_20MS.to_vec());
                            gap -= Duration::from_millis(20);
                        }
                    }
                }
            }

            let dur = pf.frame.duration();
            out.push(pf.frame.0);
            last_end = Some(pf.stamp + dur.as_millis() as i64);
        }

        out
    }
}
