// Package cortex provides components for connecting chatgear devices with
// genx transformers.
//
// # Atom
//
// Atom is the core component that bridges a chatgear.ServerPort with a
// genx.Transformer. It handles:
//
//   - State machine (recording, waiting, calling, ready, off)
//   - Audio input: decodes Opus from device, converts to genx.Stream
//   - Audio output: reads from Transformer output, writes to device
//   - Track management: handles audio interruption via stream IDs
//   - Streaming state: automatically sends streaming:true/false commands
//
// # Usage
//
//	port := chatgear.NewServerPort()
//	transformer := transformers.NewDashScopeRealtime(cfg)
//
//	atom := cortex.New(cortex.Config{
//	    Port:        port,
//	    Transformer: transformer,
//	})
//
//	// Run blocks until context is cancelled or port closes
//	atom.Run(ctx)
//
// # State Machine
//
// The Atom handles the following states from the device:
//
//   - StateRecording: User is speaking (push-to-talk mode)
//   - StateWaitingForResponse: User finished speaking, waiting for AI
//   - StateCalling: Continuous conversation mode (server VAD)
//   - StateReady: Device is idle
//   - StateOff: Device is powered off
//
// # Audio Flow
//
//	Device (Opus) -> Atom -> genx.Stream (PCM) -> Transformer
//	Transformer -> genx.Stream (PCM/Audio) -> Atom -> Device (Opus)
package cortex
