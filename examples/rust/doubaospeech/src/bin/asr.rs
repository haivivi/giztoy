//! ASR (Automatic Speech Recognition) example.
//!
//! This example demonstrates how to use the Doubao Speech SDK for ASR.
//!
//! Run with:
//! ```bash
//! export DOUBAO_APP_ID="your-app-id"
//! export DOUBAO_API_KEY="your-api-key"
//! # or for streaming:
//! export DOUBAO_ACCESS_TOKEN="your-token"
//!
//! # One-sentence recognition
//! cargo run --bin asr -- one-sentence path/to/audio.wav
//!
//! # Streaming recognition
//! cargo run --bin asr -- stream path/to/audio.pcm
//! ```

use std::env;

use giztoy_doubaospeech::{AudioFormat, Client, OneSentenceRequest, SampleRate, StreamAsrConfig};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Get credentials from environment
    let app_id = env::var("DOUBAO_APP_ID").expect("DOUBAO_APP_ID environment variable not set");
    let api_key = env::var("DOUBAO_API_KEY").ok();
    let access_token = env::var("DOUBAO_ACCESS_TOKEN").ok();
    let cluster =
        env::var("DOUBAO_CLUSTER").unwrap_or_else(|_| "volcengine_streaming_common".to_string());

    // Get command and audio file path from command line
    let args: Vec<String> = env::args().collect();
    let mode = args.get(1).map(|s| s.as_str()).unwrap_or("one-sentence");
    let audio_path = args.get(2).map(|s| s.as_str()).unwrap_or("audio.wav");

    // Create client
    let mut builder = Client::builder(&app_id).cluster(&cluster);
    if let Some(key) = api_key {
        builder = builder.api_key(&key);
    } else if let Some(token) = access_token {
        builder = builder.bearer_token(&token);
    } else {
        panic!("Either DOUBAO_API_KEY or DOUBAO_ACCESS_TOKEN must be set");
    }
    let client = builder.build()?;

    match mode {
        "one-sentence" => {
            // Example: One-sentence recognition
            println!("=== One-sentence ASR recognition ===");
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
        }
        "stream" => {
            // Example: Streaming recognition
            println!("=== Streaming ASR recognition ===");
            println!("Audio file: {}", audio_path);

            // Read audio file (should be PCM format)
            let audio_data = std::fs::read(audio_path)?;
            println!("Audio size: {} bytes", audio_data.len());

            let config = StreamAsrConfig {
                format: AudioFormat::Pcm,
                sample_rate: SampleRate::Rate16000,
                bits: 16,
                channel: 1,
                show_utterances: Some(true),
                ..Default::default()
            };

            println!("Opening streaming session...");
            let session = client.asr().open_stream_session(&config).await?;

            // Send audio in chunks (100ms each for 16kHz 16-bit mono)
            let chunk_size = 3200;
            let chunks: Vec<&[u8]> = audio_data.chunks(chunk_size).collect();
            let total_chunks = chunks.len();

            println!("Sending {} audio chunks...", total_chunks);
            for (i, chunk) in chunks.iter().enumerate() {
                let is_last = i == total_chunks - 1;
                session.send_audio(chunk, is_last).await?;
                if !is_last {
                    tokio::time::sleep(tokio::time::Duration::from_millis(100)).await;
                }
            }

            println!("\nReceiving results...");
            while let Some(result) = session.recv().await {
                match result {
                    Ok(chunk) => {
                        if !chunk.text.is_empty() {
                            println!(
                                "[{}] {}",
                                if chunk.is_final { "FINAL" } else { "PARTIAL" },
                                chunk.text
                            );
                        }
                        if chunk.is_final {
                            break;
                        }
                    }
                    Err(e) => {
                        eprintln!("Error: {}", e);
                        break;
                    }
                }
            }

            session.close().await?;
        }
        _ => {
            println!("Usage: asr <mode> <audio_file>");
            println!("Modes:");
            println!("  one-sentence  - Recognize short audio (< 60s)");
            println!("  stream        - Real-time streaming recognition");
        }
    }

    println!("\nDone!");
    Ok(())
}
