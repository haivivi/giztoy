package commands

import (
	"fmt"
	"os"
	"os/exec"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/cmd/giztoy/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long: `Manage contexts and service configurations.

A context is a named directory holding per-service YAML config files.
For example, "dev" context might contain minimax.yaml, doubao.yaml, etc.

Examples:
  giztoy config list-contexts
  giztoy config add-context staging
  giztoy config use-context dev
  giztoy config current-context
  giztoy config set dev minimax api_key sk-xxx
  giztoy config get dev minimax api_key
  giztoy config edit dev minimax`,
}

var configListContextsCmd = &cobra.Command{
	Use:     "list-contexts",
	Aliases: []string{"ls"},
	Short:   "List all contexts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		names, err := cfg.ListContexts()
		if err != nil {
			return err
		}

		if len(names) == 0 {
			fmt.Println("No contexts configured.")
			fmt.Println("Create one with: giztoy config add-context <name>")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "CURRENT\tNAME\tSERVICES")

		for _, name := range names {
			current := ""
			if name == cfg.CurrentContext {
				current = "*"
			}

			services, _ := config.ListServices(cfg.ContextDir(name))
			svcList := ""
			for i, s := range services {
				if i > 0 {
					svcList += ", "
				}
				svcList += s
			}

			fmt.Fprintf(w, "%s\t%s\t%s\n", current, name, svcList)
		}
		w.Flush()
		return nil
	},
}

var configAddContextCmd = &cobra.Command{
	Use:   "add-context <name>",
	Short: "Create a new context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		name := args[0]

		if err := cfg.AddContext(name); err != nil {
			return err
		}
		fmt.Printf("Context %q created.\n", name)
		fmt.Printf("Configure services with: giztoy config set %s <service> <key> <value>\n", name)
		return nil
	},
}

var configDeleteContextCmd = &cobra.Command{
	Use:   "delete-context <name>",
	Short: "Delete a context and all its service configs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		name := args[0]

		if err := cfg.DeleteContext(name); err != nil {
			return err
		}
		fmt.Printf("Context %q deleted.\n", name)
		return nil
	},
}

var configUseContextCmd = &cobra.Command{
	Use:   "use-context <name>",
	Short: "Set the current context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		name := args[0]

		if err := cfg.UseContext(name); err != nil {
			return err
		}
		fmt.Printf("Switched to context %q.\n", name)
		return nil
	},
}

var configCurrentContextCmd = &cobra.Command{
	Use:   "current-context",
	Short: "Display the current context name",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		if cfg.CurrentContext == "" {
			fmt.Println("No current context set.")
			return nil
		}
		fmt.Println(cfg.CurrentContext)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <context> <service> <key> <value>",
	Short: "Set a service config value",
	Long: `Set a key-value pair in a service's YAML config file.

Examples:
  giztoy config set dev minimax api_key sk-xxxx
  giztoy config set dev doubao app_id 12345
  giztoy config set dev dashscope api_key sk-xxxx`,
	Args: cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		ctxName, service, key, value := args[0], args[1], args[2], args[3]

		contextDir := cfg.ContextDir(ctxName)
		if _, err := os.Stat(contextDir); os.IsNotExist(err) {
			return fmt.Errorf("context %q not found", ctxName)
		}

		// Load existing service config as map, or create new.
		existing, err := config.LoadService[map[string]any](contextDir, service)
		if err != nil {
			// File doesn't exist yet — start fresh.
			m := map[string]any{key: value}
			existing = &m
		} else {
			(*existing)[key] = value
		}

		if err := config.SaveService(contextDir, service, existing); err != nil {
			return err
		}

		fmt.Printf("Set %s.%s = %s (context: %s)\n", service, key, value, ctxName)
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <context> <service> <key>",
	Short: "Get a service config value",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		ctxName, service, key := args[0], args[1], args[2]

		contextDir := cfg.ContextDir(ctxName)
		m, err := config.LoadService[map[string]any](contextDir, service)
		if err != nil {
			return err
		}

		val, ok := (*m)[key]
		if !ok {
			return fmt.Errorf("key %q not found in %s config", key, service)
		}

		fmt.Println(val)
		return nil
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit <context> <service>",
	Short: "Open a service config in the default editor",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := GetConfig()
		ctxName, service := args[0], args[1]

		path := cfg.ServicePath(ctxName, service)

		// Create the file if it doesn't exist.
		dir := cfg.ContextDir(ctxName)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("context %q not found", ctxName)
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte("# "+service+" configuration\n"), 0644); err != nil {
				return fmt.Errorf("create %s: %w", path, err)
			}
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}

		c := exec.Command(editor, path)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

func init() {
	configCmd.AddCommand(configListContextsCmd)
	configCmd.AddCommand(configAddContextCmd)
	configCmd.AddCommand(configDeleteContextCmd)
	configCmd.AddCommand(configUseContextCmd)
	configCmd.AddCommand(configCurrentContextCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configEditCmd)

	rootCmd.AddCommand(configCmd)
}
