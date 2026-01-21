//! Integration tests for mqtt0.
//!
//! Tests mqtt0c (client) against standard rumqttd broker.
//! Tests mqtt0d (broker) against standard rumqttc client.
//!
//! ## Running Tests
//!
//! ```bash
//! # Run all tests
//! bazel test //rust/mqtt0:mqtt0_test
//!
//! # Run example tests (ignored by default)
//! bazel test //rust/mqtt0:mqtt0_test -- --ignored
//! ```
//!
//! ## Protocol Support
//!
//! | Protocol | Protocol Level | Support |
//! |----------|---------------|---------|
//! | MQTT 3.0 | - | ❌ No formal standard |
//! | MQTT 3.1 | 3 | ❌ Legacy, use 3.1.1 |
//! | MQTT 3.1.1 | 4 (v4) | ✅ Full support |
//! | MQTT 5.0 | 5 (v5) | ✅ Full support |

use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::Arc;
use std::time::Duration;

use crate::types::{Authenticator, Message, ProtocolVersion};
use crate::{Broker, BrokerConfig, Client, ClientConfig};

/// Find an available port for testing.
fn find_available_port() -> u16 {
    static PORT: AtomicUsize = AtomicUsize::new(18000);
    PORT.fetch_add(1, Ordering::SeqCst) as u16
}

// ============================================================================
// Tests: mqtt0c (client) with standard rumqttd broker
// ============================================================================

mod client_tests {
    use super::*;

    /// Test basic connection to rumqttd.
    #[tokio::test]
    async fn test_client_connect_to_rumqttd() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        // Start rumqttd broker
        let config = create_rumqttd_config(&addr);
        let mut broker = rumqttd::Broker::new(config);
        let _handle = std::thread::spawn(move || {
            broker.start().unwrap();
        });

        // Wait for broker to start
        tokio::time::sleep(Duration::from_millis(200)).await;

        // Connect with mqtt0c
        let client = Client::connect(ClientConfig::new(&addr, "test-client")).await;
        assert!(client.is_ok(), "Failed to connect: {:?}", client.err());

        let client = client.unwrap();
        client.disconnect().await.unwrap();
    }

    /// Test publish and subscribe with rumqttd.
    #[tokio::test]
    async fn test_client_pub_sub_with_rumqttd() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        // Start rumqttd broker
        let config = create_rumqttd_config(&addr);
        let mut broker = rumqttd::Broker::new(config);
        let _handle = std::thread::spawn(move || {
            broker.start().unwrap();
        });

        tokio::time::sleep(Duration::from_millis(200)).await;

        // Connect client
        let mut client = Client::connect(ClientConfig::new(&addr, "test-client")).await.unwrap();

        // Subscribe
        client.subscribe(&["test/topic"]).await.unwrap();

        // Publish
        client.publish("test/topic", b"hello").await.unwrap();

        // Receive (with timeout)
        let msg = client.recv_timeout(Duration::from_secs(2)).await.unwrap();
        assert!(msg.is_some(), "Did not receive message");

        let msg = msg.unwrap();
        assert_eq!(msg.topic, "test/topic");
        assert_eq!(msg.payload.as_ref(), b"hello");

        client.disconnect().await.unwrap();
    }

    /// Test authentication with rumqttd.
    #[tokio::test]
    async fn test_client_auth_with_rumqttd() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        // Start rumqttd broker with auth
        let config = create_rumqttd_config_with_auth(&addr, "admin", "secret");
        let mut broker = rumqttd::Broker::new(config);
        let _handle = std::thread::spawn(move || {
            broker.start().unwrap();
        });

        tokio::time::sleep(Duration::from_millis(200)).await;

        // Connect with correct credentials
        let client = Client::connect(
            ClientConfig::new(&addr, "test-client")
                .with_credentials("admin", b"secret".to_vec()),
        )
        .await;
        assert!(client.is_ok(), "Should connect with correct credentials");
        client.unwrap().disconnect().await.unwrap();

        // Connect with wrong credentials
        let client = Client::connect(
            ClientConfig::new(&addr, "test-client2")
                .with_credentials("admin", b"wrong".to_vec()),
        )
        .await;
        assert!(client.is_err(), "Should fail with wrong credentials");
    }

    fn create_rumqttd_config(addr: &str) -> rumqttd::Config {
        use rumqttd::{Config, ConnectionSettings, RouterConfig, ServerSettings};
        use std::collections::HashMap;
        use std::net::SocketAddr;

        let socket_addr: SocketAddr = addr.parse().unwrap();

        let mut servers = HashMap::new();
        servers.insert(
            "tcp".to_string(),
            ServerSettings {
                name: "tcp".to_string(),
                listen: socket_addr,
                tls: None,
                next_connection_delay_ms: 1,
                connections: ConnectionSettings {
                    connection_timeout_ms: 60000,
                    max_payload_size: 1024 * 1024,
                    max_inflight_count: 100,
                    auth: None,
                    external_auth: None,
                    dynamic_filters: false,
                },
            },
        );

        Config {
            id: 0,
            router: RouterConfig {
                max_connections: 1000,
                max_outgoing_packet_count: 200,
                max_segment_size: 1024 * 1024,
                max_segment_count: 10,
                ..Default::default()
            },
            v4: Some(servers),
            v5: None,
            ws: None,
            prometheus: None,
            metrics: None,
            console: None,
            bridge: None,
            cluster: None,
        }
    }

    fn create_rumqttd_config_with_auth(addr: &str, username: &str, password: &str) -> rumqttd::Config {
        use rumqttd::{Config, ConnectionSettings, RouterConfig, ServerSettings};
        use std::collections::HashMap;
        use std::net::SocketAddr;
        use std::sync::Arc;
        use std::pin::Pin;
        use std::future::Future;

        let socket_addr: SocketAddr = addr.parse().unwrap();

        let expected_user = username.to_string();
        let expected_pass = password.to_string();

        let auth_handler: Arc<
            dyn Fn(String, String, String) -> Pin<Box<dyn Future<Output = bool> + Send + 'static>>
                + Send
                + Sync,
        > = Arc::new(move |_client_id, user, pass| {
            let expected_user = expected_user.clone();
            let expected_pass = expected_pass.clone();
            Box::pin(async move { user == expected_user && pass == expected_pass })
        });

        let mut servers = HashMap::new();
        servers.insert(
            "tcp".to_string(),
            ServerSettings {
                name: "tcp".to_string(),
                listen: socket_addr,
                tls: None,
                next_connection_delay_ms: 1,
                connections: ConnectionSettings {
                    connection_timeout_ms: 60000,
                    max_payload_size: 1024 * 1024,
                    max_inflight_count: 100,
                    auth: None,
                    external_auth: Some(auth_handler),
                    dynamic_filters: false,
                },
            },
        );

        Config {
            id: 0,
            router: RouterConfig {
                max_connections: 1000,
                max_outgoing_packet_count: 200,
                max_segment_size: 1024 * 1024,
                max_segment_count: 10,
                ..Default::default()
            },
            v4: Some(servers),
            v5: None,
            ws: None,
            prometheus: None,
            metrics: None,
            console: None,
            bridge: None,
            cluster: None,
        }
    }
}

// ============================================================================
// Tests: mqtt0d (broker) with standard rumqttc client
// ============================================================================

mod broker_tests {
    use super::*;
    use parking_lot::Mutex;
    use rumqttc::{AsyncClient, Event, Incoming, MqttOptions, QoS as RumqttQoS};

    /// Test basic connection to mqtt0d.
    #[tokio::test]
    async fn test_broker_basic_connect() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        // Start mqtt0d broker
        let broker = Broker::new(BrokerConfig::new(&addr));
        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Connect with rumqttc
        let mut options = MqttOptions::new("test-client", "127.0.0.1", port);
        options.set_keep_alive(Duration::from_secs(5));

        let (client, mut eventloop) = AsyncClient::new(options, 10);

        // Poll once to establish connection
        let event = eventloop.poll().await;
        assert!(event.is_ok(), "Failed to connect: {:?}", event.err());

        client.disconnect().await.unwrap();
    }

    /// Test publish and subscribe with mqtt0d.
    #[tokio::test]
    async fn test_broker_pub_sub() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        // Start mqtt0d broker
        let broker = Broker::new(BrokerConfig::new(&addr));
        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Connect with rumqttc
        let mut options = MqttOptions::new("test-client", "127.0.0.1", port);
        options.set_keep_alive(Duration::from_secs(5));

        let (client, mut eventloop) = AsyncClient::new(options, 10);

        // Subscribe
        client.subscribe("test/topic", RumqttQoS::AtMostOnce).await.unwrap();

        // Poll to process subscribe
        let _ = eventloop.poll().await;
        let _ = eventloop.poll().await;

        // Publish
        client
            .publish("test/topic", RumqttQoS::AtMostOnce, false, b"hello".to_vec())
            .await
            .unwrap();

        // Poll to receive message
        let mut received = false;
        for _ in 0..10 {
            if let Ok(Event::Incoming(Incoming::Publish(p))) = eventloop.poll().await {
                assert_eq!(p.topic, "test/topic");
                assert_eq!(p.payload.as_ref(), b"hello");
                received = true;
                break;
            }
        }
        assert!(received, "Did not receive message");

        client.disconnect().await.unwrap();
    }

    /// Test authentication with mqtt0d.
    #[tokio::test]
    async fn test_broker_auth() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        struct TestAuth;
        impl Authenticator for TestAuth {
            fn authenticate(&self, _client_id: &str, username: &str, password: &[u8]) -> bool {
                username == "admin" && password == b"secret"
            }
            fn acl(&self, _client_id: &str, _topic: &str, _write: bool) -> bool {
                true
            }
        }

        // Start mqtt0d broker with auth
        let broker = Broker::builder(BrokerConfig::new(&addr))
            .authenticator(TestAuth)
            .build();
        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Connect with correct credentials
        let mut options = MqttOptions::new("test-client", "127.0.0.1", port);
        options.set_credentials("admin", "secret");
        options.set_keep_alive(Duration::from_secs(5));

        let (client, mut eventloop) = AsyncClient::new(options, 10);
        let event = eventloop.poll().await;
        assert!(event.is_ok(), "Should connect with correct credentials");
        client.disconnect().await.unwrap();

        // Connect with wrong credentials
        let mut options = MqttOptions::new("test-client2", "127.0.0.1", port);
        options.set_credentials("admin", "wrong");
        options.set_keep_alive(Duration::from_secs(5));

        let (_client, mut eventloop) = AsyncClient::new(options, 10);
        let event = eventloop.poll().await;
        // Connection should fail
        assert!(event.is_err() || matches!(event, Ok(Event::Incoming(Incoming::ConnAck(ack))) if ack.code != rumqttc::mqttbytes::v4::ConnectReturnCode::Success));
    }

    /// Test ACL with mqtt0d - publish denied.
    #[tokio::test]
    async fn test_broker_acl_publish_denied() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        struct AclAuth;
        impl Authenticator for AclAuth {
            fn authenticate(&self, _: &str, _: &str, _: &[u8]) -> bool {
                true
            }
            fn acl(&self, _client_id: &str, topic: &str, write: bool) -> bool {
                // Only allow publish to "allowed/*"
                if write {
                    topic.starts_with("allowed/")
                } else {
                    true
                }
            }
        }

        let received = Arc::new(Mutex::new(Vec::<String>::new()));
        let received_clone = Arc::clone(&received);

        // Create a struct handler instead of closure due to lifetime inference issues
        struct TestHandler(Arc<Mutex<Vec<String>>>);
        impl crate::Handler for TestHandler {
            fn handle(&self, _client_id: &str, msg: &crate::Message) {
                self.0.lock().push(msg.topic.clone());
            }
        }

        // Start mqtt0d broker with ACL
        let broker = Broker::builder(BrokerConfig::new(&addr))
            .authenticator(AclAuth)
            .handler(TestHandler(received_clone))
            .build();
        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Connect with rumqttc
        let mut options = MqttOptions::new("test-client", "127.0.0.1", port);
        options.set_keep_alive(Duration::from_secs(5));

        let (client, mut eventloop) = AsyncClient::new(options, 10);

        // Establish connection
        let _ = eventloop.poll().await;

        // Publish to allowed topic
        client
            .publish("allowed/topic", RumqttQoS::AtMostOnce, false, b"yes".to_vec())
            .await
            .unwrap();
        let _ = eventloop.poll().await;

        // Publish to forbidden topic
        client
            .publish("forbidden/topic", RumqttQoS::AtMostOnce, false, b"no".to_vec())
            .await
            .unwrap();
        let _ = eventloop.poll().await;

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Check that only allowed topic was processed
        let received = received.lock();
        assert!(received.contains(&"allowed/topic".to_string()));
        assert!(!received.contains(&"forbidden/topic".to_string()));

        client.disconnect().await.unwrap();
    }

    /// Test ACL with mqtt0d - subscribe denied.
    #[tokio::test]
    async fn test_broker_acl_subscribe_denied() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        struct AclAuth;
        impl Authenticator for AclAuth {
            fn authenticate(&self, _: &str, _: &str, _: &[u8]) -> bool {
                true
            }
            fn acl(&self, _client_id: &str, topic: &str, write: bool) -> bool {
                // Only allow subscribe to "public/*"
                if !write {
                    topic.starts_with("public/")
                } else {
                    true
                }
            }
        }

        // Start mqtt0d broker with ACL
        let broker = Broker::builder(BrokerConfig::new(&addr))
            .authenticator(AclAuth)
            .build();
        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Connect with rumqttc
        let mut options = MqttOptions::new("test-client", "127.0.0.1", port);
        options.set_keep_alive(Duration::from_secs(5));

        let (client, mut eventloop) = AsyncClient::new(options, 10);

        // Establish connection
        let _ = eventloop.poll().await;

        // Subscribe to allowed topic
        client.subscribe("public/topic", RumqttQoS::AtMostOnce).await.unwrap();
        let _ = eventloop.poll().await;

        // Check SubAck - should have success for public/topic
        if let Ok(Event::Incoming(Incoming::SubAck(ack))) = eventloop.poll().await {
            assert_eq!(
                ack.return_codes[0],
                rumqttc::mqttbytes::v4::SubscribeReasonCode::Success(RumqttQoS::AtMostOnce)
            );
        }

        // Subscribe to forbidden topic
        client.subscribe("private/topic", RumqttQoS::AtMostOnce).await.unwrap();
        let _ = eventloop.poll().await;

        // Check SubAck - should have failure for private/topic
        if let Ok(Event::Incoming(Incoming::SubAck(ack))) = eventloop.poll().await {
            assert_eq!(
                ack.return_codes[0],
                rumqttc::mqttbytes::v4::SubscribeReasonCode::Failure
            );
        }

        client.disconnect().await.unwrap();
    }

    /// Test on_connect and on_disconnect callbacks.
    #[tokio::test]
    async fn test_broker_callbacks() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        let connected = Arc::new(Mutex::new(Vec::<String>::new()));
        let disconnected = Arc::new(Mutex::new(Vec::<String>::new()));

        let connected_clone = Arc::clone(&connected);
        let disconnected_clone = Arc::clone(&disconnected);

        // Start mqtt0d broker with callbacks
        let broker = Broker::builder(BrokerConfig::new(&addr))
            .on_connect(move |id| {
                connected_clone.lock().push(id.to_string());
            })
            .on_disconnect(move |id| {
                disconnected_clone.lock().push(id.to_string());
            })
            .build();
        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Connect with rumqttc
        let mut options = MqttOptions::new("callback-test", "127.0.0.1", port);
        options.set_keep_alive(Duration::from_secs(5));

        let (client, mut eventloop) = AsyncClient::new(options, 10);

        // Establish connection
        let _ = eventloop.poll().await;
        tokio::time::sleep(Duration::from_millis(50)).await;

        // Check on_connect was called
        assert!(connected.lock().contains(&"callback-test".to_string()));

        // Disconnect - need to poll eventloop to actually send the disconnect packet
        client.disconnect().await.unwrap();
        // Poll to ensure disconnect packet is sent
        let _ = tokio::time::timeout(Duration::from_millis(100), eventloop.poll()).await;
        // Give the broker time to process the disconnect
        tokio::time::sleep(Duration::from_millis(300)).await;

        // Check on_disconnect was called
        assert!(
            disconnected.lock().contains(&"callback-test".to_string()),
            "on_disconnect was not called. Connected: {:?}, Disconnected: {:?}",
            connected.lock(),
            disconnected.lock()
        );
    }
}

// ============================================================================
// Protocol Version Tests (MQTT 3.1.1 vs 5.0)
// ============================================================================

mod protocol_tests {
    use super::*;

    // ------------------------------------------------------------------------
    // MQTT 3.1.1 (v4) Tests with rumqttd
    // ------------------------------------------------------------------------

    /// Test v4 client connect to rumqttd with v4 config.
    #[tokio::test]
    async fn test_v4_client_to_rumqttd_v4() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        // Start rumqttd with v4 server
        let config = create_rumqttd_v4_config(&addr);
        let mut broker = rumqttd::Broker::new(config);
        let _handle = std::thread::spawn(move || {
            broker.start().unwrap();
        });

        tokio::time::sleep(Duration::from_millis(200)).await;

        // Connect with mqtt0c v4
        let config = ClientConfig::new(&addr, "v4-client")
            .with_protocol(ProtocolVersion::V4);
        let client = Client::connect(config).await;
        assert!(client.is_ok(), "v4 client should connect to v4 rumqttd");

        let client = client.unwrap();
        client.disconnect().await.unwrap();
    }

    /// Test v4 client pub/sub with rumqttd v4.
    #[tokio::test]
    async fn test_v4_client_pub_sub_with_rumqttd() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        let config = create_rumqttd_v4_config(&addr);
        let mut broker = rumqttd::Broker::new(config);
        let _handle = std::thread::spawn(move || {
            broker.start().unwrap();
        });

        tokio::time::sleep(Duration::from_millis(200)).await;

        // Connect with v4
        let mut client = Client::connect(
            ClientConfig::new(&addr, "v4-pubsub")
                .with_protocol(ProtocolVersion::V4)
        ).await.unwrap();

        // Subscribe and publish
        client.subscribe(&["v4/test"]).await.unwrap();
        client.publish("v4/test", b"v4 message").await.unwrap();

        // Receive
        let msg = client.recv_timeout(Duration::from_secs(2)).await.unwrap();
        assert!(msg.is_some());
        let msg = msg.unwrap();
        assert_eq!(msg.topic, "v4/test");
        assert_eq!(msg.payload.as_ref(), b"v4 message");

        client.disconnect().await.unwrap();
    }

    /// Test v4 wildcard subscriptions.
    #[tokio::test]
    async fn test_v4_wildcard_subscriptions() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        let config = create_rumqttd_v4_config(&addr);
        let mut broker = rumqttd::Broker::new(config);
        let _handle = std::thread::spawn(move || {
            broker.start().unwrap();
        });

        tokio::time::sleep(Duration::from_millis(200)).await;

        let mut client = Client::connect(
            ClientConfig::new(&addr, "v4-wildcard")
                .with_protocol(ProtocolVersion::V4)
        ).await.unwrap();

        // Test + wildcard
        client.subscribe(&["sensor/+/temp"]).await.unwrap();
        client.publish("sensor/room1/temp", b"25").await.unwrap();

        let msg = client.recv_timeout(Duration::from_secs(2)).await.unwrap();
        assert!(msg.is_some());
        assert_eq!(msg.unwrap().topic, "sensor/room1/temp");

        // Unsubscribe and test # wildcard
        client.unsubscribe(&["sensor/+/temp"]).await.unwrap();
        client.subscribe(&["home/#"]).await.unwrap();
        client.publish("home/living/light", b"on").await.unwrap();

        let msg = client.recv_timeout(Duration::from_secs(2)).await.unwrap();
        assert!(msg.is_some());
        assert_eq!(msg.unwrap().topic, "home/living/light");

        client.disconnect().await.unwrap();
    }

    // ------------------------------------------------------------------------
    // MQTT 5.0 (v5) Tests with rumqttd
    // ------------------------------------------------------------------------

    /// Test v5 client connect to rumqttd with v5 config.
    #[tokio::test]
    async fn test_v5_client_to_rumqttd_v5() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        // Start rumqttd with v5 server
        let config = create_rumqttd_v5_config(&addr);
        let mut broker = rumqttd::Broker::new(config);
        let _handle = std::thread::spawn(move || {
            broker.start().unwrap();
        });

        tokio::time::sleep(Duration::from_millis(200)).await;

        // Connect with mqtt0c v5
        let config = ClientConfig::new(&addr, "v5-client")
            .with_protocol(ProtocolVersion::V5);
        let client = Client::connect(config).await;
        assert!(client.is_ok(), "v5 client should connect to v5 rumqttd: {:?}", client.err());

        let client = client.unwrap();
        client.disconnect().await.unwrap();
    }

    /// Test v5 client with session expiry.
    #[tokio::test]
    async fn test_v5_client_session_expiry() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        let config = create_rumqttd_v5_config(&addr);
        let mut broker = rumqttd::Broker::new(config);
        let _handle = std::thread::spawn(move || {
            broker.start().unwrap();
        });

        tokio::time::sleep(Duration::from_millis(200)).await;

        // Connect with session expiry = 3600 seconds (1 hour)
        let config = ClientConfig::new(&addr, "v5-session")
            .with_protocol(ProtocolVersion::V5)
            .with_session_expiry(3600)
            .with_clean_session(false);

        let client = Client::connect(config).await;
        assert!(client.is_ok(), "v5 client with session expiry should connect");

        let client = client.unwrap();
        client.disconnect().await.unwrap();
    }

    /// Test v5 client with clean start.
    #[tokio::test]
    async fn test_v5_clean_start() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        let config = create_rumqttd_v5_config(&addr);
        let mut broker = rumqttd::Broker::new(config);
        let _handle = std::thread::spawn(move || {
            broker.start().unwrap();
        });

        tokio::time::sleep(Duration::from_millis(200)).await;

        // Connect with clean_start = true
        let client = Client::connect(
            ClientConfig::new(&addr, "v5-clean")
                .with_protocol(ProtocolVersion::V5)
                .with_clean_session(true)
        ).await.unwrap();

        client.disconnect().await.unwrap();

        // Reconnect with clean_start = false
        let client = Client::connect(
            ClientConfig::new(&addr, "v5-clean")
                .with_protocol(ProtocolVersion::V5)
                .with_clean_session(false)
        ).await.unwrap();

        client.disconnect().await.unwrap();
    }

    /// Test v5 client pub/sub with rumqttd v5.
    #[tokio::test]
    async fn test_v5_client_pub_sub_with_rumqttd() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        let config = create_rumqttd_v5_config(&addr);
        let mut broker = rumqttd::Broker::new(config);
        let _handle = std::thread::spawn(move || {
            broker.start().unwrap();
        });

        tokio::time::sleep(Duration::from_millis(200)).await;

        // Connect with v5
        let mut client = Client::connect(
            ClientConfig::new(&addr, "v5-pubsub")
                .with_protocol(ProtocolVersion::V5)
        ).await.unwrap();

        // Subscribe and publish
        client.subscribe(&["v5/test"]).await.unwrap();
        client.publish("v5/test", b"v5 message").await.unwrap();

        // Receive
        let msg = client.recv_timeout(Duration::from_secs(2)).await.unwrap();
        assert!(msg.is_some());
        let msg = msg.unwrap();
        assert_eq!(msg.topic, "v5/test");
        assert_eq!(msg.payload.as_ref(), b"v5 message");

        client.disconnect().await.unwrap();
    }

    /// Test v5 wildcard subscriptions.
    #[tokio::test]
    async fn test_v5_wildcard_subscriptions() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        let config = create_rumqttd_v5_config(&addr);
        let mut broker = rumqttd::Broker::new(config);
        let _handle = std::thread::spawn(move || {
            broker.start().unwrap();
        });

        tokio::time::sleep(Duration::from_millis(200)).await;

        let mut client = Client::connect(
            ClientConfig::new(&addr, "v5-wildcard")
                .with_protocol(ProtocolVersion::V5)
        ).await.unwrap();

        // Test + wildcard
        client.subscribe(&["device/+/status"]).await.unwrap();
        client.publish("device/abc123/status", b"online").await.unwrap();

        let msg = client.recv_timeout(Duration::from_secs(2)).await.unwrap();
        assert!(msg.is_some());
        assert_eq!(msg.unwrap().topic, "device/abc123/status");

        // Unsubscribe and test # wildcard
        client.unsubscribe(&["device/+/status"]).await.unwrap();
        client.subscribe(&["events/#"]).await.unwrap();
        client.publish("events/system/startup", b"1").await.unwrap();

        let msg = client.recv_timeout(Duration::from_secs(2)).await.unwrap();
        assert!(msg.is_some());
        assert_eq!(msg.unwrap().topic, "events/system/startup");

        client.disconnect().await.unwrap();
    }

    // ------------------------------------------------------------------------
    // Authentication Tests (v4 vs v5)
    // ------------------------------------------------------------------------

    /// Test v4 authentication with mqtt0d.
    #[tokio::test]
    async fn test_v4_auth_with_mqtt0d() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        struct SimpleAuth;
        impl Authenticator for SimpleAuth {
            fn authenticate(&self, _client_id: &str, username: &str, password: &[u8]) -> bool {
                username == "admin" && password == b"secret"
            }
            fn acl(&self, _client_id: &str, _topic: &str, _write: bool) -> bool {
                true
            }
        }

        let broker = Broker::builder(BrokerConfig::new(&addr))
            .authenticator(SimpleAuth)
            .build();
        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Test correct credentials (v4)
        let client = Client::connect(
            ClientConfig::new(&addr, "v4-auth-ok")
                .with_protocol(ProtocolVersion::V4)
                .with_credentials("admin", b"secret".to_vec())
        ).await;
        assert!(client.is_ok(), "v4 auth should succeed with correct credentials");
        client.unwrap().disconnect().await.unwrap();

        // Test wrong credentials (v4)
        let client = Client::connect(
            ClientConfig::new(&addr, "v4-auth-fail")
                .with_protocol(ProtocolVersion::V4)
                .with_credentials("admin", b"wrong".to_vec())
        ).await;
        assert!(client.is_err(), "v4 auth should fail with wrong credentials");
    }

    /// Test v5 authentication with mqtt0d.
    #[tokio::test]
    async fn test_v5_auth_with_mqtt0d() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        struct SimpleAuth;
        impl Authenticator for SimpleAuth {
            fn authenticate(&self, _client_id: &str, username: &str, password: &[u8]) -> bool {
                username == "admin" && password == b"secret"
            }
            fn acl(&self, _client_id: &str, _topic: &str, _write: bool) -> bool {
                true
            }
        }

        let broker = Broker::builder(BrokerConfig::new(&addr))
            .authenticator(SimpleAuth)
            .build();
        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Test correct credentials (v5)
        let client = Client::connect(
            ClientConfig::new(&addr, "v5-auth-ok")
                .with_protocol(ProtocolVersion::V5)
                .with_credentials("admin", b"secret".to_vec())
        ).await;
        assert!(client.is_ok(), "v5 auth should succeed with correct credentials");
        client.unwrap().disconnect().await.unwrap();

        // Test wrong credentials (v5)
        let client = Client::connect(
            ClientConfig::new(&addr, "v5-auth-fail")
                .with_protocol(ProtocolVersion::V5)
                .with_credentials("admin", b"wrong".to_vec())
        ).await;
        assert!(client.is_err(), "v5 auth should fail with wrong credentials");
    }

    /// Test v4 ACL with mqtt0d.
    #[tokio::test]
    async fn test_v4_acl_with_mqtt0d() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        struct TopicAcl;
        impl Authenticator for TopicAcl {
            fn authenticate(&self, _: &str, _: &str, _: &[u8]) -> bool {
                true
            }
            fn acl(&self, _client_id: &str, topic: &str, _write: bool) -> bool {
                topic.starts_with("public/")
            }
        }

        let broker = Broker::builder(BrokerConfig::new(&addr))
            .authenticator(TopicAcl)
            .build();
        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        let client = Client::connect(
            ClientConfig::new(&addr, "v4-acl")
                .with_protocol(ProtocolVersion::V4)
        ).await.unwrap();

        // Subscribe to allowed topic (should succeed)
        let result = client.subscribe(&["public/news"]).await;
        assert!(result.is_ok(), "v4 subscribe to public/ should succeed");

        // Subscribe to forbidden topic (should fail)
        let result = client.subscribe(&["private/data"]).await;
        assert!(result.is_err(), "v4 subscribe to private/ should fail");

        client.disconnect().await.unwrap();
    }

    /// Test v5 ACL with mqtt0d.
    #[tokio::test]
    async fn test_v5_acl_with_mqtt0d() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        struct TopicAcl;
        impl Authenticator for TopicAcl {
            fn authenticate(&self, _: &str, _: &str, _: &[u8]) -> bool {
                true
            }
            fn acl(&self, _client_id: &str, topic: &str, _write: bool) -> bool {
                topic.starts_with("public/")
            }
        }

        let broker = Broker::builder(BrokerConfig::new(&addr))
            .authenticator(TopicAcl)
            .build();
        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        let client = Client::connect(
            ClientConfig::new(&addr, "v5-acl")
                .with_protocol(ProtocolVersion::V5)
        ).await.unwrap();

        // Subscribe to allowed topic (should succeed)
        let result = client.subscribe(&["public/news"]).await;
        assert!(result.is_ok(), "v5 subscribe to public/ should succeed");

        // Subscribe to forbidden topic (should fail)
        let result = client.subscribe(&["private/data"]).await;
        assert!(result.is_err(), "v5 subscribe to private/ should fail");

        client.disconnect().await.unwrap();
    }

    // ------------------------------------------------------------------------
    // Cross-version Tests (mqtt0d broker)
    // ------------------------------------------------------------------------

    /// Test rumqttc v4 client with mqtt0d broker.
    #[tokio::test]
    async fn test_rumqttc_v4_with_mqtt0d() {
        use rumqttc::{AsyncClient, Event, Incoming, MqttOptions, QoS as RumqttQoS};

        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        // Start mqtt0d broker
        let broker = Broker::new(BrokerConfig::new(&addr));
        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Connect with rumqttc (v4 by default)
        let mut options = MqttOptions::new("rumqttc-v4", "127.0.0.1", port);
        options.set_keep_alive(Duration::from_secs(5));

        let (client, mut eventloop) = AsyncClient::new(options, 10);

        // Connect
        let event = eventloop.poll().await;
        assert!(event.is_ok());

        // Subscribe
        client.subscribe("mqtt0d/test", RumqttQoS::AtMostOnce).await.unwrap();
        let _ = eventloop.poll().await;
        let _ = eventloop.poll().await;

        // Publish
        client.publish("mqtt0d/test", RumqttQoS::AtMostOnce, false, b"hello".to_vec()).await.unwrap();
        let _ = eventloop.poll().await;

        // Receive
        let mut received = false;
        for _ in 0..10 {
            if let Ok(Event::Incoming(Incoming::Publish(publish))) = eventloop.poll().await {
                assert_eq!(publish.topic, "mqtt0d/test");
                assert_eq!(publish.payload.as_ref(), b"hello");
                received = true;
                break;
            }
        }
        assert!(received, "Should receive the published message");

        client.disconnect().await.unwrap();
    }

    /// Test rumqttc v5 client with mqtt0d broker.
    #[tokio::test]
    async fn test_rumqttc_v5_with_mqtt0d() {
        use rumqttc::v5::{AsyncClient, Event, mqttbytes::v5::Packet, MqttOptions};
        use rumqttc::v5::mqttbytes::QoS as RumqttQoS;

        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        // Start mqtt0d broker (auto-detects protocol version)
        let broker = Broker::new(BrokerConfig::new(&addr));
        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Connect with rumqttc v5
        let mut options = MqttOptions::new("rumqttc-v5", "127.0.0.1", port);
        options.set_keep_alive(Duration::from_secs(5));

        let (client, mut eventloop) = AsyncClient::new(options, 10);

        // Connect
        let event = eventloop.poll().await;
        assert!(event.is_ok(), "v5 client should connect to mqtt0d: {:?}", event.err());

        // Subscribe
        client.subscribe("mqtt0d/v5", RumqttQoS::AtMostOnce).await.unwrap();
        let _ = eventloop.poll().await;
        let _ = eventloop.poll().await;

        // Publish
        client.publish("mqtt0d/v5", RumqttQoS::AtMostOnce, false, b"v5 hello".to_vec()).await.unwrap();
        let _ = eventloop.poll().await;

        // Receive
        let mut received = false;
        for _ in 0..10 {
            if let Ok(Event::Incoming(Packet::Publish(publish))) = eventloop.poll().await {
                assert_eq!(publish.topic.as_ref(), b"mqtt0d/v5");
                assert_eq!(publish.payload.as_ref(), b"v5 hello");
                received = true;
                break;
            }
        }
        assert!(received, "Should receive the v5 published message");

        client.disconnect().await.unwrap();
    }

    /// Test mqtt0c v4 with mqtt0d broker.
    #[tokio::test]
    async fn test_mqtt0c_v4_with_mqtt0d() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        // Start mqtt0d broker
        let broker = Broker::new(BrokerConfig::new(&addr));
        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Connect with mqtt0c v4
        let client = Client::connect(
            ClientConfig::new(&addr, "mqtt0c-v4")
                .with_protocol(ProtocolVersion::V4)
        ).await.unwrap();

        client.subscribe(&["self/v4"]).await.unwrap();
        client.publish("self/v4", b"v4 self-test").await.unwrap();

        let msg = client.recv_timeout(Duration::from_secs(2)).await.unwrap();
        assert!(msg.is_some());
        assert_eq!(msg.unwrap().payload.as_ref(), b"v4 self-test");

        client.disconnect().await.unwrap();
    }

    /// Test mqtt0c v5 with mqtt0d broker.
    #[tokio::test]
    async fn test_mqtt0c_v5_with_mqtt0d() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        // Start mqtt0d broker
        let broker = Broker::new(BrokerConfig::new(&addr));
        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Connect with mqtt0c v5
        let client = Client::connect(
            ClientConfig::new(&addr, "mqtt0c-v5")
                .with_protocol(ProtocolVersion::V5)
                .with_session_expiry(300)
        ).await.unwrap();

        client.subscribe(&["self/v5"]).await.unwrap();
        client.publish("self/v5", b"v5 self-test").await.unwrap();

        let msg = client.recv_timeout(Duration::from_secs(2)).await.unwrap();
        assert!(msg.is_some());
        assert_eq!(msg.unwrap().payload.as_ref(), b"v5 self-test");

        client.disconnect().await.unwrap();
    }

    /// Test mixed v4/v5 clients communicating through mqtt0d.
    #[tokio::test]
    async fn test_mixed_v4_v5_clients() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        // Start mqtt0d broker
        let broker = Broker::new(BrokerConfig::new(&addr));
        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // v4 client subscribes to "from-v5" to receive v5 messages
        let v4_client = Client::connect(
            ClientConfig::new(&addr, "mixed-v4")
                .with_protocol(ProtocolVersion::V4)
        ).await.unwrap();
        v4_client.subscribe(&["from-v5"]).await.unwrap();

        // v5 client subscribes to "from-v4" to receive v4 messages
        let v5_client = Client::connect(
            ClientConfig::new(&addr, "mixed-v5")
                .with_protocol(ProtocolVersion::V5)
        ).await.unwrap();
        v5_client.subscribe(&["from-v4"]).await.unwrap();

        // v4 sends to "from-v4", v5 receives
        v4_client.publish("from-v4", b"hello from v4").await.unwrap();

        let msg = v5_client.recv_timeout(Duration::from_secs(2)).await.unwrap();
        assert!(msg.is_some(), "v5 should receive message from v4");
        assert_eq!(msg.unwrap().payload.as_ref(), b"hello from v4");

        // v5 sends to "from-v5", v4 receives
        v5_client.publish("from-v5", b"hello from v5").await.unwrap();

        let msg = v4_client.recv_timeout(Duration::from_secs(2)).await.unwrap();
        assert!(msg.is_some(), "v4 should receive message from v5");
        assert_eq!(msg.unwrap().payload.as_ref(), b"hello from v5");

        v4_client.disconnect().await.unwrap();
        v5_client.disconnect().await.unwrap();
    }

    // ------------------------------------------------------------------------
    // Helper Functions
    // ------------------------------------------------------------------------

    fn create_rumqttd_v4_config(addr: &str) -> rumqttd::Config {
        use rumqttd::{Config, ConnectionSettings, RouterConfig, ServerSettings};
        use std::collections::HashMap;
        use std::net::SocketAddr;

        let socket_addr: SocketAddr = addr.parse().unwrap();

        let mut servers = HashMap::new();
        servers.insert(
            "tcp".to_string(),
            ServerSettings {
                name: "tcp".to_string(),
                listen: socket_addr,
                tls: None,
                next_connection_delay_ms: 1,
                connections: ConnectionSettings {
                    connection_timeout_ms: 60000,
                    max_payload_size: 1024 * 1024,
                    max_inflight_count: 100,
                    auth: None,
                    external_auth: None,
                    dynamic_filters: false,
                },
            },
        );

        Config {
            id: 0,
            router: RouterConfig {
                max_connections: 1000,
                max_outgoing_packet_count: 200,
                max_segment_size: 1024 * 1024,
                max_segment_count: 10,
                ..Default::default()
            },
            v4: Some(servers),
            v5: None,
            ws: None,
            prometheus: None,
            metrics: None,
            console: None,
            bridge: None,
            cluster: None,
        }
    }

    fn create_rumqttd_v5_config(addr: &str) -> rumqttd::Config {
        use rumqttd::{Config, ConnectionSettings, RouterConfig, ServerSettings};
        use std::collections::HashMap;
        use std::net::SocketAddr;

        let socket_addr: SocketAddr = addr.parse().unwrap();

        let mut servers = HashMap::new();
        servers.insert(
            "tcp".to_string(),
            ServerSettings {
                name: "tcp".to_string(),
                listen: socket_addr,
                tls: None,
                next_connection_delay_ms: 1,
                connections: ConnectionSettings {
                    connection_timeout_ms: 60000,
                    max_payload_size: 1024 * 1024,
                    max_inflight_count: 100,
                    auth: None,
                    external_auth: None,
                    dynamic_filters: false,
                },
            },
        );

        Config {
            id: 0,
            router: RouterConfig {
                max_connections: 1000,
                max_outgoing_packet_count: 200,
                max_segment_size: 1024 * 1024,
                max_segment_count: 10,
                ..Default::default()
            },
            v4: None,
            v5: Some(servers),
            ws: None,
            prometheus: None,
            metrics: None,
            console: None,
            bridge: None,
            cluster: None,
        }
    }
}

// ============================================================================
// Example Tests (run with --ignored)
// ============================================================================

mod examples {
    use super::*;
    use tracing_subscriber;

    /// Example: Echo Server
    ///
    /// Demonstrates a simple echo server that echoes messages back to sender.
    /// Run with: cargo test --package giztoy-mqtt0 example_echo_server -- --ignored --nocapture
    #[tokio::test]
    #[ignore]
    async fn example_echo_server() {
        tracing_subscriber::fmt::init();

        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        println!("=== Echo Server Example ===");
        println!("Starting broker on {}", addr);

        // Create broker with echo handler
        struct EchoHandler {
            clients: Arc<RwLock<HashMap<String, mpsc::Sender<Message>>>>,
        }

        impl crate::Handler for EchoHandler {
            fn handle(&self, client_id: &str, msg: &Message) {
                println!("[Broker] Received from {}: {} -> {:?}",
                    client_id, msg.topic, String::from_utf8_lossy(&msg.payload));

                // Echo back to sender (would need client tracking for real impl)
            }
        }

        use parking_lot::RwLock;
        use std::collections::HashMap;
        use tokio::sync::mpsc;

        let clients: Arc<RwLock<HashMap<String, mpsc::Sender<Message>>>> =
            Arc::new(RwLock::new(HashMap::new()));

        let broker = Broker::builder(BrokerConfig::new(&addr))
            .handler(EchoHandler { clients: clients.clone() })
            .on_connect(|id| println!("[Broker] Client connected: {}", id))
            .on_disconnect(|id| println!("[Broker] Client disconnected: {}", id))
            .build();

        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Connect client
        let client = Client::connect(ClientConfig::new(&addr, "echo-client"))
            .await
            .unwrap();

        client.subscribe(&["echo/#"]).await.unwrap();
        println!("[Client] Subscribed to echo/#");

        // Send messages
        for i in 0..3 {
            let msg = format!("Hello {}", i);
            client.publish("echo/test", msg.as_bytes()).await.unwrap();
            println!("[Client] Published: {}", msg);
        }

        // Receive echoed messages
        for _ in 0..3 {
            if let Some(msg) = client.recv_timeout(Duration::from_secs(1)).await.unwrap() {
                println!("[Client] Received: {} -> {:?}",
                    msg.topic, String::from_utf8_lossy(&msg.payload));
            }
        }

        client.disconnect().await.unwrap();
        println!("=== Echo Server Example Complete ===");
    }

    /// Example: Multi-Client Chat
    ///
    /// Demonstrates multiple clients communicating through a broker.
    /// Run with: cargo test --package giztoy-mqtt0 example_multi_client_chat -- --ignored --nocapture
    #[tokio::test]
    #[ignore]
    async fn example_multi_client_chat() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        println!("=== Multi-Client Chat Example ===");
        println!("Starting broker on {}", addr);

        // Start broker
        let broker = Broker::builder(BrokerConfig::new(&addr))
            .on_connect(|id| println!("[Broker] {} joined the chat", id))
            .on_disconnect(|id| println!("[Broker] {} left the chat", id))
            .build();

        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Connect Alice and Bob
        let alice = Client::connect(ClientConfig::new(&addr, "alice")).await.unwrap();
        let bob = Client::connect(ClientConfig::new(&addr, "bob")).await.unwrap();

        // Both subscribe to chat room
        alice.subscribe(&["chat/room"]).await.unwrap();
        bob.subscribe(&["chat/room"]).await.unwrap();

        println!("[Alice] Joined chat room");
        println!("[Bob] Joined chat room");

        // Alice sends a message
        alice.publish("chat/room", b"Hello Bob!").await.unwrap();
        println!("[Alice] Sent: Hello Bob!");

        // Bob receives it
        if let Some(msg) = bob.recv_timeout(Duration::from_secs(1)).await.unwrap() {
            println!("[Bob] Received: {:?}", String::from_utf8_lossy(&msg.payload));
        }

        // Bob replies
        bob.publish("chat/room", b"Hi Alice!").await.unwrap();
        println!("[Bob] Sent: Hi Alice!");

        // Alice receives it
        if let Some(msg) = alice.recv_timeout(Duration::from_secs(1)).await.unwrap() {
            println!("[Alice] Received: {:?}", String::from_utf8_lossy(&msg.payload));
        }

        alice.disconnect().await.unwrap();
        bob.disconnect().await.unwrap();

        println!("=== Multi-Client Chat Example Complete ===");
    }

    /// Example: ACL-Protected Topics
    ///
    /// Demonstrates topic-level access control.
    /// Run with: cargo test --package giztoy-mqtt0 example_acl_protected -- --ignored --nocapture
    #[tokio::test]
    #[ignore]
    async fn example_acl_protected() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        println!("=== ACL Protected Topics Example ===");

        // Define ACL rules
        struct TopicAcl;
        impl Authenticator for TopicAcl {
            fn authenticate(&self, client_id: &str, username: &str, password: &[u8]) -> bool {
                // Simple username/password check
                let valid = match username {
                    "admin" => password == b"admin123",
                    "user" => password == b"user123",
                    _ => false,
                };
                println!("[ACL] Auth {} as {}: {}", client_id, username, if valid { "OK" } else { "DENIED" });
                valid
            }

            fn acl(&self, client_id: &str, topic: &str, write: bool) -> bool {
                let action = if write { "publish" } else { "subscribe" };

                // Admin can access everything
                if client_id.starts_with("admin") {
                    println!("[ACL] {} {} to {}: OK (admin)", client_id, action, topic);
                    return true;
                }

                // Regular users can only access public/* topics
                let allowed = topic.starts_with("public/");
                println!("[ACL] {} {} to {}: {}",
                    client_id, action, topic, if allowed { "OK" } else { "DENIED" });
                allowed
            }
        }

        let broker = Broker::builder(BrokerConfig::new(&addr))
            .authenticator(TopicAcl)
            .build();

        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        // Admin client
        let admin = Client::connect(
            ClientConfig::new(&addr, "admin-1")
                .with_credentials("admin", b"admin123".to_vec())
        ).await.unwrap();
        println!("[Admin] Connected");

        // Regular user client
        let user = Client::connect(
            ClientConfig::new(&addr, "user-1")
                .with_credentials("user", b"user123".to_vec())
        ).await.unwrap();
        println!("[User] Connected");

        // Admin subscribes to private topic (should succeed)
        admin.subscribe(&["private/admin"]).await.unwrap();
        println!("[Admin] Subscribed to private/admin: OK");

        // User tries to subscribe to private topic (should fail)
        let result = user.subscribe(&["private/secret"]).await;
        println!("[User] Subscribe to private/secret: {:?}", result);

        // User subscribes to public topic (should succeed)
        user.subscribe(&["public/news"]).await.unwrap();
        println!("[User] Subscribed to public/news: OK");

        // Admin publishes to both
        admin.publish("private/admin", b"Secret data").await.unwrap();
        admin.publish("public/news", b"Public announcement").await.unwrap();
        println!("[Admin] Published to both topics");

        // User receives public message
        if let Some(msg) = user.recv_timeout(Duration::from_secs(1)).await.unwrap() {
            println!("[User] Received from {}: {:?}",
                msg.topic, String::from_utf8_lossy(&msg.payload));
        }

        admin.disconnect().await.unwrap();
        user.disconnect().await.unwrap();

        println!("=== ACL Protected Topics Example Complete ===");
    }

    /// Example: High-Throughput Publishing
    ///
    /// Demonstrates high-speed message publishing with QoS 0.
    /// Run with: cargo test --package giztoy-mqtt0 example_high_throughput -- --ignored --nocapture
    #[tokio::test]
    #[ignore]
    async fn example_high_throughput() {
        let port = find_available_port();
        let addr = format!("127.0.0.1:{}", port);

        println!("=== High Throughput Example ===");

        let message_count = Arc::new(std::sync::atomic::AtomicUsize::new(0));
        let count_clone = message_count.clone();

        struct CountHandler(Arc<std::sync::atomic::AtomicUsize>);
        impl crate::Handler for CountHandler {
            fn handle(&self, _client_id: &str, _msg: &Message) {
                self.0.fetch_add(1, Ordering::SeqCst);
            }
        }

        let broker = Broker::builder(BrokerConfig::new(&addr))
            .handler(CountHandler(count_clone))
            .build();

        tokio::spawn(async move {
            let _ = broker.serve().await;
        });

        tokio::time::sleep(Duration::from_millis(100)).await;

        let client = Client::connect(ClientConfig::new(&addr, "throughput-client"))
            .await
            .unwrap();

        let payload = vec![0u8; 256]; // 256 bytes payload
        let num_messages = 10_000;

        println!("Publishing {} messages of 256 bytes each...", num_messages);

        let start = std::time::Instant::now();

        for _ in 0..num_messages {
            client.publish("throughput/test", &payload).await.unwrap();
        }

        let elapsed = start.elapsed();
        let rate = num_messages as f64 / elapsed.as_secs_f64();
        let throughput_mb = (num_messages * 256) as f64 / elapsed.as_secs_f64() / 1_000_000.0;

        println!("Completed in {:?}", elapsed);
        println!("Rate: {:.0} msg/s", rate);
        println!("Throughput: {:.2} MB/s", throughput_mb);

        // Give time for all messages to be processed
        tokio::time::sleep(Duration::from_millis(100)).await;

        println!("Broker received: {} messages", message_count.load(Ordering::SeqCst));

        client.disconnect().await.unwrap();
        println!("=== High Throughput Example Complete ===");
    }
}
