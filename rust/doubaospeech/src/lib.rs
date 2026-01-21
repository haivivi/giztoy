//! Doubao Speech API SDK for Rust.
//!
//! This crate provides a client for interacting with the Doubao Speech API
//! (豆包语音 API).
//!
//! # Features
//!
//! - TTS (Text-to-Speech): Voice synthesis with support for sync, streaming modes
//! - ASR (Automatic Speech Recognition): Speech recognition
//! - Voice Clone: Custom voice creation
//! - Realtime: End-to-end realtime voice dialogue
//! - Meeting: Meeting transcription
//! - Podcast: Multi-speaker podcast synthesis
//! - Translation: Simultaneous interpretation
//! - Media: Audio/video subtitle extraction
//!
//! # Quick Start
//!
//! ```rust,no_run
//! use giztoy_doubaospeech::{Client, TtsRequest};
//!
//! #[tokio::main]
//! async fn main() -> Result<(), Box<dyn std::error::Error>> {
//!     // Create client with API key authentication
//!     let client = Client::builder("your-app-id")
//!         .api_key("your-api-key")
//!         .cluster("volcano_tts")
//!         .build()?;
//!
//!     // Synchronous TTS
//!     let response = client.tts().synthesize(&TtsRequest {
//!         text: "你好，世界！".to_string(),
//!         voice_type: "zh_female_cancan".to_string(),
//!         ..Default::default()
//!     }).await?;
//!
//!     // response.audio contains the audio data
//!     println!("Audio length: {} bytes", response.audio.len());
//!
//!     Ok(())
//! }
//! ```
//!
//! # Authentication
//!
//! The client supports multiple authentication methods:
//!
//! 1. API Key (recommended, simplest):
//! ```rust,no_run
//! let client = Client::builder("app-id")
//!     .api_key("your-api-key")
//!     .build()?;
//! ```
//!
//! 2. Bearer Token:
//! ```rust,no_run
//! let client = Client::builder("app-id")
//!     .bearer_token("your-token")
//!     .build()?;
//! ```
//!
//! 3. V2/V3 API Key (for BigModel APIs):
//! ```rust,no_run
//! let client = Client::builder("app-id")
//!     .v2_api_key("access-key", "app-key")
//!     .build()?;
//! ```

mod asr;
mod client;
mod console;
mod error;
pub mod http;
mod media;
mod meeting;
mod podcast;
pub mod protocol;
mod realtime;
mod translation;
mod tts;
mod types;
mod voice_clone;

pub use asr::{
    AsrChunk, AsrResult, AsrService, AsrStreamSession, FileAsrRequest, FileAsrTaskResult,
    OneSentenceRequest, StreamAsrConfig, Utterance, Word,
};
pub use client::{
    Client, ClientBuilder, DEFAULT_BASE_URL, DEFAULT_WS_URL,
    APP_KEY_PODCAST, APP_KEY_REALTIME,
    RESOURCE_ASR_FILE, RESOURCE_ASR_STREAM, RESOURCE_ASR_STREAM_V2,
    RESOURCE_PODCAST, RESOURCE_REALTIME, RESOURCE_TRANSLATION,
    RESOURCE_TTS_V1, RESOURCE_TTS_V1_CONCUR, RESOURCE_TTS_V2, RESOURCE_TTS_V2_CONCUR,
    RESOURCE_VOICE_CLONE_V1, RESOURCE_VOICE_CLONE_V2,
};
pub use error::{status_code, Error, Result};
pub use tts::{TtsChunk, TtsRequest, TtsResponse, TtsService};
pub use types::{
    AudioEncoding, AudioFormat, AudioInfo, Language, LocationInfo, SampleRate,
    SubtitleFormat, SubtitleSegment, TaskStatus, TtsTextType,
    VoiceCloneModelType, VoiceCloneStatusType,
};
pub use media::{
    MediaService, SubtitleRequest, SubtitleResult, SubtitleTaskResult, SubtitleTaskStatus,
};
pub use meeting::{
    MeetingResult, MeetingSegment, MeetingService, MeetingTaskRequest, MeetingTaskResult,
    MeetingTaskStatus,
};
pub use podcast::{
    PodcastLine, PodcastResult, PodcastService, PodcastTaskRequest, PodcastTaskResult,
    PodcastTaskStatus,
};
pub use console::{
    Console, ListSpeakersRequest, ListSpeakersResponse, ListTimbresRequest, ListTimbresResponse,
    ListVoiceCloneStatusRequest, ListVoiceCloneStatusResponse, SpeakerInfo, TimbreCategory,
    TimbreDetailInfo, TimbreEmotion, TimbreInfo, VoiceCloneTrainStatus,
};
pub use translation::{
    TranslationAudioConfig, TranslationChunk, TranslationConfig, TranslationService,
    TranslationSession,
};
pub use voice_clone::{
    VoiceCloneInfo, VoiceCloneResult, VoiceCloneService, VoiceCloneStatus, VoiceCloneTrainRequest,
};
pub use realtime::{
    LocationInfo as RealtimeLocationInfo, RealtimeASRConfig, RealtimeASRInfo, RealtimeAudioConfig,
    RealtimeConfig, RealtimeConnection, RealtimeDialogConfig, RealtimeEvent, RealtimeEventType,
    RealtimeService, RealtimeSession, RealtimeTTSConfig, RealtimeTTSInfo,
};