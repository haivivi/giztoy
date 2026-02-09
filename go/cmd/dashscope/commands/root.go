package commands

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/cli"
)

const appName = "dashscope"

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
	Use:   "dashscope",
	Short: "DashScope (Aliyun Model Studio) CLI tool",
	Long: `DashScope CLI - A command line interface for Aliyun DashScope API.

This tool allows you to interact with DashScope's AI services including:
  - Qwen-Omni-Realtime multimodal conversation (text + audio)

Configuration is stored in ~/.giztoy/dashscope/ and supports multiple contexts,
similar to kubectl's context management.

Examples:
  # Set up a new context
  dashscope config add-context myctx --api-key YOUR_API_KEY

  # Start interactive omni conversation
  dashscope -c myctx omni chat

  # Use config file for custom settings
  dashscope -c myctx omni chat -f omni-chat.yaml -o output.pcm
`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Command returns the root cobra command for mounting into a parent CLI.
func Command() *cobra.Command {
	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global persistent flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "", "", "config file (default is ~/.giztoy/dashscope/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&contextName, "context", "c", "", "context name to use")
	rootCmd.PersistentFlags().StringVarP(&outputFile, "output", "o", "", "output file (default: stdout)")
	rootCmd.PersistentFlags().StringVarP(&inputFile, "file", "f", "", "input request file (YAML or JSON)")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "output as JSON (for piping)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Add subcommands
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(omniCmd)
}

func initConfig() {
	// Configure slog based on verbose flag
	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})))

	var err error
	globalConfig, err = cli.LoadConfigWithPath(appName, cfgFile)
	if err != nil {
		// Log but don't exit — allows the binary to run non-config commands
		// (e.g., when mounted in a unified CLI like giztoy).
		fmt.Fprintf(os.Stderr, "Warning: %s config: %v\n", appName, err)
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
			return nil, fmt.Errorf("no context specified. Use -c flag or set a default context with 'dashscope config use-context'")
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
