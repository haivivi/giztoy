//! ASR (Automatic Speech Recognition) service for Doubao Speech API.

use std::sync::Arc;

use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::{
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
#[derive(Debug, Clone, Default)]
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
#[derive(Debug, Clone, Default)]
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
