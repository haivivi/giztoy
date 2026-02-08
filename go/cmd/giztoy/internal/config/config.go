// Package config provides the unified configuration system for giztoy CLI.
//
// Configuration is stored under os.UserConfigDir()/giztoy/:
//
//	~/Library/Application Support/giztoy/   (macOS)
//	~/.config/giztoy/                       (Linux)
//	%AppData%/giztoy/                       (Windows)
//
// Layout:
//
//	giztoy/
//	├── current-context          # plain text: name of current context
//	└── contexts/
//	    ├── dev/
//	    │   ├── minimax.yaml
//	    │   ├── doubao.yaml
//	    │   ├── dashscope.yaml
//	    │   ├── cortex.yaml
//	    │   └── gear.yaml
//	    └── staging/
//	        └── ...
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// appDir is the directory name under os.UserConfigDir().
	appDir = "giztoy"

	// currentContextFile stores the name of the current context.
	currentContextFile = "current-context"

	// contextsDir is the subdirectory holding all context directories.
	contextsDir = "contexts"
)

// Config holds the root configuration state.
type Config struct {
	// Dir is the root configuration directory.
	Dir string

	// CurrentContext is the name of the active context.
	CurrentContext string
}

// Load loads the configuration from the default location.
func Load() (*Config, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine config directory: %w", err)
	}
	return LoadFrom(filepath.Join(base, appDir))
}

// LoadFrom loads the configuration from a specific root directory.
func LoadFrom(dir string) (*Config, error) {
	cfg := &Config{Dir: dir}

	// Read current-context file (optional — may not exist yet).
	data, err := os.ReadFile(filepath.Join(dir, currentContextFile))
	if err == nil {
		cfg.CurrentContext = strings.TrimSpace(string(data))
	}

	return cfg, nil
}

// ContextsDir returns the path to the contexts directory.
func (c *Config) ContextsDir() string {
	return filepath.Join(c.Dir, contextsDir)
}

// ContextDir returns the directory path for a named context.
func (c *Config) ContextDir(name string) string {
	return filepath.Join(c.Dir, contextsDir, name)
}

// CurrentContextDir returns the directory path for the current context.
// Returns an error if no current context is set.
func (c *Config) CurrentContextDir() (string, error) {
	if c.CurrentContext == "" {
		return "", fmt.Errorf("no current context set; use 'giztoy config use-context <name>'")
	}
	return c.ContextDir(c.CurrentContext), nil
}

// ResolveContext returns the directory for the given context name,
// or the current context if name is empty.
func (c *Config) ResolveContext(name string) (string, error) {
	if name == "" {
		return c.CurrentContextDir()
	}
	dir := c.ContextDir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", fmt.Errorf("context %q not found", name)
	}
	return dir, nil
}

// ListContexts returns the names of all available contexts.
func (c *Config) ListContexts() ([]string, error) {
	entries, err := os.ReadDir(c.ContextsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list contexts: %w", err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// AddContext creates a new context directory.
func (c *Config) AddContext(name string) error {
	if name == "" {
		return fmt.Errorf("context name cannot be empty")
	}

	dir := c.ContextDir(name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("context %q already exists", name)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create context %q: %w", name, err)
	}
	return nil
}

// DeleteContext removes a context directory and all its service configs.
func (c *Config) DeleteContext(name string) error {
	if name == "" {
		return fmt.Errorf("context name cannot be empty")
	}

	dir := c.ContextDir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("context %q not found", name)
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("delete context %q: %w", name, err)
	}

	// Clear current context if it was the deleted one.
	if c.CurrentContext == name {
		c.CurrentContext = ""
		return c.saveCurrentContext()
	}
	return nil
}

// UseContext switches the current context.
func (c *Config) UseContext(name string) error {
	if name == "" {
		return fmt.Errorf("context name cannot be empty")
	}

	dir := c.ContextDir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("context %q not found", name)
	}

	c.CurrentContext = name
	return c.saveCurrentContext()
}

// saveCurrentContext writes the current-context file.
func (c *Config) saveCurrentContext() error {
	if err := os.MkdirAll(c.Dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	path := filepath.Join(c.Dir, currentContextFile)
	return os.WriteFile(path, []byte(c.CurrentContext+"\n"), 0644)
}
