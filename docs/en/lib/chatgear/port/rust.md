# chatgear/port (Rust)

## Interfaces
- `ClientPortTx`: async send Opus frames, state events, stats events
- `ClientPortRx`: async receive Opus frames and commands
- `ServerPortTx`: track creation + device command methods
- `ServerPortRx`: async receive telemetry + cached getters

## Audio Output
- `AudioTrack` and `AudioTrackCtrl` traits
- Background/foreground/overlay track creation
- `interrupt()` to stop all output tracks

## Notes
- Port errors are consolidated under `PortError`.
- Track accessors return `Option<&dyn AudioTrackCtrl>` to reflect optional state.
