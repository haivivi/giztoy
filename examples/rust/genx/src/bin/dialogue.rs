//! Dialogue example - OpenAI and Gemini conversation.
//!
//! This example demonstrates two AI models having a conversation with each other.
//! OpenAI starts the conversation, and then they take turns responding.

use anyhow::{Context, Result};
use giztoy_genx::gemini::{GeminiConfig, GeminiGenerator};
use giztoy_genx::openai::{OpenAIConfig, OpenAIGenerator};
use giztoy_genx::{stream::collect_text, Generator, ModelContextBuilder};
use std::env;
use tracing::{info, Level};
use tracing_subscriber::FmtSubscriber;

const SYSTEM_PROMPT: &str = r#"你正在参与一场哲学对话。请用简洁有趣的方式回应对方的观点，
每次回复控制在2-3句话以内。可以提出反驳、补充或新的问题来推进讨论。
使用中文回复。"#;

const INITIAL_TOPIC: &str = "你认为人工智能能够真正理解人类的情感吗？请分享你的看法。";

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize logging
    let subscriber = FmtSubscriber::builder()
        .with_max_level(Level::INFO)
        .finish();
    tracing::subscriber::set_global_default(subscriber)?;

    // Read API keys from environment variables
    let openai_api_key = env::var("OPENAI_API_KEY")
        .context("OPENAI_API_KEY environment variable not set")?;
    let gemini_api_key = env::var("GEMINI_API_KEY")
        .context("GEMINI_API_KEY environment variable not set")?;

    info!("🚀 Starting AI Dialogue: OpenAI vs Gemini");
    println!("\n============================================================");
    println!("  🤖 AI 哲学对话：OpenAI vs Gemini");
    println!("============================================================\n");

    // Create generators
    let openai = OpenAIGenerator::new(OpenAIConfig {
        api_key: openai_api_key,
        model: "gpt-4o-mini".to_string(),
        ..Default::default()
    });

    let gemini = GeminiGenerator::new(GeminiConfig {
        api_key: gemini_api_key,
        model: "gemini-2.5-flash".to_string(),
        ..Default::default()
    });

    // Conversation history
    let mut history: Vec<(String, String)> = Vec::new(); // (role, content)

    // OpenAI starts with the initial topic
    println!("📝 话题: {}\n", INITIAL_TOPIC);

    // First turn: OpenAI responds to the topic
    let openai_response =
        generate_response(&openai, "OpenAI", SYSTEM_PROMPT, &history, INITIAL_TOPIC).await?;
    history.push(("OpenAI".to_string(), openai_response.clone()));
    println!("🔵 OpenAI: {}\n", openai_response);

    // Dialogue rounds
    let rounds = 3;
    for round in 1..=rounds {
        println!("--- Round {} ---\n", round);

        // Gemini responds
        let last_message = history.last().map(|(_, m)| m.as_str()).unwrap_or("");
        let gemini_response =
            generate_response(&gemini, "Gemini", SYSTEM_PROMPT, &history, last_message).await?;
        history.push(("Gemini".to_string(), gemini_response.clone()));
        println!("🟢 Gemini: {}\n", gemini_response);

        // OpenAI responds
        let last_message = history.last().map(|(_, m)| m.as_str()).unwrap_or("");
        let openai_response =
            generate_response(&openai, "OpenAI", SYSTEM_PROMPT, &history, last_message).await?;
        history.push(("OpenAI".to_string(), openai_response.clone()));
        println!("🔵 OpenAI: {}\n", openai_response);
    }

    println!("\n============================================================");
    println!("  对话结束 - 共 {} 轮", rounds);
    println!("============================================================");

    Ok(())
}

async fn generate_response(
    generator: &dyn Generator,
    name: &str,
    system_prompt: &str,
    history: &[(String, String)],
    last_message: &str,
) -> Result<String> {
    let mut builder = ModelContextBuilder::new();

    // Add system prompt
    builder.prompt_text("system", system_prompt);

    // Add conversation history
    for (role, content) in history {
        if role == name {
            builder.model_text("", content);
        } else {
            builder.user_text("", content);
        }
    }

    // Add the message to respond to (if not already in history)
    if history.is_empty() || history.last().map(|(_, m)| m.as_str()) != Some(last_message) {
        builder.user_text("", last_message);
    }

    let ctx = builder.build();

    // Generate response
    let mut stream = generator.generate_stream("", &ctx).await?;
    let text = collect_text(&mut *stream).await?;

    Ok(text.trim().to_string())
}
