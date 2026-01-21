//! Media processing CLI commands

use clap::{Args, Subcommand};
use serde::{Deserialize, Serialize};

use giztoy_doubaospeech::{Language, SubtitleFormat, SubtitleRequest};

use super::{create_client, get_context, load_request, output_result, print_success, print_verbose};
use crate::Cli;

/// Media processing service
#[derive(Args)]
pub struct MediaCommand {
    #[command(subcommand)]
    command: MediaSubcommand,
}

#[derive(Subcommand)]
enum MediaSubcommand {
    /// Extract subtitles from media
    Subtitle {
        /// Media file URL
        #[arg(short = 'u', long)]
        url: Option<String>,
        /// Output format (srt, vtt, txt)
        #[arg(short = 'F', long)]
        format: Option<String>,
        /// Language
        #[arg(short = 'l', long)]
        language: Option<String>,
    },
    /// Query media task status
    Status {
        /// Task ID to query
        task_id: String,
    },
}

/// Subtitle request from YAML/JSON file
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct SubtitleFileRequest {
    #[serde(skip_serializing_if = "Option::is_none")]
    media_url: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    format: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    language: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    callback_url: Option<String>,
}

impl MediaCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            MediaSubcommand::Subtitle { url, format, language } => {
                self.subtitle(cli, url.as_deref(), format.as_deref(), language.as_deref()).await
            }
            MediaSubcommand::Status { task_id } => {
                self.status(cli, task_id).await
            }
        }
    }

    async fn subtitle(
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
            let file_req: SubtitleFileRequest = load_request(input_file)?;
            
            let media_url = url.map(|s| s.to_string()).or(file_req.media_url)
                .ok_or_else(|| anyhow::anyhow!("media_url is required"))?;
            
            let format_str = format.or(file_req.format.as_deref()).unwrap_or("srt");
            let subtitle_format = match format_str {
                "vtt" => SubtitleFormat::Vtt,
                "json" => SubtitleFormat::Json,
                _ => SubtitleFormat::Srt,
            };

            let lang_str = language.or(file_req.language.as_deref()).unwrap_or("zh-CN");
            let lang = match lang_str {
                "en-US" => Language::EnUs,
                "ja-JP" => Language::JaJp,
                _ => Language::ZhCn,
            };

            SubtitleRequest {
                media_url,
                format: Some(subtitle_format),
                language: Some(lang),
                enable_translation: false,
                target_language: None,
                callback_url: file_req.callback_url,
            }
        } else {
            let media_url = url.ok_or_else(|| anyhow::anyhow!("media_url is required, use -u flag or -f file"))?;
            
            let format_str = format.unwrap_or("srt");
            let subtitle_format = match format_str {
                "vtt" => SubtitleFormat::Vtt,
                "json" => SubtitleFormat::Json,
                _ => SubtitleFormat::Srt,
            };

            let lang_str = language.unwrap_or("zh-CN");
            let lang = match lang_str {
                "en-US" => Language::EnUs,
                "ja-JP" => Language::JaJp,
                _ => Language::ZhCn,
            };

            SubtitleRequest {
                media_url: media_url.to_string(),
                format: Some(subtitle_format),
                language: Some(lang),
                enable_translation: false,
                target_language: None,
                callback_url: None,
            }
        };

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Media URL: {}", req.media_url));

        let result = client.media().extract_subtitle(&req).await?;

        print_success(&format!("Task created: {}", result.task_id));

        output_result(&result, cli.output.as_deref(), cli.json)
    }

    async fn status(&self, cli: &Cli, task_id: &str) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        let client = create_client(&ctx)?;

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Querying task: {}", task_id));

        let result = client.media().get_subtitle_task(task_id).await?;

        print_success(&format!("Task status: {:?}", result.status));

        output_result(&result, cli.output.as_deref(), cli.json)
    }
}
