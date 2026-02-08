package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var interactiveCmd = &cobra.Command{
	Use:     "interactive",
	Aliases: []string{"i", "tui"},
	Short:   "Interactive TUI mode",
	Long: `Start an interactive TUI (Text User Interface) mode.

Provides a visual interface for exploring and testing Doubao Speech APIs.

Features:
  - Browse available services and endpoints
  - Build and send requests interactively
  - View responses with syntax highlighting
  - Save and load request templates

Examples:
  doubaospeech interactive
  doubaospeech i
  doubaospeech tui`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("not implemented: interactive TUI mode requires a terminal UI framework (bubbletea); use individual subcommands instead")
	},
}
