//! Dual Chat Example - Two OpenAI Realtime agents chatting with each other.
//!
//! This example demonstrates:
//! - Creating two WebSocket Realtime sessions
//! - Different input/output modes: t2t, t2a, a2t, a2a
//! - Multi-turn conversation between AI agents
//! - Using MiniMax TTS for audio input generation
//! - VAD mode with pcm::Mixer for real-time audio streaming
//!
//! Usage:
//!   cargo run --bin dual_chat -- --api-key "sk-xxx" --mode t2t
//!   cargo run --bin dual_chat -- --api-key "sk-xxx" --mode a2a --rounds 10 --minimax-key "xxx"
//!   cargo run --bin dual_chat -- --api-key "sk-xxx" --mode a2a --vad --vad-type semantic_vad

use std::io::Read;
use std::sync::Arc;
use std::time::Duration;

use clap::Parser;
use giztoy_audio::pcm::{Format, Mixer, MixerOptions};
use giztoy_minimax as minimax;
use giztoy_openai_realtime::{
    self as openai, Client, Session, SessionConfig, TurnDetection, AUDIO_FORMAT_PCM16,
    EVENT_TYPE_RESPONSE_AUDIO_DELTA, EVENT_TYPE_RESPONSE_AUDIO_TRANSCRIPT_DELTA,
    EVENT_TYPE_RESPONSE_DONE, EVENT_TYPE_RESPONSE_TEXT_DELTA, EVENT_TYPE_SESSION_CREATED,
    EVENT_TYPE_SESSION_UPDATED, MODALITY_AUDIO, MODALITY_TEXT, VOICE_ALLOY, VOICE_SHIMMER,
};
use tracing::{debug, info, warn};

#[derive(Parser)]
#[command(name = "dual_chat")]
#[command(about = "Two OpenAI Realtime agents chatting with each other")]
struct Args {
    /// OpenAI API key
    #[arg(long, default_value_t = std::env::var("OPENAI_API_KEY").unwrap_or_default())]
    api_key: String,

    /// MiniMax API key (for TTS in audio modes)
    #[arg(long, default_value_t = std::env::var("MINIMAX_API_KEY").ok().unwrap_or_default())]
    minimax_key: String,

    /// Conversation mode: t2t, t2a, a2t, a2a
    #[arg(long, default_value = "t2t")]
    mode: String,

    /// Number of conversation rounds
    #[arg(long, default_value = "5")]
    rounds: usize,

    /// Initial prompt
    #[arg(long, default_value = "你好！我是 Agent A，请问你是谁？让我们开始聊天吧！")]
    prompt: String,

    /// Enable verbose output
    #[arg(short, long)]
    verbose: bool,

    /// Enable VAD (Voice Activity Detection) mode for audio input
    #[arg(long)]
    vad: bool,

    /// VAD type: server_vad or semantic_vad
    #[arg(long, default_value = "server_vad")]
    vad_type: String,
}

/// Agent represents a chat participant.
struct Agent {
    name: String,
    session: openai::WebSocketSession,
    input_mode: String,
    output_mode: String,
    voice: String,
    use_vad: bool,
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args = Args::parse();

    // Setup logging
    let filter = if args.verbose {
        "debug"
    } else {
        "info"
    };
    tracing_subscriber::fmt()
        .with_env_filter(filter)
        .with_target(false)
        .init();

    // Parse mode
    let (input_mode, output_mode) = parse_mode(&args.mode)?;

    // Setup MiniMax client for TTS
    let minimax_client = if !args.minimax_key.is_empty() {
        info!("MiniMax TTS enabled for audio input generation");
        Some(minimax::Client::new(&args.minimax_key)?)
    } else {
        None
    };

    info!("=== OpenAI Realtime Dual Chat (Rust) ===");
    info!("Mode: {} (input: {}, output: {})", args.mode, input_mode, output_mode);
    info!("Rounds: {}", args.rounds);
    if args.vad {
        info!("VAD: enabled (type: {})", args.vad_type);
    } else {
        info!("VAD: disabled (manual mode)");
    }
    info!("Initial prompt: {}", truncate(&args.prompt, 50));
    info!("");

    // Create client
    let client = Client::new(&args.api_key);

    // Create Agent A
    info!("Creating Agent A...");
    let mut agent_a = create_agent(
        &client,
        "Agent A",
        &input_mode,
        &output_mode,
        VOICE_ALLOY,
        "你是 Agent A，一个友好的 AI 助手。你正在和另一个 AI (Agent B) 对话。要求：1) 每次回复简短，不超过50字；2) 保持对话有趣；3) 可以提问或回应对方。",
        args.vad,
        &args.vad_type,
    ).await?;

    // Create Agent B
    info!("Creating Agent B...");
    let mut agent_b = create_agent(
        &client,
        "Agent B",
        &input_mode,
        &output_mode,
        VOICE_SHIMMER,
        "你是 Agent B，一个聪明的 AI 助手。你正在和另一个 AI (Agent A) 对话。要求：1) 每次回复简短，不超过50字；2) 保持对话有趣；3) 可以提问或回应对方。",
        args.vad,
        &args.vad_type,
    ).await?;

    info!("");
    info!("=== Starting Conversation ===");
    info!("");

    // Start conversation
    let mut current_message = args.prompt.clone();
    let mut current_audio: Vec<u8> = Vec::new();
    let mut successful_rounds = 0;

    for round in 1..=args.rounds {
        info!("--- Round {}/{} ---", round, args.rounds);

        // Determine sender and receiver
        let (sender_name, receiver) = if round % 2 == 1 {
            ("Agent A", &mut agent_b)
        } else {
            ("Agent B", &mut agent_a)
        };

        info!("[{} -> {}]", sender_name, receiver.name);

        // Prepare audio input if needed (using MiniMax TTS)
        if receiver.input_mode == "audio" && current_audio.is_empty() && !current_message.is_empty() {
            if let Some(ref mm_client) = minimax_client {
                info!("  [TTS] Converting text to audio via MiniMax...");
                match text_to_audio(mm_client, &current_message).await {
                    Ok(audio) => {
                        let duration = audio.len() as f64 / (24000.0 * 2.0);
                        info!("  [TTS] Generated {} bytes ({:.1}s)", audio.len(), duration);
                        current_audio = audio;
                    }
                    Err(e) => {
                        warn!("  [TTS] Warning: {} (falling back to text)", e);
                    }
                }
            }
        }

        // Log input
        if receiver.input_mode == "text" || current_audio.is_empty() {
            info!("  Input (text): {}", truncate(&current_message, 80));
        } else {
            let duration = current_audio.len() as f64 / (24000.0 * 2.0);
            info!("  Input (audio): {} bytes ({:.1}s)", current_audio.len(), duration);
        }

        // Send to receiver and get response
        match send_and_receive(receiver, &current_message, &current_audio).await {
            Ok((response_text, response_audio)) => {
                successful_rounds += 1;

                if !response_text.is_empty() {
                    info!("  Output (text): {}", truncate(&response_text, 80));
                }
                if !response_audio.is_empty() {
                    let duration = response_audio.len() as f64 / (24000.0 * 2.0);
                    info!("  Output (audio): {} bytes ({:.1}s)", response_audio.len(), duration);
                }

                // Prepare for next round
                current_message = if response_text.is_empty() && !response_audio.is_empty() {
                    "[Audio response]".to_string()
                } else {
                    response_text
                };
                current_audio = response_audio;
            }
            Err(e) => {
                warn!("  Error: {}", e);
                if round < args.rounds {
                    current_message = "请继续我们的对话，说点有趣的。".to_string();
                    current_audio.clear();
                    tokio::time::sleep(Duration::from_secs(1)).await;
                    continue;
                }
                break;
            }
        }

        info!("");
        tokio::time::sleep(Duration::from_millis(300)).await;
    }

    info!("=== Conversation Complete ===");
    info!("Successful rounds: {}/{}", successful_rounds, args.rounds);

    // Close sessions
    agent_a.session.close().await?;
    agent_b.session.close().await?;

    Ok(())
}

fn parse_mode(mode: &str) -> Result<(String, String), String> {
    match mode {
        "t2t" => Ok(("text".to_string(), "text".to_string())),
        "t2a" => Ok(("text".to_string(), "audio".to_string())),
        "a2t" => Ok(("audio".to_string(), "text".to_string())),
        "a2a" => Ok(("audio".to_string(), "audio".to_string())),
        _ => Err(format!("Invalid mode: {} (must be t2t, t2a, a2t, or a2a)", mode)),
    }
}

async fn create_agent(
    client: &Client,
    name: &str,
    input_mode: &str,
    output_mode: &str,
    voice: &str,
    instructions: &str,
    use_vad: bool,
    vad_type: &str,
) -> Result<Agent, Box<dyn std::error::Error>> {
    // Connect via WebSocket
    let mut session = client.connect_websocket(None).await?;

    // Wait for session.created
    wait_for_event(&mut session, EVENT_TYPE_SESSION_CREATED, Duration::from_secs(15)).await?;

    info!("  {} session created (WebSocket)", name);

    // Configure session
    let mut modalities = vec![MODALITY_TEXT.to_string()];
    if output_mode == "audio" || input_mode == "audio" {
        modalities.push(MODALITY_AUDIO.to_string());
    }

    // Configure VAD or manual mode
    let (turn_detection, turn_detection_disabled) = if use_vad && input_mode == "audio" {
        // Enable VAD for automatic turn detection
        info!("  {} VAD enabled (type: {})", name, vad_type);
        (
            Some(TurnDetection {
                detection_type: Some(vad_type.to_string()),
                threshold: Some(0.5),
                prefix_padding_ms: Some(300),
                silence_duration_ms: Some(800), // Increased for better detection
                create_response: Some(true),
                interrupt_response: Some(true),
                eagerness: if vad_type == "semantic_vad" {
                    Some("medium".to_string())
                } else {
                    None
                },
            }),
            false,
        )
    } else {
        // Disable VAD for manual control
        (None, true)
    };

    let config = SessionConfig {
        modalities,
        voice: Some(voice.to_string()),
        input_audio_format: Some(AUDIO_FORMAT_PCM16.to_string()),
        output_audio_format: Some(AUDIO_FORMAT_PCM16.to_string()),
        instructions: Some(instructions.to_string()),
        turn_detection,
        turn_detection_disabled,
        input_audio_transcription: if input_mode == "audio" {
            Some(openai::TranscriptionConfig {
                model: Some("whisper-1".to_string()),
            })
        } else {
            None
        },
        ..Default::default()
    };

    session.update_session(&config).await?;

    // Wait for session.updated
    wait_for_event(&mut session, EVENT_TYPE_SESSION_UPDATED, Duration::from_secs(5)).await?;

    info!("  {} configured (input: {}, output: {}, voice: {})", name, input_mode, output_mode, voice);

    Ok(Agent {
        name: name.to_string(),
        session,
        input_mode: input_mode.to_string(),
        output_mode: output_mode.to_string(),
        voice: voice.to_string(),
        use_vad,
    })
}

async fn send_and_receive(
    agent: &mut Agent,
    text_input: &str,
    audio_input: &[u8],
) -> Result<(String, Vec<u8>), Box<dyn std::error::Error>> {
    // Send input based on agent's input mode
    if agent.input_mode == "audio" && !audio_input.is_empty() {
        if agent.use_vad {
            // VAD mode: use pcm::Mixer for real-time audio streaming
            info!("  [VAD] Using pcm::Mixer for real-time audio streaming");
            send_audio_with_vad(agent, audio_input).await?;
        } else {
            // Manual mode: send audio in chunks, then commit
            let chunk_size = 4800; // 100ms at 24kHz
            for chunk in audio_input.chunks(chunk_size) {
                agent.session.append_audio(chunk).await?;
                tokio::time::sleep(Duration::from_millis(20)).await;
            }
            tokio::time::sleep(Duration::from_millis(100)).await;
            agent.session.commit_input().await?;
            // Request response manually
            agent.session.create_response(None).await?;
        }
    } else {
        // Send text
        agent.session.add_user_message(text_input).await?;
        // Request response manually
        agent.session.create_response(None).await?;
    }

    // Collect response
    collect_response(agent, Duration::from_secs(60)).await
}

/// Send audio with VAD mode using pcm::Mixer for real-time streaming.
///
/// This function:
/// 1. Creates a Mixer with silence_gap to output silence when no tracks are active
/// 2. Spawns a background thread to continuously read from Mixer and send chunks via channel
/// 3. Spawns an async task to receive chunks and send to OpenAI
/// 4. Creates a Track and writes speech audio in real-time chunks
/// 5. After speech, waits for trailing silence to trigger VAD
/// 6. VAD automatically creates a response when end-of-speech is detected
async fn send_audio_with_vad(
    agent: &mut Agent,
    audio_input: &[u8],
) -> Result<(), Box<dyn std::error::Error>> {
    // Audio format: 24kHz mono 16-bit PCM
    let format = Format::L16Mono24K;
    let chunk_duration = Duration::from_millis(20);
    let chunk_size = format.bytes_in_duration(chunk_duration) as usize; // 960 bytes per 20ms

    // Calculate durations
    let leading_silence = Duration::from_millis(300);
    let speech_duration = format.duration(audio_input.len() as u64);
    let trailing_silence = Duration::from_millis(1200); // Enough for VAD to detect end-of-speech

    info!(
        "  [VAD] Leading silence: {:?}, Speech: {:?}, Trailing silence: {:?}",
        leading_silence, speech_duration, trailing_silence
    );

    // Create Mixer with silence gap (outputs silence when no tracks are active)
    let mixer = Mixer::new(format, MixerOptions::default().with_silence_gap(Duration::from_secs(60)));

    // Channel to send audio chunks from reader thread to sender task
    let (chunk_tx, mut chunk_rx) = tokio::sync::mpsc::channel::<Vec<u8>>(100);

    // Clone mixer for reader thread
    let mixer_for_reader = Arc::clone(&mixer);

    // Calculate total duration (leading + speech + trailing)
    let total_duration = leading_silence + speech_duration + trailing_silence + Duration::from_millis(100);

    // Spawn background thread to read from mixer (blocking operation)
    let reader_handle = std::thread::spawn(move || {
        let start = std::time::Instant::now();
        let mut buf = vec![0u8; chunk_size];

        loop {
            // Check if we've exceeded total duration
            if start.elapsed() > total_duration {
                debug!("[VAD] Reader exceeded total duration, stopping");
                break;
            }

            // Read from mixer (blocking)
            match (&*mixer_for_reader).read(&mut buf) {
                Ok(n) if n > 0 => {
                    if chunk_tx.blocking_send(buf[..n].to_vec()).is_err() {
                        debug!("[VAD] Channel closed, stopping reader");
                        break;
                    }
                }
                Ok(_) => {
                    // No data, but not EOF - continue
                    std::thread::sleep(Duration::from_millis(5));
                }
                Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => {
                    debug!("[VAD] Mixer EOF");
                    break;
                }
                Err(e) if e.kind() == std::io::ErrorKind::BrokenPipe => {
                    debug!("[VAD] Mixer closed");
                    break;
                }
                Err(e) => {
                    warn!("[VAD] Mixer read error: {}", e);
                    break;
                }
            }
        }
        debug!("[VAD] Reader thread finished");
    });

    // Spawn async task to receive chunks and send to OpenAI
    let session_for_send = unsafe {
        // SAFETY: We ensure the session reference is valid during the task's lifetime
        std::mem::transmute::<&mut openai::WebSocketSession, &'static mut openai::WebSocketSession>(
            &mut agent.session,
        )
    };

    let sender_task = tokio::spawn(async move {
        while let Some(chunk) = chunk_rx.recv().await {
            if let Err(e) = session_for_send.append_audio(&chunk).await {
                warn!("[VAD] Append audio failed: {}", e);
                break;
            }
        }
        debug!("[VAD] Sender task finished");
    });

    // Wait for leading silence (mixer outputs silence when no tracks)
    tokio::time::sleep(leading_silence).await;

    // Create a Track and write speech audio in real-time
    info!("  [VAD] Writing speech audio to track in real-time...");
    let (track, ctrl) = mixer.create_track(None)?;

    let mut offset = 0;
    let mut ticker = tokio::time::interval(chunk_duration);
    while offset < audio_input.len() {
        ticker.tick().await;

        let end = (offset + chunk_size).min(audio_input.len());
        let chunk_data = &audio_input[offset..end];

        // Write chunk to track
        track.write_bytes(chunk_data)?;
        offset = end;
    }

    // Close track writing - mixer will resume outputting silence
    ctrl.close_write();
    info!("  [VAD] Speech written, waiting for trailing silence...");

    // Wait for trailing silence (VAD should detect end-of-speech during this time)
    tokio::time::sleep(trailing_silence).await;

    // Close mixer to signal EOF to reader
    mixer.close()?;

    // Wait for reader thread to finish
    let _ = reader_handle.join();

    // Wait for sender task to finish
    let _ = sender_task.await;

    // With VAD enabled, do NOT call commit_input() - VAD handles turn detection automatically
    // The server will auto-create a response when it detects end of speech

    info!("  [VAD] Audio stream complete, waiting for VAD response...");
    Ok(())
}

async fn collect_response(
    agent: &mut Agent,
    timeout: Duration,
) -> Result<(String, Vec<u8>), Box<dyn std::error::Error>> {
    let mut text = String::new();
    let mut audio = Vec::new();
    let deadline = tokio::time::Instant::now() + timeout;

    loop {
        let remaining = deadline.saturating_duration_since(tokio::time::Instant::now());
        if remaining.is_zero() {
            if !text.is_empty() || !audio.is_empty() {
                return Ok((text, audio));
            }
            return Err("timeout waiting for response".into());
        }

        match tokio::time::timeout(remaining, agent.session.recv()).await {
            Ok(Some(result)) => {
                let event = result?;
                match event.event_type.as_str() {
                    EVENT_TYPE_RESPONSE_TEXT_DELTA => {
                        if let Some(delta) = &event.delta {
                            text.push_str(delta);
                        }
                    }
                    EVENT_TYPE_RESPONSE_AUDIO_DELTA => {
                        if let Some(audio_data) = &event.audio {
                            audio.extend_from_slice(audio_data);
                        }
                    }
                    EVENT_TYPE_RESPONSE_AUDIO_TRANSCRIPT_DELTA => {
                        if let Some(delta) = &event.delta {
                            text.push_str(delta);
                        }
                    }
                    EVENT_TYPE_RESPONSE_DONE => {
                        return Ok((text, audio));
                    }
                    _ => {
                        debug!("event: {}", event.event_type);
                    }
                }
            }
            Ok(None) => {
                return Ok((text, audio));
            }
            Err(_) => {
                if !text.is_empty() || !audio.is_empty() {
                    return Ok((text, audio));
                }
                return Err("timeout waiting for response".into());
            }
        }
    }
}

async fn wait_for_event(
    session: &mut openai::WebSocketSession,
    event_type: &str,
    timeout: Duration,
) -> Result<(), Box<dyn std::error::Error>> {
    let deadline = tokio::time::Instant::now() + timeout;

    loop {
        let remaining = deadline.saturating_duration_since(tokio::time::Instant::now());
        if remaining.is_zero() {
            return Err(format!("timeout waiting for {}", event_type).into());
        }

        match tokio::time::timeout(remaining, session.recv()).await {
            Ok(Some(result)) => {
                let event = result?;
                if event.event_type == event_type {
                    return Ok(());
                }
            }
            Ok(None) => {
                return Err("session closed".into());
            }
            Err(_) => {
                return Err(format!("timeout waiting for {}", event_type).into());
            }
        }
    }
}

async fn text_to_audio(
    client: &minimax::Client,
    text: &str,
) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    let request = minimax::SpeechRequest {
        model: "speech-2.6-hd".to_string(),
        text: text.to_string(),
        voice_setting: Some(minimax::VoiceSetting {
            voice_id: "female-shaonv".to_string(),
            speed: Some(1.1),
            vol: Some(1.0),
            ..Default::default()
        }),
        audio_setting: Some(minimax::AudioSetting {
            format: Some(minimax::AudioFormat::Pcm),
            sample_rate: Some(24000),
            channel: Some(1),
            ..Default::default()
        }),
        language_boost: Some("Chinese".to_string()),
        ..Default::default()
    };

    let response = client.speech().synthesize(&request).await?;
    Ok(response.audio)
}

fn truncate(s: &str, max_len: usize) -> String {
    let chars: Vec<char> = s.chars().collect();
    if chars.len() <= max_len {
        s.to_string()
    } else {
        format!("{}...", chars[..max_len].iter().collect::<String>())
    }
}
