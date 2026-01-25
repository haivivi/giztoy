//! ASR V2 Service - BigModel ASR (大模型语音识别)
//!
//! V2 uses /api/v3/* endpoints with X-Api-* headers authentication.
//!
//! # Endpoints
//!
//! - WSS `/api/v3/sauc/bigmodel` - Streaming ASR (流式语音识别)
//! - POST `/api/v3/sauc/bigmodel_async` - Async File ASR (异步文件识别)
//!
//! # Resource IDs
//!
//! - `volc.bigasr.sauc.duration`: BigModel streaming ASR (时长版)
//! - `volc.seedasr.sauc.duration`: BigModel streaming ASR 2.0
//! - `volc.bigasr.auc.duration`: BigModel file ASR
//!
//! # Documentation
//!
//! <https://www.volcengine.com/docs/6561/1354868>

use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};

use futures::stream::{SplitSink, SplitStream};
use futures::{SinkExt, StreamExt};
use serde::{Deserialize, Serialize};
use tokio::sync::{mpsc, Mutex};
use tokio_tungstenite::tungstenite::Message as WsMessage;
use tokio_tungstenite::{connect_async, MaybeTlsStream, WebSocketStream};

use crate::{
    error::{Error, Result},
    http::HttpClient,
    protocol::{BinaryProtocol, CompressionType, Message, MessageFlags, MessageType},
};

/// Resource ID for BigModel streaming ASR.
pub const RESOURCE_ASR_STREAM: &str = "volc.bigasr.sauc.duration";

/// Resource ID for BigModel file ASR.
pub const RESOURCE_ASR_FILE: &str = "volc.bigasr.auc.duration";

// ================== Service ==================

/// ASR V2 service provides BigModel ASR functionality.
pub struct AsrV2Service {
    http: Arc<HttpClient>,
}

impl AsrV2Service {
    /// Creates a new ASR V2 service.
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Opens a streaming ASR WebSocket session.
    ///
    /// Uses endpoint: WSS `/api/v3/sauc/bigmodel`
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_doubaospeech::{Client, AsrV2Config};
    ///
    /// # async fn example() -> anyhow::Result<()> {
    /// let client = Client::builder("app-id").api_key("api-key").build()?;
    /// let mut session = client.asr_v2().open_stream_session(&AsrV2Config {
    ///     format: "pcm".to_string(),
    ///     sample_rate: 16000,
    ///     language: Some("zh-CN".to_string()),
    ///     ..Default::default()
    /// }).await?;
    ///
    /// // Send audio chunks
    /// session.send_audio(&audio_data, false).await?;
    /// session.send_audio(&last_chunk, true).await?;
    ///
    /// // Receive results
    /// while let Some(result) = session.recv().await {
    ///     let result = result?;
    ///     println!("Text: {}", result.text);
    ///     if result.is_final {
    ///         break;
    ///     }
    /// }
    /// # Ok(())
    /// # }
    /// ```
    pub async fn open_stream_session(&self, config: &AsrV2Config) -> Result<AsrV2Session> {
        use tokio_tungstenite::tungstenite::client::IntoClientRequest;

        let auth = self.http.auth();
        let resource_id = config.resource_id.as_deref().unwrap_or(RESOURCE_ASR_STREAM);
        let connect_id = format!("asr-{}", std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_nanos());

        // Build WebSocket URL
        let ws_url = format!("{}/api/v3/sauc/bigmodel", self.http.ws_url());

        // Build request with headers
        let mut request = ws_url.into_client_request()
            .map_err(|e| Error::Other(format!("build ws request: {}", e)))?;

        let headers = request.headers_mut();
        headers.insert("X-Api-App-Id", auth.app_id.parse().unwrap());
        headers.insert("X-Api-Resource-Id", resource_id.parse().unwrap());
        headers.insert("X-Api-Connect-Id", connect_id.parse().unwrap());

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

        // Create protocol handler
        let proto = BinaryProtocol::new();

        // Create channel for receiving results
        let (recv_tx, recv_rx) = mpsc::channel(100);
        let closed = Arc::new(AtomicBool::new(false));

        let session = AsrV2Session {
            write: Arc::new(Mutex::new(write)),
            proto: proto.clone(),
            config: config.clone(),
            user_id: auth.user_id.clone(),
            req_id: connect_id.clone(),
            recv_rx,
            closed: closed.clone(),
        };

        // Start receive loop
        tokio::spawn(async move {
            asr_receive_loop(read, recv_tx, proto, closed).await;
        });

        // Send session start
        session.send_session_start().await?;

        Ok(session)
    }

    /// Submits an async file ASR task.
    ///
    /// Uses endpoint: POST `/api/v3/sauc/bigmodel_async`
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_doubaospeech::{Client, AsrV2AsyncRequest};
    ///
    /// # async fn example() -> anyhow::Result<()> {
    /// let client = Client::builder("app-id").api_key("api-key").build()?;
    /// let result = client.asr_v2().submit_async(&AsrV2AsyncRequest {
    ///     audio_url: Some("https://example.com/audio.mp3".to_string()),
    ///     format: "mp3".to_string(),
    ///     language: Some("zh-CN".to_string()),
    ///     ..Default::default()
    /// }).await?;
    /// println!("Task ID: {}", result.task_id);
    /// # Ok(())
    /// # }
    /// ```
    pub async fn submit_async(&self, req: &AsrV2AsyncRequest) -> Result<AsrV2AsyncResult> {
        let resource_id = req.resource_id.as_deref().unwrap_or(RESOURCE_ASR_FILE);
        let headers = self.http.v2_headers(Some(resource_id))?;

        let body = serde_json::json!({
            "user": {
                "uid": self.http.auth().user_id,
            },
            "audio": {
                "format": req.format,
                "url": req.audio_url,
            },
            "req_params": {
                "language": req.language,
                "enable_itn": req.enable_itn,
                "enable_punc": req.enable_punc,
                "enable_diarization": req.enable_diarization,
                "speaker_num": req.speaker_num,
            },
            "callback_url": req.callback_url,
        });

        let url = format!("{}/api/v3/sauc/bigmodel_async", self.http.base_url());

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

        let result: AsrV2AsyncResult = response.json().await?;
        Ok(result)
    }

    /// Queries the status of an async ASR task.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let result = client.asr_v2().query_async("task-id").await?;
    /// println!("Status: {}", result.status);
    /// ```
    pub async fn query_async(&self, task_id: &str) -> Result<AsrV2AsyncResult> {
        let headers = self.http.v2_headers(Some(RESOURCE_ASR_FILE))?;

        let url = format!("{}/api/v3/sauc/bigmodel_async/{}", self.http.base_url(), task_id);

        let client = reqwest::Client::new();
        let response = client
            .get(&url)
            .headers(headers)
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

        let result: AsrV2AsyncResult = response.json().await?;
        Ok(result)
    }
}

// ================== Types ==================

/// ASR V2 streaming configuration.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct AsrV2Config {
    /// Audio format: pcm, wav, mp3, ogg_opus, etc.
    pub format: String,

    /// Sample rate in Hz (8000, 16000, etc.)
    pub sample_rate: i32,

    /// Number of audio channels (1 or 2)
    #[serde(default = "default_channels")]
    pub channels: i32,

    /// Bits per sample (16, etc.)
    #[serde(default = "default_bits")]
    pub bits: i32,

    /// Language: zh-CN, en-US, ja-JP, etc.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub language: Option<String>,

    /// Enable ITN (Inverse Text Normalization)
    #[serde(default)]
    pub enable_itn: bool,

    /// Enable punctuation
    #[serde(default)]
    pub enable_punc: bool,

    /// Enable speaker diarization
    #[serde(default)]
    pub enable_diarization: bool,

    /// Number of speakers (for diarization)
    #[serde(default)]
    pub speaker_num: i32,

    /// Resource ID (default: volc.bigasr.sauc.duration)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub resource_id: Option<String>,

    /// Hotwords for recognition boost
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub hotwords: Vec<String>,
}

fn default_channels() -> i32 { 1 }
fn default_bits() -> i32 { 16 }

/// ASR V2 recognition result.
#[derive(Debug, Clone, Default)]
pub struct AsrV2Result {
    /// Recognized text
    pub text: String,

    /// Utterance list with detailed info
    pub utterances: Vec<AsrV2Utterance>,

    /// Is final result (sentence complete)
    pub is_final: bool,

    /// Audio duration in milliseconds
    pub duration: i32,

    /// Request ID
    pub req_id: String,
}

/// A single utterance in ASR result.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct AsrV2Utterance {
    /// Text content
    pub text: String,

    /// Start time in milliseconds
    pub start_time: i32,

    /// End time in milliseconds
    pub end_time: i32,

    /// Whether this utterance is final
    pub definite: bool,

    /// Speaker ID (for diarization)
    #[serde(default)]
    pub speaker_id: String,

    /// Word-level details
    #[serde(default)]
    pub words: Vec<AsrV2Word>,

    /// Confidence score
    #[serde(default)]
    pub confidence: f64,
}

/// A word in ASR utterance.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct AsrV2Word {
    /// Word text
    pub text: String,

    /// Start time in milliseconds
    pub start_time: i32,

    /// End time in milliseconds
    pub end_time: i32,

    /// Confidence score
    #[serde(default)]
    pub conf: f64,
}

/// Async file ASR request.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct AsrV2AsyncRequest {
    /// Audio URL (required)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio_url: Option<String>,

    /// Audio format
    pub format: String,

    /// Language
    #[serde(skip_serializing_if = "Option::is_none")]
    pub language: Option<String>,

    /// Enable ITN
    #[serde(default)]
    pub enable_itn: bool,

    /// Enable punctuation
    #[serde(default)]
    pub enable_punc: bool,

    /// Enable speaker diarization
    #[serde(default)]
    pub enable_diarization: bool,

    /// Number of speakers
    #[serde(default)]
    pub speaker_num: i32,

    /// Callback URL for result notification
    #[serde(skip_serializing_if = "Option::is_none")]
    pub callback_url: Option<String>,

    /// Resource ID
    #[serde(skip_serializing_if = "Option::is_none")]
    pub resource_id: Option<String>,
}

/// Async ASR result.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct AsrV2AsyncResult {
    /// Task ID
    #[serde(default)]
    pub task_id: String,

    /// Recognition result text
    #[serde(default)]
    pub text: String,

    /// Detailed utterances
    #[serde(default)]
    pub utterances: Vec<AsrV2Utterance>,

    /// Task status: pending, processing, success, failed
    #[serde(default)]
    pub status: String,

    /// Error message (if failed)
    #[serde(default)]
    pub error: String,

    /// Request ID
    #[serde(default)]
    pub reqid: String,
}

// ================== Session ==================

type WsStream = WebSocketStream<MaybeTlsStream<tokio::net::TcpStream>>;

/// ASR V2 streaming session.
pub struct AsrV2Session {
    write: Arc<Mutex<SplitSink<WsStream, WsMessage>>>,
    proto: BinaryProtocol,
    config: AsrV2Config,
    user_id: String,
    req_id: String,
    recv_rx: mpsc::Receiver<AsrV2Result>,
    closed: Arc<AtomicBool>,
}

impl AsrV2Session {
    /// Sends audio data to the ASR session.
    ///
    /// If `is_last` is true, this marks the end of the audio stream.
    pub async fn send_audio(&self, audio: &[u8], is_last: bool) -> Result<()> {
        if self.closed.load(Ordering::Relaxed) {
            return Err(Error::Other("session closed".to_string()));
        }

        // Build audio-only message
        // SAUC protocol: flags=0 for normal, flags=2 for last frame
        let flags = if is_last {
            MessageFlags::NegSequence // 0x02 = last frame
        } else {
            MessageFlags::NoSequence // 0x00 = normal
        };

        let msg = Message {
            msg_type: MessageType::AudioOnlyClient,
            flags,
            payload: audio.to_vec(),
            ..Default::default()
        };

        // Marshal audio-only message (no sequence, no event)
        let data = self.marshal_audio_message(&msg)?;

        self.write.lock().await
            .send(WsMessage::Binary(data.into()))
            .await
            .map_err(Error::WebSocket)
    }

    /// Receives the next recognition result.
    ///
    /// Returns `None` when the session is closed.
    pub async fn recv(&mut self) -> Option<Result<AsrV2Result>> {
        self.recv_rx.recv().await.map(Ok)
    }

    /// Closes the session.
    pub async fn close(&self) -> Result<()> {
        if self.closed.swap(true, Ordering::Relaxed) {
            return Ok(());
        }
        // Send finish message
        let _ = self.send_session_finish().await;
        self.write.lock().await.close().await.map_err(Error::WebSocket)?;
        Ok(())
    }

    async fn send_session_start(&self) -> Result<()> {
        let audio = serde_json::json!({
            "format": self.config.format,
            "sample_rate": self.config.sample_rate,
            "channel": self.config.channels,
            "bits": self.config.bits,
        });

        let mut request = serde_json::json!({
            "reqid": self.req_id,
            "sequence": 1,
            "show_utterances": true,
            "result_type": "single",
        });

        if let Some(ref lang) = self.config.language {
            request["language"] = serde_json::json!(lang);
        }
        if self.config.enable_itn {
            request["enable_itn"] = serde_json::json!(true);
        }
        if self.config.enable_punc {
            request["enable_punc"] = serde_json::json!(true);
        }
        if self.config.enable_diarization {
            request["enable_diarization"] = serde_json::json!(true);
        }
        if !self.config.hotwords.is_empty() {
            request["hotwords"] = serde_json::json!(self.config.hotwords);
        }
        if self.config.speaker_num > 0 {
            request["speaker_num"] = serde_json::json!(self.config.speaker_num);
        }

        let payload = serde_json::json!({
            "user": {
                "uid": self.user_id,
            },
            "audio": audio,
            "request": request,
        });

        self.send_json_message(&payload).await
    }

    async fn send_session_finish(&self) -> Result<()> {
        let payload = serde_json::json!({
            "event": 2, // SessionFinish
        });
        self.send_json_message(&payload).await
    }

    async fn send_json_message(&self, payload: &serde_json::Value) -> Result<()> {
        let json_data = serde_json::to_vec(payload)
            .map_err(|e| Error::Other(format!("serialize payload: {}", e)))?;

        let msg = Message {
            msg_type: MessageType::FullClient,
            flags: MessageFlags::NoSequence,
            payload: json_data,
            ..Default::default()
        };

        let data = self.proto.marshal(&msg)?;

        self.write.lock().await
            .send(WsMessage::Binary(data.into()))
            .await
            .map_err(Error::WebSocket)
    }

    /// Marshal audio-only message (simplified format for SAUC).
    fn marshal_audio_message(&self, msg: &Message) -> Result<Vec<u8>> {
        use bytes::{BufMut, BytesMut};

        let mut buf = BytesMut::with_capacity(msg.payload.len() + 12);

        // Header (4 bytes)
        buf.put_u8(0x11); // version=1, header_size=1
        buf.put_u8((msg.msg_type as u8) << 4 | (msg.flags as u8));
        buf.put_u8(0x00); // serialization=none, compression=none
        buf.put_u8(0x00); // reserved

        // Payload length + payload (no sequence for audio-only)
        buf.put_u32(msg.payload.len() as u32);
        buf.put_slice(&msg.payload);

        Ok(buf.to_vec())
    }
}

/// Receives messages from WebSocket and sends to channel.
async fn asr_receive_loop(
    mut read: SplitStream<WsStream>,
    tx: mpsc::Sender<AsrV2Result>,
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
                match parse_asr_response(&proto, &data) {
                    Ok(Some(result)) => {
                        let is_final = result.is_final;
                        if tx.send(result).await.is_err() {
                            break;
                        }
                        if is_final {
                            break;
                        }
                    }
                    Ok(None) => continue,
                    Err(_) => break,
                }
            }
            WsMessage::Text(text) => {
                // Server might send text error message
                eprintln!("ASR server text: {}", text);
                break;
            }
            WsMessage::Close(_) => break,
            _ => continue,
        }
    }
}

/// Parses an ASR V2 WebSocket response.
fn parse_asr_response(_proto: &BinaryProtocol, data: &[u8]) -> Result<Option<AsrV2Result>> {
    use bytes::Buf;
    use std::io::Cursor;
    use flate2::read::GzDecoder;
    use std::io::Read;

    if data.len() < 12 {
        return Ok(None);
    }

    // SAUC protocol format:
    // Byte 0: version (4 bits) + header_size (4 bits) = 0x11
    // Byte 1: message_type (4 bits) + flags (4 bits)
    // Byte 2: serialization (4 bits) + compression (4 bits)
    // Byte 3: reserved
    // Byte 4-7: sequence number (4 bytes, big-endian)
    // Byte 8-11: payload size (4 bytes, big-endian)
    // Byte 12+: payload

    let message_type = (data[1] >> 4) & 0x0F;
    let message_flags = data[1] & 0x0F;
    let compression = data[2] & 0x0F;

    let mut cursor = Cursor::new(&data[4..]);
    let _sequence = cursor.get_u32(); // sequence number (unused)
    let payload_size = cursor.get_u32() as usize;

    if data.len() < 12 + payload_size {
        return Ok(None);
    }

    let mut payload = data[12..12 + payload_size].to_vec();

    // Decompress if needed
    if compression == CompressionType::Gzip as u8 {
        let mut decoder = GzDecoder::new(&payload[..]);
        let mut decompressed = Vec::new();
        decoder.read_to_end(&mut decompressed)
            .map_err(|e| Error::Other(format!("gzip decompress: {}", e)))?;
        payload = decompressed;
    }

    // Full server response (message_type = 9)
    if message_type == MessageType::FullServer as u8 {
        #[derive(Deserialize)]
        struct AsrResponse {
            audio_info: Option<AudioInfo>,
            result: Option<ResultData>,
        }

        #[derive(Deserialize)]
        struct AudioInfo {
            #[serde(default)]
            duration: i32,
        }

        #[derive(Deserialize, Default)]
        struct ResultData {
            #[serde(default)]
            text: String,
            #[serde(default)]
            utterances: Vec<UtteranceData>,
        }

        #[derive(Deserialize)]
        struct UtteranceData {
            #[serde(default)]
            text: String,
            #[serde(default)]
            start_time: i32,
            #[serde(default)]
            end_time: i32,
            #[serde(default)]
            definite: bool,
            #[serde(default)]
            words: Vec<WordData>,
        }

        #[derive(Deserialize)]
        struct WordData {
            #[serde(default)]
            text: String,
            #[serde(default)]
            start_time: i32,
            #[serde(default)]
            end_time: i32,
        }

        if let Ok(resp) = serde_json::from_slice::<AsrResponse>(&payload) {
            // Check if final: flags=3 or any utterance is definite
            let mut is_final = message_flags == 3;

            let result_data = resp.result.unwrap_or_default();

            let mut utterances = Vec::new();
            for u in result_data.utterances {
                if u.definite {
                    is_final = true;
                }
                let mut words = Vec::new();
                for w in u.words {
                    words.push(AsrV2Word {
                        text: w.text,
                        start_time: w.start_time,
                        end_time: w.end_time,
                        conf: 0.0,
                    });
                }
                utterances.push(AsrV2Utterance {
                    text: u.text,
                    start_time: u.start_time,
                    end_time: u.end_time,
                    definite: u.definite,
                    words,
                    ..Default::default()
                });
            }

            return Ok(Some(AsrV2Result {
                text: result_data.text,
                utterances,
                duration: resp.audio_info.map(|a| a.duration).unwrap_or(0),
                is_final,
                req_id: String::new(),
            }));
        }
    }

    // Error message (message_type = 15)
    if message_type == MessageType::Error as u8 {
        #[derive(Deserialize)]
        struct ErrorResponse {
            #[serde(default)]
            code: i32,
            #[serde(default)]
            message: String,
        }

        if let Ok(err) = serde_json::from_slice::<ErrorResponse>(&payload) {
            return Err(Error::api(err.code, err.message, 200));
        }
    }

    Ok(None)
}
