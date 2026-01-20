package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/pkg/cli"
)

const appName = "doubao"

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
	Use:   "doubao",
	Short: "Doubao Speech API CLI tool",
	Long: `Doubao CLI - A command line interface for Doubao Speech API (火山引擎豆包语音).

This tool allows you to interact with Doubao's speech services including:
  - TTS (Text-to-Speech): 语音合成，支持同步、流式、异步
  - ASR (Automatic Speech Recognition): 语音识别，支持一句话、流式、文件
  - Voice Clone: 声音复刻，训练自定义音色
  - Realtime: 端到端实时语音对话
  - Meeting: 会议转写
  - Podcast: 播客合成
  - Translation: 同声传译
  - Media: 音视频字幕提取

Configuration is stored in ~/.giztoy/doubao/ and supports multiple contexts,
similar to kubectl's context management.

Examples:
  # Set up a new context
  doubao config add-context myctx --token YOUR_TOKEN --app-id YOUR_APP_ID --cluster volcano_tts

  # Use context to run commands
  doubao -c myctx tts synthesize -f request.yaml -o output.mp3

  # Pipe output to another command
  doubao -c myctx asr one-sentence -f request.yaml --json | jq '.text'

  # Real-time conversation
  doubao -c myctx realtime interactive -f config.yaml
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
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "", "", "config file (default is ~/.giztoy/doubao/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&contextName, "context", "c", "", "context name to use")
	rootCmd.PersistentFlags().StringVarP(&outputFile, "output", "o", "", "output file (default: stdout)")
	rootCmd.PersistentFlags().StringVarP(&inputFile, "file", "f", "", "input request file (YAML or JSON)")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "output as JSON (for piping)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Add subcommands
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(ttsCmd)
	rootCmd.AddCommand(asrCmd)
	rootCmd.AddCommand(voiceCmd)
	rootCmd.AddCommand(realtimeCmd)
	rootCmd.AddCommand(meetingCmd)
	rootCmd.AddCommand(podcastCmd)
	rootCmd.AddCommand(translationCmd)
	rootCmd.AddCommand(mediaCmd)
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
			return nil, fmt.Errorf("no context specified. Use -c flag or set a default context with 'doubao config use-context'")
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
