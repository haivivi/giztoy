package commands

import (
	"github.com/haivivi/giztoy/go/cmd/giztoy/commands/dashscope"
)

func init() {
	rootCmd.AddCommand(dashscope.Cmd)
}
