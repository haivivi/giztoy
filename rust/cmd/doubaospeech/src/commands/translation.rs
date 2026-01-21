//! Translation (simultaneous interpretation) CLI commands

use clap::{Args, Subcommand};
use serde::{Deserialize, Serialize};

use giztoy_doubaospeech::{
    AudioFormat, Language, SampleRate, TranslationAudioConfig, TranslationConfig,
};

use super::{create_client, get_context, load_request, output_result, print_success, print_verbose};
use crate::Cli;

/// Simultaneous translation service
#[derive(Args)]
pub struct TranslationCommand {
    #[command(subcommand)]
    command: TranslationSubcommand,
}

#[derive(Subcommand)]
enum TranslationSubcommand {
    /// Stream simultaneous translation
    Stream {
        /// Audio file path
        #[arg(short = 'a', long)]
        audio: Option<String>,
        /// Source language
        #[arg(short = 's', long, default_value = "zh-CN")]
        source_lang: String,
        /// Target language
        #[arg(short = 't', long, default_value = "en-US")]
        target_lang: String,
        /// Enable TTS output
        #[arg(long)]
        enable_tts: bool,
    },
    /// Interactive translation mode
    Interactive {
        /// Source language
        #[arg(short = 's', long, default_value = "zh-CN")]
        source_lang: String,
        /// Target language
        #[arg(short = 't', long, default_value = "en-US")]
        target_lang: String,
    },
}

/// Translation config from YAML/JSON file
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[allow(dead_code)]
struct TranslationFileConfig {
    #[serde(skip_serializing_if = "Option::is_none")]
    audio_file: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    source_language: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    target_language: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    sample_rate: Option<i32>,
}

impl TranslationCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            TranslationSubcommand::Stream {
                audio,
                source_lang,
                target_lang,
                enable_tts,
            } => {
                self.stream(cli, audio.as_deref(), source_lang, target_lang, *enable_tts)
                    .await
            }
            TranslationSubcommand::Interactive {
                source_lang,
                target_lang,
            } => self.interactive(cli, source_lang, target_lang).await,
        }
    }

    async fn stream(
        &self,
        cli: &Cli,
        audio_path: Option<&str>,
        source_lang: &str,
        target_lang: &str,
        enable_tts: bool,
    ) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        let client = create_client(&ctx)?;
        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(
            cli,
            &format!("Source: {} -> Target: {}", source_lang, target_lang),
        );

        // Load audio file
        let audio_file = if let Some(input_file) = cli.input.as_deref() {
            let file_cfg: TranslationFileConfig = load_request(input_file)?;
            audio_path
                .map(|s| s.to_string())
                .or(file_cfg.audio_file)
                .ok_or_else(|| anyhow::anyhow!("audio file is required"))?
        } else {
            audio_path
                .ok_or_else(|| anyhow::anyhow!("audio file is required, use -a flag"))?
                .to_string()
        };

        let audio_data = std::fs::read(&audio_file)?;
        print_verbose(
            cli,
            &format!("Audio file: {} ({} bytes)", audio_file, audio_data.len()),
        );

        // Parse languages
        let source_language = match source_lang {
            "zh-CN" => Language::ZhCn,
            "en-US" => Language::EnUs,
            "ja-JP" => Language::JaJp,
            _ => Language::ZhCn,
        };
        let target_language = match target_lang {
            "zh-CN" => Language::ZhCn,
            "en-US" => Language::EnUs,
            "ja-JP" => Language::JaJp,
            _ => Language::EnUs,
        };

        let config = TranslationConfig {
            source_language,
            target_language,
            audio_config: TranslationAudioConfig {
                format: AudioFormat::Pcm,
                sample_rate: SampleRate::Rate16000,
                channel: 1,
                bits: 16,
            },
            enable_tts,
            tts_voice: None,
        };

        print_verbose(cli, "Opening translation session...");
        let session = client.translation().open_session(&config).await?;

        // Send audio in chunks
        let chunk_size = 3200; // 100ms of 16kHz 16-bit mono audio
        let chunks: Vec<&[u8]> = audio_data.chunks(chunk_size).collect();
        let total_chunks = chunks.len();

        for (i, chunk) in chunks.iter().enumerate() {
            let is_last = i == total_chunks - 1;
            session.send_audio(chunk, is_last).await?;
            if !is_last {
                tokio::time::sleep(tokio::time::Duration::from_millis(100)).await;
            }
        }
        print_verbose(cli, &format!("Sent {} audio chunks", total_chunks));

        // Receive results
        let mut translations = Vec::new();
        while let Some(result) = session.recv().await {
            match result {
                Ok(chunk) => {
                    if !chunk.source_text.is_empty() || !chunk.target_text.is_empty() {
                        print_verbose(
                            cli,
                            &format!("{} -> {}", chunk.source_text, chunk.target_text),
                        );
                        translations.push(serde_json::json!({
                            "source": chunk.source_text,
                            "target": chunk.target_text,
                            "is_final": chunk.is_final,
                        }));
                    }
                    if chunk.is_final {
                        break;
                    }
                }
                Err(e) => {
                    return Err(anyhow::anyhow!("Translation error: {}", e));
                }
            }
        }

        session.close().await?;
        print_success("Translation completed");

        let result = serde_json::json!({
            "translations": translations,
        });
        output_result(&result, cli.output.as_deref(), cli.json)
    }

    async fn interactive(
        &self,
        cli: &Cli,
        source_lang: &str,
        target_lang: &str,
    ) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        print_verbose(cli, &format!("Using context: {}", ctx.name));

        // Interactive mode requires microphone input which is complex
        // For now, provide guidance
        eprintln!("Interactive translation mode requires microphone input.");
        eprintln!("Use 'stream' subcommand with an audio file instead:");
        eprintln!(
            "  doubaospeech translation stream -a audio.pcm -s {} -t {}",
            source_lang, target_lang
        );

        let result = serde_json::json!({
            "note": "Use 'stream' subcommand with audio file",
            "source_language": source_lang,
            "target_language": target_lang,
        });
        output_result(&result, cli.output.as_deref(), cli.json)
    }
}
