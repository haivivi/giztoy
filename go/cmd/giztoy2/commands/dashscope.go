package commands

import "github.com/spf13/cobra"

var dashscopeCmd = &cobra.Command{
	Use:   "dashscope",
	Short: "DashScope API (Qwen-Omni multimodal chat)",
	Long: `DashScope (Aliyun Model Studio) API client.
Supports multiple app instances with different API keys and regions.

Usage: giztoy dashscope <app_name> <command...>

Examples:
  giztoy dashscope default omni chat
  giztoy dashscope intl omni chat --voice Cherry`,
}

func init() {
	addAppCommands(dashscopeCmd, "dashscope", []appField{
		{Name: "api_key", Flag: "api-key", Required: true, Desc: "DashScope API Key"},
		{Name: "workspace", Flag: "workspace", Required: false, Desc: "Workspace ID"},
		{Name: "base_url", Flag: "base-url", Required: false, Desc: "API base URL (domestic/intl)"},
	})
	rootCmd.AddCommand(dashscopeCmd)
}
