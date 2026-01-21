//! Translation service for Doubao Speech API.
//!
//! Provides real-time speech translation functionality via WebSocket.

use std::sync::Arc;

use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use futures::stream::{SplitSink, SplitStream};
use futures::{SinkExt, StreamExt};
use serde::{Deserialize, Serialize};
use tokio::sync::Mutex;
use tokio_tungstenite::tungstenite::Message as WsMessage;
use tokio_tungstenite::{connect_async, MaybeTlsStream, WebSocketStream};

use crate::{
    client::generate_req_id,
    error::{Error, Result},
    http::HttpClient,
    types::{AudioFormat, Language, SampleRate},
};

type WsStream = WebSocketStream<MaybeTlsStream<tokio::net::TcpStream>>;

/// Translation service for real-time speech translation.
///
/// API Documentation: https://www.volcengine.com/docs/6561/1305191
pub struct TranslationService {
    http: Arc<HttpClient>,
}

impl TranslationService {
    /// Creates a new Translation service.
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Returns the HTTP client for WebSocket connection setup.
    pub fn http(&self) -> &Arc<HttpClient> {
        &self.http
    }

    /// Opens a streaming translation session.
    ///
    /// Returns a `TranslationSession` for sending audio and receiving translation results.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_doubaospeech::{Client, TranslationConfig, TranslationAudioConfig, Language, AudioFormat, SampleRate};
    ///
    /// let client = Client::builder("app-id").bearer_token("token").build()?;
    /// let config = TranslationConfig {
    ///     source_language: Language::ZhCN,
    ///     target_language: Language::EnUS,
    ///     audio_config: TranslationAudioConfig {
    ///         format: AudioFormat::Pcm,
    ///         sample_rate: SampleRate::Rate16000,
    ///         channel: 1,
    ///         bits: 16,
    ///     },
    ///     enable_tts: false,
    ///     tts_voice: None,
    /// };
    /// let mut session = client.translation().open_session(&config).await?;
    ///
    /// // Send audio data
    /// session.send_audio(&audio_data, false).await?;
    /// session.send_audio(&last_chunk, true).await?;
    ///
    /// // Receive results
    /// while let Some(result) = session.recv().await {
    ///     match result {
    ///         Ok(chunk) => {
    ///             println!("Source: {}", chunk.source_text);
    ///             println!("Target: {}", chunk.target_text);
    ///         }
    ///         Err(e) => eprintln!("Error: {}", e),
    ///     }
    /// }
    /// ```
    pub async fn open_session(&self, config: &TranslationConfig) -> Result<TranslationSession> {
        let auth = self.http.auth();
        let ws_url = format!(
            "{}/api/v2/st?{}",
            self.http.ws_url(),
            self.http.ws_auth_params()
        );

        let (ws_stream, _) = connect_async(&ws_url)
            .await
            .map_err(|e| Error::WebSocket(e))?;

        let (write, read) = ws_stream.split();
        let req_id = generate_req_id();

        let session = TranslationSession {
            write: Arc::new(Mutex::new(write)),
            read: Arc::new(Mutex::new(read)),
            req_id: req_id.clone(),
            closed: Arc::new(std::sync::atomic::AtomicBool::new(false)),
        };

        // Send start request
        let cluster = auth
            .cluster
            .clone()
            .unwrap_or_else(|| "volcengine_streaming_common".to_string());

        let mut start_req = serde_json::json!({
            "app": {
                "appid": auth.app_id,
                "cluster": cluster,
            },
            "user": {
                "uid": auth.user_id,
            },
            "audio": {
                "format": config.audio_config.format.as_str(),
                "sample_rate": config.audio_config.sample_rate.as_i32(),
                "channel": config.audio_config.channel,
                "bits": config.audio_config.bits,
            },
            "request": {
                "reqid": req_id,
                "source_language": config.source_language.as_str(),
                "target_language": config.target_language.as_str(),
                "enable_asr": true,
                "enable_tts": config.enable_tts,
            }
        });

        if config.enable_tts {
            if let Some(ref voice) = config.tts_voice {
                start_req["request"]["tts_voice_type"] = serde_json::Value::String(voice.clone());
            }
        }

        let msg = WsMessage::Text(serde_json::to_string(&start_req)?.into());
        session.write.lock().await.send(msg).await.map_err(|e| Error::WebSocket(e))?;

        Ok(session)
    }
}

// ================== Configuration Types ==================

/// Translation session configuration.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TranslationConfig {
    /// Source language.
    pub source_language: Language,
    /// Target language.
    pub target_language: Language,
    /// Audio configuration.
    pub audio_config: TranslationAudioConfig,
    /// Enable TTS output.
    #[serde(default)]
    pub enable_tts: bool,
    /// TTS voice type (when enable_tts is true).
    #[serde(default)]
    pub tts_voice: Option<String>,
}

/// Audio configuration for translation.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TranslationAudioConfig {
    /// Audio format.
    pub format: AudioFormat,
    /// Sample rate.
    pub sample_rate: SampleRate,
    /// Number of channels.
    #[serde(default = "default_channel")]
    pub channel: i32,
    /// Bits per sample.
    #[serde(default = "default_bits")]
    pub bits: i32,
}

fn default_channel() -> i32 {
    1
}

fn default_bits() -> i32 {
    16
}

// ================== Response Types ==================

/// Translation chunk from streaming session.
#[derive(Debug, Clone, Default, Serialize)]
pub struct TranslationChunk {
    /// Source text (recognized speech).
    pub source_text: String,
    /// Target text (translated).
    pub target_text: String,
    /// Whether this is a definite (final) result.
    pub is_definite: bool,
    /// Whether this is the final chunk.
    pub is_final: bool,
    /// Sequence number.
    pub sequence: i32,
    /// Audio data (if TTS enabled).
    pub audio: Option<Vec<u8>>,
}

// ================== Translation Session ==================

/// Streaming translation session for real-time speech translation.
pub struct TranslationSession {
    write: Arc<Mutex<SplitSink<WsStream, WsMessage>>>,
    read: Arc<Mutex<SplitStream<WsStream>>>,
    req_id: String,
    closed: Arc<std::sync::atomic::AtomicBool>,
}

impl TranslationSession {
    /// Sends audio data to the translation session.
    ///
    /// Set `is_last` to true for the final audio chunk.
    pub async fn send_audio(&self, audio: &[u8], is_last: bool) -> Result<()> {
        if self.closed.load(std::sync::atomic::Ordering::Relaxed) {
            return Err(Error::Other("session is closed".to_string()));
        }

        // Send audio data as binary
        let msg = WsMessage::Binary(audio.to_vec().into());
        self.write.lock().await.send(msg).await.map_err(|e| Error::WebSocket(e))?;

        // If last frame, send finish command
        if is_last {
            let finish_req = serde_json::json!({
                "request": {
                    "reqid": self.req_id,
                    "command": "finish"
                }
            });
            let msg = WsMessage::Text(serde_json::to_string(&finish_req)?.into());
            self.write.lock().await.send(msg).await.map_err(|e| Error::WebSocket(e))?;
        }

        Ok(())
    }

    /// Receives the next translation chunk from the session.
    ///
    /// Returns `None` when the session is closed or the final result is received.
    pub async fn recv(&self) -> Option<Result<TranslationChunk>> {
        loop {
            if self.closed.load(std::sync::atomic::Ordering::Relaxed) {
                return None;
            }

            let msg = self.read.lock().await.next().await?;
            
            match msg {
                Ok(WsMessage::Text(text)) => {
                    match serde_json::from_str::<TranslationResponse>(&text) {
                        Ok(resp) => {
                            if resp.code != 0 && resp.code != 1000 {
                                return Some(Err(Error::api_with_req_id(
                                    resp.code,
                                    resp.message,
                                    String::new(),
                                    200,
                                )));
                            }

                            let mut chunk = TranslationChunk {
                                source_text: resp.data.source_text.clone(),
                                target_text: resp.data.target_text.clone(),
                                is_definite: resp.data.is_final,
                                is_final: resp.data.is_final,
                                sequence: resp.data.sequence,
                                audio: None,
                            };

                            // Handle ASR type
                            if resp.msg_type == "asr" {
                                chunk.source_text = resp.data.text.clone();
                            }

                            // Decode audio if present
                            if let Some(ref audio_b64) = resp.data.audio {
                                if let Ok(audio_data) = BASE64.decode(audio_b64) {
                                    chunk.audio = Some(audio_data);
                                }
                            }

                            if chunk.is_final {
                                self.closed.store(true, std::sync::atomic::Ordering::Relaxed);
                            }

                            return Some(Ok(chunk));
                        }
                        Err(_) => {
                            // Skip non-JSON messages, continue loop
                            continue;
                        }
                    }
                }
                Ok(WsMessage::Close(_)) => {
                    self.closed.store(true, std::sync::atomic::Ordering::Relaxed);
                    return None;
                }
                Ok(_) => {
                    // Skip other message types, continue loop
                    continue;
                }
                Err(e) => return Some(Err(Error::WebSocket(e))),
            }
        }
    }

    /// Closes the translation session.
    pub async fn close(&self) -> Result<()> {
        if self.closed.swap(true, std::sync::atomic::Ordering::Relaxed) {
            return Ok(());
        }
        self.write.lock().await.close().await.map_err(|e| Error::WebSocket(e))?;
        Ok(())
    }
}

// ================== Internal Response Types ==================

/// Translation response from WebSocket.
#[derive(Debug, Deserialize)]
struct TranslationResponse {
    #[serde(default, rename = "type")]
    msg_type: String,
    #[serde(default)]
    data: TranslationData,
    #[serde(default)]
    code: i32,
    #[serde(default)]
    message: String,
}

/// Translation data.
#[derive(Debug, Deserialize, Default)]
struct TranslationData {
    #[serde(default)]
    text: String,
    #[serde(default)]
    source_text: String,
    #[serde(default)]
    target_text: String,
    #[serde(default)]
    is_final: bool,
    #[serde(default)]
    audio: Option<String>,
    #[serde(default)]
    sequence: i32,
}
