// E2E Test: Doubao Realtime Basic
// 对应 Go: e2e/genx/transformers/doubao_realtime_basic/main.go
//
// 运行方式:
//   export DOUBAO_APP_ID=xxx
//   export DOUBAO_TOKEN=xxx
//   bazel run //e2e/genx/transformers:doubao_realtime_basic

use std::env;
use std::time::Duration;

use clap::Parser;
use giztoy_genx::stream::Stream;
use giztoy_genx::transformers::{DoubaoRealtime, DoubaoRealtimeOptions};
use giztoy_genx::types::{MessageChunk, Part, Role};

/// Doubao Realtime Basic E2E Test
#[derive(Parser)]
#[command(name = "doubao_realtime_basic")]
#[command(about = "Basic E2E test for Doubao Realtime transformer")]
struct Args {
    /// TTS speaker voice
    #[arg(long, default_value = "zh_female_vv_jupiter_bigtts")]
    speaker: String,

    /// Test timeout in seconds
    #[arg(long, default_value = "120")]
    timeout: u64,
}

// 测试句子
const TEST_SENTENCES: &[&str] = &[
    "你好，请用一句话介绍自己。",
    "今天天气怎么样？",
    "给我讲一个笑话。",
];

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args = Args::parse();

    println!("=== Doubao Realtime Basic Test ===");
    println!("Speaker: {}", args.speaker);
    println!();

    // 获取 API 凭证
    let app_id = env::var("DOUBAO_APP_ID")
        .expect("DOUBAO_APP_ID environment variable required");
    let token = env::var("DOUBAO_TOKEN")
        .expect("DOUBAO_TOKEN environment variable required");

    // 创建 Doubao client (假设 DoubaoClient 已实现)
    let doubao_client = DoubaoClient::new(&app_id, &token)?;

    // 创建 DoubaoRealtime transformer
    let realtime = DoubaoRealtime::new(
        doubao_client,
        DoubaoRealtimeOptions {
            speaker: args.speaker.clone(),
            format: "pcm_s16le".to_string(),
            sample_rate: 24000,
            channels: 1,
            system_role: Some("你是一个友好的助手，用简短的话回答问题。".to_string()),
            ..Default::default()
        },
    );

    // 初始化音频播放 (假设 portaudio 包装已实现)
    // let speaker_out = PortAudioOutput::new(24000, 1, Duration::from_millis(20))?;

    // 处理每个测试句子
    for (i, sentence) in TEST_SENTENCES.iter().enumerate() {
        println!("[Turn {}] Input: {}", i + 1, sentence);

        // 创建文本输入流
        let input = text_to_stream(sentence);

        // Transform through Doubao Realtime
        let mut output = realtime
            .transform("doubao/realtime", input)
            .await
            .map_err(|e| format!("Transform error: {}", e))?;

        // 收集结果
        let mut llm_text = String::new();
        let mut audio_bytes = 0usize;

        while let Ok(Some(chunk)) = output.next().await {
            match chunk.role {
                Role::User => {
                    // ASR result (for audio input mode)
                    if let Some(Part::Text(text)) = chunk.part {
                        if !text.is_empty() {
                            println!("  ASR: {}", text);
                        }
                    }
                }
                Role::Model => {
                    if let Some(Part::Text(text)) = chunk.part {
                        llm_text.push_str(&text);
                    } else if let Some(Part::Blob(blob)) = chunk.part {
                        if !blob.data.is_empty() {
                            audio_bytes += blob.data.len();
                            // 播放音频 (24kHz PCM)
                            // speaker_out.write(&blob.data)?;
                        }
                    }
                }
                _ => {}
            }
        }

        let audio_sec = audio_bytes as f64 / 48000.0; // 24kHz 16-bit mono
        println!("  LLM: {}", truncate(&llm_text, 80));
        println!("  Audio: {:.2}s", audio_sec);
        println!();

        // 验证：必须有 LLM 文本和音频输出
        assert!(
            !llm_text.is_empty(),
            "Turn {}: LLM text should not be empty",
            i + 1
        );
        assert!(
            audio_bytes > 0,
            "Turn {}: Audio bytes should be > 0",
            i + 1
        );
    }

    println!("=== Test Complete ===");
    Ok(())
}

/// Convert text to a single-chunk stream
fn text_to_stream(text: &str) -> Box<dyn Stream> {
    let chunks = vec![
        MessageChunk::text(Role::User, text),
    ];
    Box::new(VecStream::new(chunks))
}

/// Simple stream implementation from a Vec
struct VecStream {
    chunks: Vec<MessageChunk>,
    index: usize,
}

impl VecStream {
    fn new(chunks: Vec<MessageChunk>) -> Self {
        Self { chunks, index: 0 }
    }
}

#[async_trait::async_trait]
impl Stream for VecStream {
    async fn next(&mut self) -> Result<Option<MessageChunk>, giztoy_genx::error::GenxError> {
        if self.index < self.chunks.len() {
            let chunk = self.chunks[self.index].clone();
            self.index += 1;
            Ok(Some(chunk))
        } else {
            Ok(None)
        }
    }

    fn result(&self) -> Option<giztoy_genx::stream::StreamResult> {
        None
    }

    async fn close(&mut self) -> Result<(), giztoy_genx::error::GenxError> {
        Ok(())
    }

    async fn close_with_error(
        &mut self,
        _error: giztoy_genx::error::GenxError,
    ) -> Result<(), giztoy_genx::error::GenxError> {
        Ok(())
    }
}

fn truncate(s: &str, max_len: usize) -> String {
    if s.chars().count() <= max_len {
        s.to_string()
    } else {
        format!("{}...", &s[..s.char_indices().nth(max_len).unwrap_or((s.len(), ' ')).0])
    }
}
