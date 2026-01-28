package chatgear

import (
	"context"
	"iter"
	"sync"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/opusrt"
)

// NewPipe creates a connected pair of server and client connections using channels.
// This is useful for testing and in-process communication.
func NewPipe() (*PipeServerConn, *PipeClientConn) {
	// Uplink channels (client -> server)
	uplinkOpus := make(chan []byte, 1024)
	uplinkStates := make(chan *GearStateEvent, 32)
	uplinkStats := make(chan *GearStatsEvent, 32)

	// Downlink channels (server -> client)
	downlinkOpus := make(chan []byte, 1024)
	downlinkCmds := make(chan *SessionCommandEvent, 32)

	// Shared error state for cross-connection error propagation
	shared := &pipeSharedState{}

	server := &PipeServerConn{
		uplinkOpus:   uplinkOpus,
		uplinkStates: uplinkStates,
		uplinkStats:  uplinkStats,
		downlinkOpus: downlinkOpus,
		downlinkCmds: downlinkCmds,
		shared:       shared,
	}

	client := &PipeClientConn{
		uplinkOpus:   uplinkOpus,
		uplinkStates: uplinkStates,
		uplinkStats:  uplinkStats,
		downlinkOpus: downlinkOpus,
		downlinkCmds: downlinkCmds,
		shared:       shared,
	}

	return server, client
}

// pipeSharedState holds shared state between server and client connections.
type pipeSharedState struct {
	mu        sync.Mutex
	serverErr error // Error set by server, readable by client
	clientErr error // Error set by client, readable by server
}

// PipeServerConn is the server side of a pipe connection.
// It implements UplinkRx (receive from client) and DownlinkTx (send to client).
type PipeServerConn struct {
	// Uplink channels (receive from client)
	uplinkOpus   chan []byte
	uplinkStates chan *GearStateEvent
	uplinkStats  chan *GearStatsEvent

	// Downlink channels (send to client)
	downlinkOpus chan []byte
	downlinkCmds chan *SessionCommandEvent

	shared *pipeSharedState

	mu             sync.Mutex
	gearStats      *GearStatsEvent
	opusEncodeOpts []opusrt.EncodePCMOption
	closed         bool
}

// --- UplinkRx implementation (receive from client) ---

func (c *PipeServerConn) OpusFrames() iter.Seq2[[]byte, error] {
	return func(yield func([]byte, error) bool) {
		for frame := range c.uplinkOpus {
			if !yield(frame, nil) {
				return
			}
		}
		// Get client error from shared state
		c.shared.mu.Lock()
		err := c.shared.clientErr
		c.shared.mu.Unlock()
		if err != nil {
			yield(nil, err)
		}
	}
}

func (c *PipeServerConn) States() iter.Seq2[*GearStateEvent, error] {
	return func(yield func(*GearStateEvent, error) bool) {
		for state := range c.uplinkStates {
			if !yield(state, nil) {
				return
			}
		}
		c.shared.mu.Lock()
		err := c.shared.clientErr
		c.shared.mu.Unlock()
		if err != nil {
			yield(nil, err)
		}
	}
}

func (c *PipeServerConn) Stats() iter.Seq2[*GearStatsEvent, error] {
	return func(yield func(*GearStatsEvent, error) bool) {
		for stats := range c.uplinkStats {
			c.mu.Lock()
			c.gearStats = stats
			c.mu.Unlock()
			if !yield(stats, nil) {
				return
			}
		}
		c.shared.mu.Lock()
		err := c.shared.clientErr
		c.shared.mu.Unlock()
		if err != nil {
			yield(nil, err)
		}
	}
}

func (c *PipeServerConn) GearStats() *GearStatsEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gearStats
}

// --- DownlinkTx implementation (send to client) ---

func (c *PipeServerConn) SendOpusFrames(ctx context.Context, stamp opusrt.EpochMillis, frames ...[]byte) error {
	for _, frame := range frames {
		stamped := opusrt.Stamp(frame, stamp)
		select {
		case c.downlinkOpus <- stamped:
		case <-ctx.Done():
			return ctx.Err()
		}
		stamp += opusrt.EpochMillis(opusrt.Frame(frame).Duration().Milliseconds())
	}
	return nil
}

func (c *PipeServerConn) IssueCommand(ctx context.Context, cmd SessionCommand, t time.Time) error {
	evt := NewSessionCommandEvent(cmd, t)
	select {
	case c.downlinkCmds <- evt:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *PipeServerConn) SetOpusEncodeOptions(opts ...opusrt.EncodePCMOption) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.opusEncodeOpts = opts
}

func (c *PipeServerConn) OpusEncodeOptions() []opusrt.EncodePCMOption {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.opusEncodeOpts
}

// --- Lifecycle ---

func (c *PipeServerConn) Close() error {
	return c.CloseWithError(nil)
}

func (c *PipeServerConn) CloseWithError(err error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	// Set server error in shared state for client to read
	c.shared.mu.Lock()
	c.shared.serverErr = err
	c.shared.mu.Unlock()

	// Close downlink channels (server owns these)
	close(c.downlinkOpus)
	close(c.downlinkCmds)
	return nil
}

// PipeClientConn is the client side of a pipe connection.
// It implements UplinkTx (send to server) and DownlinkRx (receive from server).
type PipeClientConn struct {
	// Uplink channels (send to server)
	uplinkOpus   chan []byte
	uplinkStates chan *GearStateEvent
	uplinkStats  chan *GearStatsEvent

	// Downlink channels (receive from server)
	downlinkOpus chan []byte
	downlinkCmds chan *SessionCommandEvent

	shared *pipeSharedState

	mu     sync.Mutex
	closed bool
}

// --- UplinkTx implementation (send to server) ---

func (c *PipeClientConn) SendOpusFrames(ctx context.Context, stamp opusrt.EpochMillis, frames ...[]byte) error {
	for _, frame := range frames {
		stamped := opusrt.Stamp(frame, stamp)
		select {
		case c.uplinkOpus <- stamped:
		case <-ctx.Done():
			return ctx.Err()
		}
		stamp += opusrt.EpochMillis(opusrt.Frame(frame).Duration().Milliseconds())
	}
	return nil
}

func (c *PipeClientConn) SendState(ctx context.Context, state *GearStateEvent) error {
	select {
	case c.uplinkStates <- state:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *PipeClientConn) SendStats(ctx context.Context, stats *GearStatsEvent) error {
	select {
	case c.uplinkStats <- stats:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// --- DownlinkRx implementation (receive from server) ---

func (c *PipeClientConn) OpusFrames() iter.Seq2[[]byte, error] {
	return func(yield func([]byte, error) bool) {
		for frame := range c.downlinkOpus {
			if !yield(frame, nil) {
				return
			}
		}
		// Get server error from shared state
		c.shared.mu.Lock()
		err := c.shared.serverErr
		c.shared.mu.Unlock()
		if err != nil {
			yield(nil, err)
		}
	}
}

func (c *PipeClientConn) Commands() iter.Seq2[*SessionCommandEvent, error] {
	return func(yield func(*SessionCommandEvent, error) bool) {
		for cmd := range c.downlinkCmds {
			if !yield(cmd, nil) {
				return
			}
		}
		c.shared.mu.Lock()
		err := c.shared.serverErr
		c.shared.mu.Unlock()
		if err != nil {
			yield(nil, err)
		}
	}
}

// --- Lifecycle ---

func (c *PipeClientConn) Close() error {
	return c.CloseWithError(nil)
}

func (c *PipeClientConn) CloseWithError(err error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	// Set client error in shared state for server to read
	c.shared.mu.Lock()
	c.shared.clientErr = err
	c.shared.mu.Unlock()

	// Close uplink channels (client owns these)
	close(c.uplinkOpus)
	close(c.uplinkStates)
	close(c.uplinkStats)
	return nil
}

// Compile-time interface assertions
var (
	_ UplinkRx   = (*PipeServerConn)(nil)
	_ DownlinkTx = (*PipeServerConn)(nil)
	_ UplinkTx   = (*PipeClientConn)(nil)
	_ DownlinkRx = (*PipeClientConn)(nil)
)
