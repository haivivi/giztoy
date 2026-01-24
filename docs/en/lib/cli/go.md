# CLI Package - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/cli`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/cli)

## Types

### Config

```go
type Config struct {
    AppName        string              `yaml:"-"`
    CurrentContext string              `yaml:"current_context,omitempty"`
    Contexts       map[string]*Context `yaml:"contexts,omitempty"`
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `LoadConfig` | `func LoadConfig(appName string) (*Config, error)` | Load from default path |
| `LoadConfigWithPath` | `func LoadConfigWithPath(appName, path string) (*Config, error)` | Load from custom path |
| `Save` | `(c *Config) Save() error` | Save to disk |
| `Path` | `(c *Config) Path() string` | Get config file path |
| `Dir` | `(c *Config) Dir() string` | Get config directory |
| `AddContext` | `(c *Config) AddContext(name string, ctx *Context) error` | Add context |
| `DeleteContext` | `(c *Config) DeleteContext(name string) error` | Delete context |
| `UseContext` | `(c *Config) UseContext(name string) error` | Set current context |
| `GetContext` | `(c *Config) GetContext(name string) (*Context, error)` | Get specific context |
| `GetCurrentContext` | `(c *Config) GetCurrentContext() (*Context, error)` | Get current context |
| `ResolveContext` | `(c *Config) ResolveContext(name string) (*Context, error)` | Resolve by name or current |
| `ListContexts` | `(c *Config) ListContexts() []string` | List all context names |

### Context

```go
type Context struct {
    Name         string              `yaml:"name"`
    Client       *ClientCredentials  `yaml:"client,omitempty"`
    Console      *ConsoleCredentials `yaml:"console,omitempty"`
    APIKey       string              `yaml:"api_key,omitempty"`
    BaseURL      string              `yaml:"base_url,omitempty"`
    Timeout      int                 `yaml:"timeout,omitempty"`
    MaxRetries   int                 `yaml:"max_retries,omitempty"`
    DefaultVoice string              `yaml:"default_voice,omitempty"`
    Extra        map[string]string   `yaml:"extra,omitempty"`
}
```

### Output

```go
type OutputFormat string

const (
    FormatYAML  OutputFormat = "yaml"
    FormatJSON  OutputFormat = "json"
    FormatTable OutputFormat = "table"
    FormatRaw   OutputFormat = "raw"
)

type OutputOptions struct {
    Format OutputFormat
    File   string
    Indent string
    Writer io.Writer
}
```

**Functions:**

| Function | Signature | Description |
|----------|-----------|-------------|
| `Output` | `func Output(result any, opts OutputOptions) error` | Write formatted output |
| `OutputBytes` | `func OutputBytes(data []byte, path string) error` | Write binary data |
| `PrintSuccess` | `func PrintSuccess(format string, args ...any)` | Print ✓ message |
| `PrintError` | `func PrintError(format string, args ...any)` | Print error to stderr |
| `PrintInfo` | `func PrintInfo(format string, args ...any)` | Print ℹ message |
| `PrintWarning` | `func PrintWarning(format string, args ...any)` | Print ⚠ message |
| `PrintVerbose` | `func PrintVerbose(verbose bool, format string, args ...any)` | Conditional verbose |

### Paths

```go
type Paths struct {
    AppName string
    HomeDir string
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewPaths` | `func NewPaths(appName string) (*Paths, error)` | Create paths instance |
| `BaseDir` | `(p *Paths) BaseDir() string` | ~/.giztoy |
| `AppDir` | `(p *Paths) AppDir() string` | ~/.giztoy/<app> |
| `ConfigFile` | `(p *Paths) ConfigFile() string` | ~/.giztoy/<app>/config.yaml |
| `CacheDir` | `(p *Paths) CacheDir() string` | ~/.giztoy/<app>/cache |
| `LogDir` | `(p *Paths) LogDir() string` | ~/.giztoy/<app>/logs |
| `DataDir` | `(p *Paths) DataDir() string` | ~/.giztoy/<app>/data |
| `EnsureAppDir` | `(p *Paths) EnsureAppDir() error` | Create app dir |
| `CachePath` | `(p *Paths) CachePath(name string) string` | Path in cache |
| `LogPath` | `(p *Paths) LogPath(name string) string` | Path in logs |
| `DataPath` | `(p *Paths) DataPath(name string) string` | Path in data |

## Usage

### Load Configuration

```go
cfg, err := cli.LoadConfig("minimax")
if err != nil {
    log.Fatal(err)
}

// Get current context
ctx, err := cfg.GetCurrentContext()
if err != nil {
    log.Fatal(err)
}

fmt.Println("API Key:", cli.MaskAPIKey(ctx.APIKey))
```

### Output Results

```go
result := map[string]string{"status": "ok", "message": "done"}

// Output as JSON to stdout
cli.Output(result, cli.OutputOptions{
    Format: cli.FormatJSON,
})

// Output as YAML to file
cli.Output(result, cli.OutputOptions{
    Format: cli.FormatYAML,
    File:   "output.yaml",
})
```

### Print Helpers

```go
cli.PrintSuccess("Created context %q", "production")
cli.PrintError("Failed to connect: %v", err)
cli.PrintInfo("Using API endpoint: %s", baseURL)
cli.PrintWarning("Rate limit approaching")
cli.PrintVerbose(verbose, "Request: %+v", req)
```

### Path Management

```go
paths, _ := cli.NewPaths("minimax")

// Ensure directories exist
paths.EnsureCacheDir()
paths.EnsureLogDir()

// Get paths
cachePath := paths.CachePath("response.json")
logPath := paths.LogPath("2024-01-15.log")
```

## Dependencies

- `github.com/goccy/go-yaml` - YAML parsing
- `encoding/json` (stdlib) - JSON parsing
