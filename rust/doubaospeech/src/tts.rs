//! TTS (Text-to-Speech) service for Doubao Speech API.

use std::sync::Arc;

use async_stream::try_stream;
use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use futures::{Stream, StreamExt};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::{
    error::{status_code, Error, Result},
    http::HttpClient,
    types::{AudioEncoding, Language, SampleRate, SubtitleSegment, TaskStatus, TtsTextType},
};

/// TTS service provides text-to-speech synthesis functionality.
pub struct TtsService {
    http: Arc<HttpClient>,
}

impl TtsService {
    /// Creates a new TTS service.
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Performs synchronous TTS synthesis.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_doubaospeech::{Client, TtsRequest};
    ///
    /// let client = Client::builder("app-id").api_key("api-key").build()?;
    /// let response = client.tts().synthesize(&TtsRequest {
    ///     text: "你好，世界！".to_string(),
    ///     voice_type: "zh_female_cancan".to_string(),
    ///     ..Default::default()
    /// }).await?;
    /// // response.audio contains the audio data
    /// ```
    pub async fn synthesize(&self, req: &TtsRequest) -> Result<TtsResponse> {
        let tts_req = self.build_request(req);

        let url = "/api/v1/tts";
        let response: TtsApiResponse = self.http.request("POST", url, Some(&tts_req)).await?;

        if response.code != status_code::SUCCESS {
            return Err(Error::api_with_req_id(
                response.code,
                response.message,
                response.reqid,
                200,
            ));
        }

        // Decode audio data
        let audio = BASE64.decode(&response.data)?;
        let duration = response
            .addition
            .duration
            .parse::<i32>()
            .unwrap_or_default();

        Ok(TtsResponse {
            audio,
            duration,
            subtitles: vec![],
            req_id: response.reqid,
        })
    }

    /// Performs streaming TTS synthesis over HTTP.
    ///
    /// Returns an async stream of TTS chunks.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use futures::StreamExt;
    /// use giztoy_doubaospeech::{Client, TtsRequest};
    ///
    /// let client = Client::builder("app-id").api_key("api-key").build()?;
    /// let mut stream = client.tts().synthesize_stream(&TtsRequest {
    ///     text: "你好，世界！".to_string(),
    ///     voice_type: "zh_female_cancan".to_string(),
    ///     ..Default::default()
    /// }).await?;
    ///
    /// while let Some(result) = stream.next().await {
    ///     let chunk = result?;
    ///     // Process chunk.audio
    /// }
    /// ```
    pub async fn synthesize_stream(
        &self,
        req: &TtsRequest,
    ) -> Result<impl Stream<Item = Result<TtsChunk>>> {
        let tts_req = self.build_request(req);
        let byte_stream = self
            .http
            .request_stream("POST", "/api/v1/tts/stream", Some(tts_req))
            .await?;

        let mut stream = Box::pin(byte_stream);
        
        Ok(try_stream! {
            // Maximum buffer size to prevent memory exhaustion (1MB)
            const MAX_BUFFER_SIZE: usize = 1024 * 1024;

            let mut buffer = String::new();

            while let Some(result) = stream.next().await {
                let bytes = result?;
                buffer.push_str(&String::from_utf8_lossy(&bytes));

                // Check buffer size limit
                if buffer.len() > MAX_BUFFER_SIZE {
                    Err(Error::Other(format!(
                        "stream buffer exceeded maximum size of {} bytes without newline",
                        MAX_BUFFER_SIZE
                    )))?;
                    return;
                }

                // Process complete lines
                while let Some(newline_pos) = buffer.find('\n') {
                    let line = buffer[..newline_pos].trim().to_string();
                    buffer = buffer[newline_pos + 1..].to_string();

                    if line.is_empty() {
                        continue;
                    }

                    let chunk_resp: TtsStreamChunkResponse = match serde_json::from_str(&line) {
                        Ok(c) => c,
                        Err(_) => continue,
                    };

                    if chunk_resp.code != status_code::SUCCESS {
                        Err(Error::api_with_req_id(
                            chunk_resp.code,
                            chunk_resp.message,
                            chunk_resp.reqid,
                            200,
                        ))?;
                        return;
                    }

                    let is_last = chunk_resp.sequence < 0;

                    let audio = if !chunk_resp.data.is_empty() {
                        BASE64.decode(&chunk_resp.data)?
                    } else {
                        vec![]
                    };

                    let duration = chunk_resp
                        .addition
                        .duration
                        .parse::<i32>()
                        .unwrap_or_default();

                    yield TtsChunk {
                        audio,
                        sequence: chunk_resp.sequence,
                        is_last,
                        subtitle: None,
                        duration,
                    };

                    if is_last {
                        return;
                    }
                }
            }
        })
    }

    /// Performs streaming TTS synthesis over WebSocket using the binary protocol.
    ///
    /// Connects via WebSocket to the TTS binary endpoint and receives audio
    /// chunks as binary protocol messages. More efficient than HTTP streaming.
    ///
    /// Uses header-based authentication (X-Api-App-Id / X-Api-Access-Key).
    pub async fn synthesize_stream_ws(
        &self,
        req: &TtsRequest,
    ) -> Result<impl Stream<Item = Result<TtsChunk>>> {
        use tokio_tungstenite::tungstenite::client::IntoClientRequest;
        use tokio_tungstenite::tungstenite::Message as WsMessage;

        let tts_req = self.build_request(req);
        let auth = self.http.auth();
        let ws_url = format!("{}/api/v1/tts/ws_binary", self.http.ws_url());

        let mut request = ws_url.into_client_request()
            .map_err(|e| Error::Other(format!("build ws request: {}", e)))?;

        let headers = request.headers_mut();
        headers.insert("X-Api-App-Id", auth.app_id.parse().unwrap());
        if let Some(ref token) = auth.access_token {
            headers.insert("X-Api-Access-Key", token.parse().unwrap());
        } else if let Some(ref api_key) = auth.api_key {
            headers.insert("X-Api-Access-Key", api_key.parse().unwrap());
        }

        let (ws_stream, _) = tokio_tungstenite::connect_async(request)
            .await
            .map_err(Error::WebSocket)?;

        let (mut write, mut read) = futures::StreamExt::split(ws_stream);

        let req_json = serde_json::to_string(&tts_req)
            .map_err(|e| Error::Other(format!("serialize request: {}", e)))?;
        futures::SinkExt::send(&mut write, WsMessage::Text(req_json.into()))
            .await
            .map_err(Error::WebSocket)?;

        let proto = crate::protocol::BinaryProtocol::new();

        Ok(try_stream! {
            while let Some(msg) = futures::StreamExt::next(&mut read).await {
                let msg = msg.map_err(Error::WebSocket)?;
                match msg {
                    WsMessage::Binary(data) => {
                        let parsed = proto.unmarshal(&data)?;

                        if parsed.is_error() {
                            Err(Error::api(
                                parsed.error_code as i32,
                                String::from_utf8_lossy(&parsed.payload).to_string(),
                                200,
                            ))?;
                            return;
                        }

                        let is_last = parsed.sequence < 0;
                        yield TtsChunk {
                            audio: parsed.payload,
                            sequence: parsed.sequence,
                            is_last,
                            subtitle: None,
                            duration: 0,
                        };

                        if is_last {
                            return;
                        }
                    }
                    WsMessage::Close(_) => return,
                    _ => continue,
                }
            }
        })
    }

    /// Opens a duplex TTS WebSocket session for interactive streaming.
    ///
    /// In duplex mode, you can send text incrementally and receive audio chunks
    /// as they're generated, allowing for lower-latency streaming.
    pub async fn open_duplex_session(
        &self,
        config: &TtsDuplexConfig,
    ) -> Result<TtsDuplexSession> {
        use tokio_tungstenite::tungstenite::client::IntoClientRequest;

        let auth = self.http.auth();
        let ws_url = format!("{}/api/v1/tts/ws_binary", self.http.ws_url());

        let mut request = ws_url.into_client_request()
            .map_err(|e| Error::Other(format!("build ws request: {}", e)))?;

        let headers = request.headers_mut();
        headers.insert("X-Api-App-Id", auth.app_id.parse().unwrap());
        if let Some(ref token) = auth.access_token {
            headers.insert("X-Api-Access-Key", token.parse().unwrap());
        } else if let Some(ref api_key) = auth.api_key {
            headers.insert("X-Api-Access-Key", api_key.parse().unwrap());
        }

        let (ws_stream, _) = tokio_tungstenite::connect_async(request)
            .await
            .map_err(Error::WebSocket)?;

        let (write, read) = futures::StreamExt::split(ws_stream);

        let proto = crate::protocol::BinaryProtocol::new();

        Ok(TtsDuplexSession {
            write: Arc::new(tokio::sync::Mutex::new(write)),
            read: Arc::new(tokio::sync::Mutex::new(read)),
            proto,
            config: config.clone(),
            auth: auth.clone(),
            req_id: Uuid::new_v4().to_string(),
            started: std::sync::atomic::AtomicBool::new(false),
            closed: Arc::new(std::sync::atomic::AtomicBool::new(false)),
        })
    }

    /// Creates an async TTS task.
    ///
    /// The task runs on the server and can be polled with `get_async_task`.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let task = client.tts().create_async_task(&AsyncTtsRequest {
    ///     text: "你好，世界！".to_string(),
    ///     voice_type: "zh_female_cancan".to_string(),
    ///     ..Default::default()
    /// }).await?;
    /// println!("Task ID: {}", task.task_id);
    /// ```
    pub async fn create_async_task(&self, req: &AsyncTtsRequest) -> Result<TtsAsyncTaskResult> {
        let auth = self.http.auth();

        let mut submit_req = serde_json::json!({
            "appid": auth.app_id,
            "reqid": Uuid::new_v4().to_string(),
            "text": req.text,
            "voice_type": req.voice_type,
        });

        if let Some(encoding) = req.encoding {
            submit_req["format"] = serde_json::json!(encoding.as_str());
        }
        if let Some(sample_rate) = req.sample_rate {
            submit_req["sample_rate"] = serde_json::json!(sample_rate.as_i32());
        }
        if let Some(speed) = req.speed_ratio {
            submit_req["speed_ratio"] = serde_json::json!(speed);
        }
        if let Some(volume) = req.volume_ratio {
            submit_req["volume_ratio"] = serde_json::json!(volume);
        }
        if let Some(pitch) = req.pitch_ratio {
            submit_req["pitch_ratio"] = serde_json::json!(pitch);
        }
        if let Some(ref callback) = req.callback_url {
            submit_req["callback_url"] = serde_json::json!(callback);
        }

        #[derive(Deserialize)]
        struct Response {
            #[serde(default)]
            code: i32,
            #[serde(default)]
            message: String,
            #[serde(default)]
            task_id: String,
        }

        let resp: Response = self.http.request("POST", "/api/v1/tts_async/submit", Some(&submit_req)).await?;

        if resp.code != 0 {
            return Err(Error::api(resp.code, resp.message, 200));
        }

        Ok(TtsAsyncTaskResult {
            task_id: resp.task_id,
        })
    }

    /// Queries the status of an async TTS task.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let status = client.tts().get_async_task("task-id").await?;
    /// if status.status.is_success() {
    ///     println!("Audio URL: {}", status.audio_url);
    /// }
    /// ```
    pub async fn get_async_task(&self, task_id: &str) -> Result<TtsAsyncTaskStatus> {
        let auth = self.http.auth();

        let query_req = serde_json::json!({
            "appid": auth.app_id,
            "task_id": task_id,
        });

        #[derive(Deserialize)]
        struct Response {
            #[serde(default)]
            code: i32,
            #[serde(default)]
            message: String,
            #[serde(default)]
            task_id: String,
            #[serde(default)]
            status: String,
            #[serde(default)]
            progress: i32,
            #[serde(default)]
            audio_url: String,
            #[serde(default)]
            audio_duration: i32,
        }

        let resp: Response = self.http.request("POST", "/api/v1/tts_async/query", Some(&query_req)).await?;

        if resp.code != 0 {
            return Err(Error::api(resp.code, resp.message, 200));
        }

        let status = match resp.status.as_str() {
            "submitted" | "pending" => TaskStatus::Pending,
            "running" | "processing" => TaskStatus::Processing,
            "success" => TaskStatus::Success,
            "failed" => TaskStatus::Failed,
            "cancelled" => TaskStatus::Cancelled,
            _ => TaskStatus::Pending,
        };

        Ok(TtsAsyncTaskStatus {
            task_id: resp.task_id,
            status,
            progress: resp.progress,
            audio_url: resp.audio_url,
            audio_duration: resp.audio_duration,
        })
    }

    /// Builds the TTS request payload.
    fn build_request(&self, req: &TtsRequest) -> TtsRequestPayload {
        let auth = self.http.auth();
        let cluster = req
            .cluster
            .clone()
            .or_else(|| auth.cluster.clone())
            .unwrap_or_else(|| "volcano_tts".to_string());

        TtsRequestPayload {
            app: AppInfo {
                appid: auth.app_id.clone(),
                token: auth.access_token.clone().unwrap_or_default(),
                cluster,
            },
            user: UserInfo {
                uid: auth.user_id.clone(),
            },
            audio: TtsAudioParams {
                voice_type: req.voice_type.clone(),
                encoding: req.encoding.map(|e| e.as_str().to_string()),
                speed_ratio: req.speed_ratio,
                volume_ratio: req.volume_ratio,
                pitch_ratio: req.pitch_ratio,
                emotion: req.emotion.clone(),
                language: req.language.as_ref().map(|l| l.as_str().to_string()),
            },
            request: TtsRequestParams {
                reqid: Uuid::new_v4().to_string(),
                text: req.text.clone(),
                text_type: req.text_type.map(|t| t.as_str().to_string()),
                operation: Some("query".to_string()),
                silence_duration: req.silence_duration,
            },
        }
    }
}

// ================== Request Types ==================

/// Async TTS synthesis request.
#[derive(Debug, Clone, Default)]
pub struct AsyncTtsRequest {
    /// Text to synthesize.
    pub text: String,
    /// Voice type (e.g., "zh_female_cancan").
    pub voice_type: String,
    /// Audio encoding format.
    pub encoding: Option<AudioEncoding>,
    /// Sample rate.
    pub sample_rate: Option<SampleRate>,
    /// Speed ratio (0.5-2.0, default 1.0).
    pub speed_ratio: Option<f64>,
    /// Volume ratio (0.5-2.0, default 1.0).
    pub volume_ratio: Option<f64>,
    /// Pitch ratio (0.5-2.0, default 1.0).
    pub pitch_ratio: Option<f64>,
    /// Callback URL for task completion notification.
    pub callback_url: Option<String>,
}

/// Result of creating an async TTS task.
#[derive(Debug, Clone, Default)]
pub struct TtsAsyncTaskResult {
    /// Task ID for polling.
    pub task_id: String,
}

/// Status of an async TTS task.
#[derive(Debug, Clone, Default)]
pub struct TtsAsyncTaskStatus {
    /// Task ID.
    pub task_id: String,
    /// Current status.
    pub status: TaskStatus,
    /// Progress (0-100).
    pub progress: i32,
    /// Audio URL (when completed).
    pub audio_url: String,
    /// Audio duration in milliseconds.
    pub audio_duration: i32,
}

/// TTS synthesis request.
#[derive(Debug, Clone, Default)]
pub struct TtsRequest {
    /// Text to synthesize.
    pub text: String,
    /// Text type (plain or ssml).
    pub text_type: Option<TtsTextType>,
    /// Voice type (e.g., "zh_female_cancan").
    pub voice_type: String,
    /// Cluster name (e.g., "volcano_tts").
    pub cluster: Option<String>,
    /// Audio encoding format.
    pub encoding: Option<AudioEncoding>,
    /// Sample rate.
    pub sample_rate: Option<SampleRate>,
    /// Speed ratio (0.5-2.0, default 1.0).
    pub speed_ratio: Option<f64>,
    /// Volume ratio (0.5-2.0, default 1.0).
    pub volume_ratio: Option<f64>,
    /// Pitch ratio (0.5-2.0, default 1.0).
    pub pitch_ratio: Option<f64>,
    /// Emotion.
    pub emotion: Option<String>,
    /// Language.
    pub language: Option<Language>,
    /// Whether to enable subtitle generation.
    pub enable_subtitle: bool,
    /// Silence duration at the end (in milliseconds).
    pub silence_duration: Option<i32>,
}

/// TTS synthesis response.
#[derive(Debug, Clone)]
pub struct TtsResponse {
    /// Audio data (binary).
    pub audio: Vec<u8>,
    /// Duration in milliseconds.
    pub duration: i32,
    /// Subtitles (if enabled).
    pub subtitles: Vec<SubtitleSegment>,
    /// Request ID.
    pub req_id: String,
}

/// Streaming TTS chunk.
#[derive(Debug, Clone)]
pub struct TtsChunk {
    /// Audio data (binary).
    pub audio: Vec<u8>,
    /// Sequence number (negative means last chunk).
    pub sequence: i32,
    /// Whether this is the last chunk.
    pub is_last: bool,
    /// Subtitle segment (if enabled).
    pub subtitle: Option<SubtitleSegment>,
    /// Duration in milliseconds.
    pub duration: i32,
}

// ================== Internal Request/Response Types ==================

/// Application information.
#[derive(Debug, Serialize)]
struct AppInfo {
    appid: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    token: String,
    cluster: String,
}

/// User information.
#[derive(Debug, Serialize)]
struct UserInfo {
    uid: String,
}

/// TTS audio parameters.
#[derive(Debug, Serialize)]
struct TtsAudioParams {
    voice_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    encoding: Option<String>,
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

/// TTS request parameters.
#[derive(Debug, Serialize)]
struct TtsRequestParams {
    reqid: String,
    text: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    text_type: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    operation: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    silence_duration: Option<i32>,
}

/// TTS request payload.
#[derive(Debug, Serialize)]
struct TtsRequestPayload {
    app: AppInfo,
    user: UserInfo,
    audio: TtsAudioParams,
    request: TtsRequestParams,
}

/// TTS API response.
#[derive(Debug, Deserialize)]
struct TtsApiResponse {
    #[serde(default)]
    reqid: String,
    #[serde(default)]
    code: i32,
    #[serde(default)]
    message: String,
    #[serde(default)]
    data: String,
    #[serde(default)]
    addition: TtsAddition,
}

/// TTS response addition.
#[derive(Debug, Deserialize, Default)]
struct TtsAddition {
    #[serde(default)]
    duration: String,
}

/// TTS streaming chunk response.
#[derive(Debug, Deserialize)]
struct TtsStreamChunkResponse {
    #[serde(default)]
    reqid: String,
    #[serde(default)]
    code: i32,
    #[serde(default)]
    message: String,
    #[serde(default)]
    sequence: i32,
    #[serde(default)]
    data: String,
    #[serde(default)]
    addition: TtsAddition,
}

// ================== Duplex Session ==================

/// Configuration for a duplex TTS session.
#[derive(Debug, Clone, Default)]
pub struct TtsDuplexConfig {
    /// Voice type.
    pub voice_type: String,
    /// Audio encoding format.
    pub encoding: Option<AudioEncoding>,
    /// Speed ratio.
    pub speed_ratio: Option<f64>,
    /// Volume ratio.
    pub volume_ratio: Option<f64>,
    /// Pitch ratio.
    pub pitch_ratio: Option<f64>,
}

type WsStream = tokio_tungstenite::WebSocketStream<
    tokio_tungstenite::MaybeTlsStream<tokio::net::TcpStream>,
>;

/// A duplex TTS WebSocket session for interactive text-to-speech.
pub struct TtsDuplexSession {
    write: Arc<tokio::sync::Mutex<futures::stream::SplitSink<WsStream, tokio_tungstenite::tungstenite::Message>>>,
    read: Arc<tokio::sync::Mutex<futures::stream::SplitStream<WsStream>>>,
    proto: crate::protocol::BinaryProtocol,
    config: TtsDuplexConfig,
    auth: crate::http::AuthConfig,
    req_id: String,
    started: std::sync::atomic::AtomicBool,
    closed: Arc<std::sync::atomic::AtomicBool>,
}

impl TtsDuplexSession {
    /// Sends text to the session for synthesis.
    ///
    /// The first call sends a full request with app/user/audio config and
    /// `operation: "submit"`. Subsequent calls send only the text with
    /// `operation: "append"`. Set `is_last` to true for the final segment,
    /// which sends `operation: "finish"`.
    pub async fn send_text(&self, text: &str, is_last: bool) -> Result<()> {
        use tokio_tungstenite::tungstenite::Message as WsMessage;

        if self.closed.load(std::sync::atomic::Ordering::Relaxed) {
            return Err(Error::Other("session closed".to_string()));
        }

        let req = if !self.started.swap(true, std::sync::atomic::Ordering::Relaxed) {
            // First send: full request with config
            let mut audio = serde_json::json!({
                "voice_type": self.config.voice_type,
            });
            if let Some(encoding) = self.config.encoding {
                audio["encoding"] = serde_json::json!(encoding.as_str());
            }
            if let Some(speed) = self.config.speed_ratio {
                audio["speed_ratio"] = serde_json::json!(speed);
            }
            if let Some(volume) = self.config.volume_ratio {
                audio["volume_ratio"] = serde_json::json!(volume);
            }
            if let Some(pitch) = self.config.pitch_ratio {
                audio["pitch_ratio"] = serde_json::json!(pitch);
            }

            serde_json::json!({
                "app": {
                    "appid": self.auth.app_id,
                    "cluster": self.auth.cluster.as_deref().unwrap_or("volcano_tts"),
                },
                "user": {
                    "uid": self.auth.user_id,
                },
                "audio": audio,
                "request": {
                    "reqid": self.req_id,
                    "text": text,
                    "text_type": "plain",
                    "operation": "submit",
                },
            })
        } else if is_last {
            // Final send: finish operation
            serde_json::json!({
                "request": {
                    "reqid": self.req_id,
                    "operation": "finish",
                },
            })
        } else {
            // Append text
            serde_json::json!({
                "request": {
                    "reqid": self.req_id,
                    "text": text,
                    "operation": "append",
                },
            })
        };

        let json = serde_json::to_string(&req)
            .map_err(|e| Error::Other(format!("serialize request: {}", e)))?;
        futures::SinkExt::send(
            &mut *self.write.lock().await,
            WsMessage::Text(json.into()),
        )
        .await
        .map_err(Error::WebSocket)?;

        Ok(())
    }

    /// Receives the next audio chunk from the session.
    ///
    /// Returns `None` when the session is closed.
    pub async fn recv(&self) -> Option<Result<TtsChunk>> {
        use tokio_tungstenite::tungstenite::Message as WsMessage;

        if self.closed.load(std::sync::atomic::Ordering::Relaxed) {
            return None;
        }

        loop {
            let msg = futures::StreamExt::next(&mut *self.read.lock().await).await?;

            match msg {
                Ok(WsMessage::Binary(data)) => {
                    match self.proto.unmarshal(&data) {
                        Ok(parsed) => {
                            if parsed.is_error() {
                                return Some(Err(Error::api(
                                    parsed.error_code as i32,
                                    String::from_utf8_lossy(&parsed.payload).to_string(),
                                    200,
                                )));
                            }

                            let is_last = parsed.sequence < 0;
                            if is_last {
                                self.closed.store(true, std::sync::atomic::Ordering::Relaxed);
                            }

                            return Some(Ok(TtsChunk {
                                audio: parsed.payload,
                                sequence: parsed.sequence,
                                is_last,
                                subtitle: None,
                                duration: 0,
                            }));
                        }
                        Err(e) => return Some(Err(e)),
                    }
                }
                Ok(WsMessage::Close(_)) => {
                    self.closed.store(true, std::sync::atomic::Ordering::Relaxed);
                    return None;
                }
                Ok(_) => continue,
                Err(e) => return Some(Err(Error::WebSocket(e))),
            }
        }
    }

    /// Closes the session.
    pub async fn close(&self) -> Result<()> {
        if self.closed.swap(true, std::sync::atomic::Ordering::Relaxed) {
            return Ok(());
        }
        futures::SinkExt::close(&mut *self.write.lock().await)
            .await
            .map_err(Error::WebSocket)?;
        Ok(())
    }
}
