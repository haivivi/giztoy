//! TTS (Text-to-Speech) commands.
//!
//! Supports two API versions:
//!   - v1: Classic API (/api/v1/tts)
//!   - v2: BigModel API (/api/v3/tts/*) - Recommended
//!
//! IMPORTANT: Speaker voice suffix must match Resource ID!
//!   | Resource ID    | Speaker Suffix Required | Example                        |
//!   |----------------|-------------------------|--------------------------------|
//!   | seed-tts-2.0   | *_uranus_bigtts         | zh_female_xiaohe_uranus_bigtts |
//!   | seed-tts-1.0   | *_moon_bigtts           | zh_female_shuangkuaisisi_moon_bigtts |
//!
//! Common Error: "resource ID is mismatched with speaker related resource"
//! This means your speaker suffix doesn't match the resource_id!

use std::pin::pin;

use clap::{Args, Subcommand};
use futures::StreamExt;
use serde::{Deserialize, Serialize};

use giztoy_doubaospeech::{AudioEncoding, Language, SampleRate, TtsRequest, TtsTextType, TtsV2Request};

use super::{
    create_client, format_bytes, get_context, load_request, output_bytes, output_result,
    print_success, print_verbose,
};
use crate::Cli;

/// TTS (Text-to-Speech) service.
///
/// Supports synchronous and streaming speech synthesis.
/// Use `tts v1` for classic API or `tts v2` for BigModel API (recommended).
#[derive(Args)]
pub struct TtsCommand {
    #[command(subcommand)]
    command: TtsSubcommand,
}

#[derive(Subcommand)]
enum TtsSubcommand {
    /// TTS V1 API (Classic)
    V1(TtsV1Command),
    /// TTS V2 API (BigModel) - Recommended
    V2(TtsV2Command),
}

// ============================================================================
// V1 Commands
// ============================================================================

#[derive(Args)]
pub struct TtsV1Command {
    #[command(subcommand)]
    command: TtsV1Subcommand,
}

#[derive(Subcommand)]
enum TtsV1Subcommand {
    /// V1 synchronous synthesis
    Synthesize {
        /// Text to synthesize (alternative to -f file)
        #[arg(short = 't', long)]
        text: Option<String>,
        /// Voice type (e.g., zh_female_cancan)
        #[arg(short = 'V', long)]
        voice: Option<String>,
        /// Audio encoding format (pcm, mp3, wav, ogg_opus)
        #[arg(short = 'e', long)]
        encoding: Option<String>,
        /// Cluster name (e.g., volcano_tts)
        #[arg(long)]
        cluster: Option<String>,
    },
    /// V1 streaming synthesis
    Stream {
        /// Text to synthesize (alternative to -f file)
        #[arg(short = 't', long)]
        text: Option<String>,
        /// Voice type
        #[arg(short = 'V', long)]
        voice: Option<String>,
        /// Audio encoding format
        #[arg(short = 'e', long)]
        encoding: Option<String>,
        /// Cluster name
        #[arg(long)]
        cluster: Option<String>,
    },
}

// ============================================================================
// V2 Commands
// ============================================================================

#[derive(Args)]
pub struct TtsV2Command {
    #[command(subcommand)]
    command: TtsV2Subcommand,
}

#[derive(Subcommand)]
enum TtsV2Subcommand {
    /// V2 HTTP streaming synthesis (recommended)
    Stream {
        /// Text to synthesize (alternative to -f file)
        #[arg(short = 't', long)]
        text: Option<String>,
        /// Speaker voice (must match resource_id suffix!)
        #[arg(short = 'V', long)]
        voice: Option<String>,
        /// Audio format (pcm, mp3, ogg_opus)
        #[arg(short = 'e', long)]
        encoding: Option<String>,
    },
    /// V2 WebSocket unidirectional streaming
    Ws {
        /// Text to synthesize (alternative to -f file)
        #[arg(short = 't', long)]
        text: Option<String>,
        /// Speaker voice
        #[arg(short = 'V', long)]
        voice: Option<String>,
    },
    /// V2 WebSocket bidirectional streaming
    Bidirectional {
        /// Speaker voice
        #[arg(short = 'V', long)]
        voice: Option<String>,
    },
    /// V2 async long text synthesis
    Async {
        /// Text to synthesize (alternative to -f file)
        #[arg(short = 't', long)]
        text: Option<String>,
        /// Speaker voice
        #[arg(short = 'V', long)]
        voice: Option<String>,
        /// Callback URL for task completion notification
        #[arg(long)]
        callback_url: Option<String>,
    },
    /// Query async task status
    Status {
        /// Task ID to query
        task_id: String,
    },
}

/// TTS request from YAML/JSON file.
/// 
/// Supports both V1 format (voice_type) and V2 format (speaker).
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
struct TtsFileRequest {
    text: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    text_type: Option<String>,
    /// Voice type for V1 API (e.g., "zh_female_cancan")
    #[serde(default, skip_serializing_if = "String::is_empty")]
    voice_type: String,
    /// Speaker for V2 API (e.g., "zh_female_vv_uranus_bigtts")
    /// This is an alias for voice_type for compatibility with Go CLI V2 format.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    speaker: Option<String>,
    /// Resource ID for V2 API (e.g., "seed-tts-2.0")
    /// IMPORTANT: Must match speaker suffix!
    #[serde(default, skip_serializing_if = "Option::is_none")]
    resource_id: Option<String>,
    /// Audio format (mp3, pcm, ogg_opus) - V2 style
    #[serde(skip_serializing_if = "Option::is_none")]
    format: Option<String>,
    /// Sample rate (8000, 16000, 24000, 32000) - V2 style
    #[serde(skip_serializing_if = "Option::is_none")]
    sample_rate: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    cluster: Option<String>,
    /// Encoding for V1 API (pcm, mp3, wav, ogg_opus)
    #[serde(skip_serializing_if = "Option::is_none")]
    encoding: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    speed_ratio: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    volume_ratio: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pitch_ratio: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    emotion: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    language: Option<String>,
}

impl TtsFileRequest {
    /// Gets the voice type, preferring speaker (V2) over voice_type (V1).
    fn get_voice_type(&self) -> String {
        // V2 format uses "speaker", V1 format uses "voice_type"
        if let Some(ref speaker) = self.speaker {
            if !speaker.is_empty() {
                return speaker.clone();
            }
        }
        self.voice_type.clone()
    }

    /// Gets the encoding, preferring format (V2) over encoding (V1).
    fn get_encoding(&self) -> Option<AudioEncoding> {
        // V2 format uses "format", V1 format uses "encoding"
        let encoding_str = self.format.as_ref().or(self.encoding.as_ref())?;
        match encoding_str.as_str() {
            "pcm" => Some(AudioEncoding::Pcm),
            "wav" => Some(AudioEncoding::Wav),
            "mp3" => Some(AudioEncoding::Mp3),
            "ogg_opus" => Some(AudioEncoding::OggOpus),
            "aac" => Some(AudioEncoding::Aac),
            "m4a" => Some(AudioEncoding::M4a),
            _ => None,
        }
    }

    fn to_tts_request(&self) -> TtsRequest {
        TtsRequest {
            text: self.text.clone(),
            text_type: self.text_type.as_ref().and_then(|t| match t.as_str() {
                "ssml" => Some(TtsTextType::Ssml),
                _ => Some(TtsTextType::Plain),
            }),
            voice_type: self.get_voice_type(),
            cluster: self.cluster.clone(),
            encoding: self.get_encoding(),
            sample_rate: self.sample_rate.and_then(|sr| match sr {
                8000 => Some(SampleRate::Rate8000),
                16000 => Some(SampleRate::Rate16000),
                22050 => Some(SampleRate::Rate22050),
                24000 => Some(SampleRate::Rate24000),
                32000 => Some(SampleRate::Rate32000),
                44100 => Some(SampleRate::Rate44100),
                _ => None,
            }),
            speed_ratio: self.speed_ratio,
            volume_ratio: self.volume_ratio,
            pitch_ratio: self.pitch_ratio,
            emotion: self.emotion.clone(),
            language: self.language.as_ref().and_then(|l| match l.as_str() {
                "zh-CN" => Some(Language::ZhCn),
                "en-US" => Some(Language::EnUs),
                "ja-JP" => Some(Language::JaJp),
                _ => None,
            }),
            ..Default::default()
        }
    }
}

impl TtsCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            TtsSubcommand::V1(v1) => v1.run(cli).await,
            TtsSubcommand::V2(v2) => v2.run(cli).await,
        }
    }
}

impl TtsV1Command {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            TtsV1Subcommand::Synthesize { text, voice, encoding, cluster } => {
                synthesize_v1(cli, text.as_deref(), voice.as_deref(), encoding.as_deref(), cluster.as_deref()).await
            }
            TtsV1Subcommand::Stream { text, voice, encoding, cluster } => {
                stream_v1(cli, text.as_deref(), voice.as_deref(), encoding.as_deref(), cluster.as_deref()).await
            }
        }
    }
}

impl TtsV2Command {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            TtsV2Subcommand::Stream { text, voice, encoding } => {
                stream_v2(cli, text.as_deref(), voice.as_deref(), encoding.as_deref()).await
            }
            TtsV2Subcommand::Ws { text, voice } => {
                ws_v2(cli, text.as_deref(), voice.as_deref()).await
            }
            TtsV2Subcommand::Bidirectional { voice } => {
                bidirectional_v2(cli, voice.as_deref()).await
            }
            TtsV2Subcommand::Async { text, voice, callback_url } => {
                async_v2(cli, text.as_deref(), voice.as_deref(), callback_url.as_deref()).await
            }
            TtsV2Subcommand::Status { task_id } => {
                status_v2(cli, task_id).await
            }
        }
    }
}

// ============================================================================
// V1 Implementation Functions
// ============================================================================

async fn synthesize_v1(
    cli: &Cli,
    text: Option<&str>,
    voice: Option<&str>,
    encoding: Option<&str>,
    cluster: Option<&str>,
) -> anyhow::Result<()> {
    let ctx = get_context(cli)?;
    let client = create_client(&ctx)?;

    // Build request from file or command line args
    let req = if let Some(input_file) = cli.input.as_deref() {
        let mut file_req: TtsFileRequest = load_request(input_file)?;
        
        // Override with command line args
        if let Some(t) = text {
            file_req.text = t.to_string();
        }
        if let Some(v) = voice {
            file_req.voice_type = v.to_string();
        }
        if let Some(e) = encoding {
            file_req.encoding = Some(e.to_string());
        }
        if let Some(c) = cluster {
            file_req.cluster = Some(c.to_string());
        }
        
        // Apply default voice from context if not specified
        if file_req.get_voice_type().is_empty() {
            if let Some(default_voice) = ctx.get_extra("default_voice") {
                file_req.voice_type = default_voice.to_string();
            }
        }
        
        file_req.to_tts_request()
    } else {
        // Build from command line args only
        let text = text.ok_or_else(|| anyhow::anyhow!("text is required, use -t flag or -f file"))?;
        let voice_type = voice
            .map(|v| v.to_string())
            .or_else(|| ctx.get_extra("default_voice").map(|v| v.to_string()))
            .ok_or_else(|| anyhow::anyhow!("voice type is required, use -V flag"))?;

        TtsRequest {
            text: text.to_string(),
            voice_type,
            cluster: cluster.map(|c| c.to_string()),
            encoding: encoding.and_then(|e| match e {
                "pcm" => Some(AudioEncoding::Pcm),
                "wav" => Some(AudioEncoding::Wav),
                "mp3" => Some(AudioEncoding::Mp3),
                "ogg_opus" => Some(AudioEncoding::OggOpus),
                _ => None,
            }),
            ..Default::default()
        }
    };

    print_verbose(cli, &format!("Using V1 API (classic)"));
    print_verbose(cli, &format!("Voice: {}", req.voice_type));
    print_verbose(cli, &format!("Text length: {} characters", req.text.len()));

    let resp = client.tts().synthesize(&req).await?;

    // Output audio to file if specified
    let output_path = cli.output.as_deref();
    if let Some(path) = output_path {
        if !resp.audio.is_empty() {
            output_bytes(&resp.audio, path)?;
            print_success(&format!(
                "Audio saved to: {} ({})",
                path,
                format_bytes(resp.audio.len())
            ));
        }
    }

    // Output result
    let result = serde_json::json!({
        "api_version": "v1",
        "audio_size": resp.audio.len(),
        "duration_ms": resp.duration,
        "req_id": resp.req_id,
        "output_file": output_path,
    });

    output_result(&result, None, cli.json)
}

async fn stream_v1(
    cli: &Cli,
    text: Option<&str>,
    voice: Option<&str>,
    encoding: Option<&str>,
    cluster: Option<&str>,
) -> anyhow::Result<()> {
    let output_path = cli.output.as_deref().ok_or_else(|| {
        anyhow::anyhow!("output file is required for streaming audio, use -o flag")
    })?;

    let ctx = get_context(cli)?;
    let client = create_client(&ctx)?;

    // Build request from file or command line args
    let req = if let Some(input_file) = cli.input.as_deref() {
        let mut file_req: TtsFileRequest = load_request(input_file)?;
        
        // Override with command line args
        if let Some(t) = text {
            file_req.text = t.to_string();
        }
        if let Some(v) = voice {
            file_req.voice_type = v.to_string();
        }
        if let Some(e) = encoding {
            file_req.encoding = Some(e.to_string());
        }
        if let Some(c) = cluster {
            file_req.cluster = Some(c.to_string());
        }
        
        // Apply default voice from context if not specified
        if file_req.get_voice_type().is_empty() {
            if let Some(default_voice) = ctx.get_extra("default_voice") {
                file_req.voice_type = default_voice.to_string();
            }
        }
        
        file_req.to_tts_request()
    } else {
        // Build from command line args only
        let text = text.ok_or_else(|| anyhow::anyhow!("text is required, use -t flag or -f file"))?;
        let voice_type = voice
            .map(|v| v.to_string())
            .or_else(|| ctx.get_extra("default_voice").map(|v| v.to_string()))
            .ok_or_else(|| anyhow::anyhow!("voice type is required, use -V flag"))?;

        TtsRequest {
            text: text.to_string(),
            voice_type,
            cluster: cluster.map(|c| c.to_string()),
            encoding: encoding.and_then(|e| match e {
                "pcm" => Some(AudioEncoding::Pcm),
                "wav" => Some(AudioEncoding::Wav),
                "mp3" => Some(AudioEncoding::Mp3),
                "ogg_opus" => Some(AudioEncoding::OggOpus),
                _ => None,
            }),
            ..Default::default()
        }
    };

    print_verbose(cli, &format!("Using V1 API (classic streaming)"));
    print_verbose(cli, &format!("Streaming to: {}", output_path));

    let mut audio_buf = Vec::new();
    let mut last_duration = 0;

    let tts = client.tts();
    let stream = tts.synthesize_stream(&req).await?;
    let mut stream = pin!(stream);

    while let Some(chunk) = stream.next().await {
        let chunk = chunk?;
        if !chunk.audio.is_empty() {
            audio_buf.extend_from_slice(&chunk.audio);
        }
        if chunk.duration > 0 {
            last_duration = chunk.duration;
        }
    }

    // Write audio to file
    output_bytes(&audio_buf, output_path)?;
    print_success(&format!(
        "Audio saved to: {} ({})",
        output_path,
        format_bytes(audio_buf.len())
    ));

    // Output final info
    let result = serde_json::json!({
        "api_version": "v1",
        "audio_size": audio_buf.len(),
        "duration_ms": last_duration,
        "output_file": output_path,
    });

    output_result(&result, None, cli.json)
}

// ============================================================================
// V2 Implementation Functions
// ============================================================================

async fn stream_v2(
    cli: &Cli,
    text: Option<&str>,
    voice: Option<&str>,
    encoding: Option<&str>,
) -> anyhow::Result<()> {
    let output_path = cli.output.as_deref().ok_or_else(|| {
        anyhow::anyhow!("output file is required for streaming audio, use -o flag")
    })?;

    let ctx = get_context(cli)?;
    let client = create_client(&ctx)?;

    // Build V2 request from file or command line args
    let req = if let Some(input_file) = cli.input.as_deref() {
        let mut file_req: TtsFileRequest = load_request(input_file)?;
        
        // Override with command line args
        if let Some(t) = text {
            file_req.text = t.to_string();
        }
        if let Some(v) = voice {
            file_req.speaker = Some(v.to_string());
        }
        if let Some(e) = encoding {
            file_req.format = Some(e.to_string());
        }
        
        // Apply default voice from context if not specified
        if file_req.get_voice_type().is_empty() {
            if let Some(default_voice) = ctx.get_extra("default_voice") {
                file_req.speaker = Some(default_voice.to_string());
            }
        }
        
        // Convert to V2 request
        TtsV2Request {
            text: file_req.text.clone(),
            speaker: file_req.get_voice_type(),
            resource_id: file_req.resource_id.clone(),
            format: file_req.format.clone(),
            sample_rate: file_req.sample_rate,
            speed_ratio: file_req.speed_ratio,
            volume_ratio: file_req.volume_ratio,
            pitch_ratio: file_req.pitch_ratio,
            emotion: file_req.emotion.clone(),
            language: file_req.language.clone(),
            ..Default::default()
        }
    } else {
        // Build from command line args only
        let text = text.ok_or_else(|| anyhow::anyhow!("text is required, use -t flag or -f file"))?;
        let speaker = voice
            .map(|v| v.to_string())
            .or_else(|| ctx.get_extra("default_voice").map(|v| v.to_string()))
            .ok_or_else(|| anyhow::anyhow!("speaker voice is required, use -V flag"))?;

        TtsV2Request {
            text: text.to_string(),
            speaker,
            format: encoding.map(|e| e.to_string()),
            ..Default::default()
        }
    };

    print_verbose(cli, &format!("Using V2 API (BigModel HTTP streaming)"));
    print_verbose(cli, &format!("Speaker: {}", req.speaker));
    print_verbose(cli, &format!("Resource ID: {}", req.resource_id.as_deref().unwrap_or("seed-tts-2.0 (default)")));
    print_verbose(cli, &format!("Streaming to: {}", output_path));

    let mut audio_buf = Vec::new();
    let mut chunk_count = 0;

    // Use the new V2 TTS service
    let tts_v2 = client.tts_v2();
    let stream = tts_v2.stream(&req).await?;
    let mut stream = pin!(stream);

    while let Some(chunk) = stream.next().await {
        let chunk = chunk?;
        if !chunk.audio.is_empty() {
            audio_buf.extend_from_slice(&chunk.audio);
            chunk_count += 1;
        }
    }

    // Write audio to file
    output_bytes(&audio_buf, output_path)?;
    print_success(&format!(
        "Audio saved to: {} ({}, {} chunks)",
        output_path,
        format_bytes(audio_buf.len()),
        chunk_count
    ));

    // Output final info
    let result = serde_json::json!({
        "api_version": "v2",
        "audio_size": audio_buf.len(),
        "chunk_count": chunk_count,
        "output_file": output_path,
    });

    output_result(&result, None, cli.json)
}

async fn ws_v2(
    cli: &Cli,
    _text: Option<&str>,
    _voice: Option<&str>,
) -> anyhow::Result<()> {
    let ctx = get_context(cli)?;
    print_verbose(cli, &format!("Using context: {}", ctx.name));
    
    // TODO: Implement V2 WebSocket unidirectional streaming
    eprintln!("[V2 WebSocket unidirectional streaming not implemented yet]");
    eprintln!("Would stream speech via WebSocket");
    
    let result = serde_json::json!({
        "_note": "V2 WebSocket unidirectional streaming not implemented yet",
    });
    output_result(&result, None, cli.json)
}

async fn bidirectional_v2(
    cli: &Cli,
    voice: Option<&str>,
) -> anyhow::Result<()> {
    let output_path = cli.output.as_deref().ok_or_else(|| {
        anyhow::anyhow!("output file is required for streaming audio, use -o flag")
    })?;

    let ctx = get_context(cli)?;
    let client = create_client(&ctx)?;
    print_verbose(cli, &format!("Using V2 API (BigModel WebSocket bidirectional)"));

    // Build V2 request from file or command line args
    let (req, texts) = if let Some(input_file) = cli.input.as_deref() {
        let file_req: TtsFileRequest = load_request(input_file)?;
        
        // Get voice from file or command line
        let speaker = voice.map(|v| v.to_string())
            .or(file_req.speaker.clone())
            .or_else(|| ctx.get_extra("default_voice").map(|v| v.to_string()))
            .ok_or_else(|| anyhow::anyhow!("speaker voice is required, use -V flag"))?;
        
        let req = TtsV2Request {
            text: String::new(), // Will send text via SendText
            speaker,
            resource_id: file_req.resource_id.clone(),
            format: file_req.format.clone(),
            sample_rate: file_req.sample_rate,
            speed_ratio: file_req.speed_ratio,
            volume_ratio: file_req.volume_ratio,
            pitch_ratio: file_req.pitch_ratio,
            emotion: file_req.emotion.clone(),
            language: file_req.language.clone(),
            ..Default::default()
        };
        
        // Split text into sentences for bidirectional demo
        let text = file_req.text.clone();
        let texts: Vec<String> = if text.contains('。') || text.contains('！') || text.contains('？') {
            text.split(|c| c == '。' || c == '！' || c == '？')
                .filter(|s| !s.trim().is_empty())
                .map(|s| s.to_string())
                .collect()
        } else {
            vec![text]
        };
        
        (req, texts)
    } else {
        return Err(anyhow::anyhow!("input file is required for bidirectional mode, use -f flag"));
    };

    print_verbose(cli, &format!("Speaker: {}", req.speaker));
    print_verbose(cli, &format!("Resource ID: {}", req.resource_id.as_deref().unwrap_or("seed-tts-2.0 (default)")));
    print_verbose(cli, &format!("Text segments: {}", texts.len()));
    print_verbose(cli, &format!("Streaming to: {}", output_path));

    // Open bidirectional session
    let mut session = client.tts_v2().bidirectional(&req).await?;
    print_verbose(cli, "Session established");

    // Send all text segments
    for (i, text) in texts.iter().enumerate() {
        let is_last = i == texts.len() - 1;
        print_verbose(cli, &format!("Sending segment {}/{}: {}...", i + 1, texts.len(), &text[..text.len().min(30)]));
        session.send_text(text, is_last).await?;
    }

    // Receive audio
    let mut audio_buf = Vec::new();
    let mut chunk_count = 0;

    while let Some(result) = session.recv().await {
        let chunk = result?;
        if !chunk.audio.is_empty() {
            audio_buf.extend_from_slice(&chunk.audio);
            chunk_count += 1;
            print_verbose(cli, &format!("Received chunk {}: {} bytes", chunk_count, chunk.audio.len()));
        }
        if chunk.is_last {
            break;
        }
    }

    session.close().await?;

    // Write audio to file
    if !audio_buf.is_empty() {
        output_bytes(&audio_buf, output_path)?;
        print_success(&format!(
            "Audio saved to: {} ({}, {} chunks)",
            output_path,
            format_bytes(audio_buf.len()),
            chunk_count
        ));
    } else {
        return Err(anyhow::anyhow!("No audio data received"));
    }

    let result = serde_json::json!({
        "api_version": "v2_bidirectional",
        "audio_size": audio_buf.len(),
        "chunk_count": chunk_count,
        "text_segments": texts.len(),
        "output_file": output_path,
    });

    output_result(&result, None, cli.json)
}

async fn async_v2(
    cli: &Cli,
    _text: Option<&str>,
    _voice: Option<&str>,
    _callback_url: Option<&str>,
) -> anyhow::Result<()> {
    let ctx = get_context(cli)?;
    print_verbose(cli, &format!("Using context: {}", ctx.name));
    
    // TODO: Implement V2 async TTS task creation
    eprintln!("[V2 async TTS not implemented yet]");
    
    let result = serde_json::json!({
        "_note": "V2 async TTS not implemented yet",
        "task_id": "placeholder-task-id",
    });
    output_result(&result, None, cli.json)
}

async fn status_v2(
    cli: &Cli,
    task_id: &str,
) -> anyhow::Result<()> {
    let ctx = get_context(cli)?;
    print_verbose(cli, &format!("Using context: {}", ctx.name));
    print_verbose(cli, &format!("Querying task: {}", task_id));
    
    // TODO: Implement task status query
    let result = serde_json::json!({
        "_note": "Task status query not implemented yet",
        "task_id": task_id,
        "status": "pending",
    });
    output_result(&result, None, cli.json)
}
