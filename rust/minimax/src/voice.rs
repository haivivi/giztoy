//! Voice management service.

use std::sync::Arc;

use serde::{Deserialize, Serialize};

use super::{
    error::Result,
    http::{decode_hex_audio, HttpClient},
    types::{BaseResp, VoiceType},
};

/// Voice management service.
pub struct VoiceService {
    http: Arc<HttpClient>,
}

impl VoiceService {
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Lists available voices.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let voices = client.voice().list(None).await?;
    ///
    /// for voice in voices.all_voices() {
    ///     println!("{}: {} ({:?})", voice.voice_id, voice.voice_name, voice.voice_type);
    /// }
    /// ```
    pub async fn list(&self, voice_type: Option<VoiceType>) -> Result<VoiceListResponse> {
        #[derive(Serialize)]
        struct Request {
            voice_type: String,
        }

        #[derive(Deserialize)]
        struct ApiResponse {
            #[serde(default)]
            system_voice: Vec<VoiceInfo>,
            #[serde(default)]
            voice_cloning: Vec<VoiceInfo>,
            #[serde(default)]
            voice_generation: Vec<VoiceInfo>,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        let vt = match voice_type {
            Some(VoiceType::System) => "system",
            Some(VoiceType::VoiceCloning) => "voice_cloning",
            Some(VoiceType::VoiceGeneration) => "voice_generation",
            _ => "all",
        };

        let req = Request {
            voice_type: vt.to_string(),
        };

        let resp: ApiResponse = self.http.request("POST", "/v1/get_voice", Some(&req)).await?;

        Ok(VoiceListResponse {
            system_voice: resp.system_voice,
            voice_cloning: resp.voice_cloning,
            voice_generation: resp.voice_generation,
        })
    }

    /// Clones a voice from an audio file.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// // First upload the audio file
    /// let upload = client.file().upload(&audio_bytes, "voice.mp3", FilePurpose::VoiceClone).await?;
    ///
    /// let request = VoiceCloneRequest {
    ///     file_id: upload.file_id.parse()?,
    ///     voice_id: "my-custom-voice".to_string(),
    ///     model: Some("speech-2.6-hd".to_string()),
    ///     text: Some("Preview text for the cloned voice".to_string()),
    ///     ..Default::default()
    /// };
    ///
    /// let response = client.voice().clone(&request).await?;
    /// ```
    pub async fn clone(&self, request: &VoiceCloneRequest) -> Result<VoiceCloneResponse> {
        #[derive(Deserialize)]
        struct ApiResponse {
            voice_id: String,
            #[serde(default)]
            demo_audio: String,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        let resp: ApiResponse = self
            .http
            .request("POST", "/v1/voice_clone", Some(request))
            .await?;

        let demo_audio = if !resp.demo_audio.is_empty() {
            decode_hex_audio(&resp.demo_audio)?
        } else {
            Vec::new()
        };

        Ok(VoiceCloneResponse {
            voice_id: resp.voice_id,
            demo_audio,
        })
    }

    /// Designs a new voice from a text description.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let request = VoiceDesignRequest {
    ///     prompt: "A warm, friendly female voice with a slight British accent".to_string(),
    ///     preview_text: "Hello, how can I help you today?".to_string(),
    ///     voice_id: Some("my-designed-voice".to_string()),
    ///     ..Default::default()
    /// };
    ///
    /// let response = client.voice().design(&request).await?;
    /// std::fs::write("preview.mp3", &response.demo_audio)?;
    /// ```
    pub async fn design(&self, request: &VoiceDesignRequest) -> Result<VoiceDesignResponse> {
        #[derive(Deserialize)]
        struct ApiResponse {
            voice_id: String,
            #[serde(default)]
            demo_audio: String,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        let resp: ApiResponse = self
            .http
            .request("POST", "/v1/voice_generation", Some(request))
            .await?;

        let demo_audio = if !resp.demo_audio.is_empty() {
            decode_hex_audio(&resp.demo_audio)?
        } else {
            Vec::new()
        };

        Ok(VoiceDesignResponse {
            voice_id: resp.voice_id,
            demo_audio,
        })
    }

    /// Deletes a custom voice.
    pub async fn delete(&self, voice_id: &str) -> Result<()> {
        #[derive(Serialize)]
        struct Request<'a> {
            voice_id: &'a str,
        }

        #[derive(Deserialize)]
        struct Response {
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        let _: Response = self
            .http
            .request("POST", "/v1/voice/delete", Some(&Request { voice_id }))
            .await?;

        Ok(())
    }
}

// ==================== Request/Response Types ====================

/// Response containing available voices.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct VoiceListResponse {
    /// System predefined voices.
    #[serde(default)]
    pub system_voice: Vec<VoiceInfo>,

    /// Voices created via voice cloning.
    #[serde(default)]
    pub voice_cloning: Vec<VoiceInfo>,

    /// Voices created via voice design/generation.
    #[serde(default)]
    pub voice_generation: Vec<VoiceInfo>,
}

impl VoiceListResponse {
    /// Returns all voices combined into a single vector with type field set.
    pub fn all_voices(&self) -> Vec<VoiceInfo> {
        let mut all = Vec::with_capacity(
            self.system_voice.len() + self.voice_cloning.len() + self.voice_generation.len(),
        );

        for voice in &self.system_voice {
            let mut v = voice.clone();
            v.voice_type = Some(VoiceType::System);
            all.push(v);
        }

        for voice in &self.voice_cloning {
            let mut v = voice.clone();
            v.voice_type = Some(VoiceType::VoiceCloning);
            all.push(v);
        }

        for voice in &self.voice_generation {
            let mut v = voice.clone();
            v.voice_type = Some(VoiceType::VoiceGeneration);
            all.push(v);
        }

        all
    }
}

/// Information about a voice.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct VoiceInfo {
    /// Voice identifier.
    pub voice_id: String,

    /// Voice name.
    #[serde(default)]
    pub voice_name: String,

    /// Voice type.
    #[serde(skip_serializing_if = "Option::is_none", rename = "type")]
    pub voice_type: Option<VoiceType>,

    /// Voice description.
    #[serde(default)]
    pub description: Vec<String>,

    /// Creation time.
    #[serde(default)]
    pub created_time: String,
}

/// Request for voice cloning.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct VoiceCloneRequest {
    /// File ID of the clone audio (must be int64).
    pub file_id: i64,

    /// File ID of the demo audio (optional).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub demo_file_id: Option<i64>,

    /// Custom voice ID.
    pub voice_id: String,

    /// Model version.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,

    /// Preview text.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub text: Option<String>,
}

/// Response from voice cloning.
#[derive(Debug, Clone, Default)]
pub struct VoiceCloneResponse {
    /// Cloned voice ID.
    pub voice_id: String,

    /// Decoded demo audio.
    pub demo_audio: Vec<u8>,
}

/// Request for voice design.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct VoiceDesignRequest {
    /// Voice description.
    pub prompt: String,

    /// Preview text.
    pub preview_text: String,

    /// Custom voice ID (optional).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub voice_id: Option<String>,

    /// Model version.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
}

/// Response from voice design.
#[derive(Debug, Clone, Default)]
pub struct VoiceDesignResponse {
    /// Designed voice ID.
    pub voice_id: String,

    /// Decoded demo audio.
    pub demo_audio: Vec<u8>,
}
