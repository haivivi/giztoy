//! Meeting transcription service for Doubao Speech API.

use std::sync::Arc;

use serde::{Deserialize, Serialize};

use crate::{
    error::{Error, Result},
    http::HttpClient,
    types::{AudioFormat, Language, TaskStatus},
};

/// Meeting transcription service.
///
/// API Documentation: https://www.volcengine.com/docs/6561/1305191
pub struct MeetingService {
    http: Arc<HttpClient>,
}

impl MeetingService {
    /// Creates a new Meeting service.
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Creates a meeting transcription task.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use giztoy_doubaospeech::{Client, MeetingTaskRequest};
    ///
    /// let client = Client::builder("app-id").api_key("api-key").build()?;
    /// let result = client.meeting().create_task(&MeetingTaskRequest {
    ///     audio_url: "https://example.com/meeting.wav".to_string(),
    ///     ..Default::default()
    /// }).await?;
    /// ```
    pub async fn create_task(&self, req: &MeetingTaskRequest) -> Result<MeetingTaskResult> {
        let auth = self.http.auth();
        let req_id = crate::client::generate_req_id();

        let mut submit_req = serde_json::json!({
            "appid": auth.app_id,
            "reqid": req_id,
            "audio_url": req.audio_url,
        });

        if let Some(ref format) = req.format {
            submit_req["format"] = serde_json::json!(format.as_str());
        }
        if let Some(ref language) = req.language {
            submit_req["language"] = serde_json::json!(language.as_str());
        }
        if let Some(count) = req.speaker_count {
            submit_req["speaker_count"] = serde_json::json!(count);
        }
        if req.enable_speaker_diarization {
            submit_req["enable_speaker_diarization"] = serde_json::json!(true);
        }
        if req.enable_timestamp {
            submit_req["enable_timestamp"] = serde_json::json!(true);
        }
        if let Some(ref callback) = req.callback_url {
            submit_req["callback_url"] = serde_json::json!(callback);
        }

        let response: AsyncTaskResponse = self
            .http
            .request("POST", "/api/v1/meeting/create", Some(&submit_req))
            .await?;

        if response.code != 0 {
            return Err(Error::api(response.code, response.message, 200));
        }

        Ok(MeetingTaskResult {
            task_id: response.task_id,
            req_id,
        })
    }

    /// Queries meeting task status.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let status = client.meeting().get_task("task-id").await?;
    /// println!("Status: {:?}", status.status);
    /// ```
    pub async fn get_task(&self, task_id: &str) -> Result<MeetingTaskStatus> {
        let auth = self.http.auth();

        let query_req = serde_json::json!({
            "appid": auth.app_id,
            "task_id": task_id,
        });

        let response: MeetingQueryResponse = self
            .http
            .request("POST", "/api/v1/meeting/query", Some(&query_req))
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
            let segments = response
                .data
                .result
                .as_ref()
                .map(|r| {
                    r.segments
                        .iter()
                        .map(|s| MeetingSegment {
                            text: s.text.clone(),
                            start_time: s.start_time,
                            end_time: s.end_time,
                            speaker_id: s.speaker_id.clone(),
                        })
                        .collect()
                })
                .unwrap_or_default();

            Some(MeetingResult {
                text: response.data.result.as_ref().map(|r| r.text.clone()).unwrap_or_default(),
                duration: response.data.result.as_ref().map(|r| r.duration).unwrap_or(0),
                segments,
            })
        } else {
            None
        };

        Ok(MeetingTaskStatus {
            task_id: response.data.task_id,
            status,
            progress: response.data.progress,
            result,
        })
    }
}

// ================== Request Types ==================

/// Meeting transcription task request.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct MeetingTaskRequest {
    /// Audio URL.
    pub audio_url: String,
    /// Audio format.
    #[serde(default)]
    pub format: Option<AudioFormat>,
    /// Language.
    #[serde(default)]
    pub language: Option<Language>,
    /// Expected speaker count.
    #[serde(default)]
    pub speaker_count: Option<i32>,
    /// Enable speaker diarization.
    #[serde(default)]
    pub enable_speaker_diarization: bool,
    /// Enable timestamp.
    #[serde(default)]
    pub enable_timestamp: bool,
    /// Callback URL.
    #[serde(default)]
    pub callback_url: Option<String>,
}

// ================== Response Types ==================

/// Meeting task creation result.
#[derive(Debug, Clone, Default, Serialize)]
pub struct MeetingTaskResult {
    /// Task ID.
    pub task_id: String,
    /// Request ID.
    pub req_id: String,
}

/// Meeting task status.
#[derive(Debug, Clone, Default, Serialize)]
pub struct MeetingTaskStatus {
    /// Task ID.
    pub task_id: String,
    /// Status.
    pub status: TaskStatus,
    /// Progress (0-100).
    pub progress: i32,
    /// Result (when completed).
    pub result: Option<MeetingResult>,
}

/// Meeting transcription result.
#[derive(Debug, Clone, Default, Serialize)]
pub struct MeetingResult {
    /// Full transcription text.
    pub text: String,
    /// Audio duration in milliseconds.
    pub duration: i32,
    /// Transcription segments.
    pub segments: Vec<MeetingSegment>,
}

/// Meeting transcription segment.
#[derive(Debug, Clone, Default, Serialize)]
pub struct MeetingSegment {
    /// Segment text.
    pub text: String,
    /// Start time in milliseconds.
    pub start_time: i32,
    /// End time in milliseconds.
    pub end_time: i32,
    /// Speaker ID.
    pub speaker_id: Option<String>,
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
struct MeetingQueryResponse {
    #[serde(default)]
    code: i32,
    #[serde(default)]
    message: String,
    #[serde(default)]
    data: MeetingQueryData,
}

#[derive(Debug, Deserialize, Default)]
struct MeetingQueryData {
    #[serde(default)]
    task_id: String,
    #[serde(default)]
    status: String,
    #[serde(default)]
    progress: i32,
    #[serde(default)]
    result: Option<MeetingQueryResult>,
}

#[derive(Debug, Deserialize, Default)]
struct MeetingQueryResult {
    #[serde(default)]
    text: String,
    #[serde(default)]
    duration: i32,
    #[serde(default)]
    segments: Vec<MeetingQuerySegment>,
}

#[derive(Debug, Deserialize, Default)]
struct MeetingQuerySegment {
    #[serde(default)]
    text: String,
    #[serde(default)]
    start_time: i32,
    #[serde(default)]
    end_time: i32,
    #[serde(default)]
    speaker_id: Option<String>,
}
