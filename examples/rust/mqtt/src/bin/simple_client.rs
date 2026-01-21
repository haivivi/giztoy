//! Simple MQTT Client Example.
//!
//! This example demonstrates basic MQTT client operations:
//! - Connecting to a broker
//! - Subscribing to topics
//! - Publishing messages
//! - Handling received messages
//!
//! Run with: cargo run --bin mqtt_simple_client
//!
//! Note: Requires an MQTT broker running on localhost:1883

use anyhow::Result;
use giztoy_mqtt::{Dialer, ServeMux};
use std::sync::Arc;
use std::time::Duration;
use tracing::{info, Level};

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize logging
    tracing_subscriber::fmt()
        .with_max_level(Level::INFO)
        .init();

    info!("Starting Simple MQTT Client");

    // Create message handler
    let mux = Arc::new(ServeMux::new());

    // Handle messages on subscribed topics
    mux.handle_func("response/#", |msg| {
        info!(
            "Received on '{}': {}",
            msg.topic,
            msg.payload_str().unwrap_or("<binary>")
        );
        Ok(())
    })?;

    mux.handle_func("broadcast/#", |msg| {
        info!(
            "[BROADCAST] {}: {}",
            msg.topic,
            msg.payload_str().unwrap_or("<binary>")
        );
        Ok(())
    })?;

    // Connect to broker
    let dialer = Dialer::new()
        .with_id("simple-client")
        .with_keep_alive(30)
        .with_serve_mux(mux)
        .with_connect_timeout(Duration::from_secs(10))
        .with_on_connection_up(|| {
            info!("Connected to broker!");
        })
        .with_on_connect_error(|e| {
            info!("Connection error: {}", e);
        });

    info!("Connecting to mqtt://127.0.0.1:1883...");

    let conn = match dialer.dial("mqtt://127.0.0.1:1883").await {
        Ok(c) => c,
        Err(e) => {
            info!("Failed to connect: {}", e);
            info!("Make sure an MQTT broker is running on localhost:1883");
            info!("You can start one with: cargo run --bin mqtt_echo_server");
            return Err(e.into());
        }
    };

    info!("Connected!");

    // Subscribe to topics
    conn.subscribe("response/#").await?;
    conn.subscribe("broadcast/#").await?;
    info!("Subscribed to response/# and broadcast/#");

    // Publish some messages
    for i in 1..=5 {
        let topic = format!("request/test/{}", i);
        let payload = format!("Message {}", i);

        conn.write_to_topic(payload.as_bytes(), &topic).await?;
        info!("Published to '{}': {}", topic, payload);

        tokio::time::sleep(Duration::from_millis(500)).await;
    }

    // Keep running to receive messages
    info!("Waiting for messages... Press Ctrl+C to stop");

    tokio::signal::ctrl_c().await?;

    info!("Disconnecting...");
    conn.close().await?;
    info!("Disconnected");

    Ok(())
}
