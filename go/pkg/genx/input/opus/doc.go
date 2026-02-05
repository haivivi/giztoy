// Package opus provides utilities for converting Opus audio sources into genx.Stream.
//
// This package produces MessageChunks with MIMEType "audio/opus", where each chunk
// contains a single Opus frame.
//
// # Input Sources
//
//   - FromStampedReader: Real-time timestamped Opus with jitter buffer and pacing
//   - FromOpusReader: Sequential Opus frames (no timestamps)
//   - FromOggReader: OGG Opus container
//
// # Wire Format
//
// The StampedOpusReader interface expects data in the following format:
//
//	+--------+------------------+------------------+
//	| Version| Timestamp (7B)   | Opus Frame Data  |
//	| (1B)   | Big-endian ms    |                  |
//	+--------+------------------+------------------+
//
// Total header: 8 bytes
package opus
