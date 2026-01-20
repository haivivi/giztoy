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
    types::{AudioEncoding, Language, SampleRate, SubtitleSegment, TtsTextType},
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
            let mut buffer = String::new();

            while let Some(result) = stream.next().await {
                let bytes = result?;
                buffer.push_str(&String::from_utf8_lossy(&bytes));

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
