// Package main is the entry point for the unified giztoy CLI.
//
// Usage:
//
//	giztoy [flags] <command> [subcommand] [args]
//
// Commands:
//
//	config     - Configuration management (contexts, services)
//	minimax    - MiniMax API (text, speech, video, image, music, voice, file)
//	doubao     - Doubao Speech API (tts, asr, voice, realtime, meeting, podcast, ...)
//	dashscope  - DashScope API (omni chat)
//	cortex     - Cortex server (run)
//	gear       - Chatgear device simulator (run, config)
//	version    - Show version information
package main

import (
	"fmt"
	"os"

	"github.com/haivivi/giztoy/go/cmd/giztoy/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
