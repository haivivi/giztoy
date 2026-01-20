//! File management commands.
//!
//! Compatible with Go version's file commands.

use clap::{Args, Subcommand};

use giztoy_minimax::FilePurpose;

use super::{create_client, get_context, output_result, print_success, print_verbose};
use crate::Cli;

/// File management service.
///
/// Supports file upload, list, retrieve, and delete operations.
#[derive(Args)]
pub struct FileCommand {
    #[command(subcommand)]
    command: FileSubcommand,
}

#[derive(Subcommand)]
enum FileSubcommand {
    /// Upload a file
    Upload {
        /// Path to file
        #[arg(long)]
        file: String,
        /// File purpose (voice_clone, prompt_audio, t2a_async_input)
        #[arg(long, default_value = "voice_clone")]
        purpose: String,
    },
    /// List files
    List {
        /// File purpose filter
        #[arg(long)]
        purpose: Option<String>,
    },
    /// Retrieve file info
    Retrieve {
        /// File ID
        file_id: String,
    },
    /// Delete a file
    Delete {
        /// File ID
        file_id: String,
    },
}

impl FileCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            FileSubcommand::Upload { file, purpose } => self.upload(cli, file, purpose).await,
            FileSubcommand::List { purpose } => self.list(cli, purpose.as_deref()).await,
            FileSubcommand::Retrieve { file_id } => self.retrieve(cli, file_id).await,
            FileSubcommand::Delete { file_id } => self.delete(cli, file_id).await,
        }
    }

    async fn upload(&self, cli: &Cli, file: &str, purpose: &str) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;

        let purpose = match purpose {
            "prompt_audio" => FilePurpose::PromptAudio,
            "t2a_async_input" => FilePurpose::T2aAsyncInput,
            _ => FilePurpose::VoiceClone,
        };

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Uploading file: {}", file));
        print_verbose(cli, &format!("Purpose: {:?}", purpose));

        // Read file content
        let data = std::fs::read(file)?;
        let filename = std::path::Path::new(file)
            .file_name()
            .and_then(|n| n.to_str())
            .unwrap_or("file");

        let client = create_client(&ctx)?;
        let resp = client.file().upload(&data, filename, purpose).await?;

        print_success(&format!("File uploaded: {}", resp.file_id));

        output_result(&resp, cli.output.as_deref(), cli.json)
    }

    async fn list(&self, cli: &Cli, purpose: Option<&str>) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;

        let purpose = purpose.map(|p| match p {
            "prompt_audio" => FilePurpose::PromptAudio,
            "t2a_async_input" => FilePurpose::T2aAsyncInput,
            _ => FilePurpose::VoiceClone,
        });

        print_verbose(cli, &format!("Using context: {}", ctx.name));

        let client = create_client(&ctx)?;
        let resp = client.file().list(purpose).await?;

        print_success(&format!("Found {} file(s)", resp.files.len()));

        output_result(&resp, cli.output.as_deref(), cli.json)
    }

    async fn retrieve(&self, cli: &Cli, file_id: &str) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("File ID: {}", file_id));

        let client = create_client(&ctx)?;
        let resp = client.file().get(file_id).await?;

        output_result(&resp, cli.output.as_deref(), cli.json)
    }

    async fn delete(&self, cli: &Cli, file_id: &str) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Deleting file: {}", file_id));

        let client = create_client(&ctx)?;
        client.file().delete(file_id).await?;

        print_success(&format!("File deleted: {}", file_id));

        let result = serde_json::json!({
            "file_id": file_id,
            "deleted": true,
        });
        output_result(&result, cli.output.as_deref(), cli.json)
    }
}
