//! Realtime speech-to-speech dialogue service.
//!
//! This module provides real-time voice conversation capabilities using WebSocket.
//!
//! # Example
//!
//! ```ignore
//! use giztoy_doubaospeech::{Client, RealtimeConfig, RealtimeTTSConfig, RealtimeAudioConfig};
//!
//! let client = Client::builder("app-id").bearer_token("token").build()?;
//!
//! let config = RealtimeConfig {
//!     tts: RealtimeTTSConfig {
//!         speaker: "zh_female_cancan".to_string(),
//!         audio_config: RealtimeAudioConfig {
//!             channel: 1,
//!             format: "mp3".to_string(),
//!             sample_rate: 24000,
//!         },
//!         ..Default::default()
//!     },
//!     ..Default::default()
//! };
//!
//! let mut session = client.realtime().connect(&config).await?;
//!
//! // Send audio
//! session.send_audio(&audio_data).await?;
//!
//! // Receive events
//! while let Some(event) = session.recv().await {
//!     match event {
//!         Ok(evt) => {
//!             if let Some(audio) = evt.audio {
//!                 // Play audio
//!             }
//!         }
//!         Err(e) => eprintln!("Error: {}", e),
//!     }
//! }
//! ```

use std::collections::HashMap;
use std::sync::Arc;

use base64::Engine;
use futures::stream::{SplitSink, SplitStream};
use futures::{SinkExt, StreamExt};
use serde::{Deserialize, Serialize};
use tokio::sync::{mpsc, Mutex, RwLock};
use tokio_tungstenite::tungstenite::Message as WsMessage;
use tokio_tungstenite::{connect_async, MaybeTlsStream, WebSocketStream};

use crate::client::{generate_req_id, ClientConfig};
use crate::error::{Error, Result};
use crate::protocol::{events, BinaryProtocol, Message};

// ================== Types ==================

/// Realtime event type.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(i32)]
pub enum RealtimeEventType {
    SessionStarted = 50,
    SessionFailed = 51,
    SessionEnded = 52,
    ASRStarted = 100,
    ASRFinished = 101,
    TTSStarted = 102,
    TTSFinished = 103,
    AudioReceived = 104,
}

impl From<i32> for RealtimeEventType {
    fn from(v: i32) -> Self {
        match v {
            50 => RealtimeEventType::SessionStarted,
            51 => RealtimeEventType::SessionFailed,
            52 => RealtimeEventType::SessionEnded,
            100 => RealtimeEventType::ASRStarted,
            101 => RealtimeEventType::ASRFinished,
            102 => RealtimeEventType::TTSStarted,
            103 => RealtimeEventType::TTSFinished,
            104 => RealtimeEventType::AudioReceived,
            _ => RealtimeEventType::SessionStarted,
        }
    }
}

/// Realtime session configuration.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct RealtimeConfig {
    /// ASR configuration.
    #[serde(default)]
    pub asr: RealtimeASRConfig,
    /// TTS configuration.
    #[serde(default)]
    pub tts: RealtimeTTSConfig,
    /// Dialog configuration.
    #[serde(default)]
    pub dialog: RealtimeDialogConfig,
}

/// ASR configuration for realtime.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct RealtimeASRConfig {
    /// Extra parameters.
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub extra: HashMap<String, serde_json::Value>,
}

/// TTS configuration for realtime.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct RealtimeTTSConfig {
    /// Speaker voice.
    #[serde(default)]
    pub speaker: String,
    /// Audio configuration.
    #[serde(default)]
    pub audio_config: RealtimeAudioConfig,
    /// Extra parameters.
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub extra: HashMap<String, serde_json::Value>,
}

/// Audio configuration for realtime.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RealtimeAudioConfig {
    /// Number of channels.
    #[serde(default = "default_channel")]
    pub channel: i32,
    /// Audio format (e.g., "mp3", "pcm").
    #[serde(default = "default_format")]
    pub format: String,
    /// Sample rate in Hz.
    #[serde(default = "default_sample_rate")]
    pub sample_rate: i32,
}

fn default_channel() -> i32 {
    1
}
fn default_format() -> String {
    "mp3".to_string()
}
fn default_sample_rate() -> i32 {
    24000
}

impl Default for RealtimeAudioConfig {
    fn default() -> Self {
        Self {
            channel: 1,
            format: "mp3".to_string(),
            sample_rate: 24000,
        }
    }
}

/// Dialog configuration for realtime.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct RealtimeDialogConfig {
    /// Bot name.
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub bot_name: String,
    /// System role prompt.
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub system_role: String,
    /// Speaking style.
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub speaking_style: String,
    /// Character manifest.
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub character_manifest: String,
    /// Location information.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub location: Option<LocationInfo>,
    /// Extra parameters.
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub extra: HashMap<String, serde_json::Value>,
}

/// Location information.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct LocationInfo {
    pub longitude: f64,
    pub latitude: f64,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub city: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub country: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub province: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub district: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub town: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub country_code: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub address: String,
}

/// Realtime event.
#[derive(Debug, Default)]
pub struct RealtimeEvent {
    /// Event type.
    pub event_type: Option<RealtimeEventType>,
    /// Session ID.
    pub session_id: String,
    /// Text content.
    pub text: String,
    /// Audio data.
    pub audio: Option<Vec<u8>>,
    /// Raw payload.
    pub payload: Option<Vec<u8>>,
    /// ASR information.
    pub asr_info: Option<RealtimeASRInfo>,
    /// TTS information.
    pub tts_info: Option<RealtimeTTSInfo>,
    /// Error information.
    pub error: Option<Error>,
}

/// ASR information in realtime event.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct RealtimeASRInfo {
    /// Recognized text.
    pub text: String,
    /// Whether this is the final result.
    pub is_final: bool,
}

/// TTS information in realtime event.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct RealtimeTTSInfo {
    /// TTS type.
    pub tts_type: String,
    /// Content.
    pub content: String,
}

// ================== Service ==================

/// Realtime dialogue service.
pub struct RealtimeService {
    config: Arc<ClientConfig>,
}

impl RealtimeService {
    pub(crate) fn new(config: Arc<ClientConfig>) -> Self {
        Self { config }
    }

    /// Establishes a WebSocket connection to the realtime dialogue endpoint.
    pub async fn dial(&self) -> Result<RealtimeConnection> {
        let url = format!("{}/api/v3/realtime/dialogue", self.config.ws_url);
        let req_id = generate_req_id();

        // Build WebSocket request with auth headers
        let request = url
            .parse::<http::Uri>()
            .map_err(|e| Error::Config(format!("invalid URL: {}", e)))?;

        let host = request
            .host()
            .ok_or_else(|| Error::Config("missing host".to_string()))?;

        let auth = self.config.http.auth();
        let resource_id = "volc.speech.dialog";

        // Create WebSocket request
        let ws_url = format!(
            "{}?resource_id={}&request_id={}",
            url, resource_id, req_id
        );

        let mut ws_request = http::Request::builder()
            .uri(&ws_url)
            .header("Host", host)
            .header("X-Api-App-Key", &auth.app_id)
            .header("X-Api-Resource-Id", resource_id)
            .header("X-Api-Request-Id", &req_id);

        // Set authentication header
        if let Some(ref access_key) = auth.access_key {
            ws_request = ws_request.header("X-Api-Access-Key", access_key);
        } else if let Some(ref token) = auth.access_token {
            ws_request = ws_request.header("Authorization", format!("Bearer;{}", token));
        } else if let Some(ref api_key) = auth.api_key {
            ws_request = ws_request.header("x-api-key", api_key);
        }

        let ws_request = ws_request
            .body(())
            .map_err(|e| Error::Config(format!("build request: {}", e)))?;

        let (ws_stream, _response) = connect_async(ws_request)
            .await
            .map_err(|e| Error::Other(format!("websocket connect: {}", e)))?;

        let (write, read) = ws_stream.split();

        let conn = RealtimeConnection {
            config: self.config.clone(),
            write: Arc::new(Mutex::new(write)),
            read: Arc::new(Mutex::new(read)),
            proto: Arc::new(BinaryProtocol::new()),
            current_session: Arc::new(RwLock::new(None)),
            closed: Arc::new(RwLock::new(false)),
        };

        Ok(conn)
    }

    /// Establishes connection and starts a session (convenience method).
    pub async fn connect(&self, config: &RealtimeConfig) -> Result<RealtimeSession> {
        let conn = self.dial().await?;
        conn.start_session(config).await
    }
}

// ================== Connection ==================

type WsWriter = SplitSink<WebSocketStream<MaybeTlsStream<tokio::net::TcpStream>>, WsMessage>;
type WsReader = SplitStream<WebSocketStream<MaybeTlsStream<tokio::net::TcpStream>>>;

/// Realtime WebSocket connection.
pub struct RealtimeConnection {
    #[allow(dead_code)]
    config: Arc<ClientConfig>,
    write: Arc<Mutex<WsWriter>>,
    read: Arc<Mutex<WsReader>>,
    proto: Arc<BinaryProtocol>,
    current_session: Arc<RwLock<Option<mpsc::Sender<RealtimeEvent>>>>,
    closed: Arc<RwLock<bool>>,
}

impl RealtimeConnection {
    /// Starts a new session on this connection.
    pub async fn start_session(&self, config: &RealtimeConfig) -> Result<RealtimeSession> {
        let session_id = generate_req_id();

        // Build start request
        let start_req = self.build_start_request(&session_id, config);
        let json_data =
            serde_json::to_string(&start_req).map_err(|e| Error::Json(e))?;

        // Send start request
        {
            let mut write = self.write.lock().await;
            write
                .send(WsMessage::Text(json_data.into()))
                .await
                .map_err(|e| Error::Other(format!("send start: {}", e)))?;
        }

        // Create event channel
        let (tx, rx) = mpsc::channel(100);

        // Set current session
        {
            let mut current = self.current_session.write().await;
            *current = Some(tx.clone());
        }

        let session = RealtimeSession {
            session_id: session_id.clone(),
            conn_write: self.write.clone(),
            conn_read: self.read.clone(),
            proto: self.proto.clone(),
            event_rx: Arc::new(Mutex::new(rx)),
            event_tx: tx,
            closed: Arc::new(RwLock::new(false)),
            receive_task: Arc::new(Mutex::new(None)),
        };

        // Start receive loop
        let session_clone = RealtimeSession {
            session_id: session.session_id.clone(),
            conn_write: session.conn_write.clone(),
            conn_read: session.conn_read.clone(),
            proto: session.proto.clone(),
            event_rx: session.event_rx.clone(),
            event_tx: session.event_tx.clone(),
            closed: session.closed.clone(),
            receive_task: session.receive_task.clone(),
        };

        let handle = tokio::spawn(async move {
            session_clone.receive_loop().await;
        });

        // Store the task handle for graceful shutdown
        {
            let mut task = session.receive_task.lock().await;
            *task = Some(handle);
        }

        // Wait for session start confirmation
        let mut rx = session.event_rx.lock().await;
        match tokio::time::timeout(std::time::Duration::from_secs(10), rx.recv()).await {
            Ok(Some(_event)) => {}
            Ok(None) => {
                return Err(Error::Other("connection closed".to_string()));
            }
            Err(_) => {
                return Err(Error::Other("timeout waiting for session start".to_string()));
            }
        }
        drop(rx);

        Ok(session)
    }

    fn build_start_request(
        &self,
        session_id: &str,
        config: &RealtimeConfig,
    ) -> serde_json::Value {
        let mut dialog = serde_json::json!({
            "bot_name": config.dialog.bot_name,
            "system_role": config.dialog.system_role,
            "speaking_style": config.dialog.speaking_style,
            "character_manifest": config.dialog.character_manifest,
            "extra": config.dialog.extra,
        });

        if let Some(ref loc) = config.dialog.location {
            dialog["location"] = serde_json::json!({
                "longitude": loc.longitude,
                "latitude": loc.latitude,
                "city": loc.city,
                "country": loc.country,
                "province": loc.province,
                "district": loc.district,
                "town": loc.town,
                "country_code": loc.country_code,
                "address": loc.address,
            });
        }

        serde_json::json!({
            "type": "start",
            "data": {
                "session_id": session_id,
                "config": {
                    "asr": {
                        "extra": config.asr.extra,
                    },
                    "tts": {
                        "speaker": config.tts.speaker,
                        "audio_config": {
                            "channel": config.tts.audio_config.channel,
                            "format": config.tts.audio_config.format,
                            "sample_rate": config.tts.audio_config.sample_rate,
                        },
                    },
                    "dialog": dialog,
                }
            }
        })
    }

    /// Closes the connection.
    pub async fn close(&self) -> Result<()> {
        let mut closed = self.closed.write().await;
        if *closed {
            return Ok(());
        }
        *closed = true;

        let mut write = self.write.lock().await;
        let _ = write.close().await;

        Ok(())
    }
}

// ================== Session ==================

/// Realtime dialogue session.
pub struct RealtimeSession {
    session_id: String,
    conn_write: Arc<Mutex<WsWriter>>,
    conn_read: Arc<Mutex<WsReader>>,
    proto: Arc<BinaryProtocol>,
    event_rx: Arc<Mutex<mpsc::Receiver<RealtimeEvent>>>,
    event_tx: mpsc::Sender<RealtimeEvent>,
    closed: Arc<RwLock<bool>>,
    /// Handle to the background receive task for graceful shutdown.
    receive_task: Arc<Mutex<Option<tokio::task::JoinHandle<()>>>>,
}

impl RealtimeSession {
    /// Returns the session ID.
    pub fn session_id(&self) -> &str {
        &self.session_id
    }

    /// Sends audio data.
    pub async fn send_audio(&self, audio: &[u8]) -> Result<()> {
        if *self.closed.read().await {
            return Err(Error::Other("session closed".to_string()));
        }

        let msg = Message::audio_only(&self.session_id, audio.to_vec(), events::AUDIO_RECEIVED);
        let data = self.proto.marshal(&msg)?;

        let mut write = self.conn_write.lock().await;
        write
            .send(WsMessage::Binary(data.into()))
            .await
            .map_err(|e| Error::Other(format!("send audio: {}", e)))
    }

    /// Sends text message.
    pub async fn send_text(&self, text: &str) -> Result<()> {
        if *self.closed.read().await {
            return Err(Error::Other("session closed".to_string()));
        }

        let msg = serde_json::json!({
            "type": "text",
            "data": {
                "session_id": self.session_id,
                "text": text,
            }
        });

        let json_data = serde_json::to_string(&msg).map_err(|e| Error::Json(e))?;

        let mut write = self.conn_write.lock().await;
        write
            .send(WsMessage::Text(json_data.into()))
            .await
            .map_err(|e| Error::Other(format!("send text: {}", e)))
    }

    /// Sends a greeting (say hello).
    pub async fn say_hello(&self, content: &str) -> Result<()> {
        if *self.closed.read().await {
            return Err(Error::Other("session closed".to_string()));
        }

        let msg = serde_json::json!({
            "type": "say_hello",
            "data": {
                "session_id": self.session_id,
                "content": content,
            }
        });

        let json_data = serde_json::to_string(&msg).map_err(|e| Error::Json(e))?;

        let mut write = self.conn_write.lock().await;
        write
            .send(WsMessage::Text(json_data.into()))
            .await
            .map_err(|e| Error::Other(format!("send say_hello: {}", e)))
    }

    /// Interrupts current TTS playback.
    pub async fn interrupt(&self) -> Result<()> {
        if *self.closed.read().await {
            return Err(Error::Other("session closed".to_string()));
        }

        let msg = serde_json::json!({
            "type": "cancel",
            "data": {
                "session_id": self.session_id,
            }
        });

        let json_data = serde_json::to_string(&msg).map_err(|e| Error::Json(e))?;

        let mut write = self.conn_write.lock().await;
        write
            .send(WsMessage::Text(json_data.into()))
            .await
            .map_err(|e| Error::Other(format!("send interrupt: {}", e)))
    }

    /// Receives the next event.
    pub async fn recv(&self) -> Option<Result<RealtimeEvent>> {
        let mut rx = self.event_rx.lock().await;
        rx.recv().await.map(Ok)
    }

    /// Closes the session.
    ///
    /// This will signal the receive loop to stop and wait for it to complete,
    /// ensuring graceful shutdown.
    pub async fn close(&self) -> Result<()> {
        let mut closed = self.closed.write().await;
        if *closed {
            return Ok(());
        }
        *closed = true;
        drop(closed); // Release lock before awaiting task

        // Send finish message
        let msg = serde_json::json!({
            "type": "finish",
            "data": {
                "session_id": self.session_id,
            }
        });

        let json_data = serde_json::to_string(&msg).map_err(|e| Error::Json(e))?;

        {
            let mut write = self.conn_write.lock().await;
            let _ = write.send(WsMessage::Text(json_data.into())).await;
        }

        // Wait for the receive task to complete
        let handle = {
            let mut task = self.receive_task.lock().await;
            task.take()
        };
        if let Some(handle) = handle {
            // Wait with timeout to avoid hanging indefinitely
            let _ = tokio::time::timeout(
                std::time::Duration::from_secs(5),
                handle,
            ).await;
        }

        Ok(())
    }

    async fn receive_loop(&self) {
        loop {
            if *self.closed.read().await {
                break;
            }

            let msg = {
                let mut read = self.conn_read.lock().await;
                match read.next().await {
                    Some(Ok(msg)) => msg,
                    Some(Err(e)) => {
                        let _ = self.event_tx.send(RealtimeEvent {
                            error: Some(Error::Other(format!("websocket: {}", e))),
                            ..Default::default()
                        }).await;
                        break;
                    }
                    None => break,
                }
            };

            let event = match msg {
                WsMessage::Binary(data) => self.parse_binary_message(&data),
                WsMessage::Text(text) => self.parse_json_message(&text),
                WsMessage::Close(_) => break,
                _ => continue,
            };

            if let Some(event) = event {
                if self.event_tx.send(event).await.is_err() {
                    break;
                }
            }
        }
    }

    fn parse_binary_message(&self, data: &[u8]) -> Option<RealtimeEvent> {
        let msg = self.proto.unmarshal(data).ok()?;

        let mut event = RealtimeEvent {
            event_type: Some(RealtimeEventType::from(msg.event)),
            session_id: msg.session_id.clone(),
            ..Default::default()
        };

        if msg.is_audio_only() {
            event.audio = Some(msg.payload.clone());
            event.event_type = Some(RealtimeEventType::AudioReceived);
        } else if !msg.payload.is_empty() {
            event.payload = Some(msg.payload.clone());

            // Try to parse JSON payload
            #[derive(Deserialize)]
            struct PayloadInfo {
                #[serde(default)]
                text: String,
                #[serde(default)]
                session_id: String,
                #[serde(default)]
                asr_info: Option<RealtimeASRInfo>,
                #[serde(default)]
                tts_info: Option<RealtimeTTSInfo>,
            }

            if let Ok(info) = serde_json::from_slice::<PayloadInfo>(&msg.payload) {
                if !info.session_id.is_empty() {
                    event.session_id = info.session_id;
                }
                event.text = info.text;
                event.asr_info = info.asr_info;
                event.tts_info = info.tts_info;
            }
        }

        if msg.is_error() {
            event.error = Some(Error::Api {
                code: msg.error_code as i32,
                message: String::from_utf8_lossy(&msg.payload).to_string(),
                req_id: String::new(),
                log_id: String::new(),
                trace_id: String::new(),
                http_status: 0,
            });
        }

        Some(event)
    }

    fn parse_json_message(&self, text: &str) -> Option<RealtimeEvent> {
        #[derive(Deserialize)]
        struct JsonMessage {
            #[serde(rename = "type")]
            msg_type: String,
            data: JsonData,
        }

        #[derive(Deserialize)]
        struct JsonData {
            #[serde(default)]
            session_id: String,
            #[serde(default)]
            role: String,
            #[serde(default)]
            content: String,
            #[serde(default)]
            text: String,
            #[serde(default)]
            is_final: bool,
            #[serde(default)]
            audio: String,
        }

        let json_msg: JsonMessage = serde_json::from_str(text).ok()?;

        let mut event = RealtimeEvent {
            session_id: json_msg.data.session_id,
            ..Default::default()
        };

        match json_msg.msg_type.as_str() {
            "text" => {
                if json_msg.data.role == "user" {
                    event.event_type = Some(RealtimeEventType::ASRFinished);
                    event.asr_info = Some(RealtimeASRInfo {
                        text: json_msg.data.content.clone(),
                        is_final: json_msg.data.is_final,
                    });
                } else {
                    event.event_type = Some(RealtimeEventType::TTSStarted);
                    event.text = json_msg.data.content;
                }
            }
            "audio" => {
                event.event_type = Some(RealtimeEventType::AudioReceived);
                if !json_msg.data.audio.is_empty() {
                    if let Ok(audio_data) =
                        base64::engine::general_purpose::STANDARD.decode(&json_msg.data.audio)
                    {
                        event.audio = Some(audio_data);
                    }
                }
            }
            "status" => {
                event.event_type = Some(RealtimeEventType::SessionStarted);
            }
            "error" => {
                event.event_type = Some(RealtimeEventType::SessionFailed);
                event.error = Some(Error::Api {
                    code: 0,
                    message: json_msg.data.content,
                    req_id: String::new(),
                    log_id: String::new(),
                    trace_id: String::new(),
                    http_status: 0,
                });
            }
            _ => return None,
        }

        Some(event)
    }
}
