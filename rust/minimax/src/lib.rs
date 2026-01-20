//! MiniMax API SDK for Rust.
//!
//! This crate provides a client for interacting with the MiniMax API.

mod client;
mod error;
mod file;
pub mod http;
mod image;
mod models;
mod music;
mod speech;
mod task;
mod text;
mod types;
mod video;
mod voice;

pub use client::{Client, ClientBuilder, DEFAULT_BASE_URL, BASE_URL_GLOBAL};
pub use error::{Error, Result};
pub use file::{FileService, FileInfo, FileListResponse, UploadResponse};
pub use image::{ImageService, ImageGenerateRequest, ImageReferenceRequest, ImageResponse, ImageData};
pub use models::*;
pub use music::{MusicService, MusicRequest, MusicResponse};
pub use speech::{
    SpeechService, SpeechRequest, SpeechResponse, SpeechChunk,
    AsyncSpeechRequest, VoiceSetting, AudioSetting, PronunciationDict,
};
pub use task::Task;
pub use text::{TextService, ChatCompletionRequest, ChatCompletionResponse, Message, Choice, ChunkDelta};
pub use types::{
    AudioFormat, AudioInfo, FilePurpose, FlexibleId, OutputFormat, Subtitle,
    SubtitleSegment, TaskStatus, VoiceType,
};
pub use video::{
    VideoService, TextToVideoRequest, ImageToVideoRequest, FrameToVideoRequest,
    SubjectRefVideoRequest, VideoAgentRequest, VideoResult,
};
pub use voice::{
    VoiceService, VoiceListResponse, VoiceInfo, VoiceCloneRequest, VoiceCloneResponse,
    VoiceDesignRequest, VoiceDesignResponse,
};
