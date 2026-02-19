package commands

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <full_name>",
	Short: "Get a resource by full name",
	Long: `Get a single resource by its full KV name.

Examples:
  giztoy get creds:openai:qwen
  giztoy get genx:generator:qwen/turbo
  giztoy get creds:minimax:cn --format json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fullName := args[0]

		c, err := openCortex(cmd.Context())
		if err != nil {
			return err
		}
		defer c.Close()

		doc, err := c.Get(cmd.Context(), fullName)
		if err != nil {
			return err
		}

		if formatOutput == "json" {
			return printJSON(doc)
		}

		// default: yaml
		data, err := yaml.Marshal(doc.Fields)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
}
