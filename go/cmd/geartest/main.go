// geartest is a CLI tool to simulate a chatgear device.
//
// It provides a TUI interface and WebRTC-based audio I/O for testing
// chatgear server implementations.
//
// Usage:
//
//	geartest run                    # Run simulator with current context
//	geartest run -c staging         # Run with specified context
//	geartest config context list    # List all contexts
//	geartest config context use dev # Switch to dev context
//
// Configuration is stored in ~/.giztoy/geartest/
package main

import (
	"os"

	"github.com/haivivi/giztoy/go/cmd/geartest/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
