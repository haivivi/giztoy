//! Example: Realtime audio conversation with Qwen-Omni.
//!
//! This example demonstrates:
//! - Sending audio file to Qwen-Omni-Realtime
//! - Receiving and saving audio response
//!
//! Usage:
//!   DASHSCOPE_API_KEY=your-key cargo run --bin realtime_audio -- input.pcm output.pcm
//!
//! Audio format:
//!   - Input: 16-bit PCM, 16kHz, mono
//!   - Output: 16-bit PCM, 24kHz, mono
//!
//! Convert audio with ffmpeg:
//!   ffmpeg -i input.mp3 -ar 16000 -ac 1 -f s16le input.pcm
//!   ffmpeg -f s16le -ar 24000 -ac 1 -i output.pcm output.mp3

use giztoy_dashscope::{
    Client, RealtimeConfig, SessionConfig, TurnDetection,
    MODEL_QWEN_OMNI_TURBO_REALTIME_LATEST, VOICE_CHELSIE,
    VAD_MODE_SERVER_VAD, AUDIO_FORMAT_PCM16, MODALITY_TEXT, MODALITY_AUDIO,
    EVENT_TYPE_SESSION_CREATED, EVENT_TYPE_SESSION_UPDATED,
    EVENT_TYPE_CHOICES_RESPONSE, EVENT_TYPE_RESPONSE_DONE,
    EVENT_TYPE_RESPONSE_AUDIO_DELTA, EVENT_TYPE_RESPONSE_TEXT_DELTA,
};
use std::io::Read;
use std::time::Duration;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Initialize tracing
    tracing_subscriber::fmt()
        .with_max_level(tracing::Level::INFO)
        .with_target(false)
        .init();

    // Parse arguments
    let args: Vec<String> = std::env::args().collect();
    if args.len() < 3 {
        eprintln!("Usage: {} <input.pcm> <output.pcm>", args[0]);
        eprintln!();
        eprintln!("Audio format:");
        eprintln!("  Input:  16-bit PCM, 16kHz, mono");
        eprintln!("  Output: 16-bit PCM, 24kHz, mono");
        eprintln!();
        eprintln!("Convert audio with ffmpeg:");
        eprintln!("  ffmpeg -i input.mp3 -ar 16000 -ac 1 -f s16le input.pcm");
        eprintln!("  ffmpeg -f s16le -ar 24000 -ac 1 -i output.pcm output.mp3");
        std::process::exit(1);
    }
    let input_file = &args[1];
    let output_file = &args[2];

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
    
    println!("✓ Connected");

    // Wait for session.created
    wait_for_event(&mut session, EVENT_TYPE_SESSION_CREATED, Duration::from_secs(5)).await?;
    println!("✓ Session created");

    // Configure session
    let session_config = SessionConfig {
        voice: Some(VOICE_CHELSIE.to_string()),
        input_audio_format: Some(AUDIO_FORMAT_PCM16.to_string()),
        output_audio_format: Some(AUDIO_FORMAT_PCM16.to_string()),
        modalities: Some(vec![MODALITY_TEXT.to_string(), MODALITY_AUDIO.to_string()]),
        enable_input_audio_transcription: Some(true),
        turn_detection: Some(TurnDetection {
            detection_type: Some(VAD_MODE_SERVER_VAD.to_string()),
            threshold: Some(0.5),
            prefix_padding_ms: Some(300),
            silence_duration_ms: Some(800),
        }),
        ..Default::default()
    };

    session.update_session(&session_config).await?;
    let _ = wait_for_event(&mut session, EVENT_TYPE_SESSION_UPDATED, Duration::from_secs(3)).await;
    println!("✓ Session configured");

    // Send audio file
    println!("Sending audio file: {}", input_file);
    let mut file = std::fs::File::open(input_file)?;
    let mut total_bytes = 0;
    
    const CHUNK_SIZE: usize = 3200; // 16kHz * 2 bytes * 0.1s
    let mut buf = vec![0u8; CHUNK_SIZE];
    
    loop {
        let n = file.read(&mut buf)?;
        if n == 0 {
            break;
        }
        session.append_audio(&buf[..n]).await?;
        total_bytes += n;
        
        // Simulate real-time streaming
        tokio::time::sleep(Duration::from_millis(100)).await;
    }
    
    println!("✓ Sent {} bytes of audio", total_bytes);

    // Receive response
    println!("Waiting for response...");
    let mut audio_chunks: Vec<Vec<u8>> = Vec::new();
    let mut text_content = String::new();
    let mut last_text = String::new();
    
    let timeout_duration = Duration::from_secs(30);
    let start = std::time::Instant::now();

    loop {
        if start.elapsed() > timeout_duration {
            println!("Timeout waiting for response");
            break;
        }

        match tokio::time::timeout(Duration::from_millis(100), session.recv()).await {
            Ok(Some(Ok(event))) => {
                match event.event_type.as_str() {
                    EVENT_TYPE_CHOICES_RESPONSE => {
                        // DashScope "choices" format
                        if let Some(ref delta) = event.delta {
                            if delta.len() > last_text.len() {
                                print!("{}", &delta[last_text.len()..]);
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
                            text_content.push_str(delta);
                        }
                    }
                    EVENT_TYPE_RESPONSE_AUDIO_DELTA => {
                        if let Some(audio) = event.audio {
                            audio_chunks.push(audio);
                        }
                    }
                    EVENT_TYPE_RESPONSE_DONE => {
                        if !text_content.is_empty() {
                            println!();
                        }
                        break;
                    }
                    _ => {}
                }
            }
            Ok(Some(Err(e))) => {
                eprintln!("Error: {}", e);
                break;
            }
            Ok(None) => {
                break;
            }
            Err(_) => {
                // Timeout, continue
            }
        }
    }

    // Save audio output
    if !audio_chunks.is_empty() {
        let total_size: usize = audio_chunks.iter().map(|c| c.len()).sum();
        let mut audio = Vec::with_capacity(total_size);
        for chunk in audio_chunks {
            audio.extend(chunk);
        }
        std::fs::write(output_file, &audio)?;
        println!("✓ Audio saved to: {} ({} bytes)", output_file, total_size);
    } else {
        println!("No audio response received");
    }

    // Close session
    session.close().await?;
    println!("✓ Session closed");

    Ok(())
}

async fn wait_for_event(
    session: &mut giztoy_dashscope::RealtimeSession,
    event_type: &str,
    timeout_duration: Duration,
) -> Result<giztoy_dashscope::RealtimeEvent, Box<dyn std::error::Error>> {
    let start = std::time::Instant::now();
    loop {
        if start.elapsed() > timeout_duration {
            return Err(format!("Timeout waiting for {}", event_type).into());
        }

        match tokio::time::timeout(Duration::from_millis(100), session.recv()).await {
            Ok(Some(Ok(event))) => {
                if event.event_type == event_type {
                    return Ok(event);
                }
            }
            Ok(Some(Err(e))) => {
                return Err(e.into());
            }
            Ok(None) => {
                return Err("Session closed".into());
            }
            Err(_) => {
                // Timeout on recv, continue waiting
            }
        }
    }
}
