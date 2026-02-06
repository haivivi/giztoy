package runtime

import (
	"github.com/haivivi/giztoy/go/pkg/luau"
)

// ContextType represents the type of runtime context.
type ContextType string

const (
	// ContextTypeAgent is for streaming agent scripts (recv/emit).
	ContextTypeAgent ContextType = "agent"
	// ContextTypeTool is for one-shot tool scripts (input/output).
	ContextTypeTool ContextType = "tool"
)

// Context is the base interface for runtime contexts.
// A context provides additional methods beyond the base runtime builtins.
type Context interface {
	// Type returns the context type ("agent" or "tool").
	Type() ContextType

	// RegisterFunctions registers context-specific Luau functions to the rt table.
	// The rt table must already exist on the stack at index -1.
	RegisterFunctions(state *luau.State)

	// Close releases any resources held by the context.
	Close() error
}
