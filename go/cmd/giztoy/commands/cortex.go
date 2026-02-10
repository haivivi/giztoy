package commands

import (
	"github.com/haivivi/giztoy/go/cmd/giztoy/commands/cortex"
)

func init() {
	rootCmd.AddCommand(cortex.Cmd)
}
