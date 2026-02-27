use std::collections::VecDeque;
use std::sync::Arc;

use async_trait::async_trait;
use tokio::sync::{mpsc, Mutex};

use crate::error::GenxError;
use crate::stream::{Stream, StreamResult};
use crate::stream_id::new_stream_id;
use crate::transformer::Transformer;
use crate::types::{MessageChunk, Part, Role, StreamCtrl};

#[derive(Debug, Clone)]
pub enum DoubaoServerEvent {
    AsrResponse { text: String },
    AsrEnded,
    TtsStarted { content: String },
    ChatResponse { text: String },
    AudioReceived { data: Vec<u8> },
    TtsFinished,
    SessionEnded,
    SessionFailed { message: String },
}

#[async_trait]
pub trait DoubaoRealtimeSession: Send + Sync {
    async fn send_audio(&self, audio: &[u8]) -> Result<(), GenxError>;
    async fn send_text(&self, text: &str) -> Result<(), GenxError>;
    async fn recv(&self) -> Option<Result<DoubaoServerEvent, GenxError>>;
    async fn close(&self) -> Result<(), GenxError>;
}

#[async_trait]
pub trait DoubaoRealtimeConnector: Send + Sync {
    async fn connect(&self) -> Result<Box<dyn DoubaoRealtimeSession>, GenxError>;
}

pub struct DoubaoRealtime {
    connector: Arc<dyn DoubaoRealtimeConnector>,
    output_mime: String,
    trailing_silence: Vec<u8>,
}

impl DoubaoRealtime {
    pub fn new(connector: Arc<dyn DoubaoRealtimeConnector>) -> Self {
        Self {
            connector,
            output_mime: "audio/pcm".to_string(),
            trailing_silence: vec![0u8; 16_000],
        }
    }

    pub fn with_output_mime(mut self, mime: impl Into<String>) -> Self {
        self.output_mime = mime.into();
        self
    }

    pub fn with_trailing_silence(mut self, bytes: Vec<u8>) -> Self {
        self.trailing_silence = bytes;
        self
    }
}

#[async_trait]
impl Transformer for DoubaoRealtime {
    async fn transform(
        &self,
        _pattern: &str,
        input: Box<dyn Stream>,
    ) -> Result<Box<dyn Stream>, GenxError> {
        let session = self.connector.connect().await?;
        let session: Arc<dyn DoubaoRealtimeSession> = Arc::from(session);

        let (tx, rx) = mpsc::channel(128);
        let output = ChannelStream { rx };

        let output_mime = self.output_mime.clone();
        let trailing_silence = self.trailing_silence.clone();

        tokio::spawn(async move {
            let mut input = input;
            let stream_state = Arc::new(Mutex::new(StreamState::default()));

            let events_done = {
                let tx = tx.clone();
                let session = Arc::clone(&session);
                let stream_state = Arc::clone(&stream_state);
                let output_mime = output_mime.clone();
                tokio::spawn(async move {
                    loop {
                        let evt = match session.recv().await {
                            Some(evt) => evt,
                            None => break,
                        };
                        match evt {
                            Ok(event) => {
                                if let Err(e) = handle_event(
                                    &tx,
                                    &output_mime,
                                    Arc::clone(&stream_state),
                                    event,
                                )
                                .await
                                {
                                    let _ = tx.send(Err(e)).await;
                                    break;
                                }
                            }
                            Err(e) => {
                                let _ = tx.send(Err(e)).await;
                                break;
                            }
                        }
                    }
                })
            };

            loop {
                match input.next().await {
                    Ok(Some(chunk)) => {
                        if let Err(e) =
                            forward_input(&*session, Arc::clone(&stream_state), chunk, &trailing_silence)
                                .await
                        {
                            let _ = tx.send(Err(e)).await;
                            break;
                        }
                    }
                    Ok(None) => {
                        let _ = session.close().await;
                        break;
                    }
                    Err(e) => {
                        let _ = tx.send(Err(e)).await;
                        let _ = session.close().await;
                        break;
                    }
                }
            }

            let _ = events_done.await;
            drop(tx);
        });

        Ok(Box::new(output))
    }
}

#[derive(Default)]
struct StreamState {
    queued_stream_ids: VecDeque<String>,
    response_stream_id: String,
}

impl StreamState {
    fn push_stream_id(&mut self, id: String) {
        if !id.is_empty() {
            self.queued_stream_ids.push_back(id);
        }
    }

    fn pop_for_response(&mut self) {
        self.response_stream_id = self
            .queued_stream_ids
            .pop_front()
            .unwrap_or_else(new_stream_id);
    }

    fn current_response_stream_id(&self) -> String {
        if self.response_stream_id.is_empty() {
            new_stream_id()
        } else {
            self.response_stream_id.clone()
        }
    }
}

async fn forward_input(
    session: &dyn DoubaoRealtimeSession,
    state: Arc<Mutex<StreamState>>,
    chunk: MessageChunk,
    trailing_silence: &[u8],
) -> Result<(), GenxError> {
    if chunk.is_begin_of_stream() {
        if let Some(ctrl) = &chunk.ctrl {
            let mut st = state.lock().await;
            st.push_stream_id(ctrl.stream_id.clone());
        }
        return Ok(());
    }

    if chunk.is_end_of_stream() {
        session.send_audio(trailing_silence).await?;
        return Ok(());
    }

    match chunk.part {
        Some(Part::Blob(blob)) => {
            if !blob.data.is_empty() {
                session.send_audio(&blob.data).await?;
            }
        }
        Some(Part::Text(text)) => {
            if !text.is_empty() {
                session.send_text(&text).await?;
            }
        }
        None => {}
    }
    Ok(())
}

async fn handle_event(
    tx: &mpsc::Sender<Result<MessageChunk, GenxError>>,
    output_mime: &str,
    state: Arc<Mutex<StreamState>>,
    event: DoubaoServerEvent,
) -> Result<(), GenxError> {
    match event {
        DoubaoServerEvent::AsrResponse { text } => {
            if text.is_empty() {
                return Ok(());
            }
            let stream_id = state.lock().await.current_response_stream_id();
            send_chunk(tx, MessageChunk {
                role: Role::User,
                name: None,
                part: Some(Part::Text(text)),
                tool_call: None,
                ctrl: Some(StreamCtrl {
                    stream_id,
                    ..Default::default()
                }),
            })
            .await
        }
        DoubaoServerEvent::AsrEnded => {
            state.lock().await.pop_for_response();
            Ok(())
        }
        DoubaoServerEvent::TtsStarted { content } => {
            let stream_id = state.lock().await.current_response_stream_id();
            send_chunk(
                tx,
                MessageChunk {
                    role: Role::Model,
                    name: None,
                    part: Some(Part::blob(output_mime, Vec::<u8>::new())),
                    tool_call: None,
                    ctrl: Some(StreamCtrl {
                        stream_id: stream_id.clone(),
                        begin_of_stream: true,
                        ..Default::default()
                    }),
                },
            )
            .await?;
            if !content.is_empty() {
                send_chunk(
                    tx,
                    MessageChunk {
                        role: Role::Model,
                        name: None,
                        part: Some(Part::Text(content)),
                        tool_call: None,
                        ctrl: Some(StreamCtrl {
                            stream_id,
                            ..Default::default()
                        }),
                    },
                )
                .await?;
            }
            Ok(())
        }
        DoubaoServerEvent::ChatResponse { text } => {
            if text.is_empty() {
                return Ok(());
            }
            let stream_id = state.lock().await.current_response_stream_id();
            send_chunk(
                tx,
                MessageChunk {
                    role: Role::Model,
                    name: None,
                    part: Some(Part::Text(text)),
                    tool_call: None,
                    ctrl: Some(StreamCtrl {
                        stream_id,
                        ..Default::default()
                    }),
                },
            )
            .await
        }
        DoubaoServerEvent::AudioReceived { data } => {
            if data.is_empty() {
                return Ok(());
            }
            let stream_id = state.lock().await.current_response_stream_id();
            send_chunk(
                tx,
                MessageChunk {
                    role: Role::Model,
                    name: None,
                    part: Some(Part::blob(output_mime, data)),
                    tool_call: None,
                    ctrl: Some(StreamCtrl {
                        stream_id,
                        ..Default::default()
                    }),
                },
            )
            .await
        }
        DoubaoServerEvent::TtsFinished => {
            let stream_id = state.lock().await.current_response_stream_id();
            send_chunk(
                tx,
                MessageChunk {
                    role: Role::Model,
                    name: None,
                    part: Some(Part::blob(output_mime, Vec::<u8>::new())),
                    tool_call: None,
                    ctrl: Some(StreamCtrl {
                        stream_id,
                        end_of_stream: true,
                        ..Default::default()
                    }),
                },
            )
            .await
        }
        DoubaoServerEvent::SessionEnded => Ok(()),
        DoubaoServerEvent::SessionFailed { message } => {
            Err(GenxError::Generation { usage: Default::default(), message })
        }
    }
}

async fn send_chunk(
    tx: &mpsc::Sender<Result<MessageChunk, GenxError>>,
    chunk: MessageChunk,
) -> Result<(), GenxError> {
    tx.send(Ok(chunk))
        .await
        .map_err(|_| GenxError::Other(anyhow::anyhow!("output stream dropped")))
}

struct ChannelStream {
    rx: mpsc::Receiver<Result<MessageChunk, GenxError>>,
}

#[async_trait]
impl Stream for ChannelStream {
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

pub struct DoubaoSdkConnector {
    client: giztoy_doubaospeech::Client,
    config: giztoy_doubaospeech::RealtimeConfig,
}

impl DoubaoSdkConnector {
    pub fn new(
        client: giztoy_doubaospeech::Client,
        config: giztoy_doubaospeech::RealtimeConfig,
    ) -> Self {
        Self { client, config }
    }
}

#[async_trait]
impl DoubaoRealtimeConnector for DoubaoSdkConnector {
    async fn connect(&self) -> Result<Box<dyn DoubaoRealtimeSession>, GenxError> {
        let session = self
            .client
            .realtime()
            .connect(&self.config)
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("doubao realtime connect failed: {}", e)))?;
        Ok(Box::new(DoubaoSdkSession {
            inner: Arc::new(session),
        }))
    }
}

struct DoubaoSdkSession {
    inner: Arc<giztoy_doubaospeech::RealtimeSession>,
}

#[async_trait]
impl DoubaoRealtimeSession for DoubaoSdkSession {
    async fn send_audio(&self, audio: &[u8]) -> Result<(), GenxError> {
        self.inner
            .send_audio(audio)
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("doubao send_audio failed: {}", e)))
    }

    async fn send_text(&self, text: &str) -> Result<(), GenxError> {
        self.inner
            .send_text(text)
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("doubao send_text failed: {}", e)))
    }

    async fn recv(&self) -> Option<Result<DoubaoServerEvent, GenxError>> {
        let evt = self.inner.recv().await?;
        match evt {
            Ok(e) => Some(map_doubao_event(e)),
            Err(e) => Some(Err(GenxError::Other(anyhow::anyhow!(
                "doubao recv failed: {}",
                e
            )))),
        }
    }

    async fn close(&self) -> Result<(), GenxError> {
        self.inner
            .close()
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("doubao close failed: {}", e)))
    }
}

fn map_doubao_event(e: giztoy_doubaospeech::RealtimeEvent) -> Result<DoubaoServerEvent, GenxError> {
    use giztoy_doubaospeech::RealtimeEventType;

    match e.event_type {
        Some(RealtimeEventType::ASRFinished) => Ok(DoubaoServerEvent::AsrEnded),
        Some(RealtimeEventType::TTSFinished) => Ok(DoubaoServerEvent::TtsFinished),
        Some(RealtimeEventType::AudioReceived) => Ok(DoubaoServerEvent::AudioReceived {
            data: e.audio.unwrap_or_default(),
        }),
        Some(RealtimeEventType::SessionEnded) => Ok(DoubaoServerEvent::SessionEnded),
        Some(RealtimeEventType::SessionFailed) => Ok(DoubaoServerEvent::SessionFailed {
            message: e
                .error
                .map(|err| err.to_string())
                .unwrap_or_else(|| "session failed".to_string()),
        }),
        Some(RealtimeEventType::TTSStarted) => Ok(DoubaoServerEvent::TtsStarted {
            content: e
                .tts_info
                .map(|x| x.content)
                .filter(|x| !x.is_empty())
                .unwrap_or_else(|| e.text.clone()),
        }),
        Some(RealtimeEventType::ASRStarted) => Ok(DoubaoServerEvent::AsrResponse { text: e.text }),
        Some(RealtimeEventType::SessionStarted) => Ok(DoubaoServerEvent::ChatResponse { text: e.text }),
        None => {
            if let Some(info) = e.asr_info {
                return Ok(DoubaoServerEvent::AsrResponse { text: info.text });
            }
            if !e.text.is_empty() {
                return Ok(DoubaoServerEvent::ChatResponse { text: e.text });
            }
            if let Some(audio) = e.audio {
                return Ok(DoubaoServerEvent::AudioReceived { data: audio });
            }
            Err(GenxError::Other(anyhow::anyhow!("unknown doubao event")))
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};

    struct MockSession {
        events: Arc<Mutex<VecDeque<Result<DoubaoServerEvent, GenxError>>>>,
        fail_send_audio: bool,
        closed: Arc<AtomicBool>,
        recv_calls: Arc<AtomicUsize>,
    }

    #[async_trait]
    impl DoubaoRealtimeSession for MockSession {
        async fn send_audio(&self, _audio: &[u8]) -> Result<(), GenxError> {
            if self.fail_send_audio {
                return Err(GenxError::Other(anyhow::anyhow!("send audio failed")));
            }
            Ok(())
        }

        async fn send_text(&self, _text: &str) -> Result<(), GenxError> {
            Ok(())
        }

        async fn recv(&self) -> Option<Result<DoubaoServerEvent, GenxError>> {
            self.recv_calls.fetch_add(1, Ordering::SeqCst);
            self.events.lock().await.pop_front()
        }

        async fn close(&self) -> Result<(), GenxError> {
            self.closed.store(true, Ordering::SeqCst);
            Ok(())
        }
    }

    struct MockConnector {
        session: Option<Box<dyn DoubaoRealtimeSession>>,
        fail: bool,
    }

    #[async_trait]
    impl DoubaoRealtimeConnector for MockConnector {
        async fn connect(&self) -> Result<Box<dyn DoubaoRealtimeSession>, GenxError> {
            if self.fail {
                return Err(GenxError::Other(anyhow::anyhow!("handshake failed")));
            }
            let _ = &self.session;
            Err(GenxError::Other(anyhow::anyhow!("test connector not clonable")))
        }
    }

    struct StaticConnector {
        session: Arc<Mutex<Option<Box<dyn DoubaoRealtimeSession>>>>,
    }

    #[async_trait]
    impl DoubaoRealtimeConnector for StaticConnector {
        async fn connect(&self) -> Result<Box<dyn DoubaoRealtimeSession>, GenxError> {
            self.session
                .lock()
                .await
                .take()
                .ok_or_else(|| GenxError::Other(anyhow::anyhow!("session already taken")))
        }
    }

    struct InputStream {
        chunks: VecDeque<Result<MessageChunk, GenxError>>,
    }

    #[async_trait]
    impl Stream for InputStream {
        async fn next(&mut self) -> Result<Option<MessageChunk>, GenxError> {
            match self.chunks.pop_front() {
                Some(Ok(c)) => Ok(Some(c)),
                Some(Err(e)) => Err(e),
                None => Ok(None),
            }
        }

        fn result(&self) -> Option<StreamResult> {
            None
        }

        async fn close(&mut self) -> Result<(), GenxError> {
            Ok(())
        }

        async fn close_with_error(&mut self, _error: GenxError) -> Result<(), GenxError> {
            Ok(())
        }
    }

    fn make_transformer(
        events: Vec<Result<DoubaoServerEvent, GenxError>>,
        fail_send_audio: bool,
    ) -> (DoubaoRealtime, Arc<AtomicBool>, Arc<AtomicUsize>) {
        let closed = Arc::new(AtomicBool::new(false));
        let recv_calls = Arc::new(AtomicUsize::new(0));
        let session = MockSession {
            events: Arc::new(Mutex::new(VecDeque::from(events))),
            fail_send_audio,
            closed: Arc::clone(&closed),
            recv_calls: Arc::clone(&recv_calls),
        };
        let connector = StaticConnector {
            session: Arc::new(Mutex::new(Some(Box::new(session)))),
        };
        (DoubaoRealtime::new(Arc::new(connector)), closed, recv_calls)
    }

    #[tokio::test]
    async fn test_full_conversation_flow() {
        let events = vec![
            Ok(DoubaoServerEvent::AsrResponse {
                text: "你好".to_string(),
            }),
            Ok(DoubaoServerEvent::AsrEnded),
            Ok(DoubaoServerEvent::TtsStarted {
                content: String::new(),
            }),
            Ok(DoubaoServerEvent::ChatResponse {
                text: "你好，有什么可以帮助你？".to_string(),
            }),
            Ok(DoubaoServerEvent::AudioReceived { data: vec![1, 2, 3] }),
            Ok(DoubaoServerEvent::TtsFinished),
            Ok(DoubaoServerEvent::SessionEnded),
        ];
        let (t, _, _) = make_transformer(events, false);
        let input = InputStream {
            chunks: VecDeque::from(vec![
                Ok(MessageChunk::new_begin_of_stream("stream-1")),
                Ok(MessageChunk::blob(Role::User, "audio/pcm", vec![7, 8, 9])),
                Ok(MessageChunk::new_end_of_stream("audio/pcm")),
            ]),
        };

        let mut out = t.transform("doubao/realtime", Box::new(input)).await.unwrap();

        let c1 = out.next().await.unwrap().unwrap();
        assert_eq!(c1.role, Role::User);
        assert_eq!(c1.part.as_ref().and_then(|p| p.as_text()), Some("你好"));

        let c2 = out.next().await.unwrap().unwrap();
        assert_eq!(c2.role, Role::Model);
        assert!(c2.is_begin_of_stream());

        let c3 = out.next().await.unwrap().unwrap();
        assert_eq!(c3.part.as_ref().and_then(|p| p.as_text()), Some("你好，有什么可以帮助你？"));

        let c4 = out.next().await.unwrap().unwrap();
        assert!(c4.part.as_ref().and_then(|p| p.as_blob()).is_some());

        let c5 = out.next().await.unwrap().unwrap();
        assert!(c5.is_end_of_stream());
        assert_eq!(out.next().await.unwrap(), None);
    }

    #[tokio::test]
    async fn test_empty_input() {
        let (t, _, _) = make_transformer(vec![], false);
        let input = InputStream {
            chunks: VecDeque::new(),
        };
        let mut out = t.transform("doubao/realtime", Box::new(input)).await.unwrap();
        assert_eq!(out.next().await.unwrap(), None);
    }

    #[tokio::test]
    async fn test_text_input_only() {
        let events = vec![
            Ok(DoubaoServerEvent::AsrEnded),
            Ok(DoubaoServerEvent::ChatResponse {
                text: "我是豆包".to_string(),
            }),
            Ok(DoubaoServerEvent::SessionEnded),
        ];
        let (t, _, _) = make_transformer(events, false);
        let input = InputStream {
            chunks: VecDeque::from(vec![Ok(MessageChunk::text(
                Role::User,
                "你好，请介绍一下自己",
            ))]),
        };
        let mut out = t.transform("doubao/realtime", Box::new(input)).await.unwrap();
        let chunk = out.next().await.unwrap().unwrap();
        assert_eq!(chunk.part.as_ref().and_then(|p| p.as_text()), Some("我是豆包"));
    }

    #[tokio::test]
    async fn test_init_connection_failed() {
        let t = DoubaoRealtime::new(Arc::new(MockConnector {
            session: None,
            fail: true,
        }));
        let input = InputStream {
            chunks: VecDeque::new(),
        };
        let err = match t.transform("doubao/realtime", Box::new(input)).await {
            Ok(_) => panic!("expected init error"),
            Err(e) => e,
        };
        assert!(err.to_string().contains("handshake failed"));
    }

    #[tokio::test]
    async fn test_runtime_disconnect() {
        let events = vec![Err(GenxError::Other(anyhow::anyhow!("connection reset")))];
        let (t, _, recv_calls) = make_transformer(events, false);
        let input = InputStream {
            chunks: VecDeque::from(vec![Ok(MessageChunk::blob(
                Role::User,
                "audio/pcm",
                vec![1, 2],
            ))]),
        };
        let mut out = t.transform("doubao/realtime", Box::new(input)).await.unwrap();
        let err = out.next().await.unwrap_err();
        assert!(err.to_string().contains("connection reset"));
        assert_eq!(
            recv_calls.load(Ordering::SeqCst),
            1,
            "当前策略应为不重试，收到断连后直接失败"
        );
    }

    #[tokio::test]
    async fn test_server_error_event() {
        let events = vec![Ok(DoubaoServerEvent::SessionFailed {
            message: "resource ID mismatch".to_string(),
        })];
        let (t, _, _) = make_transformer(events, false);
        let input = InputStream {
            chunks: VecDeque::new(),
        };
        let mut out = t.transform("doubao/realtime", Box::new(input)).await.unwrap();
        let err = out.next().await.unwrap_err();
        assert!(err.to_string().contains("resource ID mismatch"));
    }

    #[tokio::test]
    async fn test_upstream_close_graceful() {
        let events = vec![Ok(DoubaoServerEvent::SessionEnded)];
        let (t, closed, _) = make_transformer(events, false);
        let input = InputStream {
            chunks: VecDeque::new(),
        };
        let mut out = t.transform("doubao/realtime", Box::new(input)).await.unwrap();
        assert_eq!(out.next().await.unwrap(), None);
        assert!(closed.load(Ordering::SeqCst));
    }

    #[tokio::test]
    async fn test_early_drop_output() {
        let events = vec![
            Ok(DoubaoServerEvent::ChatResponse {
                text: "hello".to_string(),
            }),
            Ok(DoubaoServerEvent::ChatResponse {
                text: "world".to_string(),
            }),
        ];
        let (t, _, _) = make_transformer(events, false);
        let input = InputStream {
            chunks: VecDeque::new(),
        };
        let mut out = t.transform("doubao/realtime", Box::new(input)).await.unwrap();
        let _ = out.next().await.unwrap();
        drop(out);
    }
}
