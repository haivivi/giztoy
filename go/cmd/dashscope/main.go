// Package main provides the DashScope CLI tool.
//
// Usage:
//
//	dashscope [flags] <service> <command> [args]
//
// Services:
//
//	omni     - Qwen-Omni-Realtime multimodal conversation
//	config   - Configuration management
//
// Configuration:
//
//	The CLI stores configuration in ~/.giztoy/dashscope/
//	Use 'dashscope config' commands to manage contexts.
package main

import (
	"fmt"
	"os"

	"github.com/haivivi/giztoy/go/cmd/dashscope/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
