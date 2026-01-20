//! Speech synthesis service.

use std::sync::Arc;

use async_stream::try_stream;
use futures::Stream;
use serde::{Deserialize, Serialize};

use super::{
    error::{Error, Result},
    http::{decode_hex_audio, HttpClient, SseReader},
    task::{Task, TaskType},
    types::{AudioFormat, AudioInfo, BaseResp, OutputFormat, Subtitle, SubtitleSegment},
};

/// Speech synthesis service.
pub struct SpeechService {
    http: Arc<HttpClient>,
}

impl SpeechService {
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Performs synchronous speech synthesis.
    ///
    /// The returned audio data is automatically decoded from hex format.
    /// Maximum text length is 10,000 characters.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let request = SpeechRequest {
    ///     model: "speech-2.6-hd".to_string(),
    ///     text: "Hello, world!".to_string(),
    ///     voice_setting: Some(VoiceSetting {
    ///         voice_id: "female-shaonv".to_string(),
    ///         ..Default::default()
    ///     }),
    ///     ..Default::default()
    /// };
    ///
    /// let response = client.speech().synthesize(&request).await?;
    /// ```
    pub async fn synthesize(&self, request: &SpeechRequest) -> Result<SpeechResponse> {
        let api_resp: SpeechApiResponse = self
            .http
            .request("POST", "/v1/t2a_v2", Some(request))
            .await?;

        let mut response = SpeechResponse {
            audio: Vec::new(),
            audio_url: api_resp.data.audio_url,
            extra_info: api_resp.extra_info,
            trace_id: api_resp.trace_id,
        };

        // Decode hex audio if present
        if !api_resp.data.audio.is_empty() {
            response.audio = decode_hex_audio(&api_resp.data.audio)?;
        }

        Ok(response)
    }

    /// Performs streaming speech synthesis.
    ///
    /// Returns a stream that yields audio chunks.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use futures::StreamExt;
    ///
    /// let mut audio_data = Vec::new();
    /// let mut stream = client.speech().synthesize_stream(&request).await?;
    ///
    /// while let Some(chunk) = stream.next().await {
    ///     let chunk = chunk?;
    ///     audio_data.extend_from_slice(&chunk.audio);
    /// }
    /// ```
    pub async fn synthesize_stream(
        &self,
        request: &SpeechRequest,
    ) -> Result<impl Stream<Item = Result<SpeechChunk>>> {
        // Add stream flag to request
        let stream_request = SpeechStreamRequest {
            inner: request.clone(),
            stream: true,
        };

        let byte_stream = self
            .http
            .request_stream("POST", "/v1/t2a_v2", Some(stream_request))
            .await?;

        let mut reader = SseReader::new(Box::pin(byte_stream));

        Ok(try_stream! {
            loop {
                match reader.read_event().await? {
                    Some(data) => {
                        let stream_resp: SpeechStreamResponse = serde_json::from_slice(&data)?;

                        // Check for API error
                        if let Some(base_resp) = &stream_resp.base_resp {
                            if base_resp.is_error() {
                                Err(Error::api(
                                    base_resp.status_code,
                                    &base_resp.status_msg,
                                    200,
                                ))?;
                            }
                        }

                        let mut chunk = SpeechChunk {
                            audio: Vec::new(),
                            status: stream_resp.data.status,
                            subtitle: stream_resp.subtitle,
                            extra_info: stream_resp.extra_info,
                            trace_id: stream_resp.trace_id,
                        };

                        // Decode hex audio if present
                        if !stream_resp.data.audio.is_empty() {
                            chunk.audio = decode_hex_audio(&stream_resp.data.audio)?;
                        }

                        yield chunk;
                    }
                    None => break,
                }
            }
        })
    }

    /// Creates an async speech synthesis task.
    ///
    /// For long texts up to 1,000,000 characters.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let request = AsyncSpeechRequest {
    ///     model: "speech-2.6-hd".to_string(),
    ///     text: Some(long_text),
    ///     ..Default::default()
    /// };
    ///
    /// let task = client.speech().create_async_task(&request).await?;
    /// let result = task.wait().await?;
    /// ```
    pub async fn create_async_task(&self, request: &AsyncSpeechRequest) -> Result<Task> {
        #[derive(Deserialize)]
        struct Response {
            task_id: String,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        let resp: Response = self
            .http
            .request("POST", "/v1/t2a_async", Some(request))
            .await?;

        Ok(Task::new(resp.task_id, TaskType::SpeechAsync, self.http.clone()))
    }
}

// ==================== Request/Response Types ====================

/// Request for speech synthesis.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SpeechRequest {
    /// Model version.
    pub model: String,

    /// Text to synthesize (max 10,000 characters).
    pub text: String,

    /// Voice configuration.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub voice_setting: Option<VoiceSetting>,

    /// Audio configuration.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio_setting: Option<AudioSetting>,

    /// Pronunciation rules.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pronunciation_dict: Option<PronunciationDict>,

    /// Language boost for specific language pronunciation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub language_boost: Option<String>,

    /// Enable subtitle generation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub subtitle_enable: Option<bool>,

    /// Output format: hex or url.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_format: Option<OutputFormat>,
}

/// Request for async speech synthesis.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct AsyncSpeechRequest {
    /// Model version.
    pub model: String,

    /// Text to synthesize (max 1,000,000 characters).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub text: Option<String>,

    /// File ID of a text file (alternative to text).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub file_id: Option<String>,

    /// Voice configuration.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub voice_setting: Option<VoiceSetting>,

    /// Audio configuration.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio_setting: Option<AudioSetting>,

    /// Pronunciation rules.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pronunciation_dict: Option<PronunciationDict>,

    /// Language boost for specific language pronunciation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub language_boost: Option<String>,

    /// Enable subtitle generation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub subtitle_enable: Option<bool>,
}

/// Voice configuration.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct VoiceSetting {
    /// Voice identifier.
    pub voice_id: String,

    /// Speech speed (0.5-2.0, default 1.0).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub speed: Option<f64>,

    /// Volume (0-10, default 1.0).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub vol: Option<f64>,

    /// Pitch adjustment (-12 to 12, default 0).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pitch: Option<i32>,

    /// Emotion: happy, sad, angry, fearful, disgusted, surprised, neutral.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub emotion: Option<String>,
}

/// Audio configuration.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct AudioSetting {
    /// Sample rate: 8000, 16000, 22050, 24000, 32000, 44100.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sample_rate: Option<i32>,

    /// Bitrate: 32000, 64000, 128000, 256000.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub bitrate: Option<i32>,

    /// Audio format: mp3, pcm, flac, wav.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub format: Option<AudioFormat>,

    /// Number of channels: 1 or 2.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub channel: Option<i32>,
}

/// Pronunciation rules.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PronunciationDict {
    /// List of pronunciation rules, e.g. ["处理/(chu3)(li3)", "危险/dangerous"].
    pub tone: Vec<String>,
}

/// Response from speech synthesis.
#[derive(Debug, Clone, Default)]
pub struct SpeechResponse {
    /// Decoded audio data.
    pub audio: Vec<u8>,

    /// Audio URL (when output_format is "url").
    pub audio_url: Option<String>,

    /// Audio metadata.
    pub extra_info: Option<AudioInfo>,

    /// Request trace ID.
    pub trace_id: String,
}

/// A chunk of streaming speech data.
#[derive(Debug, Clone, Default)]
pub struct SpeechChunk {
    /// Decoded audio data.
    pub audio: Vec<u8>,

    /// Status code: 1=generating, 2=complete.
    pub status: i32,

    /// Subtitle segment (if enabled).
    pub subtitle: Option<SubtitleSegment>,

    /// Audio metadata (usually in last chunk).
    pub extra_info: Option<AudioInfo>,

    /// Request trace ID (usually in last chunk).
    pub trace_id: Option<String>,
}

/// Result of an async speech task.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SpeechAsyncResult {
    /// Generated audio file ID.
    pub file_id: String,

    /// Audio metadata.
    #[serde(rename = "extra_info")]
    pub audio_info: Option<AudioInfo>,

    /// Subtitle information (if enabled).
    pub subtitle: Option<Subtitle>,
}

// ==================== Internal Types ====================

#[derive(Serialize)]
struct SpeechStreamRequest {
    #[serde(flatten)]
    inner: SpeechRequest,
    stream: bool,
}

#[derive(Deserialize)]
struct SpeechApiResponse {
    data: SpeechData,
    extra_info: Option<AudioInfo>,
    #[serde(default)]
    trace_id: String,
    #[serde(default)]
    base_resp: Option<BaseResp>,
}

#[derive(Deserialize)]
struct SpeechData {
    #[serde(default)]
    audio: String,
    #[serde(default)]
    audio_url: Option<String>,
    #[serde(default)]
    status: i32,
}

#[derive(Deserialize)]
struct SpeechStreamResponse {
    data: SpeechData,
    extra_info: Option<AudioInfo>,
    subtitle: Option<SubtitleSegment>,
    trace_id: Option<String>,
    base_resp: Option<BaseResp>,
}

// ==================== HasModel Implementations ====================

use crate::{HasModel, MODEL_SPEECH_26_HD};

impl HasModel for SpeechRequest {
    fn model(&self) -> &str {
        &self.model
    }

    fn set_model(&mut self, model: impl Into<String>) {
        self.model = model.into();
    }

    fn default_model() -> &'static str {
        MODEL_SPEECH_26_HD
    }
}

impl HasModel for AsyncSpeechRequest {
    fn model(&self) -> &str {
        &self.model
    }

    fn set_model(&mut self, model: impl Into<String>) {
        self.model = model.into();
    }

    fn default_model() -> &'static str {
        MODEL_SPEECH_26_HD
    }
}
