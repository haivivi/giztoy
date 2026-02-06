package runtime

import (
	"errors"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

var (
	// ErrAgentClosed is returned when operating on a closed agent.
	ErrAgentClosed = errors.New("agent is closed")
)

// AgentContext provides streaming I/O for agent scripts.
// Agents use recv() to receive input and emit() to send output.
type AgentContext struct {
	mu          sync.Mutex
	inputCh     chan *MessageChunk
	outputCh    chan *MessageChunk
	closeCh     chan struct{}
	closed      bool
	inputClosed bool

	inputBufSize  int
	outputBufSize int
}

// AgentContextConfig holds configuration for AgentContext.
type AgentContextConfig struct {
	InputBufferSize  int // Default: 1
	OutputBufferSize int // Default: 16
}

// NewAgentContext creates a new AgentContext.
func NewAgentContext(cfg *AgentContextConfig) *AgentContext {
	inputBuf, outputBuf := 1, 16
	if cfg != nil {
		if cfg.InputBufferSize > 0 {
			inputBuf = cfg.InputBufferSize
		}
		if cfg.OutputBufferSize > 0 {
			outputBuf = cfg.OutputBufferSize
		}
	}

	return &AgentContext{
		inputCh:       make(chan *MessageChunk, inputBuf),
		outputCh:      make(chan *MessageChunk, outputBuf),
		closeCh:       make(chan struct{}),
		inputBufSize:  inputBuf,
		outputBufSize: outputBuf,
	}
}

// Type returns the context type.
func (ac *AgentContext) Type() ContextType {
	return ContextTypeAgent
}

// RegisterFunctions registers recv and emit functions to the rt table.
func (ac *AgentContext) RegisterFunctions(state *luau.State) {
	// rt:recv() -> chunk, err
	state.RegisterFunc("__rt_recv", func(s *luau.State) int {
		chunk, err := ac.recv()
		if err != nil {
			s.PushNil()
			s.PushString(err.Error())
			return 2
		}
		pushMessageChunk(s, chunk)
		s.PushNil()
		return 2
	})
	state.GetGlobal("__rt_recv")
	state.SetField(-2, "recv")
	state.PushNil()
	state.SetGlobal("__rt_recv")

	// rt:emit(chunk) -> err
	state.RegisterFunc("__rt_emit", func(s *luau.State) int {
		if !s.IsTable(2) {
			s.PushString("emit: expected table")
			return 1
		}
		chunk := parseMessageChunk(s, 2)
		if err := ac.emit(chunk); err != nil {
			s.PushString(err.Error())
			return 1
		}
		s.PushNil()
		return 1
	})
	state.GetGlobal("__rt_emit")
	state.SetField(-2, "emit")
	state.PushNil()
	state.SetGlobal("__rt_emit")
}

// recv waits for the next input chunk.
func (ac *AgentContext) recv() (*MessageChunk, error) {
	select {
	case chunk, ok := <-ac.inputCh:
		if !ok {
			return nil, ErrAgentClosed
		}
		return chunk, nil
	case <-ac.closeCh:
		return nil, ErrAgentClosed
	}
}

// emit sends an output chunk.
func (ac *AgentContext) emit(chunk *MessageChunk) error {
	ac.mu.Lock()
	if ac.closed {
		ac.mu.Unlock()
		return ErrAgentClosed
	}
	ac.mu.Unlock()

	select {
	case ac.outputCh <- chunk:
		return nil
	case <-ac.closeCh:
		return ErrAgentClosed
	}
}

// --- External API for callers ---

// Send sends an input chunk to the agent.
func (ac *AgentContext) Send(chunk *MessageChunk) error {
	ac.mu.Lock()
	if ac.closed {
		ac.mu.Unlock()
		return ErrAgentClosed
	}
	ac.mu.Unlock()

	select {
	case ac.inputCh <- chunk:
		return nil
	case <-ac.closeCh:
		return ErrAgentClosed
	}
}

// SendText sends a text chunk to the agent.
func (ac *AgentContext) SendText(text string) error {
	return ac.Send(&MessageChunk{Part: text})
}

// Output returns the output channel for reading agent output.
func (ac *AgentContext) Output() <-chan *MessageChunk {
	return ac.outputCh
}

// Next returns the next output chunk from the agent.
// Returns nil, false when the agent is closed.
func (ac *AgentContext) Next() (*MessageChunk, bool) {
	chunk, ok := <-ac.outputCh
	return chunk, ok
}

// CloseInput closes the input channel, signaling no more input.
func (ac *AgentContext) CloseInput() {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if ac.inputClosed {
		return
	}
	ac.inputClosed = true
	close(ac.inputCh)
}

// Close closes the agent context.
func (ac *AgentContext) Close() error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if ac.closed {
		return nil
	}
	ac.closed = true
	close(ac.closeCh)
	close(ac.outputCh)
	return nil
}

// IsClosed returns true if the agent is closed.
func (ac *AgentContext) IsClosed() bool {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	return ac.closed
}

// Ensure AgentContext implements Context.
var _ Context = (*AgentContext)(nil)
