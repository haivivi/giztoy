# mqtt0

## Overview
mqtt0 is a lightweight MQTT implementation focused on QoS 0. It provides both
client and broker components with explicit control over authentication and ACL.
The Go implementation is synchronous and net.Conn-based, while the Rust
implementation is async (Tokio) with optional TLS/WebSocket transport features.

## Design Goals
- Minimal MQTT feature set with strong QoS 0 focus
- Explicit ACL/auth hooks for connect/publish/subscribe
- Simple broker suitable for embedded or internal services
- Support MQTT 3.1.1 (v4) and MQTT 5.0 (v5)
- Provide transport flexibility (TCP/TLS/WebSocket)

## Key Concepts
- Client: QoS 0 publish/subscribe, keepalive, protocol v4/v5
- Broker: connection lifecycle, ACL checks, topic routing
- Shared subscriptions: $share/{group}/{topic}
- Topic alias (v5): reduce bandwidth by reusing alias per client
- Transports: TCP/TLS/WebSocket based on URL scheme or feature flags

## Components
- Client
- Broker
- Protocol parser/encoder
- Topic trie (subscription routing)
- Transport layer

## Protocol and Transport Support
- MQTT 3.1.1 and MQTT 5.0 for client and broker
- TCP and TLS by default
- WebSocket/WSS when enabled (Rust feature flags)

## Examples
- Go: use `Connect`, `Subscribe`, `Publish`, `Recv`
- Rust: use `Client::connect`, `client.subscribe`, `client.publish`, `client.recv`

## Related Modules
- `docs/lib/trie` (topic routing)
- `docs/lib/encoding` (protocol helpers)
