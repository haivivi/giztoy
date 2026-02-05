package ogg

import (
	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
)

// OpusPacket represents an Opus packet extracted from an OGG container.
type OpusPacket struct {
	// Frame contains the raw Opus encoded data.
	Frame opus.Frame

	// Granule is the granule position (48kHz PCM samples) of this packet.
	// This represents the end position of the audio data in this packet.
	Granule int64

	// SerialNo identifies which logical stream this packet belongs to.
	// OGG files can contain multiple multiplexed or chained streams.
	SerialNo int32

	// BOS indicates this is the first packet of a logical stream (Beginning Of Stream).
	BOS bool

	// EOS indicates this is the last packet of a logical stream (End Of Stream).
	EOS bool
}
