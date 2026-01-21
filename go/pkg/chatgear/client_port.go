package chatgear

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/haivivi/giztoy/pkg/audio/opusrt"
)

// ClientPort implements ClientPortTx and ClientPortRx as a bidirectional audio port.
// It manages audio input encoding/sending and output buffering/command receiving.
type ClientPort struct {
	tx      UplinkTx
	context context.Context
	cancel  context.CancelFunc
	logger  Logger

	// Output - audio from server (ClientPortRx)
	playBuffer *opusrt.RealtimeBuffer
	commands   chan *SessionCommandEvent
	commandsMu sync.Mutex // protects commands channel and closed flag
	closed     bool

	// Input - audio to server (ClientPortTx)
	recBuffer    *opusrt.RealtimeBuffer
	inputStamp   time.Time
	inputStampMu sync.Mutex
}

// NewClientPort creates a new ClientPort for the given UplinkTx.
func NewClientPort(ctx context.Context, tx UplinkTx) *ClientPort {
	ctx, cancel := context.WithCancel(ctx)
	p := &ClientPort{
		tx:      tx,
		context: ctx,
		cancel:  cancel,
		logger:  DefaultLogger(),

		playBuffer: opusrt.RealtimeFrom(opusrt.NewBuffer(2 * time.Minute)),
		commands:   make(chan *SessionCommandEvent, 32),
		recBuffer:  opusrt.RealtimeFrom(opusrt.NewBuffer(2 * time.Minute)),
	}
	go p.streamingInputLoop()
	return p
}

// --- ClientPortTx (sending to server) ---

// SendOpusFrames sends a stamped opus frame to the server.
func (p *ClientPort) SendOpusFrames(stampedOpusFrame []byte) error {
	frame, stamp, ok := opusrt.FromStamped(stampedOpusFrame)
	if !ok {
		return fmt.Errorf("chatgear: invalid stamped opus frame")
	}
	return p.tx.SendOpusFrames(p.context, stamp, frame)
}

// SendState sends a state event to the server.
func (p *ClientPort) SendState(state *GearStateEvent) error {
	return p.tx.SendState(p.context, state)
}

// SendStats sends a stats event to the server.
func (p *ClientPort) SendStats(stats *GearStatsEvent) error {
	return p.tx.SendStats(p.context, stats)
}

// --- ClientPortRx (receiving from server) ---

// Frame reads the next Opus frame from the server.
// Implements opusrt.FrameReader.
func (p *ClientPort) Frame() (opusrt.Frame, time.Duration, error) {
	return p.playBuffer.Frame()
}

// Commands returns a channel that receives commands from the server.
func (p *ClientPort) Commands() <-chan *SessionCommandEvent {
	return p.commands
}

// --- Handle (input from transport) ---

// HandleOpusFrames handles incoming Opus frames from the server.
func (p *ClientPort) HandleOpusFrames(stampedOpusFrame []byte) {
	if _, err := p.playBuffer.Write(stampedOpusFrame); err != nil {
		p.logger.ErrorPrintf("handle opus frames: %v", err)
	}
}

// HandleCommand handles an incoming command from the server.
func (p *ClientPort) HandleCommand(cmd *SessionCommandEvent) {
	p.commandsMu.Lock()
	defer p.commandsMu.Unlock()

	if p.closed {
		return
	}

	select {
	case p.commands <- cmd:
	default:
		p.logger.WarnPrintf("commands channel is full, drop command")
	}
}

// WriteRecordingFrame writes a stamped opus frame to be sent to the server.
func (p *ClientPort) WriteRecordingFrame(stampedOpusFrame []byte) {
	if _, err := p.recBuffer.Write(stampedOpusFrame); err != nil {
		p.logger.ErrorPrintf("write recording frame: %v", err)
	}
}

// --- Lifecycle ---

// Context returns the port's context.
func (p *ClientPort) Context() context.Context {
	return p.context
}

// Close closes the port.
func (p *ClientPort) Close() error {
	p.cancel()
	p.playBuffer.Close()
	p.recBuffer.Close()

	// Safely close channel under lock to prevent send-to-closed-channel panic
	p.commandsMu.Lock()
	p.closed = true
	close(p.commands)
	p.commandsMu.Unlock()

	return p.tx.Close()
}

// RecvFrom receives data from the given DownlinkRx until closed or error.
// This method blocks; use `go port.RecvFrom(rx)` for non-blocking operation.
// Returns the first error encountered, or nil if all iterators completed normally.
func (p *ClientPort) RecvFrom(rx DownlinkRx) error {
	defer rx.Close()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	setErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		for frame, err := range rx.OpusFrames() {
			if err != nil {
				setErr(err)
				return
			}
			p.HandleOpusFrames(frame)
		}
	}()
	go func() {
		defer wg.Done()
		for cmd, err := range rx.Commands() {
			if err != nil {
				setErr(err)
				return
			}
			p.HandleCommand(cmd)
		}
	}()
	wg.Wait()
	return firstErr
}

func (p *ClientPort) streamingInputLoop() {
	for {
		select {
		case <-p.context.Done():
			return
		default:
		}

		frame, _, err := p.recBuffer.Frame()
		if err != nil {
			p.logger.ErrorPrintf("read frame from rec buffer: %v", err)
			return
		}

		if len(frame) == 0 {
			continue
		}

		p.inputStampMu.Lock()
		now := time.Now()
		if p.inputStamp.Before(now) {
			p.inputStamp = now
		}
		stamp := p.inputStamp
		p.inputStamp = p.inputStamp.Add(frame.Duration())
		p.inputStampMu.Unlock()

		if err := p.tx.SendOpusFrames(p.context, opusrt.FromTime(stamp), frame); err != nil {
			p.logger.ErrorPrintf("send opus frame: %v", err)
			continue
		}
	}
}
