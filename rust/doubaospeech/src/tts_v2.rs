//! TTS V2 Service - BigModel TTS (大模型语音合成)
//!
//! V2 uses /api/v3/* endpoints with X-Api-* headers authentication.
//!
//! # Endpoints
//!
//! - POST `/api/v3/tts/unidirectional` - Unidirectional streaming HTTP
//! - WSS  `/api/v3/tts/unidirectional` - Unidirectional streaming WebSocket
//! - WSS  `/api/v3/tts/bidirection`    - Bidirectional WebSocket
//! - POST `/api/v3/tts/async/submit`   - Async long text synthesis
//!
//! # Authentication Headers
//!
//! - `X-Api-App-Id`: APP ID
//! - `X-Api-Access-Key`: Access Token
//! - `X-Api-Resource-Id`: Resource ID (see below)
//!
//! # Resource IDs
//!
//! - `seed-tts-1.0`: BigModel TTS 1.0 character-based (requires `*_moon_bigtts` voices)
//! - `seed-tts-2.0`: BigModel TTS 2.0 character-based (requires `*_uranus_bigtts` voices)
//! - `seed-tts-1.0-concurr`: BigModel TTS 1.0 concurrent version
//! - `seed-tts-2.0-concurr`: BigModel TTS 2.0 concurrent version
//!
//! # ⚠️ IMPORTANT: Speaker voice must match Resource ID!
//!
//! | Resource ID    | Required Speaker Suffix | Example                              |
//! |----------------|-------------------------|--------------------------------------|
//! | seed-tts-2.0   | `*_uranus_bigtts`       | `zh_female_xiaohe_uranus_bigtts` ✅  |
//! | seed-tts-1.0   | `*_moon_bigtts`         | `zh_female_shuangkuaisisi_moon_bigtts` |
//!
//! # Common Error
//!
//! ```text
//! {"code": 55000000, "message": "resource ID is mismatched with speaker related resource"}
//! ```
//!
//! This means speaker suffix doesn't match resource ID, **NOT** "service not enabled"!
//!
//! # Documentation
//!
//! <https://www.volcengine.com/docs/6561/1257584>

use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};

use async_stream::try_stream;
use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use bytes::Bytes;
use futures::stream::{SplitSink, SplitStream};
use futures::{SinkExt, Stream, StreamExt};
use serde::{Deserialize, Serialize};
use tokio::sync::{mpsc, Mutex};
use tokio_tungstenite::tungstenite::Message as WsMessage;
use tokio_tungstenite::{connect_async, MaybeTlsStream, WebSocketStream};

use crate::{
    client::RESOURCE_TTS_V2,
    error::{Error, Result},
    http::HttpClient,
    protocol::{BinaryProtocol, Message, MessageFlags, MessageType, SerializationType},
};

/// TTS V2 service provides BigModel TTS functionality.
pub struct TtsV2Service {
    http: Arc<HttpClient>,
}

impl TtsV2Service {
    /// Creates a new TTS V2 service.
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Performs streaming TTS synthesis over HTTP using V2 API.
    ///
    /// Uses endpoint: POST `/api/v3/tts/unidirectional`
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use futures::StreamExt;
    /// use giztoy_doubaospeech::{Client, TtsV2Request};
    ///
    /// # async fn example() -> anyhow::Result<()> {
    /// let client = Client::builder("app-id").api_key("api-key").build()?;
    /// let mut stream = client.tts_v2().stream(&TtsV2Request {
    ///     text: "你好，世界！".to_string(),
    ///     speaker: "zh_female_xiaohe_uranus_bigtts".to_string(),
    ///     resource_id: Some("seed-tts-2.0".to_string()),
    ///     ..Default::default()
    /// }).await?;
    ///
    /// while let Some(result) = stream.next().await {
    ///     let chunk = result?;
    ///     // Process chunk.audio
    /// }
    /// # Ok(())
    /// # }
    /// ```
    pub async fn stream(
        &self,
        req: &TtsV2Request,
    ) -> Result<impl Stream<Item = Result<TtsV2Chunk>>> {
        let resource_id = req.resource_id.as_deref().unwrap_or(RESOURCE_TTS_V2);
        let headers = self.http.v2_headers(Some(resource_id))?;
        
        let body = self.build_request_body(req);
        
        let url = format!("{}/api/v3/tts/unidirectional", self.http.base_url());
        
        let client = reqwest::Client::new();
        let response = client
            .post(&url)
            .headers(headers)
            .json(&body)
            .send()
            .await?;
        
        if !response.status().is_success() {
            let status = response.status().as_u16();
            let body_text = response.text().await.unwrap_or_default();
            return Err(Error::Api {
                code: status as i32,
                message: body_text,
                req_id: String::new(),
                log_id: String::new(),
                trace_id: String::new(),
                http_status: status,
            });
        }
        
        let byte_stream = response.bytes_stream();
        let mut stream = Box::pin(byte_stream);
        
        Ok(try_stream! {
            const MAX_BUFFER_SIZE: usize = 1024 * 1024;
            let mut buffer = Vec::<u8>::new();
            
            while let Some(result) = stream.next().await {
                let bytes: Bytes = result?;
                buffer.extend_from_slice(&bytes);
                
                if buffer.len() > MAX_BUFFER_SIZE {
                    Err(Error::Other(format!(
                        "stream buffer exceeded maximum size of {} bytes",
                        MAX_BUFFER_SIZE
                    )))?;
                    return;
                }
                
                // Process complete lines (JSON responses are newline-delimited)
                while let Some(newline_pos) = buffer.iter().position(|&b| b == b'\n') {
                    let line = buffer[..newline_pos].to_vec();
                    buffer = buffer[newline_pos + 1..].to_vec();
                    
                    let line_str = String::from_utf8_lossy(&line);
                    let line_str = line_str.trim();
                    
                    if line_str.is_empty() {
                        continue;
                    }
                    
                    // Try to parse as JSON response
                    let chunk_resp: TtsV2StreamResponse = match serde_json::from_str(line_str) {
                        Ok(c) => c,
                        Err(_) => {
                            // Not a JSON line, skip
                            continue;
                        }
                    };
                    
                    // Check for errors (code 0 or 20000000 is success)
                    if chunk_resp.code != 0 && chunk_resp.code != 20000000 {
                        Err(Error::Api {
                            code: chunk_resp.code,
                            message: chunk_resp.message.unwrap_or_default(),
                            req_id: chunk_resp.reqid.clone().unwrap_or_default(),
                            log_id: String::new(),
                            trace_id: String::new(),
                            http_status: 200,
                        })?;
                        return;
                    }
                    
                    // Decode base64 audio data
                    let audio = if let Some(ref data) = chunk_resp.data {
                        if !data.is_empty() {
                            BASE64.decode(data)?
                        } else {
                            vec![]
                        }
                    } else {
                        vec![]
                    };
                    
                    let is_last = chunk_resp.done;
                    
                    yield TtsV2Chunk {
                        audio,
                        is_last,
                        req_id: chunk_resp.reqid.unwrap_or_default(),
                    };
                    
                    if is_last {
                        return;
                    }
                }
            }
        })
    }

    /// Builds the request body for TTS V2 API.
    fn build_request_body(&self, req: &TtsV2Request) -> TtsV2RequestBody {
        let mut audio_params = AudioParams::default();
        if let Some(ref format) = req.format {
            audio_params.format = Some(format.clone());
        }
        if let Some(sample_rate) = req.sample_rate {
            audio_params.sample_rate = Some(sample_rate);
        }
        if let Some(bit_rate) = req.bit_rate {
            audio_params.bit_rate = Some(bit_rate);
        }
        if let Some(speed_ratio) = req.speed_ratio {
            audio_params.speed_ratio = Some(speed_ratio);
        }
        if let Some(volume_ratio) = req.volume_ratio {
            audio_params.volume_ratio = Some(volume_ratio);
        }
        if let Some(pitch_ratio) = req.pitch_ratio {
            audio_params.pitch_ratio = Some(pitch_ratio);
        }
        if let Some(ref emotion) = req.emotion {
            audio_params.emotion = Some(emotion.clone());
        }
        if let Some(ref language) = req.language {
            audio_params.language = Some(language.clone());
        }

        TtsV2RequestBody {
            user: UserInfo {
                uid: self.http.auth().user_id.clone(),
            },
            req_params: ReqParams {
                text: req.text.clone(),
                speaker: req.speaker.clone(),
                audio_params,
            },
        }
    }
}

// ================== Request Types ==================

/// TTS V2 synthesis request.
#[derive(Debug, Clone, Default)]
pub struct TtsV2Request {
    /// Text to synthesize (required).
    pub text: String,
    
    /// Speaker voice type (required).
    ///
    /// Examples:
    /// - `zh_female_xiaohe_uranus_bigtts` (for seed-tts-2.0)
    /// - `zh_female_shuangkuaisisi_moon_bigtts` (for seed-tts-1.0)
    pub speaker: String,
    
    /// Resource ID (default: seed-tts-2.0).
    ///
    /// Available options:
    /// - `seed-tts-1.0` - requires `*_moon_bigtts` voices
    /// - `seed-tts-2.0` - requires `*_uranus_bigtts` voices
    pub resource_id: Option<String>,
    
    /// Audio format: `pcm`, `mp3`, `ogg_opus` (default: mp3).
    pub format: Option<String>,
    
    /// Sample rate: 8000, 16000, 24000, 32000 (default: 24000).
    pub sample_rate: Option<i32>,
    
    /// Bit rate for mp3: 32000, 64000, 128000.
    pub bit_rate: Option<i32>,
    
    /// Speed ratio (0.2-3.0, default 1.0).
    pub speed_ratio: Option<f64>,
    
    /// Volume ratio (0.1-3.0, default 1.0).
    pub volume_ratio: Option<f64>,
    
    /// Pitch ratio (0.1-3.0, default 1.0).
    pub pitch_ratio: Option<f64>,
    
    /// Emotion: happy, sad, angry, fear, hate, surprise.
    pub emotion: Option<String>,
    
    /// Language: zh, en, ja, etc.
    pub language: Option<String>,
}

/// TTS V2 streaming chunk.
#[derive(Debug, Clone)]
pub struct TtsV2Chunk {
    /// Audio data (binary).
    pub audio: Vec<u8>,
    
    /// Whether this is the last chunk.
    pub is_last: bool,
    
    /// Request ID.
    pub req_id: String,
}

// ================== Internal Request/Response Types ==================

#[derive(Debug, Serialize)]
struct TtsV2RequestBody {
    user: UserInfo,
    req_params: ReqParams,
}

#[derive(Debug, Serialize)]
struct UserInfo {
    uid: String,
}

#[derive(Debug, Serialize)]
struct ReqParams {
    text: String,
    speaker: String,
    audio_params: AudioParams,
}

#[derive(Debug, Serialize, Default)]
struct AudioParams {
    #[serde(skip_serializing_if = "Option::is_none")]
    format: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    sample_rate: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    bit_rate: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    speed_ratio: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    volume_ratio: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pitch_ratio: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    emotion: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    language: Option<String>,
}

#[derive(Debug, Deserialize)]
struct TtsV2StreamResponse {
    #[serde(default)]
    code: i32,
    message: Option<String>,
    reqid: Option<String>,
    #[serde(default)]
    done: bool,
    data: Option<String>,
}

// ================== Bidirectional WebSocket ==================

/// TTS V2 WebSocket event types.
pub mod tts_v2_events {
    // Upstream events (client -> server)
    /// Start connection request.
    pub const START_CONNECTION: i32 = 1;
    /// Finish connection request.
    pub const FINISH_CONNECTION: i32 = 2;
    /// Start session request.
    pub const START_SESSION: i32 = 100;
    /// Cancel session request.
    pub const CANCEL_SESSION: i32 = 101;
    /// Finish session request.
    pub const FINISH_SESSION: i32 = 102;
    /// Task request (send text).
    pub const TASK_REQUEST: i32 = 200;

    // Downstream events (server -> client)
    /// Connection started.
    pub const CONNECTION_STARTED: i32 = 50;
    /// Connection failed.
    pub const CONNECTION_FAILED: i32 = 51;
    /// Connection finished.
    pub const CONNECTION_FINISHED: i32 = 52;
    /// Session started.
    pub const SESSION_STARTED: i32 = 150;
    /// Session canceled.
    pub const SESSION_CANCELED: i32 = 151;
    /// Session finished.
    pub const SESSION_FINISHED: i32 = 152;
    /// Session failed.
    pub const SESSION_FAILED: i32 = 153;
    /// TTS sentence start.
    pub const TTS_SENTENCE_START: i32 = 350;
    /// TTS sentence end.
    pub const TTS_SENTENCE_END: i32 = 351;
    /// TTS response (audio chunk).
    pub const TTS_RESPONSE: i32 = 352;
}

type WsStream = WebSocketStream<MaybeTlsStream<tokio::net::TcpStream>>;

/// TTS V2 bidirectional WebSocket session.
pub struct TtsV2Session {
    write: Arc<Mutex<SplitSink<WsStream, WsMessage>>>,
    proto: BinaryProtocol,
    session_id: String,
    user_id: String,
    config: TtsV2Request,
    sequence: i32,
    recv_rx: mpsc::Receiver<TtsV2Chunk>,
    closed: Arc<AtomicBool>,
}

impl TtsV2Service {
    /// Opens a bidirectional WebSocket session for TTS synthesis.
    ///
    /// Uses endpoint: WSS `/api/v3/tts/bidirection`
    ///
    /// This supports sending multiple text segments and receiving audio in real-time.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_doubaospeech::{Client, TtsV2Request};
    ///
    /// # async fn example() -> anyhow::Result<()> {
    /// let client = Client::builder("app-id").api_key("api-key").build()?;
    /// let mut session = client.tts_v2().bidirectional(&TtsV2Request {
    ///     speaker: "zh_female_xiaohe_uranus_bigtts".to_string(),
    ///     resource_id: Some("seed-tts-2.0".to_string()),
    ///     ..Default::default()
    /// }).await?;
    ///
    /// // Send text
    /// session.send_text("你好，", false).await?;
    /// session.send_text("世界！", true).await?;  // isLast=true triggers completion
    ///
    /// // Receive audio
    /// while let Some(result) = session.recv().await {
    ///     let chunk = result?;
    ///     // Process chunk.audio
    ///     if chunk.is_last {
    ///         break;
    ///     }
    /// }
    /// # Ok(())
    /// # }
    /// ```
    pub async fn bidirectional(&self, config: &TtsV2Request) -> Result<TtsV2Session> {
        use tokio_tungstenite::tungstenite::client::IntoClientRequest;

        let auth = self.http.auth();
        let resource_id = config.resource_id.as_deref().unwrap_or(RESOURCE_TTS_V2);

        // Build WebSocket URL
        let ws_url = format!("{}/api/v3/tts/bidirection", self.http.ws_url());

        // Build request with headers
        let mut request = ws_url.into_client_request()
            .map_err(|e| Error::Other(format!("build ws request: {}", e)))?;

        let headers = request.headers_mut();
        headers.insert("X-Api-App-Id", auth.app_id.parse().unwrap());
        headers.insert("X-Api-Resource-Id", resource_id.parse().unwrap());

        if let Some(ref token) = auth.access_token {
            headers.insert("X-Api-Access-Key", token.parse().unwrap());
        } else if let Some(ref api_key) = auth.api_key {
            headers.insert("X-Api-Access-Key", api_key.parse().unwrap());
        } else if let Some(ref access_key) = auth.access_key {
            headers.insert("X-Api-Access-Key", access_key.parse().unwrap());
        }

        let (ws_stream, _) = connect_async(request)
            .await
            .map_err(Error::WebSocket)?;

        let (write, read) = ws_stream.split();
        let write = Arc::new(Mutex::new(write));

        // Generate session ID
        let session_id = generate_session_id();

        // Create protocol handler
        let mut proto = BinaryProtocol::new();
        proto.set_serialization(SerializationType::Json);

        // Create channel for receiving chunks
        let (recv_tx, recv_rx) = mpsc::channel(100);
        let closed = Arc::new(AtomicBool::new(false));

        let mut session = TtsV2Session {
            write: write.clone(),
            proto,
            session_id: session_id.clone(),
            user_id: auth.user_id.clone(),
            config: config.clone(),
            sequence: 0,
            recv_rx,
            closed: closed.clone(),
        };

        // Start receive loop
        let proto_clone = session.proto.clone();
        tokio::spawn(async move {
            receive_loop(read, recv_tx, proto_clone, closed).await;
        });

        // Step 1: Send StartConnection
        session.send_start_connection().await?;

        // Step 2: Wait for ConnectionStarted
        match tokio::time::timeout(std::time::Duration::from_secs(10), session.recv_rx.recv()).await {
            Ok(Some(_)) => {}
            Ok(None) => return Err(Error::Other("connection closed".to_string())),
            Err(_) => return Err(Error::Other("timeout waiting for ConnectionStarted".to_string())),
        }

        // Step 3: Send StartSession
        session.send_session_start().await?;

        // Step 4: Wait for SessionStarted
        match tokio::time::timeout(std::time::Duration::from_secs(10), session.recv_rx.recv()).await {
            Ok(Some(_)) => {}
            Ok(None) => return Err(Error::Other("connection closed".to_string())),
            Err(_) => return Err(Error::Other("timeout waiting for SessionStarted".to_string())),
        }

        Ok(session)
    }
}

impl TtsV2Session {
    /// Sends text to synthesize.
    ///
    /// If `is_last` is true, this marks the end of the text stream and
    /// automatically sends FinishSession to trigger audio completion.
    pub async fn send_text(&mut self, text: &str, is_last: bool) -> Result<()> {
        self.sequence += 1;
        self.send_task_request(text).await?;

        if is_last {
            self.send_finish_session().await?;
        }

        Ok(())
    }

    /// Receives the next audio chunk.
    ///
    /// Returns `None` when the session is closed.
    pub async fn recv(&mut self) -> Option<Result<TtsV2Chunk>> {
        self.recv_rx.recv().await.map(Ok)
    }

    /// Closes the session.
    pub async fn close(&self) -> Result<()> {
        if self.closed.swap(true, Ordering::Relaxed) {
            return Ok(());
        }
        self.write.lock().await.close().await.map_err(Error::WebSocket)?;
        Ok(())
    }

    async fn send_start_connection(&self) -> Result<()> {
        let payload = serde_json::json!({
            "namespace": "BidirectionalTTS",
        });
        self.send_connect_message(tts_v2_events::START_CONNECTION, &payload).await
    }

    async fn send_session_start(&self) -> Result<()> {
        let mut audio_params = serde_json::Map::new();
        if let Some(ref format) = self.config.format {
            audio_params.insert("format".to_string(), serde_json::json!(format));
        }
        if let Some(sample_rate) = self.config.sample_rate {
            audio_params.insert("sample_rate".to_string(), serde_json::json!(sample_rate));
        }
        if let Some(speed_ratio) = self.config.speed_ratio {
            audio_params.insert("speed_ratio".to_string(), serde_json::json!(speed_ratio));
        }
        if let Some(volume_ratio) = self.config.volume_ratio {
            audio_params.insert("volume_ratio".to_string(), serde_json::json!(volume_ratio));
        }
        if let Some(pitch_ratio) = self.config.pitch_ratio {
            audio_params.insert("pitch_ratio".to_string(), serde_json::json!(pitch_ratio));
        }
        if let Some(ref emotion) = self.config.emotion {
            audio_params.insert("emotion".to_string(), serde_json::json!(emotion));
        }
        if let Some(ref language) = self.config.language {
            audio_params.insert("language".to_string(), serde_json::json!(language));
        }

        let payload = serde_json::json!({
            "user": {
                "uid": self.user_id,
            },
            "event": tts_v2_events::START_SESSION,
            "req_params": {
                "speaker": self.config.speaker,
                "audio_params": audio_params,
            },
        });

        self.send_session_message(tts_v2_events::START_SESSION, &payload).await
    }

    async fn send_task_request(&self, text: &str) -> Result<()> {
        let payload = serde_json::json!({
            "user": {
                "uid": self.user_id,
            },
            "event": tts_v2_events::TASK_REQUEST,
            "req_params": {
                "text": text,
            },
        });

        self.send_session_message(tts_v2_events::TASK_REQUEST, &payload).await
    }

    async fn send_finish_session(&self) -> Result<()> {
        self.send_session_message(tts_v2_events::FINISH_SESSION, &serde_json::json!({})).await
    }

    /// Sends a connection-class message (no session_id).
    async fn send_connect_message(&self, event: i32, payload: &serde_json::Value) -> Result<()> {
        let json_data = serde_json::to_vec(payload)
            .map_err(|e| Error::Other(format!("serialize payload: {}", e)))?;

        // For connection events, we need custom marshal without session_id
        // Using Message with session_id empty and has_event
        let msg = Message {
            msg_type: MessageType::FullClient,
            flags: MessageFlags::WithEvent,
            event,
            session_id: String::new(), // Empty for connection events
            payload: json_data,
            ..Default::default()
        };

        // Marshal using protocol but we need special handling for connection events
        // Connection events don't include session_id length/data
        let data = self.marshal_connect_message(event, &msg.payload)?;

        self.write.lock().await
            .send(WsMessage::Binary(data.into()))
            .await
            .map_err(Error::WebSocket)
    }

    /// Sends a session-class message (with session_id).
    async fn send_session_message(&self, event: i32, payload: &serde_json::Value) -> Result<()> {
        let json_data = serde_json::to_vec(payload)
            .map_err(|e| Error::Other(format!("serialize payload: {}", e)))?;

        let msg = Message {
            msg_type: MessageType::FullClient,
            flags: MessageFlags::WithEvent,
            event,
            session_id: self.session_id.clone(),
            payload: json_data,
            ..Default::default()
        };

        let data = self.proto.marshal(&msg)?;

        self.write.lock().await
            .send(WsMessage::Binary(data.into()))
            .await
            .map_err(Error::WebSocket)
    }

    /// Custom marshal for connection events (no session_id).
    fn marshal_connect_message(&self, event: i32, payload: &[u8]) -> Result<Vec<u8>> {
        use bytes::{BufMut, BytesMut};

        let mut buf = BytesMut::with_capacity(256);

        // Header (4 bytes)
        buf.put_u8(0x11); // version=1, header_size=1
        buf.put_u8(0x14); // msg_type=1, flags=4 (with-event)
        buf.put_u8(0x10); // serialization=JSON, compression=none
        buf.put_u8(0x00); // reserved

        // Event number (4 bytes, big endian)
        buf.put_i32(event);

        // NO session_id for connection events

        // Payload length + payload
        buf.put_u32(payload.len() as u32);
        buf.put_slice(payload);

        Ok(buf.to_vec())
    }
}

/// Receives messages from WebSocket and sends to channel.
async fn receive_loop(
    mut read: SplitStream<WsStream>,
    tx: mpsc::Sender<TtsV2Chunk>,
    proto: BinaryProtocol,
    closed: Arc<AtomicBool>,
) {
    while !closed.load(Ordering::Relaxed) {
        let msg = match read.next().await {
            Some(Ok(msg)) => msg,
            Some(Err(_)) => break,
            None => break,
        };

        match msg {
            WsMessage::Binary(data) => {
                match parse_tts_response(&proto, &data) {
                    Ok(Some(chunk)) => {
                        let is_last = chunk.is_last;
                        if tx.send(chunk).await.is_err() {
                            break;
                        }
                        if is_last {
                            break;
                        }
                    }
                    Ok(None) => continue,
                    Err(_) => break,
                }
            }
            WsMessage::Close(_) => break,
            _ => continue,
        }
    }
}

/// Parses a TTS V2 WebSocket response.
fn parse_tts_response(proto: &BinaryProtocol, data: &[u8]) -> Result<Option<TtsV2Chunk>> {
    let msg = proto.unmarshal(data)?;

    // Check for error
    if msg.is_error() {
        if let Ok(err_resp) = serde_json::from_slice::<serde_json::Value>(&msg.payload) {
            let code = err_resp.get("code").and_then(|v| v.as_i64()).unwrap_or(-1) as i32;
            let message = err_resp.get("message").and_then(|v| v.as_str()).unwrap_or("unknown error").to_string();
            return Err(Error::api(code, message, 200));
        }
    }

    let event_code = msg.event;

    match event_code {
        tts_v2_events::CONNECTION_STARTED => {
            // Connection started - return empty chunk to signal progress
            Ok(Some(TtsV2Chunk {
                audio: vec![],
                is_last: false,
                req_id: String::new(),
            }))
        }
        tts_v2_events::SESSION_STARTED => {
            // Session started - return empty chunk to signal progress
            Ok(Some(TtsV2Chunk {
                audio: vec![],
                is_last: false,
                req_id: String::new(),
            }))
        }
        tts_v2_events::TTS_RESPONSE => {
            // Audio response
            let audio = if msg.is_audio_only() {
                msg.payload
            } else if let Ok(resp) = serde_json::from_slice::<serde_json::Value>(&msg.payload) {
                // Try to decode base64 audio from JSON
                if let Some(data) = resp.get("data").and_then(|v| v.as_str()) {
                    BASE64.decode(data).unwrap_or_default()
                } else {
                    vec![]
                }
            } else {
                msg.payload
            };

            Ok(Some(TtsV2Chunk {
                audio,
                is_last: false,
                req_id: String::new(),
            }))
        }
        tts_v2_events::SESSION_FINISHED => {
            Ok(Some(TtsV2Chunk {
                audio: vec![],
                is_last: true,
                req_id: String::new(),
            }))
        }
        tts_v2_events::SESSION_FAILED => {
            if let Ok(err_resp) = serde_json::from_slice::<serde_json::Value>(&msg.payload) {
                let code = err_resp.get("code").and_then(|v| v.as_i64()).unwrap_or(-1) as i32;
                let message = err_resp.get("message").and_then(|v| v.as_str()).unwrap_or("session failed").to_string();
                return Err(Error::api(code, message, 200));
            }
            Err(Error::Other("session failed".to_string()))
        }
        _ => {
            // Other events - skip
            Ok(None)
        }
    }
}

/// Generates a 12-character session ID.
fn generate_session_id() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};
    let chars: Vec<char> = "abcdefghijklmnopqrstuvwxyz0123456789".chars().collect();
    let mut result = String::with_capacity(12);
    let mut seed = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos() as u64;
    
    for _ in 0..12 {
        result.push(chars[(seed % chars.len() as u64) as usize]);
        seed = seed.wrapping_mul(1103515245).wrapping_add(12345);
    }
    result
}
