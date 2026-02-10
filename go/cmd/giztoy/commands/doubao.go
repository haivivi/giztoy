package commands

import (
	"github.com/haivivi/giztoy/go/cmd/giztoy/commands/doubao"
)

func init() {
	rootCmd.AddCommand(doubao.Cmd)
}
