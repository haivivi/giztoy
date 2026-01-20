//! File management service.

use std::sync::Arc;

use serde::{Deserialize, Serialize};

use super::{
    error::Result,
    http::HttpClient,
    types::{BaseResp, FilePurpose, FlexibleId},
};

/// File management service.
pub struct FileService {
    http: Arc<HttpClient>,
}

impl FileService {
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Uploads a file.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let audio_bytes = std::fs::read("voice_sample.mp3")?;
    ///
    /// let response = client.file()
    ///     .upload(&audio_bytes, "voice_sample.mp3", FilePurpose::VoiceClone)
    ///     .await?;
    ///
    /// println!("File ID: {}", response.file_id);
    /// ```
    pub async fn upload(
        &self,
        data: &[u8],
        filename: &str,
        purpose: FilePurpose,
    ) -> Result<UploadResponse> {
        let purpose_str = match purpose {
            FilePurpose::VoiceClone => "voice_clone",
            FilePurpose::PromptAudio => "prompt_audio",
            FilePurpose::T2aAsyncInput => "t2a_async_input",
        };

        self.http
            .upload_file(
                "/v1/files/upload",
                data.to_vec(),
                filename,
                vec![("purpose", purpose_str.to_string())],
            )
            .await
    }

    /// Lists uploaded files.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let files = client.file().list(None).await?;
    ///
    /// for file in &files.files {
    ///     println!("{}: {} ({} bytes)", file.file_id, file.filename, file.bytes);
    /// }
    /// ```
    pub async fn list(&self, purpose: Option<FilePurpose>) -> Result<FileListResponse> {
        let path = match purpose {
            Some(FilePurpose::VoiceClone) => "/v1/files/list?purpose=voice_clone",
            Some(FilePurpose::PromptAudio) => "/v1/files/list?purpose=prompt_audio",
            Some(FilePurpose::T2aAsyncInput) => "/v1/files/list?purpose=t2a_async_input",
            None => "/v1/files/list",
        };

        self.http.request::<(), _>("GET", path, None).await
    }

    /// Gets information about a file.
    pub async fn get(&self, file_id: &str) -> Result<FileInfo> {
        let path = format!("/v1/files/retrieve?file_id={}", file_id);
        
        #[derive(Deserialize)]
        struct Response {
            file: FileInfo,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        let resp: Response = self.http.request::<(), _>("GET", &path, None).await?;
        Ok(resp.file)
    }

    /// Deletes a file.
    pub async fn delete(&self, file_id: &str) -> Result<()> {
        #[derive(Serialize)]
        struct Request<'a> {
            file_id: &'a str,
        }

        #[derive(Deserialize)]
        struct Response {
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        let _: Response = self
            .http
            .request("POST", "/v1/files/delete", Some(&Request { file_id }))
            .await?;

        Ok(())
    }

    /// Gets a download URL for a file.
    pub async fn get_download_url(&self, file_id: &str) -> Result<String> {
        let path = format!("/v1/files/retrieve?file_id={}", file_id);

        #[derive(Deserialize)]
        struct Response {
            file: FileWithUrl,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        #[derive(Deserialize)]
        struct FileWithUrl {
            #[serde(default)]
            download_url: String,
        }

        let resp: Response = self.http.request::<(), _>("GET", &path, None).await?;
        Ok(resp.file.download_url)
    }
}

// ==================== Request/Response Types ====================

/// Response from file upload.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct UploadResponse {
    /// File identifier.
    pub file_id: FlexibleId,
}

/// Information about a file.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct FileInfo {
    /// File identifier.
    pub file_id: FlexibleId,

    /// File name.
    #[serde(default)]
    pub filename: String,

    /// File size in bytes.
    #[serde(default)]
    pub bytes: i64,

    /// Creation timestamp.
    #[serde(default)]
    pub created_at: i64,

    /// File purpose.
    #[serde(default)]
    pub purpose: String,

    /// File status.
    #[serde(default)]
    pub status: String,
}

/// Response from listing files.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct FileListResponse {
    /// List of files.
    #[serde(default)]
    pub files: Vec<FileInfo>,
}
