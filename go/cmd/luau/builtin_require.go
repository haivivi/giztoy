package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// builtinRequire implements require(name) -> module
// Uses pre-compiled bytecode when available for faster loading.
func (rt *Runtime) builtinRequire(state *luau.State) int {
	name := state.ToString(1)
	if name == "" {
		state.PushNil()
		return 1
	}

	// Validate module name to prevent path traversal
	if err := validateModuleName(name); err != nil {
		fmt.Printf("[luau] require error: %v\n", err)
		state.PushNil()
		return 1
	}

	// Check cache for already-loaded modules
	state.GetGlobal("__loaded")
	state.GetField(-1, name)
	if !state.IsNil(-1) {
		state.Remove(-2) // Remove __loaded
		return 1         // Return cached module
	}
	state.Pop(2) // Remove nil and __loaded

	// Mark as loading to prevent circular requires
	if rt.loaded[name] {
		fmt.Printf("[luau] warning: circular require detected for module %q\n", name)
		state.PushNil()
		return 1
	}
	rt.loaded[name] = true

	// Get stack top before execution
	topBefore := state.GetTop()

	// Try to use pre-compiled bytecode first
	if bytecode := rt.GetBytecode(name); bytecode != nil {
		// Load from bytecode cache
		if err := state.LoadBytecode(bytecode, name); err != nil {
			fmt.Printf("[luau] require: failed to load bytecode for %q: %v\n", name, err)
			state.PushNil()
			return 1
		}
		// Execute the loaded chunk
		if err := state.PCall(0, 1); err != nil {
			fmt.Printf("[luau] require: failed to execute %q: %v\n", name, err)
			state.PushNil()
			return 1
		}
	} else {
		// Fallback: find and compile module file at runtime
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
			fmt.Printf("[luau] require: module %q not found\n", name)
			state.PushNil()
			return 1
		}

		// Read module source
		source, err := os.ReadFile(modulePath)
		if err != nil {
			fmt.Printf("[luau] require: failed to read %q: %v\n", modulePath, err)
			state.PushNil()
			return 1
		}

		// Compile and execute (fallback path)
		if err := state.DoStringOpt(string(source), modulePath, luau.OptO2); err != nil {
			fmt.Printf("[luau] require: failed to execute %q: %v\n", modulePath, err)
			state.PushNil()
			return 1
		}
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

// validateModuleName checks if the module name is safe to prevent path traversal.
func validateModuleName(name string) error {
	// Reject empty names
	if name == "" {
		return fmt.Errorf("module name cannot be empty")
	}

	// Reject path traversal sequences
	if strings.Contains(name, "..") {
		return fmt.Errorf("module name %q contains path traversal sequence", name)
	}

	// Reject absolute paths
	if filepath.IsAbs(name) {
		return fmt.Errorf("module name %q is an absolute path", name)
	}

	// Reject names starting with / or \
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") {
		return fmt.Errorf("module name %q starts with path separator", name)
	}

	return nil
}
