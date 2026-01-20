//! Podcast synthesis CLI commands

use clap::{Args, Subcommand};
use serde::{Deserialize, Serialize};

use giztoy_doubaospeech::{PodcastLine, PodcastTaskRequest};

use super::{create_client, get_context, load_request, output_result, print_success, print_verbose};
use crate::Cli;

/// Podcast synthesis service
#[derive(Args)]
pub struct PodcastCommand {
    #[command(subcommand)]
    command: PodcastSubcommand,
}

#[derive(Subcommand)]
enum PodcastSubcommand {
    /// Create podcast synthesis task
    Create,
    /// Query podcast task status
    Status {
        /// Task ID to query
        task_id: String,
    },
}

/// Podcast request from YAML/JSON file
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct PodcastFileRequest {
    #[serde(default)]
    lines: Vec<PodcastLineConfig>,
    #[serde(skip_serializing_if = "Option::is_none")]
    callback_url: Option<String>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct PodcastLineConfig {
    speaker_id: String,
    text: String,
}

impl PodcastCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            PodcastSubcommand::Create => self.create(cli).await,
            PodcastSubcommand::Status { task_id } => self.status(cli, task_id).await,
        }
    }

    async fn create(&self, cli: &Cli) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        let client = create_client(&ctx)?;

        let input_file = cli.input.as_deref()
            .ok_or_else(|| anyhow::anyhow!("input file is required, use -f flag"))?;

        let file_req: PodcastFileRequest = load_request(input_file)?;

        let script: Vec<PodcastLine> = file_req.lines.into_iter().map(|l| PodcastLine {
            speaker_id: l.speaker_id,
            text: l.text,
            emotion: None,
            speed_ratio: None,
        }).collect();

        if script.is_empty() {
            return Err(anyhow::anyhow!("at least one line is required"));
        }

        let req = PodcastTaskRequest {
            script: script.clone(),
            callback_url: file_req.callback_url,
            ..Default::default()
        };

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Lines: {}", script.len()));

        let result = client.podcast().create_task(&req).await?;

        print_success(&format!("Task created: {}", result.task_id));

        output_result(&result, cli.output.as_deref(), cli.json)
    }

    async fn status(&self, cli: &Cli, task_id: &str) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        let client = create_client(&ctx)?;

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Querying task: {}", task_id));

        let result = client.podcast().get_task(task_id).await?;

        print_success(&format!("Task status: {:?}", result.status));

        output_result(&result, cli.output.as_deref(), cli.json)
    }
}
