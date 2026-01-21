//! Google Gemini Generator implementation.
//!
//! This module provides a generator for the Google Gemini API.
//!
//! # Example
//!
//! ```rust,ignore
//! use giztoy_genx::{Generator, ModelContextBuilder};
//! use giztoy_genx::gemini::{GeminiConfig, GeminiGenerator};
//!
//! let generator = GeminiGenerator::new(GeminiConfig {
//!     api_key: "AIza...".to_string(),
//!     model: "gemini-2.0-flash".to_string(),
//!     ..Default::default()
//! });
//!
//! let mut builder = ModelContextBuilder::new();
//! builder.prompt_text("system", "You are helpful.");
//! builder.user_text("user", "Hello!");
//!
//! let ctx = builder.build();
//! let mut stream = generator.generate_stream("", &ctx).await?;
//! ```

use async_trait::async_trait;
use futures::StreamExt;
use reqwest::Client;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::sync::Arc;

use crate::context::{ModelContext, ModelParams};
use crate::error::{GenxError, Usage};
use crate::stream::StreamBuilder;
use crate::tool::FuncTool;
use crate::types::{FuncCall, MessageChunk, Part, Payload, Role};
use crate::{Generator, Stream};

/// Gemini Generator configuration.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GeminiConfig {
    /// API key for authentication
    pub api_key: String,
    /// Model name (e.g., "gemini-2.0-flash", "gemini-1.5-pro")
    pub model: String,
    /// Default generation parameters
    #[serde(default)]
    pub generate_params: Option<ModelParams>,
    /// Default invoke parameters
    #[serde(default)]
    pub invoke_params: Option<ModelParams>,
}

impl Default for GeminiConfig {
    fn default() -> Self {
        Self {
            api_key: String::new(),
            model: "gemini-2.0-flash".to_string(),
            generate_params: None,
            invoke_params: None,
        }
    }
}

/// Gemini Generator.
///
/// Supports the Google Gemini API for text generation.
pub struct GeminiGenerator {
    client: Client,
    config: GeminiConfig,
}

impl GeminiGenerator {
    /// Create a new Gemini generator.
    pub fn new(config: GeminiConfig) -> Self {
        Self {
            client: Client::new(),
            config,
        }
    }

    /// Get the configuration.
    pub fn config(&self) -> &GeminiConfig {
        &self.config
    }

    /// Convert model context to Gemini format.
    fn convert_request(&self, ctx: &dyn ModelContext, _stream: bool) -> Value {
        let mut contents = Vec::new();
        let mut system_instruction: Option<Value> = None;

        // Collect prompts as system instruction
        let prompt_texts: Vec<String> = ctx.prompts().map(|p| p.text.clone()).collect();
        if !prompt_texts.is_empty() {
            system_instruction = Some(json!({
                "parts": [{"text": prompt_texts.join("\n")}]
            }));
        }

        // Convert messages
        let mut current_role: Option<&str> = None;
        let mut current_parts: Vec<Value> = Vec::new();

        for msg in ctx.messages() {
            let role = match msg.role {
                Role::User => "user",
                Role::Model => "model",
                Role::Tool => "user", // Tool results come from user side in Gemini
            };

            // Flush previous content if role changes
            if current_role.is_some() && current_role != Some(role) {
                if !current_parts.is_empty() {
                    contents.push(json!({
                        "role": current_role.unwrap(),
                        "parts": current_parts,
                    }));
                    current_parts = Vec::new();
                }
            }
            current_role = Some(role);

            match &msg.payload {
                Payload::Contents(parts) => {
                    for part in parts {
                        match part {
                            Part::Text(t) => {
                                current_parts.push(json!({"text": t}));
                            }
                            Part::Blob(b) => {
                                current_parts.push(json!({
                                    "inline_data": {
                                        "mime_type": b.mime_type,
                                        "data": base64_encode(&b.data),
                                    }
                                }));
                            }
                        }
                    }
                }
                Payload::ToolCall(tc) => {
                    // Parse arguments as JSON
                    let args: Value =
                        serde_json::from_str(&tc.func_call.arguments).unwrap_or(json!({}));
                    current_parts.push(json!({
                        "functionCall": {
                            "name": tc.func_call.name,
                            "args": args,
                        }
                    }));
                }
                Payload::ToolResult(tr) => {
                    // Parse result as JSON
                    let result: Value = serde_json::from_str(&tr.result).unwrap_or(json!({
                        "text": tr.result
                    }));
                    current_parts.push(json!({
                        "functionResponse": {
                            "name": tr.id,
                            "response": result,
                        }
                    }));
                }
            }
        }

        // Flush remaining content
        if !current_parts.is_empty() {
            if let Some(role) = current_role {
                contents.push(json!({
                    "role": role,
                    "parts": current_parts,
                }));
            }
        }

        let params = ctx.params().or(self.config.generate_params.as_ref());

        let mut generation_config = json!({});
        if let Some(p) = params {
            if let Some(max) = p.max_tokens {
                generation_config["maxOutputTokens"] = json!(max);
            }
            if let Some(temp) = p.temperature {
                generation_config["temperature"] = json!(temp);
            }
            if let Some(top_p) = p.top_p {
                generation_config["topP"] = json!(top_p);
            }
            if let Some(top_k) = p.top_k {
                generation_config["topK"] = json!(top_k as i32);
            }
        }

        let mut body = json!({
            "contents": contents,
            "generationConfig": generation_config,
        });

        if let Some(sys) = system_instruction {
            body["systemInstruction"] = sys;
        }

        body
    }

    fn api_url(&self, stream: bool) -> String {
        let method = if stream {
            "streamGenerateContent"
        } else {
            "generateContent"
        };
        format!(
            "https://generativelanguage.googleapis.com/v1beta/models/{}:{}?key={}{}",
            self.config.model,
            method,
            self.config.api_key,
            if stream { "&alt=sse" } else { "" }
        )
    }
}

/// Simple base64 encoder.
fn base64_encode(data: &[u8]) -> String {
    const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

    let mut result = String::new();
    for chunk in data.chunks(3) {
        let mut n = (chunk[0] as u32) << 16;
        if chunk.len() > 1 {
            n |= (chunk[1] as u32) << 8;
        }
        if chunk.len() > 2 {
            n |= chunk[2] as u32;
        }

        result.push(ALPHABET[(n >> 18) as usize & 0x3F] as char);
        result.push(ALPHABET[(n >> 12) as usize & 0x3F] as char);

        if chunk.len() > 1 {
            result.push(ALPHABET[(n >> 6) as usize & 0x3F] as char);
        } else {
            result.push('=');
        }

        if chunk.len() > 2 {
            result.push(ALPHABET[n as usize & 0x3F] as char);
        } else {
            result.push('=');
        }
    }
    result
}

#[async_trait]
impl Generator for GeminiGenerator {
    async fn generate_stream(
        &self,
        _model: &str,
        ctx: &dyn ModelContext,
    ) -> Result<Box<dyn Stream>, GenxError> {
        let body = self.convert_request(ctx, true);
        let url = self.api_url(true);

        let response = self
            .client
            .post(&url)
            .header("Content-Type", "application/json")
            .json(&body)
            .send()
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("HTTP error: {}", e)))?;

        if !response.status().is_success() {
            let status = response.status();
            let text = response
                .text()
                .await
                .unwrap_or_else(|_| "unknown".to_string());
            return Err(GenxError::Other(anyhow::anyhow!(
                "Gemini API error {}: {}",
                status,
                text
            )));
        }

        let builder = Arc::new(StreamBuilder::with_tools(32, vec![]));
        let builder_clone = builder.clone();

        // Spawn task to read SSE stream
        let mut stream = response.bytes_stream();
        tokio::spawn(async move {
            let mut buffer = String::new();

            while let Some(chunk_result) = stream.next().await {
                match chunk_result {
                    Ok(bytes) => {
                        buffer.push_str(&String::from_utf8_lossy(&bytes));

                        // Process lines (Gemini SSE format: "data: {...}\r\n" or "data: {...}\n")
                        while let Some(pos) = buffer.find('\n') {
                            let line = buffer[..pos].trim().to_string();
                            buffer = buffer[pos + 1..].to_string();

                            if line.is_empty() {
                                continue;
                            }

                            // Handle SSE data lines
                            let data = if let Some(d) = line.strip_prefix("data: ") {
                                d
                            } else if line.starts_with('{') {
                                // Sometimes Gemini sends raw JSON without "data: " prefix
                                &line
                            } else {
                                continue;
                            };

                            if let Ok(json) = serde_json::from_str::<Value>(data) {
                                // Extract text from candidates
                                if let Some(candidates) = json["candidates"].as_array() {
                                    for candidate in candidates {
                                        if let Some(parts) =
                                            candidate["content"]["parts"].as_array()
                                        {
                                            for part in parts {
                                                if let Some(text) = part["text"].as_str() {
                                                    let _ = builder_clone.add(&[
                                                        MessageChunk::text(Role::Model, text),
                                                    ]);
                                                }
                                            }
                                        }

                                        // Check finish reason
                                        if let Some(reason) = candidate["finishReason"].as_str() {
                                            match reason {
                                                "STOP" => {
                                                    let _ = builder_clone.done(Usage::default());
                                                    return;
                                                }
                                                "MAX_TOKENS" => {
                                                    let _ =
                                                        builder_clone.truncated(Usage::default());
                                                    return;
                                                }
                                                "SAFETY" => {
                                                    let _ = builder_clone.blocked(
                                                        Usage::default(),
                                                        "safety filter",
                                                    );
                                                    return;
                                                }
                                                _ => {}
                                            }
                                        }
                                    }
                                }

                                // Also check for error response
                                if json.get("error").is_some() {
                                    let err_msg = json["error"]["message"]
                                        .as_str()
                                        .unwrap_or("Unknown error");
                                    let _ = builder_clone.abort(GenxError::Other(anyhow::anyhow!(
                                        "Gemini error: {}",
                                        err_msg
                                    )));
                                    return;
                                }
                            }
                        }
                    }
                    Err(e) => {
                        let _ = builder_clone.abort(GenxError::Other(anyhow::anyhow!(
                            "Stream error: {}",
                            e
                        )));
                        return;
                    }
                }
            }

            let _ = builder_clone.done(Usage::default());
        });

        Ok(Box::new(builder.stream()))
    }

    async fn invoke(
        &self,
        _model: &str,
        ctx: &dyn ModelContext,
        tool: &FuncTool,
    ) -> Result<(Usage, FuncCall), GenxError> {
        let mut body = self.convert_request(ctx, false);

        // Add JSON schema for structured output
        body["generationConfig"]["responseMimeType"] = json!("application/json");
        body["generationConfig"]["responseSchema"] = tool.argument.clone();

        let url = self.api_url(false);

        let response = self
            .client
            .post(&url)
            .header("Content-Type", "application/json")
            .json(&body)
            .send()
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("HTTP error: {}", e)))?;

        if !response.status().is_success() {
            let status = response.status();
            let text = response
                .text()
                .await
                .unwrap_or_else(|_| "unknown".to_string());
            return Err(GenxError::Other(anyhow::anyhow!(
                "Gemini API error {}: {}",
                status,
                text
            )));
        }

        let json: Value = response
            .json()
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("JSON parse error: {}", e)))?;

        // Extract text from response
        let content = json["candidates"][0]["content"]["parts"][0]["text"]
            .as_str()
            .ok_or_else(|| GenxError::Other(anyhow::anyhow!("No content in response")))?;

        Ok((Usage::default(), tool.new_func_call(content)))
    }
}
