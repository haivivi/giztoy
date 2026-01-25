//! Model configuration loading and generator registry.

use std::collections::HashMap;
use std::path::Path;
use std::sync::Arc;

use anyhow::Result;
use giztoy_genx::context::ModelParams;
use giztoy_genx::gemini::{GeminiConfig, GeminiGenerator};
use giztoy_genx::openai::{OpenAIConfig, OpenAIGenerator};
use giztoy_genx::Generator;
use once_cell::sync::Lazy;
use serde::Deserialize;
use tokio::sync::RwLock;

/// Global generator registry.
static REGISTRY: Lazy<Arc<RwLock<GeneratorRegistry>>> =
    Lazy::new(|| Arc::new(RwLock::new(GeneratorRegistry::new())));

/// Registry for named generators.
pub struct GeneratorRegistry {
    generators: HashMap<String, Arc<dyn Generator>>,
}

impl GeneratorRegistry {
    fn new() -> Self {
        Self {
            generators: HashMap::new(),
        }
    }

    fn register(&mut self, name: String, generator: Arc<dyn Generator>) {
        self.generators.insert(name, generator);
    }

    fn get(&self, name: &str) -> Option<Arc<dyn Generator>> {
        self.generators.get(name).cloned()
    }

    fn names(&self) -> Vec<String> {
        self.generators.keys().cloned().collect()
    }
}

/// Get a generator by name.
pub async fn get_generator(name: &str) -> Option<Arc<dyn Generator>> {
    REGISTRY.read().await.get(name)
}

/// Get all registered model names.
pub async fn get_all_names() -> Vec<String> {
    REGISTRY.read().await.names()
}

/// Configuration file format.
#[derive(Debug, Deserialize)]
pub struct ConfigFile {
    kind: String,
    api_key: String,
    #[serde(default)]
    base_url: Option<String>,
    models: Vec<ModelEntry>,
}

/// Model entry in config.
#[derive(Debug, Deserialize)]
pub struct ModelEntry {
    name: String,
    model: String,
    #[serde(default)]
    generate_params: Option<ModelParams>,
    #[serde(default)]
    invoke_params: Option<ModelParams>,
    #[serde(default)]
    support_json_output: bool,
    #[serde(default)]
    support_tool_calls: bool,
    #[serde(default)]
    support_text_only: bool,
    #[serde(default)]
    use_system_role: bool,
}

/// Load model configs from directory recursively and register generators.
pub async fn load_from_dir(dir: &Path, verbose: bool) -> Result<Vec<String>> {
    let mut names = Vec::new();

    for entry in walkdir(dir)? {
        let path = entry;
        let ext = path.extension().and_then(|s| s.to_str()).unwrap_or("");

        if ext != "json" && ext != "yaml" && ext != "yml" {
            continue;
        }

        let data = std::fs::read(&path)?;
        let cfg: ConfigFile = match ext {
            "json" => serde_json::from_slice(&data)?,
            "yaml" | "yml" => serde_yaml::from_slice(&data)?,
            _ => continue,
        };

        if verbose {
            eprintln!("Loading config: {}", path.display());
        }

        let file_names = register_config(cfg, verbose).await?;
        names.extend(file_names);
    }

    Ok(names)
}

/// Register a configuration.
async fn register_config(cfg: ConfigFile, verbose: bool) -> Result<Vec<String>> {
    // Expand environment variables in API key
    let api_key = expand_env(&cfg.api_key);

    if verbose && api_key.is_empty() && !cfg.api_key.is_empty() {
        eprintln!(
            "Warning: API key '{}' resolved to empty (env var not set?)",
            cfg.api_key
        );
    }

    match cfg.kind.to_lowercase().as_str() {
        "openai" => register_openai(cfg, &api_key).await,
        "gemini" => register_gemini(cfg, &api_key).await,
        _ => anyhow::bail!("unknown kind: {}", cfg.kind),
    }
}

/// Expand environment variables in a string.
fn expand_env(s: &str) -> String {
    if s.is_empty() {
        return s.to_string();
    }

    if s.starts_with('$') {
        // $VAR or ${VAR}
        let var_name = if s.starts_with("${") && s.ends_with('}') {
            &s[2..s.len() - 1]
        } else {
            &s[1..]
        };
        std::env::var(var_name).unwrap_or_default()
    } else {
        s.to_string()
    }
}

/// Register OpenAI-compatible models.
async fn register_openai(cfg: ConfigFile, api_key: &str) -> Result<Vec<String>> {
    if api_key.is_empty() {
        anyhow::bail!("api_key is required for openai kind");
    }

    let base_url = cfg
        .base_url
        .clone()
        .unwrap_or_else(|| "https://api.openai.com/v1".to_string());

    let mut names = Vec::new();
    let mut registry = REGISTRY.write().await;

    for m in cfg.models {
        if m.name.is_empty() || m.model.is_empty() {
            anyhow::bail!("model entry missing name or model");
        }

        let config = OpenAIConfig {
            api_key: api_key.to_string(),
            base_url: base_url.clone(),
            model: m.model.clone(),
            support_json_output: m.support_json_output,
            support_tool_calls: m.support_tool_calls,
            support_text_only: m.support_text_only,
            use_system_role: m.use_system_role,
            generate_params: m.generate_params,
            invoke_params: m.invoke_params,
        };

        let generator = Arc::new(OpenAIGenerator::new(config));
        registry.register(m.name.clone(), generator);
        names.push(m.name);
    }

    Ok(names)
}

/// Register Gemini models.
async fn register_gemini(cfg: ConfigFile, api_key: &str) -> Result<Vec<String>> {
    if api_key.is_empty() {
        anyhow::bail!("api_key is required for gemini kind");
    }

    let mut names = Vec::new();
    let mut registry = REGISTRY.write().await;

    for m in cfg.models {
        if m.name.is_empty() || m.model.is_empty() {
            anyhow::bail!("model entry missing name or model");
        }

        let config = GeminiConfig {
            api_key: api_key.to_string(),
            model: m.model.clone(),
            generate_params: m.generate_params,
            invoke_params: m.invoke_params,
        };

        let generator = Arc::new(GeminiGenerator::new(config));
        registry.register(m.name.clone(), generator);
        names.push(m.name);
    }

    Ok(names)
}

/// Simple directory walk.
fn walkdir(dir: &Path) -> Result<Vec<std::path::PathBuf>> {
    let mut paths = Vec::new();

    fn walk(dir: &Path, paths: &mut Vec<std::path::PathBuf>) -> Result<()> {
        for entry in std::fs::read_dir(dir)? {
            let entry = entry?;
            let path = entry.path();
            if path.is_dir() {
                walk(&path, paths)?;
            } else {
                paths.push(path);
            }
        }
        Ok(())
    }

    walk(dir, &mut paths)?;
    Ok(paths)
}
