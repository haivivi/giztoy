# chatgear/transport

## Overview
The transport layer defines bidirectional streaming interfaces for chatgear.
It splits data flow into uplink (device -> server) and downlink (server ->
device) and provides a test-friendly in-process pipe.

## Design Goals
- Separate uplink/downlink responsibilities
- Provide a minimal interface that can be implemented by different transports
- Keep Opus framing metadata explicit

## Key Concepts
- `UplinkTx` / `UplinkRx`: device -> server
- `DownlinkTx` / `DownlinkRx`: server -> device
- Stamped Opus frames: carry timestamp for playback alignment
- Pipe connection for in-process testing
