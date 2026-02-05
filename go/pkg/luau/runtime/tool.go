package runtime

import (
	"errors"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

var (
	// ErrNoOutput is returned when GetOutput is called but output was never set.
	ErrNoOutput = errors.New("tool did not call output")
)

// ToolContext provides one-shot I/O for tool scripts.
// Tools use input() to get input and output() to return results.
type ToolContext struct {
	mu        sync.Mutex
	input     any
	output    any
	outputErr error
	outputSet bool
}

// NewToolContext creates a new ToolContext.
func NewToolContext() *ToolContext {
	return &ToolContext{}
}

// Type returns the context type.
func (tc *ToolContext) Type() ContextType {
	return ContextTypeTool
}

// RegisterFunctions registers input and output functions to the rt table.
func (tc *ToolContext) RegisterFunctions(state *luau.State) {
	// rt:input() -> value
	state.RegisterFunc("__rt_input", func(s *luau.State) int {
		tc.mu.Lock()
		input := tc.input
		tc.mu.Unlock()

		if input == nil {
			s.PushNil()
			return 1
		}
		goToLua(s, input)
		return 1
	})
	state.GetGlobal("__rt_input")
	state.SetField(-2, "input")
	state.PushNil()
	state.SetGlobal("__rt_input")

	// rt:output(result, err)
	state.RegisterFunc("__rt_output", func(s *luau.State) int {
		tc.mu.Lock()
		defer tc.mu.Unlock()

		// Get result (arg 2, since arg 1 is self in method call)
		if !s.IsNil(2) {
			tc.output = luaToGo(s, 2)
		}

		// Get error (arg 3)
		if !s.IsNil(3) {
			tc.outputErr = errors.New(s.ToString(3))
		}

		tc.outputSet = true
		return 0
	})
	state.GetGlobal("__rt_output")
	state.SetField(-2, "output")
	state.PushNil()
	state.SetGlobal("__rt_output")
}

// Close releases resources (no-op for ToolContext).
func (tc *ToolContext) Close() error {
	return nil
}

// --- External API for callers ---

// SetInput sets the input value for the tool.
func (tc *ToolContext) SetInput(input any) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.input = input
}

// GetOutput returns the output value and error.
// Returns ErrNoOutput if output() was never called.
func (tc *ToolContext) GetOutput() (any, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if !tc.outputSet {
		return nil, ErrNoOutput
	}
	return tc.output, tc.outputErr
}

// HasOutput returns true if output() was called.
func (tc *ToolContext) HasOutput() bool {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.outputSet
}

// Reset resets the tool context for reuse.
func (tc *ToolContext) Reset() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.input = nil
	tc.output = nil
	tc.outputErr = nil
	tc.outputSet = false
}

// Ensure ToolContext implements Context.
var _ Context = (*ToolContext)(nil)
