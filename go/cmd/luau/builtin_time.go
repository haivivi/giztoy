package main

import (
	"time"

	"github.com/haivivi/giztoy/pkg/luau"
)

// builtinTime implements __builtin.time() -> number
// Returns the current Unix timestamp in seconds (with millisecond precision).
func (rt *Runtime) builtinTime(state *luau.State) int {
	now := float64(time.Now().UnixMilli()) / 1000.0
	state.PushNumber(now)
	return 1
}

// builtinParseTime implements __builtin.parse_time(iso_string) -> number
// Parses an ISO 8601 date string and returns Unix timestamp in seconds.
// Returns nil if parsing fails.
func (rt *Runtime) builtinParseTime(state *luau.State) int {
	isoStr := state.ToString(1)
	if isoStr == "" {
		state.PushNil()
		return 1
	}

	// Try common ISO 8601 formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		t, err := time.Parse(format, isoStr)
		if err == nil {
			state.PushNumber(float64(t.UnixMilli()) / 1000.0)
			return 1
		}
	}

	state.PushNil()
	return 1
}
