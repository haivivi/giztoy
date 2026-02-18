package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	verbose    bool
	jsonOutput bool
)

var rootCmd = &cobra.Command{
	Use:   "giztoy",
	Short: "Unified CLI for giztoy AI services",
	Long: `giztoy — A unified command line interface for AI services.

Commands:
  ctx            Context configuration management
  minimax        MiniMax API
  doubaospeech   Doubao Speech API
  dashscope      DashScope API
  memory         Personal memory system
  genx           GenX model config management
  version        Version information

Use 'giztoy <command> --help' for more information about a command.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "JSON output (deterministic key order)")
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
