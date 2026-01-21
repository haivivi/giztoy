//! Multi-client MQTT communication test.
//!
//! This example demonstrates:
//! - Starting an embedded MQTT broker (server)
//! - Multiple clients connecting to the broker
//! - Clients subscribing and publishing messages
//! - Verifying all clients receive the expected messages
//!
//! Run with: cargo run --bin mqtt_multi_client

use anyhow::Result;
use giztoy_mqtt::{Dialer, Server, ServerConfig, ServeMux};
use std::collections::HashMap;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::RwLock;
use tracing::{info, Level};

/// Test data structure to track received messages
struct MessageTracker {
    /// Map of client_id -> received messages (topic, payload)
    client_messages: RwLock<HashMap<String, Vec<(String, String)>>>,
    /// Map of topic -> received payloads (for broker tracking)
    broker_messages: RwLock<HashMap<String, Vec<String>>>,
    /// Total message count
    total_count: AtomicUsize,
}

impl MessageTracker {
    fn new() -> Self {
        Self {
            client_messages: RwLock::new(HashMap::new()),
            broker_messages: RwLock::new(HashMap::new()),
            total_count: AtomicUsize::new(0),
        }
    }

    async fn record_client_message(&self, client_id: &str, topic: &str, payload: &str) {
        let mut map = self.client_messages.write().await;
        map.entry(client_id.to_string())
            .or_default()
            .push((topic.to_string(), payload.to_string()));
        self.total_count.fetch_add(1, Ordering::SeqCst);
    }

    async fn record_broker_message(&self, topic: &str, payload: &str) {
        let mut map = self.broker_messages.write().await;
        map.entry(topic.to_string())
            .or_default()
            .push(payload.to_string());
    }

    async fn get_client_messages(&self, client_id: &str) -> Vec<(String, String)> {
        let map = self.client_messages.read().await;
        map.get(client_id).cloned().unwrap_or_default()
    }

    async fn get_broker_messages(&self, topic: &str) -> Vec<String> {
        let map = self.broker_messages.read().await;
        map.get(topic).cloned().unwrap_or_default()
    }

    fn total(&self) -> usize {
        self.total_count.load(Ordering::SeqCst)
    }
}

/// Find an available port for the server
fn find_available_port() -> u16 {
    let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
    listener.local_addr().unwrap().port()
}

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize logging
    tracing_subscriber::fmt()
        .with_max_level(Level::INFO)
        .init();

    info!("Starting multi-client MQTT test");

    let port = find_available_port();
    let addr = format!("127.0.0.1:{}", port);
    info!("Using port: {}", port);

    // Create message tracker
    let tracker = Arc::new(MessageTracker::new());

    // Create broker handler to track messages
    let broker_mux = Arc::new(ServeMux::new());
    let tracker_clone = tracker.clone();
    broker_mux.handle_func("chat/#", move |msg| {
        let topic = msg.topic.clone();
        let payload = msg.payload_str().unwrap_or("").to_string();
        let tracker = tracker_clone.clone();
        tokio::spawn(async move {
            tracker.record_broker_message(&topic, &payload).await;
        });
        Ok(())
    })?;

    // Start broker
    let config = ServerConfig::new(&addr);
    let server = Server::new(config, Some(broker_mux));
    let server_clone = server.clone();

    tokio::spawn(async move {
        if let Err(e) = server_clone.serve().await {
            info!("Server stopped: {}", e);
        }
    });

    // Wait for server to start
    tokio::time::sleep(Duration::from_millis(500)).await;

    info!("Broker started on {}", addr);

    // Create clients
    let num_clients = 3;
    let mut clients = Vec::new();
    let mqtt_addr = format!("mqtt://{}", addr);

    for i in 0..num_clients {
        let client_id = format!("client-{}", i);
        let mux = Arc::new(ServeMux::new());
        let tracker_clone = tracker.clone();
        let cid = client_id.clone();

        // Each client subscribes to chat/* and records received messages
        mux.handle_func("chat/#", move |msg| {
            let topic = msg.topic.clone();
            let payload = msg.payload_str().unwrap_or("").to_string();
            let tracker = tracker_clone.clone();
            let client = cid.clone();
            tokio::spawn(async move {
                tracker.record_client_message(&client, &topic, &payload).await;
            });
            Ok(())
        })?;

        let dialer = Dialer::new()
            .with_id(&client_id)
            .with_serve_mux(mux)
            .with_connect_timeout(Duration::from_secs(5));

        let conn = dialer.dial(&mqtt_addr).await?;

        // Subscribe to chat topic
        conn.subscribe("chat/#").await?;

        info!("Client {} connected and subscribed", client_id);
        clients.push((client_id, conn));
    }

    // Wait for subscriptions to complete
    tokio::time::sleep(Duration::from_millis(300)).await;

    // Each client publishes a message
    for (client_id, conn) in clients.iter() {
        let topic = format!("chat/room1");
        let payload = format!("Hello from {}", client_id);
        
        conn.write_to_topic(payload.as_bytes(), &topic).await?;
        info!("Client {} published: {}", client_id, payload);
        
        // Small delay between publishes
        tokio::time::sleep(Duration::from_millis(100)).await;
    }

    // Wait for messages to propagate
    tokio::time::sleep(Duration::from_secs(1)).await;

    // Verify results
    info!("\n=== Verification ===");
    
    // Each client should have received messages from all clients
    let expected_messages = num_clients; // All clients send to the same topic
    
    for (client_id, _) in &clients {
        let messages = tracker.get_client_messages(client_id).await;
        info!(
            "Client {} received {} messages: {:?}",
            client_id,
            messages.len(),
            messages
        );
        
        if messages.len() >= expected_messages {
            info!("✓ {} received expected {} messages", client_id, expected_messages);
        } else {
            info!(
                "✗ {} received {} messages, expected {}",
                client_id,
                messages.len(),
                expected_messages
            );
        }
    }

    // Check broker received all messages
    let broker_msgs = tracker.get_broker_messages("chat/room1").await;
    info!(
        "\nBroker received {} messages on chat/room1: {:?}",
        broker_msgs.len(),
        broker_msgs
    );

    if broker_msgs.len() >= num_clients {
        info!("✓ Broker received all {} messages", num_clients);
    } else {
        info!(
            "✗ Broker received {} messages, expected {}",
            broker_msgs.len(),
            num_clients
        );
    }

    // Total messages tracked
    info!("\nTotal client messages tracked: {}", tracker.total());

    // Cleanup
    info!("\n=== Cleanup ===");
    for (client_id, conn) in clients {
        conn.close().await?;
        info!("Client {} disconnected", client_id);
    }

    server.close()?;
    info!("Broker stopped");

    info!("\n=== Test Complete ===");
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_multi_client_communication() -> Result<()> {
        // Initialize logging for tests
        let _ = tracing_subscriber::fmt()
            .with_max_level(Level::DEBUG)
            .try_init();

        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);
        let tracker = Arc::new(MessageTracker::new());

        // Create broker handler
        let broker_mux = Arc::new(ServeMux::new());
        let tracker_clone = tracker.clone();
        broker_mux.handle_func("test/#", move |msg| {
            let topic = msg.topic.clone();
            let payload = msg.payload_str().unwrap_or("").to_string();
            let tracker = tracker_clone.clone();
            tokio::spawn(async move {
                tracker.record_broker_message(&topic, &payload).await;
            });
            Ok(())
        })?;

        // Start broker
        let config = ServerConfig::new(&addr);
        let server = Server::new(config, Some(broker_mux));
        let server_clone = server.clone();
        
        tokio::spawn(async move {
            let _ = server_clone.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(300)).await;

        // Create two clients
        let mqtt_addr = format!("mqtt://{}", addr);

        // Client 1
        let mux1 = Arc::new(ServeMux::new());
        let tracker1 = tracker.clone();
        mux1.handle_func("test/+", move |msg| {
            let topic = msg.topic.clone();
            let payload = msg.payload_str().unwrap_or("").to_string();
            let tracker = tracker1.clone();
            tokio::spawn(async move {
                tracker.record_client_message("client1", &topic, &payload).await;
            });
            Ok(())
        })?;

        let conn1 = Dialer::new()
            .with_id("client1")
            .with_serve_mux(mux1)
            .dial(&mqtt_addr)
            .await?;
        conn1.subscribe("test/+").await?;

        // Client 2
        let mux2 = Arc::new(ServeMux::new());
        let tracker2 = tracker.clone();
        mux2.handle_func("test/+", move |msg| {
            let topic = msg.topic.clone();
            let payload = msg.payload_str().unwrap_or("").to_string();
            let tracker = tracker2.clone();
            tokio::spawn(async move {
                tracker.record_client_message("client2", &topic, &payload).await;
            });
            Ok(())
        })?;

        let conn2 = Dialer::new()
            .with_id("client2")
            .with_serve_mux(mux2)
            .dial(&mqtt_addr)
            .await?;
        conn2.subscribe("test/+").await?;

        tokio::time::sleep(Duration::from_millis(200)).await;

        // Client 1 sends message
        conn1.write_to_topic(b"hello from client1", "test/topic1").await?;
        
        // Client 2 sends message
        conn2.write_to_topic(b"hello from client2", "test/topic2").await?;

        tokio::time::sleep(Duration::from_millis(500)).await;

        // Verify both clients received both messages
        let msgs1 = tracker.get_client_messages("client1").await;
        let msgs2 = tracker.get_client_messages("client2").await;

        assert!(msgs1.len() >= 2, "Client1 should receive 2 messages, got {}", msgs1.len());
        assert!(msgs2.len() >= 2, "Client2 should receive 2 messages, got {}", msgs2.len());

        // Verify broker received all messages
        let broker_msgs1 = tracker.get_broker_messages("test/topic1").await;
        let broker_msgs2 = tracker.get_broker_messages("test/topic2").await;
        
        assert!(broker_msgs1.len() >= 1, "Broker should receive message on test/topic1");
        assert!(broker_msgs2.len() >= 1, "Broker should receive message on test/topic2");

        // Cleanup
        conn1.close().await?;
        conn2.close().await?;
        server.close()?;

        Ok(())
    }
}
