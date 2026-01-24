# chatgear/port (Go)

## Interfaces
- `ClientPortTx`: send Opus frames, state events, stats events
- `ClientPortRx`: receive Opus frames and commands
- `ServerPortTx`: create tracks, control tracks, issue commands
- `ServerPortRx`: receive Opus frames, state events, stats changes

## Audio Output
- Track creation: `NewBackgroundTrack`, `NewForegroundTrack`, `NewOverlayTrack`
- Track controls: `BackgroundTrackCtrl`, `ForegroundTrackCtrl`, `OverlayTrackCtrl`
- Global stop: `Interrupt()`

## Device Commands
- Volume, brightness, light mode
- WiFi set/delete, reset/unpair, sleep/shutdown, raise call
- OTA firmware upgrade

## Notes
- `ServerPortRx` provides getters for cached state/stat values.
- Audio tracks are based on `pcm.Track` and `pcm.TrackCtrl`.
