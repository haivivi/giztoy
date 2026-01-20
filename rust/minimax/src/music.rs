//! Music generation service.

use std::sync::Arc;

use serde::{Deserialize, Serialize};

use super::{
    error::Result,
    http::{decode_hex_audio, HttpClient},
    types::{AudioInfo, BaseResp},
};

/// Music generation service.
pub struct MusicService {
    http: Arc<HttpClient>,
}

impl MusicService {
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Generates music from a prompt and lyrics.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let request = MusicRequest {
    ///     model: Some("music-2.0".to_string()),
    ///     prompt: "Pop music, upbeat, summer vibes".to_string(),
    ///     lyrics: "[Verse]\nSunshine in my eyes\n[Chorus]\nFeeling so alive".to_string(),
    ///     ..Default::default()
    /// };
    ///
    /// let response = client.music().generate(&request).await?;
    /// std::fs::write("music.mp3", &response.audio)?;
    /// ```
    pub async fn generate(&self, request: &MusicRequest) -> Result<MusicResponse> {
        #[derive(Deserialize)]
        struct ApiResponse {
            data: MusicData,
            extra_info: Option<AudioInfo>,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        #[derive(Deserialize)]
        struct MusicData {
            #[serde(default)]
            audio: String,
            #[serde(default)]
            duration: i32,
        }

        let resp: ApiResponse = self
            .http
            .request("POST", "/v1/music_generation", Some(request))
            .await?;

        let audio = if !resp.data.audio.is_empty() {
            decode_hex_audio(&resp.data.audio)?
        } else {
            Vec::new()
        };

        Ok(MusicResponse {
            audio,
            duration: resp.data.duration,
            extra_info: resp.extra_info,
        })
    }
}

// ==================== Request/Response Types ====================

/// Request for music generation.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct MusicRequest {
    /// Model name.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,

    /// Music inspiration (10-300 characters).
    /// Describe style, mood, scene, etc.
    pub prompt: String,

    /// Song lyrics (10-600 characters).
    /// Use \n to separate lines.
    /// Supports tags: [Intro], [Verse], [Chorus], [Bridge], [Outro].
    pub lyrics: String,

    /// Sample rate: 16000, 24000, 32000, 44100.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sample_rate: Option<i32>,

    /// Bitrate: 32000, 64000, 128000, 256000.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub bitrate: Option<i32>,

    /// Audio format: mp3, wav, pcm.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub format: Option<String>,
}

/// Response from music generation.
#[derive(Debug, Clone, Default)]
pub struct MusicResponse {
    /// Decoded audio data.
    pub audio: Vec<u8>,

    /// Audio duration in milliseconds.
    pub duration: i32,

    /// Audio metadata.
    pub extra_info: Option<AudioInfo>,
}
