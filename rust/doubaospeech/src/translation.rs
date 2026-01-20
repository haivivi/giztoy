//! Translation service for Doubao Speech API.
//!
//! Provides real-time speech translation functionality via WebSocket.

use std::sync::Arc;

use serde::{Deserialize, Serialize};

use crate::{
    http::HttpClient,
    types::{AudioFormat, Language, SampleRate},
};

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
    ///
    /// Note: The actual WebSocket session implementation requires
    /// additional WebSocket libraries like tokio-tungstenite.
    /// This method provides access to the authentication config
    /// needed for establishing the connection.
    pub fn http(&self) -> &Arc<HttpClient> {
        &self.http
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
