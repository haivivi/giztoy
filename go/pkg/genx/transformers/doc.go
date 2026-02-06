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
