package agent

import "errors"

var (
	// ErrClosed indicates the Agent is closed.
	ErrClosed = errors.New("agent: closed")

	// ErrInvalidToolCall indicates an invalid tool call.
	ErrInvalidToolCall = errors.New("agent: invalid tool call")
)
