package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/genx/modelloader"
	"github.com/haivivi/giztoy/go/pkg/genx/segmentors"

	"github.com/goccy/go-yaml"
)

var (
	segModel     string
	segSchemaFile string
	segModelsDir string
)

var segmentorCmd = &cobra.Command{
	Use:   "segmentor",
	Short: "Conversation segmentor (compress + entity extraction)",
	Long: `Compress conversation messages into a structured segment with
entity and relation extraction using LLM.

The segmentor reads conversation lines from stdin or a file and produces
a JSON result with:
  - A segment summary with keywords and entity labels
  - Extracted entities (people, topics, places, objects) with attributes
  - Discovered relations between entities

Requires a generator to be registered (via --models-dir or config).

Examples:
  # From stdin
  echo -e "user: 小明今天聊了恐龙\nassistant: 他最喜欢霸王龙" | giztoy segmentor run --model qwen/turbo

  # From file with schema
  giztoy segmentor run --model qwen/turbo --schema schema.yaml < conversation.txt

  # With custom models directory
  giztoy segmentor run --model qwen/turbo --models-dir ./models < conversation.txt`,
}

var segRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the segmentor on conversation input",
	Long: `Read conversation lines from stdin and run the segmentor.

Each line of input is treated as one conversation message.
The segmentor compresses all messages into a single segment
and extracts entities and relations.

Output is formatted JSON written to stdout.`,
	RunE: runSegmentor,
}

func runSegmentor(cmd *cobra.Command, args []string) error {
	if segModel == "" {
		return fmt.Errorf("--model is required (e.g., qwen/turbo)")
	}

	// Load models if models-dir is specified.
	if segModelsDir != "" {
		modelloader.Verbose = IsVerbose()
		names, err := modelloader.LoadFromDir(segModelsDir)
		if err != nil {
			return fmt.Errorf("load models: %w", err)
		}
		if IsVerbose() {
			fmt.Fprintf(os.Stderr, "Loaded models: %s\n", strings.Join(names, ", "))
		}
	}

	// Register a segmentor for the given model pattern.
	seg := segmentors.NewGenX(segmentors.Config{Generator: segModel})

	// Read messages from stdin.
	messages, err := readLines(os.Stdin)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	if len(messages) == 0 {
		return fmt.Errorf("no input messages (pipe conversation text to stdin)")
	}

	// Load optional schema.
	input := segmentors.Input{Messages: messages}
	if segSchemaFile != "" {
		schema, err := loadSchema(segSchemaFile)
		if err != nil {
			return fmt.Errorf("load schema: %w", err)
		}
		input.Schema = schema
	}

	// Run the segmentor.
	result, err := seg.Process(cmd.Context(), input)
	if err != nil {
		return fmt.Errorf("segmentor: %w", err)
	}

	// Output JSON.
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func readLines(f *os.File) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

func loadSchema(path string) (*segmentors.Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var schema segmentors.Schema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("parse schema YAML: %w", err)
	}
	return &schema, nil
}

func init() {
	segRunCmd.Flags().StringVar(&segModel, "model", "", "generator pattern (e.g., qwen/turbo)")
	segRunCmd.Flags().StringVar(&segSchemaFile, "schema", "", "path to entity schema YAML file")
	segRunCmd.Flags().StringVar(&segModelsDir, "models-dir", "", "directory with model config files")

	segmentorCmd.AddCommand(segRunCmd)
	rootCmd.AddCommand(segmentorCmd)
}
