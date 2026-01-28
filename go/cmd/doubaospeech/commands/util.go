package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/cli"
	ds "github.com/haivivi/giztoy/go/pkg/doubaospeech"
	"github.com/goccy/go-yaml"
)

// loadRequest loads a request from a YAML or JSON file
func loadRequest(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, v); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, v); err != nil {
			return fmt.Errorf("failed to parse YAML: %w", err)
		}
	default:
		// Try YAML first, then JSON
		if err := yaml.Unmarshal(data, v); err != nil {
			if err := json.Unmarshal(data, v); err != nil {
				return fmt.Errorf("failed to parse file (tried YAML and JSON): %w", err)
			}
		}
	}

	return nil
}

// requireInputFile checks if input file is specified
func requireInputFile() error {
	if getInputFile() == "" {
		return fmt.Errorf("input file is required, use -f flag")
	}
	return nil
}

// requireOutputFile checks if output file is specified
func requireOutputFile() error {
	if getOutputFile() == "" {
		return fmt.Errorf("output file is required, use -o flag")
	}
	return nil
}

// saveToFile saves data to a file
func saveToFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}
	return os.WriteFile(path, data, 0644)
}

// formatDuration formats duration in seconds to human readable format
func formatDuration(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", seconds)
	}
	if seconds < 3600 {
		mins := int(seconds / 60)
		secs := int(seconds) % 60
		return fmt.Sprintf("%dm%ds", mins, secs)
	}
	hours := int(seconds / 3600)
	mins := (int(seconds) % 3600) / 60
	return fmt.Sprintf("%dh%dm", hours, mins)
}

// formatBytes formats bytes to human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// createClient creates a Doubao Speech API client from context configuration
func createClient(ctx *cli.Context) (*ds.Client, error) {
	if ctx.Client == nil {
		return nil, fmt.Errorf("client credentials not configured, run: doubaospeech config add-context")
	}

	var opts []ds.Option

	// Set authentication - use Bearer Token by default
	// API Keys from console start with specific patterns (like UUID format)
	// Bearer Tokens are typically longer alphanumeric strings
	if ctx.Client.APIKey != "" {
		opts = append(opts, ds.WithBearerToken(ctx.Client.APIKey))
	}

	// Use custom base URL if configured
	if ctx.BaseURL != "" {
		opts = append(opts, ds.WithBaseURL(ctx.BaseURL))
	}

	return ds.NewClient(ctx.Client.AppID, opts...), nil
}

// createConsole creates a Doubao Console API client from context configuration
func createConsole(ctx *cli.Context) (*ds.Console, error) {
	if ctx.Console == nil {
		return nil, fmt.Errorf("console credentials not configured, add --console-ak and --console-sk")
	}

	return ds.NewConsole(ctx.Console.AccessKey, ctx.Console.SecretKey), nil
}

// outputBytes outputs binary data to a file
func outputBytes(data []byte, outputPath string) error {
	return cli.OutputBytes(data, outputPath)
}

// printSuccess prints a success message
func printSuccess(format string, args ...any) {
	cli.PrintSuccess(format, args...)
}

// printInfo prints an info message
func printInfo(format string, args ...any) {
	cli.PrintInfo(format, args...)
}
