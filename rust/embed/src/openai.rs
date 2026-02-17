use reqwest::Client;

use crate::config::EmbedConfig;
use crate::embed::Embedder;
use crate::error::EmbedError;

/// OpenAI embedding models.
pub const MODEL_OPENAI_3_SMALL: &str = "text-embedding-3-small";
pub const MODEL_OPENAI_3_LARGE: &str = "text-embedding-3-large";
pub const MODEL_OPENAI_ADA_002: &str = "text-embedding-ada-002";

const OPENAI_BASE_URL: &str = "https://api.openai.com/v1";
const OPENAI_MAX_BATCH: usize = 2048;
const OPENAI_DEFAULT_DIM: usize = 1536;

/// OpenAI embedder using the OpenAI embeddings API.
///
/// Also works with any OpenAI-compatible provider (e.g. SiliconFlow)
/// via `with_config` and `EmbedConfig::with_base_url`.
pub struct OpenAI {
    client: Client,
    api_key: String,
    model: String,
    dim: usize,
    base_url: String,
}

impl OpenAI {
    pub fn new(api_key: &str) -> Self {
        Self {
            client: Client::new(),
            api_key: api_key.to_string(),
            model: MODEL_OPENAI_3_SMALL.to_string(),
            dim: OPENAI_DEFAULT_DIM,
            base_url: OPENAI_BASE_URL.to_string(),
        }
    }

    pub fn with_config(api_key: &str, cfg: EmbedConfig) -> Self {
        Self {
            client: Client::new(),
            api_key: api_key.to_string(),
            model: if cfg.model.is_empty() {
                MODEL_OPENAI_3_SMALL.to_string()
            } else {
                cfg.model
            },
            dim: if cfg.dimension == 0 {
                OPENAI_DEFAULT_DIM
            } else {
                cfg.dimension
            },
            base_url: if cfg.base_url.is_empty() {
                OPENAI_BASE_URL.to_string()
            } else {
                cfg.base_url
            },
        }
    }

    async fn call_api(&self, texts: &[&str]) -> Result<Vec<Vec<f32>>, EmbedError> {
        crate::openai_compat::call_embedding_api(
            &self.client,
            &self.api_key,
            &self.base_url,
            &self.model,
            self.dim,
            texts,
        )
        .await
    }
}

#[async_trait::async_trait]
impl Embedder for OpenAI {
    async fn embed(&self, text: &str) -> Result<Vec<f32>, EmbedError> {
        if text.is_empty() {
            return Err(EmbedError::EmptyInput);
        }
        let vecs = self.embed_batch(&[text]).await?;
        Ok(vecs.into_iter().next().unwrap())
    }

    async fn embed_batch(&self, texts: &[&str]) -> Result<Vec<Vec<f32>>, EmbedError> {
        if texts.is_empty() {
            return Err(EmbedError::EmptyInput);
        }

        let mut result = Vec::with_capacity(texts.len());
        for chunk in texts.chunks(OPENAI_MAX_BATCH) {
            let vecs = self.call_api(chunk).await?;
            result.extend(vecs);
        }
        Ok(result)
    }

    fn dimension(&self) -> usize {
        self.dim
    }
}
