# chatgear/transport (Go)

## Interfaces
- `UplinkTx`: send stamped Opus frames, state events, stats events
- `UplinkRx`: receive Opus frames, state events, stats events (iter.Seq2)
- `DownlinkTx`: send Opus frames and commands; expose Opus encode options
- `DownlinkRx`: receive Opus frames and commands (iter.Seq2)

## Pipe Helper
- `NewPipe()` returns `PipeServerConn` (UplinkRx + DownlinkTx) and
  `PipeClientConn` (UplinkTx + DownlinkRx).

## Design Notes
- Iterators are used instead of channels for receiving streams.
- Opus frame timestamps are derived from `opusrt.Frame` durations.
