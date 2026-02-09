package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/genx/generators"
	"github.com/haivivi/giztoy/go/pkg/genx/modelloader"
	"github.com/haivivi/giztoy/go/pkg/genx/transformers"
	"github.com/haivivi/giztoy/go/pkg/luau"
	"github.com/haivivi/giztoy/go/pkg/luau/runtime"
)

var luauCmd = &cobra.Command{
	Use:   "luau",
	Short: "Luau script runner",
	Long:  "Run Luau scripts with Haivivi SDK support.",
}

var luauRunCmd = &cobra.Command{
	Use:   "run <script.luau>",
	Short: "Run a Luau script",
	Long: `Run a Luau script with configurable runtime mode.

Runtime modes:
  minimal  - Basic runtime with builtins (default)
  tool     - Tool runtime with input/output via stdin/stdout
  agent    - Agent runtime with streaming I/O via rt:recv() and rt:emit()

Examples:
  giztoy luau run script.luau
  giztoy luau run --libs ./libs --models ./models script.luau
  giztoy luau run --runtime tool script.luau < input.json
  giztoy luau run --runtime agent --async script.luau`,
	Args: cobra.ExactArgs(1),
	RunE: runLuauScript,
}

var (
	luauLibsDir     string
	luauModelsDir   string
	luauRuntimeMode string
	luauAsyncMode   bool
	luauConfigPath  string
)

func init() {
	luauRunCmd.Flags().StringVar(&luauLibsDir, "libs", "", "libs directory path")
	luauRunCmd.Flags().StringVar(&luauModelsDir, "models", "", "models config directory (loads generators and transformers)")
	luauRunCmd.Flags().StringVar(&luauRuntimeMode, "runtime", "minimal", "runtime mode: minimal, tool, agent")
	luauRunCmd.Flags().BoolVar(&luauAsyncMode, "async", false, "enable async HTTP mode (experimental)")
	luauRunCmd.Flags().StringVar(&luauConfigPath, "config", "", "config file path (for tool/agent runtime)")

	luauCmd.AddCommand(luauRunCmd)
	rootCmd.AddCommand(luauCmd)
}

func runLuauScript(cmd *cobra.Command, args []string) error {
	scriptPath := args[0]

	// Load models from config directory if specified
	if luauModelsDir != "" {
		modelloader.Verbose = verbose
		loadedNames, err := modelloader.LoadFromDir(luauModelsDir)
		if err != nil {
			return fmt.Errorf("failed to load models: %w", err)
		}
		if verbose && len(loadedNames) > 0 {
			fmt.Fprintf(os.Stderr, "loaded %d models/voices: %v\n", len(loadedNames), loadedNames)
		}
	}

	// Resolve libs directory
	libsDir := luauLibsDir
	if libsDir == "" {
		// Fallback: relative to script
		libsDir = filepath.Join(filepath.Dir(scriptPath), "..", "libs")
	}

	// Read script
	source, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to read script: %w", err)
	}

	// Run based on runtime mode
	switch luauRuntimeMode {
	case "minimal":
		return luauRunMinimal(string(source), scriptPath, libsDir)
	case "tool":
		return luauRunTool(string(source), scriptPath, libsDir)
	case "agent":
		return luauRunAgent(string(source), scriptPath, libsDir)
	default:
		return fmt.Errorf("unknown runtime mode: %s", luauRuntimeMode)
	}
}

// luauRunMinimal runs a script using the minimal runtime.
func luauRunMinimal(source, scriptPath, libsDir string) error {
	state, err := luau.New()
	if err != nil {
		return fmt.Errorf("failed to create luau state: %w", err)
	}
	defer state.Close()

	state.OpenLibs()

	rt := runtime.New(state, &runtime.Options{
		LibsDir: libsDir,
	})

	if err := rt.RegisterBuiltins(); err != nil {
		return fmt.Errorf("failed to register builtins: %w", err)
	}

	if err := rt.PrecompileModules(); err != nil {
		return fmt.Errorf("failed to precompile modules: %w", err)
	}

	if luauAsyncMode {
		return rt.RunAsync(source, scriptPath)
	}

	return rt.Run(source, scriptPath)
}

// luauRunTool runs a script using the tool runtime.
func luauRunTool(source, scriptPath, libsDir string) error {
	state, err := luau.New()
	if err != nil {
		return fmt.Errorf("failed to create luau state: %w", err)
	}
	defer state.Close()

	state.OpenLibs()

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

	if err := rt.RegisterAll(); err != nil {
		return fmt.Errorf("failed to register builtins: %w", err)
	}

	if err := rt.PrecompileModules(); err != nil {
		return fmt.Errorf("failed to precompile modules: %w", err)
	}

	if err := rt.Run(source, scriptPath); err != nil {
		return err
	}

	if tc.HasOutput() {
		output, err := tc.GetOutput()
		if err != nil {
			return err
		}
		if output != nil {
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

// luauRunAgent runs a script using the agent runtime.
func luauRunAgent(source, scriptPath, libsDir string) error {
	state, err := luau.New()
	if err != nil {
		return fmt.Errorf("failed to create luau state: %w", err)
	}
	defer state.Close()

	state.OpenLibs()

	rt := runtime.NewWithOptions(state,
		runtime.WithLibsDir(libsDir),
		runtime.WithGenxGenerator(generators.DefaultMux),
		runtime.WithGenxTransformer(transformers.DefaultMux),
	)
	ac := rt.CreateAgentContext(&runtime.AgentContextConfig{
		InputBufferSize:  10,
		OutputBufferSize: 10,
	})

	if err := rt.RegisterAll(); err != nil {
		return fmt.Errorf("failed to register builtins: %w", err)
	}

	if err := rt.PrecompileModules(); err != nil {
		return fmt.Errorf("failed to precompile modules: %w", err)
	}

	var wg sync.WaitGroup
	var runErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer ac.Close()
		runErr = rt.Run(source, scriptPath)
	}()

	ac.SendText("")
	ac.CloseInput()

	var textParts []string
	for {
		chunk, ok := ac.Next()
		if !ok || chunk == nil {
			break
		}
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

	wg.Wait()

	if len(textParts) > 0 {
		fmt.Print(strings.Join(textParts, ""))
	}

	return runErr
}
