package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/haivivi/giztoy/go/pkg/cli"
	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: `Manage geartest configuration.

Configuration is stored in ~/.giztoy/geartest/config.yaml`,
}

// contextCmd represents the context subcommand
var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Manage contexts",
	Long:  `Manage geartest contexts for different environments.`,
}

// contextListCmd lists all contexts
var contextListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all contexts",
	RunE: func(cmd *cobra.Command, args []string) error {
		names := globalConfig.ListContexts()
		if len(names) == 0 {
			fmt.Println("No contexts configured.")
			fmt.Println("\nCreate one with:")
			fmt.Println("  geartest config context set dev --gear-id=<id> --mqtt=mqtt://localhost:1883")
			return nil
		}

		sort.Strings(names)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "CURRENT\tNAME\tGEAR_ID\tMQTT_URL")

		for _, name := range names {
			ctx, _ := globalConfig.GetContext(name)
			cfg := LoadGearConfig(ctx)

			current := ""
			if name == globalConfig.CurrentContext {
				current = "*"
			}

			gearID := cfg.GearID
			if gearID == "" {
				gearID = "(not set)"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", current, name, gearID, cfg.MQTTURL)
		}
		w.Flush()

		return nil
	},
}

// contextUseCmd switches the current context
var contextUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch to a context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := globalConfig.UseContext(name); err != nil {
			return err
		}
		fmt.Printf("Switched to context %q\n", name)
		return nil
	},
}

// contextSetCmd creates or updates a context
var contextSetCmd = &cobra.Command{
	Use:   "set <name>",
	Short: "Create or update a context",
	Long: `Create or update a context with the specified settings.

Examples:
  # Create a new context
  geartest config context set dev --gear-id=abc123 --mqtt=mqtt://localhost:1883

  # Update an existing context
  geartest config context set dev --gear-id=new-id

  # Set additional options
  geartest config context set staging --gear-id=staging-001 --mqtt=mqtt://staging.example.com:1883 --namespace=staging/ --web=8089`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Get existing context or create new one
		ctx, err := globalConfig.GetContext(name)
		if err != nil {
			ctx = &cli.Context{Name: name}
		}

		// Load existing gear config
		gearCfg := LoadGearConfig(ctx)

		// Apply flags
		if cmd.Flags().Changed("gear-id") {
			gearCfg.GearID, _ = cmd.Flags().GetString("gear-id")
		}
		if cmd.Flags().Changed("mqtt") {
			gearCfg.MQTTURL, _ = cmd.Flags().GetString("mqtt")
		}
		if cmd.Flags().Changed("namespace") {
			gearCfg.Namespace, _ = cmd.Flags().GetString("namespace")
		}
		if cmd.Flags().Changed("web") {
			gearCfg.WebPort, _ = cmd.Flags().GetInt("web")
		}
		if cmd.Flags().Changed("version") {
			gearCfg.SysVersion, _ = cmd.Flags().GetString("version")
		}

		// Save gear config to context
		SaveGearConfig(ctx, gearCfg)

		// Add or update context
		if err := globalConfig.AddContext(name, ctx); err != nil {
			return err
		}

		fmt.Printf("Context %q saved\n", name)
		return nil
	},
}

// contextDeleteCmd deletes a context
var contextDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := globalConfig.DeleteContext(name); err != nil {
			return err
		}
		fmt.Printf("Context %q deleted\n", name)
		return nil
	},
}

// contextShowCmd shows the current context details
var contextShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show context details",
	Long:  `Show details of a context. If no name is provided, shows the current context.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var ctx *cli.Context
		var err error
		var name string

		if len(args) > 0 {
			name = args[0]
			ctx, err = globalConfig.GetContext(name)
		} else {
			if globalConfig.CurrentContext == "" {
				return fmt.Errorf("no current context set. Use 'geartest config context use <name>' to set one")
			}
			name = globalConfig.CurrentContext
			ctx, err = globalConfig.GetCurrentContext()
		}
		if err != nil {
			return err
		}

		cfg := LoadGearConfig(ctx)

		fmt.Printf("Context: %s", name)
		if name == globalConfig.CurrentContext {
			fmt.Print(" (current)")
		}
		fmt.Println()
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf("Gear ID:      %s\n", valueOrNotSet(cfg.GearID))
		fmt.Printf("MQTT URL:     %s\n", cfg.MQTTURL)
		fmt.Printf("Namespace:    %s\n", valueOrNotSet(cfg.Namespace))
		fmt.Printf("Web Port:     %d\n", cfg.WebPort)
		fmt.Printf("Sys Version:  %s\n", cfg.SysVersion)
		fmt.Println()
		fmt.Printf("Config file: %s\n", globalConfig.Path())

		return nil
	},
}

// contextCurrentCmd shows the current context name
var contextCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show current context name",
	RunE: func(cmd *cobra.Command, args []string) error {
		if globalConfig.CurrentContext == "" {
			fmt.Println("No current context set")
			return nil
		}
		fmt.Println(globalConfig.CurrentContext)
		return nil
	},
}

func valueOrNotSet(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

func init() {
	// Add context subcommand to config
	configCmd.AddCommand(contextCmd)

	// Add context commands
	contextCmd.AddCommand(contextListCmd)
	contextCmd.AddCommand(contextUseCmd)
	contextCmd.AddCommand(contextSetCmd)
	contextCmd.AddCommand(contextDeleteCmd)
	contextCmd.AddCommand(contextShowCmd)
	contextCmd.AddCommand(contextCurrentCmd)

	// Flags for context set
	contextSetCmd.Flags().String("gear-id", "", "gear ID to simulate")
	contextSetCmd.Flags().String("mqtt", "", "MQTT broker URL")
	contextSetCmd.Flags().String("namespace", "", "MQTT topic namespace")
	contextSetCmd.Flags().Int("web", 8088, "web control panel port")
	contextSetCmd.Flags().String("version", "0_zh", "system version")
}
