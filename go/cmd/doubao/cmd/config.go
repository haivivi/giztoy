package cmd

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

Configuration is stored in ~/.giztoy/doubao/config.yaml`,
}

var configAddContextCmd = &cobra.Command{
	Use:   "add-context <name>",
	Short: "Add a new context",
	Long: `Add a new context with the specified name.

Doubao Speech API requires:
  - Bearer Token: For authentication
  - App ID: Your application ID
  - Cluster: Service cluster (volcano_tts, volcano_asr, volcano_icl, etc.)

Example:
  doubao config add-context myctx --token YOUR_TOKEN --app-id YOUR_APP_ID --cluster volcano_tts
  doubao config add-context prod --token TOKEN --app-id APPID --cluster volcano_tts --default-voice zh_female_cancan`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		token, err := cmd.Flags().GetString("token")
		if err != nil {
			return fmt.Errorf("failed to read 'token' flag: %w", err)
		}
		if token == "" {
			return fmt.Errorf("--token is required")
		}

		appID, err := cmd.Flags().GetString("app-id")
		if err != nil {
			return fmt.Errorf("failed to read 'app-id' flag: %w", err)
		}
		if appID == "" {
			return fmt.Errorf("--app-id is required")
		}

		cluster, err := cmd.Flags().GetString("cluster")
		if err != nil {
			return fmt.Errorf("failed to read 'cluster' flag: %w", err)
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

		defaultVoice, err := cmd.Flags().GetString("default-voice")
		if err != nil {
			return fmt.Errorf("failed to read 'default-voice' flag: %w", err)
		}

		userID, err := cmd.Flags().GetString("user-id")
		if err != nil {
			return fmt.Errorf("failed to read 'user-id' flag: %w", err)
		}

		ctx := &cli.Context{
			APIKey:     token, // Store token as APIKey field
			BaseURL:    baseURL,
			Timeout:    timeout,
			MaxRetries: maxRetries,
		}

		// Store Doubao-specific settings in Extra
		ctx.SetExtra("app_id", appID)
		if cluster != "" {
			ctx.SetExtra("cluster", cluster)
		}
		if defaultVoice != "" {
			ctx.SetExtra("default_voice", defaultVoice)
		}
		if userID != "" {
			ctx.SetExtra("user_id", userID)
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
		fmt.Fprintln(w, "CURRENT\tNAME\tCLUSTER\tDEFAULT_VOICE")

		for name, ctx := range cfg.Contexts {
			current := ""
			if name == cfg.CurrentContext {
				current = "*"
			}
			cluster := ctx.GetExtra("cluster")
			if cluster == "" {
				cluster = "(default)"
			}
			defaultVoice := ctx.GetExtra("default_voice")
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", current, name, cluster, defaultVoice)
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
				fmt.Printf("    Token: %s\n", cli.MaskAPIKey(ctx.APIKey))
				if appID := ctx.GetExtra("app_id"); appID != "" {
					fmt.Printf("    App ID: %s\n", appID)
				}
				if cluster := ctx.GetExtra("cluster"); cluster != "" {
					fmt.Printf("    Cluster: %s\n", cluster)
				}
				if ctx.BaseURL != "" {
					fmt.Printf("    Base URL: %s\n", ctx.BaseURL)
				}
				if ctx.Timeout > 0 {
					fmt.Printf("    Timeout: %ds\n", ctx.Timeout)
				}
				if defaultVoice := ctx.GetExtra("default_voice"); defaultVoice != "" {
					fmt.Printf("    Default Voice: %s\n", defaultVoice)
				}
				if userID := ctx.GetExtra("user_id"); userID != "" {
					fmt.Printf("    User ID: %s\n", userID)
				}
			}
		}

		return nil
	},
}

func init() {
	// add-context flags
	configAddContextCmd.Flags().String("token", "", "Bearer token (required)")
	configAddContextCmd.Flags().String("app-id", "", "Application ID (required)")
	configAddContextCmd.Flags().String("cluster", "", "Service cluster (volcano_tts, volcano_asr, etc.)")
	configAddContextCmd.Flags().String("base-url", "", "API base URL")
	configAddContextCmd.Flags().Int("timeout", 0, "Request timeout in seconds")
	configAddContextCmd.Flags().Int("max-retries", 0, "Maximum retries")
	configAddContextCmd.Flags().String("default-voice", "", "Default voice type")
	configAddContextCmd.Flags().String("user-id", "", "User ID for tracking")

	// Add subcommands
	configCmd.AddCommand(configAddContextCmd)
	configCmd.AddCommand(configDeleteContextCmd)
	configCmd.AddCommand(configUseContextCmd)
	configCmd.AddCommand(configGetContextCmd)
	configCmd.AddCommand(configListContextsCmd)
	configCmd.AddCommand(configViewCmd)
}
