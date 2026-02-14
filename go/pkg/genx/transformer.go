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
// # Context Lifecycle
//
// The ctx parameter controls ONLY the initialization phase, analogous to
// fs.Open(ctx) returning a *File whose lifetime is independent of ctx:
//
//   - Transform(ctx) — ctx governs initialization only (dial WebSocket,
//     handshake, create session, load model). Once Transform returns,
//     ctx's job is done.
//   - Background goroutines (transformLoop, processLoop) MUST NOT hold
//     or select on ctx. Their lifetime is governed entirely by the input
//     Stream: input.Next() returning io.EOF or error terminates the loop.
//   - To cancel a running transformer, the caller closes the input Stream.
//     This is analogous to calling file.Close() rather than canceling the
//     ctx that was used to open the file.
//
// # Stream Termination: EOF
//
// When input.Next() returns io.EOF, the input Stream is physically done.
// The transformer must:
//  1. Flush any buffered data and emit the results
//  2. Return from the processing goroutine (defer closes the output)
//
// The transformer MUST NOT fabricate an EoS marker on EOF. The downstream
// consumer detects termination by receiving io.EOF from output.Next().
//
// # Sub-stream Boundaries: EoS Markers
//
// EoS markers (MessageChunk.Ctrl.EndOfStream=true) are logical sub-stream
// boundaries sent by the CALLER, not by the transformer. A long-lived
// stream may contain multiple sub-streams delimited by EoS markers:
//
//	[text, text, EoS] → [text, text, EoS] → [text, text, EoS] → EOF
//	     sub-stream 1        sub-stream 2        sub-stream 3
//
// When a transformer receives an EoS marker matching its input type:
//  1. Flush buffered data and emit the results
//  2. Emit a TRANSLATED EoS marker with the output MIME type
//  3. Continue the loop — more sub-streams may follow
//
// This ensures EoS boundaries are preserved through the pipeline:
//
//	[Text] → TTS → [Audio] → ASR → [Text]
//	[Text EoS] → TTS → [Audio EoS] → ASR → [Text EoS]
//
// Summary:
//   - EoS received → flush + emit + translate EoS → continue loop
//   - EOF received → flush + emit → return (no EoS fabricated)
//
// # Examples
//
//   - TTS: text/plain → audio/* (translates Text EoS → Audio EoS)
//   - ASR: audio/* → text/plain (translates Audio EoS → Text EoS)
//   - Realtime: User Audio → Model Audio (changes Role to model)
//   - Format converter: audio/mp3 → audio/ogg (translates MP3 EoS → OGG EoS)
//
// # Error Propagation
//
// Transformers should support bidirectional error propagation:
//   - Forward: input.Close() → output.Next() returns EOF
//   - Backward: When output consumer calls CloseWithError(err),
//     the transformer should propagate the error to input
//
// This allows downstream errors to cancel upstream processing.
type Transformer interface {
	// Transform creates an output Stream from an input Stream.
	//
	// The ctx controls initialization only (connection, handshake, session
	// setup). Once Transform returns successfully, ctx is no longer used.
	// The pattern identifies the model/voice/resource (e.g., "doubao/vv",
	// "minimax/shaonv"). Concrete implementations may ignore the pattern.
	//
	// Transform should synchronously wait for initialization to complete
	// (e.g., WebSocket connection established) before returning.
	//
	// Returns error if initialization fails (e.g., connection refused).
	// Subsequent processing errors are returned via Stream.Next().
	Transform(ctx context.Context, pattern string, input Stream) (Stream, error)
}
