# chatgear (Go)

## Package Layout
- `state.go`: gear state enum, state events
- `stats.go`: telemetry structs, merge logic
- `command.go`: session command types and JSON mapping
- `conn.go`: uplink/downlink interfaces
- `port.go`: client/server port interfaces
- `conn_pipe.go`: in-process pipe connection for tests

## Public Interfaces
- **State**: `GearState`, `GearStateEvent`, `GearStateChangeCause`
- **Stats**: `GearStatsEvent`, `GearStatsChanges` and related structs
- **Commands**: `SessionCommand` with `SessionCommandEvent`
- **Uplink/Downlink**: `UplinkTx`, `UplinkRx`, `DownlinkTx`, `DownlinkRx`
- **Ports**: `ClientPortTx/Rx`, `ServerPortTx/Rx`
- **Pipe**: `NewPipe` for test or in-process wiring

## Design Notes
- JSON encoding is typed via `commandType()` and a tagged event wrapper.
- All time fields use `jsontime.Milli` for millisecond epoch values.
- Opus frames are stamped with `opusrt.EpochMillis` during transport.
- Stats merge logic performs partial updates with `GearStatsChanges`.

## Usage Notes
- `UplinkRx`/`DownlinkRx` expose iterators (`iter.Seq2`) instead of channels.
- `ServerPortTx` exposes track creation for background/foreground/overlay audio.
