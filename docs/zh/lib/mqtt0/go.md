# mqtt0 (Go)

## Package Layout
- `doc.go`: high-level overview and usage examples
- `client.go`: QoS 0 client implementation
- `broker.go`: broker implementation with ACL hooks
- `packet_v4.go`, `packet_v5.go`, `packet.go`: protocol encode/decode
- `listener.go`, `dialer.go`: transport helpers
- `trie.go`: subscription routing

## Public Interfaces
- `ClientConfig`: broker address, protocol version, TLS config, keepalive, etc.
- `Client`: `Connect`, `Subscribe`, `Unsubscribe`, `Publish`, `Recv`, `Close`
- `Broker`: `Serve`, `ServeConn`, ACL hooks, callbacks
- `Authenticator`: access control on connect/publish/subscribe
- `Handler`: callback for inbound broker messages
- `Message`, `ProtocolVersion`, `QoS`

## Design Notes
- Single connection with separate read/write locks to guard concurrent access.
- Request/response operations (SUBSCRIBE/UNSUBSCRIBE) read from the same stream
  as inbound PUBLISH messages.
- Keepalive runs in a goroutine when `AutoKeepalive` is enabled.
- Shared subscriptions and topic aliasing are handled in the broker.

## Transport
- URL-based address parsing: `tcp://`, `tls://`, `ws://`, `wss://`
- `Dialer` hook allows custom connection logic
- TLS config supported via `ClientConfig.TLSConfig`

## Notable Behaviors
- QoS 0 only; no packet persistence or retransmission.
- Broker drops messages when per-client channel is full (non-blocking send).
