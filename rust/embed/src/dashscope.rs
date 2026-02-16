use reqwest::Client;

use crate::config::EmbedConfig;
use crate::embed::Embedder;
use crate::error::EmbedError;

/// DashScope embedding models.
pub const MODEL_DASHSCOPE_V4: &str = "text-embedding-v4";
pub const MODEL_DASHSCOPE_V3: &str = "text-embedding-v3";
pub const MODEL_DASHSCOPE_V2: &str = "text-embedding-v2";
pub const MODEL_DASHSCOPE_V1: &str = "text-embedding-v1";

const DASHSCOPE_BASE_URL: &str = "https://dashscope.aliyuncs.com/compatible-mode/v1";
const DASHSCOPE_MAX_BATCH: usize = 10;
const DASHSCOPE_DEFAULT_DIM: usize = 1024;

/// DashScope embedder using Aliyun DashScope's OpenAI-compatible API.
pub struct DashScope {
    client: Client,
    api_key: String,
    model: String,
    dim: usize,
    base_url: String,
}

impl DashScope {
    pub fn new(api_key: &str) -> Self {
        Self {
            client: Client::new(),
            api_key: api_key.to_string(),
            model: MODEL_DASHSCOPE_V4.to_string(),
            dim: DASHSCOPE_DEFAULT_DIM,
            base_url: DASHSCOPE_BASE_URL.to_string(),
        }
    }

    pub fn with_config(api_key: &str, cfg: EmbedConfig) -> Self {
        Self {
            client: Client::new(),
            api_key: api_key.to_string(),
            model: if cfg.model.is_empty() {
                MODEL_DASHSCOPE_V4.to_string()
            } else {
                cfg.model
            },
            dim: if cfg.dimension == 0 {
                DASHSCOPE_DEFAULT_DIM
            } else {
                cfg.dimension
            },
            base_url: if cfg.base_url.is_empty() {
                DASHSCOPE_BASE_URL.to_string()
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
impl Embedder for DashScope {
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
        for chunk in texts.chunks(DASHSCOPE_MAX_BATCH) {
            let vecs = self.call_api(chunk).await?;
            result.extend(vecs);
        }
        Ok(result)
    }

    fn dimension(&self) -> usize {
        self.dim
    }
}
