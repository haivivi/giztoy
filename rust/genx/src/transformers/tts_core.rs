use std::sync::Arc;

use async_trait::async_trait;
use tokio::sync::mpsc;

use crate::error::GenxError;
use crate::stream::{Stream, StreamResult};
use crate::stream_id::new_stream_id;
use crate::types::{MessageChunk, Part, StreamCtrl};

#[async_trait]
pub(crate) trait TtsProvider: Send + Sync {
    fn mime_type(&self) -> &str;
    async fn synthesize_stream(
        &self,
        text: &str,
        emitter: &mut dyn AudioEmitter,
    ) -> Result<(), GenxError>;
}

#[async_trait]
pub(crate) trait AudioEmitter: Send {
    async fn emit(&mut self, audio: Vec<u8>) -> Result<(), GenxError>;
}

pub(crate) fn spawn_tts_transform_loop(
    provider: Arc<dyn TtsProvider>,
    input: Box<dyn Stream>,
) -> Box<dyn Stream> {
    let (tx, rx) = mpsc::channel(128);
    tokio::spawn(async move {
        run_tts_transform_loop(provider, input, tx).await;
    });
    Box::new(TransformerChannelStream { rx })
}

async fn run_tts_transform_loop(
    provider: Arc<dyn TtsProvider>,
    mut input: Box<dyn Stream>,
    tx: mpsc::Sender<Result<MessageChunk, GenxError>>,
) {
    let mut text_buffer = String::new();
    let mut last_meta: Option<MessageChunk> = None;
    let mut current_stream_id = String::new();

    loop {
        let next = input.next().await;
        match next {
            Ok(Some(chunk)) => {
                if let Some(ctrl) = &chunk.ctrl {
                    if !ctrl.stream_id.is_empty() {
                        current_stream_id = ctrl.stream_id.clone();
                    }
                }
                if current_stream_id.is_empty() {
                    current_stream_id = new_stream_id();
                }

                let prev_meta = last_meta.clone();

                if chunk.is_end_of_stream() {
                    match chunk.part.as_ref() {
                        Some(Part::Text(_)) => {
                            let flush_meta = if prev_meta.is_some() {
                                prev_meta
                            } else {
                                Some(chunk.clone())
                            };
                            if !flush_synthesis(
                                &*provider,
                                &mut text_buffer,
                                &flush_meta,
                                &current_stream_id,
                                &tx,
                            )
                            .await
                            {
                                return;
                            }

                            let eos = with_meta(
                                &flush_meta,
                                MessageChunk {
                                    role: chunk.role,
                                    name: chunk.name.clone(),
                                    part: Some(Part::blob(provider.mime_type(), Vec::<u8>::new())),
                                    tool_call: None,
                                    ctrl: Some(StreamCtrl {
                                        stream_id: current_stream_id.clone(),
                                        end_of_stream: true,
                                        ..Default::default()
                                    }),
                                },
                            );
                            if tx.send(Ok(eos)).await.is_err() {
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

                last_meta = Some(chunk.clone());

                match chunk.part.as_ref() {
                    Some(Part::Text(text)) => {
                        text_buffer.push_str(text);
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
                let _ = flush_synthesis(
                    &*provider,
                    &mut text_buffer,
                    &last_meta,
                    &current_stream_id,
                    &tx,
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

async fn flush_synthesis(
    provider: &dyn TtsProvider,
    text_buffer: &mut String,
    last_meta: &Option<MessageChunk>,
    stream_id: &str,
    tx: &mpsc::Sender<Result<MessageChunk, GenxError>>,
) -> bool {
    if text_buffer.is_empty() {
        return true;
    }

    let sid = if stream_id.is_empty() {
        new_stream_id()
    } else {
        stream_id.to_string()
    };

    let bos = with_meta(
        last_meta,
        MessageChunk {
            role: last_meta.as_ref().map_or(crate::types::Role::Model, |c| c.role),
            name: last_meta.as_ref().and_then(|c| c.name.clone()),
            part: None,
            tool_call: None,
            ctrl: Some(StreamCtrl {
                stream_id: sid.clone(),
                begin_of_stream: true,
                ..Default::default()
            }),
        },
    );
    if tx.send(Ok(bos)).await.is_err() {
        return false;
    }

    let mut emitter = TxAudioEmitter {
        tx,
        last_meta,
        stream_id: &sid,
        mime_type: provider.mime_type(),
    };

    if let Err(e) = provider.synthesize_stream(text_buffer, &mut emitter).await {
        let _ = tx.send(Err(e)).await;
        return false;
    }

    text_buffer.clear();
    true
}

struct TxAudioEmitter<'a> {
    tx: &'a mpsc::Sender<Result<MessageChunk, GenxError>>,
    last_meta: &'a Option<MessageChunk>,
    stream_id: &'a str,
    mime_type: &'a str,
}

#[async_trait]
impl AudioEmitter for TxAudioEmitter<'_> {
    async fn emit(&mut self, audio: Vec<u8>) -> Result<(), GenxError> {
        if audio.is_empty() {
            return Ok(());
        }

        let out = with_meta(
            self.last_meta,
            MessageChunk {
                role: self
                    .last_meta
                    .as_ref()
                    .map_or(crate::types::Role::Model, |c| c.role),
                name: self.last_meta.as_ref().and_then(|c| c.name.clone()),
                part: Some(Part::blob(self.mime_type, audio)),
                tool_call: None,
                ctrl: Some(StreamCtrl {
                    stream_id: self.stream_id.to_string(),
                    ..Default::default()
                }),
            },
        );
        self.tx
            .send(Ok(out))
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("tts output closed: {}", e)))
    }
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

#[cfg(test)]
mod tests {
    use super::*;
    use serde::Deserialize;
    use std::sync::atomic::{AtomicUsize, Ordering};
    use tokio::sync::Mutex;

    struct MockProvider {
        mime: String,
        responses: Mutex<Vec<Result<Vec<Result<Vec<u8>, GenxError>>, GenxError>>>,
        polls: Arc<AtomicUsize>,
    }

    #[async_trait]
    impl TtsProvider for MockProvider {
        fn mime_type(&self) -> &str {
            &self.mime
        }

        async fn synthesize_stream(
            &self,
            _text: &str,
            emitter: &mut dyn AudioEmitter,
        ) -> Result<(), GenxError> {
            match self.responses.lock().await.remove(0) {
                Ok(items) => {
                    for item in items {
                        let polls = Arc::clone(&self.polls);
                        polls.fetch_add(1, Ordering::Relaxed);
                        emitter.emit(item?).await?;
                    }
                    Ok(())
                }
                Err(e) => Err(e),
            }
        }
    }

    fn input_stream(chunks: Vec<MessageChunk>) -> Box<dyn Stream> {
        let builder = crate::stream::StreamBuilder::with_tools(16, vec![]);
        builder.add(&chunks).expect("add chunks");
        builder.done(crate::error::Usage::default()).expect("done");
        Box::new(builder.stream())
    }

    #[tokio::test]
    async fn tts_empty_text_eos_emits_only_audio_eos() {
        let provider: Arc<dyn TtsProvider> = Arc::new(MockProvider {
            mime: "audio/mpeg".to_string(),
            responses: Mutex::new(vec![]),
            polls: Arc::new(AtomicUsize::new(0)),
        });

        let mut out = spawn_tts_transform_loop(
            provider,
            input_stream(vec![MessageChunk::new_text_end_of_stream()]),
        );

        let eos = out.next().await.expect("next").expect("chunk");
        assert!(eos.is_end_of_stream());
        assert!(matches!(eos.part, Some(Part::Blob(_))));
        assert!(out.next().await.expect("eof").is_none());
    }

    #[tokio::test]
    async fn tts_stops_when_downstream_receiver_dropped() {
        let polls = Arc::new(AtomicUsize::new(0));
        let provider: Arc<dyn TtsProvider> = Arc::new(MockProvider {
            mime: "audio/mpeg".to_string(),
            responses: Mutex::new(vec![Ok(vec![
                Ok(vec![1]),
                Ok(vec![2]),
                Ok(vec![3]),
                Ok(vec![4]),
            ])]),
            polls: Arc::clone(&polls),
        });

        let out = spawn_tts_transform_loop(
            provider,
            input_stream(vec![
                MessageChunk::text(crate::types::Role::Model, "hello"),
                MessageChunk::new_text_end_of_stream(),
            ]),
        );

        // 不消费任何输出，直接丢弃 receiver，模拟 sink 写失败
        drop(out);

        tokio::time::sleep(std::time::Duration::from_millis(50)).await;
        assert_eq!(polls.load(Ordering::Relaxed), 0);
    }

    #[tokio::test]
    async fn tts_preserves_emitted_audio_when_provider_fails_midstream() {
        let provider: Arc<dyn TtsProvider> = Arc::new(MockProvider {
            mime: "audio/mpeg".to_string(),
            responses: Mutex::new(vec![Ok(vec![
                Ok(vec![9, 9]),
                Err(GenxError::Other(anyhow::anyhow!("midstream fail"))),
            ])]),
            polls: Arc::new(AtomicUsize::new(0)),
        });

        let mut out = spawn_tts_transform_loop(
            provider,
            input_stream(vec![
                MessageChunk::text(crate::types::Role::Model, "hello"),
                MessageChunk::new_text_end_of_stream(),
            ]),
        );

        let _bos = out.next().await.expect("bos").expect("bos chunk");
        let audio = out.next().await.expect("audio").expect("audio chunk");
        assert!(matches!(audio.part, Some(Part::Blob(_))));

        let err = out.next().await.expect_err("expect provider stream error");
        assert!(err.to_string().contains("midstream fail"));
    }

    #[derive(Debug, Deserialize)]
    struct TtsFixture {
        input: Vec<TtsFixtureChunk>,
        expected: Vec<TtsFixtureChunk>,
    }

    #[derive(Debug, Deserialize)]
    struct TtsFixtureChunk {
        r#type: String,
        role: Option<String>,
        text: Option<String>,
        stream_id: Option<String>,
        mime_type: Option<String>,
    }

    #[tokio::test]
    async fn tts_parity_fixture_matches_go_semantics() {
        let fixture: TtsFixture = serde_yaml::from_str(include_str!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/tests/parity/fixtures/tts_fixture.yaml"
        )))
        .expect("parse tts fixture");

        let input_chunks = fixture
            .input
            .iter()
            .map(|c| match c.r#type.as_str() {
                "text" => {
                    let mut chunk = MessageChunk::text(
                        if c.role.as_deref() == Some("user") {
                            crate::types::Role::User
                        } else {
                            crate::types::Role::Model
                        },
                        c.text.clone().unwrap_or_default(),
                    );
                    if let Some(stream_id) = &c.stream_id {
                        chunk.ctrl = Some(StreamCtrl {
                            stream_id: stream_id.clone(),
                            ..Default::default()
                        });
                    }
                    chunk
                }
                "eos_text" => MessageChunk::new_text_end_of_stream(),
                other => panic!("unsupported fixture input type: {other}"),
            })
            .collect::<Vec<_>>();

        let provider: Arc<dyn TtsProvider> = Arc::new(MockProvider {
            mime: "audio/mpeg".to_string(),
            responses: Mutex::new(vec![Ok(vec![Ok(vec![1, 2, 3])])]),
            polls: Arc::new(AtomicUsize::new(0)),
        });

        let mut out = spawn_tts_transform_loop(provider, input_stream(input_chunks));
        let mut actual = Vec::new();
        while let Some(chunk) = out.next().await.expect("next") {
            actual.push(chunk);
        }

        assert_eq!(actual.len(), fixture.expected.len());
        for (idx, exp) in fixture.expected.iter().enumerate() {
            let got = &actual[idx];
            match exp.r#type.as_str() {
                "bos" => assert!(got.is_begin_of_stream(), "idx={idx}"),
                "blob" => {
                    let blob = got.part.as_ref().and_then(|p| p.as_blob()).expect("blob");
                    assert_eq!(blob.mime_type, exp.mime_type.clone().unwrap_or_default());
                }
                "eos_blob" => {
                    assert!(got.is_end_of_stream(), "idx={idx}");
                    let blob = got.part.as_ref().and_then(|p| p.as_blob()).expect("blob");
                    assert_eq!(blob.mime_type, exp.mime_type.clone().unwrap_or_default());
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

struct TransformerChannelStream {
    rx: mpsc::Receiver<Result<MessageChunk, GenxError>>,
}

#[async_trait]
impl Stream for TransformerChannelStream {
    async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
        match self.rx.recv().await {
            Some(Ok(chunk)) => Ok(Some(chunk)),
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
