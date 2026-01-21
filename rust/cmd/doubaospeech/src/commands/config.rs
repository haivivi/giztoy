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
/// Configuration is stored in ~/.giztoy/doubaospeech/config.yaml
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
        /// App ID (required)
        #[arg(long)]
        app_id: String,
        /// Bearer Token (access_token)
        #[arg(long)]
        token: Option<String>,
        /// API Key (simple authentication)
        #[arg(long)]
        api_key: Option<String>,
        /// Cluster name (e.g., volcano_tts)
        #[arg(long)]
        cluster: Option<String>,
        /// User ID
        #[arg(long)]
        user_id: Option<String>,
        /// API base URL
        #[arg(long)]
        base_url: Option<String>,
        /// Request timeout in seconds
        #[arg(long)]
        timeout: Option<i32>,
        /// Maximum retries
        #[arg(long)]
        max_retries: Option<i32>,
        /// Default voice type
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
                app_id,
                token,
                api_key,
                cluster,
                user_id,
                base_url,
                timeout,
                max_retries,
                default_voice,
            } => {
                let mut cfg = get_config(cli)?;

                // Require at least one authentication method
                if token.is_none() && api_key.is_none() {
                    anyhow::bail!("either --token or --api-key is required");
                }

                let mut ctx = CliContext {
                    api_key: app_id.clone(), // Store app_id in api_key field for compatibility
                    base_url: base_url.clone().unwrap_or_default(),
                    timeout: timeout.unwrap_or(0),
                    max_retries: max_retries.unwrap_or(0),
                    ..Default::default()
                };

                ctx.set_extra("app_id", app_id);
                if let Some(token) = token {
                    ctx.set_extra("token", token);
                }
                if let Some(api_key) = api_key {
                    ctx.set_extra("api_key", api_key);
                }
                if let Some(cluster) = cluster {
                    ctx.set_extra("cluster", cluster);
                }
                if let Some(user_id) = user_id {
                    ctx.set_extra("user_id", user_id);
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
                println!(
                    "{:<8} {:<20} {:<20} {:<20}",
                    "CURRENT", "NAME", "CLUSTER", "DEFAULT_VOICE"
                );

                let mut names: Vec<_> = cfg.contexts.keys().collect();
                names.sort();

                for name in names {
                    let ctx = cfg.contexts.get(name).unwrap();
                    let current = if name == &cfg.current_context {
                        "*"
                    } else {
                        ""
                    };
                    let cluster = ctx.get_extra("cluster").unwrap_or("-");
                    let default_voice = ctx.get_extra("default_voice").unwrap_or("-");
                    println!("{:<8} {:<20} {:<20} {:<20}", current, name, cluster, default_voice);
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
                        if let Some(app_id) = ctx.get_extra("app_id") {
                            println!("    App ID: {}", app_id);
                        }
                        if let Some(token) = ctx.get_extra("token") {
                            println!("    Token: {}", mask_api_key(token));
                        }
                        if let Some(api_key) = ctx.get_extra("api_key") {
                            println!("    API Key: {}", mask_api_key(api_key));
                        }
                        if let Some(cluster) = ctx.get_extra("cluster") {
                            if !cluster.is_empty() {
                                println!("    Cluster: {}", cluster);
                            }
                        }
                        if !ctx.base_url.is_empty() {
                            println!("    Base URL: {}", ctx.base_url);
                        }
                        if ctx.timeout > 0 {
                            println!("    Timeout: {}s", ctx.timeout);
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
