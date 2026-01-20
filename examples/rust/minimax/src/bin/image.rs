//! Image generation example.
//!
//! This example demonstrates how to use the MiniMax SDK for image generation.
//!
//! Run with:
//! ```bash
//! export MINIMAX_API_KEY="your-api-key"
//! cargo run --example image
//! ```

use std::env;

use giztoy_minimax::{Client, ImageGenerateRequest, MODEL_IMAGE_01, ASPECT_RATIO_16X9};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Get API key from environment
    let api_key = env::var("MINIMAX_API_KEY")
        .expect("MINIMAX_API_KEY environment variable not set");

    // Create client
    let client = Client::new(api_key)?;

    // Example 1: Simple image generation
    println!("Example 1: Simple image generation");
    println!("---");

    let request = ImageGenerateRequest {
        model: MODEL_IMAGE_01.to_string(),
        prompt: "A serene Japanese garden with cherry blossoms, koi pond, and a small wooden bridge, in the style of traditional watercolor painting".to_string(),
        aspect_ratio: Some(ASPECT_RATIO_16X9.to_string()),
        n: Some(1),
        prompt_optimizer: Some(true),
    };

    println!("Generating image...");
    let response = client.image().generate(&request).await?;

    for (i, image) in response.images.iter().enumerate() {
        println!("Image {}: {}", i + 1, image.url);
    }

    // Example 2: Generate multiple images
    println!("\n\nExample 2: Generate multiple images");
    println!("---");

    let multi_request = ImageGenerateRequest {
        model: MODEL_IMAGE_01.to_string(),
        prompt: "A futuristic cityscape at sunset, with flying cars and neon lights".to_string(),
        aspect_ratio: Some("1:1".to_string()),
        n: Some(2),
        prompt_optimizer: Some(true),
    };

    println!("Generating 2 images...");
    let response = client.image().generate(&multi_request).await?;

    for (i, image) in response.images.iter().enumerate() {
        println!("Image {}: {}", i + 1, image.url);
    }

    println!("\nDone!");
    Ok(())
}
