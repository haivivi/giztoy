// Package audio provides audio processing utilities.
//
// This package serves as an umbrella for audio-related sub-packages:
//
//   - pcm: PCM (Pulse Code Modulation) audio format handling
//
// For buffer utilities, use the separate github.com/haivivi/giztoy/go/pkg/buffer package.
//
// Example usage:
//
//	import (
//	    "github.com/haivivi/giztoy/pkg/audio/pcm"
//	    "github.com/haivivi/giztoy/pkg/buffer"
//	)
//
//	// Create a buffer for audio data
//	buf := buffer.Bytes16KB()
//
//	// Work with PCM format
//	format := pcm.L16Mono16K
//	chunk := format.DataChunk(audioData)
package audio
