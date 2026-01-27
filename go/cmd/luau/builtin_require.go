package main

import (
	"os"
	"path/filepath"

	"github.com/haivivi/giztoy/pkg/luau"
)

// builtinRequire implements require(name) -> module
func (rt *Runtime) builtinRequire(state *luau.State) int {
	name := state.ToString(1)
	if name == "" {
		state.PushNil()
		return 1
	}

	// Check cache
	state.GetGlobal("__loaded")
	state.GetField(-1, name)
	if !state.IsNil(-1) {
		state.Remove(-2) // Remove __loaded
		return 1         // Return cached module
	}
	state.Pop(2) // Remove nil and __loaded

	// Find module file
	// 1. libs/<name>.luau
	// 2. libs/<name>/init.luau
	var modulePath string
	candidates := []string{
		filepath.Join(rt.libsDir, name+".luau"),
		filepath.Join(rt.libsDir, name, "init.luau"),
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			modulePath = p
			break
		}
	}

	if modulePath == "" {
		state.PushNil()
		return 1
	}

	// Mark as loading to prevent circular requires
	if rt.loaded[name] {
		state.PushNil()
		return 1
	}
	rt.loaded[name] = true

	// Read module source
	source, err := os.ReadFile(modulePath)
	if err != nil {
		state.PushNil()
		return 1
	}

	// Get stack top before execution
	topBefore := state.GetTop()

	// Execute module (expect it to return a value)
	if err := state.DoStringOpt(string(source), modulePath, luau.OptO2); err != nil {
		state.PushNil()
		return 1
	}

	// Check if module returned a value
	topAfter := state.GetTop()
	if topAfter <= topBefore {
		// Module didn't return anything, push nil
		state.PushNil()
	}

	// Cache the result
	state.PushValue(-1)       // Copy return value
	state.GetGlobal("__loaded")
	state.PushValue(-2)       // Copy again
	state.SetField(-2, name)  // __loaded[name] = module
	state.Pop(2)              // Remove __loaded and copy

	return 1
}
