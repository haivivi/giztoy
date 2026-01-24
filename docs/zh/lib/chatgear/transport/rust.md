# chatgear/transport (Rust)

## Interfaces
- `UplinkTx`: async send Opus frames, state events, stats events
- `UplinkRx`: async receive Opus frames, state events, stats events
- `DownlinkTx`: async send Opus frames and commands
- `DownlinkRx`: async receive Opus frames and commands
- `OpusEncodeOptions`: connection-level Opus encoding metadata

## Connection Traits
- `ServerConn`: `UplinkRx + DownlinkTx`
- `ClientConn`: `UplinkTx + DownlinkRx`

## Design Notes
- All transport interfaces are async traits.
- Opus encode metadata is modeled explicitly but not attached to the traits.
