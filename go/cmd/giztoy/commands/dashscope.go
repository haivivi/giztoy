package commands

import (
	dashscopecmd "github.com/haivivi/giztoy/go/cmd/dashscope/commands"
)

func init() {
	cmd := dashscopecmd.Command()
	cmd.Use = "dashscope"
	cmd.Short = "DashScope API (omni multimodal chat)"
	rootCmd.AddCommand(cmd)
}
