package chatgear

import (
	"iter"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
)

// StampedOpusFrame represents an opus frame with its timestamp.
type StampedOpusFrame struct {
	Timestamp time.Time
	Frame     opus.Frame
}

// =============================================================================
// Uplink: Client -> Server
// =============================================================================

// UplinkTx is the client-side transmitter for sending data to the server.
type UplinkTx interface {
	// SendOpusFrame sends an opus frame to the server.
	SendOpusFrame(timestamp time.Time, frame opus.Frame) error

	// SendState sends a state event to the server.
	SendState(state *StateEvent) error

	// SendStats sends a stats event to the server.
	SendStats(stats *StatsEvent) error

	// Close closes the uplink.
	Close() error
}

// UplinkRx is the server-side receiver for data from the client.
type UplinkRx interface {
	// OpusFrames returns an iterator for opus frames from the client.
	OpusFrames() iter.Seq2[StampedOpusFrame, error]

	// States returns an iterator for state events from the client.
	States() iter.Seq2[*StateEvent, error]

	// Stats returns an iterator for stats events from the client.
	Stats() iter.Seq2[*StatsEvent, error]

	// LatestStats returns the latest stats from the client.
	LatestStats() *StatsEvent

	// Close closes the receiver.
	Close() error
}

// =============================================================================
// Downlink: Server -> Client
// =============================================================================

// DownlinkTx is the server-side transmitter for sending data to the client.
type DownlinkTx interface {
	// SendOpusFrame sends an opus frame to the client.
	SendOpusFrame(timestamp time.Time, frame opus.Frame) error

	// IssueCommand issues a command to the client.
	IssueCommand(cmd Command, t time.Time) error

	// Close closes the downlink.
	Close() error
}

// DownlinkRx is the client-side receiver for data from the server.
type DownlinkRx interface {
	// OpusFrames returns an iterator for opus frames from the server.
	OpusFrames() iter.Seq2[StampedOpusFrame, error]

	// Commands returns an iterator for commands from the server.
	Commands() iter.Seq2[*CommandEvent, error]

	// Close closes the receiver.
	Close() error
}
