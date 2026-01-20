package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/goccy/go-yaml"
)

// OutputFormat represents the output format type
type OutputFormat string

const (
	// FormatYAML outputs as YAML (default for terminal)
	FormatYAML OutputFormat = "yaml"
	// FormatJSON outputs as JSON
	FormatJSON OutputFormat = "json"
	// FormatTable outputs as formatted table
	FormatTable OutputFormat = "table"
	// FormatRaw outputs raw data
	FormatRaw OutputFormat = "raw"
)

// OutputOptions configures output behavior
type OutputOptions struct {
	// Format is the output format (yaml, json, table, raw)
	Format OutputFormat

	// File is the output file path (empty for stdout)
	File string

	// Indent is the indentation for JSON output
	Indent string

	// Writer is an optional custom writer (overrides File)
	Writer io.Writer
}

// Output writes the result to the configured destination
func Output(result any, opts OutputOptions) error {
	var w io.Writer = os.Stdout

	if opts.Writer != nil {
		w = opts.Writer
	} else if opts.File != "" {
		f, err := os.Create(opts.File)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		w = f
	}

	switch opts.Format {
	case FormatJSON:
		return outputJSON(w, result, opts.Indent)
	case FormatYAML, "":
		return outputYAML(w, result)
	case FormatRaw:
		return outputRaw(w, result)
	default:
		return fmt.Errorf("unsupported output format: %s", opts.Format)
	}
}

func outputJSON(w io.Writer, result any, indent string) error {
	enc := json.NewEncoder(w)
	if indent == "" {
		indent = "  "
	}
	enc.SetIndent("", indent)
	return enc.Encode(result)
}

func outputYAML(w io.Writer, result any) error {
	data, err := yaml.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}
	_, err = w.Write(data)
	return err
}

func outputRaw(w io.Writer, result any) error {
	switch v := result.(type) {
	case []byte:
		_, err := w.Write(v)
		return err
	case string:
		_, err := w.Write([]byte(v))
		return err
	default:
		return outputYAML(w, result)
	}
}

// OutputBytes writes binary data to a file
func OutputBytes(data []byte, path string) error {
	if path == "" {
		return fmt.Errorf("output file path is required for binary data")
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}

// Print helpers for terminal output

// PrintSuccess prints a success message with checkmark
func PrintSuccess(format string, args ...any) {
	fmt.Printf("✓ "+format+"\n", args...)
}

// PrintError prints an error message to stderr
func PrintError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

// PrintInfo prints an info message
func PrintInfo(format string, args ...any) {
	fmt.Printf("ℹ "+format+"\n", args...)
}

// PrintWarning prints a warning message
func PrintWarning(format string, args ...any) {
	fmt.Printf("⚠ "+format+"\n", args...)
}

// PrintVerbose prints verbose output to stderr
func PrintVerbose(verbose bool, format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] "+format+"\n", args...)
	}
}
