// Package transformers provides stream transformers for audio and text processing.
//
// # Overview
//
// This package implements genx.Transformer for various backends:
//   - TTS (Text-to-Speech): Text chunks → Audio chunks
//   - ASR (Speech-to-Text): Audio chunks → Text chunks
//   - Realtime: Bidirectional audio (Audio → Audio, with internal ASR+LLM+TTS)
//
// # Supported Backends
//
// Doubao (火山引擎):
//   - DoubaoTTSSeedV2: seed-tts-2.0 (大模型 TTS 2.0)
//   - DoubaoTTSICLV2: seed-icl-2.0 (声音复刻 2.0)
//   - DoubaoASRSAUC: volc.bigasr.sauc.duration (大模型 ASR)
//   - DoubaoRealtime: Doubao realtime conversation
//
// DashScope (阿里云):
//   - DashScopeRealtime: Qwen-Omni-Turbo-Realtime
//
// MiniMax:
//   - MinimaxTTS: MiniMax text-to-speech
//
// # Lifecycle
//
// All transformers in this package follow the genx.Transformer lifecycle contract:
//
//   - Transform(ctx) uses ctx ONLY for initialization (dial, handshake, session).
//   - Background goroutines do NOT hold ctx. They exit when input.Next()
//     returns io.EOF or error.
//   - To cancel a running transformer, close the input Stream.
//
// See genx.Transformer documentation for the full contract.
//
// # EOF vs EoS Convention
//
// Transformers handle two kinds of "end" signals differently:
//
// io.EOF (from input.Next()):
//   - The input Stream is physically done. No more chunks will arrive.
//   - Transformer flushes buffered data, emits results, and returns.
//   - The output Stream is closed by defer. Downstream sees io.EOF.
//   - Transformer does NOT fabricate an EoS marker.
//
// EoS marker (MessageChunk.Ctrl.EndOfStream=true):
//   - A logical sub-stream boundary sent by the CALLER.
//   - Transformer flushes buffered data, emits results.
//   - Transformer emits a TRANSLATED EoS marker (e.g., Text EoS → Audio EoS).
//   - Transformer continues the loop — more sub-streams may follow.
//
// # Usage
//
// Register transformers with patterns:
//
//	transformers.Handle("tts/cancan", NewDoubaoTTSSeedV2(client, "zh_female_cancan"))
//	transformers.Handle("asr/zh", NewDoubaoASRSAUC(client))
//
// Transform streams:
//
//	output := transformers.Transform(ctx, "tts/cancan", textStream)
//
// # Options
//
// Each transformer supports two types of configuration:
//
// 1. Construction-time options (functional options pattern):
//
//	tts := NewDoubaoTTSSeedV2(client, speaker,
//	    WithDoubaoTTSSeedV2Format("ogg_opus"),
//	    WithDoubaoTTSSeedV2SampleRate(24000),
//	)
//
// 2. Runtime options (via context, for future use):
//
//	ctx := WithDoubaoTTSSeedV2CtxOptions(ctx, DoubaoTTSSeedV2CtxOptions{})
//	output := tts.Transform(ctx, input)
package transformers
