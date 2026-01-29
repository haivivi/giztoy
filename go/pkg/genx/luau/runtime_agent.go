package luau

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/agent"
)

// ErrAgentClosed is returned when Recv is called on a closed agent.
var ErrAgentClosed = errors.New("agent is closed")

// AgentRuntime extends ToolRuntime with Agent-specific I/O capabilities.
// It provides Recv/Emit for streaming conversation with external callers.
type AgentRuntime interface {
	ToolRuntime

	// Recv waits for and returns the next input from the caller.
	// Returns nil, ErrAgentClosed when the agent is closed.
	// This is an async operation that should yield in Luau.
	Recv() (genx.Contents, error)

	// Emit sends an output chunk to the caller.
	// Set chunk.EOF = true to indicate end of current response.
	Emit(chunk *genx.MessageChunk) error

	// Close closes the agent runtime, signaling no more input.
	Close() error
}

// agentRuntime implements AgentRuntime.
type agentRuntime struct {
	*toolRuntime

	mu       sync.Mutex
	inputCh  chan genx.Contents      // Input from caller
	outputCh chan *genx.MessageChunk // Output to caller
	closed   bool
	closeCh  chan struct{}
}

// AgentRuntimeConfig holds configuration for creating an AgentRuntime.
type AgentRuntimeConfig struct {
	AgentName    string
	AgentModel   string
	Logger       Logger
	InputBuffer  int // Buffer size for input channel (default: 1)
	OutputBuffer int // Buffer size for output channel (default: 16)
}

// NewAgentRuntime creates a new AgentRuntime.
func NewAgentRuntime(ctx context.Context, rt agent.Runtime, state agent.AgentState, cfg *AgentRuntimeConfig) AgentRuntime {
	var logger Logger
	var agentName, agentModel string
	inputBuf, outputBuf := 1, 16

	if cfg != nil {
		logger = cfg.Logger
		agentName = cfg.AgentName
		agentModel = cfg.AgentModel
		if cfg.InputBuffer > 0 {
			inputBuf = cfg.InputBuffer
		}
		if cfg.OutputBuffer > 0 {
			outputBuf = cfg.OutputBuffer
		}
	}

	if logger == nil {
		logger = &defaultLogger{}
	}

	tr := &toolRuntime{
		ctx:        ctx,
		runtime:    rt,
		state:      state,
		logger:     logger,
		agentName:  agentName,
		agentModel: agentModel,
	}

	return &agentRuntime{
		toolRuntime: tr,
		inputCh:     make(chan genx.Contents, inputBuf),
		outputCh:    make(chan *genx.MessageChunk, outputBuf),
		closeCh:     make(chan struct{}),
	}
}

// InputChannel returns the input channel for sending input to the agent.
// This is used by the caller to send messages to the agent.
func (ar *agentRuntime) InputChannel() chan<- genx.Contents {
	return ar.inputCh
}

// Input implements ToolRuntime.Input for AgentRuntime.
// For AgentRuntime, this returns the tool input (not agent input).
func (ar *agentRuntime) Input() (any, error) {
	return ar.toolRuntime.Input()
}

// Output implements ToolRuntime.Output for AgentRuntime.
func (ar *agentRuntime) Output(result any, err error) {
	ar.toolRuntime.Output(result, err)
}

// OutputChannel returns the output channel for receiving output from the agent.
// This is used by the caller to receive messages from the agent.
func (ar *agentRuntime) OutputChannel() <-chan *genx.MessageChunk {
	return ar.outputCh
}

// Recv waits for the next input from the caller.
func (ar *agentRuntime) Recv() (genx.Contents, error) {
	select {
	case input, ok := <-ar.inputCh:
		if !ok {
			return nil, ErrAgentClosed
		}
		return input, nil
	case <-ar.closeCh:
		return nil, ErrAgentClosed
	case <-ar.ctx.Done():
		return nil, ar.ctx.Err()
	}
}

// Emit sends an output chunk to the caller.
func (ar *agentRuntime) Emit(chunk *genx.MessageChunk) error {
	ar.mu.Lock()
	if ar.closed {
		ar.mu.Unlock()
		return ErrAgentClosed
	}
	ar.mu.Unlock()

	select {
	case ar.outputCh <- chunk:
		return nil
	case <-ar.closeCh:
		return ErrAgentClosed
	case <-ar.ctx.Done():
		return ar.ctx.Err()
	}
}

// Close closes the agent runtime.
func (ar *agentRuntime) Close() error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	if ar.closed {
		return nil
	}
	ar.closed = true
	close(ar.closeCh)
	close(ar.outputCh)
	return nil
}

// SendInput sends input to the agent (for caller use).
func (ar *agentRuntime) SendInput(contents genx.Contents) error {
	ar.mu.Lock()
	if ar.closed {
		ar.mu.Unlock()
		return ErrAgentClosed
	}
	ar.mu.Unlock()

	select {
	case ar.inputCh <- contents:
		return nil
	case <-ar.closeCh:
		return ErrAgentClosed
	case <-ar.ctx.Done():
		return ar.ctx.Err()
	}
}

// CloseInput closes the input channel (signals no more input).
// This method is idempotent - calling it multiple times is safe.
func (ar *agentRuntime) CloseInput() {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	// Check if already fully closed
	select {
	case <-ar.closeCh:
		return
	default:
	}

	// Safely close inputCh (recover from panic if already closed)
	defer func() {
		_ = recover()
	}()
	close(ar.inputCh)
}

// --- Convenience methods for the caller ---

// AgentHandle provides a handle for external callers to interact with an AgentRuntime.
type AgentHandle struct {
	ar *agentRuntime
}

// NewAgentHandle creates a handle from an AgentRuntime.
// This is used by callers to interact with the agent.
// Panics if ar is not the expected *agentRuntime implementation.
func NewAgentHandle(ar AgentRuntime) *AgentHandle {
	impl, ok := ar.(*agentRuntime)
	if !ok {
		panic("luau: NewAgentHandle expects *agentRuntime implementation of AgentRuntime")
	}
	return &AgentHandle{ar: impl}
}

// Send sends input to the agent.
func (h *AgentHandle) Send(contents genx.Contents) error {
	return h.ar.SendInput(contents)
}

// SendText sends a text message to the agent.
func (h *AgentHandle) SendText(text string) error {
	return h.ar.SendInput(genx.Contents{genx.Text(text)})
}

// Next returns the next output chunk from the agent.
// Returns nil, false when the agent is done.
func (h *AgentHandle) Next() (*genx.MessageChunk, bool) {
	chunk, ok := <-h.ar.outputCh
	return chunk, ok
}

// Collect collects all output from the agent until EOF or close.
func (h *AgentHandle) Collect() ([]genx.MessageChunk, error) {
	var chunks []genx.MessageChunk
	for chunk := range h.ar.outputCh {
		chunks = append(chunks, *chunk)
	}
	return chunks, nil
}

// CollectText collects all text output from the agent.
func (h *AgentHandle) CollectText() (string, error) {
	var sb strings.Builder
	for chunk := range h.ar.outputCh {
		if chunk.Part != nil {
			if text, ok := chunk.Part.(genx.Text); ok {
				sb.WriteString(string(text))
			}
		}
	}
	return sb.String(), nil
}

// CloseInput signals that no more input will be sent.
func (h *AgentHandle) CloseInput() {
	h.ar.CloseInput()
}

// Ensure agentRuntime satisfies ToolRuntime interface requirements.
// Methods from embedded *toolRuntime are automatically promoted.
var _ ToolRuntime = (*agentRuntime)(nil)
