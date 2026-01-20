//! Voice Clone service for Doubao Speech API.

use std::sync::Arc;

use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use serde::{Deserialize, Serialize};

use crate::{
    error::{Error, Result},
    http::HttpClient,
    types::{Language, VoiceCloneModelType, VoiceCloneStatusType},
};

/// Voice Clone service provides voice cloning functionality.
///
/// API Documentation: https://www.volcengine.com/docs/6561/1305191
pub struct VoiceCloneService {
    http: Arc<HttpClient>,
}

impl VoiceCloneService {
    /// Creates a new Voice Clone service.
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Trains a custom voice using audio data.
    ///
    /// The speaker_id should follow the format: S_xxxxxxxxx (e.g., S_TR0rbVuI1)
    ///
    /// Audio requirements:
    /// - Duration: 10-60 seconds recommended
    /// - Formats: wav, mp3, ogg, pcm
    /// - Sample rate: 16kHz or 24kHz
    ///
    /// After training completes, use the speaker_id in TTS with:
    /// - Cluster: volcano_icl (for ICL 1.0) or volcano_mega (for DiT)
    /// - Voice type: your speaker_id
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_doubaospeech::{Client, VoiceCloneTrainRequest};
    ///
    /// let client = Client::builder("app-id").api_key("api-key").build()?;
    /// let audio_data = std::fs::read("voice_sample.wav")?;
    /// let result = client.voice_clone().train(&VoiceCloneTrainRequest {
    ///     speaker_id: "S_MyVoice001".to_string(),
    ///     audio_data: Some(audio_data),
    ///     ..Default::default()
    /// }).await?;
    /// ```
    pub async fn train(&self, req: &VoiceCloneTrainRequest) -> Result<VoiceCloneResult> {
        let auth = self.http.auth();

        // Audio format - infer from data or use wav as default
        let audio_format = if let Some(ref data) = req.audio_data {
            detect_audio_format(data)
        } else {
            "wav".to_string()
        };

        // Model type (1=ICL1.0, 2=DiT标准, 3=DiT还原, 4=ICL2.0)
        let model_type = match req.model_type {
            VoiceCloneModelType::Standard => 1,
            VoiceCloneModelType::Pro => 3, // DiT 还原版
        };

        // Build request body
        let mut request_body = VoiceCloneUploadRequest {
            appid: auth.app_id.clone(),
            speaker_id: req.speaker_id.clone(),
            audio_format,
            model_type,
            language: None,
            text: req.text.clone(),
            audio_data: None,
        };

        // Set language
        if let Some(ref lang) = req.language {
            let lang_code = match lang {
                Language::EnUs | Language::EnGb => 1,
                Language::JaJp => 2,
                _ => 0, // zh
            };
            request_body.language = Some(lang_code);
        }

        // Set audio data as base64
        if let Some(ref data) = req.audio_data {
            request_body.audio_data = Some(BASE64.encode(data));
        }

        let url = "/api/v1/mega_tts/audio/upload";
        let response: VoiceCloneUploadResponse = self.http.request("POST", url, Some(&request_body)).await?;

        if response.base_resp.status_code != 0 {
            return Err(Error::api(
                response.base_resp.status_code,
                response.base_resp.status_message,
                200,
            ));
        }

        let speaker_id = if response.speaker_id.is_empty() {
            req.speaker_id.clone()
        } else {
            response.speaker_id
        };

        Ok(VoiceCloneResult {
            speaker_id,
            status: VoiceCloneStatusType::Processing,
            message: None,
        })
    }

    /// Queries training status.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let status = client.voice_clone().get_status("S_MyVoice001").await?;
    /// println!("Status: {:?}", status.status);
    /// ```
    pub async fn get_status(&self, speaker_id: &str) -> Result<VoiceCloneStatus> {
        let auth = self.http.auth();
        let url = format!(
            "/api/v1/mega_tts/status?appid={}&speaker_id={}",
            auth.app_id, speaker_id
        );

        let response: VoiceCloneStatusResponse = self.http.request("GET", &url, None::<&()>).await?;

        if response.base_resp.status_code != 0 {
            return Err(Error::api(
                response.base_resp.status_code,
                response.base_resp.status_message,
                200,
            ));
        }

        // Convert status
        let status = match response.status.as_str() {
            "Processing" => VoiceCloneStatusType::Processing,
            "Success" => VoiceCloneStatusType::Success,
            "Failed" => VoiceCloneStatusType::Failed,
            _ => VoiceCloneStatusType::Pending,
        };

        Ok(VoiceCloneStatus {
            speaker_id: response.speaker_id,
            status,
            progress: None,
            message: None,
            demo_audio: response.demo_audio,
            created_at: None,
            updated_at: None,
        })
    }
}

// ================== Request Types ==================

/// Voice clone training request.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct VoiceCloneTrainRequest {
    /// Speaker ID (format: S_xxxxxxxxx).
    #[serde(default)]
    pub speaker_id: String,
    /// Audio URLs (alternative to audio_data).
    #[serde(default)]
    pub audio_urls: Vec<String>,
    /// Audio data (binary).
    #[serde(skip)]
    pub audio_data: Option<Vec<u8>>,
    /// Text for the audio (optional).
    #[serde(default)]
    pub text: Option<String>,
    /// Language.
    #[serde(default)]
    pub language: Option<Language>,
    /// Model type.
    #[serde(default)]
    pub model_type: VoiceCloneModelType,
}

// ================== Response Types ==================

/// Voice clone result.
#[derive(Debug, Clone, Default, Serialize)]
pub struct VoiceCloneResult {
    /// Speaker ID.
    pub speaker_id: String,
    /// Status.
    pub status: VoiceCloneStatusType,
    /// Message.
    pub message: Option<String>,
}

/// Voice clone training status.
#[derive(Debug, Clone, Default, Serialize)]
pub struct VoiceCloneStatus {
    /// Speaker ID.
    pub speaker_id: String,
    /// Status.
    pub status: VoiceCloneStatusType,
    /// Progress (0-100).
    pub progress: Option<i32>,
    /// Message.
    pub message: Option<String>,
    /// Demo audio URL.
    pub demo_audio: Option<String>,
    /// Created timestamp.
    pub created_at: Option<i64>,
    /// Updated timestamp.
    pub updated_at: Option<i64>,
}

/// Voice clone info.
#[derive(Debug, Clone, Default, Serialize)]
pub struct VoiceCloneInfo {
    /// Speaker ID.
    pub speaker_id: String,
    /// Status.
    pub status: VoiceCloneStatusType,
    /// Language.
    pub language: Option<Language>,
    /// Model type.
    pub model_type: VoiceCloneModelType,
    /// Created timestamp.
    pub created_at: Option<i64>,
}

// ================== Internal Request/Response Types ==================

/// Voice clone upload request.
#[derive(Debug, Serialize)]
struct VoiceCloneUploadRequest {
    appid: String,
    speaker_id: String,
    audio_format: String,
    model_type: i32,
    #[serde(skip_serializing_if = "Option::is_none")]
    language: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    text: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    audio_data: Option<String>,
}

/// Base response.
#[derive(Debug, Deserialize, Default)]
struct BaseResp {
    #[serde(rename = "StatusCode", default)]
    status_code: i32,
    #[serde(rename = "StatusMessage", default)]
    status_message: String,
}

/// Voice clone upload response.
#[derive(Debug, Deserialize, Default)]
struct VoiceCloneUploadResponse {
    #[serde(rename = "BaseResp", default)]
    base_resp: BaseResp,
    #[serde(default)]
    speaker_id: String,
}

/// Voice clone status response.
#[derive(Debug, Deserialize, Default)]
struct VoiceCloneStatusResponse {
    #[serde(rename = "BaseResp", default)]
    base_resp: BaseResp,
    #[serde(default)]
    speaker_id: String,
    #[serde(default)]
    status: String,
    #[serde(default)]
    demo_audio: Option<String>,
}

// ================== Helper Functions ==================

/// Detects audio format from file header.
fn detect_audio_format(data: &[u8]) -> String {
    if data.len() < 12 {
        return "wav".to_string();
    }

    // Check for WAV (RIFF header)
    if &data[0..4] == b"RIFF" && &data[8..12] == b"WAVE" {
        return "wav".to_string();
    }

    // Check for MP3 (ID3 or sync word)
    if &data[0..3] == b"ID3" || (data[0] == 0xFF && (data[1] & 0xE0) == 0xE0) {
        return "mp3".to_string();
    }

    // Check for OGG (OggS)
    if &data[0..4] == b"OggS" {
        return "ogg".to_string();
    }

    "wav".to_string() // default
}
