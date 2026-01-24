# CLI Package - Rust Implementation

Crate: `giztoy-cli`

📚 [Rust Documentation](https://docs.rs/giztoy-cli)

## Types

### Config

```rust
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Config {
    #[serde(skip)]
    pub app_name: String,
    
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub current_context: String,
    
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub contexts: HashMap<String, Context>,
    
    #[serde(skip)]
    config_path: PathBuf,
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `default_config_dir` | `fn default_config_dir(app_name: &str) -> Option<PathBuf>` | Get default config dir |
| `default_config_path` | `fn default_config_path(app_name: &str) -> Option<PathBuf>` | Get default config path |
| `path` | `fn path(&self) -> &PathBuf` | Get config file path |
| `dir` | `fn dir(&self) -> Option<&Path>` | Get config directory |
| `save` | `fn save(&self) -> Result<()>` | Save to disk |
| `add_context` | `fn add_context(&mut self, name: &str, ctx: Context) -> Result<()>` | Add context |
| `delete_context` | `fn delete_context(&mut self, name: &str) -> Result<()>` | Delete context |
| `use_context` | `fn use_context(&mut self, name: &str) -> Result<()>` | Set current context |
| `get_context` | `fn get_context(&self, name: &str) -> Option<&Context>` | Get specific context |
| `get_current_context` | `fn get_current_context(&self) -> Option<&Context>` | Get current context |
| `resolve_context` | `fn resolve_context(&self, name: Option<&str>) -> Option<&Context>` | Resolve by name or current |
| `list_contexts` | `fn list_contexts(&self) -> Vec<&str>` | List all context names |

**Free Functions:**

| Function | Signature | Description |
|----------|-----------|-------------|
| `load_config` | `fn load_config(app_name: &str, custom_path: Option<&str>) -> Result<Config>` | Load config |
| `save_config` | `fn save_config(app_name: &str, config: &Config, custom_path: Option<&str>) -> Result<()>` | Save config |
| `mask_api_key` | `fn mask_api_key(key: &str) -> String` | Mask API key |

### Context

```rust
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Context {
    pub name: String,
    pub client: Option<ClientCredentials>,
    pub console: Option<ConsoleCredentials>,
    pub api_key: String,
    pub base_url: String,
    pub timeout: i32,
    pub max_retries: i32,
    pub default_voice: String,
    pub extra: HashMap<String, String>,
}
```

### Output

```rust
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum OutputFormat {
    #[default]
    Yaml,
    Json,
}

pub struct Output {
    pub format: OutputFormat,
    pub file: Option<String>,
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `new` | `fn new(format: OutputFormat, file: Option<String>) -> Self` | Create output config |
| `write` | `fn write<T: Serialize>(&self, value: &T) -> Result<()>` | Write formatted output |
| `write_binary` | `fn write_binary(&self, data: &[u8], path: &str) -> Result<()>` | Write binary data |

**Free Functions:**

| Function | Signature | Description |
|----------|-----------|-------------|
| `print_verbose` | `fn print_verbose(enabled: bool, message: &str)` | Print verbose message |
| `guess_extension` | `fn guess_extension(format: &str) -> &str` | Guess file extension |

### Paths

```rust
#[derive(Debug, Clone)]
pub struct Paths {
    pub app_name: String,
    pub home_dir: PathBuf,
}
```

**Methods:**

| Method | Signature | Description |
|--------|-----------|-------------|
| `new` | `fn new(app_name: impl Into<String>) -> io::Result<Self>` | Create paths instance |
| `base_dir` | `fn base_dir(&self) -> PathBuf` | ~/.giztoy |
| `app_dir` | `fn app_dir(&self) -> PathBuf` | ~/.giztoy/<app> |
| `config_file` | `fn config_file(&self) -> PathBuf` | ~/.giztoy/<app>/config.yaml |
| `cache_dir` | `fn cache_dir(&self) -> PathBuf` | ~/.giztoy/<app>/cache |
| `log_dir` | `fn log_dir(&self) -> PathBuf` | ~/.giztoy/<app>/logs |
| `data_dir` | `fn data_dir(&self) -> PathBuf` | ~/.giztoy/<app>/data |
| `ensure_app_dir` | `fn ensure_app_dir(&self) -> io::Result<()>` | Create app dir |
| `ensure_cache_dir` | `fn ensure_cache_dir(&self) -> io::Result<()>` | Create cache dir |
| `cache_path` | `fn cache_path(&self, name: &str) -> PathBuf` | Path in cache |
| `log_path` | `fn log_path(&self, name: &str) -> PathBuf` | Path in logs |
| `data_path` | `fn data_path(&self, name: &str) -> PathBuf` | Path in data |

## Usage

### Load Configuration

```rust
use giztoy_cli::{load_config, mask_api_key};

let cfg = load_config("minimax", None)?;

if let Some(ctx) = cfg.get_current_context() {
    println!("API Key: {}", mask_api_key(&ctx.api_key));
}
```

### Output Results

```rust
use giztoy_cli::{Output, OutputFormat};
use serde::Serialize;

#[derive(Serialize)]
struct Result {
    status: String,
    message: String,
}

let result = Result {
    status: "ok".to_string(),
    message: "done".to_string(),
};

// Output as JSON to stdout
let output = Output::new(OutputFormat::Json, None);
output.write(&result)?;

// Output to file
let output = Output::new(OutputFormat::Yaml, Some("output.yaml".to_string()));
output.write(&result)?;
```

### Path Management

```rust
use giztoy_cli::Paths;

let paths = Paths::new("minimax")?;

// Ensure directories exist
paths.ensure_cache_dir()?;
paths.ensure_log_dir()?;

// Get paths
let cache_path = paths.cache_path("response.json");
let log_path = paths.log_path("2024-01-15.log");
```

## Dependencies

- `serde` + `serde_yaml` + `serde_json` - Serialization
- `dirs` - Home directory detection
- `anyhow` - Error handling

## Differences from Go

| Aspect | Go | Rust |
|--------|----|----- |
| Error handling | `error` return | `anyhow::Result` |
| Config loading | `LoadConfig(app)` | `load_config(app, None)` |
| Output formats | yaml, json, table, raw | yaml, json only |
| Print helpers | `PrintSuccess`, `PrintError`, etc. | `print_verbose` only |
| Path returns | `string` | `PathBuf` |
