package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

const (
	// DefaultBaseDir is the base configuration directory name
	DefaultBaseDir = ".giztoy"
	// DefaultConfigFile is the default configuration filename
	DefaultConfigFile = "config.yaml"
)

// Config represents the main configuration structure for a CLI app
type Config struct {
	// AppName is the application name (e.g., "minimax", "doubao")
	AppName string `yaml:"-"`

	// CurrentContext is the name of the currently active context
	CurrentContext string `yaml:"current_context,omitempty"`

	// Contexts is a map of context name to context configuration
	Contexts map[string]*Context `yaml:"contexts,omitempty"`

	// configPath is the path to the config file
	configPath string
}

// Context represents a single API context configuration
type Context struct {
	// Name is the context name
	Name string `yaml:"name"`

	// Client contains speech API credentials (TTS, ASR, etc.) - used by doubao
	Client *ClientCredentials `yaml:"client,omitempty"`

	// Console contains console API credentials (ListTimbres, etc.) - used by doubao
	Console *ConsoleCredentials `yaml:"console,omitempty"`

	// APIKey is the API key for authentication - used by minimax
	APIKey string `yaml:"api_key,omitempty"`

	// BaseURL is the API base URL (optional, uses default if empty)
	BaseURL string `yaml:"base_url,omitempty"`

	// Timeout is the request timeout in seconds (optional)
	Timeout int `yaml:"timeout,omitempty"`

	// MaxRetries is the maximum number of retries (optional)
	MaxRetries int `yaml:"max_retries,omitempty"`

	// DefaultVoice is the default voice type for TTS (optional)
	DefaultVoice string `yaml:"default_voice,omitempty"`

	// Extra stores application-specific settings - used by minimax
	Extra map[string]string `yaml:"extra,omitempty"`
}

// ClientCredentials contains credentials for speech APIs (TTS, ASR, Realtime, etc.)
type ClientCredentials struct {
	// AppID is the application ID
	AppID string `yaml:"app_id"`

	// APIKey is the API key (Bearer token or x-api-key)
	APIKey string `yaml:"api_key"`
}

// ConsoleCredentials contains credentials for console APIs (ListTimbres, etc.)
type ConsoleCredentials struct {
	// AccessKey is the Volcengine AK for OpenAPI signature
	AccessKey string `yaml:"access_key"`

	// SecretKey is the Volcengine SK for OpenAPI signature
	SecretKey string `yaml:"secret_key"`
}

// LoadConfig loads or creates configuration for the specified app
func LoadConfig(appName string) (*Config, error) {
	return LoadConfigWithPath(appName, "")
}

// LoadConfigWithPath loads configuration from a custom path
func LoadConfigWithPath(appName, customPath string) (*Config, error) {
	var configPath string

	if customPath != "" {
		configPath = customPath
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(home, DefaultBaseDir, appName, DefaultConfigFile)
	}

	// Ensure config directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	cfg := &Config{
		AppName:    appName,
		Contexts:   make(map[string]*Context),
		configPath: configPath,
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create empty config file
			return cfg, cfg.Save()
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Ensure contexts map is initialized
	if cfg.Contexts == nil {
		cfg.Contexts = make(map[string]*Context)
	}

	cfg.AppName = appName
	cfg.configPath = configPath

	return cfg, nil
}

// Save saves the configuration to disk
func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(c.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Path returns the config file path
func (c *Config) Path() string {
	return c.configPath
}

// Dir returns the config directory path
func (c *Config) Dir() string {
	return filepath.Dir(c.configPath)
}

// AddContext adds a new context
func (c *Config) AddContext(name string, ctx *Context) error {
	ctx.Name = name
	c.Contexts[name] = ctx
	return c.Save()
}

// DeleteContext removes a context
func (c *Config) DeleteContext(name string) error {
	if _, ok := c.Contexts[name]; !ok {
		return fmt.Errorf("context %q not found", name)
	}
	delete(c.Contexts, name)
	if c.CurrentContext == name {
		c.CurrentContext = ""
	}
	return c.Save()
}

// UseContext sets the current context
func (c *Config) UseContext(name string) error {
	if _, ok := c.Contexts[name]; !ok {
		return fmt.Errorf("context %q not found", name)
	}
	c.CurrentContext = name
	return c.Save()
}

// GetContext returns a specific context
func (c *Config) GetContext(name string) (*Context, error) {
	ctx, ok := c.Contexts[name]
	if !ok {
		return nil, fmt.Errorf("context %q not found", name)
	}
	return ctx, nil
}

// GetCurrentContext returns the current context
func (c *Config) GetCurrentContext() (*Context, error) {
	if c.CurrentContext == "" {
		return nil, fmt.Errorf("no current context set")
	}
	return c.GetContext(c.CurrentContext)
}

// ResolveContext returns the context by name, or current context if name is empty
func (c *Config) ResolveContext(name string) (*Context, error) {
	if name == "" {
		return c.GetCurrentContext()
	}
	return c.GetContext(name)
}

// ListContexts returns all context names
func (c *Config) ListContexts() []string {
	names := make([]string, 0, len(c.Contexts))
	for name := range c.Contexts {
		names = append(names, name)
	}
	return names
}

// GetExtra returns an extra value for the context
func (ctx *Context) GetExtra(key string) string {
	if ctx.Extra == nil {
		return ""
	}
	return ctx.Extra[key]
}

// SetExtra sets an extra value for the context
func (ctx *Context) SetExtra(key, value string) {
	if ctx.Extra == nil {
		ctx.Extra = make(map[string]string)
	}
	ctx.Extra[key] = value
}

// MaskAPIKey masks the API key for display
func MaskAPIKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}
