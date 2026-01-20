//! Video generation service.

use std::sync::Arc;

use serde::{Deserialize, Serialize};

use super::{
    error::Result,
    http::HttpClient,
    task::{Task, TaskType},
    types::BaseResp,
};

/// Video generation service.
pub struct VideoService {
    http: Arc<HttpClient>,
}

impl VideoService {
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Creates a text-to-video generation task.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let request = TextToVideoRequest {
    ///     model: "MiniMax-Hailuo-2.3".to_string(),
    ///     prompt: "A cat playing piano".to_string(),
    ///     ..Default::default()
    /// };
    ///
    /// let task = client.video().create_text_to_video(&request).await?;
    /// let result = task.wait().await?;
    /// ```
    pub async fn create_text_to_video(&self, request: &TextToVideoRequest) -> Result<Task> {
        #[derive(Deserialize)]
        struct Response {
            task_id: String,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        let resp: Response = self
            .http
            .request("POST", "/v1/video_generation", Some(request))
            .await?;

        Ok(Task::new(resp.task_id, TaskType::Video, self.http.clone()))
    }

    /// Creates an image-to-video generation task.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let request = ImageToVideoRequest {
    ///     model: "I2V-01".to_string(),
    ///     first_frame_image: "https://example.com/image.jpg".to_string(),
    ///     prompt: Some("The person starts walking".to_string()),
    ///     ..Default::default()
    /// };
    ///
    /// let task = client.video().create_image_to_video(&request).await?;
    /// let result = task.wait().await?;
    /// ```
    pub async fn create_image_to_video(&self, request: &ImageToVideoRequest) -> Result<Task> {
        #[derive(Deserialize)]
        struct Response {
            task_id: String,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        let resp: Response = self
            .http
            .request("POST", "/v1/video_generation", Some(request))
            .await?;

        Ok(Task::new(resp.task_id, TaskType::Video, self.http.clone()))
    }

    /// Creates a first-and-last-frame video generation task.
    ///
    /// Generate video with both first and last frame specified.
    pub async fn create_frame_to_video(&self, request: &FrameToVideoRequest) -> Result<Task> {
        #[derive(Deserialize)]
        struct Response {
            task_id: String,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        let resp: Response = self
            .http
            .request("POST", "/v1/video_generation", Some(request))
            .await?;

        Ok(Task::new(resp.task_id, TaskType::Video, self.http.clone()))
    }

    /// Creates a subject reference video generation task.
    ///
    /// Generate video with a subject reference image.
    pub async fn create_subject_ref_video(&self, request: &SubjectRefVideoRequest) -> Result<Task> {
        #[derive(Deserialize)]
        struct Response {
            task_id: String,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        let resp: Response = self
            .http
            .request("POST", "/v1/video_generation", Some(request))
            .await?;

        Ok(Task::new(resp.task_id, TaskType::Video, self.http.clone()))
    }

    /// Creates a video agent task.
    pub async fn create_agent_task(&self, request: &VideoAgentRequest) -> Result<Task> {
        #[derive(Deserialize)]
        struct Response {
            task_id: String,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        let resp: Response = self
            .http
            .request("POST", "/v1/video_agent/generate", Some(request))
            .await?;

        Ok(Task::new(resp.task_id, TaskType::VideoAgent, self.http.clone()))
    }
}

// ==================== Request/Response Types ====================

/// Request for text-to-video generation.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TextToVideoRequest {
    /// Model name.
    pub model: String,

    /// Video description.
    pub prompt: String,

    /// Video duration in seconds (6 or 10).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub duration: Option<i32>,

    /// Resolution: 768P or 1080P.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub resolution: Option<String>,
}

/// Request for image-to-video generation.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ImageToVideoRequest {
    /// Model name (I2V series).
    pub model: String,

    /// First frame image URL or base64.
    pub first_frame_image: String,

    /// Video description.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub prompt: Option<String>,

    /// Video duration in seconds.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub duration: Option<i32>,

    /// Resolution.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub resolution: Option<String>,
}

/// Request for first-and-last-frame video generation.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct FrameToVideoRequest {
    /// Model name.
    pub model: String,

    /// First frame image.
    pub first_frame_image: String,

    /// Last frame image.
    pub last_frame_image: String,

    /// Video description.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub prompt: Option<String>,
}

/// Request for subject reference video generation.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SubjectRefVideoRequest {
    /// Model name.
    pub model: String,

    /// Video description.
    pub prompt: String,

    /// Subject reference image.
    pub subject_reference: String,
}

/// Request for video agent task.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct VideoAgentRequest {
    /// Template ID.
    pub template_id: String,

    /// Media inputs.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub media_inputs: Option<Vec<MediaInput>>,

    /// Text inputs.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub text_inputs: Option<Vec<TextInput>>,
}

/// Media input for video agent.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct MediaInput {
    /// Media type: image or video.
    #[serde(rename = "type")]
    pub media_type: String,

    /// Media file URL.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub url: Option<String>,

    /// Media file ID.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub file_id: Option<String>,
}

/// Text input for video agent.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TextInput {
    /// Input key name.
    pub key: String,

    /// Input value.
    pub value: String,
}

/// Result of a video generation task.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct VideoResult {
    /// Generated video file ID.
    pub file_id: String,

    /// Width of the generated video.
    #[serde(default)]
    pub video_width: i32,

    /// Height of the generated video.
    #[serde(default)]
    pub video_height: i32,

    /// Video download URL (for agent tasks).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub download_url: Option<String>,
}
