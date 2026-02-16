//! Podcast synthesis service for Doubao Speech API.
//!
//! Supports two API versions:
//!
//! # V1 HTTP API (Async)
//!
//! - POST `/api/v1/podcast/submit` - Submit async task
//! - POST `/api/v1/podcast/query` - Query task status
//!
//! # V3 SAMI Podcast API (WebSocket) - Recommended
//!
//! - WSS `/api/v3/sami/podcasttts` - Real-time streaming
//! - Resource ID: `volc.service_type.10050`
//! - Fixed App Key: `aGjiRDfUWi`
//!
//! ## SAMI Podcast Required Speakers
//!
//! SAMI Podcast requires speakers with `_v2_saturn_bigtts` suffix:
//!
//! | Speaker | Description |
//! |---------|-------------|
//! | `zh_male_dayixiansheng_v2_saturn_bigtts` | 大一先生 |
//! | `zh_female_mizaitongxue_v2_saturn_bigtts` | 米仔同学 |
//! | `zh_male_liufei_v2_saturn_bigtts` | 刘飞 |
//! | `zh_male_xiaolei_v2_saturn_bigtts` | 小雷 |

use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};

use futures::stream::{SplitSink, SplitStream};
use futures::{SinkExt, StreamExt};
use serde::{Deserialize, Serialize};
use tokio::sync::Mutex;
use tokio_tungstenite::tungstenite::Message as WsMessage;
use tokio_tungstenite::{connect_async, MaybeTlsStream, WebSocketStream};

use crate::{
    error::{Error, Result},
    http::HttpClient,
    protocol::{BinaryProtocol, Message, MessageFlags, MessageType, SerializationType},
    types::{AudioEncoding, SampleRate, TaskStatus},
};

/// Podcast synthesis service.
///
/// API Documentation: https://www.volcengine.com/docs/6561/1668014
pub struct PodcastService {
    http: Arc<HttpClient>,
}

impl PodcastService {
    /// Creates a new Podcast service.
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Creates a podcast synthesis task.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_doubaospeech::{Client, PodcastTaskRequest, PodcastLine};
    ///
    /// let client = Client::builder("app-id").api_key("api-key").build()?;
    /// let result = client.podcast().create_task(&PodcastTaskRequest {
    ///     script: vec![
    ///         PodcastLine {
    ///             speaker_id: "speaker_1".to_string(),
    ///             text: "Hello!".to_string(),
    ///             ..Default::default()
    ///         },
    ///         PodcastLine {
    ///             speaker_id: "speaker_2".to_string(),
    ///             text: "Hi there!".to_string(),
    ///             ..Default::default()
    ///         },
    ///     ],
    ///     ..Default::default()
    /// }).await?;
    /// ```
    pub async fn create_task(&self, req: &PodcastTaskRequest) -> Result<PodcastTaskResult> {
        let auth = self.http.auth();
        let req_id = crate::client::generate_req_id();

        // Build dialogue list
        let dialogues: Vec<serde_json::Value> = req
            .script
            .iter()
            .map(|line| {
                let mut d = serde_json::json!({
                    "speaker": line.speaker_id,
                    "text": line.text,
                });
                if let Some(ref emotion) = line.emotion {
                    d["emotion"] = serde_json::json!(emotion);
                }
                if let Some(speed) = line.speed_ratio {
                    d["speed_ratio"] = serde_json::json!(speed);
                }
                d
            })
            .collect();

        let mut submit_req = serde_json::json!({
            "app": {
                "appid": auth.app_id,
                "cluster": auth.cluster.as_deref().unwrap_or("volcano_mega"),
            },
            "user": {
                "uid": auth.user_id,
            },
            "request": {
                "reqid": req_id,
                "dialogues": dialogues,
            },
        });

        if let Some(ref encoding) = req.encoding {
            let mut audio = serde_json::json!({
                "encoding": encoding.as_str(),
            });
            if let Some(sample_rate) = req.sample_rate {
                audio["sample_rate"] = serde_json::json!(sample_rate.as_i32());
            }
            submit_req["audio"] = audio;
        }

        if let Some(ref callback) = req.callback_url {
            submit_req["request"]["callback_url"] = serde_json::json!(callback);
        }

        let response: AsyncTaskResponse = self
            .http
            .request("POST", "/api/v1/podcast/submit", Some(&submit_req))
            .await?;

        if response.code != 0 {
            return Err(Error::api(response.code, response.message, 200));
        }

        Ok(PodcastTaskResult {
            task_id: response.task_id,
            req_id,
        })
    }

    /// Opens a SAMI Podcast WebSocket session for real-time podcast synthesis.
    ///
    /// This uses the V3 SAMI Podcast endpoint: WSS `/api/v3/sami/podcasttts`
    ///
    /// # Required Speakers
    ///
    /// SAMI Podcast requires speakers with `_v2_saturn_bigtts` suffix:
    /// - `zh_male_dayixiansheng_v2_saturn_bigtts`
    /// - `zh_female_mizaitongxue_v2_saturn_bigtts`
    /// - `zh_male_liufei_v2_saturn_bigtts`
    /// - `zh_male_xiaolei_v2_saturn_bigtts`
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_doubaospeech::{Client, PodcastSAMIRequest, PodcastSpeakerInfo};
    ///
    /// # async fn example() -> anyhow::Result<()> {
    /// let client = Client::builder("app-id").api_key("api-key").build()?;
    /// let session = client.podcast().stream_sami(&PodcastSAMIRequest {
    ///     action: 0,
    ///     input_text: Some("AI is changing...".to_string()),
    ///     speaker_info: Some(PodcastSpeakerInfo {
    ///         speakers: vec![
    ///             "zh_male_dayixiansheng_v2_saturn_bigtts".to_string(),
    ///             "zh_female_mizaitongxue_v2_saturn_bigtts".to_string(),
    ///         ],
    ///         random_order: Some(true),
    ///     }),
    ///     ..Default::default()
    /// }).await?;
    ///
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
    pub async fn stream_sami(&self, req: &PodcastSAMIRequest) -> Result<PodcastSAMISession> {
        use tokio_tungstenite::tungstenite::client::IntoClientRequest;

        let auth = self.http.auth();
        let session_id = format!("podcast-{}", std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_nanos());

        // Build WebSocket URL
        let ws_url = format!("{}/api/v3/sami/podcasttts", self.http.ws_url());

        // Build WebSocket request with custom headers
        let mut request = ws_url.into_client_request()
            .map_err(|e| Error::Other(format!("build ws request: {}", e)))?;

        // Add SAMI Podcast headers
        // - X-Api-App-Id: APP ID
        // - X-Api-Access-Key: Access Token
        // - X-Api-Resource-Id: volc.service_type.10050 (fixed)
        // - X-Api-App-Key: aGjiRDfUWi (fixed)
        let headers = request.headers_mut();
        headers.insert("X-Api-App-Id", auth.app_id.parse().unwrap());
        headers.insert("X-Api-Resource-Id", "volc.service_type.10050".parse().unwrap());
        headers.insert("X-Api-App-Key", "aGjiRDfUWi".parse().unwrap());
        headers.insert("X-Api-Request-Id", session_id.parse().unwrap());

        // Add access key
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

        // Create protocol handler with JSON serialization
        let mut proto = BinaryProtocol::new();
        proto.set_serialization(SerializationType::Json);

        let session = PodcastSAMISession {
            write: Arc::new(Mutex::new(write)),
            read: Arc::new(Mutex::new(read)),
            proto,
            session_id: session_id.clone(),
            closed: Arc::new(AtomicBool::new(false)),
        };

        // Send the request using shared binary protocol
        session.send_request(req).await?;

        Ok(session)
    }

    /// Opens a non-SAMI podcast WebSocket stream session.
    ///
    /// This uses the V3 podcast endpoint (`/api/v3/tts/podcast`) with
    /// query-param authentication, which is the older streaming method
    /// before SAMI was introduced.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let session = client.podcast().stream(&PodcastStreamRequest {
    ///     text: "AI is changing our lives...".to_string(),
    ///     speakers: vec![
    ///         PodcastSpeaker { name: "host".to_string(), voice_type: "zh_male_dayixiansheng".to_string(), ..Default::default() },
    ///         PodcastSpeaker { name: "guest".to_string(), voice_type: "zh_female_mizaitongxue".to_string(), ..Default::default() },
    ///     ],
    ///     ..Default::default()
    /// }).await?;
    ///
    /// while let Some(result) = session.recv().await {
    ///     let chunk = result?;
    ///     // Process chunk.audio
    ///     if chunk.is_last { break; }
    /// }
    /// ```
    pub async fn stream(&self, req: &PodcastStreamRequest) -> Result<PodcastStreamSession> {
        use tokio_tungstenite::tungstenite::client::IntoClientRequest;

        let auth = self.http.auth();

        let mut endpoint = format!("{}/api/v3/tts/podcast?appid={}", self.http.ws_url(), auth.app_id);
        if let Some(ref token) = auth.access_token {
            endpoint.push_str(&format!("&token={}", token));
        }
        if let Some(ref cluster) = auth.cluster {
            endpoint.push_str(&format!("&cluster={}", cluster));
        }

        let request = endpoint.into_client_request()
            .map_err(|e| Error::Other(format!("build ws request: {}", e)))?;

        let (ws_stream, _) = connect_async(request)
            .await
            .map_err(Error::WebSocket)?;

        let (write, read) = ws_stream.split();

        let session = PodcastStreamSession {
            write: Arc::new(Mutex::new(write)),
            read: Arc::new(Mutex::new(read)),
            closed: Arc::new(AtomicBool::new(false)),
        };

        // Send the podcast request
        let req_json = serde_json::to_vec(req)
            .map_err(|e| Error::Other(format!("serialize request: {}", e)))?;
        session.write.lock().await
            .send(WsMessage::Text(String::from_utf8_lossy(&req_json).into_owned().into()))
            .await
            .map_err(Error::WebSocket)?;

        Ok(session)
    }

    /// Queries podcast task status.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let status = client.podcast().get_task("task-id").await?;
    /// println!("Status: {:?}", status.status);
    /// ```
    pub async fn get_task(&self, task_id: &str) -> Result<PodcastTaskStatus> {
        let auth = self.http.auth();

        let query_req = serde_json::json!({
            "appid": auth.app_id,
            "task_id": task_id,
        });

        let response: PodcastQueryResponse = self
            .http
            .request("POST", "/api/v1/podcast/query", Some(&query_req))
            .await?;

        if response.code != 0 {
            return Err(Error::api(response.code, response.message, 200));
        }

        // Convert status
        let status = match response.data.status.as_str() {
            "submitted" | "pending" => TaskStatus::Pending,
            "running" | "processing" => TaskStatus::Processing,
            "success" => TaskStatus::Success,
            "failed" => TaskStatus::Failed,
            _ => TaskStatus::Pending,
        };

        // Convert result
        let result = if status == TaskStatus::Success {
            Some(PodcastResult {
                audio_url: response.data.audio_url,
                duration: response.data.duration,
            })
        } else {
            None
        };

        Ok(PodcastTaskStatus {
            task_id: response.data.task_id,
            status,
            progress: response.data.progress,
            result,
        })
    }
}

// ================== Request Types ==================

/// Podcast synthesis task request.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PodcastTaskRequest {
    /// Script lines.
    pub script: Vec<PodcastLine>,
    /// Audio encoding.
    #[serde(default)]
    pub encoding: Option<AudioEncoding>,
    /// Sample rate.
    #[serde(default)]
    pub sample_rate: Option<SampleRate>,
    /// Callback URL.
    #[serde(default)]
    pub callback_url: Option<String>,
}

/// Podcast script line.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PodcastLine {
    /// Speaker ID.
    pub speaker_id: String,
    /// Text content.
    pub text: String,
    /// Emotion.
    #[serde(default)]
    pub emotion: Option<String>,
    /// Speed ratio.
    #[serde(default)]
    pub speed_ratio: Option<f32>,
}

// ================== Response Types ==================

/// Podcast task creation result.
#[derive(Debug, Clone, Default, Serialize)]
pub struct PodcastTaskResult {
    /// Task ID.
    pub task_id: String,
    /// Request ID.
    pub req_id: String,
}

/// Podcast task status.
#[derive(Debug, Clone, Default, Serialize)]
pub struct PodcastTaskStatus {
    /// Task ID.
    pub task_id: String,
    /// Status.
    pub status: TaskStatus,
    /// Progress (0-100).
    pub progress: i32,
    /// Result (when completed).
    pub result: Option<PodcastResult>,
}

/// Podcast synthesis result.
#[derive(Debug, Clone, Default, Serialize)]
pub struct PodcastResult {
    /// Audio URL.
    pub audio_url: String,
    /// Audio duration in milliseconds.
    pub duration: i32,
}

// ================== Internal Types ==================

#[derive(Debug, Deserialize)]
struct AsyncTaskResponse {
    #[serde(default)]
    code: i32,
    #[serde(default)]
    message: String,
    #[serde(default)]
    task_id: String,
}

#[derive(Debug, Deserialize)]
struct PodcastQueryResponse {
    #[serde(default)]
    code: i32,
    #[serde(default)]
    message: String,
    #[serde(default)]
    data: PodcastQueryData,
}

#[derive(Debug, Deserialize, Default)]
struct PodcastQueryData {
    #[serde(default)]
    task_id: String,
    #[serde(default)]
    status: String,
    #[serde(default)]
    progress: i32,
    #[serde(default)]
    audio_url: String,
    #[serde(default)]
    duration: i32,
}

// ================== SAMI Podcast Types ==================

/// SAMI Podcast streaming request.
///
/// # Example
///
/// ```rust,no_run
/// use giztoy_doubaospeech::{Client, PodcastSAMIRequest, PodcastSpeakerInfo};
///
/// let client = Client::builder("app-id").api_key("api-key").build()?;
/// let mut session = client.podcast().stream_sami(&PodcastSAMIRequest {
///     action: 0, // Summary generation
///     input_text: Some("AI is changing our lives...".to_string()),
///     speaker_info: Some(PodcastSpeakerInfo {
///         speakers: vec![
///             "zh_male_dayixiansheng_v2_saturn_bigtts".to_string(),
///             "zh_female_mizaitongxue_v2_saturn_bigtts".to_string(),
///         ],
///         random_order: Some(true),
///     }),
///     ..Default::default()
/// }).await?;
/// ```
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PodcastSAMIRequest {
    /// Action type:
    /// - 0: Summary generation (from input_text)
    /// - 3: Direct dialogue generation (from nlp_texts)
    /// - 4: Extended generation (from prompt_text)
    #[serde(default)]
    pub action: i32,

    /// Input ID for tracking
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_id: Option<String>,

    /// Input text for action=0 (summary generation)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_text: Option<String>,

    /// Dialogue texts for action=3 (direct generation)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub nlp_texts: Option<Vec<PodcastDialogue>>,

    /// Prompt text for action=4 (extended generation)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub prompt_text: Option<String>,

    /// Use head music
    #[serde(skip_serializing_if = "Option::is_none")]
    pub use_head_music: Option<bool>,

    /// Use tail music
    #[serde(skip_serializing_if = "Option::is_none")]
    pub use_tail_music: Option<bool>,

    /// Audio configuration
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio_config: Option<PodcastAudioConfig>,

    /// Speaker configuration (exactly 2 speakers required)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub speaker_info: Option<PodcastSpeakerInfo>,
}

/// Podcast dialogue line for direct generation (action=3).
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PodcastDialogue {
    /// Speaker identifier: "speaker_1" or "speaker_2"
    pub speaker: String,
    /// Dialogue text
    pub text: String,
}

/// Podcast audio output configuration.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PodcastAudioConfig {
    /// Audio format: pcm, mp3, ogg_opus, aac
    #[serde(skip_serializing_if = "Option::is_none")]
    pub format: Option<String>,
    /// Sample rate: 16000, 24000, 48000
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sample_rate: Option<i32>,
    /// Speech rate: -50 ~ 100
    #[serde(skip_serializing_if = "Option::is_none")]
    pub speech_rate: Option<i32>,
}

/// Podcast speaker configuration.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PodcastSpeakerInfo {
    /// Randomize speaker order
    #[serde(skip_serializing_if = "Option::is_none")]
    pub random_order: Option<bool>,
    /// Speaker voices (exactly 2 required, must have _v2_saturn_bigtts suffix)
    pub speakers: Vec<String>,
}

/// SAMI Podcast streaming chunk.
#[derive(Debug, Clone, Default)]
pub struct PodcastSAMIChunk {
    /// Event type name
    pub event: String,
    /// Event code
    pub event_code: i32,
    /// Task ID
    pub task_id: String,
    /// Sequence number
    pub sequence: i32,
    /// Audio data (binary)
    pub audio: Vec<u8>,
    /// Generated text (for summary)
    pub text: String,
    /// Status message
    pub message: String,
    /// Whether this is the last chunk
    pub is_last: bool,
}

// ================== SAMI Podcast Session ==================

type WsStream = WebSocketStream<MaybeTlsStream<tokio::net::TcpStream>>;

/// SAMI Podcast WebSocket streaming session.
pub struct PodcastSAMISession {
    write: Arc<Mutex<SplitSink<WsStream, WsMessage>>>,
    read: Arc<Mutex<SplitStream<WsStream>>>,
    proto: BinaryProtocol,
    session_id: String,
    closed: Arc<AtomicBool>,
}

/// SAMI Podcast event codes.
pub mod sami_events {
    /// Start session request (client -> server).
    pub const START_SESSION: i32 = 100;
    /// Session started (server -> client).
    pub const SESSION_STARTED: i32 = 150;
    /// Session finished (server -> client).
    pub const SESSION_FINISHED: i32 = 152;
    /// Usage response (server -> client).
    pub const USAGE_RESPONSE: i32 = 154;
    /// Podcast round start (server -> client).
    pub const PODCAST_ROUND_START: i32 = 360;
    /// Podcast round response with audio (server -> client).
    pub const PODCAST_ROUND_RESPONSE: i32 = 361;
    /// Podcast round end (server -> client).
    pub const PODCAST_ROUND_END: i32 = 362;
    /// Podcast generation end (server -> client).
    pub const PODCAST_END: i32 = 363;
}

impl PodcastSAMISession {
    /// Sends the initial SAMI request using shared binary protocol.
    async fn send_request(&self, req: &PodcastSAMIRequest) -> Result<()> {
        let json_data = serde_json::to_vec(req)
            .map_err(|e| Error::Other(format!("serialize request: {}", e)))?;

        // Build message using shared protocol
        let msg = Message {
            msg_type: MessageType::FullClient,
            flags: MessageFlags::WithEvent,
            event: sami_events::START_SESSION,
            session_id: self.session_id.clone(),
            payload: json_data,
            ..Default::default()
        };

        let data = self.proto.marshal(&msg)?;

        self.write.lock().await
            .send(WsMessage::Binary(data.into()))
            .await
            .map_err(Error::WebSocket)?;

        Ok(())
    }

    /// Receives the next audio chunk from the session.
    ///
    /// Returns `None` when the session is closed or the final chunk is received.
    pub async fn recv(&self) -> Option<Result<PodcastSAMIChunk>> {
        loop {
            if self.closed.load(Ordering::Relaxed) {
                return None;
            }

            let msg = self.read.lock().await.next().await?;

            match msg {
                Ok(WsMessage::Binary(data)) => {
                    match self.parse_message(&data) {
                        Ok(Some(chunk)) => {
                            let is_last = chunk.is_last;
                            if is_last {
                                self.closed.store(true, Ordering::Relaxed);
                            }
                            return Some(Ok(chunk));
                        }
                        Ok(None) => {
                            // Skip this message, continue receiving
                            continue;
                        }
                        Err(e) => return Some(Err(e)),
                    }
                }
                Ok(WsMessage::Close(_)) => {
                    self.closed.store(true, Ordering::Relaxed);
                    return None;
                }
                Ok(_) => continue,
                Err(e) => return Some(Err(Error::WebSocket(e))),
            }
        }
    }

    /// Closes the session.
    pub async fn close(&self) -> Result<()> {
        if self.closed.swap(true, Ordering::Relaxed) {
            return Ok(());
        }
        self.write.lock().await.close().await.map_err(Error::WebSocket)?;
        Ok(())
    }

    /// Parses a binary protocol message using shared protocol.
    fn parse_message(&self, data: &[u8]) -> Result<Option<PodcastSAMIChunk>> {
        // Use shared protocol to unmarshal
        let msg = self.proto.unmarshal(data)?;

        // Check for error message
        if msg.is_error() {
            if let Ok(err_resp) = serde_json::from_slice::<serde_json::Value>(&msg.payload) {
                let code = err_resp.get("code").and_then(|v| v.as_i64()).unwrap_or(-1) as i32;
                let message = err_resp.get("message").and_then(|v| v.as_str()).unwrap_or("unknown error").to_string();
                return Err(Error::api(code, message, 200));
            }
            return Ok(None);
        }

        let event_code = msg.event;

        // Determine if last: 152 = SessionFinished, 363 = PodcastEnd
        let is_last = event_code == sami_events::SESSION_FINISHED || event_code == sami_events::PODCAST_END;

        let mut chunk = PodcastSAMIChunk {
            event_code,
            is_last,
            ..Default::default()
        };

        // Map event codes to names
        chunk.event = match event_code {
            sami_events::SESSION_STARTED => "SessionStarted".to_string(),
            sami_events::PODCAST_ROUND_START => "PodcastRoundStart".to_string(),
            sami_events::PODCAST_ROUND_RESPONSE => "PodcastRoundResponse".to_string(),
            sami_events::PODCAST_ROUND_END => "PodcastRoundEnd".to_string(),
            sami_events::PODCAST_END => "PodcastEnd".to_string(),
            sami_events::SESSION_FINISHED => "SessionFinished".to_string(),
            sami_events::USAGE_RESPONSE => "UsageResponse".to_string(),
            _ => format!("Event_{}", event_code),
        };

        // Parse payload based on message type
        if msg.is_audio_only() {
            // Raw audio data
            chunk.audio = msg.payload;
        } else if !msg.payload.is_empty() {
            // JSON response
            if let Ok(resp) = serde_json::from_slice::<serde_json::Value>(&msg.payload) {
                chunk.task_id = resp.get("task_id").and_then(|v| v.as_str()).unwrap_or("").to_string();
                chunk.sequence = resp.get("sequence").and_then(|v| v.as_i64()).unwrap_or(0) as i32;
                chunk.text = resp.get("data").and_then(|v| v.as_str()).unwrap_or("").to_string();
                chunk.message = resp.get("message").and_then(|v| v.as_str()).unwrap_or("").to_string();
            }
        }

        Ok(Some(chunk))
    }
}

// ================== Non-SAMI Podcast Stream Types ==================

/// Podcast stream request (non-SAMI, V3 endpoint).
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PodcastStreamRequest {
    /// Input text content.
    pub text: String,
    /// Speaker configurations.
    pub speakers: Vec<PodcastSpeaker>,
    /// Audio encoding format.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub encoding: Option<AudioEncoding>,
    /// Sample rate.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sample_rate: Option<SampleRate>,
}

/// Speaker configuration for podcast.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PodcastSpeaker {
    /// Speaker name/role.
    pub name: String,
    /// Voice type.
    pub voice_type: String,
    /// Speed ratio.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub speed_ratio: Option<f32>,
    /// Volume ratio.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub volume_ratio: Option<f32>,
}

/// Non-SAMI podcast streaming chunk.
#[derive(Debug, Clone, Default)]
pub struct PodcastStreamChunk {
    /// Audio data (binary).
    pub audio: Vec<u8>,
    /// Sequence number.
    pub sequence: i32,
    /// Whether this is the last chunk.
    pub is_last: bool,
}

/// Non-SAMI podcast WebSocket streaming session.
pub struct PodcastStreamSession {
    write: Arc<Mutex<futures::stream::SplitSink<WsStream, WsMessage>>>,
    read: Arc<Mutex<futures::stream::SplitStream<WsStream>>>,
    closed: Arc<AtomicBool>,
}

impl PodcastStreamSession {
    /// Receives the next audio chunk from the session.
    ///
    /// Returns `None` when the session is closed or the final chunk is received.
    pub async fn recv(&self) -> Option<Result<PodcastStreamChunk>> {
        loop {
            if self.closed.load(Ordering::Relaxed) {
                return None;
            }

            let msg = self.read.lock().await.next().await?;

            match msg {
                Ok(WsMessage::Binary(data)) => {
                    // Binary audio data
                    return Some(Ok(PodcastStreamChunk {
                        audio: data.to_vec(),
                        sequence: 0,
                        is_last: false,
                    }));
                }
                Ok(WsMessage::Text(text)) => {
                    // JSON control message
                    if let Ok(resp) = serde_json::from_str::<serde_json::Value>(&text) {
                        let code = resp.get("code").and_then(|v| v.as_i64()).unwrap_or(0) as i32;
                        if code != 0 {
                            let message = resp.get("message").and_then(|v| v.as_str()).unwrap_or("").to_string();
                            return Some(Err(Error::api(code, message, 200)));
                        }

                        let is_last = resp.get("is_last").and_then(|v| v.as_bool()).unwrap_or(false);
                        if is_last {
                            self.closed.store(true, Ordering::Relaxed);
                            return Some(Ok(PodcastStreamChunk {
                                audio: Vec::new(),
                                sequence: -1,
                                is_last: true,
                            }));
                        }
                    }
                    continue;
                }
                Ok(WsMessage::Close(_)) => {
                    self.closed.store(true, Ordering::Relaxed);
                    return None;
                }
                Ok(_) => continue,
                Err(e) => return Some(Err(Error::WebSocket(e))),
            }
        }
    }

    /// Closes the session.
    pub async fn close(&self) -> Result<()> {
        if self.closed.swap(true, Ordering::Relaxed) {
            return Ok(());
        }
        self.write.lock().await.close().await.map_err(Error::WebSocket)?;
        Ok(())
    }
}
