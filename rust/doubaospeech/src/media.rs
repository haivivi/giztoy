//! Media processing service for Doubao Speech API.

use std::sync::Arc;

use serde::{Deserialize, Serialize};

use crate::{
    error::{Error, Result},
    http::HttpClient,
    types::{Language, SubtitleFormat, TaskStatus},
};

/// Media processing service (subtitle extraction).
///
/// API Documentation: https://www.volcengine.com/docs/6561/1305191
pub struct MediaService {
    http: Arc<HttpClient>,
}

impl MediaService {
    /// Creates a new Media service.
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Extracts subtitles from media.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_doubaospeech::{Client, SubtitleRequest};
    ///
    /// let client = Client::builder("app-id").api_key("api-key").build()?;
    /// let result = client.media().extract_subtitle(&SubtitleRequest {
    ///     media_url: "https://example.com/video.mp4".to_string(),
    ///     ..Default::default()
    /// }).await?;
    /// ```
    pub async fn extract_subtitle(&self, req: &SubtitleRequest) -> Result<SubtitleTaskResult> {
        let auth = self.http.auth();
        let req_id = crate::client::generate_req_id();

        let mut submit_req = serde_json::json!({
            "appid": auth.app_id,
            "reqid": req_id,
            "media_url": req.media_url,
        });

        if let Some(ref language) = req.language {
            submit_req["language"] = serde_json::json!(language.as_str());
        }
        if let Some(ref format) = req.format {
            let format_str = match format {
                SubtitleFormat::Srt => "srt",
                SubtitleFormat::Vtt => "vtt",
                SubtitleFormat::Json => "json",
            };
            submit_req["output_format"] = serde_json::json!(format_str);
        }
        if req.enable_translation {
            submit_req["enable_translation"] = serde_json::json!(true);
            if let Some(ref target) = req.target_language {
                submit_req["target_language"] = serde_json::json!(target.as_str());
            }
        }
        if let Some(ref callback) = req.callback_url {
            submit_req["callback_url"] = serde_json::json!(callback);
        }

        let response: AsyncTaskResponse = self
            .http
            .request("POST", "/api/v1/subtitle/submit", Some(&submit_req))
            .await?;

        if response.code != 0 {
            return Err(Error::api(response.code, response.message, 200));
        }

        Ok(SubtitleTaskResult {
            task_id: response.task_id,
            req_id,
        })
    }

    /// Queries subtitle task status.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let status = client.media().get_subtitle_task("task-id").await?;
    /// println!("Status: {:?}", status.status);
    /// ```
    pub async fn get_subtitle_task(&self, task_id: &str) -> Result<SubtitleTaskStatus> {
        let auth = self.http.auth();

        let query_req = serde_json::json!({
            "appid": auth.app_id,
            "task_id": task_id,
        });

        let response: SubtitleQueryResponse = self
            .http
            .request("POST", "/api/v1/subtitle/query", Some(&query_req))
            .await?;

        if response.code != 0 {
            return Err(Error::api(response.code, response.message, 200));
        }

        // Convert status
        let status = match response.data.status.as_str() {
            "submitted" | "pending" => TaskStatus::Pending,
            "running" | "processing" => TaskStatus::Processing,
            "success" => TaskStatus::Success,
            "failed" => TaskStatus::Failed,
            _ => TaskStatus::Pending,
        };

        // Convert result
        let result = if status == TaskStatus::Success {
            Some(SubtitleResult {
                subtitle_url: response.data.subtitle_url,
                subtitle_content: response.data.subtitle_content,
                duration: response.data.duration,
            })
        } else {
            None
        };

        Ok(SubtitleTaskStatus {
            task_id: response.data.task_id,
            status,
            progress: response.data.progress,
            result,
        })
    }
}

// ================== Request Types ==================

/// Subtitle extraction request.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SubtitleRequest {
    /// Media URL.
    pub media_url: String,
    /// Language.
    #[serde(default)]
    pub language: Option<Language>,
    /// Output format.
    #[serde(default)]
    pub format: Option<SubtitleFormat>,
    /// Enable translation.
    #[serde(default)]
    pub enable_translation: bool,
    /// Target language for translation.
    #[serde(default)]
    pub target_language: Option<Language>,
    /// Callback URL.
    #[serde(default)]
    pub callback_url: Option<String>,
}

// ================== Response Types ==================

/// Subtitle task creation result.
#[derive(Debug, Clone, Default, Serialize)]
pub struct SubtitleTaskResult {
    /// Task ID.
    pub task_id: String,
    /// Request ID.
    pub req_id: String,
}

/// Subtitle task status.
#[derive(Debug, Clone, Default, Serialize)]
pub struct SubtitleTaskStatus {
    /// Task ID.
    pub task_id: String,
    /// Status.
    pub status: TaskStatus,
    /// Progress (0-100).
    pub progress: i32,
    /// Result (when completed).
    pub result: Option<SubtitleResult>,
}

/// Subtitle extraction result.
#[derive(Debug, Clone, Default, Serialize)]
pub struct SubtitleResult {
    /// Subtitle URL.
    pub subtitle_url: String,
    /// Subtitle content (if requested inline).
    pub subtitle_content: Option<String>,
    /// Media duration in milliseconds.
    pub duration: i32,
}

// ================== Internal Types ==================

#[derive(Debug, Deserialize)]
struct AsyncTaskResponse {
    #[serde(default)]
    code: i32,
    #[serde(default)]
    message: String,
    #[serde(default)]
    task_id: String,
}

#[derive(Debug, Deserialize)]
struct SubtitleQueryResponse {
    #[serde(default)]
    code: i32,
    #[serde(default)]
    message: String,
    #[serde(default)]
    data: SubtitleQueryData,
}

#[derive(Debug, Deserialize, Default)]
struct SubtitleQueryData {
    #[serde(default)]
    task_id: String,
    #[serde(default)]
    status: String,
    #[serde(default)]
    progress: i32,
    #[serde(default)]
    subtitle_url: String,
    #[serde(default)]
    subtitle_content: Option<String>,
    #[serde(default)]
    duration: i32,
}
