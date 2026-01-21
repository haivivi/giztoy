//! Podcast synthesis service for Doubao Speech API.

use std::sync::Arc;

use serde::{Deserialize, Serialize};

use crate::{
    error::{Error, Result},
    http::HttpClient,
    types::{AudioEncoding, SampleRate, TaskStatus},
};

/// Podcast synthesis service.
///
/// API Documentation: https://www.volcengine.com/docs/6561/1668014
pub struct PodcastService {
    http: Arc<HttpClient>,
}

impl PodcastService {
    /// Creates a new Podcast service.
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Creates a podcast synthesis task.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_doubaospeech::{Client, PodcastTaskRequest, PodcastLine};
    ///
    /// let client = Client::builder("app-id").api_key("api-key").build()?;
    /// let result = client.podcast().create_task(&PodcastTaskRequest {
    ///     script: vec![
    ///         PodcastLine {
    ///             speaker_id: "speaker_1".to_string(),
    ///             text: "Hello!".to_string(),
    ///             ..Default::default()
    ///         },
    ///         PodcastLine {
    ///             speaker_id: "speaker_2".to_string(),
    ///             text: "Hi there!".to_string(),
    ///             ..Default::default()
    ///         },
    ///     ],
    ///     ..Default::default()
    /// }).await?;
    /// ```
    pub async fn create_task(&self, req: &PodcastTaskRequest) -> Result<PodcastTaskResult> {
        let auth = self.http.auth();
        let req_id = crate::client::generate_req_id();

        // Build dialogue list
        let dialogues: Vec<serde_json::Value> = req
            .script
            .iter()
            .map(|line| {
                let mut d = serde_json::json!({
                    "speaker": line.speaker_id,
                    "text": line.text,
                });
                if let Some(ref emotion) = line.emotion {
                    d["emotion"] = serde_json::json!(emotion);
                }
                if let Some(speed) = line.speed_ratio {
                    d["speed_ratio"] = serde_json::json!(speed);
                }
                d
            })
            .collect();

        let mut submit_req = serde_json::json!({
            "app": {
                "appid": auth.app_id,
                "cluster": auth.cluster.as_deref().unwrap_or("volcano_mega"),
            },
            "user": {
                "uid": auth.user_id,
            },
            "request": {
                "reqid": req_id,
                "dialogues": dialogues,
            },
        });

        if let Some(ref encoding) = req.encoding {
            let mut audio = serde_json::json!({
                "encoding": encoding.as_str(),
            });
            if let Some(sample_rate) = req.sample_rate {
                audio["sample_rate"] = serde_json::json!(sample_rate.as_i32());
            }
            submit_req["audio"] = audio;
        }

        if let Some(ref callback) = req.callback_url {
            submit_req["request"]["callback_url"] = serde_json::json!(callback);
        }

        let response: AsyncTaskResponse = self
            .http
            .request("POST", "/api/v1/podcast/submit", Some(&submit_req))
            .await?;

        if response.code != 0 {
            return Err(Error::api(response.code, response.message, 200));
        }

        Ok(PodcastTaskResult {
            task_id: response.task_id,
            req_id,
        })
    }

    /// Queries podcast task status.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let status = client.podcast().get_task("task-id").await?;
    /// println!("Status: {:?}", status.status);
    /// ```
    pub async fn get_task(&self, task_id: &str) -> Result<PodcastTaskStatus> {
        let auth = self.http.auth();

        let query_req = serde_json::json!({
            "appid": auth.app_id,
            "task_id": task_id,
        });

        let response: PodcastQueryResponse = self
            .http
            .request("POST", "/api/v1/podcast/query", Some(&query_req))
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
            Some(PodcastResult {
                audio_url: response.data.audio_url,
                duration: response.data.duration,
            })
        } else {
            None
        };

        Ok(PodcastTaskStatus {
            task_id: response.data.task_id,
            status,
            progress: response.data.progress,
            result,
        })
    }
}

// ================== Request Types ==================

/// Podcast synthesis task request.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PodcastTaskRequest {
    /// Script lines.
    pub script: Vec<PodcastLine>,
    /// Audio encoding.
    #[serde(default)]
    pub encoding: Option<AudioEncoding>,
    /// Sample rate.
    #[serde(default)]
    pub sample_rate: Option<SampleRate>,
    /// Callback URL.
    #[serde(default)]
    pub callback_url: Option<String>,
}

/// Podcast script line.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PodcastLine {
    /// Speaker ID.
    pub speaker_id: String,
    /// Text content.
    pub text: String,
    /// Emotion.
    #[serde(default)]
    pub emotion: Option<String>,
    /// Speed ratio.
    #[serde(default)]
    pub speed_ratio: Option<f32>,
}

// ================== Response Types ==================

/// Podcast task creation result.
#[derive(Debug, Clone, Default, Serialize)]
pub struct PodcastTaskResult {
    /// Task ID.
    pub task_id: String,
    /// Request ID.
    pub req_id: String,
}

/// Podcast task status.
#[derive(Debug, Clone, Default, Serialize)]
pub struct PodcastTaskStatus {
    /// Task ID.
    pub task_id: String,
    /// Status.
    pub status: TaskStatus,
    /// Progress (0-100).
    pub progress: i32,
    /// Result (when completed).
    pub result: Option<PodcastResult>,
}

/// Podcast synthesis result.
#[derive(Debug, Clone, Default, Serialize)]
pub struct PodcastResult {
    /// Audio URL.
    pub audio_url: String,
    /// Audio duration in milliseconds.
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
struct PodcastQueryResponse {
    #[serde(default)]
    code: i32,
    #[serde(default)]
    message: String,
    #[serde(default)]
    data: PodcastQueryData,
}

#[derive(Debug, Deserialize, Default)]
struct PodcastQueryData {
    #[serde(default)]
    task_id: String,
    #[serde(default)]
    status: String,
    #[serde(default)]
    progress: i32,
    #[serde(default)]
    audio_url: String,
    #[serde(default)]
    duration: i32,
}
