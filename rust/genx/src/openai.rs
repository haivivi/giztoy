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
use base64::Engine;
use futures::StreamExt;
use reqwest::Client;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::sync::Arc;

use crate::context::{ModelContext, ModelParams};
use crate::error::{GenxError, Usage};
use crate::stream::StreamBuilder;
use crate::tool::FuncTool;
use crate::types::{FuncCall, MessageChunk, Part, Payload, Role, ToolCall};
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
    /// Extra fields to include in API requests
    #[serde(default)]
    pub extra_fields: Option<std::collections::HashMap<String, serde_json::Value>>,
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
            extra_fields: None,
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

                    // Check if we have any non-text parts
                    let has_multimodal = contents.iter().any(|p| matches!(p, Part::Blob(_)));

                    if has_multimodal {
                        // Use multimodal format with content array
                        let mut content_parts = Vec::new();
                        for part in contents {
                            match part {
                                Part::Text(t) => {
                                    content_parts.push(json!({
                                        "type": "text",
                                        "text": t,
                                    }));
                                }
                                Part::Blob(blob) => {
                                    // Encode blob as base64 data URL
                                    let b64 = base64::engine::general_purpose::STANDARD
                                        .encode(&blob.data);
                                    let data_url = format!("data:{};base64,{}", blob.mime_type, b64);
                                    content_parts.push(json!({
                                        "type": "image_url",
                                        "image_url": {
                                            "url": data_url,
                                        }
                                    }));
                                }
                            }
                        }
                        if !content_parts.is_empty() {
                            messages.push(json!({
                                "role": role,
                                "content": content_parts,
                            }));
                        }
                    } else {
                        // Use simple text format
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

    /// Convert tools to OpenAI format.
    fn convert_tools(&self, ctx: &dyn ModelContext) -> Vec<Value> {
        let mut tools = Vec::new();
        for tool in ctx.tools() {
            // Only include function tools (those with a schema)
            if let Some(schema) = tool.schema() {
                tools.push(json!({
                    "type": "function",
                    "function": {
                        "name": tool.name(),
                        "description": tool.description(),
                        "parameters": schema,
                        "strict": true,
                    }
                }));
            }
            // Skip built-in tools like SearchWebTool that don't have schemas
        }
        tools
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

        // Add stream options to get usage in streaming mode
        if stream {
            body["stream_options"] = json!({
                "include_usage": true
            });
        }

        // Add tools if any
        let tools = self.convert_tools(ctx);
        if !tools.is_empty() {
            body["tools"] = json!(tools);
        }

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

        // Merge extra fields (provider-specific extensions)
        if let Some(ref extra) = self.config.extra_fields {
            for (key, value) in extra {
                body[key] = value.clone();
            }
        }

        body
    }

    /// Parse usage from OpenAI response.
    fn parse_usage(json: &Value) -> Usage {
        let usage = &json["usage"];
        if usage.is_null() {
            return Usage::default();
        }

        Usage {
            prompt_token_count: usage["prompt_tokens"].as_i64().unwrap_or(0),
            cached_content_token_count: usage["prompt_tokens_details"]["cached_tokens"]
                .as_i64()
                .unwrap_or(0),
            generated_token_count: usage["completion_tokens"].as_i64().unwrap_or(0),
        }
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
            let mut final_usage = Usage::default();

            let process_events = |buffer: &mut String, builder: &StreamBuilder, final_usage: &mut Usage| -> Option<()> {
                // Process complete SSE events (separated by \n\n or standalone \n)
                while let Some(pos) = buffer.find('\n') {
                    let line = buffer[..pos].trim().to_string();
                    buffer.drain(..pos + 1);
                    // Skip empty lines (SSE event separator)
                    if line.is_empty() {
                        continue;
                    }
                    if let Some(data) = line.strip_prefix("data: ").or_else(|| line.strip_prefix("data:")) {
                        let data = data.trim();
                        if data == "[DONE]" {
                            let _ = builder.done(final_usage.clone());
                            return Some(());
                        }
                        if let Ok(json) = serde_json::from_str::<Value>(data) {
                            let usage = Self::parse_usage(&json);
                            if usage.prompt_token_count > 0 || usage.generated_token_count > 0 {
                                *final_usage = usage;
                            }
                            if let Some(choices) = json["choices"].as_array() {
                                for choice in choices {
                                    if let Some(content) = choice["delta"]["content"].as_str() {
                                        let _ = builder.add(&[MessageChunk::text(Role::Model, content)]);
                                    }
                                    if let Some(tool_calls) = choice["delta"]["tool_calls"].as_array() {
                                        for tc in tool_calls {
                                            let index = tc["index"].as_i64().unwrap_or(0);
                                            let id = tc["id"].as_str().unwrap_or("").to_string();
                                            let name = tc["function"]["name"].as_str().unwrap_or("").to_string();
                                            let arguments = tc["function"]["arguments"].as_str().unwrap_or("").to_string();
                                            let _ = builder.add(&[MessageChunk::tool_call(
                                                Role::Model,
                                                ToolCall::with_index(id, index, FuncCall { name, arguments }),
                                            )]);
                                        }
                                    }
                                    if let Some(reason) = choice["finish_reason"].as_str() {
                                        match reason {
                                            "stop" | "tool_calls" => {
                                                let _ = builder.done(final_usage.clone());
                                                return Some(());
                                            }
                                            "length" => {
                                                let _ = builder.truncated(final_usage.clone());
                                                return Some(());
                                            }
                                            "content_filter" => {
                                                let _ = builder.blocked(final_usage.clone(), "content_filter");
                                                return Some(());
                                            }
                                            _ => {}
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
                None
            };

            while let Some(chunk_result) = stream.next().await {
                match chunk_result {
                    Ok(bytes) => {
                        buffer.push_str(&String::from_utf8_lossy(&bytes));
                        if process_events(&mut buffer, &builder_clone, &mut final_usage).is_some() {
                            return;
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

            // Process any remaining buffer after stream ends
            buffer.push('\n');
            process_events(&mut buffer, &builder_clone, &mut final_usage);
            let _ = builder_clone.done(final_usage);
        });

        Ok(Box::new(builder.stream()))
    }

    async fn invoke(
        &self,
        _model: &str,
        ctx: &dyn ModelContext,
        tool: &FuncTool,
    ) -> Result<(Usage, FuncCall), GenxError> {
        // Use tool-calling approach instead of response_format
        // This is the correct way to get structured output from OpenAI
        let messages = self.convert_messages(ctx);
        let params = ctx.params().or(self.config.invoke_params.as_ref());

        let mut body = json!({
            "model": self.config.model,
            "messages": messages,
            "stream": false,
            "tools": [{
                "type": "function",
                "function": {
                    "name": tool.name,
                    "description": tool.description,
                    "parameters": tool.argument,
                    "strict": true,
                }
            }],
            "tool_choice": {
                "type": "function",
                "function": {
                    "name": tool.name
                }
            }
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

        // Merge extra fields (provider-specific extensions)
        if let Some(ref extra) = self.config.extra_fields {
            for (key, value) in extra {
                body[key] = value.clone();
            }
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

        // Parse usage
        let usage = Self::parse_usage(&json);

        // Extract tool call from response
        let tool_calls = &json["choices"][0]["message"]["tool_calls"];
        if let Some(tc) = tool_calls.as_array().and_then(|arr| arr.first()) {
            let arguments = tc["function"]["arguments"]
                .as_str()
                .unwrap_or("{}")
                .to_string();

            Ok((
                usage,
                FuncCall {
                    name: tool.name.clone(),
                    arguments,
                },
            ))
        } else {
            // Fallback: try to extract content if tool_calls is not present
            let content = json["choices"][0]["message"]["content"]
                .as_str()
                .ok_or_else(|| GenxError::Other(anyhow::anyhow!("No tool_calls or content in response")))?;

            Ok((usage, tool.new_func_call(content)))
        }
    }
}
