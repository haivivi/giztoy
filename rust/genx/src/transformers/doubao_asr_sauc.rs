use std::sync::Arc;

use async_trait::async_trait;
use giztoy_doubaospeech as doubaospeech;
use tokio::sync::{mpsc, Mutex};

use crate::error::GenxError;
use crate::stream::{Stream, StreamResult};
use crate::stream_id::new_stream_id;
use crate::transformer::Transformer;
use crate::types::{MessageChunk, Part, StreamCtrl};

pub struct DoubaoAsrSaucTransformer {
    backend: Arc<dyn AsrBackend>,
}

impl DoubaoAsrSaucTransformer {
    pub fn new(client: Arc<doubaospeech::Client>) -> Self {
        Self {
            backend: Arc::new(DoubaoAsrSaucBackend {
                client,
                format: "ogg".to_string(),
                sample_rate: 16000,
                channels: 1,
                bits: 16,
                language: Some("zh-CN".to_string()),
                enable_itn: true,
                enable_punc: true,
                hotwords: Vec::new(),
            }),
        }
    }

    pub(crate) fn new_with_backend(backend: Arc<dyn AsrBackend>) -> Self {
        Self { backend }
    }
}

#[async_trait]
impl Transformer for DoubaoAsrSaucTransformer {
    async fn transform(
        &self,
        _pattern: &str,
        input: Box<dyn Stream>,
    ) -> Result<Box<dyn Stream>, GenxError> {
        let (tx, rx) = mpsc::channel(128);
        let backend = Arc::clone(&self.backend);
        tokio::spawn(async move {
            run_asr_transform_loop(backend, input, tx).await;
        });
        Ok(Box::new(AsrChannelStream { rx }))
    }
}

#[async_trait]
pub(crate) trait AsrBackend: Send + Sync {
    async fn open_session(&self) -> Result<Box<dyn AsrSession>, GenxError>;
}

#[async_trait]
pub(crate) trait AsrSession: Send + Sync {
    async fn send_audio(&mut self, audio: Vec<u8>, is_last: bool) -> Result<(), GenxError>;
    async fn recv(&mut self) -> Result<Option<AsrResult>, GenxError>;
    async fn close(&mut self) -> Result<(), GenxError>;
}

#[derive(Clone, Debug, Default)]
pub(crate) struct AsrResult {
    pub text: String,
    pub utterances: Vec<AsrUtterance>,
    pub is_final: bool,
}

#[derive(Clone, Debug, Default)]
pub(crate) struct AsrUtterance {
    pub text: String,
    pub end_time: i32,
    pub definite: bool,
}

struct DoubaoAsrSaucBackend {
    client: Arc<doubaospeech::Client>,
    format: String,
    sample_rate: i32,
    channels: i32,
    bits: i32,
    language: Option<String>,
    enable_itn: bool,
    enable_punc: bool,
    hotwords: Vec<String>,
}

#[async_trait]
impl AsrBackend for DoubaoAsrSaucBackend {
    async fn open_session(&self) -> Result<Box<dyn AsrSession>, GenxError> {
        let cfg = doubaospeech::AsrV2Config {
            format: self.format.clone(),
            sample_rate: self.sample_rate,
            channels: self.channels,
            bits: self.bits,
            language: self.language.clone(),
            enable_itn: self.enable_itn,
            enable_punc: self.enable_punc,
            hotwords: self.hotwords.clone(),
            resource_id: Some(doubaospeech::RESOURCE_ASR_STREAM.to_string()),
            ..Default::default()
        };

        let session = self
            .client
            .asr_v2()
            .open_stream_session(&cfg)
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("doubao asr open session: {}", e)))?;

        Ok(Box::new(DoubaoAsrSessionAdapter {
            inner: Mutex::new(session),
        }))
    }
}

struct DoubaoAsrSessionAdapter {
    inner: Mutex<doubaospeech::AsrV2Session>,
}

#[async_trait]
impl AsrSession for DoubaoAsrSessionAdapter {
    async fn send_audio(&mut self, audio: Vec<u8>, is_last: bool) -> Result<(), GenxError> {
        let guard = self.inner.get_mut();
        guard
            .send_audio(&audio, is_last)
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("doubao asr send audio: {}", e)))
    }

    async fn recv(&mut self) -> Result<Option<AsrResult>, GenxError> {
        let guard = self.inner.get_mut();
        match guard.recv().await {
            Some(Ok(v)) => Ok(Some(AsrResult {
                text: v.text,
                utterances: v
                    .utterances
                    .into_iter()
                    .map(|u| AsrUtterance {
                        text: u.text,
                        end_time: u.end_time,
                        definite: u.definite,
                    })
                    .collect(),
                is_final: v.is_final,
            })),
            Some(Err(e)) => Err(GenxError::Other(anyhow::anyhow!(
                "doubao asr recv result: {}",
                e
            ))),
            None => Ok(None),
        }
    }

    async fn close(&mut self) -> Result<(), GenxError> {
        let guard = self.inner.get_mut();
        guard
            .close()
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("doubao asr close session: {}", e)))
    }
}

async fn run_asr_transform_loop(
    backend: Arc<dyn AsrBackend>,
    mut input: Box<dyn Stream>,
    tx: mpsc::Sender<Result<MessageChunk, GenxError>>,
) {
    let mut session: Option<Box<dyn AsrSession>> = None;
    let mut last_meta: Option<MessageChunk> = None;
    let mut current_stream_id = String::new();

    loop {
        match input.next().await {
            Ok(Some(chunk)) => {
                if let Some(ctrl) = &chunk.ctrl {
                    if !ctrl.stream_id.is_empty() {
                        current_stream_id = ctrl.stream_id.clone();
                    }
                }
                if current_stream_id.is_empty() {
                    current_stream_id = new_stream_id();
                }
                last_meta = Some(chunk.clone());

                if chunk.is_end_of_stream() {
                    match chunk.part.as_ref() {
                        Some(Part::Blob(blob)) if blob.mime_type.starts_with("audio/") => {
                            if !finish_session_and_emit(
                                &mut session,
                                &tx,
                                &last_meta,
                                &current_stream_id,
                                true,
                            )
                            .await
                            {
                                return;
                            }
                            current_stream_id.clear();
                        }
                        _ => {
                            let mut passthrough = chunk.clone();
                            passthrough.ctrl = Some(merge_ctrl_with_stream_id(
                                passthrough.ctrl.as_ref(),
                                &current_stream_id,
                            ));
                            if tx.send(Ok(passthrough)).await.is_err() {
                                return;
                            }
                        }
                    }
                    continue;
                }

                match chunk.part.as_ref() {
                    Some(Part::Blob(blob)) if blob.mime_type.starts_with("audio/") => {
                        if session.is_none() {
                            match backend.open_session().await {
                                Ok(s) => session = Some(s),
                                Err(e) => {
                                    let _ = tx.send(Err(e)).await;
                                    return;
                                }
                            }
                        }

                        if let Some(s) = session.as_mut()
                            && let Err(e) = s.send_audio(blob.data.clone(), false).await
                        {
                            let _ = tx.send(Err(e)).await;
                            return;
                        }
                    }
                    _ => {
                        let mut passthrough = chunk.clone();
                        passthrough.ctrl = Some(merge_ctrl_with_stream_id(
                            passthrough.ctrl.as_ref(),
                            &current_stream_id,
                        ));
                        if tx.send(Ok(passthrough)).await.is_err() {
                            return;
                        }
                    }
                }
            }
            Ok(None) => {
                let _ = finish_session_and_emit(
                    &mut session,
                    &tx,
                    &last_meta,
                    &current_stream_id,
                    false,
                )
                .await;
                return;
            }
            Err(e) => {
                let _ = tx.send(Err(e)).await;
                return;
            }
        }
    }
}

async fn finish_session_and_emit(
    session: &mut Option<Box<dyn AsrSession>>,
    tx: &mpsc::Sender<Result<MessageChunk, GenxError>>,
    last_meta: &Option<MessageChunk>,
    stream_id: &str,
    emit_text_eos: bool,
) -> bool {
    let Some(mut s) = session.take() else {
        if emit_text_eos {
            let eos = with_meta(
                last_meta,
                MessageChunk {
                    role: last_meta.as_ref().map_or(crate::types::Role::User, |c| c.role),
                    name: last_meta.as_ref().and_then(|c| c.name.clone()),
                    part: Some(Part::text("")),
                    tool_call: None,
                    ctrl: Some(StreamCtrl {
                        stream_id: stream_id.to_string(),
                        end_of_stream: true,
                        ..Default::default()
                    }),
                },
            );
            if tx.send(Ok(eos)).await.is_err() {
                return false;
            }
        }
        return true;
    };

    if let Err(e) = s.send_audio(Vec::new(), true).await {
        let _ = tx.send(Err(e)).await;
        return false;
    }

    let mut last_end_time = 0;
    loop {
        match s.recv().await {
            Ok(Some(result)) => {
                let mut emitted_any_utterance = false;
                for utt in result.utterances {
                    if utt.definite && utt.end_time > last_end_time && !utt.text.is_empty() {
                        let out = with_meta(
                            last_meta,
                            MessageChunk {
                                role: last_meta
                                    .as_ref()
                                    .map_or(crate::types::Role::User, |c| c.role),
                                name: last_meta.as_ref().and_then(|c| c.name.clone()),
                                part: Some(Part::text(utt.text)),
                                tool_call: None,
                                ctrl: Some(StreamCtrl {
                                    stream_id: stream_id.to_string(),
                                    ..Default::default()
                                }),
                            },
                        );
                        if tx.send(Ok(out)).await.is_err() {
                            return false;
                        }
                        emitted_any_utterance = true;
                        last_end_time = utt.end_time;
                    }
                }

                if !emitted_any_utterance && result.is_final && !result.text.is_empty() {
                    let out = with_meta(
                        last_meta,
                        MessageChunk {
                            role: last_meta.as_ref().map_or(crate::types::Role::User, |c| c.role),
                            name: last_meta.as_ref().and_then(|c| c.name.clone()),
                            part: Some(Part::text(result.text)),
                            tool_call: None,
                            ctrl: Some(StreamCtrl {
                                stream_id: stream_id.to_string(),
                                ..Default::default()
                            }),
                        },
                    );
                    if tx.send(Ok(out)).await.is_err() {
                        return false;
                    }
                }
            }
            Ok(None) => break,
            Err(e) => {
                let _ = tx.send(Err(e)).await;
                return false;
            }
        }
    }

    if let Err(e) = s.close().await {
        let _ = tx.send(Err(e)).await;
        return false;
    }

    if emit_text_eos {
        let eos = with_meta(
            last_meta,
            MessageChunk {
                role: last_meta.as_ref().map_or(crate::types::Role::User, |c| c.role),
                name: last_meta.as_ref().and_then(|c| c.name.clone()),
                part: Some(Part::text("")),
                tool_call: None,
                ctrl: Some(StreamCtrl {
                    stream_id: stream_id.to_string(),
                    end_of_stream: true,
                    ..Default::default()
                }),
            },
        );
        if tx.send(Ok(eos)).await.is_err() {
            return false;
        }
    }

    true
}

fn with_meta(meta: &Option<MessageChunk>, mut chunk: MessageChunk) -> MessageChunk {
    if let Some(m) = meta {
        chunk.role = m.role;
        chunk.name = m.name.clone();
    }
    chunk
}

fn merge_ctrl_with_stream_id(ctrl: Option<&StreamCtrl>, stream_id: &str) -> StreamCtrl {
    let mut c = ctrl.cloned().unwrap_or_default();
    c.stream_id = stream_id.to_string();
    c
}

struct AsrChannelStream {
    rx: mpsc::Receiver<Result<MessageChunk, GenxError>>,
}

#[async_trait]
impl Stream for AsrChannelStream {
    async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
        match self.rx.recv().await {
            Some(Ok(c)) => Ok(Some(c)),
            Some(Err(e)) => Err(e),
            None => Ok(None),
        }
    }

    fn result(&self) -> Option<StreamResult> {
        None
    }

    async fn close(&mut self) -> Result<(), GenxError> {
        self.rx.close();
        Ok(())
    }

    async fn close_with_error(&mut self, _error: GenxError) -> Result<(), GenxError> {
        self.rx.close();
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde::Deserialize;
    use crate::types::Role;
    use std::collections::VecDeque;
    use tokio::sync::Mutex;

    #[derive(Default)]
    struct MockBackend {
        sessions: Mutex<VecDeque<MockSession>>,
    }

    #[async_trait]
    impl AsrBackend for MockBackend {
        async fn open_session(&self) -> Result<Box<dyn AsrSession>, GenxError> {
            let mut guard = self.sessions.lock().await;
            let s = guard.pop_front().expect("mock session exists");
            Ok(Box::new(s))
        }
    }

    struct MockSession {
        recv_items: VecDeque<Result<Option<AsrResult>, GenxError>>,
    }

    #[async_trait]
    impl AsrSession for MockSession {
        async fn send_audio(&mut self, _audio: Vec<u8>, _is_last: bool) -> Result<(), GenxError> {
            Ok(())
        }

        async fn recv(&mut self) -> Result<Option<AsrResult>, GenxError> {
            self.recv_items.pop_front().unwrap_or(Ok(None))
        }

        async fn close(&mut self) -> Result<(), GenxError> {
            Ok(())
        }
    }

    fn input_stream(chunks: Vec<MessageChunk>) -> Box<dyn Stream> {
        let builder = crate::stream::StreamBuilder::with_tools(16, vec![]);
        builder.add(&chunks).expect("add chunks");
        builder.done(crate::error::Usage::default()).expect("done");
        Box::new(builder.stream())
    }

    #[tokio::test]
    async fn asr_deduplicates_utterance_and_emits_text_eos() {
        let backend = Arc::new(MockBackend::default());
        backend.sessions.lock().await.push_back(MockSession {
            recv_items: VecDeque::from(vec![
                Ok(Some(AsrResult {
                    utterances: vec![AsrUtterance {
                        text: "你好".to_string(),
                        end_time: 100,
                        definite: true,
                    }],
                    ..Default::default()
                })),
                Ok(Some(AsrResult {
                    utterances: vec![AsrUtterance {
                        text: "你好".to_string(),
                        end_time: 100,
                        definite: true,
                    }],
                    ..Default::default()
                })),
                Ok(None),
            ]),
        });

        let t = DoubaoAsrSaucTransformer::new_with_backend(backend);
        let mut out = t
            .transform(
                "doubao/sauc",
                input_stream(vec![
                    MessageChunk::blob(Role::User, "audio/ogg", vec![1, 2]),
                    MessageChunk::new_end_of_stream("audio/ogg"),
                ]),
            )
            .await
            .expect("transform");

        let text = out.next().await.expect("next1").expect("chunk1");
        assert_eq!(text.part.as_ref().and_then(|p| p.as_text()), Some("你好"));

        let eos = out.next().await.expect("next2").expect("chunk2");
        assert!(eos.is_end_of_stream());
        assert_eq!(eos.part.as_ref().and_then(|p| p.as_text()), Some(""));
        assert!(out.next().await.expect("eof").is_none());
    }

    #[tokio::test]
    async fn asr_non_audio_chunk_passthrough() {
        let backend = Arc::new(MockBackend::default());
        let t = DoubaoAsrSaucTransformer::new_with_backend(backend);

        let passthrough = MessageChunk::text(Role::Model, "passthrough");
        let mut out = t
            .transform("doubao/sauc", input_stream(vec![passthrough.clone()]))
            .await
            .expect("transform");

        let got = out.next().await.expect("next").expect("chunk");
        assert_eq!(got.part, passthrough.part);
    }

    #[derive(Debug, Deserialize)]
    struct AsrFixture {
        input: Vec<AsrFixtureChunk>,
        expected: Vec<AsrFixtureChunk>,
    }

    #[derive(Debug, Deserialize)]
    struct AsrFixtureChunk {
        r#type: String,
        role: Option<String>,
        mime_type: Option<String>,
        data: Option<Vec<u8>>,
        stream_id: Option<String>,
        text: Option<String>,
    }

    #[tokio::test]
    async fn asr_parity_fixture_matches_go_semantics() {
        let fixture: AsrFixture = serde_yaml::from_str(include_str!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/tests/parity/fixtures/asr_fixture.yaml"
        )))
        .expect("parse asr fixture");

        let mut input = Vec::new();
        for c in &fixture.input {
            match c.r#type.as_str() {
                "audio" => {
                    let role = if c.role.as_deref() == Some("model") {
                        Role::Model
                    } else {
                        Role::User
                    };
                    let mut chunk = MessageChunk::blob(
                        role,
                        c.mime_type
                            .clone()
                            .unwrap_or_else(|| "audio/ogg".to_string()),
                        c.data.clone().unwrap_or_default(),
                    );
                    if let Some(stream_id) = &c.stream_id {
                        chunk.ctrl = Some(StreamCtrl {
                            stream_id: stream_id.clone(),
                            ..Default::default()
                        });
                    }
                    input.push(chunk);
                }
                "eos_audio" => input.push(MessageChunk::new_end_of_stream("audio/ogg")),
                other => panic!("unsupported fixture input type: {other}"),
            }
        }

        let backend = Arc::new(MockBackend::default());
        backend.sessions.lock().await.push_back(MockSession {
            recv_items: VecDeque::from(vec![
                Ok(Some(AsrResult {
                    utterances: vec![AsrUtterance {
                        text: "你好".to_string(),
                        end_time: 100,
                        definite: true,
                    }],
                    ..Default::default()
                })),
                Ok(None),
            ]),
        });

        let t = DoubaoAsrSaucTransformer::new_with_backend(backend);
        let mut out = t
            .transform("doubao/sauc", input_stream(input))
            .await
            .expect("transform");

        let mut actual = Vec::new();
        while let Some(chunk) = out.next().await.expect("next") {
            actual.push(chunk);
        }

        assert_eq!(actual.len(), fixture.expected.len());
        for (idx, exp) in fixture.expected.iter().enumerate() {
            let got = &actual[idx];
            match exp.r#type.as_str() {
                "text" => {
                    assert_eq!(got.part.as_ref().and_then(|p| p.as_text()), exp.text.as_deref());
                }
                "eos_text" => {
                    assert!(got.is_end_of_stream(), "idx={idx}");
                    assert_eq!(got.part.as_ref().and_then(|p| p.as_text()), Some(""));
                }
                other => panic!("unsupported fixture expected type: {other}"),
            }
            if let Some(expected_stream_id) = &exp.stream_id {
                assert_eq!(
                    got.ctrl.as_ref().map(|c| c.stream_id.as_str()),
                    Some(expected_stream_id.as_str())
                );
            }
        }
    }
}
