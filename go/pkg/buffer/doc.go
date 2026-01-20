// Package buffer provides thread-safe buffer implementations for streaming data processing.
//
// The buffer package offers three main buffer types, each optimized for different use cases:
//
//   - BlockBuffer: A fixed-size circular buffer that blocks when full or empty.
//     Ideal for scenarios requiring predictable memory usage and flow control.
//
//   - Buffer: A growable buffer that automatically expands as needed.
//     Suitable for variable data sizes where the total size is unknown.
//
//   - RingBuffer: A fixed-size buffer that overwrites oldest data when full.
//     Perfect for maintaining sliding windows of recent data.
//
// All buffers implement common interfaces (io.Reader, io.Writer, io.Closer) and support
// concurrent access from multiple goroutines. They provide graceful shutdown mechanisms
// through CloseWrite() (allows reads to continue) or CloseWithError() (immediate closure).
//
// The package also includes convenience functions for creating byte-specific buffers
// with common sizes (1KB, 4KB, 16KB, etc.) and a BytesBuffer interface for unified
// access to byte buffer implementations.
//
// Example usage:
//
//	// Create a 4KB blocking buffer
//	buf := buffer.Bytes4KB()
//
//	// Write data
//	buf.Write([]byte("hello"))
//
//	// Read data
//	data := make([]byte, 5)
//	n, err := buf.Read(data)
//
//	// Graceful shutdown
//	buf.CloseWrite()
package buffer
