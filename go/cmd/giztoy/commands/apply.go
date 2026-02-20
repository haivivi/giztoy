package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/cortex"
)

var applyFile string

var applyCmd = &cobra.Command{
	Use:   "apply -f <file>",
	Short: "Apply resource configuration from YAML",
	Long: `Apply one or more resource documents from a YAML file.
Use '-' to read from stdin. Multi-document YAML (--- separated) is supported.

Supported kinds:
  creds/openai, creds/genai, creds/minimax, creds/doubaospeech, creds/dashscope
  genx/generator, genx/tts, genx/asr, genx/realtime, genx/segmentor, genx/profiler

Examples:
  giztoy apply -f setup.yaml
  giztoy apply -f creds.yaml
  cat config.yaml | giztoy apply -f -`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if applyFile == "" {
			return fmt.Errorf("flag -f is required")
		}

		docs, err := cortex.ParseDocumentsFromFile(applyFile)
		if err != nil {
			return err
		}
		if len(docs) == 0 {
			return fmt.Errorf("no documents found in %s", applyFile)
		}

		c, err := openCortex(cmd.Context())
		if err != nil {
			return err
		}
		defer c.Close()

		results, err := c.Apply(cmd.Context(), docs)
		if err != nil {
			return err
		}

		if formatOutput == "json" {
			return printJSON(results)
		}

		for _, r := range results {
			fmt.Printf("%s %s %s\n", r.Kind, r.Name, r.Status)
		}
		return nil
	},
}

func init() {
	applyCmd.Flags().StringVarP(&applyFile, "file", "f", "", "YAML file to apply (use '-' for stdin)")
	rootCmd.AddCommand(applyCmd)
}
