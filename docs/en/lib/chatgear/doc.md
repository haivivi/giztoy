# chatgear

## Overview
chatgear defines the core protocol types for device-to-server communication:
commands, state events, statistics, and audio streaming metadata. It focuses on
interface design rather than transport implementation, and provides an
in-process pipe for testing.

## Design Goals
- Stable, typed protocol for device state and control
- Clear separation between uplink (device -> server) and downlink (server -> device)
- Explicit metadata for timestamps and command issuance
- Audio streaming with Opus frame stamping
- Support both Go and Rust with comparable API surfaces

## Key Concepts
- **Session commands**: device control commands with typed payloads
- **State events**: gear state transitions with causes
- **Stats events**: telemetry snapshots and incremental changes
- **Uplink/Downlink**: split interfaces for bidirectional streams
- **Ports**: higher-level client/server port abstraction

## Submodules
- `transport`: uplink/downlink connection traits and pipe helpers
- `port`: client/server port traits and audio track controls

## External Reference
- `/Users/idy/Work/haivivi/x/docs/chatgear` (original protocol/design notes)

## Related Modules
- `docs/lib/audio/opusrt` for Opus frame handling
- `docs/lib/jsontime` for millisecond timestamps
