//! ASR (Automatic Speech Recognition) example.
//!
//! This example demonstrates how to use the Doubao Speech SDK for ASR.
//!
//! Run with:
//! ```bash
//! export DOUBAO_APP_ID="your-app-id"
//! export DOUBAO_API_KEY="your-api-key"
//! cargo run --bin asr -- path/to/audio.wav
//! ```

use std::env;

use giztoy_doubaospeech::{AudioFormat, Client, OneSentenceRequest, SampleRate};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Get credentials from environment
    let app_id = env::var("DOUBAO_APP_ID").expect("DOUBAO_APP_ID environment variable not set");
    let api_key = env::var("DOUBAO_API_KEY").expect("DOUBAO_API_KEY environment variable not set");
    let cluster = env::var("DOUBAO_CLUSTER").unwrap_or_else(|_| "volcengine_streaming_common".to_string());

    // Get audio file path from command line
    let args: Vec<String> = env::args().collect();
    let audio_path = args.get(1).map(|s| s.as_str()).unwrap_or("audio.wav");

    // Create client
    let client = Client::builder(&app_id)
        .api_key(&api_key)
        .cluster(&cluster)
        .build()?;

    // Example: One-sentence recognition
    println!("One-sentence ASR recognition");
    println!("Audio file: {}", audio_path);

    // Read audio file
    let audio_data = std::fs::read(audio_path)?;
    println!("Audio size: {} bytes", audio_data.len());

    let request = OneSentenceRequest {
        audio: Some(audio_data),
        format: AudioFormat::Wav,
        sample_rate: Some(SampleRate::Rate16000),
        enable_itn: Some(true),
        enable_punc: Some(true),
        ..Default::default()
    };

    let response = client.asr().recognize_one_sentence(&request).await?;

    println!("\nRecognition result:");
    println!("Text: {}", response.text);
    println!("Duration: {} ms", response.duration);

    println!("\nDone!");
    Ok(())
}
