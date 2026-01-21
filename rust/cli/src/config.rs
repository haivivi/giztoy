//! Configuration management for CLI tools.
//!
//! Compatible with Go version's configuration format.
//! Configuration is stored in ~/.giztoy/{app_name}/config.yaml

use std::collections::HashMap;
use std::path::PathBuf;

use serde::{Deserialize, Serialize};

/// Default base configuration directory name.
pub const DEFAULT_BASE_DIR: &str = ".giztoy";
/// Default configuration filename.
pub const DEFAULT_CONFIG_FILE: &str = "config.yaml";

/// CLI configuration.
///
/// Compatible with Go version's Config structure.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Config {
    /// Application name (not serialized).
    #[serde(skip)]
    pub app_name: String,

    /// Name of the currently active context.
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub current_context: String,

    /// Map of context name to context configuration.
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub contexts: HashMap<String, Context>,

    /// Path to the config file (not serialized).
    #[serde(skip)]
    config_path: PathBuf,
}

/// A single API context configuration.
///
/// Compatible with Go version's Context structure.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Context {
    /// Context name.
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub name: String,

    /// Client credentials for speech APIs (TTS, ASR, etc.) - used by doubao.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub client: Option<ClientCredentials>,

    /// Console credentials for console APIs (ListTimbres, etc.) - used by doubao.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub console: Option<ConsoleCredentials>,

    /// API key for authentication - used by minimax.
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub api_key: String,

    /// API base URL (optional, uses default if empty).
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub base_url: String,

    /// Request timeout in seconds (optional).
    #[serde(default, skip_serializing_if = "is_zero")]
    pub timeout: i32,

    /// Maximum number of retries (optional).
    #[serde(default, skip_serializing_if = "is_zero")]
    pub max_retries: i32,

    /// Default voice for TTS (optional).
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub default_voice: String,

    /// Application-specific settings.
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub extra: HashMap<String, String>,
}

/// Client credentials for speech APIs (TTS, ASR, Realtime, etc.).
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ClientCredentials {
    /// Application ID.
    pub app_id: String,

    /// API key (Bearer token or x-api-key).
    pub api_key: String,
}

/// Console credentials for console APIs (ListTimbres, etc.).
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ConsoleCredentials {
    /// Volcengine AK for OpenAPI signature.
    pub access_key: String,

    /// Volcengine SK for OpenAPI signature.
    pub secret_key: String,
}

fn is_zero(n: &i32) -> bool {
    *n == 0
}

impl Config {
    /// Gets the default config directory.
    pub fn default_config_dir(app_name: &str) -> Option<PathBuf> {
        dirs::home_dir().map(|home| home.join(DEFAULT_BASE_DIR).join(app_name))
    }

    /// Gets the default config file path.
    pub fn default_config_path(app_name: &str) -> Option<PathBuf> {
        Self::default_config_dir(app_name).map(|dir| dir.join(DEFAULT_CONFIG_FILE))
    }

    /// Returns the config file path.
    pub fn path(&self) -> &PathBuf {
        &self.config_path
    }

    /// Returns the config directory path.
    pub fn dir(&self) -> Option<&std::path::Path> {
        self.config_path.parent()
    }

    /// Saves the configuration to disk.
    pub fn save(&self) -> anyhow::Result<()> {
        let content = serde_yaml::to_string(self)?;
        std::fs::write(&self.config_path, content)?;
        Ok(())
    }

    /// Adds a new context.
    pub fn add_context(&mut self, name: &str, mut ctx: Context) -> anyhow::Result<()> {
        ctx.name = name.to_string();
        self.contexts.insert(name.to_string(), ctx);
        self.save()
    }

    /// Deletes a context.
    pub fn delete_context(&mut self, name: &str) -> anyhow::Result<()> {
        if !self.contexts.contains_key(name) {
            anyhow::bail!("context '{}' not found", name);
        }
        self.contexts.remove(name);
        if self.current_context == name {
            self.current_context.clear();
        }
        self.save()
    }

    /// Sets the current context.
    pub fn use_context(&mut self, name: &str) -> anyhow::Result<()> {
        if !self.contexts.contains_key(name) {
            anyhow::bail!("context '{}' not found", name);
        }
        self.current_context = name.to_string();
        self.save()
    }

    /// Gets a specific context.
    pub fn get_context(&self, name: &str) -> Option<&Context> {
        self.contexts.get(name)
    }

    /// Gets the current context.
    pub fn get_current_context(&self) -> Option<&Context> {
        if self.current_context.is_empty() {
            return None;
        }
        self.contexts.get(&self.current_context)
    }

    /// Resolves the context by name, or current context if name is empty.
    pub fn resolve_context(&self, name: Option<&str>) -> Option<&Context> {
        match name {
            Some(n) if !n.is_empty() => self.get_context(n),
            _ => self.get_current_context(),
        }
    }

    /// Lists all context names.
    pub fn list_contexts(&self) -> Vec<&str> {
        self.contexts.keys().map(|s| s.as_str()).collect()
    }
}

impl Context {
    /// Gets an extra value.
    pub fn get_extra(&self, key: &str) -> Option<&str> {
        self.extra.get(key).map(|s| s.as_str())
    }

    /// Sets an extra value.
    pub fn set_extra(&mut self, key: impl Into<String>, value: impl Into<String>) {
        self.extra.insert(key.into(), value.into());
    }
}

/// Loads configuration for the specified app.
pub fn load_config(app_name: &str, custom_path: Option<&str>) -> anyhow::Result<Config> {
    let config_path = match custom_path {
        Some(p) => PathBuf::from(p),
        None => Config::default_config_path(app_name)
            .ok_or_else(|| anyhow::anyhow!("cannot determine config path"))?,
    };

    // Ensure config directory exists
    if let Some(parent) = config_path.parent() {
        std::fs::create_dir_all(parent)?;
    }

    let mut cfg = if config_path.exists() {
        let content = std::fs::read_to_string(&config_path)?;
        serde_yaml::from_str(&content)?
    } else {
        // Create empty config file
        let cfg = Config::default();
        let content = serde_yaml::to_string(&cfg)?;
        std::fs::write(&config_path, content)?;
        cfg
    };

    cfg.app_name = app_name.to_string();
    cfg.config_path = config_path;

    Ok(cfg)
}

/// Saves configuration to the specified path.
pub fn save_config(app_name: &str, config: &Config, custom_path: Option<&str>) -> anyhow::Result<()> {
    let config_path = match custom_path {
        Some(p) => PathBuf::from(p),
        None => Config::default_config_path(app_name)
            .ok_or_else(|| anyhow::anyhow!("cannot determine config path"))?,
    };

    if let Some(parent) = config_path.parent() {
        std::fs::create_dir_all(parent)?;
    }

    let content = serde_yaml::to_string(config)?;
    std::fs::write(&config_path, content)?;
    Ok(())
}

/// Masks the API key for display.
pub fn mask_api_key(key: &str) -> String {
    if key.len() <= 8 {
        "*".repeat(key.len())
    } else {
        format!(
            "{}{}{}",
            &key[..4],
            "*".repeat(key.len() - 8),
            &key[key.len() - 4..]
        )
    }
}
