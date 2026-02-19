package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/cortex"
	"github.com/haivivi/giztoy/go/pkg/kv"
)

var (
	verbose      bool
	formatOutput string
	outputFile   string
)

var rootCmd = &cobra.Command{
	Use:   "giztoy",
	Short: "Unified CLI for giztoy AI services",
	Long: `giztoy — A unified command line interface for AI services.

Commands:
  ctx       Context configuration management (file-based bootstrap)
  apply     Declare and write resources (creds, genx configs)
  list      List resources by prefix
  get       Get a resource by full name
  delete    Delete a resource by full name
  run       Execute a task (TTS, chat, ASR, etc.)
  version   Version information

Resource kinds:
  creds/openai, creds/genai, creds/minimax, creds/doubaospeech, creds/dashscope
  genx/generator, genx/tts, genx/asr, genx/realtime, genx/segmentor, genx/profiler

Examples:
  giztoy ctx add dev && giztoy ctx use dev
  giztoy ctx config set kv badger:///tmp/dev
  giztoy apply -f setup.yaml
  giztoy list creds:*
  giztoy get creds:openai:qwen
  giztoy run -f task.yaml`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVar(&formatOutput, "format", "table", "output format: table, json, yaml, name")
	rootCmd.PersistentFlags().StringVarP(&outputFile, "o", "o", "", "output file path")
}

// testKVOverride is set during tests to share a KV instance across commands.
var testKVOverride kv.Store

// openCortex creates a Cortex instance from the current ctx config.
func openCortex(ctx context.Context) (*cortex.Cortex, error) {
	store, err := openStore()
	if err != nil {
		return nil, err
	}
	var opts []cortex.Option
	if testKVOverride != nil {
		opts = append(opts, cortex.WithKV(testKVOverride))
	}
	return cortex.New(ctx, store, opts...)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printVerbose(format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

func newTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
}
