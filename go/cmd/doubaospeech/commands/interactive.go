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
  doubao interactive
  doubao i
  doubao tui`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := getContext()
		if err != nil {
			// Don't require context for interactive mode
			fmt.Println("Note: No context selected. Use 'doubao config use-context' to set one.")
		} else {
			fmt.Printf("Using context: %s\n", ctx.Name)
		}

		// TODO: Implement TUI using bubbletea or similar
		fmt.Println()
		fmt.Println("╔══════════════════════════════════════════════════════════════╗")
		fmt.Println("║               Doubao Speech API Interactive Mode             ║")
		fmt.Println("╠══════════════════════════════════════════════════════════════╣")
		fmt.Println("║                                                              ║")
		fmt.Println("║  [TUI not implemented yet]                                   ║")
		fmt.Println("║                                                              ║")
		fmt.Println("║  Available services:                                         ║")
		fmt.Println("║    1. TTS - Text-to-Speech                                   ║")
		fmt.Println("║    2. ASR - Speech Recognition                               ║")
		fmt.Println("║    3. Voice - Voice Cloning                                  ║")
		fmt.Println("║    4. Realtime - Real-time Conversation                      ║")
		fmt.Println("║    5. Meeting - Meeting Transcription                        ║")
		fmt.Println("║    6. Podcast - Podcast Synthesis                            ║")
		fmt.Println("║    7. Translation - Simultaneous Translation                 ║")
		fmt.Println("║    8. Media - Subtitle Extraction                            ║")
		fmt.Println("║    9. Console - Account Management                           ║")
		fmt.Println("║                                                              ║")
		fmt.Println("║  Press Ctrl+C to exit                                        ║")
		fmt.Println("║                                                              ║")
		fmt.Println("╚══════════════════════════════════════════════════════════════╝")
		fmt.Println()

		return nil
	},
}
