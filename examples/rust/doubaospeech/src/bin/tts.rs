//! TTS (Text-to-Speech) synthesis example.
//!
//! This example demonstrates how to use the Doubao Speech SDK for TTS synthesis.
//!
//! Run with:
//! ```bash
//! export DOUBAO_APP_ID="your-app-id"
//! export DOUBAO_API_KEY="your-api-key"
//! cargo run --bin tts
//! ```

use std::env;
use std::pin::pin;

use futures::StreamExt;
use giztoy_doubaospeech::{AudioEncoding, Client, TtsRequest};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Get credentials from environment
    let app_id = env::var("DOUBAO_APP_ID").expect("DOUBAO_APP_ID environment variable not set");
    let api_key = env::var("DOUBAO_API_KEY").expect("DOUBAO_API_KEY environment variable not set");
    let cluster = env::var("DOUBAO_CLUSTER").unwrap_or_else(|_| "volcano_tts".to_string());

    // Create client
    let client = Client::builder(&app_id)
        .api_key(&api_key)
        .cluster(&cluster)
        .build()?;

    // Example 1: Simple TTS synthesis
    println!("Example 1: Simple TTS synthesis");

    let request = TtsRequest {
        text: "你好，这是一个语音合成的示例。Hello, this is a TTS example.".to_string(),
        voice_type: "zh_female_cancan".to_string(),
        encoding: Some(AudioEncoding::Mp3),
        speed_ratio: Some(1.0),
        ..Default::default()
    };

    let response = client.tts().synthesize(&request).await?;

    // Save audio to file
    std::fs::write("output_simple.mp3", &response.audio)?;
    println!("Audio saved to output_simple.mp3 ({} bytes)", response.audio.len());
    println!("Duration: {} ms", response.duration);
    println!("Request ID: {}", response.req_id);

    // Example 2: Streaming TTS synthesis
    println!("\nExample 2: Streaming TTS synthesis");

    let stream_request = TtsRequest {
        text: "这是一个流式语音合成的示例，音频会边生成边返回，非常适合实时播放场景。".to_string(),
        voice_type: "zh_female_cancan".to_string(),
        encoding: Some(AudioEncoding::Mp3),
        ..Default::default()
    };

    let mut audio_data = Vec::new();
    let tts = client.tts();
    let stream = tts.synthesize_stream(&stream_request).await?;
    let mut stream = pin!(stream);

    let mut chunk_count = 0;
    while let Some(chunk) = stream.next().await {
        let chunk = chunk?;
        audio_data.extend_from_slice(&chunk.audio);
        chunk_count += 1;
        print!(".");
        
        if chunk.is_last {
            println!(" (last chunk)");
            break;
        }
    }

    std::fs::write("output_stream.mp3", &audio_data)?;
    println!(
        "Audio saved to output_stream.mp3 ({} bytes, {} chunks)",
        audio_data.len(),
        chunk_count
    );

    println!("\nDone!");
    Ok(())
}
