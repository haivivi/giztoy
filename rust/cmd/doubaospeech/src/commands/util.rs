//! Utility functions for CLI commands.

use std::path::Path;

use giztoy_cli::config::{load_config, Config, Context};
use giztoy_doubaospeech::Client;

use crate::Cli;

const APP_NAME: &str = "doubao";

/// Gets the global configuration.
pub fn get_config(cli: &Cli) -> anyhow::Result<Config> {
    load_config(APP_NAME, cli.config.as_deref())
}

/// Gets the context configuration to use.
pub fn get_context(cli: &Cli) -> anyhow::Result<Context> {
    let cfg = get_config(cli)?;

    match cfg.resolve_context(cli.context.as_deref()) {
        Some(ctx) => Ok(ctx.clone()),
        None => {
            if cli.context.is_none() {
                anyhow::bail!(
                    "no context specified. Use -c flag or set a default context with 'doubaospeech config use-context'"
                );
            }
            anyhow::bail!("context '{}' not found", cli.context.as_deref().unwrap());
        }
    }
}

/// Loads a request from a YAML or JSON file.
pub fn load_request<T: serde::de::DeserializeOwned>(path: &str) -> anyhow::Result<T> {
    let content = std::fs::read_to_string(path)?;
    let ext = Path::new(path)
        .extension()
        .and_then(|e| e.to_str())
        .unwrap_or("yaml");

    let result = match ext.to_lowercase().as_str() {
        "json" => serde_json::from_str(&content)?,
        _ => serde_yaml::from_str(&content)?,
    };

    Ok(result)
}

/// Requires input file to be provided.
pub fn require_input_file(cli: &Cli) -> anyhow::Result<&str> {
    cli.input
        .as_deref()
        .ok_or_else(|| anyhow::anyhow!("input file is required, use -f flag"))
}

/// Outputs binary data to a file.
pub fn output_bytes(data: &[u8], output_path: &str) -> anyhow::Result<()> {
    std::fs::write(output_path, data)?;
    Ok(())
}

/// Outputs result as JSON or YAML.
pub fn output_result<T: serde::Serialize>(
    result: &T,
    output_path: Option<&str>,
    as_json: bool,
) -> anyhow::Result<()> {
    let output = if as_json {
        serde_json::to_string_pretty(result)?
    } else {
        serde_yaml::to_string(result)?
    };

    match output_path {
        Some(path) => std::fs::write(path, output)?,
        None => print!("{}", output),
    }

    Ok(())
}

/// Prints verbose output if enabled.
pub fn print_verbose(cli: &Cli, msg: &str) {
    if cli.verbose {
        eprintln!("[verbose] {}", msg);
    }
}

/// Prints success message.
pub fn print_success(msg: &str) {
    eprintln!("\x1b[32m✓\x1b[0m {}", msg);
}

/// Prints error message.
#[allow(dead_code)]
pub fn print_error(msg: &str) {
    eprintln!("\x1b[31m✗\x1b[0m {}", msg);
}

/// Prints info message.
#[allow(dead_code)]
pub fn print_info(msg: &str) {
    eprintln!("\x1b[34mℹ\x1b[0m {}", msg);
}

/// Formats bytes to human readable string.
pub fn format_bytes(bytes: usize) -> String {
    const KB: usize = 1024;
    const MB: usize = KB * 1024;
    const GB: usize = MB * 1024;

    if bytes >= GB {
        format!("{:.2} GB", bytes as f64 / GB as f64)
    } else if bytes >= MB {
        format!("{:.2} MB", bytes as f64 / MB as f64)
    } else if bytes >= KB {
        format!("{:.2} KB", bytes as f64 / KB as f64)
    } else {
        format!("{} B", bytes)
    }
}

/// Formats duration in milliseconds to human readable string.
#[allow(dead_code)]
pub fn format_duration(ms: i64) -> String {
    if ms >= 60000 {
        format!("{:.1}m", ms as f64 / 60000.0)
    } else if ms >= 1000 {
        format!("{:.1}s", ms as f64 / 1000.0)
    } else {
        format!("{}ms", ms)
    }
}

/// Creates a Doubao Speech API client from context configuration.
pub fn create_client(ctx: &Context) -> anyhow::Result<Client> {
    // Get app_id from client credentials or extra fields
    let app_id = if let Some(ref client) = ctx.client {
        client.app_id.as_str()
    } else {
        ctx.get_extra("app_id").unwrap_or(&ctx.api_key)
    };

    if app_id.is_empty() {
        anyhow::bail!("app_id not found in context '{}'", ctx.name);
    }

    let mut builder = Client::builder(app_id);

    // Set authentication - check for token (bearer) or api_key
    // Priority: client.api_key > extra["token"] > extra["api_key"]
    let has_auth = if let Some(ref client) = ctx.client {
        if !client.api_key.is_empty() {
            // client.api_key is used as bearer token for doubao
            builder = builder.bearer_token(&client.api_key);
            true
        } else {
            false
        }
    } else {
        false
    };

    if !has_auth {
        // Fall back to extra fields
        if let Some(token) = ctx.get_extra("token") {
            builder = builder.bearer_token(token);
        } else if let Some(api_key) = ctx.get_extra("api_key") {
            builder = builder.api_key(api_key);
        } else {
            anyhow::bail!("no token or api_key found in context '{}'", ctx.name);
        }
    }

    // Set cluster if specified
    if let Some(cluster) = ctx.get_extra("cluster") {
        builder = builder.cluster(cluster);
    }

    // Set user_id if specified
    if let Some(user_id) = ctx.get_extra("user_id") {
        builder = builder.user_id(user_id);
    }

    if !ctx.base_url.is_empty() {
        builder = builder.base_url(&ctx.base_url);
    }

    if ctx.max_retries > 0 {
        builder = builder.max_retries(ctx.max_retries as u32);
    }

    Ok(builder.build()?)
}
