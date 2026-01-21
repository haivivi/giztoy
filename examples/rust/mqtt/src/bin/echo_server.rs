//! MQTT Echo Server Example.
//!
//! This example demonstrates running an MQTT broker that echoes
//! messages back to clients on a response topic.
//!
//! Run with: cargo run --bin mqtt_echo_server

use anyhow::Result;
use giztoy_mqtt::{Server, ServerConfig, ServeMux};
use std::sync::Arc;
use tracing::{info, Level};

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize logging
    tracing_subscriber::fmt()
        .with_max_level(Level::INFO)
        .init();

    info!("Starting MQTT Echo Server");

    // Create handler that echoes messages
    let mux = Arc::new(ServeMux::new());

    // Echo handler: receives on "request/*" and the broker can publish to "response/*"
    mux.handle_func("request/#", |msg| {
        let topic = msg.topic.clone();
        let payload = msg.payload_str().unwrap_or("<binary>").to_string();
        info!("Received on {}: {}", topic, payload);
        
        // Note: In a real application, you'd use the server's write_to_topic
        // to echo back. For this demo, we just log the message.
        let response_topic = topic.replace("request/", "response/");
        info!("Would echo to: {}", response_topic);
        
        Ok(())
    })?;

    // Connection callbacks
    let on_connect = |client_id: &str| {
        info!("Client connected: {}", client_id);
    };

    let on_disconnect = |client_id: &str| {
        info!("Client disconnected: {}", client_id);
    };

    // Build server with callbacks
    let config = ServerConfig::new("127.0.0.1:1883");
    let server = Server::builder(config)
        .handler(mux)
        .on_connect(on_connect)
        .on_disconnect(on_disconnect)
        .build();

    info!("Echo server listening on 127.0.0.1:1883");
    info!("Subscribe to 'response/#' to receive echoed messages");
    info!("Publish to 'request/...' to send messages");
    info!("Press Ctrl+C to stop");

    // Run server
    server.serve().await?;

    Ok(())
}
