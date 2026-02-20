package cortex

import (
	"context"
	"fmt"
)

// RunHandler executes a task document and returns the result.
type RunHandler func(ctx context.Context, c *Cortex, task Document) (*RunResult, error)

// RunResult holds the output of a run operation.
type RunResult struct {
	Kind   string `json:"kind"`
	Name   string `json:"name,omitempty"`
	Status string `json:"status"` // "ok", "error"

	// Output fields (kind-dependent)
	Text       string `json:"text,omitempty"`
	AudioFile  string `json:"audio_file,omitempty"`
	AudioSize  int    `json:"audio_size,omitempty"`
	TaskID     string `json:"task_id,omitempty"`
	OutputFile string `json:"output_file,omitempty"`

	// Generic data for JSON output
	Data map[string]any `json:"data,omitempty"`
}

var runHandlers = map[string]RunHandler{}

// RegisterRunHandler registers a handler for a run kind.
func RegisterRunHandler(kind string, handler RunHandler) {
	runHandlers[kind] = handler
}

// Run executes a task document by dispatching to the appropriate handler.
func (c *Cortex) Run(ctx context.Context, task Document) (*RunResult, error) {
	handler, ok := runHandlers[task.Kind]
	if !ok {
		return nil, fmt.Errorf("unknown run kind %q; no handler registered", task.Kind)
	}
	return handler(ctx, c, task)
}

// RunKinds returns all registered run handler kinds.
func RunKinds() []string {
	kinds := make([]string, 0, len(runHandlers))
	for k := range runHandlers {
		kinds = append(kinds, k)
	}
	return kinds
}

// ResolveCred reads a cred from KV by its full name (e.g. "openai:qwen").
// Returns the cred document's fields.
func (c *Cortex) ResolveCred(ctx context.Context, credRef string) (map[string]any, error) {
	fullName := "creds:" + credRef
	doc, err := c.Get(ctx, fullName)
	if err != nil {
		return nil, fmt.Errorf("resolve cred %q: %w", credRef, err)
	}
	return doc.Fields, nil
}
