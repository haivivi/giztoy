// Package main provides a Luau script runner for testing Haivivi SDK.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/haivivi/giztoy/pkg/luau"
)

func main() {
	libsDir := flag.String("libs", "", "libs directory path")
	asyncMode := flag.Bool("async", false, "enable async HTTP mode (experimental)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: luau-runner [--libs=<path>] [--async] <script.luau>")
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
		state:         state,
		libsDir:       resolvedLibsDir,
		kvs:           make(map[string]any),
		loaded:        make(map[string]bool),
		bytecodeCache: make(map[string][]byte),
	}

	// Register builtins
	if err := rt.RegisterBuiltins(); err != nil {
		fmt.Fprintln(os.Stderr, "failed to register builtins:", err)
		os.Exit(1)
	}

	// Pre-compile all modules in libs directory
	if err := rt.PrecompileModules(); err != nil {
		fmt.Fprintln(os.Stderr, "failed to precompile modules:", err)
		os.Exit(1)
	}

	// Read script
	source, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to read script:", err)
		os.Exit(1)
	}

	if *asyncMode {
		// Async mode: run script in a thread with event loop
		if err := rt.RunAsync(string(source), scriptPath); err != nil {
			fmt.Fprintln(os.Stderr, "script error:", err)
			os.Exit(1)
		}
	} else {
		// Sync mode: execute script directly (blocking HTTP)
		if err := state.DoStringOpt(string(source), scriptPath, luau.OptO2); err != nil {
			fmt.Fprintln(os.Stderr, "script error:", err)
			os.Exit(1)
		}
	}
}

// RunAsync runs a script in a thread with an event loop for async HTTP.
func (rt *Runtime) RunAsync(source, chunkname string) error {
	// Compile script to bytecode
	bytecode, err := rt.state.Compile(source, luau.OptO2)
	if err != nil {
		return fmt.Errorf("compile error: %w", err)
	}

	// Create a new thread to run the script
	thread, err := rt.state.NewThread()
	if err != nil {
		return fmt.Errorf("failed to create thread: %w", err)
	}
	defer thread.Close()

	// Load bytecode onto thread's stack
	if err := thread.LoadBytecode(bytecode, chunkname); err != nil {
		return fmt.Errorf("failed to load bytecode: %w", err)
	}

	// Set current thread for async HTTP
	rt.currentThread = thread

	// Initial resume (start the script)
	status, _ := thread.Resume(0)

	// Event loop: poll for completed HTTP requests
	for status == luau.CoStatusYield || rt.HasPendingRequests() {
		if rt.HasPendingRequests() {
			// Wait a bit for HTTP requests to complete
			time.Sleep(1 * time.Millisecond)

			// Poll for completed requests (this will resume threads)
			rt.PollCompletedHTTP()
		}

		// Check thread status
		status = thread.Status()
		if status == luau.CoStatusOK {
			break
		}
		if status != luau.CoStatusYield {
			// Error
			errMsg := thread.ToString(-1)
			return fmt.Errorf("runtime error: %s", errMsg)
		}
	}

	rt.currentThread = nil

	// Check final status
	if status != luau.CoStatusOK {
		errMsg := thread.ToString(-1)
		return fmt.Errorf("runtime error: %s", errMsg)
	}

	return nil
}

