package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <full_name>",
	Short: "Delete a resource by full name",
	Long: `Delete a single resource by its full KV name.

Examples:
  giztoy delete creds:openai:qwen
  giztoy delete genx:generator:qwen/turbo`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fullName := args[0]

		c, err := openCortex(cmd.Context())
		if err != nil {
			return err
		}
		defer c.Close()

		if err := c.Delete(cmd.Context(), fullName); err != nil {
			return err
		}

		if formatOutput == "json" {
			return printJSON(map[string]any{"name": fullName, "status": "deleted"})
		}
		fmt.Printf("Deleted %s\n", fullName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
