package commands

import (
	gearcmd "github.com/haivivi/giztoy/go/cmd/geartest/commands"
)

func init() {
	cmd := gearcmd.Command()
	cmd.Use = "gear"
	cmd.Short = "Chatgear device simulator"
	rootCmd.AddCommand(cmd)
}
