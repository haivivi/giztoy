package commands

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/pkg/cli"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long: `Manage CLI configuration and contexts.

Contexts allow you to manage multiple API configurations,
similar to kubectl's context management.

Configuration is stored in ~/.giztoy/minimax/config.yaml`,
}

var configAddContextCmd = &cobra.Command{
	Use:   "add-context <name>",
	Short: "Add a new context",
	Long: `Add a new context with the specified name.

Example:
  minimax config add-context myctx --api-key YOUR_API_KEY
  minimax config add-context prod --api-key KEY --base-url https://api.minimaxi.com`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		apiKey, err := cmd.Flags().GetString("api-key")
		if err != nil {
			return fmt.Errorf("failed to read 'api-key' flag: %w", err)
		}
		if apiKey == "" {
			return fmt.Errorf("--api-key is required")
		}

		baseURL, err := cmd.Flags().GetString("base-url")
		if err != nil {
			return fmt.Errorf("failed to read 'base-url' flag: %w", err)
		}
		timeout, err := cmd.Flags().GetInt("timeout")
		if err != nil {
			return fmt.Errorf("failed to read 'timeout' flag: %w", err)
		}
		maxRetries, err := cmd.Flags().GetInt("max-retries")
		if err != nil {
			return fmt.Errorf("failed to read 'max-retries' flag: %w", err)
		}
		defaultModel, err := cmd.Flags().GetString("default-model")
		if err != nil {
			return fmt.Errorf("failed to read 'default-model' flag: %w", err)
		}
		defaultVoice, err := cmd.Flags().GetString("default-voice")
		if err != nil {
			return fmt.Errorf("failed to read 'default-voice' flag: %w", err)
		}

		ctx := &cli.Context{
			APIKey:     apiKey,
			BaseURL:    baseURL,
			Timeout:    timeout,
			MaxRetries: maxRetries,
		}

		// Store app-specific settings in Extra
		if defaultModel != "" {
			ctx.SetExtra("default_model", defaultModel)
		}
		if defaultVoice != "" {
			ctx.SetExtra("default_voice", defaultVoice)
		}

		cfg := getConfig()
		if err := cfg.AddContext(name, ctx); err != nil {
			return err
		}

		cli.PrintSuccess("Context %q added successfully", name)
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

		cli.PrintSuccess("Context %q deleted", name)
		return nil
	},
}

var configUseContextCmd = &cobra.Command{
	Use:   "use-context <name>",
	Short: "Set the current context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg := getConfig()
		if err := cfg.UseContext(name); err != nil {
			return err
		}

		cli.PrintSuccess("Switched to context %q", name)
		return nil
	},
}

var configGetContextCmd = &cobra.Command{
	Use:   "get-context",
	Short: "Display the current context",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := getConfig()

		if cfg.CurrentContext == "" {
			fmt.Println("No current context set")
			return nil
		}

		fmt.Println(cfg.CurrentContext)
		return nil
	},
}

var configListContextsCmd = &cobra.Command{
	Use:     "list-contexts",
	Aliases: []string{"get-contexts"},
	Short:   "List all contexts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := getConfig()

		if len(cfg.Contexts) == 0 {
			fmt.Println("No contexts configured")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "CURRENT\tNAME\tBASE_URL\tDEFAULT_MODEL")

		for name, ctx := range cfg.Contexts {
			current := ""
			if name == cfg.CurrentContext {
				current = "*"
			}
			baseURL := ctx.BaseURL
			if baseURL == "" {
				baseURL = "(default)"
			}
			defaultModel := ctx.GetExtra("default_model")
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", current, name, baseURL, defaultModel)
		}

		w.Flush()
		return nil
	},
}

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "View the current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := getConfig()

		fmt.Printf("Config file: %s\n", cfg.Path())
		fmt.Printf("Current context: %s\n", cfg.CurrentContext)
		fmt.Printf("Contexts: %d\n", len(cfg.Contexts))

		if len(cfg.Contexts) > 0 {
			fmt.Println("\nContext details:")
			for name, ctx := range cfg.Contexts {
				fmt.Printf("\n  %s:\n", name)
				fmt.Printf("    API Key: %s\n", cli.MaskAPIKey(ctx.APIKey))
				if ctx.BaseURL != "" {
					fmt.Printf("    Base URL: %s\n", ctx.BaseURL)
				}
				if ctx.Timeout > 0 {
					fmt.Printf("    Timeout: %ds\n", ctx.Timeout)
				}
				if defaultModel := ctx.GetExtra("default_model"); defaultModel != "" {
					fmt.Printf("    Default Model: %s\n", defaultModel)
				}
				if defaultVoice := ctx.GetExtra("default_voice"); defaultVoice != "" {
					fmt.Printf("    Default Voice: %s\n", defaultVoice)
				}
			}
		}

		return nil
	},
}

func init() {
	// add-context flags
	configAddContextCmd.Flags().String("api-key", "", "API key (required)")
	configAddContextCmd.Flags().String("base-url", "", "API base URL")
	configAddContextCmd.Flags().Int("timeout", 0, "Request timeout in seconds")
	configAddContextCmd.Flags().Int("max-retries", 0, "Maximum retries")
	configAddContextCmd.Flags().String("default-model", "", "Default model")
	configAddContextCmd.Flags().String("default-voice", "", "Default voice ID")

	// Add subcommands
	configCmd.AddCommand(configAddContextCmd)
	configCmd.AddCommand(configDeleteContextCmd)
	configCmd.AddCommand(configUseContextCmd)
	configCmd.AddCommand(configGetContextCmd)
	configCmd.AddCommand(configListContextsCmd)
	configCmd.AddCommand(configViewCmd)
}
