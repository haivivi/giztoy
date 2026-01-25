//! ASR (Automatic Speech Recognition) commands.
//!
//! Supports two API versions:
//!   - v1: Classic API (/api/v1/asr)
//!   - v2: BigModel API (/api/v3/sauc/*) - Recommended
//!
//! V2 Resource IDs:
//!   - volc.bigasr.sauc.duration: Streaming ASR (duration-based billing)
//!   - volc.bigasr.auc.duration: File ASR (duration-based billing)

use clap::{Args, Subcommand};
use serde::{Deserialize, Serialize};

use giztoy_doubaospeech::{
    AsrV2AsyncRequest, AsrV2Config, AudioFormat, FileAsrRequest, Language, OneSentenceRequest,
    SampleRate, StreamAsrConfig,
};

use super::{create_client, get_context, load_request, output_result, print_success, print_verbose};
use crate::Cli;

/// ASR (Automatic Speech Recognition) service.
///
/// Supports one-sentence recognition and file recognition.
/// Use `asr v1` for classic API or `asr v2` for BigModel API (recommended).
#[derive(Args)]
pub struct AsrCommand {
    #[command(subcommand)]
    command: AsrSubcommand,
}

#[derive(Subcommand)]
enum AsrSubcommand {
    /// ASR V1 API (Classic)
    V1(AsrV1Command),
    /// ASR V2 API (BigModel) - Recommended
    V2(AsrV2Command),
}

// ============================================================================
// V1 Commands
// ============================================================================

#[derive(Args)]
pub struct AsrV1Command {
    #[command(subcommand)]
    command: AsrV1Subcommand,
}

#[derive(Subcommand)]
enum AsrV1Subcommand {
    /// V1 one-sentence recognition (< 60s)
    Recognize {
        /// Audio file path
        #[arg(short = 'a', long)]
        audio: Option<String>,
        /// Audio format (wav, mp3, pcm)
        #[arg(short = 'F', long)]
        format: Option<String>,
        /// Sample rate
        #[arg(short = 's', long)]
        sample_rate: Option<i32>,
        /// Language (zh-CN, en-US)
        #[arg(short = 'l', long)]
        language: Option<String>,
    },
    /// V1 streaming recognition
    Stream {
        /// Audio file path
        #[arg(short = 'a', long)]
        audio: Option<String>,
        /// Audio format (wav, mp3, pcm)
        #[arg(short = 'F', long)]
        format: Option<String>,
        /// Sample rate
        #[arg(short = 's', long)]
        sample_rate: Option<i32>,
        /// Language (zh-CN, en-US)
        #[arg(short = 'l', long)]
        language: Option<String>,
    },
}

// ============================================================================
// V2 Commands
// ============================================================================

#[derive(Args)]
pub struct AsrV2Command {
    #[command(subcommand)]
    command: AsrV2Subcommand,
}

#[derive(Subcommand)]
enum AsrV2Subcommand {
    /// V2 BigModel streaming recognition (recommended)
    Stream {
        /// Audio file path
        #[arg(short = 'a', long)]
        audio: Option<String>,
        /// Audio format (wav, mp3, pcm)
        #[arg(short = 'F', long)]
        format: Option<String>,
        /// Sample rate
        #[arg(short = 's', long)]
        sample_rate: Option<i32>,
        /// Language (zh-CN, en-US)
        #[arg(short = 'l', long)]
        language: Option<String>,
    },
    /// V2 BigModel file recognition
    File {
        /// Audio file path or URL
        #[arg(short = 'a', long)]
        audio: Option<String>,
        /// Audio format
        #[arg(short = 'F', long)]
        format: Option<String>,
        /// Language
        #[arg(short = 'l', long)]
        language: Option<String>,
        /// Callback URL for task completion notification
        #[arg(long)]
        callback_url: Option<String>,
    },
    /// Query async task status
    Status {
        /// Task ID to query
        task_id: String,
    },
}

/// ASR request from YAML/JSON file.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct AsrFileRequest {
    #[serde(skip_serializing_if = "Option::is_none")]
    audio_file: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    audio_url: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    format: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    sample_rate: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    language: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    enable_itn: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    enable_punc: Option<bool>,
}

impl AsrCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            AsrSubcommand::V1(v1) => v1.run(cli).await,
            AsrSubcommand::V2(v2) => v2.run(cli).await,
        }
    }
}

impl AsrV1Command {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            AsrV1Subcommand::Recognize { audio, format, sample_rate, language } => {
                recognize_v1(cli, audio.as_deref(), format.as_deref(), *sample_rate, language.as_deref()).await
            }
            AsrV1Subcommand::Stream { audio, format, sample_rate, language } => {
                stream_v1(cli, audio.as_deref(), format.as_deref(), *sample_rate, language.as_deref()).await
            }
        }
    }
}

impl AsrV2Command {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            AsrV2Subcommand::Stream { audio, format, sample_rate, language } => {
                stream_v2(cli, audio.as_deref(), format.as_deref(), *sample_rate, language.as_deref()).await
            }
            AsrV2Subcommand::File { audio, format, language, callback_url } => {
                file_v2(cli, audio.as_deref(), format.as_deref(), language.as_deref(), callback_url.as_deref()).await
            }
            AsrV2Subcommand::Status { task_id } => {
                status_v2(cli, task_id).await
            }
        }
    }
}

// ============================================================================
// V1 Implementation Functions
// ============================================================================

async fn recognize_v1(
    cli: &Cli,
    audio_path: Option<&str>,
    format: Option<&str>,
    sample_rate: Option<i32>,
    language: Option<&str>,
) -> anyhow::Result<()> {
    let ctx = get_context(cli)?;
    let client = create_client(&ctx)?;

    // Build request from file or command line args
    let (audio_data, req) = if let Some(input_file) = cli.input.as_deref() {
        let file_req: AsrFileRequest = load_request(input_file)?;

        // Load audio from file if specified
        let audio = if let Some(audio_file) = audio_path.or(file_req.audio_file.as_deref()) {
            Some(std::fs::read(audio_file)?)
        } else {
            None
        };

        let format_str = format.or(file_req.format.as_deref()).unwrap_or("wav");
        let audio_format = match format_str {
            "wav" => AudioFormat::Wav,
            "mp3" => AudioFormat::Mp3,
            "pcm" => AudioFormat::Pcm,
            "ogg" => AudioFormat::Ogg,
            _ => AudioFormat::Wav,
        };

        let sr = sample_rate.or(file_req.sample_rate);
        let sample_rate_enum = sr.and_then(|r| match r {
            8000 => Some(SampleRate::Rate8000),
            16000 => Some(SampleRate::Rate16000),
            22050 => Some(SampleRate::Rate22050),
            24000 => Some(SampleRate::Rate24000),
            32000 => Some(SampleRate::Rate32000),
            44100 => Some(SampleRate::Rate44100),
            48000 => Some(SampleRate::Rate48000),
            _ => None,
        });

        let lang = language.or(file_req.language.as_deref());
        let language_enum = lang.and_then(|l| match l {
            "zh-CN" => Some(Language::ZhCn),
            "en-US" => Some(Language::EnUs),
            "ja-JP" => Some(Language::JaJp),
            _ => None,
        });

        (
            audio,
            OneSentenceRequest {
                audio: None,
                audio_url: file_req.audio_url,
                format: audio_format,
                sample_rate: sample_rate_enum,
                language: language_enum,
                enable_itn: file_req.enable_itn,
                enable_punc: file_req.enable_punc,
                ..Default::default()
            },
        )
    } else {
        // Build from command line args only
        let audio_file =
            audio_path.ok_or_else(|| anyhow::anyhow!("audio file is required, use -a flag"))?;
        let audio = std::fs::read(audio_file)?;

        let format_str = format.unwrap_or("wav");
        let audio_format = match format_str {
            "wav" => AudioFormat::Wav,
            "mp3" => AudioFormat::Mp3,
            "pcm" => AudioFormat::Pcm,
            "ogg" => AudioFormat::Ogg,
            _ => AudioFormat::Wav,
        };

        let sample_rate_enum = sample_rate.and_then(|r| match r {
            8000 => Some(SampleRate::Rate8000),
            16000 => Some(SampleRate::Rate16000),
            22050 => Some(SampleRate::Rate22050),
            24000 => Some(SampleRate::Rate24000),
            32000 => Some(SampleRate::Rate32000),
            44100 => Some(SampleRate::Rate44100),
            48000 => Some(SampleRate::Rate48000),
            _ => None,
        });

        let language_enum = language.and_then(|l| match l {
            "zh-CN" => Some(Language::ZhCn),
            "en-US" => Some(Language::EnUs),
            "ja-JP" => Some(Language::JaJp),
            _ => None,
        });

        (
            Some(audio),
            OneSentenceRequest {
                audio: None,
                format: audio_format,
                sample_rate: sample_rate_enum,
                language: language_enum,
                ..Default::default()
            },
        )
    };

    // Set audio data
    let mut final_req = req;
    final_req.audio = audio_data;

    print_verbose(cli, &format!("Using V1 API (classic)"));
    print_verbose(cli, &format!("Format: {:?}", final_req.format));

    let response = client.asr().recognize_one_sentence(&final_req).await?;

    print_success(&format!("Recognition completed"));

    // Output result
    let result = serde_json::json!({
        "api_version": "v1",
        "text": response.text,
        "duration_ms": response.duration,
    });

    output_result(&result, cli.output.as_deref(), cli.json)
}

async fn stream_v1(
    cli: &Cli,
    audio_path: Option<&str>,
    format: Option<&str>,
    sample_rate: Option<i32>,
    language: Option<&str>,
) -> anyhow::Result<()> {
    let ctx = get_context(cli)?;
    let client = create_client(&ctx)?;
    print_verbose(cli, &format!("Using V1 API (classic streaming)"));

    // Load audio file
    let audio_file = if let Some(input_file) = cli.input.as_deref() {
        let file_req: AsrFileRequest = load_request(input_file)?;
        audio_path
            .or(file_req.audio_file.as_deref())
            .ok_or_else(|| anyhow::anyhow!("audio file is required"))?
            .to_string()
    } else {
        audio_path
            .ok_or_else(|| anyhow::anyhow!("audio file is required, use -a flag"))?
            .to_string()
    };

    let audio_data = std::fs::read(&audio_file)?;
    print_verbose(cli, &format!("Audio file: {} ({} bytes)", audio_file, audio_data.len()));

    // Parse format
    let format_str = format.unwrap_or("pcm");
    let audio_format = match format_str {
        "wav" => AudioFormat::Wav,
        "mp3" => AudioFormat::Mp3,
        "pcm" => AudioFormat::Pcm,
        "ogg" => AudioFormat::Ogg,
        _ => AudioFormat::Pcm,
    };

    // Parse sample rate
    let sr = sample_rate.unwrap_or(16000);
    let sample_rate_enum = match sr {
        8000 => SampleRate::Rate8000,
        16000 => SampleRate::Rate16000,
        22050 => SampleRate::Rate22050,
        24000 => SampleRate::Rate24000,
        32000 => SampleRate::Rate32000,
        44100 => SampleRate::Rate44100,
        48000 => SampleRate::Rate48000,
        _ => SampleRate::Rate16000,
    };

    // Parse language
    let language_enum = language.and_then(|l| match l {
        "zh-CN" => Some(Language::ZhCn),
        "en-US" => Some(Language::EnUs),
        "ja-JP" => Some(Language::JaJp),
        _ => None,
    });

    let config = StreamAsrConfig {
        format: audio_format,
        sample_rate: sample_rate_enum,
        bits: 16,
        channel: 1,
        language: language_enum,
        show_utterances: Some(true),
        ..Default::default()
    };

    print_verbose(cli, "Opening streaming ASR session...");
    let session = client.asr().open_stream_session(&config).await?;

    // Send audio in chunks
    let chunk_size = 3200; // 100ms of 16kHz 16-bit mono audio
    let chunks: Vec<&[u8]> = audio_data.chunks(chunk_size).collect();
    let total_chunks = chunks.len();

    for (i, chunk) in chunks.iter().enumerate() {
        let is_last = i == total_chunks - 1;
        session.send_audio(chunk, is_last).await?;
        if !is_last {
            tokio::time::sleep(tokio::time::Duration::from_millis(100)).await;
        }
    }
    print_verbose(cli, &format!("Sent {} audio chunks", total_chunks));

    // Receive results
    let mut final_text = String::new();
    while let Some(result) = session.recv().await {
        match result {
            Ok(chunk) => {
                if !chunk.text.is_empty() {
                    print_verbose(cli, &format!("Partial: {}", chunk.text));
                }
                if chunk.is_final {
                    final_text = chunk.text;
                    break;
                }
            }
            Err(e) => {
                return Err(anyhow::anyhow!("ASR error: {}", e));
            }
        }
    }

    session.close().await?;
    print_success("Streaming ASR completed");

    let result = serde_json::json!({
        "api_version": "v1",
        "text": final_text,
    });
    output_result(&result, cli.output.as_deref(), cli.json)
}

// ============================================================================
// V2 Implementation Functions
// ============================================================================

async fn stream_v2(
    cli: &Cli,
    audio_path: Option<&str>,
    format: Option<&str>,
    sample_rate: Option<i32>,
    language: Option<&str>,
) -> anyhow::Result<()> {
    let ctx = get_context(cli)?;
    let client = create_client(&ctx)?;
    print_verbose(cli, "Using V2 API (BigModel streaming via /api/v3/sauc/bigmodel)");

    // Load audio file
    let audio_file = if let Some(input_file) = cli.input.as_deref() {
        let file_req: AsrFileRequest = load_request(input_file)?;
        audio_path
            .or(file_req.audio_file.as_deref())
            .ok_or_else(|| anyhow::anyhow!("audio file is required"))?
            .to_string()
    } else {
        audio_path
            .ok_or_else(|| anyhow::anyhow!("audio file is required, use -a flag"))?
            .to_string()
    };

    let audio_data = std::fs::read(&audio_file)?;
    print_verbose(cli, &format!("Audio file: {} ({} bytes)", audio_file, audio_data.len()));

    // Parse format
    let format_str = format.unwrap_or("pcm");

    // Parse sample rate
    let sr = sample_rate.unwrap_or(16000);

    // Build AsrV2Config for BigModel
    let config = AsrV2Config {
        format: format_str.to_string(),
        sample_rate: sr,
        channels: 1,
        bits: 16,
        language: language.map(|s| s.to_string()),
        enable_itn: true,
        enable_punc: true,
        ..Default::default()
    };

    print_verbose(cli, "Opening V2 BigModel ASR session...");
    let mut session = client.asr_v2().open_stream_session(&config).await?;

    // Send audio in chunks
    let chunk_size = 3200; // 100ms of 16kHz 16-bit mono audio
    let chunks: Vec<&[u8]> = audio_data.chunks(chunk_size).collect();
    let total_chunks = chunks.len();

    for (i, chunk) in chunks.iter().enumerate() {
        let is_last = i == total_chunks - 1;
        session.send_audio(chunk, is_last).await?;
        if !is_last {
            tokio::time::sleep(tokio::time::Duration::from_millis(100)).await;
        }
    }
    print_verbose(cli, &format!("Sent {} audio chunks", total_chunks));

    // Receive results
    let mut final_text = String::new();
    let mut duration_ms = 0;
    while let Some(result) = session.recv().await {
        let chunk = result?;
        if !chunk.text.is_empty() {
            print_verbose(cli, &format!("Partial: {}", chunk.text));
        }
        if chunk.duration > 0 {
            duration_ms = chunk.duration;
        }
        if chunk.is_final {
            final_text = chunk.text;
            break;
        }
    }

    session.close().await?;
    print_success("V2 BigModel ASR completed");

    let result = serde_json::json!({
        "api_version": "v2_bigmodel",
        "text": final_text,
        "duration_ms": duration_ms,
    });
    output_result(&result, cli.output.as_deref(), cli.json)
}

async fn file_v2(
    cli: &Cli,
    audio: Option<&str>,
    format: Option<&str>,
    language: Option<&str>,
    callback_url: Option<&str>,
) -> anyhow::Result<()> {
    let ctx = get_context(cli)?;
    let client = create_client(&ctx)?;
    print_verbose(cli, "Using V2 API (BigModel async file via /api/v3/sauc/bigmodel_async)");

    // Get audio URL and format
    let (audio_url, audio_format) = if let Some(input_file) = cli.input.as_deref() {
        let file_req: AsrFileRequest = load_request(input_file)?;
        let url = audio
            .map(|s| s.to_string())
            .or(file_req.audio_url)
            .ok_or_else(|| anyhow::anyhow!("audio URL is required"))?;
        let fmt = format
            .map(|s| s.to_string())
            .or(file_req.format)
            .unwrap_or_else(|| "mp3".to_string());
        (url, fmt)
    } else {
        let url = audio
            .ok_or_else(|| anyhow::anyhow!("audio URL is required, use -a flag"))?
            .to_string();
        let fmt = format.unwrap_or("mp3").to_string();
        (url, fmt)
    };

    print_verbose(cli, &format!("Audio URL: {}", audio_url));
    print_verbose(cli, &format!("Format: {}", audio_format));

    let req = AsrV2AsyncRequest {
        audio_url: Some(audio_url),
        format: audio_format,
        language: language.map(|s| s.to_string()),
        enable_itn: true,
        enable_punc: true,
        callback_url: callback_url.map(|s| s.to_string()),
        ..Default::default()
    };

    print_verbose(cli, "Submitting V2 BigModel async file ASR task...");
    let result = client.asr_v2().submit_async(&req).await?;

    print_success(&format!("Task submitted: {}", result.task_id));

    let output = serde_json::json!({
        "api_version": "v2_bigmodel",
        "task_id": result.task_id,
        "status": result.status,
    });
    output_result(&output, cli.output.as_deref(), cli.json)
}

async fn status_v2(cli: &Cli, task_id: &str) -> anyhow::Result<()> {
    let ctx = get_context(cli)?;
    let client = create_client(&ctx)?;
    print_verbose(cli, &format!("Using context: {}", ctx.name));
    print_verbose(cli, &format!("Querying task: {}", task_id));

    let result = client.asr_v2().query_async(task_id).await?;

    print_success(&format!("Task status: {}", result.status));

    let output = serde_json::json!({
        "api_version": "v2_bigmodel",
        "task_id": result.task_id,
        "status": result.status,
        "text": result.text,
        "error": result.error,
    });
    output_result(&output, cli.output.as_deref(), cli.json)
}
