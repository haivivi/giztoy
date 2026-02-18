package commands

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var genxCmd = &cobra.Command{
	Use:   "genx",
	Short: "GenX model config management (generator/transformer/segmentor/profiler)",
	Long: `Manage GenX model configurations stored in the current context.

Config files are JSON/YAML with 'type' and 'schema' fields.
Supported types: generator, tts, asr, realtime, segmentor, profiler.

Examples:
  giztoy genx add testdata/models/generator-qwen.json
  giztoy genx add testdata/models/tts-minimax.json
  giztoy genx list
  giztoy genx list --type generator
  giztoy genx remove qwen/turbo-latest`,
}

var genxFilterType string

var genxAddCmd = &cobra.Command{
	Use:   "add <config-file>",
	Short: "Add a model config to the current context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		if err := s.GenXAdd(args[0]); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{"file": args[0], "status": "added"})
		}
		fmt.Printf("Added %s to genx config.\n", args[0])
		return nil
	},
}

var genxListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered model configs",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		infos, err := s.GenXList(genxFilterType)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(infos)
		}
		if len(infos) == 0 {
			fmt.Println("No genx configs registered.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TYPE\tPATTERN\tSCHEMA\tFILE")
		for _, info := range infos {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", info.Type, info.Pattern, info.Schema, info.File)
		}
		w.Flush()
		return nil
	},
}

var genxRemoveCmd = &cobra.Command{
	Use:   "remove <pattern>",
	Short: "Remove a model config by pattern",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		if err := s.GenXRemove(args[0]); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{"pattern": args[0], "status": "removed"})
		}
		fmt.Printf("Removed %s from genx config.\n", args[0])
		return nil
	},
}

func init() {
	genxListCmd.Flags().StringVar(&genxFilterType, "type", "", "filter by type (generator, tts, asr, realtime, segmentor, profiler)")

	genxCmd.AddCommand(genxAddCmd)
	genxCmd.AddCommand(genxListCmd)
	genxCmd.AddCommand(genxRemoveCmd)

	rootCmd.AddCommand(genxCmd)
}
