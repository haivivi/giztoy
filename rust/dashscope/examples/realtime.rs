//! Example: Realtime session with Qwen-Omni.
//!
//! Usage:
//!   DASHSCOPE_API_KEY=your-key cargo run --example realtime

use giztoy_dashscope::{
    Client, RealtimeConfig, SessionConfig, TurnDetection,
    MODEL_QWEN_OMNI_TURBO_REALTIME_LATEST, VOICE_CHELSIE,
    VAD_MODE_SERVER_VAD, AUDIO_FORMAT_PCM16, MODALITY_TEXT, MODALITY_AUDIO,
    EVENT_TYPE_SESSION_CREATED, EVENT_TYPE_SESSION_UPDATED,
};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Get API key from environment
    let api_key = std::env::var("DASHSCOPE_API_KEY")
        .expect("DASHSCOPE_API_KEY environment variable not set");

    // Create client
    let client = Client::new(&api_key)?;

    // Connect to realtime session
    let config = RealtimeConfig {
        model: MODEL_QWEN_OMNI_TURBO_REALTIME_LATEST.to_string(),
    };
    
    let mut session = client.realtime().connect(&config).await?;
    println!("Connected to Qwen-Omni-Realtime");

    // Update session configuration
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

    // Wait for session.created
    loop {
        if let Some(result) = session.recv().await {
            match result {
                Ok(event) => {
                    if event.event_type == EVENT_TYPE_SESSION_CREATED {
                        println!("Session created: {:?}", event.session);
                        break;
                    }
                }
                Err(e) => {
                    eprintln!("Error: {}", e);
                    return Err(e.into());
                }
            }
        }
    }

    // Update session
    session.update_session(&session_config).await?;
    println!("Session configuration sent");

    // Wait for session.updated
    loop {
        if let Some(result) = session.recv().await {
            match result {
                Ok(event) => {
                    if event.event_type == EVENT_TYPE_SESSION_UPDATED {
                        println!("Session updated");
                        break;
                    }
                }
                Err(e) => {
                    eprintln!("Error: {}", e);
                    return Err(e.into());
                }
            }
        }
    }

    println!("\nReady to receive audio input...");
    println!("This example demonstrates the connection flow.");
    println!("In a real application, you would send audio data and handle responses.");

    // Close session
    session.close().await?;
    println!("\nSession closed");

    Ok(())
}
