// Package genx provides a unified streaming framework for multi-modal AI.
//
// # Core Types
//
// MessageChunk is the fundamental unit of data in a Stream:
//   - Role: The producer of this message (user, model, or tool)
//   - Name: The name of the producer (e.g., "alice", "assistant")
//   - Part: The content payload (Text or Blob)
//   - Ctrl: Stream control signals (optional, for routing and state)
//
// Stream is the primary data flow abstraction:
//
//	type Stream interface {
//	    Next() (*MessageChunk, error)
//	    Close() error
//	    CloseWithError(error) error
//	}
//
// Transformer converts a Stream into another Stream, and may modify
// any field of MessageChunk (Role, Name, Part, Ctrl).
//
// # Package Structure
//
//   - genx/input: Convert external sources to genx.Stream
//     (e.g., audio input with realtime pacing, jitter buffer)
//
//   - genx/output: Route genx.Stream to multiple outputs
//     (demux by StreamID, Role, and MIME type)
//
//   - genx/transformers: Stream transformers
//     (TTS, ASR, realtime models - may modify any MessageChunk field)
//
//   - genx/generators: LLM model wrappers
//     (OpenAI, Gemini, etc.)
//
//   - genx/agent: Agent implementations
//     (ReAct agent, Match agent)
//
//   - genx/match: Intent matching utilities
//
//   - genx/luau: Luau scripting integration
//
// # Data Flow Example
//
// A typical audio conversation pipeline:
//
//	Audio Input -> ASR Transformer -> Agent -> TTS Transformer -> Audio Output
//	(Role=user)    (Part: audio→text)         (Part: text→audio)  (Role=model)
//
// Notice that Role stays "user" through ASR (it's still user's words),
// and becomes "model" after the Agent processes it.
package genx
