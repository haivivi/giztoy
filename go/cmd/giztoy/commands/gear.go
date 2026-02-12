package commands

import (
	"github.com/haivivi/giztoy/go/cmd/giztoy/commands/gear"
)

func init() {
	rootCmd.AddCommand(gear.Cmd)
}
