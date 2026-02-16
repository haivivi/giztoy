//! Utility functions for CLI commands.

use std::path::Path;

use super::cli_config::{load_config, Config, Context};
use giztoy_dashscope::Client;

use crate::Cli;

const APP_NAME: &str = "dashscope";

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
                    "no context specified. Use -c flag or set a default context with 'dashscope config use-context'"
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
pub fn print_error(msg: &str) {
    eprintln!("\x1b[31m✗\x1b[0m {}", msg);
}

/// Prints info message.
pub fn print_info(msg: &str) {
    eprintln!("\x1b[34mℹ\x1b[0m {}", msg);
}

/// Prints warning message.
pub fn print_warning(msg: &str) {
    eprintln!("\x1b[33m⚠\x1b[0m {}", msg);
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

/// Creates a DashScope API client from context configuration.
pub fn create_client(ctx: &Context) -> anyhow::Result<Client> {
    let mut builder = Client::builder(&ctx.api_key);

    if !ctx.base_url.is_empty() {
        builder = builder.base_url(&ctx.base_url);
    }

    // Check for workspace in extra
    if let Some(workspace) = ctx.get_extra("workspace") {
        if !workspace.is_empty() {
            builder = builder.workspace(workspace);
        }
    }

    Ok(builder.build()?)
}
