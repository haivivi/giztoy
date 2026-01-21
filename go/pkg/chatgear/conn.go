package chatgear

import (
	"context"
	"iter"
	"time"

	"github.com/haivivi/giztoy/pkg/audio/opusrt"
)

// =============================================================================
// Uplink: Client -> Server
// =============================================================================

// UplinkTx is the client-side transmitter for sending data to the server.
type UplinkTx interface {
	// SendOpusFrames sends opus frames to the server.
	SendOpusFrames(ctx context.Context, stamp opusrt.EpochMillis, frames ...[]byte) error

	// SendState sends a state event to the server.
	SendState(ctx context.Context, state *GearStateEvent) error

	// SendStats sends a stats event to the server.
	SendStats(ctx context.Context, stats *GearStatsEvent) error

	// Close closes the uplink.
	Close() error
}

// UplinkRx is the server-side receiver for data from the client.
type UplinkRx interface {
	// OpusFrames returns an iterator for opus frames from the client.
	OpusFrames() iter.Seq2[[]byte, error]

	// States returns an iterator for state events from the client.
	States() iter.Seq2[*GearStateEvent, error]

	// Stats returns an iterator for stats events from the client.
	Stats() iter.Seq2[*GearStatsEvent, error]

	// GearStats returns the latest gear stats from the client.
	GearStats() *GearStatsEvent

	// Close closes the receiver.
	Close() error
}

// =============================================================================
// Downlink: Server -> Client
// =============================================================================

// DownlinkTx is the server-side transmitter for sending data to the client.
type DownlinkTx interface {
	// SendOpusFrames sends opus frames to the client.
	SendOpusFrames(ctx context.Context, stamp opusrt.EpochMillis, frames ...[]byte) error

	// IssueCommand issues a command to the client.
	IssueCommand(ctx context.Context, cmd SessionCommand, t time.Time) error

	// OpusEncodeOptions returns the opus encoding options for the connection.
	OpusEncodeOptions() []opusrt.EncodePCMOption

	// Close closes the downlink.
	Close() error
}

// DownlinkRx is the client-side receiver for data from the server.
type DownlinkRx interface {
	// OpusFrames returns an iterator for opus frames from the server.
	OpusFrames() iter.Seq2[[]byte, error]

	// Commands returns an iterator for commands from the server.
	Commands() iter.Seq2[*SessionCommandEvent, error]

	// Close closes the receiver.
	Close() error
}
