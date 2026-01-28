package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/cli"
)

const appName = "minimax"

var (
	// Global flags
	cfgFile     string
	contextName string
	outputFile  string
	inputFile   string
	outputJSON  bool
	verbose     bool

	// Global configuration
	globalConfig *cli.Config
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "minimax",
	Short: "MiniMax API CLI tool",
	Long: `MiniMax CLI - A command line interface for MiniMax API.

This tool allows you to interact with MiniMax's AI services including:
  - Text generation (chat completions)
  - Speech synthesis (TTS)
  - Video generation (T2V, I2V)
  - Image generation
  - Music generation
  - Voice management (clone, design)
  - File management

Configuration is stored in ~/.giztoy/minimax/ and supports multiple contexts,
similar to kubectl's context management.

Examples:
  # Set up a new context
  minimax config add-context myctx --api-key YOUR_API_KEY

  # Use context to run commands
  minimax -c myctx speech synthesize -f request.yaml

  # Pipe output to another command
  minimax -c myctx text chat -f chat.yaml --json | jq '.choices[0].message'
`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global persistent flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "", "", "config file (default is ~/.giztoy/minimax/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&contextName, "context", "c", "", "context name to use")
	rootCmd.PersistentFlags().StringVarP(&outputFile, "output", "o", "", "output file (default: stdout)")
	rootCmd.PersistentFlags().StringVarP(&inputFile, "file", "f", "", "input request file (YAML or JSON)")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "output as JSON (for piping)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Add subcommands
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(textCmd)
	rootCmd.AddCommand(speechCmd)
	rootCmd.AddCommand(videoCmd)
	rootCmd.AddCommand(imageCmd)
	rootCmd.AddCommand(musicCmd)
	rootCmd.AddCommand(voiceCmd)
	rootCmd.AddCommand(fileCmd)
	rootCmd.AddCommand(interactiveCmd)
}

func initConfig() {
	var err error
	globalConfig, err = cli.LoadConfigWithPath(appName, cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing config: %v\n", err)
		os.Exit(1)
	}
}

// getConfig returns the global configuration
func getConfig() *cli.Config {
	return globalConfig
}

// getContext returns the context configuration to use
func getContext() (*cli.Context, error) {
	cfg := getConfig()
	if cfg == nil {
		return nil, fmt.Errorf("configuration not initialized")
	}

	ctx, err := cfg.ResolveContext(contextName)
	if err != nil {
		if contextName == "" {
			return nil, fmt.Errorf("no context specified. Use -c flag or set a default context with 'minimax config use-context'")
		}
		return nil, err
	}

	return ctx, nil
}

// getInputFile returns the input file path
func getInputFile() string {
	return inputFile
}

// getOutputFile returns the output file path
func getOutputFile() string {
	return outputFile
}

// isJSONOutput returns whether output should be JSON
func isJSONOutput() bool {
	return outputJSON
}

// isVerbose returns whether verbose mode is enabled
func isVerbose() bool {
	return verbose
}

// outputResult outputs the result using cli package
func outputResult(result any, outputPath string, asJSON bool) error {
	format := cli.FormatYAML
	if asJSON {
		format = cli.FormatJSON
	}
	return cli.Output(result, cli.OutputOptions{
		Format: format,
		File:   outputPath,
	})
}

// printVerbose prints verbose output if enabled
func printVerbose(format string, args ...any) {
	cli.PrintVerbose(verbose, format, args...)
}
