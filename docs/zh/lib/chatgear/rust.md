# chatgear (Rust)

## Crate Layout
- `state.rs`: gear state enum and events
- `stats.rs`: telemetry structs and merge logic
- `command.rs`: session commands and JSON helpers
- `conn.rs`: uplink/downlink async traits
- `port.rs`: client/server port traits
- `conn_pipe.rs`: in-process pipe helper

## Public Interfaces
- **State**: `GearState`, `GearStateEvent`, `GearStateChangeCause`
- **Stats**: `GearStatsEvent`, `GearStatsChanges`
- **Commands**: `SessionCommand`, `SessionCommandEvent`, `Command` enum
- **Uplink/Downlink**: `UplinkTx`, `UplinkRx`, `DownlinkTx`, `DownlinkRx`
- **Ports**: `ClientPortTx/Rx`, `ServerPortTx/Rx`
- **Pipe**: `new_pipe`

## Design Notes
- Async traits are used for most IO-facing APIs.
- Commands serialize into JSON value payloads; a typed `Command` enum can parse
  from `(type, payload)` pairs.
- Port traits split audio track control and device command APIs.

## Differences vs Go
- Rust favors async traits and owned `Vec<u8>` payloads.
- Command event uses `serde_json::Value` instead of typed interface payloads.
