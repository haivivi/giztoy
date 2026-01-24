# mqtt0 (Rust)

## Crate Layout
- `lib.rs`: public exports, crate overview
- `client.rs`: async QoS 0 client
- `broker.rs`: async broker with ACL hooks
- `protocol.rs`: MQTT encode/decode
- `transport.rs`: TCP/TLS/WebSocket transport abstraction
- `trie.rs`: subscription routing
- `types.rs`: public types and traits

## Public Interfaces
- `Client`, `ClientConfig`: async connect/subscribe/publish/recv
- `Broker`, `BrokerConfig`, `BrokerBuilder`: broker setup and lifecycle
- `Authenticator`, `Handler`: ACL and message handling
- `Message`, `ProtocolVersion`, `QoS`
- `TransportType`, `Transport` (feature-gated TLS/WebSocket)

## Design Notes
- Fully async, based on Tokio and mpsc channels.
- Builder pattern for broker configuration and hooks.
- Transport features are behind Cargo feature flags (TLS, WebSocket).

## Differences vs Go
- Rust uses async traits for client/broker operations.
- TLS/WebSocket support is feature-gated.
- Broker construction encourages builder configuration.
