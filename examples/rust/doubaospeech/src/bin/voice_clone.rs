//! Voice Clone example
//!
//! This example demonstrates how to use the Voice Clone service to train
//! custom voices from audio files.
//!
//! Usage:
//!   cargo run --bin voice_clone -- --audio sample.wav --speaker-id my_voice
//!
//! Environment variables:
//!   DOUBAO_APP_ID - Application ID
//!   DOUBAO_ACCESS_TOKEN - Access token

use giztoy_doubaospeech::{Client, VoiceCloneTrainRequest};
use std::env;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Parse command line arguments
    let args: Vec<String> = env::args().collect();
    
    let (audio_path, speaker_id) = if args.len() >= 5 {
        let mut audio = String::new();
        let mut speaker = String::new();
        let mut i = 1;
        while i < args.len() {
            match args[i].as_str() {
                "--audio" | "-a" => {
                    audio = args.get(i + 1).cloned().unwrap_or_default();
                    i += 2;
                }
                "--speaker-id" | "-s" => {
                    speaker = args.get(i + 1).cloned().unwrap_or_default();
                    i += 2;
                }
                _ => i += 1,
            }
        }
        (audio, speaker)
    } else {
        // Demo mode - just show how to use the API
        println!("Voice Clone Example");
        println!("==================");
        println!();
        println!("Usage: voice_clone --audio <file> --speaker-id <id>");
        println!();
        println!("This example shows how to:");
        println!("  1. Train a custom voice from an audio file");
        println!("  2. Query the training status");
        println!();
        
        // Show code example
        show_example();
        return Ok(());
    };

    if audio_path.is_empty() || speaker_id.is_empty() {
        eprintln!("Error: Both --audio and --speaker-id are required");
        std::process::exit(1);
    }

    // Create client
    let app_id = env::var("DOUBAO_APP_ID").expect("DOUBAO_APP_ID not set");
    let access_token = env::var("DOUBAO_ACCESS_TOKEN").expect("DOUBAO_ACCESS_TOKEN not set");

    let client = Client::builder(&app_id)
        .bearer_token(&access_token)
        .build()?;

    // Read audio file
    println!("Reading audio file: {}", audio_path);
    let audio_data = std::fs::read(&audio_path)?;
    println!("Audio size: {} bytes", audio_data.len());

    // Create training request
    let req = VoiceCloneTrainRequest {
        speaker_id: speaker_id.clone(),
        audio_data: Some(audio_data),
        ..Default::default()
    };

    // Start training
    println!("\nStarting voice clone training...");
    let voice_clone = client.voice_clone();
    let result = voice_clone.train(&req).await?;

    println!("Training started!");
    println!("  Speaker ID: {}", result.speaker_id);
    println!("  Status: {:?}", result.status);

    // Query status
    println!("\nQuerying training status...");
    let status = voice_clone.get_status(&speaker_id).await?;

    println!("Current status:");
    println!("  Speaker ID: {}", status.speaker_id);
    println!("  Status: {:?}", status.status);
    if let Some(ref demo_audio) = status.demo_audio {
        println!("  Demo Audio: {}", demo_audio);
    }

    Ok(())
}

fn show_example() {
    println!("Example code:");
    println!();
    println!(r#"
use giztoy_doubaospeech::{{Client, VoiceCloneTrainRequest}};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {{
    // Create client
    let client = Client::builder("your_app_id")
        .bearer_token("your_access_token")
        .build()?;

    // Read audio file
    let audio_data = std::fs::read("sample.wav")?;

    // Create training request
    let req = VoiceCloneTrainRequest {{
        speaker_id: "my_custom_voice".to_string(),
        audio_data: Some(audio_data),
        ..Default::default()
    }};

    // Start training
    let voice_clone = client.voice_clone();
    let result = voice_clone.train(&req).await?;
    println!("Training started: {{:?}}", result);

    // Query status
    let status = voice_clone.get_status("my_custom_voice").await?;
    println!("Status: {{:?}}", status);

    Ok(())
}}
"#);
}
