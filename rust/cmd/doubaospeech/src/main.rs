//! Doubao Speech CLI - A command line interface for Doubao Speech API.

use clap::{Parser, Subcommand};

mod commands;

use commands::{
    AsrCommand, ConfigCommand, MediaCommand, MeetingCommand, PodcastCommand,
    RealtimeCommand, TranslationCommand, TtsCommand, VoiceCloneCommand,
};

/// Doubao Speech CLI - A command line interface for Doubao Speech API.
///
/// This tool allows you to interact with Doubao Speech API services including:
///   - TTS (Text-to-Speech): Voice synthesis
///   - ASR (Speech Recognition): Speech-to-text
///   - Voice Clone: Custom voice training
///   - Realtime: Real-time voice conversation
///   - Meeting: Meeting transcription
///   - Podcast: Multi-speaker podcast synthesis
///   - Media: Audio/video subtitle extraction
///   - Translation: Simultaneous interpretation
///
/// Configuration is stored in ~/.giztoy/doubao/ and supports multiple contexts,
/// similar to kubectl's context management.
#[derive(Parser)]
#[command(name = "doubaospeech")]
#[command(about = "Doubao Speech API CLI tool")]
#[command(version)]
pub struct Cli {
    /// Config file (default is ~/.giztoy/doubao/config.yaml)
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
    /// TTS (Text-to-Speech) service
    Tts(TtsCommand),
    /// ASR (Automatic Speech Recognition) service
    Asr(AsrCommand),
    /// Voice Clone service
    #[command(name = "voice")]
    VoiceClone(VoiceCloneCommand),
    /// Realtime voice conversation service
    Realtime(RealtimeCommand),
    /// Meeting transcription service
    Meeting(MeetingCommand),
    /// Podcast synthesis service
    Podcast(PodcastCommand),
    /// Media processing service
    Media(MediaCommand),
    /// Simultaneous translation service
    Translation(TranslationCommand),
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();

    match &cli.command {
        Commands::Config(cmd) => cmd.run(&cli).await,
        Commands::Tts(cmd) => cmd.run(&cli).await,
        Commands::Asr(cmd) => cmd.run(&cli).await,
        Commands::VoiceClone(cmd) => cmd.run(&cli).await,
        Commands::Realtime(cmd) => cmd.run(&cli).await,
        Commands::Meeting(cmd) => cmd.run(&cli).await,
        Commands::Podcast(cmd) => cmd.run(&cli).await,
        Commands::Media(cmd) => cmd.run(&cli).await,
        Commands::Translation(cmd) => cmd.run(&cli).await,
    }
}
