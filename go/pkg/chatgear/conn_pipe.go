package chatgear

import (
	"iter"
	"sync"
	"time"

	"github.com/haivivi/giztoy/go/pkg/audio/codec/opus"
)

// NewPipe creates a connected pair of server and client connections using channels.
// This is useful for testing and in-process communication.
func NewPipe() (*PipeServerConn, *PipeClientConn) {
	// Uplink channels (client -> server)
	uplinkOpus := make(chan StampedOpusFrame, 1024)
	uplinkStates := make(chan *StateEvent, 32)
	uplinkStats := make(chan *StatsEvent, 32)

	// Downlink channels (server -> client)
	downlinkOpus := make(chan StampedOpusFrame, 1024)
	downlinkCmds := make(chan *CommandEvent, 32)

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

// =============================================================================
// PipeServerConn: Server side of pipe connection
// Implements UplinkRx (receive from client) and DownlinkTx (send to client)
// =============================================================================

type PipeServerConn struct {
	// Uplink channels (receive from client)
	uplinkOpus   chan StampedOpusFrame
	uplinkStates chan *StateEvent
	uplinkStats  chan *StatsEvent

	// Downlink channels (send to client)
	downlinkOpus chan StampedOpusFrame
	downlinkCmds chan *CommandEvent

	shared *pipeSharedState

	mu          sync.Mutex
	latestStats *StatsEvent
	closed      bool
}

// --- UplinkRx implementation (receive from client) ---

func (c *PipeServerConn) OpusFrames() iter.Seq2[StampedOpusFrame, error] {
	return func(yield func(StampedOpusFrame, error) bool) {
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
			yield(StampedOpusFrame{}, err)
		}
	}
}

func (c *PipeServerConn) States() iter.Seq2[*StateEvent, error] {
	return func(yield func(*StateEvent, error) bool) {
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

func (c *PipeServerConn) Stats() iter.Seq2[*StatsEvent, error] {
	return func(yield func(*StatsEvent, error) bool) {
		for stats := range c.uplinkStats {
			c.mu.Lock()
			c.latestStats = stats
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

func (c *PipeServerConn) LatestStats() *StatsEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.latestStats
}

// --- DownlinkTx implementation (send to client) ---

func (c *PipeServerConn) SendOpusFrame(timestamp time.Time, frame opus.Frame) error {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return nil
	}

	select {
	case c.downlinkOpus <- StampedOpusFrame{Timestamp: timestamp, Frame: frame}:
		return nil
	default:
		// Channel full, drop frame
		return nil
	}
}

func (c *PipeServerConn) IssueCommand(cmd Command, t time.Time) error {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return nil
	}

	evt := NewCommandEvent(cmd, t)
	select {
	case c.downlinkCmds <- evt:
		return nil
	default:
		// Channel full, drop command
		return nil
	}
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

// =============================================================================
// PipeClientConn: Client side of pipe connection
// Implements UplinkTx (send to server) and DownlinkRx (receive from server)
// =============================================================================

type PipeClientConn struct {
	// Uplink channels (send to server)
	uplinkOpus   chan StampedOpusFrame
	uplinkStates chan *StateEvent
	uplinkStats  chan *StatsEvent

	// Downlink channels (receive from server)
	downlinkOpus chan StampedOpusFrame
	downlinkCmds chan *CommandEvent

	shared *pipeSharedState

	mu     sync.Mutex
	closed bool
}

// --- UplinkTx implementation (send to server) ---

func (c *PipeClientConn) SendOpusFrame(timestamp time.Time, frame opus.Frame) error {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return nil
	}

	select {
	case c.uplinkOpus <- StampedOpusFrame{Timestamp: timestamp, Frame: frame}:
		return nil
	default:
		// Channel full, drop frame
		return nil
	}
}

func (c *PipeClientConn) SendState(state *StateEvent) error {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return nil
	}

	select {
	case c.uplinkStates <- state:
		return nil
	default:
		// Channel full, drop state
		return nil
	}
}

func (c *PipeClientConn) SendStats(stats *StatsEvent) error {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return nil
	}

	select {
	case c.uplinkStats <- stats:
		return nil
	default:
		// Channel full, drop stats
		return nil
	}
}

// --- DownlinkRx implementation (receive from server) ---

func (c *PipeClientConn) OpusFrames() iter.Seq2[StampedOpusFrame, error] {
	return func(yield func(StampedOpusFrame, error) bool) {
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
			yield(StampedOpusFrame{}, err)
		}
	}
}

func (c *PipeClientConn) Commands() iter.Seq2[*CommandEvent, error] {
	return func(yield func(*CommandEvent, error) bool) {
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
