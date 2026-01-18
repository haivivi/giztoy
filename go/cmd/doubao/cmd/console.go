package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Console management service",
	Long: `Console management service.

Manage API keys, voices, services, and monitor usage.`,
}

// ==================== Timbre (Voice) Commands ====================

var consoleTimbreCmd = &cobra.Command{
	Use:   "timbre",
	Short: "Voice timbre management",
	Long:  `Manage voice timbres available in your account.`,
}

var consoleTimbreListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available timbres",
	Long: `List all available voice timbres.

Examples:
  doubao -c myctx console timbre list
  doubao -c myctx console timbre list --page 1 --size 20`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := getContext()
		if err != nil {
			return err
		}

		page, _ := cmd.Flags().GetInt("page")
		size, _ := cmd.Flags().GetInt("size")

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Page: %d, Size: %d", page, size)

		// TODO: Implement timbre list API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"timbres": []map[string]any{
				{"id": "zh_female_cancan", "name": "灿灿", "language": "zh-CN", "status": "active"},
				{"id": "zh_male_yangguang", "name": "阳光", "language": "zh-CN", "status": "active"},
				{"id": "en_female_sweet", "name": "Sweet", "language": "en-US", "status": "active"},
			},
			"page":  page,
			"size":  size,
			"total": 100,
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var consoleSpeakerListCmd = &cobra.Command{
	Use:   "speaker",
	Short: "List speakers by language",
	Long: `List speakers filtered by language.

Examples:
  doubao -c myctx console timbre speaker --language zh-CN
  doubao -c myctx console timbre speaker --language en-US --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := getContext()
		if err != nil {
			return err
		}

		language, _ := cmd.Flags().GetString("language")

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Language filter: %s", language)

		// TODO: Implement speaker list API call
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"language": language,
			"speakers": []map[string]any{
				{"id": "zh_female_cancan", "name": "灿灿", "style": "温柔"},
				{"id": "zh_male_yangguang", "name": "阳光", "style": "活力"},
			},
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

// ==================== API Key Commands ====================

var consoleAPIKeyCmd = &cobra.Command{
	Use:   "apikey",
	Short: "API key management",
	Long:  `Manage API keys for your account.`,
}

var consoleAPIKeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys",
	Long: `List all API keys in your account.

Examples:
  doubao -c myctx console apikey list
  doubao -c myctx console apikey list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)

		// TODO: Implement API key list
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"apikeys": []map[string]any{
				{"id": "key_1", "name": "Production", "status": "active", "created_at": "2024-01-01"},
				{"id": "key_2", "name": "Development", "status": "active", "created_at": "2024-01-02"},
			},
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var consoleAPIKeyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key",
	Long: `Create a new API key.

Examples:
  doubao -c myctx console apikey create --name "My API Key"
  doubao -c myctx console apikey create --name "Temp Key" --expires 2024-12-31`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := getContext()
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")
		expires, _ := cmd.Flags().GetString("expires")

		if name == "" {
			return fmt.Errorf("--name is required")
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Creating API key: %s", name)

		// TODO: Implement API key creation
		result := map[string]any{
			"_note":      "API call not implemented yet",
			"_context":   ctx.Name,
			"name":       name,
			"expires":    expires,
			"api_key":    "dk-xxxxxxxxxxxxxxxxxxxxxxxx",
			"created_at": "2024-01-01T10:00:00Z",
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var consoleAPIKeyDeleteCmd = &cobra.Command{
	Use:   "delete <apikey_id>",
	Short: "Delete an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		apiKeyID := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Deleting API key: %s", apiKeyID)

		// TODO: Implement API key deletion
		fmt.Printf("[Not implemented] Would delete API key: %s\n", apiKeyID)

		return nil
	},
}

var consoleAPIKeyUpdateCmd = &cobra.Command{
	Use:   "update <apikey_id>",
	Short: "Update an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		apiKeyID := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")
		status, _ := cmd.Flags().GetString("status")

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Updating API key: %s", apiKeyID)

		// TODO: Implement API key update
		result := map[string]any{
			"_note":     "API call not implemented yet",
			"_context":  ctx.Name,
			"apikey_id": apiKeyID,
			"name":      name,
			"status":    status,
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

// ==================== Service Commands ====================

var consoleServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Service management",
	Long:  `Manage services in your account.`,
}

var consoleServiceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show service status",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)

		// TODO: Implement service status
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SERVICE\tSTATUS\tQUOTA\tUSED")
		fmt.Fprintln(w, "TTS\tactive\t1,000,000\t50,000")
		fmt.Fprintln(w, "ASR\tactive\t500,000\t25,000")
		fmt.Fprintln(w, "Voice Clone\tactive\t10\t2")
		fmt.Fprintln(w, "Realtime\tactive\t100 hours\t5 hours")
		w.Flush()

		return nil
	},
}

var consoleServiceActivateCmd = &cobra.Command{
	Use:   "activate <service_id>",
	Short: "Activate a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceID := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)

		// TODO: Implement service activation
		fmt.Printf("[Not implemented] Would activate service: %s\n", serviceID)

		return nil
	},
}

var consoleServicePauseCmd = &cobra.Command{
	Use:   "pause <service_id>",
	Short: "Pause a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceID := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)

		// TODO: Implement service pause
		fmt.Printf("[Not implemented] Would pause service: %s\n", serviceID)

		return nil
	},
}

var consoleServiceResumeCmd = &cobra.Command{
	Use:   "resume <service_id>",
	Short: "Resume a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceID := args[0]

		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)

		// TODO: Implement service resume
		fmt.Printf("[Not implemented] Would resume service: %s\n", serviceID)

		return nil
	},
}

// ==================== Monitoring Commands ====================

var consoleQuotaCmd = &cobra.Command{
	Use:   "quota",
	Short: "Show quota information",
	Long: `Show quota information for services.

Examples:
  doubao -c myctx console quota
  doubao -c myctx console quota --service tts`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := getContext()
		if err != nil {
			return err
		}

		serviceID, _ := cmd.Flags().GetString("service")

		printVerbose("Using context: %s", ctx.Name)
		if serviceID != "" {
			printVerbose("Service filter: %s", serviceID)
		}

		// TODO: Implement quota query
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"quotas": []map[string]any{
				{"service": "TTS", "total": 1000000, "used": 50000, "remaining": 950000},
				{"service": "ASR", "total": 500000, "used": 25000, "remaining": 475000},
			},
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var consoleUsageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show usage statistics",
	Long: `Show usage statistics for a time period.

Examples:
  doubao -c myctx console usage --start 2024-01-01 --end 2024-01-31
  doubao -c myctx console usage --start 2024-01-01 --end 2024-01-31 --granularity day`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := getContext()
		if err != nil {
			return err
		}

		start, _ := cmd.Flags().GetString("start")
		end, _ := cmd.Flags().GetString("end")
		granularity, _ := cmd.Flags().GetString("granularity")

		if start == "" || end == "" {
			return fmt.Errorf("--start and --end are required")
		}

		printVerbose("Using context: %s", ctx.Name)
		printVerbose("Period: %s to %s", start, end)
		printVerbose("Granularity: %s", granularity)

		// TODO: Implement usage query
		result := map[string]any{
			"_note":       "API call not implemented yet",
			"_context":    ctx.Name,
			"start":       start,
			"end":         end,
			"granularity": granularity,
			"data": []map[string]any{
				{"date": "2024-01-01", "tts_chars": 10000, "asr_seconds": 3600},
				{"date": "2024-01-02", "tts_chars": 15000, "asr_seconds": 5400},
			},
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

var consoleQPSCmd = &cobra.Command{
	Use:   "qps",
	Short: "Show current QPS",
	Long: `Show current queries per second.

Examples:
  doubao -c myctx console qps`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := getContext()
		if err != nil {
			return err
		}

		printVerbose("Using context: %s", ctx.Name)

		// TODO: Implement QPS query
		result := map[string]any{
			"_note":    "API call not implemented yet",
			"_context": ctx.Name,
			"qps": map[string]any{
				"tts":      5.2,
				"asr":      3.1,
				"realtime": 0.5,
			},
			"limits": map[string]any{
				"tts":      100,
				"asr":      50,
				"realtime": 10,
			},
		}

		return outputResult(result, getOutputFile(), isJSONOutput())
	},
}

func init() {
	// Timbre commands
	consoleTimbreListCmd.Flags().Int("page", 1, "Page number")
	consoleTimbreListCmd.Flags().Int("size", 20, "Page size")
	consoleSpeakerListCmd.Flags().String("language", "", "Language filter (e.g., zh-CN, en-US)")

	consoleTimbreCmd.AddCommand(consoleTimbreListCmd)
	consoleTimbreCmd.AddCommand(consoleSpeakerListCmd)

	// API Key commands
	consoleAPIKeyCreateCmd.Flags().String("name", "", "API key name (required)")
	consoleAPIKeyCreateCmd.Flags().String("expires", "", "Expiration date (optional)")
	consoleAPIKeyUpdateCmd.Flags().String("name", "", "New name")
	consoleAPIKeyUpdateCmd.Flags().String("status", "", "New status (active/inactive)")

	consoleAPIKeyCmd.AddCommand(consoleAPIKeyListCmd)
	consoleAPIKeyCmd.AddCommand(consoleAPIKeyCreateCmd)
	consoleAPIKeyCmd.AddCommand(consoleAPIKeyDeleteCmd)
	consoleAPIKeyCmd.AddCommand(consoleAPIKeyUpdateCmd)

	// Service commands
	consoleServiceCmd.AddCommand(consoleServiceStatusCmd)
	consoleServiceCmd.AddCommand(consoleServiceActivateCmd)
	consoleServiceCmd.AddCommand(consoleServicePauseCmd)
	consoleServiceCmd.AddCommand(consoleServiceResumeCmd)

	// Monitoring commands
	consoleQuotaCmd.Flags().String("service", "", "Service filter")
	consoleUsageCmd.Flags().String("start", "", "Start date (required)")
	consoleUsageCmd.Flags().String("end", "", "End date (required)")
	consoleUsageCmd.Flags().String("granularity", "day", "Granularity (hour/day/month)")

	// Add all to console
	consoleCmd.AddCommand(consoleTimbreCmd)
	consoleCmd.AddCommand(consoleAPIKeyCmd)
	consoleCmd.AddCommand(consoleServiceCmd)
	consoleCmd.AddCommand(consoleQuotaCmd)
	consoleCmd.AddCommand(consoleUsageCmd)
	consoleCmd.AddCommand(consoleQPSCmd)
}
