package commands

import (
	"github.com/haivivi/giztoy/go/cmd/giztoy/commands/minimax"
)

func init() {
	rootCmd.AddCommand(minimax.Cmd)
}
