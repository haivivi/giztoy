//! Qwen-Omni-Realtime multimodal conversation commands.

use std::io::Read;
use std::time::Duration;

use clap::{Args, Subcommand};
use serde::{Deserialize, Serialize};
use tokio::time::timeout;

use giztoy_dashscope::{
    RealtimeConfig, SessionConfig, TurnDetection,
    MODEL_QWEN_OMNI_TURBO_REALTIME_LATEST, VOICE_CHELSIE,
    VAD_MODE_SERVER_VAD, AUDIO_FORMAT_PCM16, MODALITY_TEXT, MODALITY_AUDIO,
    EVENT_TYPE_SESSION_CREATED, EVENT_TYPE_SESSION_UPDATED,
    EVENT_TYPE_CHOICES_RESPONSE, EVENT_TYPE_RESPONSE_DONE,
    EVENT_TYPE_RESPONSE_TEXT_DELTA, EVENT_TYPE_RESPONSE_AUDIO_DELTA,
    EVENT_TYPE_RESPONSE_AUDIO_DONE, EVENT_TYPE_ERROR,
    EVENT_TYPE_INPUT_SPEECH_STARTED, EVENT_TYPE_INPUT_SPEECH_STOPPED,
    EVENT_TYPE_INPUT_AUDIO_COMMITTED, EVENT_TYPE_RESPONSE_CREATED,
};

use super::{
    create_client, get_context, load_request, output_bytes,
    print_error, print_info, print_success, print_verbose, print_warning,
};
use crate::Cli;

/// Qwen-Omni-Realtime multimodal conversation service.
#[derive(Args)]
pub struct OmniCommand {
    #[command(subcommand)]
    command: OmniSubcommand,
}

#[derive(Subcommand)]
enum OmniSubcommand {
    /// Start an interactive chat session
    Chat {
        /// Model to use (overrides config file)
        #[arg(long)]
        model: Option<String>,
        /// Voice for audio output (overrides config file)
        #[arg(long)]
        voice: Option<String>,
        /// Input audio file (overrides config file)
        #[arg(long)]
        audio: Option<String>,
        /// System instructions (overrides config file)
        #[arg(long)]
        instructions: Option<String>,
    },
}

/// Configuration for omni chat command.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OmniChatConfig {
    /// Model ID to use.
    #[serde(default)]
    pub model: String,

    /// Voice ID for TTS output.
    #[serde(default)]
    pub voice: String,

    /// System instructions.
    #[serde(default)]
    pub instructions: String,

    /// Input audio format.
    #[serde(default)]
    pub input_audio_format: String,

    /// Output audio format.
    #[serde(default)]
    pub output_audio_format: String,

    /// Output modalities.
    #[serde(default)]
    pub modalities: Vec<String>,

    /// Enable input audio transcription.
    #[serde(default)]
    pub enable_input_audio_transcription: bool,

    /// Model for input transcription.
    #[serde(default)]
    pub input_audio_transcription_model: String,

    /// Turn detection configuration.
    #[serde(default)]
    pub turn_detection: Option<TurnDetectionConfig>,

    /// Input audio file path.
    #[serde(default)]
    pub audio_file: String,
}

/// Turn detection configuration.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TurnDetectionConfig {
    #[serde(rename = "type", default)]
    pub detection_type: String,
    #[serde(default)]
    pub threshold: f64,
    #[serde(default)]
    pub prefix_padding_ms: i32,
    #[serde(default)]
    pub silence_duration_ms: i32,
}

impl Default for OmniChatConfig {
    fn default() -> Self {
        Self {
            model: MODEL_QWEN_OMNI_TURBO_REALTIME_LATEST.to_string(),
            voice: VOICE_CHELSIE.to_string(),
            instructions: String::new(),
            input_audio_format: AUDIO_FORMAT_PCM16.to_string(),
            output_audio_format: AUDIO_FORMAT_PCM16.to_string(),
            modalities: vec![MODALITY_TEXT.to_string(), MODALITY_AUDIO.to_string()],
            enable_input_audio_transcription: true,
            input_audio_transcription_model: "gummy-realtime-v1".to_string(),
            turn_detection: Some(TurnDetectionConfig {
                detection_type: VAD_MODE_SERVER_VAD.to_string(),
                threshold: 0.5,
                prefix_padding_ms: 300,
                silence_duration_ms: 800,
            }),
            audio_file: String::new(),
        }
    }
}

impl OmniChatConfig {
    /// Converts to SessionConfig.
    pub fn to_session_config(&self) -> SessionConfig {
        let mut cfg = SessionConfig {
            voice: Some(self.voice.clone()),
            input_audio_format: Some(self.input_audio_format.clone()),
            output_audio_format: Some(self.output_audio_format.clone()),
            modalities: Some(self.modalities.clone()),
            instructions: if self.instructions.is_empty() {
                None
            } else {
                Some(self.instructions.clone())
            },
            enable_input_audio_transcription: Some(self.enable_input_audio_transcription),
            input_audio_transcription_model: if self.input_audio_transcription_model.is_empty() {
                None
            } else {
                Some(self.input_audio_transcription_model.clone())
            },
            ..Default::default()
        };

        if let Some(ref td) = self.turn_detection {
            cfg.turn_detection = Some(TurnDetection {
                detection_type: Some(td.detection_type.clone()),
                threshold: if td.threshold > 0.0 { Some(td.threshold) } else { None },
                prefix_padding_ms: if td.prefix_padding_ms > 0 { Some(td.prefix_padding_ms) } else { None },
                silence_duration_ms: if td.silence_duration_ms > 0 { Some(td.silence_duration_ms) } else { None },
            });
        }

        cfg
    }
}

impl OmniCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            OmniSubcommand::Chat {
                model,
                voice,
                audio,
                instructions,
            } => {
                run_omni_chat(cli, model.as_deref(), voice.as_deref(), audio.as_deref(), instructions.as_deref()).await
            }
        }
    }
}

async fn run_omni_chat(
    cli: &Cli,
    model_override: Option<&str>,
    voice_override: Option<&str>,
    audio_override: Option<&str>,
    instructions_override: Option<&str>,
) -> anyhow::Result<()> {
    let ctx = get_context(cli)?;

    // Load config: start with defaults, merge file config, then apply flag overrides
    let mut config = OmniChatConfig::default();

    // Load from file if specified
    if let Some(ref input_file) = cli.input {
        let file_config: OmniChatConfig = load_request(input_file)?;
        merge_config(&mut config, &file_config);
        print_verbose(cli, &format!("Loaded config from: {}", input_file));
    }

    // Apply flag overrides
    if let Some(m) = model_override {
        config.model = m.to_string();
    }
    if let Some(v) = voice_override {
        config.voice = v.to_string();
    }
    if let Some(a) = audio_override {
        config.audio_file = a.to_string();
    }
    if let Some(i) = instructions_override {
        config.instructions = i.to_string();
    }

    print_verbose(cli, &format!("Using context: {}", ctx.name));
    print_verbose(cli, &format!("Model: {}", config.model));
    print_verbose(cli, &format!("Voice: {}", config.voice));

    // Create client
    let client = create_client(&ctx)?;

    // Connect to realtime session
    let realtime_config = RealtimeConfig {
        model: config.model.clone(),
    };

    print_info("Connecting to Qwen-Omni-Realtime...");
    let mut session = timeout(
        Duration::from_secs(30),
        client.realtime().connect(&realtime_config),
    )
    .await
    .map_err(|_| anyhow::anyhow!("Connection timeout"))??;

    print_success("Connected to Qwen-Omni-Realtime");

    // Wait for session.created
    print_verbose(cli, "Waiting for session.created...");
    let created = wait_for_event(&mut session, EVENT_TYPE_SESSION_CREATED, Duration::from_secs(5)).await?;
    print_verbose(cli, &format!("Session created: {:?}", created.session));

    // Update session configuration
    print_verbose(cli, "Updating session config...");
    session.update_session(&config.to_session_config()).await?;

    // Wait for session.updated
    let _ = wait_for_event(&mut session, EVENT_TYPE_SESSION_UPDATED, Duration::from_secs(5)).await;
    print_verbose(cli, "Session updated");

    // Audio file mode
    if !config.audio_file.is_empty() {
        send_audio_file(&mut session, &config.audio_file, cli).await?;
        
        // Handle events and collect audio
        let audio_chunks = handle_events(&mut session, cli).await?;
        
        // Save audio if output specified
        if let Some(ref output_path) = cli.output {
            if !audio_chunks.is_empty() {
                let total_size: usize = audio_chunks.iter().map(|c| c.len()).sum();
                let mut audio = Vec::with_capacity(total_size);
                for chunk in audio_chunks {
                    audio.extend(chunk);
                }
                output_bytes(&audio, output_path)?;
                print_success(&format!("Audio saved to: {} ({} bytes)", output_path, total_size));
            }
        }

        session.close().await?;
        return Ok(());
    }

    // Interactive mode
    println!("\nInteractive mode (VAD enabled).");
    println!("Commands:");
    println!("  /audio <file>  - Send audio file (16-bit PCM, 16kHz)");
    println!("  /voice <id>    - Change voice");
    println!("  /clear         - Clear input buffer");
    println!("  /exit          - End session");
    println!();

    // Simple interactive loop
    use std::io::{BufRead, Write};
    let stdin = std::io::stdin();
    let mut stdout = std::io::stdout();

    loop {
        print!("> ");
        stdout.flush()?;

        let mut input = String::new();
        if stdin.lock().read_line(&mut input)? == 0 {
            break;
        }

        let input = input.trim();
        if input.is_empty() {
            continue;
        }

        if input.starts_with('/') {
            let parts: Vec<&str> = input.splitn(2, ' ').collect();
            let cmd = parts[0].to_lowercase();

            match cmd.as_str() {
                "/exit" | "/quit" => {
                    println!("Goodbye!");
                    break;
                }
                "/audio" => {
                    if parts.len() < 2 {
                        print_error("Usage: /audio <filepath>");
                    } else {
                        if let Err(e) = send_audio_file(&mut session, parts[1], cli).await {
                            print_error(&format!("Failed to send audio: {}", e));
                        }
                    }
                }
                "/clear" => {
                    if let Err(e) = session.clear_input().await {
                        print_error(&format!("Failed to clear: {}", e));
                    } else {
                        print_info("Input buffer cleared");
                    }
                }
                "/voice" => {
                    if parts.len() < 2 {
                        print_info(&format!(
                            "Available voices: {}, {}, {}, {}",
                            VOICE_CHELSIE,
                            giztoy_dashscope::VOICE_CHERRY,
                            giztoy_dashscope::VOICE_SERENA,
                            giztoy_dashscope::VOICE_ETHAN
                        ));
                    } else {
                        let new_voice = parts[1];
                        let update_config = SessionConfig {
                            voice: Some(new_voice.to_string()),
                            ..Default::default()
                        };
                        if let Err(e) = session.update_session(&update_config).await {
                            print_error(&format!("Failed to change voice: {}", e));
                        } else {
                            print_success(&format!("Voice changed to: {}", new_voice));
                        }
                    }
                }
                "/help" => {
                    println!("Commands:");
                    println!("  /audio <file> - Send audio file (16-bit PCM, 16kHz)");
                    println!("  /clear        - Clear input buffer");
                    println!("  /voice <id>   - Change voice");
                    println!("  /exit, /quit  - End session");
                    println!("  /help         - Show this help");
                }
                _ => {
                    print_error(&format!("Unknown command: {} (try /help)", cmd));
                }
            }
        } else {
            // Treat as audio file path
            if let Err(e) = send_audio_file(&mut session, input, cli).await {
                print_error(&format!("Failed to send: {}", e));
            }
        }
    }

    session.close().await?;
    Ok(())
}

fn merge_config(dst: &mut OmniChatConfig, src: &OmniChatConfig) {
    if !src.model.is_empty() {
        dst.model = src.model.clone();
    }
    if !src.voice.is_empty() {
        dst.voice = src.voice.clone();
    }
    if !src.instructions.is_empty() {
        dst.instructions = src.instructions.clone();
    }
    if !src.input_audio_format.is_empty() {
        dst.input_audio_format = src.input_audio_format.clone();
    }
    if !src.output_audio_format.is_empty() {
        dst.output_audio_format = src.output_audio_format.clone();
    }
    if !src.modalities.is_empty() {
        dst.modalities = src.modalities.clone();
    }
    if src.enable_input_audio_transcription {
        dst.enable_input_audio_transcription = src.enable_input_audio_transcription;
    }
    if !src.input_audio_transcription_model.is_empty() {
        dst.input_audio_transcription_model = src.input_audio_transcription_model.clone();
    }
    if src.turn_detection.is_some() {
        dst.turn_detection = src.turn_detection.clone();
    }
    if !src.audio_file.is_empty() {
        dst.audio_file = src.audio_file.clone();
    }
}

async fn wait_for_event(
    session: &mut giztoy_dashscope::RealtimeSession,
    event_type: &str,
    timeout_duration: Duration,
) -> anyhow::Result<giztoy_dashscope::RealtimeEvent> {
    let start = std::time::Instant::now();
    loop {
        if start.elapsed() > timeout_duration {
            anyhow::bail!("Timeout waiting for {}", event_type);
        }

        match timeout(Duration::from_millis(100), session.recv()).await {
            Ok(Some(Ok(event))) => {
                if event.event_type == event_type {
                    return Ok(event);
                }
            }
            Ok(Some(Err(e))) => {
                return Err(e.into());
            }
            Ok(None) => {
                anyhow::bail!("Session closed while waiting for {}", event_type);
            }
            Err(_) => {
                // Timeout on recv, continue waiting
            }
        }
    }
}

async fn send_audio_file(
    session: &mut giztoy_dashscope::RealtimeSession,
    filepath: &str,
    cli: &Cli,
) -> anyhow::Result<()> {
    let mut file = std::fs::File::open(filepath)?;
    print_info(&format!("Sending audio file: {}", filepath));

    // Read and send in chunks (16kHz * 2 bytes * 0.1s = 3200 bytes per chunk)
    const CHUNK_SIZE: usize = 3200;
    let mut buf = vec![0u8; CHUNK_SIZE];
    let mut total_bytes = 0;

    loop {
        let n = file.read(&mut buf)?;
        if n == 0 {
            break;
        }

        session.append_audio(&buf[..n]).await?;
        total_bytes += n;

        // Small delay to simulate real-time streaming
        tokio::time::sleep(Duration::from_millis(100)).await;
    }

    print_info(&format!("Sent {} bytes of audio", total_bytes));
    Ok(())
}

async fn handle_events(
    session: &mut giztoy_dashscope::RealtimeSession,
    cli: &Cli,
) -> anyhow::Result<Vec<Vec<u8>>> {
    let mut audio_chunks = Vec::new();
    let mut current_text = String::new();
    let mut last_text = String::new();

    // Wait for response with timeout
    let timeout_duration = Duration::from_secs(60);
    let start = std::time::Instant::now();

    loop {
        if start.elapsed() > timeout_duration {
            print_warning("Timeout waiting for response");
            break;
        }

        match timeout(Duration::from_millis(100), session.recv()).await {
            Ok(Some(Ok(event))) => {
                match event.event_type.as_str() {
                    EVENT_TYPE_RESPONSE_CREATED => {
                        print_verbose(cli, "Response started");
                        current_text.clear();
                        last_text.clear();
                        audio_chunks.clear();
                    }
                    EVENT_TYPE_CHOICES_RESPONSE => {
                        // DashScope "choices" format response
                        if let Some(ref delta) = event.delta {
                            if delta.len() > last_text.len() {
                                print!("{}", &delta[last_text.len()..]);
                                std::io::Write::flush(&mut std::io::stdout())?;
                                last_text = delta.clone();
                            }
                        }
                        if let Some(audio) = event.audio {
                            audio_chunks.push(audio);
                        }
                        if event.finish_reason.is_some() {
                            println!();
                            break;
                        }
                    }
                    EVENT_TYPE_RESPONSE_TEXT_DELTA => {
                        if let Some(ref delta) = event.delta {
                            print!("{}", delta);
                            std::io::Write::flush(&mut std::io::stdout())?;
                            current_text.push_str(delta);
                        }
                    }
                    EVENT_TYPE_RESPONSE_AUDIO_DELTA => {
                        if let Some(audio) = event.audio {
                            audio_chunks.push(audio);
                        }
                    }
                    EVENT_TYPE_RESPONSE_AUDIO_DONE => {
                        let total_size: usize = audio_chunks.iter().map(|c| c.len()).sum();
                        print_verbose(cli, &format!("Audio received: {} bytes", total_size));
                    }
                    EVENT_TYPE_RESPONSE_DONE => {
                        if !current_text.is_empty() {
                            println!();
                        }
                        if let Some(ref usage) = event.usage {
                            print_verbose(cli, &format!(
                                "Tokens - Input: {:?}, Output: {:?}",
                                usage.input_tokens, usage.output_tokens
                            ));
                        }
                        break;
                    }
                    EVENT_TYPE_ERROR => {
                        if let Some(ref err) = event.error {
                            print_error(&format!(
                                "Error: {:?} - {:?}",
                                err.code, err.message
                            ));
                        }
                        break;
                    }
                    EVENT_TYPE_INPUT_SPEECH_STARTED => {
                        print_verbose(cli, "Speech started");
                    }
                    EVENT_TYPE_INPUT_SPEECH_STOPPED => {
                        print_verbose(cli, "Speech stopped");
                    }
                    EVENT_TYPE_INPUT_AUDIO_COMMITTED => {
                        print_verbose(cli, "Audio committed");
                    }
                    _ => {
                        print_verbose(cli, &format!("Event: {}", event.event_type));
                    }
                }
            }
            Ok(Some(Err(e))) => {
                print_error(&format!("Event error: {}", e));
                break;
            }
            Ok(None) => {
                break;
            }
            Err(_) => {
                // Timeout on recv, continue
            }
        }
    }

    Ok(audio_chunks)
}
