//! Text generation commands.
//!
//! Compatible with Go version's text commands.

use std::io::Write;
use std::pin::pin;

use clap::{Args, Subcommand};
use futures::StreamExt;

use giztoy_minimax::{ChatCompletionRequest, MODEL_M2_1};

use super::{
    create_client, get_context, load_request, output_result, print_verbose, require_input_file,
};
use crate::Cli;

/// Text generation service.
///
/// Supports chat completions with streaming.
#[derive(Args)]
pub struct TextCommand {
    #[command(subcommand)]
    command: TextSubcommand,
}

#[derive(Subcommand)]
enum TextSubcommand {
    /// Create a chat completion
    Chat,
    /// Create a streaming chat completion
    Stream,
}

impl TextCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            TextSubcommand::Chat => self.chat(cli).await,
            TextSubcommand::Stream => self.stream(cli).await,
        }
    }

    async fn chat(&self, cli: &Cli) -> anyhow::Result<()> {
        let input_file = require_input_file(cli)?;
        let ctx = get_context(cli)?;

        let mut req: ChatCompletionRequest = load_request(input_file)?;

        // Use defaults if not specified
        if req.model.is_empty() {
            req.model = MODEL_M2_1.to_string();
        }

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Model: {}", req.model));
        print_verbose(cli, &format!("Messages: {}", req.messages.len()));

        let client = create_client(&ctx)?;
        let resp = client.text().create_chat_completion(&req).await?;

        // Output result
        output_result(&resp, cli.output.as_deref(), cli.json)
    }

    async fn stream(&self, cli: &Cli) -> anyhow::Result<()> {
        let input_file = require_input_file(cli)?;
        let ctx = get_context(cli)?;

        let mut req: ChatCompletionRequest = load_request(input_file)?;

        // Use defaults if not specified
        if req.model.is_empty() {
            req.model = MODEL_M2_1.to_string();
        }

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Model: {}", req.model));

        let client = create_client(&ctx)?;

        // Streaming output
        if cli.json {
            // JSON mode: output each chunk as JSON
            let stream = client.text().create_chat_completion_stream(&req).await?;
            let mut stream = pin!(stream);

            while let Some(chunk) = stream.next().await {
                let chunk = chunk?;
                let json = serde_json::to_string(&chunk)?;
                println!("{}", json);
            }
        } else {
            // Text mode: output content directly
            let stream = client.text().create_chat_completion_stream(&req).await?;
            let mut stream = pin!(stream);

            while let Some(chunk) = stream.next().await {
                let chunk = chunk?;
                if let Some(choice) = chunk.choices.first() {
                    if let Some(content) = &choice.delta.content {
                        print!("{}", content);
                        std::io::stdout().flush()?;
                    }
                }
            }
            println!();
        }

        Ok(())
    }
}
