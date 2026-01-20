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

Configuration is stored in ~/.giztoy/doubao/config.yaml`,
}

var configAddContextCmd = &cobra.Command{
	Use:   "add-context <name>",
	Short: "Add a new context",
	Long: `Add a new context with the specified name.

Doubao Speech API (Client) requires:
  - App ID: Your application ID
  - API Key: For authentication (Bearer token)

Console API (optional) requires:
  - Access Key: Volcengine AK
  - Secret Key: Volcengine SK

Note: cluster is NOT stored in context, it should be specified in request YAML files.

Example:
  # Add context with client credentials only
  doubao config add-context myctx --app-id YOUR_APP_ID --api-key YOUR_API_KEY

  # Add context with both client and console credentials
  doubao config add-context prod \
    --app-id YOUR_APP_ID --api-key YOUR_API_KEY \
    --console-ak YOUR_AK --console-sk YOUR_SK \
    --default-voice zh_female_cancan`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Client credentials (required)
		apiKey, err := cmd.Flags().GetString("api-key")
		if err != nil {
			return fmt.Errorf("failed to read 'api-key' flag: %w", err)
		}
		// Support legacy --token flag
		if apiKey == "" {
			apiKey, _ = cmd.Flags().GetString("token")
		}
		if apiKey == "" {
			return fmt.Errorf("--api-key is required")
		}

		appID, err := cmd.Flags().GetString("app-id")
		if err != nil {
			return fmt.Errorf("failed to read 'app-id' flag: %w", err)
		}
		if appID == "" {
			return fmt.Errorf("--app-id is required")
		}

		// Console credentials (optional)
		consoleAK, err := cmd.Flags().GetString("console-ak")
		if err != nil {
			return fmt.Errorf("failed to read 'console-ak' flag: %w", err)
		}
		consoleSK, err := cmd.Flags().GetString("console-sk")
		if err != nil {
			return fmt.Errorf("failed to read 'console-sk' flag: %w", err)
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

		ctx := &cli.Context{
			Client: &cli.ClientCredentials{
				AppID:  appID,
				APIKey: apiKey,
			},
			BaseURL:      baseURL,
			Timeout:      timeout,
			MaxRetries:   maxRetries,
			DefaultVoice: defaultVoice,
		}

		// Add console credentials if provided
		if consoleAK != "" && consoleSK != "" {
			ctx.Console = &cli.ConsoleCredentials{
				AccessKey: consoleAK,
				SecretKey: consoleSK,
			}
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
		fmt.Fprintln(w, "CURRENT\tNAME\tCLIENT\tCONSOLE\tDEFAULT_VOICE")

		for name, ctx := range cfg.Contexts {
			current := ""
			if name == cfg.CurrentContext {
				current = "*"
			}
			clientStatus := "✗"
			if ctx.Client != nil && ctx.Client.AppID != "" && ctx.Client.APIKey != "" {
				clientStatus = "✓"
			}
			consoleStatus := "✗"
			if ctx.Console != nil && ctx.Console.AccessKey != "" && ctx.Console.SecretKey != "" {
				consoleStatus = "✓"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", current, name, clientStatus, consoleStatus, ctx.DefaultVoice)
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

				// Client credentials
				if ctx.Client != nil {
					fmt.Println("    Client (Speech API):")
					fmt.Printf("      App ID: %s\n", ctx.Client.AppID)
					fmt.Printf("      API Key: %s\n", cli.MaskAPIKey(ctx.Client.APIKey))
				}

				// Console credentials
				if ctx.Console != nil {
					fmt.Println("    Console (OpenAPI):")
					fmt.Printf("      Access Key: %s\n", cli.MaskAPIKey(ctx.Console.AccessKey))
					fmt.Printf("      Secret Key: %s\n", cli.MaskAPIKey(ctx.Console.SecretKey))
				}

				// Optional settings
				if ctx.BaseURL != "" {
					fmt.Printf("    Base URL: %s\n", ctx.BaseURL)
				}
				if ctx.Timeout > 0 {
					fmt.Printf("    Timeout: %ds\n", ctx.Timeout)
				}
				if ctx.DefaultVoice != "" {
					fmt.Printf("    Default Voice: %s\n", ctx.DefaultVoice)
				}
			}
		}

		return nil
	},
}

func init() {
	// add-context flags - Client credentials (required)
	configAddContextCmd.Flags().String("api-key", "", "API key for speech APIs (required)")
	configAddContextCmd.Flags().String("token", "", "Bearer token (deprecated, use --api-key)")
	configAddContextCmd.Flags().String("app-id", "", "Application ID (required)")

	// add-context flags - Console credentials (optional)
	configAddContextCmd.Flags().String("console-ak", "", "Volcengine Access Key for Console API")
	configAddContextCmd.Flags().String("console-sk", "", "Volcengine Secret Key for Console API")

	// add-context flags - Optional settings
	configAddContextCmd.Flags().String("base-url", "", "API base URL")
	configAddContextCmd.Flags().Int("timeout", 0, "Request timeout in seconds")
	configAddContextCmd.Flags().Int("max-retries", 0, "Maximum retries")
	configAddContextCmd.Flags().String("default-voice", "", "Default voice type")

	// Hide deprecated flag
	configAddContextCmd.Flags().MarkDeprecated("token", "use --api-key instead")

	// Add subcommands
	configCmd.AddCommand(configAddContextCmd)
	configCmd.AddCommand(configDeleteContextCmd)
	configCmd.AddCommand(configUseContextCmd)
	configCmd.AddCommand(configGetContextCmd)
	configCmd.AddCommand(configListContextsCmd)
	configCmd.AddCommand(configViewCmd)
}
