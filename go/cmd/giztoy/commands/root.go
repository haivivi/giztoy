package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/cmd/giztoy/internal/config"
)

var (
	// Global flags
	verbose bool

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

  # Use the current context for subcommands
  giztoy config use-context dev
  giztoy minimax text chat -f chat.yaml

  # Or specify context on the subcommand
  giztoy minimax -c dev speech synthesize -f request.yaml`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

// configLoadErr stores the error from config.Load() for deferred reporting.
var configLoadErr error

func initConfig() {
	cfg, err := config.Load()
	if err != nil {
		// Store error for deferred reporting — commands that need config
		// will get a clear error via GetConfig(). This avoids failing
		// non-config commands like 'giztoy version'.
		configLoadErr = err
		return
	}
	globalConfig = cfg
}

// GetConfig returns the global configuration.
// Returns an error if the config could not be loaded (e.g., HOME not set).
func GetConfig() (*config.Config, error) {
	if globalConfig == nil {
		if configLoadErr != nil {
			return nil, fmt.Errorf("config not available: %w", configLoadErr)
		}
		// Try loading again (e.g., dir was created since init).
		cfg, err := config.Load()
		if err != nil {
			return nil, fmt.Errorf("config not available: %w", err)
		}
		globalConfig = cfg
	}
	return globalConfig, nil
}

// IsVerbose returns whether verbose mode is enabled.
func IsVerbose() bool {
	return verbose
}
