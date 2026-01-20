//! Console API service for Doubao Speech.
//!
//! Provides access to Volcengine Console APIs for managing voices, timbres, etc.

use std::time::Duration;

use reqwest::Client as ReqwestClient;
use serde::{Deserialize, Serialize};

use crate::error::{Error, Result};

const CONSOLE_BASE_URL: &str = "https://open.volcengineapi.com";
// Reserved for AK/SK signature authentication
#[allow(dead_code)]
const CONSOLE_SERVICE: &str = "speech_saas_prod";
#[allow(dead_code)]
const CONSOLE_REGION: &str = "cn-north-1";

/// Console API client.
///
/// Supports two authentication methods:
/// 1. API Key (recommended): Use `Console::with_api_key`
/// 2. AK/SK signature: Use `Console::with_ak_sk`
pub struct Console {
    config: ConsoleConfig,
    client: ReqwestClient,
}

struct ConsoleConfig {
    api_key: Option<String>,
    access_key: Option<String>,
    secret_key: Option<String>,
    base_url: String,
}

impl Console {
    /// Creates a Console client using API Key authentication.
    ///
    /// API Key is from Volcengine console:
    /// https://console.volcengine.com/speech/new/setting/apikeys
    pub fn with_api_key(api_key: impl Into<String>) -> Self {
        Self {
            config: ConsoleConfig {
                api_key: Some(api_key.into()),
                access_key: None,
                secret_key: None,
                base_url: CONSOLE_BASE_URL.to_string(),
            },
            client: ReqwestClient::builder()
                .timeout(Duration::from_secs(30))
                .build()
                .expect("Failed to create HTTP client"),
        }
    }

    /// Creates a Console client using AK/SK signature authentication.
    ///
    /// Access Key and Secret Key are from Volcengine IAM console.
    pub fn with_ak_sk(access_key: impl Into<String>, secret_key: impl Into<String>) -> Self {
        Self {
            config: ConsoleConfig {
                api_key: None,
                access_key: Some(access_key.into()),
                secret_key: Some(secret_key.into()),
                base_url: CONSOLE_BASE_URL.to_string(),
            },
            client: ReqwestClient::builder()
                .timeout(Duration::from_secs(30))
                .build()
                .expect("Failed to create HTTP client"),
        }
    }

    /// Lists available TTS timbres (big model).
    ///
    /// API: ListBigModelTTSTimbres, Version: 2025-05-20
    pub async fn list_timbres(&self, req: &ListTimbresRequest) -> Result<ListTimbresResponse> {
        self.do_request("ListBigModelTTSTimbres", "2025-05-20", req)
            .await
    }

    /// Lists available speakers.
    ///
    /// API: ListSpeakers, Version: 2025-05-20
    pub async fn list_speakers(&self, req: &ListSpeakersRequest) -> Result<ListSpeakersResponse> {
        self.do_request("ListSpeakers", "2025-05-20", req).await
    }

    /// Lists voice clone training status.
    ///
    /// API: ListMegaTTSTrainStatus, Version: 2023-11-07
    pub async fn list_voice_clone_status(
        &self,
        req: &ListVoiceCloneStatusRequest,
    ) -> Result<ListVoiceCloneStatusResponse> {
        self.do_request("ListMegaTTSTrainStatus", "2023-11-07", req)
            .await
    }

    async fn do_request<T: Serialize, R: for<'de> Deserialize<'de>>(
        &self,
        action: &str,
        version: &str,
        body: &T,
    ) -> Result<R> {
        let url = format!(
            "{}?Action={}&Version={}",
            self.config.base_url, action, version
        );

        let mut request = self.client.post(&url).json(body);

        // Set authentication
        if let Some(ref api_key) = self.config.api_key {
            request = request.header("x-api-key", api_key);
        } else if self.config.access_key.is_some() && self.config.secret_key.is_some() {
            // AK/SK signature authentication would go here
            // For simplicity, we only implement API Key auth in this version
            return Err(Error::Config(
                "AK/SK signature authentication not yet implemented".to_string(),
            ));
        } else {
            return Err(Error::Config(
                "No authentication configured: use API Key or AK/SK".to_string(),
            ));
        }

        let response = request
            .send()
            .await?;

        let status = response.status();
        let body_bytes = response
            .bytes()
            .await?;

        // Parse response
        let api_resp: ConsoleApiResponse<R> =
            serde_json::from_slice(&body_bytes)?;

        if let Some(error) = api_resp.response_metadata.error {
            return Err(Error::api(error.code_n, error.message, status.as_u16()));
        }

        api_resp.result.ok_or_else(|| {
            Error::api(-1, "No result in response", status.as_u16())
        })
    }
}

// ================== Request Types ==================

/// Request for listing timbres.
#[derive(Debug, Clone, Default, Serialize)]
pub struct ListTimbresRequest {
    /// Page number.
    #[serde(rename = "PageNumber", skip_serializing_if = "Option::is_none")]
    pub page_number: Option<i32>,
    /// Page size.
    #[serde(rename = "PageSize", skip_serializing_if = "Option::is_none")]
    pub page_size: Option<i32>,
    /// Timbre type filter.
    #[serde(rename = "TimbreType", skip_serializing_if = "Option::is_none")]
    pub timbre_type: Option<String>,
}

/// Request for listing speakers.
#[derive(Debug, Clone, Default, Serialize)]
pub struct ListSpeakersRequest {
    /// Page number.
    #[serde(rename = "PageNumber", skip_serializing_if = "Option::is_none")]
    pub page_number: Option<i32>,
    /// Page size.
    #[serde(rename = "PageSize", skip_serializing_if = "Option::is_none")]
    pub page_size: Option<i32>,
    /// Speaker type filter.
    #[serde(rename = "SpeakerType", skip_serializing_if = "Option::is_none")]
    pub speaker_type: Option<String>,
    /// Language filter.
    #[serde(rename = "Language", skip_serializing_if = "Option::is_none")]
    pub language: Option<String>,
}

/// Request for listing voice clone status.
#[derive(Debug, Clone, Default, Serialize)]
pub struct ListVoiceCloneStatusRequest {
    /// App ID.
    #[serde(rename = "AppID")]
    pub app_id: String,
    /// Page number.
    #[serde(rename = "PageNumber", skip_serializing_if = "Option::is_none")]
    pub page_number: Option<i32>,
    /// Page size.
    #[serde(rename = "PageSize", skip_serializing_if = "Option::is_none")]
    pub page_size: Option<i32>,
    /// Status filter.
    #[serde(rename = "Status", skip_serializing_if = "Option::is_none")]
    pub status: Option<String>,
}

// ================== Response Types ==================

/// Response for listing timbres.
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct ListTimbresResponse {
    /// List of timbres.
    #[serde(rename = "Timbres", default)]
    pub timbres: Vec<TimbreInfo>,
}

/// Timbre information.
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct TimbreInfo {
    /// Speaker ID.
    #[serde(rename = "SpeakerID", default)]
    pub speaker_id: String,
    /// Timbre details.
    #[serde(rename = "TimbreInfos", default)]
    pub timbre_infos: Vec<TimbreDetailInfo>,
}

/// Timbre detail information.
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct TimbreDetailInfo {
    /// Speaker name.
    #[serde(rename = "SpeakerName", default)]
    pub speaker_name: String,
    /// Gender.
    #[serde(rename = "Gender", default)]
    pub gender: String,
    /// Age.
    #[serde(rename = "Age", default)]
    pub age: String,
    /// Categories.
    #[serde(rename = "Categories", default)]
    pub categories: Vec<TimbreCategory>,
    /// Emotions.
    #[serde(rename = "Emotions", default)]
    pub emotions: Vec<TimbreEmotion>,
}

/// Timbre category.
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct TimbreCategory {
    /// Category name.
    #[serde(rename = "Category", default)]
    pub category: String,
}

/// Timbre emotion.
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct TimbreEmotion {
    /// Emotion name.
    #[serde(rename = "Emotion", default)]
    pub emotion: String,
    /// Emotion type.
    #[serde(rename = "EmotionType", default)]
    pub emotion_type: String,
    /// Demo text.
    #[serde(rename = "DemoText", default)]
    pub demo_text: String,
    /// Demo URL.
    #[serde(rename = "DemoURL", default)]
    pub demo_url: String,
}

/// Response for listing speakers.
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct ListSpeakersResponse {
    /// Total count.
    #[serde(rename = "Total", default)]
    pub total: i32,
    /// List of speakers.
    #[serde(rename = "Speakers", default)]
    pub speakers: Vec<SpeakerInfo>,
}

/// Speaker information.
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct SpeakerInfo {
    /// Speaker ID.
    #[serde(rename = "ID", default)]
    pub id: String,
    /// Voice type.
    #[serde(rename = "VoiceType", default)]
    pub voice_type: String,
    /// Speaker name.
    #[serde(rename = "Name", default)]
    pub name: String,
    /// Avatar URL.
    #[serde(rename = "Avatar", default)]
    pub avatar: String,
    /// Gender.
    #[serde(rename = "Gender", default)]
    pub gender: String,
    /// Age.
    #[serde(rename = "Age", default)]
    pub age: String,
    /// Trial URL.
    #[serde(rename = "TrialURL", default)]
    pub trial_url: Option<String>,
}

/// Response for listing voice clone status.
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct ListVoiceCloneStatusResponse {
    /// Total count.
    #[serde(rename = "Total", default)]
    pub total: i32,
    /// List of statuses.
    #[serde(rename = "Statuses", default)]
    pub statuses: Vec<VoiceCloneTrainStatus>,
}

/// Voice clone training status.
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct VoiceCloneTrainStatus {
    /// Speaker ID.
    #[serde(rename = "SpeakerID", default)]
    pub speaker_id: String,
    /// Instance number.
    #[serde(rename = "InstanceNO", default)]
    pub instance_no: String,
    /// Whether activatable.
    #[serde(rename = "IsActivatable", default)]
    pub is_activatable: bool,
    /// State.
    #[serde(rename = "State", default)]
    pub state: String,
    /// Demo audio URL.
    #[serde(rename = "DemoAudio", default)]
    pub demo_audio: Option<String>,
    /// Version.
    #[serde(rename = "Version", default)]
    pub version: String,
    /// Create time.
    #[serde(rename = "CreateTime", default)]
    pub create_time: i64,
    /// Expire time.
    #[serde(rename = "ExpireTime", default)]
    pub expire_time: i64,
    /// Alias.
    #[serde(rename = "Alias", default)]
    pub alias: Option<String>,
    /// Resource ID.
    #[serde(rename = "ResourceID", default)]
    pub resource_id: String,
}

// ================== Internal Types ==================

#[derive(Debug, Deserialize)]
struct ConsoleApiResponse<T> {
    #[serde(rename = "ResponseMetadata")]
    response_metadata: ResponseMetadata,
    #[serde(rename = "Result")]
    result: Option<T>,
}

#[derive(Debug, Deserialize)]
struct ResponseMetadata {
    #[serde(rename = "RequestId", default)]
    _request_id: String,
    #[serde(rename = "Action", default)]
    _action: String,
    #[serde(rename = "Version", default)]
    _version: String,
    #[serde(rename = "Error")]
    error: Option<ConsoleError>,
}

#[derive(Debug, Deserialize)]
struct ConsoleError {
    #[serde(rename = "Code", default)]
    _code: String,
    #[serde(rename = "CodeN", default)]
    code_n: i32,
    #[serde(rename = "Message", default)]
    message: String,
}

// Suppress unused warnings for internal fields
#[allow(dead_code)]
const _: () = {
    fn _assert_send<T: Send>() {}
    fn _assert_sync<T: Sync>() {}
};
