// Package main provides a Luau script runner for testing Haivivi SDK.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/haivivi/giztoy/pkg/luau"
)

func main() {
	libsDir := flag.String("libs", "", "libs directory path")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: luau-runner [--libs=<path>] <script.luau>")
		os.Exit(1)
	}

	scriptPath := flag.Arg(0)

	// Create Luau state
	state, err := luau.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to create luau state:", err)
		os.Exit(1)
	}
	defer state.Close()

	state.OpenLibs()

	// Resolve libs directory
	resolvedLibsDir := *libsDir
	if resolvedLibsDir == "" {
		// Fallback: relative to script
		resolvedLibsDir = filepath.Join(filepath.Dir(scriptPath), "..", "libs")
	}

	// Initialize runtime
	rt := &Runtime{
		state:   state,
		libsDir: resolvedLibsDir,
		kvs:     make(map[string]any),
		loaded:  make(map[string]bool),
	}

	// Register builtins
	if err := rt.RegisterBuiltins(); err != nil {
		fmt.Fprintln(os.Stderr, "failed to register builtins:", err)
		os.Exit(1)
	}

	// Read and execute script
	source, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to read script:", err)
		os.Exit(1)
	}

	if err := state.DoStringOpt(string(source), scriptPath, luau.OptO2); err != nil {
		fmt.Fprintln(os.Stderr, "script error:", err)
		os.Exit(1)
	}
}
