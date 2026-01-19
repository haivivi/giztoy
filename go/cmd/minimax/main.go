// Package main provides the MiniMax CLI tool.
//
// Usage:
//
//	minimax [flags] <service> <command> [args]
//
// Services:
//
//	text     - Text generation service
//	speech   - Speech synthesis service
//	video    - Video generation service
//	image    - Image generation service
//	music    - Music generation service
//	voice    - Voice management service
//	file     - File management service
//	config   - Configuration management
//
// Configuration:
//
//	The CLI stores configuration in ~/.giztoy/minimax/
//	Use 'minimax config' commands to manage contexts.
package main

import (
	"fmt"
	"os"

	"github.com/haivivi/giztoy/cmd/minimax/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
