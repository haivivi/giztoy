//! Translation (simultaneous interpretation) example.
//!
//! This example demonstrates how to use the Doubao Speech SDK for real-time translation.
//!
//! Run with:
//! ```bash
//! export DOUBAO_APP_ID="your-app-id"
//! export DOUBAO_ACCESS_TOKEN="your-token"
//!
//! # Translate Chinese audio to English
//! cargo run --bin translation -- path/to/chinese_audio.pcm zh-CN en-US
//! ```

use std::env;

use giztoy_doubaospeech::{
    AudioFormat, Client, Language, SampleRate, TranslationAudioConfig, TranslationConfig,
};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Get credentials from environment
    let app_id = env::var("DOUBAO_APP_ID").expect("DOUBAO_APP_ID environment variable not set");
    let api_key = env::var("DOUBAO_API_KEY").ok();
    let access_token = env::var("DOUBAO_ACCESS_TOKEN").ok();
    let cluster =
        env::var("DOUBAO_CLUSTER").unwrap_or_else(|_| "volcengine_streaming_common".to_string());

    // Get command line arguments
    let args: Vec<String> = env::args().collect();
    let audio_path = args.get(1).map(|s| s.as_str()).unwrap_or("audio.pcm");
    let source_lang = args.get(2).map(|s| s.as_str()).unwrap_or("zh-CN");
    let target_lang = args.get(3).map(|s| s.as_str()).unwrap_or("en-US");

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

    println!("=== Streaming Translation ===");
    println!("Audio file: {}", audio_path);
    println!("Source language: {}", source_lang);
    println!("Target language: {}", target_lang);

    // Read audio file (should be PCM format)
    let audio_data = std::fs::read(audio_path)?;
    println!("Audio size: {} bytes", audio_data.len());

    // Parse languages
    let source_language = match source_lang {
        "zh-CN" => Language::ZhCn,
        "en-US" => Language::EnUs,
        "ja-JP" => Language::JaJp,
        _ => Language::ZhCn,
    };
    let target_language = match target_lang {
        "zh-CN" => Language::ZhCn,
        "en-US" => Language::EnUs,
        "ja-JP" => Language::JaJp,
        _ => Language::EnUs,
    };

    let config = TranslationConfig {
        source_language,
        target_language,
        audio_config: TranslationAudioConfig {
            format: AudioFormat::Pcm,
            sample_rate: SampleRate::Rate16000,
            channel: 1,
            bits: 16,
        },
        enable_tts: false,
        tts_voice: None,
    };

    println!("\nOpening translation session...");
    let session = client.translation().open_session(&config).await?;

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

    println!("\nReceiving translations...");
    println!("---");
    while let Some(result) = session.recv().await {
        match result {
            Ok(chunk) => {
                if !chunk.source_text.is_empty() || !chunk.target_text.is_empty() {
                    println!(
                        "[{}] {} -> {}",
                        if chunk.is_final { "FINAL" } else { "PARTIAL" },
                        chunk.source_text,
                        chunk.target_text
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
    println!("---");

    session.close().await?;

    println!("\nDone!");
    Ok(())
}
