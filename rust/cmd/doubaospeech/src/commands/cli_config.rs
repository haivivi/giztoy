//! Inlined CLI configuration management (formerly giztoy-cli).
//!
//! Compatible with Go version's configuration format.
//! Configuration is stored in ~/.giztoy/{app_name}/config.yaml

use std::collections::HashMap;
use std::path::PathBuf;

use serde::{Deserialize, Serialize};

const DEFAULT_BASE_DIR: &str = ".giztoy";
const DEFAULT_CONFIG_FILE: &str = "config.yaml";

/// CLI configuration.
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

/// A single API context configuration.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Context {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub name: String,

    /// Client credentials for speech APIs (TTS, ASR, etc.).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub client: Option<ClientCredentials>,

    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub api_key: String,

    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub base_url: String,

    #[serde(default, skip_serializing_if = "is_zero")]
    pub timeout: i32,

    #[serde(default, skip_serializing_if = "is_zero")]
    pub max_retries: i32,

    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub default_voice: String,

    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub extra: HashMap<String, String>,
}

/// Client credentials for speech APIs.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ClientCredentials {
    pub app_id: String,
    pub api_key: String,
}

fn is_zero(n: &i32) -> bool {
    *n == 0
}

impl Config {
    fn default_config_path(app_name: &str) -> Option<PathBuf> {
        dirs::home_dir().map(|home| home.join(DEFAULT_BASE_DIR).join(app_name).join(DEFAULT_CONFIG_FILE))
    }

    pub fn path(&self) -> &PathBuf {
        &self.config_path
    }

    pub fn save(&self) -> anyhow::Result<()> {
        let content = serde_yaml::to_string(self)?;
        std::fs::write(&self.config_path, content)?;
        Ok(())
    }

    pub fn add_context(&mut self, name: &str, mut ctx: Context) -> anyhow::Result<()> {
        ctx.name = name.to_string();
        self.contexts.insert(name.to_string(), ctx);
        self.save()
    }

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

    pub fn use_context(&mut self, name: &str) -> anyhow::Result<()> {
        if !self.contexts.contains_key(name) {
            anyhow::bail!("context '{}' not found", name);
        }
        self.current_context = name.to_string();
        self.save()
    }

    pub fn resolve_context(&self, name: Option<&str>) -> Option<&Context> {
        match name {
            Some(n) if !n.is_empty() => self.contexts.get(n),
            _ => {
                if self.current_context.is_empty() {
                    None
                } else {
                    self.contexts.get(&self.current_context)
                }
            }
        }
    }
}

impl Context {
    pub fn get_extra(&self, key: &str) -> Option<&str> {
        self.extra.get(key).map(|s| s.as_str())
    }

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

    if let Some(parent) = config_path.parent() {
        std::fs::create_dir_all(parent)?;
    }

    let mut cfg = if config_path.exists() {
        let content = std::fs::read_to_string(&config_path)?;
        serde_yaml::from_str(&content)?
    } else {
        let cfg = Config::default();
        let content = serde_yaml::to_string(&cfg)?;
        std::fs::write(&config_path, content)?;
        cfg
    };

    cfg.app_name = app_name.to_string();
    cfg.config_path = config_path;
    Ok(cfg)
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
