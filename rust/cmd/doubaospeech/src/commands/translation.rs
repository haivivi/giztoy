//! Translation (simultaneous interpretation) CLI commands

use clap::{Args, Subcommand};
use serde::{Deserialize, Serialize};

use super::{get_context, output_result, print_verbose};
use crate::Cli;

/// Simultaneous translation service
#[derive(Args)]
pub struct TranslationCommand {
    #[command(subcommand)]
    command: TranslationSubcommand,
}

#[derive(Subcommand)]
enum TranslationSubcommand {
    /// Stream simultaneous translation
    Stream {
        /// Source language
        #[arg(short = 's', long, default_value = "zh-CN")]
        source_lang: String,
        /// Target language
        #[arg(short = 't', long, default_value = "en-US")]
        target_lang: String,
    },
    /// Interactive translation mode
    Interactive {
        /// Source language
        #[arg(short = 's', long, default_value = "zh-CN")]
        source_lang: String,
        /// Target language
        #[arg(short = 't', long, default_value = "en-US")]
        target_lang: String,
    },
}

/// Translation config from YAML/JSON file
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct TranslationFileConfig {
    #[serde(skip_serializing_if = "Option::is_none")]
    source_language: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    target_language: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    sample_rate: Option<i32>,
}

impl TranslationCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            TranslationSubcommand::Stream { source_lang, target_lang } => {
                self.stream(cli, source_lang, target_lang).await
            }
            TranslationSubcommand::Interactive { source_lang, target_lang } => {
                self.interactive(cli, source_lang, target_lang).await
            }
        }
    }

    async fn stream(
        &self,
        cli: &Cli,
        source_lang: &str,
        target_lang: &str,
    ) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Source: {} -> Target: {}", source_lang, target_lang));

        // TODO: Implement streaming translation
        eprintln!("[Streaming translation not implemented yet]");
        eprintln!("Would start real-time translation from {} to {}", source_lang, target_lang);

        let result = serde_json::json!({
            "_note": "Streaming translation not implemented yet",
            "source_language": source_lang,
            "target_language": target_lang,
        });
        output_result(&result, cli.output.as_deref(), cli.json)
    }

    async fn interactive(
        &self,
        cli: &Cli,
        source_lang: &str,
        target_lang: &str,
    ) -> anyhow::Result<()> {
        let ctx = get_context(cli)?;
        print_verbose(cli, &format!("Using context: {}", ctx.name));

        // TODO: Implement interactive translation
        eprintln!("[Interactive translation not implemented yet]");
        eprintln!("Would start interactive translation mode");
        eprintln!("Source: {} -> Target: {}", source_lang, target_lang);

        let result = serde_json::json!({
            "_note": "Interactive translation not implemented yet",
            "source_language": source_lang,
            "target_language": target_lang,
        });
        output_result(&result, cli.output.as_deref(), cli.json)
    }
}
