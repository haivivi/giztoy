package speech

import (
	"io"

	"github.com/haivivi/giztoy/go/pkg/audio/pcm"
)

// Speech is a stream of speech segments.
// Unlike Voice, Speech includes text transcription alongside audio.
type Speech interface {
	// Next returns the next speech segment.
	// Returns iterator.Done when no more segments are available.
	Next() (SpeechSegment, error)

	// Close closes the speech stream and releases resources.
	Close() error
}

// SpeechSegment is a single speech segment containing audio data and transcript.
type SpeechSegment interface {
	// Decode decodes the speech segment into a voice segment.
	// The best parameter suggests the preferred PCM format, but the
	// implementation may return a different format if more suitable.
	Decode(best pcm.Format) VoiceSegment

	// Transcribe returns a reader for the transcript text.
	// The caller should close the reader when done.
	Transcribe() io.ReadCloser

	// Close closes the segment and releases resources.
	Close() error
}

// SpeechStream is a stream of Speech objects.
type SpeechStream interface {
	// Next returns the next Speech.
	// Returns iterator.Done when no more speeches are available.
	Next() (Speech, error)

	// Close closes the stream and releases resources.
	Close() error
}
