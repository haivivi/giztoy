//! Image generation service.

use std::sync::Arc;

use serde::{Deserialize, Serialize};

use super::{error::Result, http::HttpClient, types::BaseResp};

/// Image generation service.
pub struct ImageService {
    http: Arc<HttpClient>,
}

impl ImageService {
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Generates images from a text prompt.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let request = ImageGenerateRequest {
    ///     model: "image-01".to_string(),
    ///     prompt: "A beautiful sunset over mountains".to_string(),
    ///     aspect_ratio: Some("16:9".to_string()),
    ///     n: Some(1),
    ///     ..Default::default()
    /// };
    ///
    /// let response = client.image().generate(&request).await?;
    /// println!("Generated image URL: {}", response.images[0].url);
    /// ```
    pub async fn generate(&self, request: &ImageGenerateRequest) -> Result<ImageResponse> {
        #[derive(Deserialize)]
        struct ApiResponse {
            data: ImageResponseData,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        #[derive(Deserialize)]
        struct ImageResponseData {
            #[serde(default)]
            images: Vec<ImageData>,
        }

        let resp: ApiResponse = self
            .http
            .request("POST", "/v1/image/generation", Some(request))
            .await?;

        Ok(ImageResponse {
            images: resp.data.images,
        })
    }

    /// Generates images with a reference image.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let request = ImageReferenceRequest {
    ///     model: "image-01".to_string(),
    ///     prompt: "Same style but with a cat".to_string(),
    ///     image_prompt: "https://example.com/reference.jpg".to_string(),
    ///     image_prompt_strength: Some(0.5),
    ///     ..Default::default()
    /// };
    ///
    /// let response = client.image().generate_with_reference(&request).await?;
    /// ```
    pub async fn generate_with_reference(
        &self,
        request: &ImageReferenceRequest,
    ) -> Result<ImageResponse> {
        #[derive(Deserialize)]
        struct ApiResponse {
            data: ImageResponseData,
            #[serde(default)]
            base_resp: Option<BaseResp>,
        }

        #[derive(Deserialize)]
        struct ImageResponseData {
            #[serde(default)]
            images: Vec<ImageData>,
        }

        let resp: ApiResponse = self
            .http
            .request("POST", "/v1/image/generation", Some(request))
            .await?;

        Ok(ImageResponse {
            images: resp.data.images,
        })
    }
}

// ==================== Request/Response Types ====================

/// Request for image generation.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ImageGenerateRequest {
    /// Model name.
    pub model: String,

    /// Image description.
    pub prompt: String,

    /// Aspect ratio: 1:1, 16:9, 9:16, 4:3, 3:4, 3:2, 2:3, 21:9, 9:21.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub aspect_ratio: Option<String>,

    /// Number of images to generate (1-9).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub n: Option<i32>,

    /// Enable prompt optimization.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub prompt_optimizer: Option<bool>,
}

/// Request for image generation with reference.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ImageReferenceRequest {
    /// Model name.
    pub model: String,

    /// Image description.
    pub prompt: String,

    /// Reference image URL.
    pub image_prompt: String,

    /// Reference image influence (0-1).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub image_prompt_strength: Option<f64>,

    /// Aspect ratio.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub aspect_ratio: Option<String>,

    /// Number of images to generate.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub n: Option<i32>,

    /// Enable prompt optimization.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub prompt_optimizer: Option<bool>,
}

/// Response from image generation.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ImageResponse {
    pub images: Vec<ImageData>,
}

/// Image data.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ImageData {
    /// Image URL.
    pub url: String,
}
