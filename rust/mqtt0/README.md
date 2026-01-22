# mqtt0 - Lightweight QoS 0 MQTT Library

A lightweight MQTT library focusing on QoS 0 (at-most-once delivery), with `no_std` support for embedded systems.

## Architecture

The library is split into three crates:

```
mqtt0/
├── src/              # Core protocol crate (no_std compatible)
│   ├── lib.rs
│   ├── error.rs      # Error types
│   ├── types.rs      # Common types (Message, QoS, etc.)
│   └── protocol/     # MQTT protocol encoding/decoding
│       ├── mod.rs
│       ├── codec.rs  # Low-level encoding utilities
│       ├── v4.rs     # MQTT 3.1.1 support
│       └── v5.rs     # MQTT 5.0 support
│
├── client/           # Client crate (mqtt0-client)
│   └── src/
│       ├── lib.rs
│       ├── config.rs
│       ├── error.rs
│       └── tokio_client.rs
│
└── broker/           # Broker crate (mqtt0-broker)
    └── src/
        ├── lib.rs
        ├── broker.rs
        ├── error.rs
        ├── trie.rs   # Topic matching
        └── types.rs  # Authenticator, Handler traits
```

## Features

### mqtt0 (core)

- `std` (default): Standard library support
- `alloc`: Heap allocation support (required for most operations)
- `rumqttc-compat`: Use rumqttc's mqttbytes for parsing (std only)

### mqtt0-client

- `tokio` (default): Tokio async runtime
- `embassy`: Embassy async runtime (for embedded systems) - *planned*

### mqtt0-broker

- `tokio` (default): Tokio async runtime
- `tls`: TLS support
- `websocket`: WebSocket support
- `full`: All features enabled

## no_std Usage

To use the core crate in a `no_std` environment:

```toml
[dependencies]
mqtt0 = { version = "0.1", default-features = false, features = ["alloc"] }
```

## Example

### Client

```rust
use mqtt0_client::{Client, ClientConfig};

#[tokio::main]
async fn main() -> mqtt0_client::Result<()> {
    let client = Client::connect(
        ClientConfig::new("127.0.0.1:1883", "my-client")
    ).await?;

    client.subscribe(&["test/topic"]).await?;
    client.publish("test/topic", b"hello").await?;

    let msg = client.recv().await?;
    println!("Received: {:?}", msg);

    client.disconnect().await?;
    Ok(())
}
```

### Broker

```rust
use mqtt0_broker::{Broker, BrokerConfig};

#[tokio::main]
async fn main() -> mqtt0_broker::Result<()> {
    let broker = Broker::builder(BrokerConfig::new("127.0.0.1:1883"))
        .on_connect(|client_id| println!("Connected: {}", client_id))
        .on_disconnect(|client_id| println!("Disconnected: {}", client_id))
        .build();

    broker.serve().await
}
```

## Protocol Support

- **MQTT 3.1.1**: Full QoS 0 support
- **MQTT 5.0**: Basic QoS 0 support (properties partially implemented)

## Design Principles

1. **Simplicity**: Focus on QoS 0 only
2. **no_std Compatible**: Core protocol works without std
3. **Runtime Agnostic**: Client supports multiple async runtimes
4. **Zero-Copy Where Possible**: Use `bytes::Bytes` for payloads
5. **Full ACL Control**: Broker provides complete authentication/authorization

## License

MIT
