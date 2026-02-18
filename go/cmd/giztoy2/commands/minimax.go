package commands

import "github.com/spf13/cobra"

var minimaxCmd = &cobra.Command{
	Use:   "minimax",
	Short: "MiniMax API (text, speech, video, image, music, voice, file)",
	Long: `MiniMax API client. Supports multiple app instances with different
API keys and base URLs (domestic vs overseas).

Usage: giztoy minimax <app_name> <command...>

Examples:
  giztoy minimax cn text chat -f chat.yaml
  giztoy minimax global speech synthesize -f request.yaml`,
}

func init() {
	addAppCommands(minimaxCmd, "minimax", []appField{
		{Name: "api_key", Flag: "api-key", Required: true, Desc: "MiniMax API Key"},
		{Name: "base_url", Flag: "base-url", Required: false, Desc: "API base URL (domestic/overseas)"},
		{Name: "default_model", Flag: "default-model", Required: false, Desc: "Default model name"},
		{Name: "default_voice", Flag: "default-voice", Required: false, Desc: "Default voice ID"},
		{Name: "max_retries", Flag: "max-retries", Required: false, Desc: "Max retry count"},
	})
	rootCmd.AddCommand(minimaxCmd)
}
