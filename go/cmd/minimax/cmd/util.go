package cmd

import (
	"fmt"

	"github.com/haivivi/giztoy/pkg/cli"
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
