// Package voiceprint provides speaker identification via audio embeddings
// and locality-sensitive hashing (LSH).
//
// # Architecture
//
// The pipeline processes audio in three stages:
//
//  1. Model.Extract: PCM16 16kHz mono audio → 192-dimensional embedding
//  2. Hasher.Hash: embedding → 16-bit hex hash (e.g., "A3F8")
//  3. Detector.Feed: sliding window of hashes → SpeakerStatus
//
// # Multi-Level Precision
//
// Voice hashes support multi-level precision via prefix truncation,
// similar to geohash:
//
//	16 bit: A3F8  ← exact match
//	12 bit: A3F   ← fuzzy match
//	 8 bit: A3    ← group level
//	 4 bit: A     ← coarse partition
//	 0 bit: *     ← no filter
//
// # Integration
//
// The Transformer wraps the full pipeline as a [genx.Transformer],
// consuming audio/pcm chunks and injecting SpeakerChunk metadata
// into the stream. Downstream consumers (e.g., memory) use the
// voice label strings ("voice:A3F8") without depending on this package.
package voiceprint

import "fmt"

// SpeakerStatus indicates the speaker detection result.
type SpeakerStatus int

const (
	// StatusUnknown means the speaker cannot be determined
	// (e.g., too much noise, hash is unstable).
	StatusUnknown SpeakerStatus = iota

	// StatusSingle means a single stable speaker is detected.
	StatusSingle

	// StatusOverlap means multiple speakers are detected
	// (hash alternates between two or more values).
	StatusOverlap
)

func (s SpeakerStatus) String() string {
	switch s {
	case StatusUnknown:
		return "unknown"
	case StatusSingle:
		return "single"
	case StatusOverlap:
		return "overlap"
	default:
		return fmt.Sprintf("SpeakerStatus(%d)", int(s))
	}
}

// SpeakerChunk is the output of the voiceprint detection pipeline.
// It represents the speaker state for a window of audio.
type SpeakerChunk struct {
	// Status is the detection result.
	Status SpeakerStatus

	// Speaker is the primary voice label (e.g., "voice:A3F8").
	// Empty when Status is StatusUnknown.
	Speaker string

	// Candidates lists all detected voice labels when Status is StatusOverlap.
	// For StatusSingle, this contains only the primary speaker.
	// For StatusUnknown, this is empty.
	Candidates []string

	// Confidence is a value in [0, 1] indicating detection confidence.
	// Higher values mean the hash window is more stable.
	Confidence float32
}

// VoiceLabel returns a prefixed voice label string for use as a graph entity
// or segment label. Format: "voice:{hash}".
func VoiceLabel(hash string) string {
	return "voice:" + hash
}
