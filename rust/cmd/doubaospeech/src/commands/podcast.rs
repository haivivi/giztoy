//! Podcast synthesis CLI commands
//!
//! Supports two API types:
//!   - http: Async HTTP API (submit and poll)
//!   - sami: Real-time WebSocket streaming (recommended)
//!
//! SAMI Podcast requires specific speakers with _v2_saturn_bigtts suffix:
//!   - zh_male_dayixiansheng_v2_saturn_bigtts
//!   - zh_female_mizaitongxue_v2_saturn_bigtts
//!   - zh_male_liufei_v2_saturn_bigtts
//!   - zh_male_xiaolei_v2_saturn_bigtts

use clap::{Args, Subcommand};
use serde::{Deserialize, Serialize};

use giztoy_doubaospeech::{
    PodcastAudioConfig, PodcastLine, PodcastSAMIRequest, PodcastSpeakerInfo, PodcastTaskRequest,
};

use super::{create_client, get_context, load_request, output_result, print_success, print_verbose};
use crate::Cli;

/// Podcast synthesis service.
///
/// Use `podcast http` for async HTTP API or `podcast sami` for WebSocket streaming (recommended).
#[derive(Args)]
pub struct PodcastCommand {
    #[command(subcommand)]
    command: PodcastSubcommand,
}

#[derive(Subcommand)]
enum PodcastSubcommand {
    /// Podcast HTTP API (async)
    Http(PodcastHttpCommand),
    /// SAMI Podcast WebSocket streaming (recommended)
    Sami,
}

// ============================================================================
// HTTP Commands
// ============================================================================

#[derive(Args)]
pub struct PodcastHttpCommand {
    #[command(subcommand)]
    command: PodcastHttpSubcommand,
}

#[derive(Subcommand)]
enum PodcastHttpSubcommand {
    /// Submit async podcast task
    Submit,
    /// Query podcast task status
    Status {
        /// Task ID to query
        task_id: String,
    },
}

/// Podcast HTTP request from YAML/JSON file
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct PodcastHttpFileRequest {
    #[serde(default)]
    lines: Vec<PodcastLineConfig>,
    #[serde(skip_serializing_if = "Option::is_none")]
    callback_url: Option<String>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct PodcastLineConfig {
    speaker_id: String,
    text: String,
}

/// SAMI Podcast request from YAML/JSON file
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct PodcastSamiFileRequest {
    /// Action type: 0 = summary generation, 1 = direct script
    #[serde(default)]
    action: i32,
    /// Input text
    input_text: String,
    /// Audio config
    #[serde(skip_serializing_if = "Option::is_none")]
    audio_config: Option<AudioConfigRequest>,
    /// Speaker info
    #[serde(skip_serializing_if = "Option::is_none")]
    speaker_info: Option<SpeakerInfoRequest>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct AudioConfigRequest {
    format: Option<String>,
    sample_rate: Option<i32>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct SpeakerInfoRequest {
    random_order: Option<bool>,
    speakers: Vec<String>,
}

impl PodcastCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            PodcastSubcommand::Http(http) => http.run(cli).await,
            PodcastSubcommand::Sami => run_sami(cli).await,
        }
    }
}

impl PodcastHttpCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            PodcastHttpSubcommand::Submit => submit_http(cli).await,
            PodcastHttpSubcommand::Status { task_id } => status_http(cli, task_id).await,
        }
    }
}

// ============================================================================
// HTTP Implementation Functions
// ============================================================================

async fn submit_http(cli: &Cli) -> anyhow::Result<()> {
    let ctx = get_context(cli)?;
    let client = create_client(&ctx)?;

    let input_file = cli.input.as_deref()
        .ok_or_else(|| anyhow::anyhow!("input file is required, use -f flag"))?;

    let file_req: PodcastHttpFileRequest = load_request(input_file)?;

    let script: Vec<PodcastLine> = file_req.lines.into_iter().map(|l| PodcastLine {
        speaker_id: l.speaker_id,
        text: l.text,
        emotion: None,
        speed_ratio: None,
    }).collect();

    if script.is_empty() {
        return Err(anyhow::anyhow!("at least one line is required"));
    }

    let req = PodcastTaskRequest {
        script: script.clone(),
        callback_url: file_req.callback_url,
        ..Default::default()
    };

    print_verbose(cli, &format!("Using HTTP API (async)"));
    print_verbose(cli, &format!("Lines: {}", script.len()));

    let result = client.podcast().create_task(&req).await?;

    print_success(&format!("Task created: {}", result.task_id));

    output_result(&result, cli.output.as_deref(), cli.json)
}

async fn status_http(cli: &Cli, task_id: &str) -> anyhow::Result<()> {
    let ctx = get_context(cli)?;
    let client = create_client(&ctx)?;

    print_verbose(cli, &format!("Using context: {}", ctx.name));
    print_verbose(cli, &format!("Querying task: {}", task_id));

    let result = client.podcast().get_task(task_id).await?;

    print_success(&format!("Task status: {:?}", result.status));

    output_result(&result, cli.output.as_deref(), cli.json)
}

// ============================================================================
// SAMI Implementation Functions
// ============================================================================

async fn run_sami(cli: &Cli) -> anyhow::Result<()> {
    let ctx = get_context(cli)?;
    let client = create_client(&ctx)?;
    print_verbose(cli, &format!("Using SAMI Podcast API (WebSocket streaming)"));

    let output_path = cli.output.as_deref()
        .ok_or_else(|| anyhow::anyhow!("output file is required, use -o flag"))?;

    let input_file = cli.input.as_deref()
        .ok_or_else(|| anyhow::anyhow!("input file is required, use -f flag"))?;

    let file_req: PodcastSamiFileRequest = load_request(input_file)?;

    print_verbose(cli, &format!("Input text length: {} characters", file_req.input_text.len()));
    if let Some(ref speaker_info) = file_req.speaker_info {
        print_verbose(cli, &format!("Speakers: {:?}", speaker_info.speakers));
    }

    // Convert to SDK request
    let req = PodcastSAMIRequest {
        action: file_req.action,
        input_text: Some(file_req.input_text.clone()),
        audio_config: file_req.audio_config.map(|ac| PodcastAudioConfig {
            format: ac.format,
            sample_rate: ac.sample_rate,
            speech_rate: None,
        }),
        speaker_info: file_req.speaker_info.map(|si| PodcastSpeakerInfo {
            speakers: si.speakers,
            random_order: si.random_order,
        }),
        ..Default::default()
    };

    // Note: SAMI Podcast with action=0 (summary generation) requires LLM processing
    // which can take several minutes
    if req.action == 0 {
        eprintln!("ℹ 📝 Generating podcast summary (this may take 2-5 minutes for LLM processing)...");
    }

    let session = client.podcast().stream_sami(&req).await?;
    print_verbose(cli, "Session opened");

    let mut audio_buf = Vec::new();
    let mut chunk_count = 0;
    let mut round_count = 0;
    let start_time = std::time::Instant::now();

    while let Some(result) = session.recv().await {
        let chunk = result?;

        if !chunk.audio.is_empty() {
            audio_buf.extend_from_slice(&chunk.audio);
            chunk_count += 1;
            let elapsed = start_time.elapsed().as_secs_f64();
            print_verbose(cli, &format!(
                "[{:.1}s] 🔊 +{} bytes (total: {:.1} KB)",
                elapsed,
                chunk.audio.len(),
                audio_buf.len() as f64 / 1024.0
            ));
        }

        // Show progress for events
        if !chunk.event.is_empty() {
            let elapsed = start_time.elapsed().as_secs_f64();
            match chunk.event.as_str() {
                "SessionStarted" => {
                    print_verbose(cli, &format!("[{:.1}s] ✅ Session started", elapsed));
                }
                "PodcastRoundStart" => {
                    round_count += 1;
                    eprintln!("ℹ [{:.1}s] 🎙️ Starting round {}...", elapsed, round_count);
                }
                "PodcastRoundEnd" => {
                    print_verbose(cli, &format!("[{:.1}s] ✅ Round {} completed", elapsed, round_count));
                }
                "PodcastEnd" => {
                    eprintln!("ℹ [{:.1}s] 🎉 Podcast generation completed!", elapsed);
                }
                _ => {
                    print_verbose(cli, &format!("[{:.1}s] Event: {}", elapsed, chunk.event));
                }
            }
        }

        if chunk.is_last {
            break;
        }
    }

    session.close().await?;

    // Write audio to file
    if !audio_buf.is_empty() {
        super::output_bytes(&audio_buf, output_path)?;
        print_success(&format!(
            "Audio saved to: {} ({:.1} KB)",
            output_path,
            audio_buf.len() as f64 / 1024.0
        ));
    } else {
        return Err(anyhow::anyhow!("No audio data received"));
    }

    let result = serde_json::json!({
        "api_type": "sami",
        "audio_size": audio_buf.len(),
        "chunks": chunk_count,
        "rounds": round_count,
        "duration_s": start_time.elapsed().as_secs_f64(),
        "output_file": output_path,
    });

    output_result(&result, None, cli.json)
}
