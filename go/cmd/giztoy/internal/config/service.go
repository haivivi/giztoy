package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

// ServicePath returns the YAML file path for a service within a context.
// For example, ServicePath("dev", "minimax") → ".../contexts/dev/minimax.yaml".
func (c *Config) ServicePath(context, service string) string {
	return filepath.Join(c.ContextDir(context), service+".yaml")
}

// LoadService loads a service configuration from the given context directory.
// The service name maps to a YAML file: "{contextDir}/{service}.yaml".
func LoadService[T any](contextDir, service string) (*T, error) {
	path := filepath.Join(contextDir, service+".yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("service config %q not found in context (expected: %s)", service, path)
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var v T
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &v, nil
}

// SaveService writes a service configuration to the given context directory.
func SaveService[T any](contextDir, service string, v *T) error {
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		return fmt.Errorf("create context dir: %w", err)
	}

	path := filepath.Join(contextDir, service+".yaml")

	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal %s config: %w", service, err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// ListServices returns the service names configured in a context directory.
// Each .yaml file corresponds to one service.
func ListServices(contextDir string) ([]string, error) {
	entries, err := os.ReadDir(contextDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list services: %w", err)
	}

	var services []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := filepath.Ext(name)
		if ext == ".yaml" || ext == ".yml" {
			services = append(services, name[:len(name)-len(ext)])
		}
	}
	return services, nil
}
