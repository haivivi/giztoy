//! DashScope CLI - A command line interface for DashScope (Aliyun Model Studio) API.

use clap::{Parser, Subcommand};

mod commands;

use commands::{ConfigCommand, OmniCommand};

/// DashScope CLI - A command line interface for DashScope API.
///
/// This tool allows you to interact with DashScope's AI services including:
///   - Qwen-Omni-Realtime multimodal conversation (text + audio)
///
/// Configuration is stored in ~/.giztoy/dashscope/ and supports multiple contexts,
/// similar to kubectl's context management.
#[derive(Parser)]
#[command(name = "dashscope")]
#[command(about = "DashScope (Aliyun Model Studio) API CLI tool")]
#[command(version)]
pub struct Cli {
    /// Config file (default is ~/.giztoy/dashscope/config.yaml)
    #[arg(long, global = true)]
    pub config: Option<String>,

    /// Context name to use
    #[arg(short = 'c', long, global = true)]
    pub context: Option<String>,

    /// Output file (default: stdout)
    #[arg(short = 'o', long, global = true)]
    pub output: Option<String>,

    /// Input request file (YAML or JSON)
    #[arg(short = 'f', long = "file", global = true)]
    pub input: Option<String>,

    /// Output as JSON (for piping)
    #[arg(long, global = true)]
    pub json: bool,

    /// Verbose output
    #[arg(short = 'v', long, global = true)]
    pub verbose: bool,

    #[command(subcommand)]
    pub command: Commands,
}

#[derive(Subcommand)]
pub enum Commands {
    /// Manage CLI configuration
    Config(ConfigCommand),
    /// Qwen-Omni-Realtime multimodal conversation
    Omni(OmniCommand),
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();

    // Setup logging
    if cli.verbose {
        tracing_subscriber::fmt()
            .with_max_level(tracing::Level::DEBUG)
            .with_target(false)
            .init();
    }

    match &cli.command {
        Commands::Config(cmd) => cmd.run(&cli).await,
        Commands::Omni(cmd) => cmd.run(&cli).await,
    }
}
