//! Text generation (chat completion) example.
//!
//! This example demonstrates how to use the MiniMax SDK for text generation.
//!
//! Run with:
//! ```bash
//! export MINIMAX_API_KEY="your-api-key"
//! cargo run --example minimax-text
//! ```

use std::env;
use std::pin::pin;

use giztoy_minimax::{ChatCompletionRequest, Client, Message, MODEL_M2_1};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Get API key from environment
    let api_key =
        env::var("MINIMAX_API_KEY").expect("MINIMAX_API_KEY environment variable not set");

    // Create client
    let client = Client::new(api_key)?;

    // Example 1: Simple chat completion
    println!("Example 1: Simple chat completion");
    println!("---");

    let request = ChatCompletionRequest {
        model: MODEL_M2_1.to_string(),
        messages: vec![
            Message::system("You are a helpful assistant."),
            Message::user("What is the capital of France?"),
        ],
        max_tokens: Some(100),
        temperature: Some(0.7),
        ..Default::default()
    };

    let response = client.text().create_chat_completion(&request).await?;

    if let Some(choice) = response.choices.first() {
        if let Some(content) = choice.message.content_str() {
            println!("Assistant: {}", content);
        }
    }

    if let Some(usage) = &response.usage {
        println!(
            "\nTokens: {} prompt, {} completion, {} total",
            usage.prompt_tokens, usage.completion_tokens, usage.total_tokens
        );
    }

    // Example 2: Streaming chat completion
    println!("\n\nExample 2: Streaming chat completion");
    println!("---");

    use futures::StreamExt;

    let stream_request = ChatCompletionRequest {
        model: MODEL_M2_1.to_string(),
        messages: vec![
            Message::system("You are a helpful assistant."),
            Message::user("Write a short poem about programming."),
        ],
        max_tokens: Some(200),
        ..Default::default()
    };

    print!("Assistant: ");

    let s = client
        .text()
        .create_chat_completion_stream(&stream_request)
        .await?;
    let mut stream = pin!(s);

    while let Some(chunk) = stream.next().await {
        let chunk = chunk?;
        if let Some(choice) = chunk.choices.first() {
            if let Some(content) = &choice.delta.content {
                print!("{}", content);
                std::io::Write::flush(&mut std::io::stdout())?;
            }
        }
    }
    println!();

    // Example 3: Multi-turn conversation
    println!("\n\nExample 3: Multi-turn conversation");
    println!("---");

    let conversation_request = ChatCompletionRequest {
        model: MODEL_M2_1.to_string(),
        messages: vec![
            Message::system("You are a helpful assistant that speaks concisely."),
            Message::user("What is 2 + 2?"),
            Message::assistant("4"),
            Message::user("And if we multiply that by 3?"),
        ],
        max_tokens: Some(50),
        ..Default::default()
    };

    let response = client
        .text()
        .create_chat_completion(&conversation_request)
        .await?;

    if let Some(choice) = response.choices.first() {
        if let Some(content) = choice.message.content_str() {
            println!("Assistant: {}", content);
        }
    }

    println!("\nDone!");
    Ok(())
}
