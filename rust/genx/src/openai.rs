//! OpenAI Generator implementation.
//!
//! This module provides a generator for the OpenAI API (and compatible endpoints).
//!
//! # Example
//!
//! ```rust,ignore
//! use giztoy_genx::{Generator, ModelContextBuilder};
//! use giztoy_genx::openai::{OpenAIConfig, OpenAIGenerator};
//!
//! let generator = OpenAIGenerator::new(OpenAIConfig {
//!     api_key: "sk-xxx".to_string(),
//!     model: "gpt-4o-mini".to_string(),
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

/// OpenAI Generator configuration.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OpenAIConfig {
    /// API key for authentication
    pub api_key: String,
    /// Base URL for the API (default: https://api.openai.com/v1)
    #[serde(default = "default_base_url")]
    pub base_url: String,
    /// Model name (e.g., "gpt-4o-mini", "gpt-4o")
    pub model: String,
    /// Whether the model supports JSON schema output
    #[serde(default)]
    pub support_json_output: bool,
    /// Whether the model supports tool calls
    #[serde(default)]
    pub support_tool_calls: bool,
    /// Whether the model supports text-only output
    #[serde(default)]
    pub support_text_only: bool,
    /// Whether to use "system" role (true) or "developer" role (false)
    #[serde(default = "default_use_system_role")]
    pub use_system_role: bool,
    /// Default generation parameters
    #[serde(default)]
    pub generate_params: Option<ModelParams>,
    /// Default invoke parameters
    #[serde(default)]
    pub invoke_params: Option<ModelParams>,
}

fn default_base_url() -> String {
    "https://api.openai.com/v1".to_string()
}

fn default_use_system_role() -> bool {
    true
}

impl Default for OpenAIConfig {
    fn default() -> Self {
        Self {
            api_key: String::new(),
            base_url: default_base_url(),
            model: "gpt-4o-mini".to_string(),
            support_json_output: true,
            support_tool_calls: true,
            support_text_only: true,
            use_system_role: true,
            generate_params: None,
            invoke_params: None,
        }
    }
}

/// OpenAI Generator.
///
/// Supports the OpenAI Chat Completions API and compatible endpoints.
pub struct OpenAIGenerator {
    client: Client,
    config: OpenAIConfig,
}

impl OpenAIGenerator {
    /// Create a new OpenAI generator.
    pub fn new(config: OpenAIConfig) -> Self {
        Self {
            client: Client::new(),
            config,
        }
    }

    /// Get the configuration.
    pub fn config(&self) -> &OpenAIConfig {
        &self.config
    }

    /// Convert model context to OpenAI messages format.
    fn convert_messages(&self, ctx: &dyn ModelContext) -> Vec<Value> {
        let mut messages = Vec::new();

        // Add prompts as system messages
        for prompt in ctx.prompts() {
            let role = if self.config.use_system_role {
                "system"
            } else {
                "developer"
            };
            messages.push(json!({
                "role": role,
                "content": prompt.text,
            }));
        }

        // Add conversation messages
        for msg in ctx.messages() {
            match &msg.payload {
                Payload::Contents(contents) => {
                    let role = match msg.role {
                        Role::User => "user",
                        Role::Model => "assistant",
                        Role::Tool => continue,
                    };

                    let mut text_parts = Vec::new();
                    for part in contents {
                        if let Part::Text(t) = part {
                            text_parts.push(t.clone());
                        }
                    }

                    if !text_parts.is_empty() {
                        messages.push(json!({
                            "role": role,
                            "content": text_parts.join(""),
                        }));
                    }
                }
                Payload::ToolCall(tc) => {
                    messages.push(json!({
                        "role": "assistant",
                        "tool_calls": [{
                            "id": tc.id,
                            "type": "function",
                            "function": {
                                "name": tc.func_call.name,
                                "arguments": tc.func_call.arguments,
                            }
                        }]
                    }));
                }
                Payload::ToolResult(tr) => {
                    messages.push(json!({
                        "role": "tool",
                        "tool_call_id": tr.id,
                        "content": tr.result,
                    }));
                }
            }
        }

        messages
    }

    /// Build request body.
    fn build_request(&self, ctx: &dyn ModelContext, stream: bool) -> Value {
        let messages = self.convert_messages(ctx);
        let params = ctx.params().or(self.config.generate_params.as_ref());

        let mut body = json!({
            "model": self.config.model,
            "messages": messages,
            "stream": stream,
        });

        if let Some(p) = params {
            if let Some(max) = p.max_tokens {
                body["max_completion_tokens"] = json!(max);
            }
            if let Some(temp) = p.temperature {
                body["temperature"] = json!(temp);
            }
            if let Some(top_p) = p.top_p {
                body["top_p"] = json!(top_p);
            }
        }

        body
    }
}

#[async_trait]
impl Generator for OpenAIGenerator {
    async fn generate_stream(
        &self,
        _model: &str,
        ctx: &dyn ModelContext,
    ) -> Result<Box<dyn Stream>, GenxError> {
        let body = self.build_request(ctx, true);
        let url = format!("{}/chat/completions", self.config.base_url);

        let response = self
            .client
            .post(&url)
            .header("Authorization", format!("Bearer {}", self.config.api_key))
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
                "OpenAI API error {}: {}",
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

                        // Process complete SSE events
                        while let Some(pos) = buffer.find("\n\n") {
                            let event = buffer[..pos].to_string();
                            buffer = buffer[pos + 2..].to_string();

                            for line in event.lines() {
                                if let Some(data) = line.strip_prefix("data: ") {
                                    if data == "[DONE]" {
                                        let _ = builder_clone.done(Usage::default());
                                        return;
                                    }

                                    if let Ok(json) = serde_json::from_str::<Value>(data) {
                                        if let Some(choices) = json["choices"].as_array() {
                                            for choice in choices {
                                                if let Some(content) =
                                                    choice["delta"]["content"].as_str()
                                                {
                                                    let _ = builder_clone.add(&[
                                                        MessageChunk::text(Role::Model, content),
                                                    ]);
                                                }

                                                // Check finish reason
                                                if let Some(reason) =
                                                    choice["finish_reason"].as_str()
                                                {
                                                    match reason {
                                                        "stop" | "tool_calls" => {
                                                            let _ =
                                                                builder_clone.done(Usage::default());
                                                            return;
                                                        }
                                                        "length" => {
                                                            let _ = builder_clone
                                                                .truncated(Usage::default());
                                                            return;
                                                        }
                                                        "content_filter" => {
                                                            let _ = builder_clone.blocked(
                                                                Usage::default(),
                                                                "content_filter",
                                                            );
                                                            return;
                                                        }
                                                        _ => {}
                                                    }
                                                }
                                            }
                                        }
                                    }
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
        let mut body = self.build_request(ctx, false);

        // Add JSON schema for structured output
        if self.config.support_json_output {
            body["response_format"] = json!({
                "type": "json_schema",
                "json_schema": {
                    "name": tool.name,
                    "description": tool.description,
                    "schema": tool.argument,
                    "strict": true,
                }
            });
        }

        let url = format!("{}/chat/completions", self.config.base_url);

        let response = self
            .client
            .post(&url)
            .header("Authorization", format!("Bearer {}", self.config.api_key))
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
                "OpenAI API error {}: {}",
                status,
                text
            )));
        }

        let json: Value = response
            .json()
            .await
            .map_err(|e| GenxError::Other(anyhow::anyhow!("JSON parse error: {}", e)))?;

        let content = json["choices"][0]["message"]["content"]
            .as_str()
            .ok_or_else(|| GenxError::Other(anyhow::anyhow!("No content in response")))?;

        Ok((Usage::default(), tool.new_func_call(content)))
    }
}
