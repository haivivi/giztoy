//! Image generation commands.
//!
//! Compatible with Go version's image commands.

use clap::{Args, Subcommand};

use giztoy_minimax::{ImageGenerateRequest, ImageReferenceRequest, MODEL_IMAGE_01};

use super::{
    create_client, get_context, load_request, output_result, print_success, print_verbose,
    require_input_file,
};
use crate::Cli;

/// Image generation service.
///
/// Supports text-to-image generation.
#[derive(Args)]
pub struct ImageCommand {
    #[command(subcommand)]
    command: ImageSubcommand,
}

#[derive(Subcommand)]
enum ImageSubcommand {
    /// Generate image from text prompt
    Generate,
    /// Generate image with reference
    Reference,
}

impl ImageCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            ImageSubcommand::Generate => self.generate(cli).await,
            ImageSubcommand::Reference => self.reference(cli).await,
        }
    }

    async fn generate(&self, cli: &Cli) -> anyhow::Result<()> {
        let input_file = require_input_file(cli)?;
        let ctx = get_context(cli)?;

        let mut req: ImageGenerateRequest = load_request(input_file)?;

        // Use defaults if not specified
        if req.model.is_empty() {
            req.model = MODEL_IMAGE_01.to_string();
        }

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Model: {}", req.model));
        print_verbose(cli, &format!("Prompt: {}", req.prompt));

        let client = create_client(&ctx)?;
        let resp = client.image().generate(&req).await?;

        print_success(&format!("Generated {} image(s)", resp.images.len()));

        output_result(&resp, cli.output.as_deref(), cli.json)
    }

    async fn reference(&self, cli: &Cli) -> anyhow::Result<()> {
        let input_file = require_input_file(cli)?;
        let ctx = get_context(cli)?;

        let mut req: ImageReferenceRequest = load_request(input_file)?;

        // Use defaults if not specified
        if req.model.is_empty() {
            req.model = MODEL_IMAGE_01.to_string();
        }

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Model: {}", req.model));

        let client = create_client(&ctx)?;
        let resp = client.image().generate_with_reference(&req).await?;

        print_success(&format!("Generated {} image(s)", resp.images.len()));

        output_result(&resp, cli.output.as_deref(), cli.json)
    }
}
