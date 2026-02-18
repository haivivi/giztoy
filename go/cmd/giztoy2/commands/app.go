package commands

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/cortex"
)

type appField struct {
	Name     string
	Flag     string
	Required bool
	Desc     string
}

func addAppCommands(parent *cobra.Command, service string, fields []appField) {
	appCmd := &cobra.Command{
		Use:   "app",
		Short: fmt.Sprintf("Manage %s app instances", service),
	}

	addCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new app",
		Args:  cobra.ExactArgs(1),
		RunE:  makeAppAddFunc(service, fields),
	}
	for _, f := range fields {
		if f.Required {
			addCmd.Flags().String(f.Flag, "", f.Desc+" (required)")
		} else {
			addCmd.Flags().String(f.Flag, "", f.Desc)
		}
	}

	useCmd := &cobra.Command{
		Use:   "use <name>",
		Short: "Switch the current app",
		Args:  cobra.ExactArgs(1),
		RunE:  makeAppUseFunc(service),
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all apps",
		RunE:  makeAppListFunc(service),
	}

	showCmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show app details",
		Args:  cobra.ExactArgs(1),
		RunE:  makeAppShowFunc(service),
	}

	removeCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an app",
		Args:  cobra.ExactArgs(1),
		RunE:  makeAppRemoveFunc(service),
	}

	appCmd.AddCommand(addCmd, useCmd, listCmd, showCmd, removeCmd)
	parent.AddCommand(appCmd)
}

func makeAppAddFunc(service string, fields []appField) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg := make(map[string]any)
		for _, f := range fields {
			val, _ := cmd.Flags().GetString(f.Flag)
			if val != "" {
				cfg[f.Flag] = val
			} else if f.Required {
				return fmt.Errorf("--%s is required", f.Flag)
			}
		}
		s, err := openStore()
		if err != nil {
			return err
		}
		if err := s.AppAdd(service, name, cfg); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{"service": service, "name": name, "status": "created"})
		}
		fmt.Printf("App %q added to %s.\n", name, service)
		return nil
	}
}

func makeAppUseFunc(service string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		if err := s.AppUse(service, args[0]); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{"service": service, "name": args[0], "status": "active"})
		}
		fmt.Printf("Switched %s to app %q.\n", service, args[0])
		return nil
	}
}

func makeAppListFunc(service string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		infos, err := s.AppList(service)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(infos)
		}
		if len(infos) == 0 {
			fmt.Printf("No apps configured for %s.\n", service)
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, info := range infos {
			marker := "  "
			if info.Current {
				marker = "* "
			}
			fmt.Fprintf(w, "%s%s\n", marker, info.Name)
		}
		w.Flush()
		return nil
	}
}

func makeAppShowFunc(service string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		cfg, err := s.AppShow(service, args[0])
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(cfg)
		}
		fmt.Printf("App: %s/%s\n", service, args[0])
		for k, v := range cfg {
			val := fmt.Sprintf("%v", v)
			if k == "api_key" || k == "token" || k == "console_sk" {
				if len(val) > 8 {
					val = val[:4] + "..." + val[len(val)-4:]
				}
			}
			fmt.Printf("  %s: %s\n", k, val)
		}
		return nil
	}
}

func makeAppRemoveFunc(service string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		if err := s.AppRemove(service, args[0]); err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{"service": service, "name": args[0], "status": "removed"})
		}
		fmt.Printf("App %q removed from %s.\n", args[0], service)
		return nil
	}
}

// resolveAppName resolves the app name: if provided use it,
// otherwise fall back to the current app for the service.
func resolveAppName(s *cortex.ConfigStore, service, appName string) (string, error) {
	if appName != "" {
		return appName, nil
	}
	return s.AppCurrent(service)
}
