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

/// Test hybrid mode: MiniMax TTS -> Doubao ASR -> MiniMax LLM -> MiniMax TTS
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

    println!("  Full hybrid pipeline:");
    println!("  MiniMax TTS → Doubao ASR → MiniMax LLM → MiniMax TTS");
    println!();

    // Step 1: Use MiniMax TTS to generate audio for a question
    let user_question = "明天北京的天气怎么样";
    println!("  [Step 1] MiniMax TTS: Generating audio for '{}'", user_question);

    let tts_resp1 = mm_client
        .speech()
        .synthesize(&SpeechRequest {
            model: "speech-01-turbo".to_string(),
            text: user_question.to_string(),
            voice_setting: Some(VoiceSetting {
                voice_id: "female-shaonv".to_string(),
                ..Default::default()
            }),
            audio_setting: Some(AudioSetting {
                format: Some(AudioFormat::Pcm),
                sample_rate: Some(16000),
                ..Default::default()
            }),
            ..Default::default()
        })
        .await?;

    println!("  ✅ Generated audio: {} bytes (PCM 16kHz)", tts_resp1.audio.len());

    // Save the question audio
    fs::create_dir_all("tmp")?;
    fs::write("tmp/hybrid_question_rust.pcm", &tts_resp1.audio)?;
    println!("  ✅ Saved: tmp/hybrid_question_rust.pcm");

    // Step 2: Use Doubao ASR to recognize the audio
    println!("\n  [Step 2] Doubao ASR: Recognizing audio...");

    use giztoy_doubaospeech::{AudioFormat as DsAudioFormat, Language, OneSentenceRequest, SampleRate};

    let asr_resp = ds_client
        .asr()
        .recognize_one_sentence(&OneSentenceRequest {
            audio: Some(tts_resp1.audio.clone()),
            format: DsAudioFormat::Pcm,
            sample_rate: Some(SampleRate::Rate16000),
            language: Some(Language::ZhCn),
            ..Default::default()
        })
        .await?;

    let recognized_text = &asr_resp.text;
    println!("  ✅ ASR Result: '{}'", recognized_text);

    // Step 3: Use MiniMax LLM to process the recognized text
    println!("\n  [Step 3] MiniMax LLM: Processing recognized text...");

    let llm_resp = mm_client
        .text()
        .create_chat_completion(&ChatCompletionRequest {
            model: "MiniMax-Text-01".to_string(),
            messages: vec![
                Message::system("你是一个天气助手，简短回答天气问题。回答不要超过50字。"),
                Message::user(recognized_text),
            ],
            ..Default::default()
        })
        .await?;

    let llm_text = llm_resp
        .choices
        .first()
        .and_then(|c| c.message.content_str())
        .unwrap_or("");

    println!("  ✅ LLM Response: '{}'", truncate(llm_text, 100));

    // Step 4: Use MiniMax TTS to synthesize the response
    println!("\n  [Step 4] MiniMax TTS: Synthesizing response...");

    let tts_resp2 = mm_client
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

    println!("  ✅ Response audio: {} bytes", tts_resp2.audio.len());

    fs::write("tmp/hybrid_response_rust.mp3", &tts_resp2.audio)?;
    println!("  ✅ Saved: tmp/hybrid_response_rust.mp3");

    println!("\n  ========================================");
    println!("  Hybrid Pipeline Summary:");
    println!("  Input Question:    '{}'", user_question);
    println!("  ASR Recognized:    '{}'", recognized_text);
    println!("  LLM Response:      '{}'", truncate(llm_text, 60));
    println!("  Output Audio:      tmp/hybrid_response_rust.mp3");
    println!("  ========================================");

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
