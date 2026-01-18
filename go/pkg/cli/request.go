package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadRequest loads a request from a YAML or JSON file into the provided struct
func LoadRequest(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	return ParseRequest(data, path, v)
}

// ParseRequest parses request data based on file extension or content
func ParseRequest(data []byte, filename string, v any) error {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, v); err != nil {
			return fmt.Errorf("failed to parse YAML: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, v); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
	default:
		// Try YAML first, then JSON
		if err := yaml.Unmarshal(data, v); err != nil {
			if err2 := json.Unmarshal(data, v); err2 != nil {
				return fmt.Errorf("failed to parse file (tried YAML and JSON)")
			}
		}
	}

	return nil
}

// LoadRequestFromStdin loads a request from stdin
func LoadRequestFromStdin(v any) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}

	// Try JSON first for stdin, then YAML
	if err := json.Unmarshal(data, v); err != nil {
		if err2 := yaml.Unmarshal(data, v); err2 != nil {
			return fmt.Errorf("failed to parse input (tried JSON and YAML)")
		}
	}

	return nil
}

// MustLoadRequest loads a request or exits with error
func MustLoadRequest(path string, v any) {
	if err := LoadRequest(path, v); err != nil {
		PrintError("Failed to load request: %v", err)
		os.Exit(1)
	}
}
