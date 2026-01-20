//! Meeting transcription CLI commands

use clap::{Args, Subcommand};
use serde::{Deserialize, Serialize};

use giztoy_doubaospeech::{AudioFormat, Language, MeetingTaskRequest};

use super::{create_client, get_context, load_request, output_result, print_success, print_verbose};
use crate::Cli;

/// Meeting transcription service
#[derive(Args)]
pub struct MeetingCommand {
    #[command(subcommand)]
    command: MeetingSubcommand,
}

#[derive(Subcommand)]
enum MeetingSubcommand {
    /// Create meeting transcription task
    Create {
        /// Audio file URL
        #[arg(short = 'u', long)]
        url: Option<String>,
        /// Audio format
        #[arg(short = 'F', long)]
        format: Option<String>,
        /// Language
        #[arg(short = 'l', long)]
        language: Option<String>,
    },
    /// Query meeting task status
    Status {
        /// Task ID to query
        task_id: String,
    },
}

/// Meeting request from YAML/JSON file
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct MeetingFileRequest {
    #[serde(skip_serializing_if = "Option::is_none")]
    audio_url: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    format: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    language: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    callback_url: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    enable_diarization: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    speaker_count: Option<i32>,
}

impl MeetingCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            MeetingSubcommand::Create { url, format, language } => {
                self.create(cli, url.as_deref(), format.as_deref(), language.as_deref()).await
            }
            MeetingSubcommand::Status { task_id } => {
                self.status(cli, task_id).await
            }
        }
    }

    async fn create(
        &self,
        cli: &Cli,
        url: Option<&str>,
        format: Option<&str>,
        language: Option<&str>,
    ) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        let client = create_client(&ctx)?;

        // Load request from file or build from args
        let req = if let Some(input_file) = cli.input.as_deref() {
            let file_req: MeetingFileRequest = load_request(input_file)?;
            
            let audio_url = url.map(|s| s.to_string()).or(file_req.audio_url)
                .ok_or_else(|| anyhow::anyhow!("audio_url is required"))?;
            
            let format_str = format.or(file_req.format.as_deref()).unwrap_or("mp3");
            let audio_format = match format_str {
                "wav" => AudioFormat::Wav,
                "mp3" => AudioFormat::Mp3,
                "pcm" => AudioFormat::Pcm,
                "ogg" => AudioFormat::Ogg,
                _ => AudioFormat::Mp3,
            };

            let lang_str = language.or(file_req.language.as_deref()).unwrap_or("zh-CN");
            let lang = match lang_str {
                "en-US" => Language::EnUs,
                "ja-JP" => Language::JaJp,
                _ => Language::ZhCn,
            };

            MeetingTaskRequest {
                audio_url,
                format: Some(audio_format),
                language: Some(lang),
                callback_url: file_req.callback_url,
                enable_speaker_diarization: file_req.enable_diarization.unwrap_or(false),
                speaker_count: file_req.speaker_count,
                ..Default::default()
            }
        } else {
            let audio_url = url.ok_or_else(|| anyhow::anyhow!("audio_url is required, use -u flag or -f file"))?;
            
            let format_str = format.unwrap_or("mp3");
            let audio_format = match format_str {
                "wav" => AudioFormat::Wav,
                "mp3" => AudioFormat::Mp3,
                "pcm" => AudioFormat::Pcm,
                "ogg" => AudioFormat::Ogg,
                _ => AudioFormat::Mp3,
            };

            let lang_str = language.unwrap_or("zh-CN");
            let lang = match lang_str {
                "en-US" => Language::EnUs,
                "ja-JP" => Language::JaJp,
                _ => Language::ZhCn,
            };

            MeetingTaskRequest {
                audio_url: audio_url.to_string(),
                format: Some(audio_format),
                language: Some(lang),
                ..Default::default()
            }
        };

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Audio URL: {}", req.audio_url));

        let result = client.meeting().create_task(&req).await?;

        print_success(&format!("Task created: {}", result.task_id));

        output_result(&result, cli.output.as_deref(), cli.json)
    }

    async fn status(&self, cli: &Cli, task_id: &str) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        let client = create_client(&ctx)?;

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Querying task: {}", task_id));

        let result = client.meeting().get_task(task_id).await?;

        print_success(&format!("Task status: {:?}", result.status));

        output_result(&result, cli.output.as_deref(), cli.json)
    }
}
