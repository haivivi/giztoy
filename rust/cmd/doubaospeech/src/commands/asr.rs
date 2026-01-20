//! ASR (Automatic Speech Recognition) commands.
//!
//! Compatible with Go version's ASR commands.

use clap::{Args, Subcommand};
use serde::{Deserialize, Serialize};

use giztoy_doubaospeech::{AudioFormat, Language, OneSentenceRequest, SampleRate};

use super::{create_client, get_context, load_request, output_result, print_success, print_verbose};
use crate::Cli;

/// ASR (Automatic Speech Recognition) service.
///
/// Supports one-sentence recognition and file recognition.
#[derive(Args)]
pub struct AsrCommand {
    #[command(subcommand)]
    command: AsrSubcommand,
}

#[derive(Subcommand)]
enum AsrSubcommand {
    /// Recognize short audio (< 60s)
    OneSentence {
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
    /// Real-time streaming recognition
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
    /// Async file recognition
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
            AsrSubcommand::OneSentence {
                audio,
                format,
                sample_rate,
                language,
            } => {
                self.one_sentence(
                    cli,
                    audio.as_deref(),
                    format.as_deref(),
                    *sample_rate,
                    language.as_deref(),
                )
                .await
            }
            AsrSubcommand::Stream {
                audio,
                format,
                sample_rate,
                language,
            } => {
                self.stream(
                    cli,
                    audio.as_deref(),
                    format.as_deref(),
                    *sample_rate,
                    language.as_deref(),
                )
                .await
            }
            AsrSubcommand::File {
                audio,
                format,
                language,
                callback_url,
            } => {
                self.file(
                    cli,
                    audio.as_deref(),
                    format.as_deref(),
                    language.as_deref(),
                    callback_url.as_deref(),
                )
                .await
            }
            AsrSubcommand::Status { task_id } => self.status(cli, task_id).await,
        }
    }

    async fn one_sentence(
        &self,
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

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Format: {:?}", final_req.format));

        let response = client.asr().recognize_one_sentence(&final_req).await?;

        print_success(&format!("Recognition completed"));

        // Output result
        let result = serde_json::json!({
            "text": response.text,
            "duration_ms": response.duration,
        });

        output_result(&result, cli.output.as_deref(), cli.json)
    }

    async fn stream(
        &self,
        cli: &Cli,
        _audio_path: Option<&str>,
        _format: Option<&str>,
        _sample_rate: Option<i32>,
        _language: Option<&str>,
    ) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        print_verbose(cli, &format!("Using context: {}", ctx.name));

        // TODO: Implement streaming ASR
        eprintln!("[Streaming ASR not implemented yet]");
        eprintln!("Would start real-time streaming recognition");

        let result = serde_json::json!({
            "_note": "Streaming ASR not implemented yet",
        });
        output_result(&result, cli.output.as_deref(), cli.json)
    }

    async fn file(
        &self,
        cli: &Cli,
        _audio: Option<&str>,
        _format: Option<&str>,
        _language: Option<&str>,
        _callback_url: Option<&str>,
    ) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        print_verbose(cli, &format!("Using context: {}", ctx.name));

        // TODO: Implement file ASR
        eprintln!("[File ASR not implemented yet]");

        let result = serde_json::json!({
            "_note": "File ASR not implemented yet",
            "task_id": "placeholder-task-id",
        });
        output_result(&result, cli.output.as_deref(), cli.json)
    }

    async fn status(&self, cli: &Cli, task_id: &str) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Querying task: {}", task_id));

        // TODO: Implement task status query
        let result = serde_json::json!({
            "_note": "Task status query not implemented yet",
            "task_id": task_id,
            "status": "pending",
        });
        output_result(&result, cli.output.as_deref(), cli.json)
    }
}
