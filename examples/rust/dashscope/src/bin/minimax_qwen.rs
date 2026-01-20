//! Example: Multi-round conversation using MiniMax TTS and DashScope Qwen-Omni.
//!
//! This example demonstrates an end-to-end conversation flow:
//! 1. Use MiniMax TTS to generate speech from text (16kHz PCM16)
//! 2. Send the speech to DashScope Qwen-Omni-Realtime
//! 3. Receive AI response (text + audio)
//! 4. Repeat for multi-round conversation
//!
//! Environment variables required:
//!   MINIMAX_API_KEY - MiniMax API key
//!   DASHSCOPE_API_KEY - DashScope API key
//!
//! Usage:
//!   cargo run --bin minimax_qwen

use std::io::Write;
use std::time::Duration;

use giztoy_dashscope::{
    Client as DashScopeClient, RealtimeConfig, SessionConfig, TurnDetection,
    MODEL_QWEN_OMNI_TURBO_REALTIME_LATEST, VOICE_CHELSIE,
    VAD_MODE_SERVER_VAD, AUDIO_FORMAT_PCM16, MODALITY_TEXT, MODALITY_AUDIO,
    EVENT_TYPE_SESSION_CREATED, EVENT_TYPE_SESSION_UPDATED,
    EVENT_TYPE_CHOICES_RESPONSE, EVENT_TYPE_RESPONSE_DONE,
    EVENT_TYPE_RESPONSE_TEXT_DELTA, EVENT_TYPE_RESPONSE_AUDIO_DELTA,
    EVENT_TYPE_ERROR, EVENT_TYPE_INPUT_SPEECH_STARTED,
    EVENT_TYPE_RESPONSE_CREATED,
};
use giztoy_minimax::{
    Client as MiniMaxClient, SpeechRequest, VoiceSetting, AudioSetting, AudioFormat,
};

/// A conversation turn with input text and expected response handling.
struct ConversationTurn {
    /// User's input text (will be converted to speech)
    user_text: &'static str,
    /// Description for logging
    description: &'static str,
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Initialize tracing
    tracing_subscriber::fmt()
        .with_max_level(tracing::Level::INFO)
        .with_target(false)
        .init();

    // Get API keys
    let minimax_api_key = std::env::var("MINIMAX_API_KEY")
        .expect("MINIMAX_API_KEY environment variable not set");
    let dashscope_api_key = std::env::var("DASHSCOPE_API_KEY")
        .expect("DASHSCOPE_API_KEY environment variable not set");

    println!("╔══════════════════════════════════════════════════════════════╗");
    println!("║     MiniMax TTS + DashScope Qwen-Omni Multi-Round Chat       ║");
    println!("╚══════════════════════════════════════════════════════════════╝");
    println!();

    // Create clients
    println!("[1/4] Creating MiniMax client...");
    let minimax_client = MiniMaxClient::new(&minimax_api_key)?;
    println!("      ✓ MiniMax client ready");

    println!("[2/4] Creating DashScope client...");
    let dashscope_client = DashScopeClient::new(&dashscope_api_key)?;
    println!("      ✓ DashScope client ready");

    // Connect to Qwen-Omni-Realtime
    println!("[3/4] Connecting to Qwen-Omni-Realtime...");
    let config = RealtimeConfig {
        model: MODEL_QWEN_OMNI_TURBO_REALTIME_LATEST.to_string(),
    };

    let mut session = tokio::time::timeout(
        Duration::from_secs(30),
        dashscope_client.realtime().connect(&config),
    )
    .await
    .map_err(|_| "Connection timeout")??;
    println!("      ✓ WebSocket connected");

    // Wait for session.created
    let session_id = wait_for_session_created(&mut session).await?;
    println!("      ✓ Session created: {}", session_id.as_deref().unwrap_or("unknown"));

    // Configure session
    println!("[4/4] Configuring session...");
    let session_config = SessionConfig {
        voice: Some(VOICE_CHELSIE.to_string()),
        input_audio_format: Some(AUDIO_FORMAT_PCM16.to_string()),
        output_audio_format: Some(AUDIO_FORMAT_PCM16.to_string()),
        modalities: Some(vec![MODALITY_TEXT.to_string(), MODALITY_AUDIO.to_string()]),
        enable_input_audio_transcription: Some(true),
        input_audio_transcription_model: Some("gummy-realtime-v1".to_string()),
        turn_detection: Some(TurnDetection {
            detection_type: Some(VAD_MODE_SERVER_VAD.to_string()),
            threshold: Some(0.5),
            prefix_padding_ms: Some(300),
            silence_duration_ms: Some(800),
        }),
        instructions: Some("You are a helpful AI assistant. Please respond concisely.".to_string()),
        ..Default::default()
    };
    session.update_session(&session_config).await?;
    wait_for_session_updated(&mut session).await?;
    println!("      ✓ Session configured");

    println!();
    println!("═══════════════════ Starting Conversation ═══════════════════");
    println!();

    // Define conversation turns
    let turns = vec![
        ConversationTurn {
            user_text: "你好，请介绍一下你自己。",
            description: "Greeting and introduction",
        },
        ConversationTurn {
            user_text: "今天天气怎么样？",
            description: "Ask about weather",
        },
        ConversationTurn {
            user_text: "谢谢你的回答，再见！",
            description: "Say goodbye",
        },
    ];

    let total_turns = turns.len();
    for (i, turn) in turns.into_iter().enumerate() {
        println!("┌─────────────────────────────────────────────────────────────┐");
        println!("│ Turn {}/{}: {}                        ", i + 1, total_turns, turn.description);
        println!("└─────────────────────────────────────────────────────────────┘");
        println!();

        // Generate speech using MiniMax TTS
        println!("  📝 User: \"{}\"", turn.user_text);
        println!();
        print!("  🔊 Generating speech via MiniMax TTS... ");
        std::io::stdout().flush()?;

        let audio_data = generate_speech(&minimax_client, turn.user_text).await?;
        println!("✓ ({} bytes)", audio_data.len());

        // Send audio to Qwen-Omni
        print!("  📤 Sending audio to Qwen-Omni... ");
        std::io::stdout().flush()?;

        send_audio_to_session(&mut session, &audio_data).await?;
        println!("✓");

        // Wait for and display response
        println!();
        println!("  🤖 Qwen Response:");
        print!("     ");

        let (response_text, response_audio) = receive_response(&mut session).await?;
        
        if response_text.is_empty() {
            println!("(no text response received)");
        } else {
            println!();
        }

        if !response_audio.is_empty() {
            println!();
            println!("     📢 Audio response: {} bytes", response_audio.len());
        }

        println!();

        // Small delay between turns
        if i < total_turns - 1 {
            tokio::time::sleep(Duration::from_millis(500)).await;
        }
    }

    // Close session
    println!("═══════════════════ Conversation Complete ═══════════════════");
    println!();
    session.close().await?;
    println!("✓ Session closed");

    Ok(())
}

/// Generate speech from text using MiniMax TTS.
///
/// Returns PCM16 audio data at 16kHz sample rate (required by DashScope).
async fn generate_speech(
    client: &MiniMaxClient,
    text: &str,
) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    let request = SpeechRequest {
        model: "speech-2.6-hd".to_string(),
        text: text.to_string(),
        voice_setting: Some(VoiceSetting {
            voice_id: "female-shaonv".to_string(),
            speed: Some(1.0),
            vol: Some(1.0),
            emotion: Some("happy".to_string()),
            ..Default::default()
        }),
        audio_setting: Some(AudioSetting {
            sample_rate: Some(16000),  // 16kHz for DashScope
            format: Some(AudioFormat::Pcm),
            channel: Some(1),  // Mono
            ..Default::default()
        }),
        language_boost: Some("Chinese".to_string()),
        ..Default::default()
    };

    let response = client.speech().synthesize(&request).await?;
    Ok(response.audio)
}

/// Wait for session.created event and return session ID.
async fn wait_for_session_created(
    session: &mut giztoy_dashscope::RealtimeSession,
) -> Result<Option<String>, Box<dyn std::error::Error>> {
    let timeout = Duration::from_secs(10);
    let start = std::time::Instant::now();

    loop {
        if start.elapsed() > timeout {
            return Err("Timeout waiting for session.created".into());
        }

        match tokio::time::timeout(Duration::from_millis(100), session.recv()).await {
            Ok(Some(Ok(event))) => {
                if event.event_type == EVENT_TYPE_SESSION_CREATED {
                    return Ok(event.session.and_then(|s| s.id));
                }
            }
            Ok(Some(Err(e))) => return Err(e.into()),
            Ok(None) => return Err("Session closed unexpectedly".into()),
            Err(_) => continue,
        }
    }
}

/// Wait for session.updated event.
async fn wait_for_session_updated(
    session: &mut giztoy_dashscope::RealtimeSession,
) -> Result<(), Box<dyn std::error::Error>> {
    let timeout = Duration::from_secs(5);
    let start = std::time::Instant::now();

    loop {
        if start.elapsed() > timeout {
            // Timeout is OK, session may already be updated
            return Ok(());
        }

        match tokio::time::timeout(Duration::from_millis(100), session.recv()).await {
            Ok(Some(Ok(event))) => {
                if event.event_type == EVENT_TYPE_SESSION_UPDATED {
                    return Ok(());
                }
            }
            Ok(Some(Err(_))) | Ok(None) | Err(_) => continue,
        }
    }
}

/// Send audio data to the session in chunks.
async fn send_audio_to_session(
    session: &mut giztoy_dashscope::RealtimeSession,
    audio_data: &[u8],
) -> Result<(), Box<dyn std::error::Error>> {
    // Send audio in chunks (100ms chunks at 16kHz, 16-bit = 3200 bytes)
    const CHUNK_SIZE: usize = 3200;
    const CHUNK_INTERVAL_MS: u64 = 100;

    for chunk in audio_data.chunks(CHUNK_SIZE) {
        session.append_audio(chunk).await?;
        tokio::time::sleep(Duration::from_millis(CHUNK_INTERVAL_MS)).await;
    }

    // Commit the audio buffer to trigger processing
    session.commit_input().await?;

    Ok(())
}

/// Receive and process response from Qwen-Omni.
///
/// Returns (response_text, response_audio).
async fn receive_response(
    session: &mut giztoy_dashscope::RealtimeSession,
) -> Result<(String, Vec<u8>), Box<dyn std::error::Error>> {
    let mut response_text = String::new();
    let mut last_text = String::new();
    let mut response_audio: Vec<u8> = Vec::new();

    let timeout = Duration::from_secs(30);  // Reduced timeout
    let start = std::time::Instant::now();
    let mut got_response = false;
    let mut speech_detected = false;

    loop {
        if start.elapsed() > timeout {
            if !got_response {
                if speech_detected {
                    println!("(timeout waiting for AI response after speech detected)");
                } else {
                    println!("(timeout - speech not detected by server)");
                }
            }
            break;
        }

        match tokio::time::timeout(Duration::from_millis(100), session.recv()).await {
            Ok(Some(Ok(event))) => {
                match event.event_type.as_str() {
                    EVENT_TYPE_INPUT_SPEECH_STARTED => {
                        speech_detected = true;
                        print!("[speech detected] ");
                        std::io::stdout().flush()?;
                    }
                    EVENT_TYPE_RESPONSE_CREATED => {
                        got_response = true;
                        response_text.clear();
                        last_text.clear();
                        response_audio.clear();
                    }
                    EVENT_TYPE_CHOICES_RESPONSE => {
                        // DashScope's "choices" format
                        if let Some(ref delta) = event.delta {
                            if delta.len() > last_text.len() {
                                let new_text = &delta[last_text.len()..];
                                print!("{}", new_text);
                                std::io::stdout().flush()?;
                                last_text = delta.clone();
                            }
                        }
                        if let Some(audio) = event.audio {
                            response_audio.extend(audio);
                        }
                        if event.finish_reason.is_some() {
                            response_text = last_text.clone();
                            break;
                        }
                    }
                    EVENT_TYPE_RESPONSE_TEXT_DELTA => {
                        if let Some(ref delta) = event.delta {
                            print!("{}", delta);
                            std::io::stdout().flush()?;
                            response_text.push_str(delta);
                        }
                    }
                    EVENT_TYPE_RESPONSE_AUDIO_DELTA => {
                        if let Some(audio) = event.audio {
                            response_audio.extend(audio);
                        }
                    }
                    EVENT_TYPE_RESPONSE_DONE => {
                        break;
                    }
                    EVENT_TYPE_ERROR => {
                        if let Some(ref err) = event.error {
                            let code = err.code.as_deref().unwrap_or("unknown");
                            let message = err.message.as_deref().unwrap_or("unknown error");
                            eprintln!("\n     ❌ Error: {} - {}", code, message);
                        }
                        break;
                    }
                    _ => {}
                }
            }
            Ok(Some(Err(e))) => {
                // Check if it's a close frame
                let err_str = e.to_string();
                if err_str.contains("close") || err_str.contains("timeout") {
                    if !got_response {
                        println!("(server closed connection: {})", err_str);
                    }
                    break;
                }
                return Err(e.into());
            }
            Ok(None) => {
                if !got_response {
                    println!("(session closed)");
                }
                break;
            }
            Err(_) => continue,
        }
    }

    Ok((response_text, response_audio))
}
