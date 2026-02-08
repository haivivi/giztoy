package commands

import (
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cortextest",
	Short: "Cortex Atom server for testing",
	Long: `cortextest is a CLI tool for testing cortex.Atom with various transformers.

It starts an embedded MQTT broker and creates Atom instances for each
connected gear device. The Atom bridges the device audio with a configurable
transformer (e.g., DashScope realtime, Doubao, etc.).

Usage:
  cortextest run --port :1883 --transformer dashscope`,
}

// Command returns the root cobra command for mounting into a parent CLI.
func Command() *cobra.Command {
	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(runCmd)
}
