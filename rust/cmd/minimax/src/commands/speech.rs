//! Speech synthesis commands.
//!
//! Compatible with Go version's speech commands.

use std::pin::pin;

use clap::{Args, Subcommand};
use futures::StreamExt;

use giztoy_minimax::{AsyncSpeechRequest, SpeechRequest, MODEL_SPEECH_26_HD};

use super::{
    create_client, format_bytes, get_context, load_request, output_bytes, output_result,
    print_success, print_verbose, require_input_file,
};
use crate::Cli;

/// Speech synthesis service.
///
/// Supports synchronous and asynchronous speech synthesis.
#[derive(Args)]
pub struct SpeechCommand {
    #[command(subcommand)]
    command: SpeechSubcommand,
}

#[derive(Subcommand)]
enum SpeechSubcommand {
    /// Synthesize speech from text
    Synthesize,
    /// Stream speech synthesis
    Stream,
    /// Create async speech synthesis task
    Async,
}

impl SpeechCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            SpeechSubcommand::Synthesize => self.synthesize(cli).await,
            SpeechSubcommand::Stream => self.stream(cli).await,
            SpeechSubcommand::Async => self.create_async(cli).await,
        }
    }

    async fn synthesize(&self, cli: &Cli) -> anyhow::Result<()> {
        let input_file = require_input_file(cli)?;
        let ctx = get_context(cli)?;

        let mut req: SpeechRequest = load_request(input_file)?;

        // Use defaults if not specified
        if req.model.is_empty() {
            req.model = MODEL_SPEECH_26_HD.to_string();
        }
        if let Some(ref mut voice) = req.voice_setting {
            if voice.voice_id.is_empty() {
                if let Some(default_voice) = ctx.get_extra("default_voice") {
                    voice.voice_id = default_voice.to_string();
                }
            }
        }

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Model: {}", req.model));
        print_verbose(cli, &format!("Text length: {} characters", req.text.len()));

        let client = create_client(&ctx)?;
        let resp = client.speech().synthesize(&req).await?;

        // Output audio to file if specified
        let output_path = cli.output.as_deref();
        if let Some(path) = output_path {
            if !resp.audio.is_empty() {
                output_bytes(&resp.audio, path)?;
                print_verbose(cli, &format!("Audio saved to: {}", path));
            }
        }

        // Output result
        let result = serde_json::json!({
            "audio_size": resp.audio.len(),
            "audio_url": resp.audio_url,
            "trace_id": resp.trace_id,
            "extra_info": resp.extra_info,
            "output_file": output_path,
        });

        output_result(&result, None, cli.json)
    }

    async fn stream(&self, cli: &Cli) -> anyhow::Result<()> {
        let input_file = require_input_file(cli)?;
        let output_path = cli
            .output
            .as_deref()
            .ok_or_else(|| anyhow::anyhow!("output file is required for streaming audio, use -o flag"))?;

        let ctx = get_context(cli)?;

        let mut req: SpeechRequest = load_request(input_file)?;

        // Use defaults if not specified
        if req.model.is_empty() {
            req.model = MODEL_SPEECH_26_HD.to_string();
        }

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Streaming to: {}", output_path));

        let client = create_client(&ctx)?;

        let mut audio_buf = Vec::new();
        let mut last_trace_id = None;
        let mut last_extra_info = None;

        let stream = client.speech().synthesize_stream(&req).await?;
        let mut stream = pin!(stream);

        while let Some(chunk) = stream.next().await {
            let chunk = chunk?;
            if !chunk.audio.is_empty() {
                audio_buf.extend_from_slice(&chunk.audio);
            }
            if chunk.trace_id.is_some() {
                last_trace_id = chunk.trace_id;
            }
            if chunk.extra_info.is_some() {
                last_extra_info = chunk.extra_info;
            }
        }

        // Write audio to file
        output_bytes(&audio_buf, output_path)?;
        print_success(&format!(
            "Audio saved to: {} ({})",
            output_path,
            format_bytes(audio_buf.len())
        ));

        // Output final info
        let result = serde_json::json!({
            "audio_size": audio_buf.len(),
            "extra_info": last_extra_info,
            "trace_id": last_trace_id,
            "output_file": output_path,
        });

        output_result(&result, None, cli.json)
    }

    async fn create_async(&self, cli: &Cli) -> anyhow::Result<()> {
        let input_file = require_input_file(cli)?;
        let ctx = get_context(cli)?;

        let mut req: AsyncSpeechRequest = load_request(input_file)?;

        // Use defaults if not specified
        if req.model.is_empty() {
            req.model = MODEL_SPEECH_26_HD.to_string();
        }

        let text_len = req.text.as_ref().map(|t| t.len()).unwrap_or(0);
        
        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Text length: {} characters", text_len));

        let client = create_client(&ctx)?;
        let task = client.speech().create_async_task(&req).await?;

        print_success(&format!("Async task created: {}", task.id()));

        let result = serde_json::json!({
            "task_id": task.id(),
            "status": "created",
        });

        output_result(&result, cli.output.as_deref(), cli.json)
    }
}
