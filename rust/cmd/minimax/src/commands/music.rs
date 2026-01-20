//! Music generation commands.
//!
//! Compatible with Go version's music commands.

use clap::{Args, Subcommand};

use giztoy_minimax::{MusicRequest, MODEL_MUSIC_20};

use super::{
    create_client, format_bytes, get_context, load_request, output_bytes, output_result,
    print_success, print_verbose, require_input_file,
};
use crate::Cli;

/// Music generation service.
///
/// Supports music generation from prompts and lyrics.
#[derive(Args)]
pub struct MusicCommand {
    #[command(subcommand)]
    command: MusicSubcommand,
}

#[derive(Subcommand)]
enum MusicSubcommand {
    /// Generate music from prompt and lyrics
    Generate,
}

impl MusicCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            MusicSubcommand::Generate => self.generate(cli).await,
        }
    }

    async fn generate(&self, cli: &Cli) -> anyhow::Result<()> {
        let input_file = require_input_file(cli)?;
        let ctx = get_context(cli)?;

        let mut req: MusicRequest = load_request(input_file)?;

        // Use defaults if not specified
        if req.model.is_none() {
            req.model = Some(MODEL_MUSIC_20.to_string());
        }

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(
            cli,
            &format!("Model: {}", req.model.as_deref().unwrap_or("default")),
        );

        let client = create_client(&ctx)?;
        let resp = client.music().generate(&req).await?;

        // Determine output file extension
        let ext = req.format.as_deref().unwrap_or("mp3");
        let default_output = format!("music.{}", ext);
        let output_path = cli.output.as_deref().unwrap_or(&default_output);

        // Write audio to file
        output_bytes(&resp.audio, output_path)?;
        print_success(&format!(
            "Music saved to: {} ({})",
            output_path,
            format_bytes(resp.audio.len())
        ));

        // Output final info
        let result = serde_json::json!({
            "audio_size": resp.audio.len(),
            "duration": resp.duration,
            "extra_info": resp.extra_info,
            "output_file": output_path,
        });

        output_result(&result, None, cli.json)
    }
}
