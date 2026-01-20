//! Voice Clone CLI commands

use clap::{Args, Subcommand};
use giztoy_doubaospeech::{Language, VoiceCloneModelType, VoiceCloneTrainRequest};

use super::{create_client, get_context, load_request, output_result, print_info, print_success};
use crate::Cli;

/// Voice clone commands
#[derive(Args)]
pub struct VoiceCloneCommand {
    #[command(subcommand)]
    command: VoiceCloneSubcommand,
}

#[derive(Subcommand)]
enum VoiceCloneSubcommand {
    /// Train a custom voice from audio file
    Train {
        /// Audio file path (wav, mp3, etc.)
        #[arg(short, long)]
        audio: String,

        /// Speaker ID for the cloned voice
        #[arg(short, long)]
        speaker_id: String,

        /// Model type (standard or pro)
        #[arg(short, long, default_value = "standard")]
        model: String,

        /// Language (zh, en, ja, etc.)
        #[arg(short, long)]
        language: Option<String>,
    },
    /// Get voice clone training status
    Status {
        /// Speaker ID to query
        #[arg(short, long)]
        speaker_id: String,
    },
    /// List trained voices
    List,
}

impl VoiceCloneCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            VoiceCloneSubcommand::Train {
                audio,
                speaker_id,
                model,
                language,
            } => {
                self.train_voice(cli, audio, speaker_id, model, language.as_deref()).await
            }
            VoiceCloneSubcommand::Status { speaker_id } => {
                self.get_status(cli, speaker_id).await
            }
            VoiceCloneSubcommand::List => {
                self.list_voices(cli).await
            }
        }
    }

    async fn train_voice(
        &self,
        cli: &Cli,
        audio_path: &str,
        speaker_id: &str,
        model: &str,
        language: Option<&str>,
    ) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        let client = create_client(&ctx)?;

        // Load base request from file if provided
        let mut req: VoiceCloneTrainRequest = if let Some(ref path) = cli.input {
            load_request(path)?
        } else {
            VoiceCloneTrainRequest::default()
        };

        // Override with CLI arguments
        req.speaker_id = speaker_id.to_string();
        req.model_type = match model {
            "pro" => VoiceCloneModelType::Pro,
            _ => VoiceCloneModelType::Standard,
        };
        if let Some(lang) = language {
            req.language = Some(match lang {
                "en" | "en-us" => Language::EnUs,
                "en-gb" => Language::EnGb,
                "ja" | "ja-jp" => Language::JaJp,
                _ => Language::ZhCn,
            });
        }

        if cli.verbose {
            print_info(&format!("Training voice from: {}", audio_path));
            print_info(&format!("Speaker ID: {}", req.speaker_id));
            print_info(&format!("Model: {:?}", req.model_type));
        }

        // Read audio file
        let audio_data = std::fs::read(audio_path)?;
        req.audio_data = Some(audio_data);

        let voice_clone = client.voice_clone();
        let result = voice_clone.train(&req).await?;

        if cli.verbose {
            print_success("Voice clone training started");
        }

        output_result(&result, cli.output.as_deref(), cli.json)?;

        Ok(())
    }

    async fn get_status(&self, cli: &Cli, speaker_id: &str) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        let client = create_client(&ctx)?;

        if cli.verbose {
            print_info(&format!("Querying status for speaker: {}", speaker_id));
        }

        let voice_clone = client.voice_clone();
        let status = voice_clone.get_status(speaker_id).await?;

        if cli.verbose {
            print_success(&format!("Status: {:?}", status.status));
        }

        output_result(&status, cli.output.as_deref(), cli.json)?;

        Ok(())
    }

    async fn list_voices(&self, cli: &Cli) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;

        if cli.verbose {
            print_info("Listing trained voices");
        }

        // TODO: Implement voice list via Console API
        eprintln!("[Voice list not implemented yet]");
        eprintln!("Use Console API to list trained voices");

        let result = serde_json::json!({
            "_note": "Voice list not implemented yet",
            "context": ctx.name,
        });
        output_result(&result, cli.output.as_deref(), cli.json)
    }
}
