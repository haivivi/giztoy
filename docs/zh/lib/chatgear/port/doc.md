# chatgear/port

## Overview
Port interfaces represent higher-level client/server roles built on top of the
transport layer. They combine audio streaming, state/stats telemetry, and
command control into a single abstraction.

## Design Goals
- Provide a symmetric client/server API surface
- Hide transport details while preserving real-time audio controls
- Expose device control commands alongside audio output

## Key Concepts
- **ClientPort**: device-side send/receive split (Tx/Rx)
- **ServerPort**: server-side send/receive split (Tx/Rx)
- **Audio tracks**: background/foreground/overlay output streams
- **Device commands**: volume, brightness, WiFi, OTA, power, etc.
