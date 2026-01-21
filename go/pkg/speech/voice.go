// Package speech provides interfaces for voice and speech processing.
// Voice represents pure audio data, while Speech adds text transcription.
// This package also provides ASR (Automatic Speech Recognition) and
// TTS (Text-to-Speech) service interfaces.
package speech

import (
	"github.com/haivivi/giztoy/pkg/audio/pcm"
)

// Voice is a stream of audio segments.
type Voice interface {
	// Next returns the next voice segment.
	// Returns iterator.Done when no more segments are available.
	Next() (VoiceSegment, error)

	// Close closes the voice stream and releases resources.
	Close() error
}

// VoiceSegment is a single audio segment, which can be read as PCM data.
type VoiceSegment interface {
	// Read reads PCM data into p.
	Read(p []byte) (n int, err error)

	// Format returns the PCM format of this segment.
	Format() pcm.Format

	// Close closes the segment and releases resources.
	Close() error
}

// VoiceStream is a stream of Voice objects.
type VoiceStream interface {
	// Next returns the next Voice.
	// Returns iterator.Done when no more voices are available.
	Next() (Voice, error)

	// Close closes the stream and releases resources.
	Close() error
}
