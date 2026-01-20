//! MiniMax CLI - A command line interface for MiniMax API.

use clap::{Parser, Subcommand};

mod commands;

use commands::{
    ConfigCommand, FileCommand, ImageCommand, MusicCommand, SpeechCommand, TextCommand,
    VideoCommand, VoiceCommand,
};

/// MiniMax CLI - A command line interface for MiniMax API.
///
/// This tool allows you to interact with MiniMax's AI services including:
///   - Text generation (chat completions)
///   - Speech synthesis (TTS)
///   - Video generation (T2V, I2V)
///   - Image generation
///   - Music generation
///   - Voice management (clone, design)
///   - File management
///
/// Configuration is stored in ~/.giztoy/minimax/ and supports multiple contexts,
/// similar to kubectl's context management.
#[derive(Parser)]
#[command(name = "minimax")]
#[command(about = "MiniMax API CLI tool")]
#[command(version)]
pub struct Cli {
    /// Config file (default is ~/.giztoy/minimax/config.yaml)
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
    /// Text generation service
    Text(TextCommand),
    /// Speech synthesis service
    Speech(SpeechCommand),
    /// Video generation service
    Video(VideoCommand),
    /// Image generation service
    Image(ImageCommand),
    /// Music generation service
    Music(MusicCommand),
    /// Voice management service
    Voice(VoiceCommand),
    /// File management service
    File(FileCommand),
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();

    match &cli.command {
        Commands::Config(cmd) => cmd.run(&cli).await,
        Commands::Text(cmd) => cmd.run(&cli).await,
        Commands::Speech(cmd) => cmd.run(&cli).await,
        Commands::Video(cmd) => cmd.run(&cli).await,
        Commands::Image(cmd) => cmd.run(&cli).await,
        Commands::Music(cmd) => cmd.run(&cli).await,
        Commands::Voice(cmd) => cmd.run(&cli).await,
        Commands::File(cmd) => cmd.run(&cli).await,
    }
}
