//! Video generation commands.
//!
//! Compatible with Go version's video commands.

use clap::{Args, Subcommand};

use giztoy_minimax::{
    FrameToVideoRequest, ImageToVideoRequest, SubjectRefVideoRequest, TextToVideoRequest,
    VideoAgentRequest, MODEL_HAILUO_23,
};

use super::{
    create_client, get_context, load_request, output_result, print_success, print_verbose,
    require_input_file,
};
use crate::Cli;

/// Video generation service.
///
/// Supports text-to-video, image-to-video, and other video generation modes.
#[derive(Args)]
pub struct VideoCommand {
    #[command(subcommand)]
    command: VideoSubcommand,
}

#[derive(Subcommand)]
enum VideoSubcommand {
    /// Create text-to-video task
    #[command(name = "t2v")]
    TextToVideo,
    /// Create image-to-video task
    #[command(name = "i2v")]
    ImageToVideo,
    /// Create frame-to-video task
    #[command(name = "f2v")]
    FrameToVideo,
    /// Create subject reference video task
    #[command(name = "s2v")]
    SubjectRefVideo,
    /// Create video agent task
    Agent,
}

impl VideoCommand {
    pub async fn run(&self, cli: &Cli) -> anyhow::Result<()> {
        match &self.command {
            VideoSubcommand::TextToVideo => self.text_to_video(cli).await,
            VideoSubcommand::ImageToVideo => self.image_to_video(cli).await,
            VideoSubcommand::FrameToVideo => self.frame_to_video(cli).await,
            VideoSubcommand::SubjectRefVideo => self.subject_ref_video(cli).await,
            VideoSubcommand::Agent => self.video_agent(cli).await,
        }
    }

    async fn text_to_video(&self, cli: &Cli) -> anyhow::Result<()> {
        let input_file = require_input_file(cli)?;
        let ctx = get_context(cli)?;

        let mut req: TextToVideoRequest = load_request(input_file)?;

        // Use defaults if not specified
        if req.model.is_empty() {
            req.model = MODEL_HAILUO_23.to_string();
        }

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Model: {}", req.model));

        let client = create_client(&ctx)?;
        let task = client.video().create_text_to_video(&req).await?;

        print_success(&format!("Video task created: {}", task.id()));

        let result = serde_json::json!({
            "task_id": task.id(),
            "status": "created",
        });

        output_result(&result, cli.output.as_deref(), cli.json)
    }

    async fn image_to_video(&self, cli: &Cli) -> anyhow::Result<()> {
        let input_file = require_input_file(cli)?;
        let ctx = get_context(cli)?;

        let mut req: ImageToVideoRequest = load_request(input_file)?;

        // Use defaults if not specified
        if req.model.is_empty() {
            req.model = MODEL_HAILUO_23.to_string();
        }

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Model: {}", req.model));

        let client = create_client(&ctx)?;
        let task = client.video().create_image_to_video(&req).await?;

        print_success(&format!("Video task created: {}", task.id()));

        let result = serde_json::json!({
            "task_id": task.id(),
            "status": "created",
        });

        output_result(&result, cli.output.as_deref(), cli.json)
    }

    async fn frame_to_video(&self, cli: &Cli) -> anyhow::Result<()> {
        let input_file = require_input_file(cli)?;
        let ctx = get_context(cli)?;

        let req: FrameToVideoRequest = load_request(input_file)?;

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Model: {}", req.model));

        let client = create_client(&ctx)?;
        let task = client.video().create_frame_to_video(&req).await?;

        print_success(&format!("Video task created: {}", task.id()));

        let result = serde_json::json!({
            "task_id": task.id(),
            "status": "created",
        });

        output_result(&result, cli.output.as_deref(), cli.json)
    }

    async fn subject_ref_video(&self, cli: &Cli) -> anyhow::Result<()> {
        let input_file = require_input_file(cli)?;
        let ctx = get_context(cli)?;

        let req: SubjectRefVideoRequest = load_request(input_file)?;

        print_verbose(cli, &format!("Using context: {}", ctx.name));
        print_verbose(cli, &format!("Model: {}", req.model));

        let client = create_client(&ctx)?;
        let task = client.video().create_subject_ref_video(&req).await?;

        print_success(&format!("Video task created: {}", task.id()));

        let result = serde_json::json!({
            "task_id": task.id(),
            "status": "created",
        });

        output_result(&result, cli.output.as_deref(), cli.json)
    }

    async fn video_agent(&self, cli: &Cli) -> anyhow::Result<()> {
        let input_file = require_input_file(cli)?;
        let ctx = get_context(cli)?;

        let req: VideoAgentRequest = load_request(input_file)?;

        print_verbose(cli, &format!("Using context: {}", ctx.name));

        let client = create_client(&ctx)?;
        let task = client.video().create_agent_task(&req).await?;

        print_success(&format!("Video agent task created: {}", task.id()));

        let result = serde_json::json!({
            "task_id": task.id(),
            "status": "created",
        });

        output_result(&result, cli.output.as_deref(), cli.json)
    }
}
