//! Text generation (chat completion) service.

use std::sync::Arc;

use async_stream::try_stream;
use futures::Stream;
use serde::{Deserialize, Serialize};
use serde_json::Value;

use super::{
    error::Result,
    http::{HttpClient, SseReader},
};

/// Text generation service.
pub struct TextService {
    http: Arc<HttpClient>,
}

impl TextService {
    pub(crate) fn new(http: Arc<HttpClient>) -> Self {
        Self { http }
    }

    /// Creates a chat completion.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// let request = ChatCompletionRequest {
    ///     model: "MiniMax-M2.1".to_string(),
    ///     messages: vec![
    ///         Message::user("Hello, how are you?"),
    ///     ],
    ///     ..Default::default()
    /// };
    ///
    /// let response = client.text().create_chat_completion(&request).await?;
    /// println!("{}", response.choices[0].message.content);
    /// ```
    pub async fn create_chat_completion(
        &self,
        request: &ChatCompletionRequest,
    ) -> Result<ChatCompletionResponse> {
        self.http
            .request("POST", "/v1/chat/completions", Some(request))
            .await
    }

    /// Creates a streaming chat completion.
    ///
    /// Returns a stream that yields chunks.
    ///
    /// # Example
    ///
    /// ```rust,no_run
    /// use futures::StreamExt;
    ///
    /// let mut stream = client.text().create_chat_completion_stream(&request).await?;
    ///
    /// while let Some(chunk) = stream.next().await {
    ///     let chunk = chunk?;
    ///     if let Some(choice) = chunk.choices.first() {
    ///         if let Some(content) = &choice.delta.content {
    ///             print!("{}", content);
    ///         }
    ///     }
    /// }
    /// ```
    pub async fn create_chat_completion_stream(
        &self,
        request: &ChatCompletionRequest,
    ) -> Result<impl Stream<Item = Result<ChatCompletionChunk>>> {
        // Add stream flag to request
        let stream_request = ChatCompletionStreamRequest {
            inner: request.clone(),
            stream: true,
        };

        let byte_stream = self
            .http
            .request_stream("POST", "/v1/chat/completions", Some(stream_request))
            .await?;

        let mut reader = SseReader::new(Box::pin(byte_stream));

        Ok(try_stream! {
            loop {
                match reader.read_event().await? {
                    Some(data) => {
                        let chunk: ChatCompletionChunk = serde_json::from_slice(&data)?;
                        yield chunk;
                    }
                    None => break,
                }
            }
        })
    }
}

// ==================== Request/Response Types ====================

/// Request for chat completion.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ChatCompletionRequest {
    /// Model name.
    pub model: String,

    /// Conversation history.
    pub messages: Vec<Message>,

    /// Maximum output tokens.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_tokens: Option<i32>,

    /// Sampling temperature (0-2).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f64>,

    /// Nucleus sampling parameter.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub top_p: Option<f64>,

    /// List of available tools.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tools: Option<Vec<Tool>>,

    /// Tool selection strategy.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_choice: Option<Value>,
}

/// A chat message.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Message {
    /// Message role: system, user, assistant, tool.
    pub role: String,

    /// Message content (string or content array).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub content: Option<Value>,

    /// Tool calls (for assistant messages).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_calls: Option<Vec<ToolCall>>,

    /// Tool call ID (for tool messages).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tool_call_id: Option<String>,
}

impl Message {
    /// Creates a system message.
    pub fn system(content: impl Into<String>) -> Self {
        Self {
            role: "system".to_string(),
            content: Some(Value::String(content.into())),
            ..Default::default()
        }
    }

    /// Creates a user message.
    pub fn user(content: impl Into<String>) -> Self {
        Self {
            role: "user".to_string(),
            content: Some(Value::String(content.into())),
            ..Default::default()
        }
    }

    /// Creates an assistant message.
    pub fn assistant(content: impl Into<String>) -> Self {
        Self {
            role: "assistant".to_string(),
            content: Some(Value::String(content.into())),
            ..Default::default()
        }
    }

    /// Creates a tool result message.
    pub fn tool(tool_call_id: impl Into<String>, content: impl Into<String>) -> Self {
        Self {
            role: "tool".to_string(),
            content: Some(Value::String(content.into())),
            tool_call_id: Some(tool_call_id.into()),
            ..Default::default()
        }
    }

    /// Returns the content as a string, if it is one.
    pub fn content_str(&self) -> Option<&str> {
        self.content.as_ref().and_then(|v| v.as_str())
    }
}

/// Tool definition.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Tool {
    #[serde(rename = "type")]
    pub tool_type: String,
    pub function: FunctionTool,
}

impl Tool {
    /// Creates a function tool.
    pub fn function(name: impl Into<String>, description: impl Into<String>, parameters: Value) -> Self {
        Self {
            tool_type: "function".to_string(),
            function: FunctionTool {
                name: name.into(),
                description: Some(description.into()),
                parameters: Some(parameters),
            },
        }
    }
}

/// Function tool definition.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct FunctionTool {
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parameters: Option<Value>,
}

/// Tool call.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ToolCall {
    pub id: String,
    #[serde(rename = "type")]
    pub call_type: String,
    pub function: FunctionToolCall,
}

/// Function tool call.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct FunctionToolCall {
    pub name: String,
    pub arguments: String,
}

/// Response from chat completion.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ChatCompletionResponse {
    pub id: String,
    pub object: String,
    pub created: i64,
    pub model: String,
    pub choices: Vec<Choice>,
    pub usage: Option<Usage>,
}

/// A completion choice.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Choice {
    pub index: i32,
    pub message: Message,
    pub finish_reason: Option<String>,
}

/// Token usage information.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Usage {
    pub prompt_tokens: i32,
    pub completion_tokens: i32,
    pub total_tokens: i32,
}

/// A streaming chat chunk.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ChatCompletionChunk {
    pub id: String,
    pub object: String,
    pub created: i64,
    pub model: String,
    pub choices: Vec<ChunkChoice>,
}

/// A streaming choice.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ChunkChoice {
    pub index: i32,
    pub delta: ChunkDelta,
    pub finish_reason: Option<String>,
}

/// Delta content in a streaming chunk.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ChunkDelta {
    pub role: Option<String>,
    pub content: Option<String>,
    pub tool_calls: Option<Vec<ToolCall>>,
}

// ==================== Internal Types ====================

#[derive(Serialize)]
struct ChatCompletionStreamRequest {
    #[serde(flatten)]
    inner: ChatCompletionRequest,
    stream: bool,
}
