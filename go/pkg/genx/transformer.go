package genx

import "context"

// Transformer converts a Stream into another Stream.
//
// # Contract
//
// Transformers may modify any field of MessageChunk:
//   - Role: e.g., realtime model changes user → model
//   - Name: e.g., set to model name
//   - Part: e.g., TTS converts Text → Blob, ASR converts Blob → Text
//   - Ctrl: preserve or modify as needed
//
// # Type Contract
//
// Each transformer should declare:
//   - Input type: MIME type(s) it processes (e.g., "text/plain", "audio/*")
//   - Output type: MIME type(s) it produces (e.g., "audio/ogg", "text/plain")
//
// Transformers must:
//   - Process only messages matching their input type
//   - Pass through non-matching messages unchanged
//
// # EoS (End-of-Stream) Handling
//
// When a transformer receives an EoS marker matching its input type:
//  1. Finish processing current input (e.g., complete TTS synthesis)
//  2. Emit all pending output chunks
//  3. Emit an EoS marker with the transformer's output MIME type
//
// This ensures EoS boundaries are preserved through the pipeline:
//
//	[Text] -> TTS -> [Audio] -> ASR -> [Text]
//	[Text EoS] -> TTS -> [Audio EoS] -> ASR -> [Text EoS]
//
// # Examples
//
//   - TTS: text/plain → audio/* (converts Text EoS → Audio EoS)
//   - ASR: audio/* → text/plain (converts Audio EoS → Text EoS)
//   - Realtime: User Audio → Model Audio (changes Role to model)
//   - Format converter: audio/mp3 → audio/ogg (converts MP3 EoS → OGG EoS)
//
// # Error Propagation
//
// Transformers should support bidirectional error propagation:
//   - Forward: input.Close() -> output.Next() returns EOF
//   - Backward: When output consumer calls CloseWithError(err),
//     the transformer should propagate the error to input
//
// This allows downstream errors to cancel upstream processing.
//
// # Context
//
// The ctx parameter carries:
//   - Cancellation signals
//   - Runtime options (transformer-specific, via context values)
type Transformer interface {
	// Transform creates an output Stream from an input Stream.
	// The ctx can carry cancellation and transformer-specific options.
	// The pattern identifies the model/voice/resource (e.g., "doubao/vv", "minimax/shaonv").
	// Concrete implementations may ignore the pattern parameter.
	//
	// Transform should synchronously wait for initialization to complete
	// (e.g., WebSocket connection established) before returning.
	// The ctx is used for timeout control during initialization.
	//
	// Returns error if initialization fails (e.g., connection refused).
	// Subsequent processing errors are returned via Stream.Next().
	Transform(ctx context.Context, pattern string, input Stream) (Stream, error)
}
