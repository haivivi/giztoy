package commands

import (
	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/cmd/giztoy/internal/config"
)

var (
	// Global flags
	contextName string
	verbose     bool

	// Global configuration (loaded at init time)
	globalConfig *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "giztoy",
	Short: "Unified CLI for giztoy AI services",
	Long: `giztoy - A unified command line interface for AI services.

Supported services:
  minimax    MiniMax API (text, speech, video, image, music, voice, file)
  doubao     Doubao Speech API (tts, asr, voice, realtime, meeting, ...)
  dashscope  DashScope API (omni multimodal chat)
  cortex     Cortex server (bridges devices with AI transformers)
  gear       Chatgear device simulator

Configuration is stored in the OS config directory:
  macOS:   ~/Library/Application Support/giztoy/
  Linux:   ~/.config/giztoy/
  Windows: %AppData%/giztoy/

Use 'giztoy config' to manage contexts and service configurations.

Examples:
  # Create a context and configure a service
  giztoy config add-context dev
  giztoy config set dev minimax api_key YOUR_KEY

  # Run a command with a specific context
  giztoy -c dev minimax speech synthesize -f request.yaml

  # Use the current context
  giztoy config use-context dev
  giztoy minimax text chat -f chat.yaml`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&contextName, "context", "c", "", "context name (default: current-context)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

func initConfig() {
	cfg, err := config.Load()
	if err != nil {
		// Non-fatal: config dir may not exist yet.
		cfg = &config.Config{}
	}
	globalConfig = cfg
}

// GetConfig returns the global configuration.
func GetConfig() *config.Config {
	return globalConfig
}

// GetContextName returns the context name from the flag.
func GetContextName() string {
	return contextName
}

// IsVerbose returns whether verbose mode is enabled.
func IsVerbose() bool {
	return verbose
}
