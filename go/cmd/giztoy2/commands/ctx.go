package commands

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"


	"github.com/haivivi/giztoy/go/pkg/cortex"
)

func openStore() (*cortex.ConfigStore, error) {
	if dir := os.Getenv("GIZTOY_CONFIG_DIR"); dir != "" {
		return cortex.OpenConfigStoreAt(dir)
	}
	return cortex.OpenConfigStore()
}

var ctxCmd = &cobra.Command{
	Use:   "ctx",
	Short: "Context configuration management",
	Long: `Manage contexts and storage backend configurations.

A context is a named set of storage backend connections (KV, Storage,
VecStore, Embed). Switching contexts switches the entire backend stack.

Examples:
  giztoy ctx add dev
  giztoy ctx use dev
  giztoy ctx config set kv badger:///data/dev
  giztoy ctx list`,
}

var ctxAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create a new context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		if err := s.CtxAdd(args[0]); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{"name": args[0], "status": "created"})
		}
		fmt.Printf("Context %q created.\n", args[0])
		return nil
	},
}

var ctxRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		if err := s.CtxRemove(args[0]); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{"name": args[0], "status": "removed"})
		}
		fmt.Printf("Context %q removed.\n", args[0])
		return nil
	},
}

var ctxUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch the current context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		if err := s.CtxUse(args[0]); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{"name": args[0], "status": "active"})
		}
		fmt.Printf("Switched to context %q.\n", args[0])
		return nil
	},
}

var ctxCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the current context name",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		name, err := s.CtxCurrent()
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{"current": name})
		}
		fmt.Println(name)
		return nil
	},
}

var ctxListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all contexts",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		infos, err := s.CtxList()
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(infos)
		}
		if len(infos) == 0 {
			fmt.Println("No contexts configured.")
			fmt.Println("Create one with: giztoy ctx add <name>")
			return nil
		}
		for _, info := range infos {
			marker := "  "
			if info.Current {
				marker = "* "
			}
			fmt.Printf("%s%s\n", marker, info.Name)
		}
		return nil
	},
}

var ctxShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show context details",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		ctxName, cfg, err := s.CtxShow(name)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{
				"name":     ctxName,
				"kv":       cfg.KV,
				"storage":  cfg.Storage,
				"vecstore": cfg.VecStore,
				"embed":    cfg.Embed,
			})
		}
		fmt.Printf("Context: %s\n", ctxName)
		fmt.Printf("  kv:       %s\n", valueOrEmpty(cfg.KV))
		fmt.Printf("  storage:  %s\n", valueOrEmpty(cfg.Storage))
		fmt.Printf("  vecstore: %s\n", valueOrEmpty(cfg.VecStore))
		fmt.Printf("  embed:    %s\n", valueOrEmpty(cfg.Embed))
		return nil
	},
}

var ctxConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage context config keys",
}

var ctxConfigSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config key on the current context",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		if err := s.CtxConfigSet(args[0], args[1]); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{"key": args[0], "value": args[1], "status": "set"})
		}
		fmt.Printf("Set %s = %s\n", args[0], args[1])
		return nil
	},
}

var ctxConfigListCmd = &cobra.Command{
	Use:   "list",
	Short: "List supported config keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		keys := s.CtxConfigList()
		if jsonOutput {
			return printJSON(keys)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, k := range keys {
			fmt.Fprintf(w, "%s\t%s\n", k.Key, k.Description)
		}
		w.Flush()
		return nil
	},
}

func init() {
	ctxConfigCmd.AddCommand(ctxConfigSetCmd)
	ctxConfigCmd.AddCommand(ctxConfigListCmd)

	ctxCmd.AddCommand(ctxAddCmd)
	ctxCmd.AddCommand(ctxRemoveCmd)
	ctxCmd.AddCommand(ctxUseCmd)
	ctxCmd.AddCommand(ctxCurrentCmd)
	ctxCmd.AddCommand(ctxListCmd)
	ctxCmd.AddCommand(ctxShowCmd)
	ctxCmd.AddCommand(ctxConfigCmd)

	rootCmd.AddCommand(ctxCmd)
}

func valueOrEmpty(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}
