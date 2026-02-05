// Package main provides a Luau script runner for testing Haivivi SDK.
// It supports multiple runtime modes for different testing scenarios.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/genx/generators"
	"github.com/haivivi/giztoy/go/pkg/genx/modelloader"
	"github.com/haivivi/giztoy/go/pkg/genx/transformers"
	"github.com/haivivi/giztoy/go/pkg/luau"
	"github.com/haivivi/giztoy/go/pkg/luau/runtime"
)

func main() {
	// Command-line flags
	dir := flag.String("dir", "", "libs directory path (alias for -libs)")
	libs := flag.String("libs", "", "libs directory path")
	models := flag.String("models", "", "models config directory (loads generators and transformers)")
	runtimeMode := flag.String("runtime", "minimal", "runtime mode: minimal, tool, agent")
	asyncMode := flag.Bool("async", false, "enable async HTTP mode (experimental)")
	config := flag.String("config", "", "config file path (for tool/agent runtime)")
	verbose := flag.Bool("v", false, "verbose mode (show loaded models)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: luau-runner [options] <script.luau>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -dir, -libs <path>   libs directory path")
		fmt.Fprintln(os.Stderr, "  -models <path>       models config directory (loads generators and transformers)")
		fmt.Fprintln(os.Stderr, "  -runtime <mode>      runtime mode: minimal (default), tool, agent")
		fmt.Fprintln(os.Stderr, "  -async               enable async HTTP mode")
		fmt.Fprintln(os.Stderr, "  -config <path>       config file (for tool/agent runtime)")
		fmt.Fprintln(os.Stderr, "  -v                   verbose mode")
		os.Exit(1)
	}

	// Load models from config directory if specified
	if *models != "" {
		modelloader.Verbose = *verbose
		loadedNames, err := modelloader.LoadFromDir(*models)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load models: %v\n", err)
			os.Exit(1)
		}
		if *verbose && len(loadedNames) > 0 {
			fmt.Fprintf(os.Stderr, "loaded %d models/voices: %v\n", len(loadedNames), loadedNames)
		}
	}

	scriptPath := flag.Arg(0)

	// Resolve libs directory (prefer -dir over -libs)
	libsDir := *dir
	if libsDir == "" {
		libsDir = *libs
	}
	if libsDir == "" {
		// Fallback: relative to script
		libsDir = filepath.Join(filepath.Dir(scriptPath), "..", "libs")
	}

	// Read script
	source, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to read script:", err)
		os.Exit(1)
	}

	// Run based on runtime mode
	switch *runtimeMode {
	case "minimal":
		if err := runMinimal(string(source), scriptPath, libsDir, *asyncMode); err != nil {
			fmt.Fprintln(os.Stderr, "script error:", err)
			os.Exit(1)
		}
	case "tool":
		if err := runTool(string(source), scriptPath, libsDir, *config); err != nil {
			fmt.Fprintln(os.Stderr, "script error:", err)
			os.Exit(1)
		}
	case "agent":
		if err := runAgent(string(source), scriptPath, libsDir, *config); err != nil {
			fmt.Fprintln(os.Stderr, "script error:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown runtime mode: %s\n", *runtimeMode)
		os.Exit(1)
	}
}

// runMinimal runs a script using the minimal runtime.
func runMinimal(source, scriptPath, libsDir string, async bool) error {
	// Create Luau state
	state, err := luau.New()
	if err != nil {
		return fmt.Errorf("failed to create luau state: %w", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Initialize runtime
	rt := runtime.New(state, &runtime.Options{
		LibsDir: libsDir,
	})

	// Register builtins
	if err := rt.RegisterBuiltins(); err != nil {
		return fmt.Errorf("failed to register builtins: %w", err)
	}

	// Pre-compile all modules in libs directory
	if err := rt.PrecompileModules(); err != nil {
		return fmt.Errorf("failed to precompile modules: %w", err)
	}

	if async {
		// Async mode: run script in a thread with event loop
		return rt.RunAsync(source, scriptPath)
	}

	// Sync mode: execute script directly (blocking HTTP)
	return rt.Run(source, scriptPath)
}

// runTool runs a script using the tool runtime.
// This provides access to rt:input() and rt:output() for one-shot tool execution.
// Input is read from stdin as JSON.
func runTool(source, scriptPath, libsDir, configPath string) error {
	// Create Luau state
	state, err := luau.New()
	if err != nil {
		return fmt.Errorf("failed to create luau state: %w", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Initialize runtime with tool context, generator, and transformers
	rt := runtime.NewWithOptions(state,
		runtime.WithLibsDir(libsDir),
		runtime.WithGenxGenerator(generators.DefaultMux),
		runtime.WithGenxTransformer(transformers.DefaultMux),
	)
	tc := rt.CreateToolContext()

	// Read stdin as JSON input (if any)
	stdinData, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}
	if len(stdinData) > 0 {
		var input any
		if err := json.Unmarshal(stdinData, &input); err != nil {
			return fmt.Errorf("failed to parse stdin as JSON: %w", err)
		}
		tc.SetInput(input)
	}

	// Register builtins and context functions
	if err := rt.RegisterAll(); err != nil {
		return fmt.Errorf("failed to register builtins: %w", err)
	}

	// Pre-compile all modules in libs directory
	if err := rt.PrecompileModules(); err != nil {
		return fmt.Errorf("failed to precompile modules: %w", err)
	}

	// Run the script
	if err := rt.Run(source, scriptPath); err != nil {
		return err
	}

	// Get output if any
	if tc.HasOutput() {
		output, err := tc.GetOutput()
		if err != nil {
			return err
		}
		if output != nil {
			// Output as JSON for structured data
			jsonOutput, err := json.Marshal(output)
			if err != nil {
				fmt.Printf("%v\n", output)
			} else {
				fmt.Printf("%s\n", jsonOutput)
			}
		}
	}

	return nil
}

// runAgent runs a script using the agent runtime.
// This provides full agent capabilities including I/O streaming via rt:recv() and rt:emit().
//
// In CLI mode, input is read from stdin and output is printed to stdout.
func runAgent(source, scriptPath, libsDir, configPath string) error {
	// Create Luau state
	state, err := luau.New()
	if err != nil {
		return fmt.Errorf("failed to create luau state: %w", err)
	}
	defer state.Close()

	state.OpenLibs()

	// Initialize runtime with agent context, generator, and transformers
	rt := runtime.NewWithOptions(state,
		runtime.WithLibsDir(libsDir),
		runtime.WithGenxGenerator(generators.DefaultMux),
		runtime.WithGenxTransformer(transformers.DefaultMux),
	)
	ac := rt.CreateAgentContext(&runtime.AgentContextConfig{
		InputBufferSize:  10,
		OutputBufferSize: 10,
	})

	// Register builtins and context functions
	if err := rt.RegisterAll(); err != nil {
		return fmt.Errorf("failed to register builtins: %w", err)
	}

	// Pre-compile all modules in libs directory
	if err := rt.PrecompileModules(); err != nil {
		return fmt.Errorf("failed to precompile modules: %w", err)
	}

	// Run script in a goroutine so we can handle I/O streaming
	var wg sync.WaitGroup
	var runErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer ac.Close() // Close output channel when script finishes
		runErr = rt.Run(source, scriptPath)
	}()

	// Send initial empty input to start the script
	// (agent scripts typically call rt:recv() first)
	ac.SendText("")

	// Close input to signal no more input
	ac.CloseInput()

	// Collect output chunks and print text parts
	var textParts []string
	for {
		chunk, ok := ac.Next()
		if !ok || chunk == nil {
			break
		}
		// Extract text from chunk.Part (can be string or struct)
		if chunk.Part != nil {
			switch p := chunk.Part.(type) {
			case string:
				if p != "" {
					textParts = append(textParts, p)
				}
			case map[string]any:
				if t, ok := p["type"].(string); ok && t == "text" {
					if v, ok := p["value"].(string); ok && v != "" {
						textParts = append(textParts, v)
					}
				}
			}
		}
	}

	// Wait for script to finish (should already be done since output channel closed)
	wg.Wait()

	// Print collected text
	if len(textParts) > 0 {
		fmt.Print(strings.Join(textParts, ""))
	}

	if runErr != nil {
		return runErr
	}

	return nil
}
