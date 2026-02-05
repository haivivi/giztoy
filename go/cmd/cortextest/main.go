// cortextest is a CLI tool for testing cortex.Atom with various transformers.
//
// It starts an embedded MQTT broker and creates Atom instances for each
// connected gear device.
//
// Usage:
//
//	cortextest run --port :1883 --transformer dashscope
package main

import (
	"os"

	"github.com/haivivi/giztoy/go/cmd/cortextest/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
