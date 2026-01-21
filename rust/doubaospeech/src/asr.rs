//! ASR (Automatic Speech Recognition) service for Doubao Speech API.

use std::sync::Arc;

use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use futures::stream::{SplitSink, SplitStream};
use futures::{SinkExt, StreamExt};
use serde::{Deserialize, Serialize};
use tokio::sync::Mutex;
use tokio_tungstenite::tungstenite::Message as WsMessage;
use tokio_tungstenite::{connect_async, MaybeTlsStream, WebSocketStream};
use uuid::Uuid;

use crate::{
    client::generate_req_id,
    error::{status_code, Error, Result},
    http::HttpClient,
    types::{AudioFormat, AudioInfo, Language, SampleRate},
};

/// ASR service provides automatic speech recognition functionality.
pub struct AsrService {
    http: Arc<HttpClient>,
}

impl AsrService {
    /// Creates a new ASR service.
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Performs one-sentence recognition (ASR 1.0).
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_doubaospeech::{Client, OneSentenceRequest, AudioFormat};
    ///
    /// let client = Client::builder("app-id").api_key("api-key").build()?;
    /// let audio_data = std::fs::read("audio.wav")?;
    /// let response = client.asr().recognize_one_sentence(&OneSentenceRequest {
    ///     audio: Some(audio_data),
    ///     format: AudioFormat::Wav,
    ///     ..Default::default()
    /// }).await?;
    /// println!("Text: {}", response.text);
    /// ```
    pub async fn recognize_one_sentence(&self, req: &OneSentenceRequest) -> Result<AsrResult> {
        let asr_req = self.build_request(req)?;

        let url = "/api/v1/asr";
        let response: AsrApiResponse = self.http.request("POST", url, Some(&asr_req)).await?;

        // Check for ASR success code (1000) or generic success (0)
        if response.code != status_code::ASR_SUCCESS && response.code != 0 {
            return Err(Error::api_with_req_id(
                response.code,
                response.message,
                response.reqid,
                200,
            ));
        }

        Ok(AsrResult {
            text: response.result.text,
            duration: response.result.duration,
            utterances: vec![],
        })
    }

    /// Opens a streaming ASR session (ASR 2.0).
    ///
    /// Returns an `AsrStreamSession` for sending audio and receiving recognition results.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_doubaospeech::{Client, StreamAsrConfig, AudioFormat, SampleRate};
    ///
    /// let client = Client::builder("app-id").bearer_token("token").build()?;
    /// let config = StreamAsrConfig {
    ///     format: AudioFormat::Pcm,
    ///     sample_rate: SampleRate::Rate16000,
    ///     bits: 16,
    ///     channel: 1,
    ///     ..Default::default()
    /// };
    /// let mut session = client.asr().open_stream_session(&config).await?;
    ///
    /// // Send audio data
    /// session.send_audio(&audio_data, false).await?;
    /// session.send_audio(&last_chunk, true).await?;
    ///
    /// // Receive results
    /// while let Some(result) = session.recv().await {
    ///     match result {
    ///         Ok(chunk) => println!("Text: {}", chunk.text),
    ///         Err(e) => eprintln!("Error: {}", e),
    ///     }
    /// }
    /// ```
    pub async fn open_stream_session(&self, config: &StreamAsrConfig) -> Result<AsrStreamSession> {
        let auth = self.http.auth();
        let ws_url = format!(
            "{}/api/v2/asr?{}",
            self.http.ws_url(),
            self.http.ws_auth_params()
        );

        let (ws_stream, _) = connect_async(&ws_url)
            .await
            .map_err(|e| Error::WebSocket(e))?;

        let (write, read) = ws_stream.split();
        let req_id = generate_req_id();

        let session = AsrStreamSession {
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

        let start_req = serde_json::json!({
            "app": {
                "appid": auth.app_id,
                "cluster": cluster,
            },
            "user": {
                "uid": auth.user_id,
            },
            "audio": {
                "format": config.format.as_str(),
                "sample_rate": config.sample_rate.as_i32(),
                "channel": config.channel,
                "bits": config.bits,
            },
            "request": {
                "reqid": req_id,
                "workflow": "audio_in,resample,partition,vad,fe,decode,itn,nlu_punctuate",
                "show_utterances": config.show_utterances.unwrap_or(true),
                "result_type": "full",
                "language": config.language.as_ref().map(|l| l.as_str()).unwrap_or("zh-CN"),
            }
        });

        let msg = WsMessage::Text(serde_json::to_string(&start_req)?.into());
        session.write.lock().await.send(msg).await.map_err(|e| Error::WebSocket(e))?;

        Ok(session)
    }

    /// Submits a file for asynchronous ASR recognition (ASR 2.0).
    ///
    /// Returns a task ID that can be used to query the recognition status.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_doubaospeech::{Client, FileAsrRequest};
    ///
    /// let client = Client::builder("app-id").api_key("api-key").build()?;
    /// let task_id = client.asr().recognize_file(&FileAsrRequest {
    ///     audio_url: "https://example.com/audio.mp3".to_string(),
    ///     ..Default::default()
    /// }).await?;
    /// println!("Task ID: {}", task_id);
    /// ```
    pub async fn recognize_file(&self, req: &FileAsrRequest) -> Result<String> {
        let submit_req = FileAsrSubmitRequest {
            app_id: self.http.auth().app_id.clone(),
            req_id: generate_req_id(),
            audio_url: req.audio_url.clone(),
            language: req.language.as_ref().map(|l| l.as_str().to_string()),
            enable_itn: req.enable_itn,
            enable_punc: req.enable_punc,
            callback_url: req.callback_url.clone(),
        };

        let response: FileAsrSubmitResponse = self
            .http
            .request("POST", "/api/v1/asr/submit", Some(&submit_req))
            .await?;

        if response.code != 0 {
            return Err(Error::api_with_req_id(
                response.code,
                response.message,
                response.req_id,
                200,
            ));
        }

        Ok(response.task_id)
    }

    /// Queries the status of an asynchronous ASR task.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_doubaospeech::Client;
    ///
    /// let client = Client::builder("app-id").api_key("api-key").build()?;
    /// let result = client.asr().query_task("task-id").await?;
    /// println!("Status: {:?}", result);
    /// ```
    pub async fn query_task(&self, task_id: &str) -> Result<FileAsrTaskResult> {
        let query_req = FileAsrQueryRequest {
            app_id: self.http.auth().app_id.clone(),
            task_id: task_id.to_string(),
        };

        let response: FileAsrQueryResponse = self
            .http
            .request("POST", "/api/v1/asr/query", Some(&query_req))
            .await?;

        if response.code != 0 {
            return Err(Error::api_with_req_id(
                response.code,
                response.message,
                response.req_id.unwrap_or_default(),
                200,
            ));
        }

        Ok(FileAsrTaskResult {
            task_id: response.task_id,
            status: response.status,
            text: response.text,
            utterances: response.utterances.unwrap_or_default().into_iter().map(|u| {
                Utterance {
                    text: u.text,
                    start_time: u.start_time,
                    end_time: u.end_time,
                    definite: true,
                    words: u.words.unwrap_or_default().into_iter().map(|w| {
                        Word {
                            text: w.text,
                            start_time: w.start_time,
                            end_time: w.end_time,
                        }
                    }).collect(),
                }
            }).collect(),
        })
    }

    /// Builds the ASR request payload.
    fn build_request(&self, req: &OneSentenceRequest) -> Result<AsrRequestPayload> {
        let auth = self.http.auth();
        let cluster = auth
            .cluster
            .clone()
            .unwrap_or_else(|| "volcengine_streaming_common".to_string());

        // Encode audio data
        let audio_data = if let Some(ref audio) = req.audio {
            Some(BASE64.encode(audio))
        } else {
            None
        };

        Ok(AsrRequestPayload {
            app: AppInfo {
                appid: auth.app_id.clone(),
                cluster,
            },
            user: UserInfo {
                uid: auth.user_id.clone(),
            },
            audio: AsrAudioParams {
                format: req.format.as_str().to_string(),
                sample_rate: req.sample_rate.map(|r| r.as_i32()),
                channel: req.channel,
                bits: req.bits,
                url: req.audio_url.clone(),
                data: audio_data,
            },
            request: AsrRequestParams {
                reqid: Uuid::new_v4().to_string(),
                language: req.language.as_ref().map(|l| l.as_str().to_string()),
                enable_itn: req.enable_itn,
                enable_punc: req.enable_punc,
                enable_ddc: req.enable_ddc,
                show_utterances: None,
                result_type: None,
                workflow: None,
                command: None,
            },
        })
    }
}

// ================== Request Types ==================

/// One-sentence ASR request.
#[derive(Debug, Clone, Default)]
pub struct OneSentenceRequest {
    /// Audio data (binary).
    pub audio: Option<Vec<u8>>,
    /// Audio URL (alternative to audio data).
    pub audio_url: Option<String>,
    /// Audio format.
    pub format: AudioFormat,
    /// Sample rate.
    pub sample_rate: Option<SampleRate>,
    /// Number of channels.
    pub channel: Option<i32>,
    /// Bit depth.
    pub bits: Option<i32>,
    /// Language.
    pub language: Option<Language>,
    /// Enable ITN (Inverse Text Normalization).
    pub enable_itn: Option<bool>,
    /// Enable punctuation.
    pub enable_punc: Option<bool>,
    /// Enable DDC (Disfluency Detection and Correction).
    pub enable_ddc: Option<bool>,
}

/// Streaming ASR configuration.
#[derive(Debug, Clone, Default)]
pub struct StreamAsrConfig {
    /// Audio format.
    pub format: AudioFormat,
    /// Sample rate.
    pub sample_rate: SampleRate,
    /// Bit depth.
    pub bits: i32,
    /// Number of channels.
    pub channel: i32,
    /// Language.
    pub language: Option<Language>,
    /// Model name.
    pub model_name: Option<String>,
    /// Enable ITN.
    pub enable_itn: Option<bool>,
    /// Enable punctuation.
    pub enable_punc: Option<bool>,
    /// Enable DDC.
    pub enable_ddc: Option<bool>,
    /// Show utterances.
    pub show_utterances: Option<bool>,
    /// Enable non-streaming mode.
    pub enable_nonstream: Option<bool>,
}

/// File ASR request.
#[derive(Debug, Clone, Default)]
pub struct FileAsrRequest {
    /// Audio URL.
    pub audio_url: String,
    /// Audio format.
    pub format: Option<AudioFormat>,
    /// Language.
    pub language: Option<Language>,
    /// Enable ITN.
    pub enable_itn: Option<bool>,
    /// Enable punctuation.
    pub enable_punc: Option<bool>,
    /// Enable DDC.
    pub enable_ddc: Option<bool>,
    /// Enable timestamp.
    pub enable_timestamp: Option<bool>,
    /// Callback URL.
    pub callback_url: Option<String>,
}

// ================== Response Types ==================

/// ASR result.
#[derive(Debug, Clone, Default)]
pub struct AsrResult {
    /// Recognized text.
    pub text: String,
    /// Duration in milliseconds.
    pub duration: i32,
    /// Utterances.
    pub utterances: Vec<Utterance>,
}

/// Utterance segment.
#[derive(Debug, Clone, Default, Serialize)]
pub struct Utterance {
    /// Text.
    pub text: String,
    /// Start time in milliseconds.
    pub start_time: i32,
    /// End time in milliseconds.
    pub end_time: i32,
    /// Whether this is a definite result.
    pub definite: bool,
    /// Words.
    pub words: Vec<Word>,
}

/// Word information.
#[derive(Debug, Clone, Default, Serialize)]
pub struct Word {
    /// Text.
    pub text: String,
    /// Start time in milliseconds.
    pub start_time: i32,
    /// End time in milliseconds.
    pub end_time: i32,
}

/// Streaming ASR chunk.
#[derive(Debug, Clone, Default)]
pub struct AsrChunk {
    /// Recognized text.
    pub text: String,
    /// Whether this is a definite result.
    pub is_definite: bool,
    /// Whether this is the final result.
    pub is_final: bool,
    /// Utterances.
    pub utterances: Vec<Utterance>,
    /// Audio info.
    pub audio_info: Option<AudioInfo>,
    /// Sequence number.
    pub sequence: i32,
}

// ================== Internal Request/Response Types ==================

/// Application information.
#[derive(Debug, Serialize)]
struct AppInfo {
    appid: String,
    cluster: String,
}

/// User information.
#[derive(Debug, Serialize)]
struct UserInfo {
    uid: String,
}

/// ASR audio parameters.
#[derive(Debug, Serialize)]
struct AsrAudioParams {
    format: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    sample_rate: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    channel: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    bits: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    url: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    data: Option<String>,
}

/// ASR request parameters.
#[derive(Debug, Serialize)]
struct AsrRequestParams {
    reqid: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    language: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    enable_itn: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    enable_punc: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    enable_ddc: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    show_utterances: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    result_type: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    workflow: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    command: Option<String>,
}

/// ASR request payload.
#[derive(Debug, Serialize)]
struct AsrRequestPayload {
    app: AppInfo,
    user: UserInfo,
    audio: AsrAudioParams,
    request: AsrRequestParams,
}

/// ASR API response.
#[derive(Debug, Deserialize)]
struct AsrApiResponse {
    #[serde(default)]
    reqid: String,
    #[serde(default)]
    code: i32,
    #[serde(default)]
    message: String,
    #[serde(default)]
    result: AsrResultData,
}

/// ASR result data.
#[derive(Debug, Deserialize, Default)]
struct AsrResultData {
    #[serde(default)]
    text: String,
    #[serde(default)]
    duration: i32,
}

// ================== Streaming ASR Session ==================

type WsStream = WebSocketStream<MaybeTlsStream<tokio::net::TcpStream>>;

/// Streaming ASR session for real-time speech recognition.
pub struct AsrStreamSession {
    write: Arc<Mutex<SplitSink<WsStream, WsMessage>>>,
    read: Arc<Mutex<SplitStream<WsStream>>>,
    req_id: String,
    closed: Arc<std::sync::atomic::AtomicBool>,
}

impl AsrStreamSession {
    /// Sends audio data to the ASR session.
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

    /// Receives the next ASR chunk from the session.
    ///
    /// Returns `None` when the session is closed or the final result is received.
    pub async fn recv(&self) -> Option<Result<AsrChunk>> {
        loop {
            if self.closed.load(std::sync::atomic::Ordering::Relaxed) {
                return None;
            }

            let msg = self.read.lock().await.next().await?;
            
            match msg {
                Ok(WsMessage::Text(text)) => {
                    match serde_json::from_str::<StreamAsrResponse>(&text) {
                        Ok(resp) => {
                            if resp.code != status_code::ASR_SUCCESS && resp.code != 0 {
                                return Some(Err(Error::api_with_req_id(
                                    resp.code,
                                    resp.message,
                                    resp.reqid,
                                    200,
                                )));
                            }

                            let utterances = resp.result.utterances.unwrap_or_default().into_iter().map(|u| {
                                Utterance {
                                    text: u.text,
                                    start_time: u.start_time,
                                    end_time: u.end_time,
                                    definite: resp.result.is_final,
                                    words: u.words.unwrap_or_default().into_iter().map(|w| {
                                        Word {
                                            text: w.text,
                                            start_time: w.start_time,
                                            end_time: w.end_time,
                                        }
                                    }).collect(),
                                }
                            }).collect();

                            let chunk = AsrChunk {
                                text: resp.result.text,
                                is_definite: resp.result.is_final,
                                is_final: resp.result.is_final,
                                utterances,
                                audio_info: None,
                                sequence: 0,
                            };

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

    /// Closes the ASR session.
    pub async fn close(&self) -> Result<()> {
        if self.closed.swap(true, std::sync::atomic::Ordering::Relaxed) {
            return Ok(());
        }
        self.write.lock().await.close().await.map_err(|e| Error::WebSocket(e))?;
        Ok(())
    }
}

// ================== File ASR Types ==================

/// File ASR submit request.
#[derive(Debug, Serialize)]
struct FileAsrSubmitRequest {
    #[serde(rename = "appid")]
    app_id: String,
    #[serde(rename = "reqid")]
    req_id: String,
    audio_url: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    language: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    enable_itn: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    enable_punc: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    callback_url: Option<String>,
}

/// File ASR submit response.
#[derive(Debug, Deserialize)]
struct FileAsrSubmitResponse {
    #[serde(default)]
    code: i32,
    #[serde(default)]
    message: String,
    #[serde(default, rename = "reqid")]
    req_id: String,
    #[serde(default, rename = "id")]
    task_id: String,
}

/// File ASR query request.
#[derive(Debug, Serialize)]
struct FileAsrQueryRequest {
    #[serde(rename = "appid")]
    app_id: String,
    #[serde(rename = "id")]
    task_id: String,
}

/// File ASR query response.
#[derive(Debug, Deserialize)]
struct FileAsrQueryResponse {
    #[serde(default)]
    code: i32,
    #[serde(default)]
    message: String,
    #[serde(default, rename = "reqid")]
    req_id: Option<String>,
    #[serde(default, rename = "id")]
    task_id: String,
    #[serde(default)]
    status: String,
    #[serde(default)]
    text: Option<String>,
    #[serde(default)]
    utterances: Option<Vec<FileAsrUtterance>>,
}

/// File ASR utterance.
#[derive(Debug, Deserialize)]
struct FileAsrUtterance {
    #[serde(default)]
    text: String,
    #[serde(default)]
    start_time: i32,
    #[serde(default)]
    end_time: i32,
    #[serde(default)]
    words: Option<Vec<FileAsrWord>>,
}

/// File ASR word.
#[derive(Debug, Deserialize)]
struct FileAsrWord {
    #[serde(default)]
    text: String,
    #[serde(default)]
    start_time: i32,
    #[serde(default)]
    end_time: i32,
}

/// File ASR task result.
#[derive(Debug, Clone, Default, Serialize)]
pub struct FileAsrTaskResult {
    /// Task ID.
    pub task_id: String,
    /// Task status (e.g., "running", "success", "failed").
    pub status: String,
    /// Recognized text (when status is "success").
    pub text: Option<String>,
    /// Utterances with timestamps.
    pub utterances: Vec<Utterance>,
}

// ================== Streaming ASR Response Types ==================

/// Streaming ASR response.
#[derive(Debug, Deserialize)]
struct StreamAsrResponse {
    #[serde(default)]
    reqid: String,
    #[serde(default)]
    code: i32,
    #[serde(default)]
    message: String,
    #[serde(default)]
    result: StreamAsrResult,
}

/// Streaming ASR result.
#[derive(Debug, Deserialize, Default)]
struct StreamAsrResult {
    #[serde(default)]
    text: String,
    #[serde(default)]
    is_final: bool,
    #[serde(default)]
    utterances: Option<Vec<StreamAsrUtterance>>,
}

/// Streaming ASR utterance.
#[derive(Debug, Deserialize)]
struct StreamAsrUtterance {
    #[serde(default)]
    text: String,
    #[serde(default)]
    start_time: i32,
    #[serde(default)]
    end_time: i32,
    #[serde(default)]
    words: Option<Vec<StreamAsrWord>>,
}

/// Streaming ASR word.
#[derive(Debug, Deserialize)]
struct StreamAsrWord {
    #[serde(default)]
    text: String,
    #[serde(default)]
    start_time: i32,
    #[serde(default)]
    end_time: i32,
}
