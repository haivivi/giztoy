//! Example: Basic realtime session connection with Qwen-Omni.
//!
//! This example demonstrates:
//! - Creating a DashScope client
//! - Connecting to Qwen-Omni-Realtime
//! - Configuring the session
//! - Handling session events
//!
//! Usage:
//!   DASHSCOPE_API_KEY=your-key cargo run --bin realtime

use giztoy_dashscope::{
    Client, RealtimeConfig, SessionConfig, TurnDetection,
    MODEL_QWEN_OMNI_TURBO_REALTIME_LATEST, VOICE_CHELSIE,
    VAD_MODE_SERVER_VAD, AUDIO_FORMAT_PCM16, MODALITY_TEXT, MODALITY_AUDIO,
    EVENT_TYPE_SESSION_CREATED, EVENT_TYPE_SESSION_UPDATED,
};
use std::time::Duration;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Initialize tracing
    tracing_subscriber::fmt()
        .with_max_level(tracing::Level::INFO)
        .with_target(false)
        .init();

    // Get API key from environment
    let api_key = std::env::var("DASHSCOPE_API_KEY")
        .expect("DASHSCOPE_API_KEY environment variable not set");

    println!("Creating DashScope client...");
    let client = Client::new(&api_key)?;

    println!("Connecting to Qwen-Omni-Realtime...");
    let config = RealtimeConfig {
        model: MODEL_QWEN_OMNI_TURBO_REALTIME_LATEST.to_string(),
    };
    
    let mut session = tokio::time::timeout(
        Duration::from_secs(30),
        client.realtime().connect(&config),
    )
    .await
    .map_err(|_| "Connection timeout")??;
    
    println!("✓ Connected to Qwen-Omni-Realtime");

    // Wait for session.created event
    println!("Waiting for session.created...");
    loop {
        match tokio::time::timeout(Duration::from_secs(5), session.recv()).await {
            Ok(Some(Ok(event))) => {
                if event.event_type == EVENT_TYPE_SESSION_CREATED {
                    println!("✓ Session created");
                    if let Some(ref info) = event.session {
                        println!("  Session ID: {:?}", info.id);
                    }
                    break;
                }
            }
            Ok(Some(Err(e))) => {
                eprintln!("Error: {}", e);
                return Err(e.into());
            }
            Ok(None) => {
                eprintln!("Session closed unexpectedly");
                return Err("Session closed".into());
            }
            Err(_) => {
                eprintln!("Timeout waiting for session.created");
                return Err("Timeout".into());
            }
        }
    }

    // Configure session
    println!("Configuring session...");
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
        ..Default::default()
    };

    session.update_session(&session_config).await?;
    println!("✓ Session configuration sent");

    // Wait for session.updated
    loop {
        match tokio::time::timeout(Duration::from_secs(5), session.recv()).await {
            Ok(Some(Ok(event))) => {
                if event.event_type == EVENT_TYPE_SESSION_UPDATED {
                    println!("✓ Session updated");
                    break;
                }
            }
            Ok(Some(Err(e))) => {
                eprintln!("Warning: {}", e);
            }
            Ok(None) => {
                println!("Session closed");
                break;
            }
            Err(_) => {
                println!("Timeout waiting for session.updated (continuing anyway)");
                break;
            }
        }
    }

    println!();
    println!("Session is ready!");
    println!();
    println!("This example demonstrates the connection flow.");
    println!("In a real application, you would:");
    println!("  1. Send audio data via session.append_audio()");
    println!("  2. Handle response events (audio, text, etc.)");
    println!("  3. Save output audio or display text");
    println!();

    // Close session
    session.close().await?;
    println!("✓ Session closed");

    Ok(())
}
