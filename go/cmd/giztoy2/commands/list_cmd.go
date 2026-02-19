package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/cortex"
)

var (
	listLimit int
	listFrom  string
	listAll   bool
)

var listCmd = &cobra.Command{
	Use:   "list <prefix*>",
	Short: "List resources by prefix",
	Long: `List resources matching a prefix pattern. Pattern must end with *.

Examples:
  giztoy list creds:*
  giztoy list creds:openai:*
  giztoy list genx:generator:*
  giztoy list genx:* --limit=20
  giztoy list creds:* --all`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := args[0]

		c, err := openCortex(cmd.Context())
		if err != nil {
			return err
		}
		defer c.Close()

		docs, err := c.List(cmd.Context(), pattern, cortex.ListOpts{
			Limit: listLimit,
			From:  listFrom,
			All:   listAll,
		})
		if err != nil {
			return err
		}

		if formatOutput == "json" {
			return printJSON(docs)
		}

		if formatOutput == "name" {
			for _, doc := range docs {
				fmt.Println(doc.FullName())
			}
			return nil
		}

		if len(docs) == 0 {
			fmt.Println("No resources found.")
			return nil
		}

		// table format
		w := newTabWriter()
		fmt.Fprintln(w, "KIND\tNAME\tDETAILS")
		for _, doc := range docs {
			details := summarizeDoc(doc)
			fmt.Fprintf(w, "%s\t%s\t%s\n", doc.Kind, doc.Name(), details)
		}
		w.Flush()
		fmt.Printf("(%d items)\n", len(docs))
		return nil
	},
}

func summarizeDoc(doc cortex.Document) string {
	switch {
	case doc.GetString("model") != "":
		return "model=" + doc.GetString("model")
	case doc.GetString("voice_id") != "":
		return "voice_id=" + doc.GetString("voice_id")
	case doc.GetString("base_url") != "":
		return "base_url=" + doc.GetString("base_url")
	case doc.GetString("api_key") != "":
		k := doc.GetString("api_key")
		if len(k) > 8 {
			return "api_key=" + k[:4] + "..."
		}
		return "api_key=***"
	case doc.GetString("app_id") != "":
		return "app_id=" + doc.GetString("app_id")
	default:
		return ""
	}
}

func init() {
	listCmd.Flags().IntVar(&listLimit, "limit", 10, "max items to return")
	listCmd.Flags().StringVar(&listFrom, "from", "", "start listing after this key")
	listCmd.Flags().BoolVar(&listAll, "all", false, "list all items (ignore limit)")

	rootCmd.AddCommand(listCmd)
}
