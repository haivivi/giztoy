//! Async task management.

use std::{sync::Arc, time::Duration};

use serde::{Deserialize, Serialize};

use super::{
    error::{Error, Result},
    http::HttpClient,
    speech::SpeechAsyncResult,
    types::{BaseResp, TaskStatus},
    video::VideoResult,
};

/// Task type.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TaskType {
    SpeechAsync,
    Video,
    VideoAgent,
}

/// An async task that can be polled for completion.
pub struct Task {
    id: String,
    task_type: TaskType,
    http: Arc<HttpClient>,
}

impl Task {
    /// Creates a new task.
    pub fn new(id: String, task_type: TaskType, http: Arc<HttpClient>) -> Self {
        Self { id, task_type, http }
    }

    /// Returns the task ID.
    pub fn id(&self) -> &str {
        &self.id
    }

    /// Returns the task type.
    pub fn task_type(&self) -> TaskType {
        self.task_type
    }

    /// Queries the current status of the task.
    pub async fn status(&self) -> Result<TaskStatusResponse> {
        match self.task_type {
            TaskType::SpeechAsync => self.query_speech_status().await,
            TaskType::Video => self.query_video_status().await,
            TaskType::VideoAgent => self.query_video_agent_status().await,
        }
    }

    /// Waits for the task to complete and returns the result.
    ///
    /// This method polls the task status every 2 seconds until it completes.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let task = client.speech().create_async_task(&request).await?;
    /// let result = task.wait().await?;
    ///
    /// match result {
    ///     TaskResult::Speech(speech) => {
    ///         println!("File ID: {}", speech.file_id);
    ///     }
    ///     _ => {}
    /// }
    /// ```
    pub async fn wait(&self) -> Result<TaskResult> {
        self.wait_with_interval(Duration::from_secs(2)).await
    }

    /// Waits for the task to complete with a custom polling interval.
    pub async fn wait_with_interval(&self, interval: Duration) -> Result<TaskResult> {
        loop {
            let status = self.status().await?;

            match status.status {
                TaskStatus::Success => {
                    return Ok(status
                        .result
                        .ok_or_else(|| Error::TaskFailed("no result returned".to_string()))?);
                }
                TaskStatus::Failed => {
                    return Err(Error::TaskFailed(
                        status.error_message.unwrap_or_else(|| "unknown error".to_string()),
                    ));
                }
                _ => {
                    tokio::time::sleep(interval).await;
                }
            }
        }
    }

    async fn query_speech_status(&self) -> Result<TaskStatusResponse> {
        #[derive(Serialize)]
        struct Request<'a> {
            task_id: &'a str,
        }

        #[derive(Deserialize)]
        struct Response {
            status: TaskStatus,
            #[serde(default)]
            file_id: String,
            extra_info: Option<super::types::AudioInfo>,
            subtitle: Option<super::types::Subtitle>,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        let resp: Response = self
            .http
            .request(
                "POST",
                "/v1/t2a_async/fetch",
                Some(&Request { task_id: &self.id }),
            )
            .await?;

        let result = if resp.status == TaskStatus::Success {
            Some(TaskResult::Speech(SpeechAsyncResult {
                file_id: resp.file_id,
                audio_info: resp.extra_info,
                subtitle: resp.subtitle,
            }))
        } else {
            None
        };

        Ok(TaskStatusResponse {
            task_id: self.id.clone(),
            status: resp.status,
            result,
            error_message: None,
        })
    }

    async fn query_video_status(&self) -> Result<TaskStatusResponse> {
        let path = format!("/v1/query/video_generation?task_id={}", self.id);

        #[derive(Deserialize)]
        struct Response {
            status: TaskStatus,
            #[serde(default)]
            file_id: String,
            #[serde(default)]
            video_width: i32,
            #[serde(default)]
            video_height: i32,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        let resp: Response = self.http.request::<(), _>("GET", &path, None).await?;

        let result = if resp.status == TaskStatus::Success {
            Some(TaskResult::Video(VideoResult {
                file_id: resp.file_id,
                video_width: resp.video_width,
                video_height: resp.video_height,
                download_url: None,
            }))
        } else {
            None
        };

        Ok(TaskStatusResponse {
            task_id: self.id.clone(),
            status: resp.status,
            result,
            error_message: None,
        })
    }

    async fn query_video_agent_status(&self) -> Result<TaskStatusResponse> {
        let path = format!("/v1/query/video_agent?task_id={}", self.id);

        #[derive(Deserialize)]
        struct Response {
            status: TaskStatus,
            #[serde(default)]
            file_id: String,
            #[serde(default)]
            download_url: String,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        let resp: Response = self.http.request::<(), _>("GET", &path, None).await?;

        let result = if resp.status == TaskStatus::Success {
            Some(TaskResult::Video(VideoResult {
                file_id: resp.file_id,
                video_width: 0,
                video_height: 0,
                download_url: Some(resp.download_url),
            }))
        } else {
            None
        };

        Ok(TaskStatusResponse {
            task_id: self.id.clone(),
            status: resp.status,
            result,
            error_message: None,
        })
    }
}

/// Response from querying task status.
#[derive(Debug, Clone)]
pub struct TaskStatusResponse {
    /// Task ID.
    pub task_id: String,

    /// Current status.
    pub status: TaskStatus,

    /// Result (if completed successfully).
    pub result: Option<TaskResult>,

    /// Error message (if failed).
    pub error_message: Option<String>,
}

/// Result of a completed task.
#[derive(Debug, Clone)]
pub enum TaskResult {
    Speech(SpeechAsyncResult),
    Video(VideoResult),
}
