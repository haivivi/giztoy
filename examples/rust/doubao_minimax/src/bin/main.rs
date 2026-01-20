//! Doubao Realtime + MiniMax Integration Test
//!
//! This example demonstrates multi-turn conversation testing using:
//! 1. Doubao Realtime for Speech-to-Speech dialogue
//! 2. MiniMax LLM for text generation (optional)
//! 3. MiniMax TTS for speech synthesis (optional)
//!
//! Usage:
//!
//! ```bash
//! export DOUBAO_APP_ID="your_app_id"
//! export DOUBAO_TOKEN="your_token"
//! export MINIMAX_API_KEY="your_api_key"
//! export MINIMAX_GROUP_ID="your_group_id"  # optional
//! cargo run --bin doubao_minimax
//! ```

use std::env;
use std::fs;

use anyhow::Result;
// Note: MiniMax Rust SDK does not support group_id parameter
use giztoy_doubaospeech::{
    Client as DoubaoClient, RealtimeAudioConfig, RealtimeConfig, RealtimeDialogConfig,
    RealtimeEventType, RealtimeTTSConfig,
};
use giztoy_minimax::{
    AudioFormat, AudioSetting, ChatCompletionRequest, Client as MinimaxClient, Message,
    SpeechRequest, VoiceSetting,
};

#[tokio::main]
async fn main() -> Result<()> {
    // Doubao configuration
    let doubao_app_id = env::var("DOUBAO_APP_ID").ok();
    let doubao_token = env::var("DOUBAO_TOKEN").ok();

    // MiniMax configuration
    let minimax_api_key = env::var("MINIMAX_API_KEY").ok();
    let minimax_group_id = env::var("MINIMAX_GROUP_ID").ok();

    println!("========================================");
    println!("  Doubao Realtime + MiniMax Test (Rust)");
    println!("========================================");
    println!();

    // Test 1: Doubao Realtime basic test
    if let (Some(app_id), Some(token)) = (&doubao_app_id, &doubao_token) {
        println!("Doubao App ID: {}", app_id);
        println!("Doubao Token: {}...", truncate(token, 10));

        println!("\n[Test 1] Doubao Realtime Basic Test");
        println!("------------------------------------");
        test_doubao_realtime(app_id, token).await?;

        println!("\n[Test 2] Multi-turn Conversation");
        println!("---------------------------------");
        test_multi_turn_conversation(app_id, token).await?;
    } else {
        println!("[Test 1 & 2] Skipped (DOUBAO_APP_ID or DOUBAO_TOKEN not set)");
    }

    // Test 3: MiniMax integration
    if let Some(api_key) = &minimax_api_key {
        println!("\nMiniMax API Key: {}...", truncate(api_key, 10));
        if let Some(group_id) = &minimax_group_id {
            println!("MiniMax Group ID: {}", group_id);
        }

        println!("\n[Test 3] MiniMax LLM + TTS Test");
        println!("-------------------------------");
        test_minimax(api_key, minimax_group_id.as_deref()).await?;

        // Test 4: Hybrid mode
        if let (Some(app_id), Some(token)) = (&doubao_app_id, &doubao_token) {
            println!("\n[Test 4] Hybrid Mode (Doubao + MiniMax)");
            println!("---------------------------------------");
            test_hybrid_mode(app_id, token, api_key, minimax_group_id.as_deref()).await?;
        }
    } else {
        println!("\n[Test 3 & 4] Skipped (MINIMAX_API_KEY not set)");
    }

    println!("\n========================================");
    println!("  All tests completed!");
    println!("========================================");

    Ok(())
}

/// Test basic Doubao Realtime functionality
async fn test_doubao_realtime(app_id: &str, token: &str) -> Result<()> {
    let client = DoubaoClient::builder(app_id).bearer_token(token).build()?;

    let config = RealtimeConfig {
        tts: RealtimeTTSConfig {
            audio_config: RealtimeAudioConfig {
                channel: 1,
                format: "mp3".to_string(),
                sample_rate: 24000,
            },
            ..Default::default()
        },
        dialog: RealtimeDialogConfig {
            bot_name: "小豆".to_string(),
            system_role: "你是一个友好的助手，回答要简短精炼".to_string(),
            ..Default::default()
        },
        ..Default::default()
    };

    println!("  Connecting to Doubao Realtime...");
    let session = client.realtime().connect(&config).await?;
    println!("  ✅ Connected!");

    // Send greeting
    println!("  Sending: 你好，今天天气怎么样？");
    session.send_text("你好，今天天气怎么样？").await?;

    // Receive response
    let mut audio_size = 0;
    let mut response_text = String::new();

    while let Some(event_result) = session.recv().await {
        match event_result {
            Ok(event) => {
                if let Some(ref audio) = event.audio {
                    audio_size += audio.len();
                }
                if !event.text.is_empty() {
                    response_text.push_str(&event.text);
                }

                if event.event_type == Some(RealtimeEventType::TTSFinished) {
                    break;
                }
            }
            Err(e) => {
                println!("  Error: {}", e);
                break;
            }
        }
    }

    println!("  ✅ Response: {}", truncate(&response_text, 50));
    println!("  ✅ Audio received: {} bytes", audio_size);

    Ok(())
}

/// Test multi-turn conversation
async fn test_multi_turn_conversation(app_id: &str, token: &str) -> Result<()> {
    let client = DoubaoClient::builder(app_id).bearer_token(token).build()?;

    let config = RealtimeConfig {
        tts: RealtimeTTSConfig {
            audio_config: RealtimeAudioConfig {
                channel: 1,
                format: "mp3".to_string(),
                sample_rate: 24000,
            },
            ..Default::default()
        },
        dialog: RealtimeDialogConfig {
            bot_name: "小豆".to_string(),
            system_role: "你是一个知识丰富的助手，擅长回答各种问题".to_string(),
            ..Default::default()
        },
        ..Default::default()
    };

    println!("  Connecting...");
    let session = client.realtime().connect(&config).await?;
    println!("  ✅ Connected!");

    // Multi-turn conversation
    let turns = [
        "你好，请用一句话介绍一下自己",
        "北京有哪些著名的景点？",
        "长城有多长？",
    ];

    for (i, question) in turns.iter().enumerate() {
        println!("\n  [Turn {}] User: {}", i + 1, question);

        session.send_text(question).await?;

        let mut response_text = String::new();
        let mut audio_size = 0;

        while let Some(event_result) = session.recv().await {
            match event_result {
                Ok(event) => {
                    if let Some(ref audio) = event.audio {
                        audio_size += audio.len();
                    }
                    if !event.text.is_empty() {
                        response_text.push_str(&event.text);
                    }

                    if event.event_type == Some(RealtimeEventType::TTSFinished) {
                        break;
                    }
                }
                Err(e) => {
                    println!("  Error: {}", e);
                    break;
                }
            }
        }

        println!("  [Turn {}] Bot: {}", i + 1, truncate(&response_text, 100));
        println!("  Audio: {} bytes", audio_size);
    }

    println!("\n  ✅ Multi-turn conversation completed!");

    Ok(())
}

/// Test MiniMax LLM and TTS
async fn test_minimax(api_key: &str, _group_id: Option<&str>) -> Result<()> {
    // Note: MiniMax Rust SDK does not support group_id parameter
    let client = MinimaxClient::builder(api_key).build()?;

    // Test LLM
    println!("  Testing MiniMax LLM...");
    let llm_resp = client
        .text()
        .create_chat_completion(&ChatCompletionRequest {
            model: "MiniMax-Text-01".to_string(),
            messages: vec![
                Message::system("你是一个友好的助手"),
                Message::user("请用一句话介绍你自己"),
            ],
            ..Default::default()
        })
        .await?;

    let llm_text = llm_resp
        .choices
        .first()
        .and_then(|c| c.message.content_str())
        .unwrap_or("");

    println!("  ✅ LLM Response: {}", truncate(llm_text, 80));

    // Test TTS
    println!("  Testing MiniMax TTS...");
    let tts_resp = client
        .speech()
        .synthesize(&SpeechRequest {
            model: "speech-01-turbo".to_string(),
            text: llm_text.to_string(),
            voice_setting: Some(VoiceSetting {
                voice_id: "female-shaonv".to_string(),
                ..Default::default()
            }),
            audio_setting: Some(AudioSetting {
                format: Some(AudioFormat::Mp3),
                sample_rate: Some(24000),
                ..Default::default()
            }),
            ..Default::default()
        })
        .await?;

    println!("  ✅ TTS Audio: {} bytes", tts_resp.audio.len());

    // Save audio
    fs::create_dir_all("tmp")?;
    fs::write("tmp/minimax_tts_rust.mp3", &tts_resp.audio)?;
    println!("  ✅ Audio saved: tmp/minimax_tts_rust.mp3");

    Ok(())
}

/// Test hybrid mode: Doubao + MiniMax
async fn test_hybrid_mode(
    doubao_app_id: &str,
    doubao_token: &str,
    minimax_api_key: &str,
    _minimax_group_id: Option<&str>,
) -> Result<()> {
    // Create MiniMax client (Note: group_id not supported in Rust SDK)
    let mm_client = MinimaxClient::builder(minimax_api_key).build()?;

    // Create Doubao client
    let ds_client = DoubaoClient::builder(doubao_app_id)
        .bearer_token(doubao_token)
        .build()?;

    println!("  Testing full hybrid pipeline...");
    println!("  (Doubao Realtime) <-> (MiniMax LLM) -> (MiniMax TTS)");

    // Simulate user input (in real use, this would come from Doubao ASR)
    let test_input = "帮我查一下明天北京的天气";
    println!("  User Input: {}", test_input);

    // Use MiniMax LLM to process
    let llm_resp = mm_client
        .text()
        .create_chat_completion(&ChatCompletionRequest {
            model: "MiniMax-Text-01".to_string(),
            messages: vec![
                Message::system("你是一个天气助手，简短回答天气问题"),
                Message::user(test_input),
            ],
            ..Default::default()
        })
        .await?;

    let llm_text = llm_resp
        .choices
        .first()
        .and_then(|c| c.message.content_str())
        .unwrap_or("");

    println!("  LLM Response: {}", truncate(llm_text, 100));

    // Use MiniMax TTS to synthesize
    let tts_resp = mm_client
        .speech()
        .synthesize(&SpeechRequest {
            model: "speech-01-turbo".to_string(),
            text: llm_text.to_string(),
            voice_setting: Some(VoiceSetting {
                voice_id: "female-shaonv".to_string(),
                ..Default::default()
            }),
            audio_setting: Some(AudioSetting {
                format: Some(AudioFormat::Mp3),
                sample_rate: Some(24000),
                ..Default::default()
            }),
            ..Default::default()
        })
        .await?;

    println!("  TTS Audio: {} bytes", tts_resp.audio.len());

    fs::write("tmp/hybrid_pipeline_rust.mp3", &tts_resp.audio)?;
    println!("  ✅ Audio saved: tmp/hybrid_pipeline_rust.mp3");

    // Also test Doubao Realtime connection
    println!("\n  Testing Doubao Realtime connection...");
    let config = RealtimeConfig {
        dialog: RealtimeDialogConfig {
            bot_name: "Test".to_string(),
            system_role: "你是一个助手".to_string(),
            ..Default::default()
        },
        ..Default::default()
    };

    let session = ds_client.realtime().connect(&config).await?;
    println!("  ✅ Doubao Realtime connected!");

    // Send a test message
    session.send_text("你好").await?;

    // Wait for response
    let mut got_response = false;
    while let Some(event_result) = session.recv().await {
        match event_result {
            Ok(event) => {
                if !event.text.is_empty() || event.audio.is_some() {
                    got_response = true;
                }
                if event.event_type == Some(RealtimeEventType::TTSFinished) {
                    break;
                }
            }
            Err(_) => break,
        }
    }

    if got_response {
        println!("  ✅ Doubao Realtime response received!");
    }

    println!("\n  ✅ Hybrid mode test completed!");

    Ok(())
}

/// Truncate a string to maxLen characters
fn truncate(s: &str, max_len: usize) -> String {
    let chars: Vec<char> = s.chars().collect();
    if chars.len() <= max_len {
        s.to_string()
    } else {
        format!("{}...", chars[..max_len].iter().collect::<String>())
    }
}
