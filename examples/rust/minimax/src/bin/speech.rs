//! Speech synthesis example.
//!
//! This example demonstrates how to use the MiniMax SDK for speech synthesis.
//!
//! Run with:
//! ```bash
//! export MINIMAX_API_KEY="your-api-key"
//! cargo run --example minimax-speech
//! ```

use std::env;
use std::pin::pin;

use giztoy_minimax::{
    AudioSetting, Client, SpeechRequest, VoiceSetting,
    MODEL_SPEECH_26_HD, VOICE_FEMALE_SHAONV,
};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Get API key from environment
    let api_key = env::var("MINIMAX_API_KEY")
        .expect("MINIMAX_API_KEY environment variable not set");

    // Create client
    let client = Client::new(api_key)?;

    // Example 1: Simple speech synthesis
    println!("Example 1: Simple speech synthesis");
    
    let request = SpeechRequest {
        model: MODEL_SPEECH_26_HD.to_string(),
        text: "你好，这是一个语音合成的示例。Hello, this is a speech synthesis example.".to_string(),
        voice_setting: Some(VoiceSetting {
            voice_id: VOICE_FEMALE_SHAONV.to_string(),
            speed: Some(1.0),
            ..Default::default()
        }),
        audio_setting: Some(AudioSetting {
            sample_rate: Some(32000),
            ..Default::default()
        }),
        ..Default::default()
    };

    let response = client.speech().synthesize(&request).await?;
    
    // Save audio to file
    std::fs::write("output_simple.mp3", &response.audio)?;
    println!("Audio saved to output_simple.mp3 ({} bytes)", response.audio.len());
    
    if let Some(info) = &response.extra_info {
        println!("Duration: {} ms", info.audio_length);
        println!("Sample rate: {}", info.audio_sample_rate);
    }

    // Example 2: Streaming speech synthesis
    println!("\nExample 2: Streaming speech synthesis");
    
    use futures::StreamExt;
    
    let stream_request = SpeechRequest {
        model: MODEL_SPEECH_26_HD.to_string(),
        text: "这是一个流式语音合成的示例，音频会边生成边返回。".to_string(),
        voice_setting: Some(VoiceSetting {
            voice_id: VOICE_FEMALE_SHAONV.to_string(),
            ..Default::default()
        }),
        ..Default::default()
    };

    let mut audio_data = Vec::new();
    let speech_client = client.speech();
    let stream = speech_client.synthesize_stream(&stream_request).await?;
    let mut stream = pin!(stream);
    
    let mut chunk_count = 0;
    while let Some(chunk) = stream.next().await {
        let chunk = chunk?;
        audio_data.extend_from_slice(&chunk.audio);
        chunk_count += 1;
        print!(".");
    }
    println!();
    
    std::fs::write("output_stream.mp3", &audio_data)?;
    println!("Audio saved to output_stream.mp3 ({} bytes, {} chunks)", audio_data.len(), chunk_count);

    println!("\nDone!");
    Ok(())
}
