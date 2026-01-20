//! Voice management commands.
//!
//! Compatible with Go version's voice commands.

use clap::{Args, Subcommand};

use giztoy_minimax::{FilePurpose, VoiceCloneRequest, VoiceDesignRequest, VoiceType};

use super::{
    create_client, get_context, load_request, output_result, print_success, print_verbose,
    require_input_file,
};
use crate::Cli;

/// Voice management service.
///
/// Supports voice listing, cloning, and design.
#[derive(Args)]
pub struct VoiceCommand {
    #[command(subcommand)]
    command: VoiceSubcommand,
}

#[derive(Subcommand)]
enum VoiceSubcommand {
    /// List available voices
    List {
        /// Voice type filter (all, system, voice_cloning)
        #[arg(long, default_value = "all")]
        voice_type: String,
    },
    /// Upload voice clone source
    #[command(name = "upload-clone-source")]
    UploadCloneSource {
        /// Path to audio file
        #[arg(long)]
        file: String,
    },
    /// Clone a voice
    Clone,
    /// Design a voice
    Design,
}

impl VoiceCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            VoiceSubcommand::List { voice_type } => self.list(cli, voice_type).await,
            VoiceSubcommand::UploadCloneSource { file } => self.upload_clone_source(cli, file).await,
            VoiceSubcommand::Clone => self.clone_voice(cli).await,
            VoiceSubcommand::Design => self.design_voice(cli).await,
        }
    }

    async fn list(&self, cli: &Cli, voice_type: &str) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;

        let vt = match voice_type {
            "system" => Some(VoiceType::System),
            "voice_cloning" => Some(VoiceType::VoiceCloning),
            _ => None,
        };

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Voice type: {:?}", vt));

        let client = create_client(&ctx)?;
        let resp = client.voice().list(vt).await?;

        let total = resp.system_voice.len() + resp.voice_cloning.len() + resp.voice_generation.len();
        print_success(&format!("Found {} voice(s)", total));

        output_result(&resp, cli.output.as_deref(), cli.json)
    }

    async fn upload_clone_source(&self, cli: &Cli, file: &str) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Uploading file: {}", file));

        // Read file content
        let data = std::fs::read(file)?;
        let filename = std::path::Path::new(file)
            .file_name()
            .and_then(|n| n.to_str())
            .unwrap_or("audio.mp3");

        let client = create_client(&ctx)?;
        let resp = client.file().upload(&data, filename, FilePurpose::VoiceClone).await?;

        print_success(&format!("Voice clone source uploaded: {}", resp.file_id));

        output_result(&resp, cli.output.as_deref(), cli.json)
    }

    async fn clone_voice(&self, cli: &Cli) -> anyhow::Result<()> {
        let input_file = require_input_file(cli)?;
        let ctx = get_context(cli)?;

        let req: VoiceCloneRequest = load_request(input_file)?;

        print_verbose(cli, &format!("Using context: {}", ctx.name));

        let client = create_client(&ctx)?;
        let resp = client.voice().clone(&req).await?;

        print_success(&format!("Voice cloned: {}", resp.voice_id));

        // VoiceCloneResponse doesn't derive Serialize, output manually
        let result = serde_json::json!({
            "voice_id": resp.voice_id,
            "demo_audio_size": resp.demo_audio.len(),
        });
        output_result(&result, cli.output.as_deref(), cli.json)
    }

    async fn design_voice(&self, cli: &Cli) -> anyhow::Result<()> {
        let input_file = require_input_file(cli)?;
        let ctx = get_context(cli)?;

        let req: VoiceDesignRequest = load_request(input_file)?;

        print_verbose(cli, &format!("Using context: {}", ctx.name));

        let client = create_client(&ctx)?;
        let resp = client.voice().design(&req).await?;

        print_success(&format!("Voice designed: {}", resp.voice_id));

        // VoiceDesignResponse doesn't derive Serialize, output manually
        let result = serde_json::json!({
            "voice_id": resp.voice_id,
            "demo_audio_size": resp.demo_audio.len(),
        });
        output_result(&result, cli.output.as_deref(), cli.json)
    }
}
