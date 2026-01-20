//! Realtime (Real-time voice conversation) commands.

use std::io::Write;
use std::time::Duration;

use clap::{Args, Subcommand};
use serde::{Deserialize, Serialize};
use tokio::time::timeout;

use giztoy_doubaospeech::{
    RealtimeASRConfig, RealtimeAudioConfig, RealtimeConfig, RealtimeDialogConfig,
    RealtimeEventType, RealtimeTTSConfig,
};

use super::{create_client, get_context, load_request, output_result, print_success, print_verbose};
use crate::Cli;

/// Realtime voice conversation service.
///
/// Enables bidirectional voice communication with AI.
#[derive(Args)]
pub struct RealtimeCommand {
    #[command(subcommand)]
    command: RealtimeSubcommand,
}

#[derive(Subcommand)]
enum RealtimeSubcommand {
    /// Connect to realtime service
    Connect,
    /// Test realtime connection (send greeting and receive response)
    Test {
        /// Greeting text to send
        #[arg(short = 'g', long, default_value = "你好")]
        greeting: String,
    },
}

/// Realtime config from YAML/JSON file.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct RealtimeFileConfig {
    #[serde(default)]
    asr: AsrConfig,
    #[serde(default)]
    tts: TtsConfig,
    #[serde(default)]
    dialog: DialogConfig,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct AsrConfig {
    #[serde(default)]
    extra: std::collections::HashMap<String, serde_json::Value>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct TtsConfig {
    #[serde(default)]
    speaker: String,
    #[serde(default)]
    audio_config: AudioConfig,
    #[serde(default)]
    extra: std::collections::HashMap<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct AudioConfig {
    #[serde(default = "default_channel")]
    channel: i32,
    #[serde(default = "default_format")]
    format: String,
    #[serde(default = "default_sample_rate")]
    sample_rate: i32,
}

impl Default for AudioConfig {
    fn default() -> Self {
        Self {
            channel: 1,
            format: "mp3".to_string(),
            sample_rate: 24000,
        }
    }
}

fn default_channel() -> i32 {
    1
}
fn default_format() -> String {
    "mp3".to_string()
}
fn default_sample_rate() -> i32 {
    24000
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct DialogConfig {
    #[serde(default)]
    bot_name: String,
    #[serde(default)]
    system_role: String,
    #[serde(default)]
    speaking_style: String,
    #[serde(default)]
    character_manifest: String,
    #[serde(default)]
    extra: std::collections::HashMap<String, serde_json::Value>,
}

impl RealtimeFileConfig {
    fn to_realtime_config(&self) -> RealtimeConfig {
        RealtimeConfig {
            asr: RealtimeASRConfig {
                extra: self.asr.extra.clone(),
            },
            tts: RealtimeTTSConfig {
                speaker: self.tts.speaker.clone(),
                audio_config: RealtimeAudioConfig {
                    channel: self.tts.audio_config.channel,
                    format: self.tts.audio_config.format.clone(),
                    sample_rate: self.tts.audio_config.sample_rate,
                },
                extra: self.tts.extra.clone(),
            },
            dialog: RealtimeDialogConfig {
                bot_name: self.dialog.bot_name.clone(),
                system_role: self.dialog.system_role.clone(),
                speaking_style: self.dialog.speaking_style.clone(),
                character_manifest: self.dialog.character_manifest.clone(),
                extra: self.dialog.extra.clone(),
                ..Default::default()
            },
        }
    }
}

impl RealtimeCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            RealtimeSubcommand::Connect => self.connect(cli).await,
            RealtimeSubcommand::Test { greeting } => self.test(cli, greeting).await,
        }
    }

    async fn connect(&self, cli: &Cli) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        let client = create_client(&ctx)?;

        // Load config from file
        let config = if let Some(input_file) = cli.input.as_deref() {
            let file_config: RealtimeFileConfig = load_request(input_file)?;
            file_config.to_realtime_config()
        } else {
            // Default config
            RealtimeConfig {
                tts: RealtimeTTSConfig {
                    speaker: "zh_female_cancan".to_string(),
                    audio_config: RealtimeAudioConfig::default(),
                    ..Default::default()
                },
                dialog: RealtimeDialogConfig {
                    bot_name: "助手".to_string(),
                    system_role: "你是一个友好的语音助手".to_string(),
                    ..Default::default()
                },
                ..Default::default()
            }
        };

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Speaker: {}", config.tts.speaker));

        // Connect to realtime service
        eprintln!("Connecting to realtime service...");
        let session = client.realtime().connect(&config).await?;

        print_success(&format!("Connected! Session ID: {}", session.session_id()));

        // Keep connection alive and print events
        eprintln!("Listening for events... (Press Ctrl+C to exit)");

        loop {
            match timeout(Duration::from_secs(30), session.recv()).await {
                Ok(Some(Ok(event))) => {
                    match event.event_type {
                        Some(RealtimeEventType::AudioReceived) => {
                            if let Some(ref audio) = event.audio {
                                eprint!("🔊 Audio received: {} bytes\r", audio.len());
                                std::io::stderr().flush().ok();
                            }
                        }
                        Some(RealtimeEventType::ASRFinished) => {
                            if let Some(ref info) = event.asr_info {
                                eprintln!("🎤 ASR: {}", info.text);
                            }
                        }
                        Some(RealtimeEventType::TTSStarted) => {
                            eprintln!("🗣️ TTS: {}", event.text);
                        }
                        Some(RealtimeEventType::SessionFailed) => {
                            if let Some(ref err) = event.error {
                                eprintln!("❌ Session failed: {}", err);
                            }
                            break;
                        }
                        Some(RealtimeEventType::SessionEnded) => {
                            eprintln!("Session ended");
                            break;
                        }
                        _ => {
                            print_verbose(cli, &format!("Event: {:?}", event.event_type));
                        }
                    }
                }
                Ok(Some(Err(e))) => {
                    eprintln!("Error: {}", e);
                    break;
                }
                Ok(None) => {
                    eprintln!("Connection closed");
                    break;
                }
                Err(_) => {
                    // Timeout, keep waiting
                    continue;
                }
            }
        }

        session.close().await?;
        Ok(())
    }

    async fn test(&self, cli: &Cli, greeting: &str) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        let client = create_client(&ctx)?;

        // Load config from file or use default
        let config = if let Some(input_file) = cli.input.as_deref() {
            let file_config: RealtimeFileConfig = load_request(input_file)?;
            file_config.to_realtime_config()
        } else {
            RealtimeConfig {
                tts: RealtimeTTSConfig {
                    speaker: "zh_female_cancan".to_string(),
                    audio_config: RealtimeAudioConfig::default(),
                    ..Default::default()
                },
                dialog: RealtimeDialogConfig {
                    bot_name: "助手".to_string(),
                    system_role: "你是一个友好的语音助手".to_string(),
                    ..Default::default()
                },
                ..Default::default()
            }
        };

        print_verbose(cli, &format!("Using context: {}", ctx.name));

        // Connect
        eprintln!("Connecting to realtime service...");
        let session = client.realtime().connect(&config).await?;
        print_success(&format!("Connected! Session ID: {}", session.session_id()));

        // Send greeting
        eprintln!("Sending greeting: {}", greeting);
        session.say_hello(greeting).await?;

        // Collect responses
        let mut audio_bytes = 0usize;
        let mut response_text = String::new();

        eprintln!("Waiting for response...");

        loop {
            match timeout(Duration::from_secs(10), session.recv()).await {
                Ok(Some(Ok(event))) => {
                    match event.event_type {
                        Some(RealtimeEventType::AudioReceived) => {
                            if let Some(ref audio) = event.audio {
                                audio_bytes += audio.len();
                            }
                        }
                        Some(RealtimeEventType::TTSStarted) => {
                            response_text = event.text.clone();
                            eprintln!("🗣️ Response: {}", event.text);
                        }
                        Some(RealtimeEventType::TTSFinished) => {
                            break;
                        }
                        Some(RealtimeEventType::SessionFailed) => {
                            if let Some(ref err) = event.error {
                                return Err(anyhow::anyhow!("Session failed: {}", err));
                            }
                            break;
                        }
                        _ => {}
                    }
                }
                Ok(Some(Err(e))) => {
                    return Err(anyhow::anyhow!("Error: {}", e));
                }
                Ok(None) => {
                    break;
                }
                Err(_) => {
                    // Timeout
                    break;
                }
            }
        }

        session.close().await?;

        let result = serde_json::json!({
            "session_id": session.session_id(),
            "greeting": greeting,
            "response_text": response_text,
            "audio_bytes": audio_bytes,
        });

        output_result(&result, cli.output.as_deref(), cli.json)
    }
}
