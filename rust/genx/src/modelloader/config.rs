//! Configuration file types and loading.

use std::collections::HashMap;
use std::path::Path;
use std::sync::Arc;

use serde::{Deserialize, Serialize};

use crate::context::ModelParams;
use crate::error::GenxError;
use crate::generators::Mux as GeneratorMux;
use crate::openai::{OpenAIConfig, OpenAIGenerator};
use crate::profilers::{GenXProfiler, ProfilerConfig, ProfilerMux};
use crate::segmentors::{GenXSegmentor, SegmentorConfig, SegmentorMux};

/// Top-level configuration file structure.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConfigFile {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub schema: Option<String>,
    #[serde(default, rename = "type", skip_serializing_if = "Option::is_none")]
    pub type_: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub kind: Option<String>,

    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub api_key: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub base_url: Option<String>,

    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub models: Vec<Entry>,

    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub app_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub token: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cluster: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,

    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub voices: Vec<VoiceEntry>,

    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub default_params: Option<HashMap<String, serde_json::Value>>,
}

/// A model entry in the config.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Entry {
    pub name: String,
    #[serde(default)]
    pub model: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub generate_params: Option<ModelParams>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub invoke_params: Option<ModelParams>,
    #[serde(default)]
    pub support_json_output: bool,
    #[serde(default)]
    pub support_tool_calls: bool,
    #[serde(default)]
    pub support_text_only: bool,
    #[serde(default)]
    pub use_system_role: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub extra_fields: Option<HashMap<String, serde_json::Value>>,

    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub voice: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub resource_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub desc: Option<String>,
}

/// A TTS voice entry.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VoiceEntry {
    pub name: String,
    pub voice_id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub desc: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cluster: Option<String>,
}

/// Expand environment variable references in a config value.
///
/// Matches Go's `modelloader.expandEnv` behavior exactly:
///   - If the string does not start with `$`, return it unchanged
///   - If it starts with `$`, apply Go `os.ExpandEnv` semantics
///
/// This means config values are either literal or env-var references:
///   - `"$HOME"` → value of HOME
///   - `"${API_KEY}"` → value of API_KEY
///   - `"$HOME/data"` → value of HOME + "/data"
///   - `"sk-plain-key"` → unchanged (no $ prefix)
///   - `"sk-abc$HOME"` → unchanged ($ not at start)
fn expand_env(s: &str) -> String {
    if s.is_empty() || !s.starts_with('$') {
        return s.to_string();
    }
    os_expand_env(s)
}

/// Returns true if `c` is a Go shell-special variable character.
///
/// Go's `os.Expand` treats these as single-character variable names that
/// consume the character after `$`. Matches Go's `isShellSpecialVar`:
///   digits 0-9, and `$`, `-`, `!`, `?`, `#`, `@`, `*`
fn is_shell_special(c: char) -> bool {
    matches!(c, '0'..='9' | '$' | '-' | '!' | '?' | '#' | '@' | '*')
}

/// Implements Go `os.ExpandEnv` — expands `$VAR` and `${VAR}` anywhere in the string.
///
/// Go semantics (verified against Go 1.25 os.ExpandEnv):
///   - `$name` → env var (name = longest run of `[a-zA-Z_][a-zA-Z0-9_]*`)
///   - `${name}` → env var
///   - `$0`..`$9`, `$$`, `$-`, `$!`, `$?`, `$#`, `$@`, `$*` → single-char shell var
///   - `$.`, `$,`, `$/`, `$+`, `$=`, `$ ` → literal `$` (char preserved)
///   - trailing `$` → literal `$`
fn os_expand_env(s: &str) -> String {
    let chars: Vec<char> = s.chars().collect();
    let mut result = String::with_capacity(s.len());
    let mut i = 0;

    while i < chars.len() {
        if chars[i] != '$' {
            result.push(chars[i]);
            i += 1;
            continue;
        }

        i += 1;
        if i >= chars.len() {
            // Trailing $ with nothing after → literal $
            result.push('$');
            break;
        }

        // ${VAR}
        if chars[i] == '{' {
            i += 1;
            let start = i;
            while i < chars.len() && chars[i] != '}' {
                i += 1;
            }
            let var_name: String = chars[start..i].iter().collect();
            result.push_str(&std::env::var(&var_name).unwrap_or_default());
            if i < chars.len() {
                i += 1;
            }
            continue;
        }

        // $name — name is longest run of [a-zA-Z_][a-zA-Z0-9_]*
        // Note: Go treats leading digits as shell special vars (single char),
        // not as identifier starts. So $0abc → expand "0" then literal "abc".
        if chars[i].is_ascii_alphabetic() || chars[i] == '_' {
            let start = i;
            while i < chars.len() && (chars[i].is_ascii_alphanumeric() || chars[i] == '_') {
                i += 1;
            }
            let var_name: String = chars[start..i].iter().collect();
            result.push_str(&std::env::var(&var_name).unwrap_or_default());
        } else if is_shell_special(chars[i]) {
            // Go shell special vars: single-char names like $0..$9, $$, $-, $!, $?, $#, $@, $*
            // These consume the character after $ and expand as a single-char var name.
            let var_name = chars[i].to_string();
            i += 1;
            result.push_str(&std::env::var(&var_name).unwrap_or_default());
        } else {
            // Non-identifier, non-shell-special char (e.g. `.`, `,`, `/`, `+`, `=`, space)
            // Go does NOT consume the char — $ is treated as literal.
            result.push('$');
            // Do not advance i — the char will be processed in the next iteration.
        }
    }

    result
}

/// Parse a config file from a path (YAML or JSON).
pub fn parse_config(path: &Path) -> Result<ConfigFile, GenxError> {
    let data = std::fs::read_to_string(path)
        .map_err(|e| GenxError::Other(anyhow::anyhow!("read {}: {}", path.display(), e)))?;

    let ext = path
        .extension()
        .and_then(|e| e.to_str())
        .unwrap_or("")
        .to_lowercase();

    match ext.as_str() {
        "json" => serde_json::from_str(&data)
            .map_err(|e| GenxError::Other(anyhow::anyhow!("parse {}: {}", path.display(), e))),
        "yaml" | "yml" => serde_yaml::from_str(&data)
            .map_err(|e| GenxError::Other(anyhow::anyhow!("parse {}: {}", path.display(), e))),
        _ => Err(GenxError::Other(anyhow::anyhow!(
            "unsupported config extension: {}",
            ext
        ))),
    }
}

/// Mux collection for registering models.
///
/// `generators` is wrapped in `Arc<std::sync::RwLock>` so segmentors/profilers
/// share the same instance (matching Go's shared-pointer semantics). Generators
/// registered after segmentor/profiler creation are immediately visible.
pub struct MuxSet {
    pub generators: Arc<std::sync::RwLock<GeneratorMux>>,
    pub segmentors: SegmentorMux,
    pub profilers: ProfilerMux,
}

impl MuxSet {
    pub fn new() -> Self {
        Self {
            generators: Arc::new(std::sync::RwLock::new(GeneratorMux::new())),
            segmentors: SegmentorMux::new(),
            profilers: ProfilerMux::new(),
        }
    }
}

impl Default for MuxSet {
    fn default() -> Self {
        Self::new()
    }
}

/// Recursively collect config files from a directory (matching Go's filepath.WalkDir).
fn walk_config_files(dir: &Path, configs: &mut Vec<ConfigFile>) -> Result<(), GenxError> {
    let entries = std::fs::read_dir(dir)
        .map_err(|e| GenxError::Other(anyhow::anyhow!("read dir {}: {}", dir.display(), e)))?;

    for entry in entries {
        let entry = entry
            .map_err(|e| GenxError::Other(anyhow::anyhow!("dir entry: {}", e)))?;
        let path = entry.path();
        if path.is_dir() {
            walk_config_files(&path, configs)?;
            continue;
        }
        let ext = path
            .extension()
            .and_then(|e| e.to_str())
            .unwrap_or("")
            .to_lowercase();
        if ext != "json" && ext != "yaml" && ext != "yml" {
            continue;
        }
        configs.push(parse_config(&path)?);
    }
    Ok(())
}

/// Load all config files from a directory recursively and register to the MuxSet.
/// Uses two-pass loading: first pass registers generators, second pass
/// registers segmentors/profilers (which reference generators).
/// Skips files with missing credentials. Returns registered model names.
pub fn load_from_dir(dir: &Path, muxes: &mut MuxSet) -> Result<Vec<String>, GenxError> {
    let mut configs = Vec::new();
    walk_config_files(dir, &mut configs)?;

    let mut names = Vec::new();

    // Pass 1: register generators (kind-based legacy OR schema+type=generator)
    for cfg in &configs {
        let is_generator = match (&cfg.schema, cfg.type_.as_deref()) {
            (Some(_), Some("generator")) => true,
            (None, _) if cfg.kind.is_some() => true, // legacy kind-based = always generator
            _ => false,
        };
        if !is_generator {
            continue;
        }
        match register_config(cfg.clone(), muxes) {
            Ok(file_names) => names.extend(file_names),
            Err(e) if e.to_string().contains("is required") => continue,
            Err(e) => return Err(e),
        }
    }

    // Pass 2: register segmentors, profilers, and other non-generator types
    for cfg in configs {
        let is_generator = match (&cfg.schema, cfg.type_.as_deref()) {
            (Some(_), Some("generator")) => true,
            (None, _) if cfg.kind.is_some() => true,
            _ => false,
        };
        if is_generator {
            continue;
        }
        match register_config(cfg, muxes) {
            Ok(file_names) => names.extend(file_names),
            Err(e) if e.to_string().contains("is required") => continue,
            Err(e) => return Err(e),
        }
    }

    Ok(names)
}

/// Register a single config to the appropriate muxes.
pub fn register_config(
    mut cfg: ConfigFile,
    muxes: &mut MuxSet,
) -> Result<Vec<String>, GenxError> {
    if let Some(ref key) = cfg.api_key {
        cfg.api_key = Some(expand_env(key));
    }
    if let Some(ref token) = cfg.token {
        cfg.token = Some(expand_env(token));
    }
    if let Some(ref app_id) = cfg.app_id {
        cfg.app_id = Some(expand_env(app_id));
    }

    if let Some(ref schema) = cfg.schema {
        return register_by_schema(schema.clone(), &cfg, muxes);
    }

    if let Some(ref kind) = cfg.kind {
        return match kind.to_lowercase().as_str() {
            "openai" => register_openai(&cfg, &muxes.generators),
            _ => Err(GenxError::Other(anyhow::anyhow!("unknown kind: {}", kind))),
        };
    }

    Err(GenxError::Other(anyhow::anyhow!(
        "config has neither schema nor kind"
    )))
}

fn register_by_schema(
    schema: String,
    cfg: &ConfigFile,
    muxes: &mut MuxSet,
) -> Result<Vec<String>, GenxError> {
    let type_ = cfg.type_.as_deref().unwrap_or("");
    match type_ {
        "generator" => {
            let provider = schema
                .split('/')
                .next()
                .unwrap_or("");
            match provider {
                "openai" => register_openai(cfg, &muxes.generators),
                _ => Err(GenxError::Other(anyhow::anyhow!(
                    "unknown generator provider: {}",
                    provider
                ))),
            }
        }
        "segmentor" => register_segmentors(cfg, &mut muxes.segmentors, Arc::clone(&muxes.generators)),
        "profiler" => register_profilers(cfg, &mut muxes.profilers, Arc::clone(&muxes.generators)),
        _ => Err(GenxError::Other(anyhow::anyhow!(
            "unknown type: {}",
            type_
        ))),
    }
}

fn register_openai(
    cfg: &ConfigFile,
    gen_mux: &Arc<std::sync::RwLock<GeneratorMux>>,
) -> Result<Vec<String>, GenxError> {
    let api_key = cfg
        .api_key
        .as_deref()
        .filter(|k| !k.is_empty())
        .ok_or_else(|| GenxError::Other(anyhow::anyhow!("api_key is required for openai")))?;

    let mut names = Vec::new();
    for m in &cfg.models {
        if m.name.is_empty() || m.model.is_empty() {
            return Err(GenxError::Other(anyhow::anyhow!(
                "model entry missing name or model"
            )));
        }

        let config = OpenAIConfig {
            api_key: api_key.to_string(),
            base_url: cfg.base_url.clone().unwrap_or_else(|| "https://api.openai.com/v1".to_string()),
            model: m.model.clone(),
            support_json_output: m.support_json_output,
            support_tool_calls: m.support_tool_calls,
            support_text_only: m.support_text_only,
            use_system_role: m.use_system_role,
            generate_params: m.generate_params.clone(),
            invoke_params: m.invoke_params.clone(),
            extra_fields: m.extra_fields.clone(),
        };

        gen_mux.write().unwrap_or_else(|e| e.into_inner()).handle(&m.name, Arc::new(OpenAIGenerator::new(config)))?;
        names.push(m.name.clone());
    }
    Ok(names)
}

/// Wrapper that implements Generator by delegating to a shared RwLock<GeneratorMux>.
/// This ensures segmentors/profilers always see the latest registered generators.
///
/// On each call: acquires read lock → looks up target generator → clones its Arc
/// → releases lock → calls the generator. Only the target Arc is cloned (O(1)),
/// not the entire HashMap.
struct SharedGeneratorMux(Arc<std::sync::RwLock<GeneratorMux>>);

#[async_trait::async_trait]
impl crate::Generator for SharedGeneratorMux {
    async fn generate_stream(
        &self,
        model: &str,
        ctx: &dyn crate::context::ModelContext,
    ) -> Result<Box<dyn crate::stream::Stream>, GenxError> {
        let target = self.0.read().unwrap_or_else(|e| e.into_inner()).get_arc(model)?;
        target.generate_stream(model, ctx).await
    }

    async fn invoke(
        &self,
        model: &str,
        ctx: &dyn crate::context::ModelContext,
        tool: &crate::tool::FuncTool,
    ) -> Result<(crate::error::Usage, crate::types::FuncCall), GenxError> {
        let target = self.0.read().unwrap_or_else(|e| e.into_inner()).get_arc(model)?;
        target.invoke(model, ctx, tool).await
    }
}

fn register_segmentors(
    cfg: &ConfigFile,
    seg_mux: &mut SegmentorMux,
    gen_mux: Arc<std::sync::RwLock<GeneratorMux>>,
) -> Result<Vec<String>, GenxError> {
    let mut names = Vec::new();
    for m in &cfg.models {
        if m.name.is_empty() {
            return Err(GenxError::Other(anyhow::anyhow!(
                "segmentor entry missing name"
            )));
        }
        let generator = if m.model.is_empty() {
            return Err(GenxError::Other(anyhow::anyhow!(
                "segmentor entry {:?} missing model (generator pattern)",
                m.name
            )));
        } else {
            m.model.clone()
        };

        let seg = GenXSegmentor::with_generator(
            SegmentorConfig {
                generator,
                prompt_version: None,
            },
            Arc::new(SharedGeneratorMux(Arc::clone(&gen_mux))),
        );
        seg_mux.handle(&m.name, Arc::new(seg))?;
        names.push(m.name.clone());
    }
    Ok(names)
}

fn register_profilers(
    cfg: &ConfigFile,
    prof_mux: &mut ProfilerMux,
    gen_mux: Arc<std::sync::RwLock<GeneratorMux>>,
) -> Result<Vec<String>, GenxError> {
    let mut names = Vec::new();
    for m in &cfg.models {
        if m.name.is_empty() {
            return Err(GenxError::Other(anyhow::anyhow!(
                "profiler entry missing name"
            )));
        }
        let generator = if m.model.is_empty() {
            return Err(GenxError::Other(anyhow::anyhow!(
                "profiler entry {:?} missing model (generator pattern)",
                m.name
            )));
        } else {
            m.model.clone()
        };

        let prof = GenXProfiler::with_generator(
            ProfilerConfig {
                generator,
                prompt_version: None,
            },
            Arc::new(SharedGeneratorMux(Arc::clone(&gen_mux))),
        );
        prof_mux.handle(&m.name, Arc::new(prof))?;
        names.push(m.name.clone());
    }
    Ok(names)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn t13_1_load_yaml_config() {
        let Some(path) = testdata_path("modelloader/config_openai.yaml") else { return };
        let cfg = parse_config(&path).unwrap();
        assert_eq!(cfg.kind.as_deref(), Some("openai"));
        assert!(!cfg.models.is_empty());
    }

    #[test]
    fn t13_2_load_json_config() {
        let yaml_str = r#"{"kind": "openai", "api_key": "test-key", "models": [{"name": "test", "model": "gpt-4"}]}"#;
        let cfg: ConfigFile = serde_json::from_str(yaml_str).unwrap();
        assert_eq!(cfg.kind.as_deref(), Some("openai"));
        assert_eq!(cfg.models[0].name, "test");
    }

    #[test]
    fn t13_3_register_openai() {
        let cfg = ConfigFile {
            kind: Some("openai".into()),
            api_key: Some("test-key-123".into()),
            models: vec![Entry {
                name: "test/gpt4".into(),
                model: "gpt-4o-mini".into(),
                ..default_entry()
            }],
            ..default_config()
        };
        let mut muxes = MuxSet::new();
        let names = register_config(cfg, &mut muxes).unwrap();
        assert_eq!(names, vec!["test/gpt4"]);
    }

    #[test]
    fn t13_4_register_segmentor() {
        let mut muxes = MuxSet::new();
        // First register a generator
        let gen_cfg = ConfigFile {
            kind: Some("openai".into()),
            api_key: Some("test-key".into()),
            models: vec![Entry {
                name: "qwen/turbo".into(),
                model: "gpt-4o-mini".into(),
                ..default_entry()
            }],
            ..default_config()
        };
        register_config(gen_cfg, &mut muxes).unwrap();

        // Then register segmentor referencing that generator
        let seg_cfg = ConfigFile {
            schema: Some("openai/chat/v1".into()),
            type_: Some("segmentor".into()),
            models: vec![Entry {
                name: "seg/default".into(),
                model: "qwen/turbo".into(),
                ..default_entry()
            }],
            ..default_config()
        };
        let names = register_config(seg_cfg, &mut muxes).unwrap();
        assert_eq!(names, vec!["seg/default"]);
        assert!(muxes.segmentors.get("seg/default").is_ok());
    }

    #[test]
    fn t13_5_register_profiler() {
        let mut muxes = MuxSet::new();
        let gen_cfg = ConfigFile {
            kind: Some("openai".into()),
            api_key: Some("test-key".into()),
            models: vec![Entry {
                name: "qwen/turbo".into(),
                model: "gpt-4o-mini".into(),
                ..default_entry()
            }],
            ..default_config()
        };
        register_config(gen_cfg, &mut muxes).unwrap();

        let prof_cfg = ConfigFile {
            schema: Some("openai/chat/v1".into()),
            type_: Some("profiler".into()),
            models: vec![Entry {
                name: "prof/default".into(),
                model: "qwen/turbo".into(),
                ..default_entry()
            }],
            ..default_config()
        };
        let names = register_config(prof_cfg, &mut muxes).unwrap();
        assert_eq!(names, vec!["prof/default"]);
        assert!(muxes.profilers.get("prof/default").is_ok());
    }

    #[test]
    fn t13_6_missing_api_key() {
        let cfg = ConfigFile {
            kind: Some("openai".into()),
            api_key: None,
            models: vec![Entry {
                name: "test".into(),
                model: "gpt-4".into(),
                ..default_entry()
            }],
            ..default_config()
        };
        let mut muxes = MuxSet::new();
        let err = register_config(cfg, &mut muxes).unwrap_err();
        assert!(err.to_string().contains("api_key is required"));
    }

    #[test]
    fn t13_7_unknown_schema() {
        let cfg = ConfigFile {
            schema: Some("unknown/provider/v1".into()),
            type_: Some("generator".into()),
            api_key: Some("key".into()),
            models: vec![Entry {
                name: "test".into(),
                model: "m".into(),
                ..default_entry()
            }],
            ..default_config()
        };
        let mut muxes = MuxSet::new();
        let err = register_config(cfg, &mut muxes).unwrap_err();
        assert!(err.to_string().contains("unknown generator provider"));
    }

    #[test]
    fn t13_8_load_from_dir() {
        let Some(dir) = testdata_dir("modelloader") else { return };
        let mut muxes = MuxSet::new();
        let result = load_from_dir(&dir, &mut muxes);
        // May fail due to missing API keys in env, which is expected (skipped)
        match result {
            Ok(names) => assert!(!names.is_empty()),
            Err(e) => {
                let msg = e.to_string();
                assert!(
                    msg.contains("is required") || msg.contains("api_key"),
                    "unexpected error: {}",
                    msg
                );
            }
        }
    }

    fn testdata_path(rel: &str) -> Option<std::path::PathBuf> {
        let cargo_dir = std::path::Path::new(env!("CARGO_MANIFEST_DIR"));
        let path = cargo_dir.join("../../testdata/genx").join(rel);
        if path.exists() { Some(path) } else { None }
    }

    fn testdata_dir(rel: &str) -> Option<std::path::PathBuf> {
        let cargo_dir = std::path::Path::new(env!("CARGO_MANIFEST_DIR"));
        let path = cargo_dir.join("../../testdata/genx").join(rel);
        if path.is_dir() { Some(path) } else { None }
    }

    fn default_config() -> ConfigFile {
        ConfigFile {
            schema: None, type_: None, kind: None,
            api_key: None, base_url: None,
            models: vec![], app_id: None, token: None,
            cluster: None, model: None, voices: vec![],
            default_params: None,
        }
    }

    fn default_entry() -> Entry {
        Entry {
            name: String::new(), model: String::new(),
            generate_params: None, invoke_params: None,
            support_json_output: false, support_tool_calls: false,
            support_text_only: false, use_system_role: false,
            extra_fields: None, voice: None, resource_id: None,
            desc: None,
        }
    }

    /// Golden test matrix matching Go's modelloader.expandEnv behavior.
    /// Verified against: `go run /tmp/test_expand2.go` with Go 1.25.
    #[test]
    fn t13_expand_env_golden_matrix() {
        unsafe {
            std::env::set_var("_GX_HOME", "/home/user");
            std::env::set_var("_GX_API_KEY", "sk-123");
        }

        // (input, expected) — each pair matches Go expandEnv output
        let cases: &[(&str, &str)] = &[
            ("", ""),
            ("plain text", "plain text"),
            ("$_GX_HOME", "/home/user"),
            ("${_GX_HOME}", "/home/user"),
            ("$_GX_HOME/data", "/home/user/data"),
            ("${_GX_HOME}/data", "/home/user/data"),
            ("$$", ""),                                  // Go: $$ = empty var name → ""
            ("$$_GX_HOME", "_GX_HOME"),                  // Go: $$ → "" then "_GX_HOME" literal
            ("price: $$5", "price: $$5"),                // doesn't start with $ → unchanged
            ("sk-abc$_GX_HOME-xyz", "sk-abc$_GX_HOME-xyz"), // doesn't start with $ → unchanged
            ("$_GX_UNSET_VAR_99999", ""),                // unset var → ""
            ("end$", "end$"),                            // doesn't start with $ → unchanged
            ("$", "$"),                                  // lone $ at end → literal
            ("${_GX_API_KEY}", "sk-123"),
            ("prefix${_GX_API_KEY}suffix", "prefix${_GX_API_KEY}suffix"), // doesn't start with $
        ];

        for (input, expected) in cases {
            assert_eq!(
                expand_env(input), *expected,
                "expand_env({:?}) should be {:?}", input, expected,
            );
        }

        unsafe {
            std::env::remove_var("_GX_HOME");
            std::env::remove_var("_GX_API_KEY");
        }
    }

    /// Test os_expand_env directly — exhaustive golden matrix verified against Go 1.25.
    ///
    /// Each case was produced by running `os.ExpandEnv(input)` in Go and recording
    /// the output. The Rust implementation must match byte-for-byte.
    ///
    /// Uses `_GX_OE_*` env vars (distinct from `_GX_HOME`/`_GX_API_KEY` in expand_env
    /// test) to avoid parallel test env var races.
    #[test]
    fn t13_os_expand_env_full_string() {
        unsafe {
            std::env::set_var("_GX_OE_V", "val");
            std::env::set_var("_GX_OE_H", "/home/user");
        }

        // (input, expected_from_go, description)
        let cases: &[(&str, &str, &str)] = &[
            // Normal identifier expansion
            ("mid$_GX_OE_V/end", "midval/end", "var in middle"),
            ("${_GX_OE_V}!", "val!", "braced var + suffix"),
            ("$_GX_OE_H/data", "/home/user/data", "var + slash suffix"),

            // Trailing / lone dollar
            ("$", "$", "lone dollar"),
            ("no vars", "no vars", "no dollar sign"),

            // Shell special vars (single-char, consumed)
            ("$$", "", "double dollar → empty var"),
            ("$$_GX_OE_H", "_GX_OE_H", "$$ → empty, then literal _GX_OE_H"),
            ("$-abc", "abc", "dash is shell special, consumed"),
            ("$!abc", "abc", "bang is shell special, consumed"),
            ("$?abc", "abc", "question is shell special, consumed"),
            ("$#abc", "abc", "hash is shell special, consumed"),
            ("$@abc", "abc", "at is shell special, consumed"),
            ("$*abc", "abc", "star is shell special, consumed"),
            ("$0abc", "abc", "digit 0 is shell special, consumed"),
            ("$9xyz", "xyz", "digit 9 is shell special, consumed"),

            // Non-identifier, non-shell-special chars (NOT consumed, $ becomes literal)
            ("$.abc", "$.abc", "dot: not consumed, $ literal"),
            ("$,abc", "$,abc", "comma: not consumed, $ literal"),
            ("$/abc", "$/abc", "slash: not consumed, $ literal"),
            ("$ abc", "$ abc", "space: not consumed, $ literal"),
            ("$+abc", "$+abc", "plus: not consumed, $ literal"),
            ("$=abc", "$=abc", "equals: not consumed, $ literal"),
        ];

        for (input, expected, desc) in cases {
            assert_eq!(
                os_expand_env(input), *expected,
                "os_expand_env({:?}) — {}", input, desc,
            );
        }

        unsafe {
            std::env::remove_var("_GX_OE_V");
            std::env::remove_var("_GX_OE_H");
        }
    }

    #[test]
    fn t13_unsupported_extension() {
        let tmp = std::env::temp_dir().join("test_config.txt");
        std::fs::write(&tmp, "data").unwrap();
        let result = parse_config(&tmp);
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("unsupported"));
        let _ = std::fs::remove_file(&tmp);
    }

    #[test]
    fn t13_unknown_type() {
        let cfg = ConfigFile {
            schema: Some("openai/chat/v1".into()),
            type_: Some("unknown_type".into()),
            api_key: Some("key".into()),
            ..default_config()
        };
        let mut muxes = MuxSet::new();
        let err = register_config(cfg, &mut muxes).unwrap_err();
        assert!(err.to_string().contains("unknown type"));
    }

    #[test]
    fn t13_segmentor_missing_model() {
        let mut muxes = MuxSet::new();
        let gen_cfg = ConfigFile {
            kind: Some("openai".into()),
            api_key: Some("key".into()),
            models: vec![Entry { name: "g".into(), model: "m".into(), ..default_entry() }],
            ..default_config()
        };
        register_config(gen_cfg, &mut muxes).unwrap();

        let cfg = ConfigFile {
            schema: Some("openai/chat/v1".into()),
            type_: Some("segmentor".into()),
            models: vec![Entry { name: "seg".into(), model: "".into(), ..default_entry() }],
            ..default_config()
        };
        let err = register_config(cfg, &mut muxes).unwrap_err();
        assert!(err.to_string().contains("missing model"));
    }

    #[test]
    fn t13_profiler_missing_name() {
        let mut muxes = MuxSet::new();
        let gen_cfg = ConfigFile {
            kind: Some("openai".into()),
            api_key: Some("key".into()),
            models: vec![Entry { name: "g".into(), model: "m".into(), ..default_entry() }],
            ..default_config()
        };
        register_config(gen_cfg, &mut muxes).unwrap();

        let cfg = ConfigFile {
            schema: Some("openai/chat/v1".into()),
            type_: Some("profiler".into()),
            models: vec![Entry { name: "".into(), model: "g".into(), ..default_entry() }],
            ..default_config()
        };
        let err = register_config(cfg, &mut muxes).unwrap_err();
        assert!(err.to_string().contains("missing name"));
    }

    #[test]
    fn t13_voice_entry_fields() {
        let yaml = r#"{"name": "tts/test", "voice_id": "zh_female", "desc": "test voice", "cluster": "cn"}"#;
        let ve: VoiceEntry = serde_json::from_str(yaml).unwrap();
        assert_eq!(ve.name, "tts/test");
        assert_eq!(ve.voice_id, "zh_female");
        assert_eq!(ve.desc.as_deref(), Some("test voice"));
        assert_eq!(ve.cluster.as_deref(), Some("cn"));
    }

    #[test]
    fn t13_entry_fields() {
        let json = r#"{
            "name": "test",
            "model": "gpt-4",
            "support_json_output": true,
            "support_tool_calls": true,
            "use_system_role": true,
            "voice": "zh_female",
            "resource_id": "res_123"
        }"#;
        let e: Entry = serde_json::from_str(json).unwrap();
        assert_eq!(e.name, "test");
        assert!(e.support_json_output);
        assert!(e.support_tool_calls);
        assert!(e.use_system_role);
        assert_eq!(e.voice.as_deref(), Some("zh_female"));
        assert_eq!(e.resource_id.as_deref(), Some("res_123"));
    }

    #[test]
    fn t13_parse_segmentor_yaml() {
        let Some(path) = testdata_path("modelloader/config_segmentor.yaml") else { return };
        let cfg = parse_config(&path).unwrap();
        assert_eq!(cfg.schema.as_deref(), Some("openai/chat/v1"));
        assert_eq!(cfg.type_.as_deref(), Some("segmentor"));
        assert!(!cfg.models.is_empty());
    }

    #[test]
    fn t13_no_schema_no_kind() {
        let cfg = ConfigFile {
            ..default_config()
        };
        let mut muxes = MuxSet::new();
        let err = register_config(cfg, &mut muxes).unwrap_err();
        assert!(err.to_string().contains("neither schema nor kind"));
    }
}
