// Package opusrt provides real-time Opus stream processing utilities.
//
// This package handles:
//   - Timestamped Opus frames (StampedFrame)
//   - Jitter buffer for out-of-order frame reordering
//   - Real-time playback simulation with packet loss handling
//   - OGG container reading and writing
//
// The core types are:
//   - Frame: Raw Opus frame data
//   - StampedFrame: Opus frame with embedded timestamp
//   - EpochMillis: Millisecond-precision timestamp
//   - Buffer: Jitter buffer for frame reordering
//   - RealtimeBuffer: Real-time playback with loss detection
//
// Example usage:
//
//	// Create a jitter buffer
//	buf := opusrt.NewBuffer(2 * time.Minute)
//
//	// Write stamped frames (can arrive out of order)
//	buf.Write(stampedData)
//
//	// Read frames in order
//	frame, loss, err := buf.Frame()
//	if loss > 0 {
//	    // Handle packet loss with PLC
//	}
package opusrt
