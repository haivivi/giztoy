//! Luau Agent Dialogue - Two Luau agents having a conversation.
//!
//! This example demonstrates two Luau agent instances running the same script,
//! with a host program coordinating messages between them.
//!
//! Usage:
//!   OPENAI_API_KEY=xxx cargo run --bin luau_dialogue
//!   OPENAI_API_KEY=xxx bazel run //examples/rust/genx:luau_dialogue

use anyhow::{Context, Result};
use giztoy_genx::openai::{OpenAIConfig, OpenAIGenerator};
use giztoy_genx::{stream::collect_text, Generator, ModelContextBuilder};
use std::env;
use std::path::PathBuf;
use std::sync::Arc;
use tokio::sync::mpsc;
use tracing::{info, Level};
use tracing_subscriber::FmtSubscriber;

const INITIAL_TOPIC: &str = "你认为人工智能能够真正理解人类的情感吗？请分享你的看法。";
const DIALOG_ROUNDS: usize = 3;

const SYSTEM_PROMPT: &str = r#"你正在参与一场哲学对话。请用简洁有趣的方式回应对方的观点，
每次回复控制在2-3句话以内。可以提出反驳、补充或新的问题来推进讨论。
使用中文回复。"#;

/// Agent state for the dialogue
struct Agent {
    name: String,
    model: String,
    history: Vec<(String, String)>, // (role, content)
    generator: Arc<dyn Generator>,
}

impl Agent {
    fn new(name: &str, generator: Arc<dyn Generator>, model: String) -> Self {
        Self {
            name: name.to_string(),
            model,
            history: Vec::new(),
            generator,
        }
    }

    async fn respond(&mut self, input: &str) -> Result<String> {
        // Add user message to history
        self.history.push(("user".to_string(), input.to_string()));

        // Build model context
        let mut builder = ModelContextBuilder::new();
        builder.prompt_text("system", SYSTEM_PROMPT);

        for (role, content) in &self.history {
            if role == "user" {
                builder.user_text("", content);
            } else {
                builder.model_text("", content);
            }
        }

        let ctx = builder.build();

        // Generate response
        let mut stream = self.generator.generate_stream(&self.model, &ctx).await?;
        let response = collect_text(&mut *stream).await?;
        let response = response.trim().to_string();

        // Add assistant response to history
        self.history
            .push(("assistant".to_string(), response.clone()));

        Ok(response)
    }
}

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize logging
    let subscriber = FmtSubscriber::builder()
        .with_max_level(Level::INFO)
        .finish();
    tracing::subscriber::set_global_default(subscriber)?;

    // Check for API key (supports OpenAI or DeepSeek)
    let (api_key, base_url, model) = if let Ok(key) = env::var("OPENAI_API_KEY") {
        (key, env::var("OPENAI_BASE_URL").ok(), "gpt-4o-mini".to_string())
    } else if let Ok(key) = env::var("DEEPSEEK_API_KEY") {
        (key, Some("https://api.deepseek.com".to_string()), "deepseek-chat".to_string())
    } else {
        anyhow::bail!("OPENAI_API_KEY or DEEPSEEK_API_KEY environment variable is required")
    };

    info!("Starting Luau Agent Dialogue");
    println!("\n============================================================");
    println!("  Luau Agent Dialogue: Alice vs Bob (Rust Host)");
    println!("============================================================\n");

    // Create generator
    let mut config = OpenAIConfig {
        api_key,
        model: model.clone(),
        ..Default::default()
    };
    if let Some(url) = base_url {
        config.base_url = url;
    }
    let generator: Arc<dyn Generator> = Arc::new(OpenAIGenerator::new(config));

    // Create agents
    // Note: In a full implementation, these would be Luau scripts running
    // with the Rust agent runtime. For now, we simulate the agent behavior
    // directly in Rust to demonstrate the dialogue pattern.
    let mut alice = Agent::new("Alice", generator.clone(), model.clone());
    let mut bob = Agent::new("Bob", generator.clone(), model.clone());

    // Start the conversation
    println!("Topic: {}\n", INITIAL_TOPIC);

    // Alice responds to initial topic
    let mut current_input = alice.respond(INITIAL_TOPIC).await?;
    println!("Alice: {}\n", current_input);

    // Dialogue loop
    for round in 1..=DIALOG_ROUNDS {
        println!("--- Round {} ---\n", round);

        // Bob responds to Alice
        let bob_response = bob.respond(&current_input).await?;
        println!("Bob: {}\n", bob_response);

        // Alice responds to Bob (if not last round)
        if round < DIALOG_ROUNDS {
            current_input = alice.respond(&bob_response).await?;
            println!("Alice: {}\n", current_input);
        }
    }

    println!("============================================================");
    println!("  Dialogue finished - {} rounds", DIALOG_ROUNDS);
    println!("============================================================");

    Ok(())
}
