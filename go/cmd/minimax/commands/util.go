package commands

import (
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/cli"
	"github.com/haivivi/giztoy/go/pkg/minimax"
)

// loadRequest loads a request from a YAML or JSON file
func loadRequest(path string, v any) error {
	return cli.LoadRequest(path, v)
}

// outputBytes outputs binary data to a file
func outputBytes(data []byte, outputPath string) error {
	return cli.OutputBytes(data, outputPath)
}

// printError prints an error message to stderr
func printError(format string, args ...any) {
	cli.PrintError(format, args...)
}

// printSuccess prints a success message
func printSuccess(format string, args ...any) {
	cli.PrintSuccess(format, args...)
}

// printInfo prints an info message
func printInfo(format string, args ...any) {
	cli.PrintInfo(format, args...)
}

// requireInputFile checks if input file is provided
func requireInputFile() error {
	if getInputFile() == "" {
		return fmt.Errorf("input file is required, use -f flag")
	}
	return nil
}

// formatDuration formats milliseconds to human readable string
func formatDuration(ms int) string {
	return cli.FormatDuration(ms)
}

// formatBytes formats bytes to human readable string
func formatBytes(bytes int) string {
	return cli.FormatBytesInt(bytes)
}

// createClient creates a MiniMax API client from context configuration
func createClient(ctx *cli.Context) *minimax.Client {
	var opts []minimax.Option
	
	// Use custom base URL if configured
	if ctx.BaseURL != "" {
		opts = append(opts, minimax.WithBaseURL(ctx.BaseURL))
	}
	
	// Use custom retry count if configured
	if ctx.MaxRetries > 0 {
		opts = append(opts, minimax.WithRetry(ctx.MaxRetries))
	}
	
	return minimax.NewClient(ctx.APIKey, opts...)
}
