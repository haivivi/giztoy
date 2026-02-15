package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/genx/modelloader"
	"github.com/haivivi/giztoy/go/pkg/genx/profilers"
	"github.com/haivivi/giztoy/go/pkg/genx/segmentors"
)

var (
	profModel      string
	profSchemaFile string
	profModelsDir  string
	profExtracted  string
	profProfiles   string
)

var profilerCmd = &cobra.Command{
	Use:   "profiler",
	Short: "Entity profiler (schema evolution + profile updates)",
	Long: `Analyze conversation metadata to update entity profiles and evolve
the entity schema using LLM.

The profiler takes the output of a segmentor (extracted entities, relations)
along with the original conversation and existing profiles, and produces:
  - Schema changes: new fields or modifications
  - Profile updates: attribute values for entities
  - Additional relations

Requires a generator to be registered (via --models-dir or config).

Examples:
  # Pipeline: segmentor → profiler
  cat conversation.txt | giztoy segmentor run --model qwen/turbo > extracted.json
  cat conversation.txt | giztoy profiler run --model qwen/turbo --extracted extracted.json

  # With existing schema and profiles
  cat conversation.txt | giztoy profiler run --model qwen/turbo \
    --extracted extracted.json --schema schema.yaml --profiles profiles.json`,
}

var profRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the profiler on conversation input",
	Long: `Read conversation lines from stdin and run the profiler.

The profiler requires the segmentor's output (--extracted flag) to know
which entities were found. It then analyzes the conversation to update
entity profiles and propose schema changes.

Output is formatted JSON written to stdout.`,
	RunE: runProfiler,
}

func runProfiler(cmd *cobra.Command, args []string) error {
	if profModel == "" {
		return fmt.Errorf("--model is required (e.g., qwen/turbo)")
	}
	if profExtracted == "" {
		return fmt.Errorf("--extracted is required (path to segmentor output JSON)")
	}

	// Load models if models-dir is specified.
	if profModelsDir != "" {
		modelloader.Verbose = IsVerbose()
		names, err := modelloader.LoadFromDir(profModelsDir)
		if err != nil {
			return fmt.Errorf("load models: %w", err)
		}
		if IsVerbose() {
			fmt.Fprintf(os.Stderr, "Loaded models: %s\n", strings.Join(names, ", "))
		}
	}

	// Create profiler.
	prof := profilers.NewGenX(profilers.Config{Generator: profModel})

	// Read messages from stdin.
	messages, err := readLines(os.Stdin)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	if len(messages) == 0 {
		return fmt.Errorf("no input messages (pipe conversation text to stdin)")
	}

	// Load extracted segmentor output.
	extracted, err := loadExtracted(profExtracted)
	if err != nil {
		return fmt.Errorf("load extracted: %w", err)
	}

	// Build profiler input.
	input := profilers.Input{
		Messages:  messages,
		Extracted: extracted,
	}

	// Load optional schema.
	if profSchemaFile != "" {
		schema, err := loadSchema(profSchemaFile)
		if err != nil {
			return fmt.Errorf("load schema: %w", err)
		}
		input.Schema = schema
	}

	// Load optional existing profiles.
	if profProfiles != "" {
		profiles, err := loadProfiles(profProfiles)
		if err != nil {
			return fmt.Errorf("load profiles: %w", err)
		}
		input.Profiles = profiles
	}

	// Run the profiler.
	result, err := prof.Process(cmd.Context(), input)
	if err != nil {
		return fmt.Errorf("profiler: %w", err)
	}

	// Output JSON.
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func loadExtracted(path string) (*segmentors.Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result segmentors.Result
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse extracted JSON: %w", err)
	}
	return &result, nil
}

func loadProfiles(path string) (map[string]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var profiles map[string]map[string]any
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("parse profiles JSON: %w", err)
	}
	return profiles, nil
}

func init() {
	profRunCmd.Flags().StringVar(&profModel, "model", "", "generator pattern (e.g., qwen/turbo)")
	profRunCmd.Flags().StringVar(&profSchemaFile, "schema", "", "path to entity schema YAML file")
	profRunCmd.Flags().StringVar(&profModelsDir, "models-dir", "", "directory with model config files")
	profRunCmd.Flags().StringVar(&profExtracted, "extracted", "", "path to segmentor output JSON")
	profRunCmd.Flags().StringVar(&profProfiles, "profiles", "", "path to existing profiles JSON")

	profilerCmd.AddCommand(profRunCmd)
	rootCmd.AddCommand(profilerCmd)
}
