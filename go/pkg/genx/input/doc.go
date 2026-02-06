// Package input provides utilities for converting audio sources into genx.Stream.
//
// # Subpackages
//
//   - input/opus: Convert Opus audio sources to genx.Stream (audio/opus chunks)
//
// # Generic Utilities
//
// This package provides a generic JitterBuffer that can be used to reorder
// out-of-order packets by timestamp. It is used internally by input/opus
// for real-time audio streaming.
package input
