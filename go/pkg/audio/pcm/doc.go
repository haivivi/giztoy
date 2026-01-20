// Package pcm provides types and utilities for working with PCM (Pulse Code Modulation) audio data.
//
// The package defines audio formats for common configurations (16-bit mono at various sample rates)
// and provides interfaces for reading and writing audio chunks.
//
// Key types:
//   - Format: Represents audio format (sample rate, channels, bit depth)
//   - Chunk: Interface for audio data chunks
//   - DataChunk: Concrete implementation of Chunk for raw audio data
//   - SilenceChunk: Chunk that produces silence of a specified duration
//   - Writer: Interface for writing audio chunks
//
// Example usage:
//
//	// Create a 16kHz mono format
//	format := pcm.L16Mono16K
//
//	// Calculate bytes needed for 20ms of audio
//	bytes := format.BytesInDuration(20 * time.Millisecond)
//
//	// Create a silence chunk
//	silence := format.SilenceChunk(100 * time.Millisecond)
//
//	// Create a data chunk
//	chunk := format.DataChunk(audioData)
package pcm
