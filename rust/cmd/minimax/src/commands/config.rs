//! Configuration management commands.
//!
//! Compatible with Go version's config commands.

use clap::{Args, Subcommand};

use giztoy_cli::config::{mask_api_key, Context as CliContext};

use super::{get_config, print_success};
use crate::Cli;

/// Manage CLI configuration.
///
/// Contexts allow you to manage multiple API configurations,
/// similar to kubectl's context management.
///
/// Configuration is stored in ~/.giztoy/minimax/config.yaml
#[derive(Args)]
pub struct ConfigCommand {
    #[command(subcommand)]
    command: ConfigSubcommand,
}

#[derive(Subcommand)]
enum ConfigSubcommand {
    /// Add a new context
    #[command(name = "add-context")]
    AddContext {
        /// Context name
        name: String,
        /// API key (required)
        #[arg(long)]
        api_key: String,
        /// API base URL
        #[arg(long)]
        base_url: Option<String>,
        /// Request timeout in seconds
        #[arg(long)]
        timeout: Option<i32>,
        /// Maximum retries
        #[arg(long)]
        max_retries: Option<i32>,
        /// Default model
        #[arg(long)]
        default_model: Option<String>,
        /// Default voice ID
        #[arg(long)]
        default_voice: Option<String>,
    },
    /// Delete a context
    #[command(name = "delete-context")]
    DeleteContext {
        /// Context name
        name: String,
    },
    /// Set the current context
    #[command(name = "use-context")]
    UseContext {
        /// Context name
        name: String,
    },
    /// Display the current context
    #[command(name = "get-context")]
    GetContext,
    /// List all contexts
    #[command(name = "list-contexts", alias = "get-contexts")]
    ListContexts,
    /// View the current configuration
    View,
}

impl ConfigCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            ConfigSubcommand::AddContext {
                name,
                api_key,
                base_url,
                timeout,
                max_retries,
                default_model,
                default_voice,
            } => {
                let mut cfg = get_config(cli)?;

                let mut ctx = CliContext {
                    api_key: api_key.clone(),
                    base_url: base_url.clone().unwrap_or_default(),
                    timeout: timeout.unwrap_or(0),
                    max_retries: max_retries.unwrap_or(0),
                    ..Default::default()
                };

                if let Some(model) = default_model {
                    ctx.set_extra("default_model", model);
                }
                if let Some(voice) = default_voice {
                    ctx.set_extra("default_voice", voice);
                }

                cfg.add_context(name, ctx)?;
                print_success(&format!("Context \"{}\" added successfully", name));
                Ok(())
            }

            ConfigSubcommand::DeleteContext { name } => {
                let mut cfg = get_config(cli)?;
                cfg.delete_context(name)?;
                print_success(&format!("Context \"{}\" deleted", name));
                Ok(())
            }

            ConfigSubcommand::UseContext { name } => {
                let mut cfg = get_config(cli)?;
                cfg.use_context(name)?;
                print_success(&format!("Switched to context \"{}\"", name));
                Ok(())
            }

            ConfigSubcommand::GetContext => {
                let cfg = get_config(cli)?;
                if cfg.current_context.is_empty() {
                    println!("No current context set");
                } else {
                    println!("{}", cfg.current_context);
                }
                Ok(())
            }

            ConfigSubcommand::ListContexts => {
                let cfg = get_config(cli)?;

                if cfg.contexts.is_empty() {
                    println!("No contexts configured");
                    return Ok(());
                }

                // Print table header
                println!("{:<8} {:<20} {:<30} {}", "CURRENT", "NAME", "BASE_URL", "DEFAULT_MODEL");

                let mut names: Vec<_> = cfg.contexts.keys().collect();
                names.sort();

                for name in names {
                    let ctx = cfg.contexts.get(name).unwrap();
                    let current = if name == &cfg.current_context { "*" } else { "" };
                    let base_url = if ctx.base_url.is_empty() {
                        "(default)"
                    } else {
                        &ctx.base_url
                    };
                    let default_model = ctx.get_extra("default_model").unwrap_or("");
                    println!("{:<8} {:<20} {:<30} {}", current, name, base_url, default_model);
                }

                Ok(())
            }

            ConfigSubcommand::View => {
                let cfg = get_config(cli)?;

                println!("Config file: {}", cfg.path().display());
                println!("Current context: {}", cfg.current_context);
                println!("Contexts: {}", cfg.contexts.len());

                if !cfg.contexts.is_empty() {
                    println!("\nContext details:");

                    let mut names: Vec<_> = cfg.contexts.keys().collect();
                    names.sort();

                    for name in names {
                        let ctx = cfg.contexts.get(name).unwrap();
                        println!("\n  {}:", name);
                        println!("    API Key: {}", mask_api_key(&ctx.api_key));
                        if !ctx.base_url.is_empty() {
                            println!("    Base URL: {}", ctx.base_url);
                        }
                        if ctx.timeout > 0 {
                            println!("    Timeout: {}s", ctx.timeout);
                        }
                        if let Some(model) = ctx.get_extra("default_model") {
                            if !model.is_empty() {
                                println!("    Default Model: {}", model);
                            }
                        }
                        if let Some(voice) = ctx.get_extra("default_voice") {
                            if !voice.is_empty() {
                                println!("    Default Voice: {}", voice);
                            }
                        }
                    }
                }

                Ok(())
            }
        }
    }
}
