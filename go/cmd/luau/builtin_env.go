package main

import (
	"os"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// builtinEnv implements __builtin.env(key) -> value
func (rt *Runtime) builtinEnv(state *luau.State) int {
	key := state.ToString(1)
	if key == "" {
		state.PushNil()
		return 1
	}

	value := os.Getenv(key)
	if value == "" {
		state.PushNil()
		return 1
	}

	state.PushString(value)
	return 1
}
