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
pub enum DashScopeServerEvent {
    SessionCreated,
    InputAudioTranscriptionCompleted { transcript: String },
    ResponseCreated,
    ResponseTextDelta { delta: String },
    ResponseTextDone,
    ResponseAudioDelta { data: Vec<u8> },
    ResponseAudioDone,
    Error { message: String },
}

#[async_trait]
pub trait DashScopeRealtimeSession: Send + Sync {
    async fn update_session(&self, config: &giztoy_dashscope::SessionConfig) -> Result<(), GenxError>;
    async fn append_audio(&self, audio: &[u8]) -> Result<(), GenxError>;
    async fn commit_input(&self) -> Result<(), GenxError>;
    async fn create_response(&self) -> Result<(), GenxError>;
    async fn cancel_response(&self) -> Result<(), GenxError>;
    async fn recv(&self) -> Option<Result<DashScopeServerEvent, GenxError>>;
    async fn close(&self) -> Result<(), GenxError>;
}

#[async_trait]
pub trait DashScopeRealtimeConnector: Send + Sync {
    async fn connect(&self) -> Result<Box<dyn DashScopeRealtimeSession>, GenxError>;
}

pub struct DashScopeRealtime {
    connector: Arc<dyn DashScopeRealtimeConnector>,
    session_config: giztoy_dashscope::SessionConfig,
    output_mime: String,
    input_chunk_size: usize,
}

impl DashScopeRealtime {
    pub fn new(connector: Arc<dyn DashScopeRealtimeConnector>) -> Self {
        Self {
            connector,
            session_config: giztoy_dashscope::SessionConfig {
                modalities: Some(vec!["text".to_string(), "audio".to_string()]),
                enable_input_audio_transcription: Some(true),
                ..Default::default()
            },
            output_mime: "audio/pcm".to_string(),
            input_chunk_size: 3200,
        }
    }

    pub fn with_session_config(mut self, cfg: giztoy_dashscope::SessionConfig) -> Self {
        self.session_config = cfg;
        self
    }

    pub fn with_output_mime(mut self, mime: impl Into<String>) -> Self {
        self.output_mime = mime.into();
        self
    }
}

#[async_trait]
impl Transformer for DashScopeRealtime {
    async fn transform(
        &self,
        _pattern: &str,
        input: Box<dyn Stream>,
    ) -> Result<Box<dyn Stream>, GenxError> {
        let session = self.connector.connect().await?;
        let session: Arc<dyn DashScopeRealtimeSession> = Arc::from(session);

        wait_for_session_created(&*session).await?;
        session.update_session(&self.session_config).await?;

        let (tx, rx) = mpsc::channel(128);
        let output = ChannelStream { rx };
        let output_mime = self.output_mime.clone();
        let input_chunk_size = self.input_chunk_size;

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
                                    Arc::clone(&stream_state),
                                    &output_mime,
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

            let mut audio_buf = Vec::new();
            loop {
                match input.next().await {
                    Ok(Some(chunk)) => {
                        if let Err(e) = forward_input(
                            &*session,
                            Arc::clone(&stream_state),
                            chunk,
                            &mut audio_buf,
                            input_chunk_size,
                        )
                        .await
                        {
                            let _ = tx.send(Err(e)).await;
                            break;
                        }
                    }
                    Ok(None) => {
                        let _ = flush_audio_buffer(&*session, &mut audio_buf, input_chunk_size, true).await;
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

async fn wait_for_session_created(session: &dyn DashScopeRealtimeSession) -> Result<(), GenxError> {
    loop {
        match session.recv().await {
            Some(Ok(DashScopeServerEvent::SessionCreated)) => return Ok(()),
            Some(Ok(DashScopeServerEvent::Error { message })) => {
                return Err(GenxError::Other(anyhow::anyhow!(
                    "dashscope session init failed: {}",
                    message
                )));
            }
            Some(Ok(_)) => continue,
            Some(Err(e)) => return Err(e),
            None => {
                return Err(GenxError::Other(anyhow::anyhow!(
                    "dashscope session.created not received"
                )));
            }
        }
    }
}

#[derive(Default)]
struct StreamState {
    queued_stream_ids: VecDeque<String>,
    response_stream_id: String,
    response_stream_assigned_for_turn: bool,
}

impl StreamState {
    fn push_stream_id(&mut self, id: String) {
        if id.is_empty() {
            return;
        }
        if self.queued_stream_ids.back().is_none_or(|last| last != &id) {
            self.queued_stream_ids.push_back(id);
        }
    }

    fn pop_for_response(&mut self) {
        self.response_stream_id = self
            .queued_stream_ids
            .pop_front()
            .unwrap_or_else(new_stream_id);
        self.response_stream_assigned_for_turn = true;
    }

    fn ensure_response_stream_id_for_turn(&mut self) {
        if !self.response_stream_assigned_for_turn {
            self.pop_for_response();
        }
    }

    fn reset_turn_assignment(&mut self) {
        self.response_stream_assigned_for_turn = false;
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
    session: &dyn DashScopeRealtimeSession,
    state: Arc<Mutex<StreamState>>,
    chunk: MessageChunk,
    audio_buf: &mut Vec<u8>,
    chunk_size: usize,
) -> Result<(), GenxError> {
    if let Some(ctrl) = &chunk.ctrl {
        if !ctrl.stream_id.is_empty() {
            state.lock().await.push_stream_id(ctrl.stream_id.clone());
        }
        if ctrl.begin_of_stream {
            let _ = session.cancel_response().await;
        }
    }

    if chunk.is_end_of_stream() {
        flush_audio_buffer(session, audio_buf, chunk_size, true).await?;
        session.commit_input().await?;
        session.create_response().await?;
        return Ok(());
    }

    if let Some(Part::Blob(blob)) = chunk.part {
        if !blob.data.is_empty() {
            audio_buf.extend_from_slice(&blob.data);
            flush_audio_buffer(session, audio_buf, chunk_size, false).await?;
        }
    }

    Ok(())
}

async fn flush_audio_buffer(
    session: &dyn DashScopeRealtimeSession,
    audio_buf: &mut Vec<u8>,
    chunk_size: usize,
    flush_remainder: bool,
) -> Result<(), GenxError> {
    while audio_buf.len() >= chunk_size {
        let chunk: Vec<u8> = audio_buf.drain(0..chunk_size).collect();
        session.append_audio(&chunk).await?;
    }
    if flush_remainder && !audio_buf.is_empty() {
        let chunk: Vec<u8> = std::mem::take(audio_buf);
        session.append_audio(&chunk).await?;
    }
    Ok(())
}

async fn handle_event(
    tx: &mpsc::Sender<Result<MessageChunk, GenxError>>,
    state: Arc<Mutex<StreamState>>,
    output_mime: &str,
    event: DashScopeServerEvent,
) -> Result<(), GenxError> {
    match event {
        DashScopeServerEvent::SessionCreated => Ok(()),
        DashScopeServerEvent::InputAudioTranscriptionCompleted { transcript } => {
            let mut st = state.lock().await;
            st.ensure_response_stream_id_for_turn();
            if transcript.is_empty() {
                return Ok(());
            }
            let stream_id = st.current_response_stream_id();
            drop(st);
            send_chunk(
                tx,
                MessageChunk {
                    role: Role::User,
                    name: None,
                    part: Some(Part::Text(transcript)),
                    tool_call: None,
                    ctrl: Some(StreamCtrl {
                        stream_id: stream_id.clone(),
                        ..Default::default()
                    }),
                },
            )
            .await?;
            send_chunk(
                tx,
                MessageChunk {
                    role: Role::User,
                    name: None,
                    part: Some(Part::Text(String::new())),
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
        DashScopeServerEvent::ResponseCreated => {
            let mut st = state.lock().await;
            st.ensure_response_stream_id_for_turn();
            let stream_id = st.current_response_stream_id();
            drop(st);
            send_chunk(
                tx,
                MessageChunk {
                    role: Role::Model,
                    name: None,
                    part: Some(Part::blob(output_mime, Vec::<u8>::new())),
                    tool_call: None,
                    ctrl: Some(StreamCtrl {
                        stream_id,
                        begin_of_stream: true,
                        ..Default::default()
                    }),
                },
            )
            .await
        }
        DashScopeServerEvent::ResponseTextDelta { delta } => {
            if delta.is_empty() {
                return Ok(());
            }
            let stream_id = state.lock().await.current_response_stream_id();
            send_chunk(
                tx,
                MessageChunk {
                    role: Role::Model,
                    name: None,
                    part: Some(Part::Text(delta)),
                    tool_call: None,
                    ctrl: Some(StreamCtrl {
                        stream_id,
                        ..Default::default()
                    }),
                },
            )
            .await
        }
        DashScopeServerEvent::ResponseTextDone => {
            let mut st = state.lock().await;
            let stream_id = st.current_response_stream_id();
            st.reset_turn_assignment();
            drop(st);
            send_chunk(
                tx,
                MessageChunk {
                    role: Role::Model,
                    name: None,
                    part: Some(Part::Text(String::new())),
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
        DashScopeServerEvent::ResponseAudioDelta { data } => {
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
        DashScopeServerEvent::ResponseAudioDone => {
            let mut st = state.lock().await;
            let stream_id = st.current_response_stream_id();
            st.reset_turn_assignment();
            drop(st);
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
        DashScopeServerEvent::Error { message } => Err(GenxError::Generation {
            usage: Default::default(),
            message,
        }),
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

pub struct DashScopeSdkConnector {
    client: giztoy_dashscope::Client,
    cfg: giztoy_dashscope::RealtimeConfig,
}

impl DashScopeSdkConnector {
    pub fn new(client: giztoy_dashscope::Client, cfg: giztoy_dashscope::RealtimeConfig) -> Self {
        Self { client, cfg }
    }
}

#[async_trait]
impl DashScopeRealtimeConnector for DashScopeSdkConnector {
    async fn connect(&self) -> Result<Box<dyn DashScopeRealtimeSession>, GenxError> {
        let session = self
            .client
            .realtime()
            .connect(&self.cfg)
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("dashscope connect failed: {}", e)))?;
        Ok(Box::new(DashScopeSdkSession {
            inner: Arc::new(Mutex::new(session)),
        }))
    }
}

struct DashScopeSdkSession {
    inner: Arc<Mutex<giztoy_dashscope::RealtimeSession>>,
}

#[async_trait]
impl DashScopeRealtimeSession for DashScopeSdkSession {
    async fn update_session(&self, config: &giztoy_dashscope::SessionConfig) -> Result<(), GenxError> {
        self.inner
            .lock()
            .await
            .update_session(config)
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("dashscope update_session failed: {}", e)))
    }

    async fn append_audio(&self, audio: &[u8]) -> Result<(), GenxError> {
        self.inner
            .lock()
            .await
            .append_audio(audio)
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("dashscope append_audio failed: {}", e)))
    }

    async fn commit_input(&self) -> Result<(), GenxError> {
        self.inner
            .lock()
            .await
            .commit_input()
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("dashscope commit_input failed: {}", e)))
    }

    async fn create_response(&self) -> Result<(), GenxError> {
        self.inner
            .lock()
            .await
            .create_response(None)
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("dashscope create_response failed: {}", e)))
    }

    async fn cancel_response(&self) -> Result<(), GenxError> {
        self.inner
            .lock()
            .await
            .cancel_response()
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("dashscope cancel_response failed: {}", e)))
    }

    async fn recv(&self) -> Option<Result<DashScopeServerEvent, GenxError>> {
        let evt = self.inner.lock().await.recv().await?;
        match evt {
            Ok(e) => Some(Ok(map_dashscope_event(e))),
            Err(e) => Some(Err(GenxError::Other(anyhow::anyhow!(
                "dashscope recv failed: {}",
                e
            )))),
        }
    }

    async fn close(&self) -> Result<(), GenxError> {
        self.inner
            .lock()
            .await
            .close()
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("dashscope close failed: {}", e)))
    }
}

fn map_dashscope_event(evt: giztoy_dashscope::RealtimeEvent) -> DashScopeServerEvent {
    use giztoy_dashscope::*;
    match evt.event_type.as_str() {
        EVENT_TYPE_SESSION_CREATED => DashScopeServerEvent::SessionCreated,
        EVENT_TYPE_INPUT_AUDIO_TRANSCRIPTION_COMPLETED => {
            DashScopeServerEvent::InputAudioTranscriptionCompleted {
                transcript: evt.transcript.unwrap_or_default(),
            }
        }
        EVENT_TYPE_RESPONSE_CREATED => DashScopeServerEvent::ResponseCreated,
        EVENT_TYPE_RESPONSE_TEXT_DELTA => DashScopeServerEvent::ResponseTextDelta {
            delta: evt.delta.unwrap_or_default(),
        },
        EVENT_TYPE_RESPONSE_TEXT_DONE => DashScopeServerEvent::ResponseTextDone,
        EVENT_TYPE_RESPONSE_AUDIO_DELTA => DashScopeServerEvent::ResponseAudioDelta {
            data: evt.audio.unwrap_or_default(),
        },
        EVENT_TYPE_RESPONSE_AUDIO_DONE => DashScopeServerEvent::ResponseAudioDone,
        EVENT_TYPE_ERROR => DashScopeServerEvent::Error {
            message: evt
                .error
                .and_then(|e| e.message)
                .unwrap_or_else(|| "dashscope error".to_string()),
        },
        _ => DashScopeServerEvent::Error {
            message: format!("unsupported event: {}", evt.event_type),
        },
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicUsize, Ordering};

    struct MockSession {
        events: Arc<Mutex<VecDeque<Result<DashScopeServerEvent, GenxError>>>>,
        appended_audio_sizes: Arc<Mutex<Vec<usize>>>,
        recv_calls: Arc<AtomicUsize>,
    }

    #[async_trait]
    impl DashScopeRealtimeSession for MockSession {
        async fn update_session(
            &self,
            _config: &giztoy_dashscope::SessionConfig,
        ) -> Result<(), GenxError> {
            Ok(())
        }

        async fn append_audio(&self, _audio: &[u8]) -> Result<(), GenxError> {
            self.appended_audio_sizes.lock().await.push(_audio.len());
            Ok(())
        }

        async fn commit_input(&self) -> Result<(), GenxError> {
            Ok(())
        }

        async fn create_response(&self) -> Result<(), GenxError> {
            Ok(())
        }

        async fn cancel_response(&self) -> Result<(), GenxError> {
            Ok(())
        }

        async fn recv(&self) -> Option<Result<DashScopeServerEvent, GenxError>> {
            self.recv_calls.fetch_add(1, Ordering::SeqCst);
            self.events.lock().await.pop_front()
        }

        async fn close(&self) -> Result<(), GenxError> {
            Ok(())
        }
    }

    struct StaticConnector {
        session: Arc<Mutex<Option<Box<dyn DashScopeRealtimeSession>>>>,
    }

    #[async_trait]
    impl DashScopeRealtimeConnector for StaticConnector {
        async fn connect(&self) -> Result<Box<dyn DashScopeRealtimeSession>, GenxError> {
            self.session
                .lock()
                .await
                .take()
                .ok_or_else(|| GenxError::Other(anyhow::anyhow!("session already taken")))
        }
    }

    struct FailingConnector;

    #[async_trait]
    impl DashScopeRealtimeConnector for FailingConnector {
        async fn connect(&self) -> Result<Box<dyn DashScopeRealtimeSession>, GenxError> {
            Err(GenxError::Other(anyhow::anyhow!("connection refused")))
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
        events: Vec<Result<DashScopeServerEvent, GenxError>>,
    ) -> (DashScopeRealtime, Arc<Mutex<Vec<usize>>>, Arc<AtomicUsize>) {
        let appended_audio_sizes = Arc::new(Mutex::new(Vec::new()));
        let recv_calls = Arc::new(AtomicUsize::new(0));
        let session = MockSession {
            events: Arc::new(Mutex::new(VecDeque::from(events))),
            appended_audio_sizes: Arc::clone(&appended_audio_sizes),
            recv_calls: Arc::clone(&recv_calls),
        };
        let connector = StaticConnector {
            session: Arc::new(Mutex::new(Some(Box::new(session)))),
        };
        (
            DashScopeRealtime::new(Arc::new(connector)),
            appended_audio_sizes,
            recv_calls,
        )
    }

    #[tokio::test]
    async fn test_full_conversation_flow() {
        let (t, _, _) = make_transformer(vec![
            Ok(DashScopeServerEvent::SessionCreated),
            Ok(DashScopeServerEvent::InputAudioTranscriptionCompleted {
                transcript: "你好".to_string(),
            }),
            Ok(DashScopeServerEvent::ResponseCreated),
            Ok(DashScopeServerEvent::ResponseTextDelta {
                delta: "你好".to_string(),
            }),
            Ok(DashScopeServerEvent::ResponseAudioDelta { data: vec![1, 2] }),
            Ok(DashScopeServerEvent::ResponseAudioDone),
        ]);
        let input = InputStream {
            chunks: VecDeque::from(vec![
                Ok(MessageChunk::new_begin_of_stream("stream-1")),
                Ok(MessageChunk::blob(Role::User, "audio/pcm", vec![9; 1000])),
                Ok(MessageChunk::new_end_of_stream("audio/pcm")),
            ]),
        };

        let mut out = t.transform("dashscope/realtime", Box::new(input)).await.unwrap();
        let c1 = out.next().await.unwrap().unwrap();
        assert_eq!(c1.role, Role::User);
        assert_eq!(c1.part.as_ref().and_then(|p| p.as_text()), Some("你好"));

        let c2 = out.next().await.unwrap().unwrap();
        assert!(c2.is_end_of_stream());
        assert_eq!(c2.role, Role::User);

        let c3 = out.next().await.unwrap().unwrap();
        assert!(c3.is_begin_of_stream());
        assert_eq!(c3.role, Role::Model);
        assert_eq!(
            c1.ctrl.as_ref().map(|c| c.stream_id.clone()),
            c3.ctrl.as_ref().map(|c| c.stream_id.clone()),
            "同一轮输入的 ASR 与模型输出必须共享 stream_id"
        );

        let c4 = out.next().await.unwrap().unwrap();
        assert_eq!(c4.part.as_ref().and_then(|p| p.as_text()), Some("你好"));

        let c5 = out.next().await.unwrap().unwrap();
        assert!(c5.part.as_ref().and_then(|p| p.as_blob()).is_some());

        let c6 = out.next().await.unwrap().unwrap();
        assert!(c6.is_end_of_stream());
    }

    #[tokio::test]
    async fn test_init_connection_failed() {
        let t = DashScopeRealtime::new(Arc::new(FailingConnector));
        let input = InputStream {
            chunks: VecDeque::new(),
        };
        let err = match t.transform("dashscope/realtime", Box::new(input)).await {
            Ok(_) => panic!("expected init error"),
            Err(e) => e,
        };
        assert!(err.to_string().contains("connection refused"));
    }

    #[tokio::test]
    async fn test_runtime_disconnect() {
        let (t, _, recv_calls) = make_transformer(vec![
            Ok(DashScopeServerEvent::SessionCreated),
            Err(GenxError::Other(anyhow::anyhow!("socket closed"))),
        ]);
        let input = InputStream {
            chunks: VecDeque::from(vec![Ok(MessageChunk::blob(
                Role::User,
                "audio/pcm",
                vec![1],
            ))]),
        };
        let mut out = t.transform("dashscope/realtime", Box::new(input)).await.unwrap();
        let err = out.next().await.unwrap_err();
        assert!(err.to_string().contains("socket closed"));
        assert_eq!(
            recv_calls.load(Ordering::SeqCst),
            2,
            "当前策略应为不重试，收到断连后直接失败"
        );
    }

    #[tokio::test]
    async fn test_audio_chunking_only_flush_remainder_on_eos() {
        let (t, appended_audio_sizes, _) = make_transformer(vec![
            Ok(DashScopeServerEvent::SessionCreated),
            Ok(DashScopeServerEvent::ResponseCreated),
            Ok(DashScopeServerEvent::ResponseAudioDone),
        ]);

        let input = InputStream {
            chunks: VecDeque::from(vec![
                Ok(MessageChunk::new_begin_of_stream("stream-2")),
                Ok(MessageChunk::blob(Role::User, "audio/pcm", vec![1; 1000])),
                Ok(MessageChunk::blob(Role::User, "audio/pcm", vec![2; 1000])),
                Ok(MessageChunk::blob(Role::User, "audio/pcm", vec![3; 1000])),
                Ok(MessageChunk::new_end_of_stream("audio/pcm")),
            ]),
        };

        let mut out = t.transform("dashscope/realtime", Box::new(input)).await.unwrap();
        while out.next().await.unwrap().is_some() {}

        let sizes = appended_audio_sizes.lock().await.clone();
        assert_eq!(sizes, vec![3000], "EOS 前不应发送 remainder，EOS 时一次性 flush");
    }
}
