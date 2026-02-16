use reqwest::Client;
use serde::{Deserialize, Serialize};

use crate::error::EmbedError;

/// OpenAI-compatible embedding request body.
#[derive(Serialize)]
struct EmbeddingRequest<'a> {
    model: &'a str,
    input: &'a [&'a str],
    dimensions: usize,
    encoding_format: &'a str,
}

/// OpenAI-compatible embedding response.
#[derive(Deserialize)]
struct EmbeddingResponse {
    data: Vec<EmbeddingData>,
}

#[derive(Deserialize)]
struct EmbeddingData {
    index: usize,
    embedding: Vec<f64>,
}

/// Call an OpenAI-compatible embedding API endpoint.
///
/// Both DashScope and OpenAI use the same request/response format.
/// The only differences are base_url, model name, and max batch size
/// (handled by the caller).
pub(crate) async fn call_embedding_api(
    client: &Client,
    api_key: &str,
    base_url: &str,
    model: &str,
    dimensions: usize,
    texts: &[&str],
) -> Result<Vec<Vec<f32>>, EmbedError> {
    let url = format!("{base_url}/embeddings");
    let body = EmbeddingRequest {
        model,
        input: texts,
        dimensions,
        encoding_format: "float",
    };

    let resp = client
        .post(&url)
        .header("Authorization", format!("Bearer {api_key}"))
        .header("Content-Type", "application/json")
        .json(&body)
        .send()
        .await
        .map_err(|e| EmbedError::Api(e.to_string()))?;

    if !resp.status().is_success() {
        let status = resp.status();
        let body = resp.text().await.unwrap_or_default();
        return Err(EmbedError::Api(format!(
            "HTTP {status}: {body}"
        )));
    }

    let data: EmbeddingResponse = resp
        .json()
        .await
        .map_err(|e| EmbedError::Api(e.to_string()))?;

    // Fill results by index (API may return out of order).
    let mut vecs: Vec<Option<Vec<f32>>> = vec![None; texts.len()];
    for item in data.data {
        if item.index >= texts.len() {
            return Err(EmbedError::UnexpectedIndex {
                index: item.index,
                batch_size: texts.len(),
            });
        }
        // float64 -> f32 conversion (matching Go behavior).
        vecs[item.index] = Some(item.embedding.iter().map(|&v| v as f32).collect());
    }

    // Verify all slots are filled.
    vecs.into_iter()
        .enumerate()
        .map(|(i, v)| v.ok_or(EmbedError::MissingIndex(i)))
        .collect()
}
