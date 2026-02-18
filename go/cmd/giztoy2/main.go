package main

import (
	"fmt"
	"os"

	"github.com/haivivi/giztoy/go/cmd/giztoy2/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
