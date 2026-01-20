//! Output utilities for CLI tools.

use std::{fs::File, io::Write};

use serde::Serialize;

/// Output format.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum OutputFormat {
    /// YAML format (default).
    #[default]
    Yaml,
    /// JSON format.
    Json,
}

/// Output configuration.
pub struct Output {
    pub format: OutputFormat,
    pub file: Option<String>,
}

impl Output {
    /// Creates a new output configuration.
    pub fn new(format: OutputFormat, file: Option<String>) -> Self {
        Self { format, file }
    }

    /// Outputs the result.
    pub fn write<T: Serialize>(&self, value: &T) -> anyhow::Result<()> {
        let output = match self.format {
            OutputFormat::Yaml => serde_yaml::to_string(value)?,
            OutputFormat::Json => serde_json::to_string_pretty(value)?,
        };

        match &self.file {
            Some(path) => {
                let mut file = File::create(path)?;
                file.write_all(output.as_bytes())?;
            }
            None => {
                println!("{}", output);
            }
        }

        Ok(())
    }

    /// Writes binary data to a file.
    pub fn write_binary(&self, data: &[u8], path: &str) -> anyhow::Result<()> {
        let mut file = File::create(path)?;
        file.write_all(data)?;
        Ok(())
    }
}

/// Prints verbose output if enabled.
pub fn print_verbose(enabled: bool, message: &str) {
    if enabled {
        eprintln!("[verbose] {}", message);
    }
}

/// Guesses the output file extension based on format.
pub fn guess_extension(format: &str) -> &str {
    match format.to_lowercase().as_str() {
        "mp3" => "mp3",
        "wav" => "wav",
        "flac" => "flac",
        "pcm" => "pcm",
        "mp4" => "mp4",
        "png" => "png",
        "jpg" | "jpeg" => "jpg",
        _ => "bin",
    }
}
