// Package cli provides common CLI utilities for giztoy command-line tools.
//
// This package includes:
//   - Configuration management (contexts, profiles)
//   - Output formatting (JSON, YAML, table)
//   - Request file loading (YAML/JSON)
//   - Common flags and options
//
// Configuration is stored in ~/.giztoy/<app>/ directory, supporting
// multiple contexts similar to kubectl.
//
// Example usage:
//
//	// Initialize config for your app
//	cfg, err := cli.LoadConfig("minimax")
//
//	// Get current context
//	ctx, err := cfg.CurrentContext()
//
//	// Output result
//	cli.Output(result, cli.OutputOptions{
//	    Format: cli.FormatJSON,
//	    File:   outputPath,
//	})
package cli
