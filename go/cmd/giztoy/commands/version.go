package commands

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/cmd/giztoy/internal/build"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(build.String())
		if IsVerbose() {
			fmt.Printf("  go:     %s\n", runtime.Version())
			fmt.Printf("  config: %s\n", GetConfig().Dir)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
