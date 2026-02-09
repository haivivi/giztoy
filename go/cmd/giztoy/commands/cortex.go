package commands

import (
	cortexcmd "github.com/haivivi/giztoy/go/cmd/cortextest/commands"
)

func init() {
	cmd := cortexcmd.Command()
	cmd.Use = "cortex"
	cmd.Short = "Cortex server (bridges devices with AI transformers)"
	rootCmd.AddCommand(cmd)
}
