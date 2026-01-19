package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/pkg/cli"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
	Long: `Manage DashScope CLI configuration.

Configuration is stored in ~/.giztoy/dashscope/config.yaml.
Multiple contexts can be defined for different accounts or environments.`,
}

var configAddContextCmd = &cobra.Command{
	Use:   "add-context <name>",
	Short: "Add a new context",
	Long: `Add a new context with API credentials.

Examples:
  dashscope config add-context myctx --api-key sk-xxxxx
  dashscope config add-context myctx --api-key sk-xxxxx --workspace ws-xxxxx`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		apiKey, _ := cmd.Flags().GetString("api-key")
		baseURL, _ := cmd.Flags().GetString("base-url")
		workspace, _ := cmd.Flags().GetString("workspace")

		if apiKey == "" {
			return fmt.Errorf("api-key is required")
		}

		ctx := &cli.Context{
			Name:   name,
			APIKey: apiKey,
		}
		if baseURL != "" {
			ctx.BaseURL = baseURL
		}
		if workspace != "" {
			if ctx.Extra == nil {
				ctx.Extra = make(map[string]string)
			}
			ctx.Extra["workspace"] = workspace
		}

		cfg := getConfig()
		if err := cfg.AddContext(name, ctx); err != nil {
			return err
		}

		cli.PrintSuccess("Context '%s' added successfully", name)
		return nil
	},
}

var configDeleteContextCmd = &cobra.Command{
	Use:   "delete-context <name>",
	Short: "Delete a context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg := getConfig()
		if err := cfg.DeleteContext(name); err != nil {
			return err
		}
		cli.PrintSuccess("Context '%s' deleted", name)
		return nil
	},
}

var configUseContextCmd = &cobra.Command{
	Use:   "use-context <name>",
	Short: "Set the default context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg := getConfig()
		if err := cfg.UseContext(name); err != nil {
			return err
		}
		cli.PrintSuccess("Switched to context '%s'", name)
		return nil
	},
}

var configGetContextCmd = &cobra.Command{
	Use:   "get-context",
	Short: "Show the current context",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := getConfig()
		if cfg.CurrentContext == "" {
			fmt.Println("No current context set")
		} else {
			fmt.Println(cfg.CurrentContext)
		}
		return nil
	},
}

var configListContextsCmd = &cobra.Command{
	Use:   "list-contexts",
	Short: "List all contexts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := getConfig()
		if len(cfg.Contexts) == 0 {
			fmt.Println("No contexts configured")
			return nil
		}

		for name := range cfg.Contexts {
			marker := "  "
			if name == cfg.CurrentContext {
				marker = "* "
			}
			fmt.Printf("%s%s\n", marker, name)
		}
		return nil
	},
}

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "View full configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := getConfig()
		return outputResult(cfg, "", isJSONOutput())
	},
}

func init() {
	// add-context flags
	configAddContextCmd.Flags().StringP("api-key", "k", "", "API key (required)")
	configAddContextCmd.Flags().StringP("base-url", "u", "", "Base URL (default: wss://dashscope.aliyuncs.com)")
	configAddContextCmd.Flags().StringP("workspace", "w", "", "Workspace ID for resource isolation")

	configCmd.AddCommand(configAddContextCmd)
	configCmd.AddCommand(configDeleteContextCmd)
	configCmd.AddCommand(configUseContextCmd)
	configCmd.AddCommand(configGetContextCmd)
	configCmd.AddCommand(configListContextsCmd)
	configCmd.AddCommand(configViewCmd)
}
