//! GenX - A universal interface for Large Language Models.
//!
//! This crate provides a common abstraction layer for interacting with various LLM providers
//! (OpenAI, Google Gemini, etc.) with support for:
//!
//! - Streaming responses
//! - Function/tool calling with JSON Schema
//! - Multi-modal content (text, images, audio)
//! - Chain-of-thought prompting
//!
//! # Quick Start
//!
//! ```rust,ignore
//! use giztoy_genx::{Generator, ModelContextBuilder, FuncTool};
//! use schemars::JsonSchema;
//! use serde::Deserialize;
//!
//! #[derive(JsonSchema, Deserialize)]
//! struct SearchArgs {
//!     query: String,
//! }
//!
//! // Build context
//! let mut builder = ModelContextBuilder::new();
//! builder.prompt_text("system", "You are a helpful assistant.");
//! builder.user_text("user", "Search for Rust tutorials");
//! builder.add_tool(FuncTool::new::<SearchArgs>("search", "Search the web"));
//!
//! let ctx = builder.build();
//!
//! // Use with a generator (e.g., OpenAI)
//! // let stream = generator.generate_stream(&ctx).await?;
//! ```
//!
//! # Modules
//!
//! - [`types`]: Core message and content types
//! - [`tool`]: Function tool definitions with JSON Schema
//! - [`context`]: Model context building and management
//! - [`error`]: Error types and stream status
//! - [`stream`]: Streaming response handling

pub mod context;
pub mod error;
pub mod gemini;
pub mod generators;
pub mod json_utils;
pub mod r#match;
pub mod modelcontexts;
pub mod openai;
pub mod profilers;
pub mod segmentors;
pub mod stream;
pub mod stream_id;
pub mod stream_utils;
pub mod tee;
pub mod tool;
pub mod transformer;
pub mod types;

// Re-exports for convenience
pub use context::{ModelContext, ModelContextBuilder, ModelParams, MultiModelContext, Prompt};
pub use error::{GenxError, State, Status, Usage};
pub use stream::{
    collect_text, collect_tool_calls, collect_tool_calls_streamed, Stream, StreamBuilder,
    StreamEvent, StreamImpl, StreamResult,
};
pub use stream_id::new_stream_id;
pub use tool::{AnyTool, BoxFuture, FuncTool, SearchWebTool, Tool};
pub use transformer::Transformer;
pub use types::{
    Blob, Contents, FuncCall, Message, MessageChunk, Part, Payload, Role, StreamCtrl, ToolCall,
    ToolResult,
};

use async_trait::async_trait;

/// Trait for LLM generators.
///
/// Implementations of this trait provide the ability to generate responses
/// from language models, either as a stream or as a single invocation.
#[async_trait]
pub trait Generator: Send + Sync {
    /// Generate a streaming response.
    ///
    /// # Arguments
    ///
    /// * `model` - The model identifier (may be ignored if generator has a default)
    /// * `ctx` - The model context containing prompts, messages, and tools
    ///
    /// # Returns
    ///
    /// A stream that yields message chunks as they are generated.
    async fn generate_stream(
        &self,
        model: &str,
        ctx: &dyn ModelContext,
    ) -> Result<Box<dyn Stream>, GenxError>;

    /// Invoke a function tool and get the result.
    ///
    /// This is useful for structured output where you want the model to
    /// respond with a specific JSON schema.
    ///
    /// # Arguments
    ///
    /// * `model` - The model identifier
    /// * `ctx` - The model context
    /// * `tool` - The function tool to invoke
    ///
    /// # Returns
    ///
    /// The usage statistics and the function call with parsed arguments.
    async fn invoke(
        &self,
        model: &str,
        ctx: &dyn ModelContext,
        tool: &FuncTool,
    ) -> Result<(Usage, FuncCall), GenxError>;
}

/// Inspect a tool for debugging.
pub fn inspect_tool(tool: &dyn Tool) -> String {
    format!("### {}\n{}", tool.name(), tool.description())
}

/// Inspect a message for debugging.
pub fn inspect_message(msg: &Message) -> String {
    let mut lines = vec![format!("### {}", msg.role)];

    if let Some(name) = &msg.name {
        lines.push(name.clone());
    }

    match &msg.payload {
        Payload::Contents(contents) => {
            for part in contents {
                match part {
                    Part::Text(t) => lines.push(t.clone()),
                    Part::Blob(b) => {
                        lines.push(b.mime_type.clone());
                        lines.push(format!("[{} bytes]", b.data.len()));
                    }
                }
            }
        }
        Payload::ToolCall(tc) => {
            lines.push(format!("[{}]", tc.id));
            lines.push(format!("{}({})", tc.func_call.name, tc.func_call.arguments));
        }
        Payload::ToolResult(tr) => {
            lines.push(format!("[{}]", tr.id));
            lines.push(tr.result.clone());
        }
    }

    lines.join("\n")
}

/// Inspect model context for debugging.
pub fn inspect_model_context(ctx: &dyn ModelContext) -> String {
    let mut output = String::new();

    // Params
    output.push_str("## Params\n");
    if let Some(params) = ctx.params() {
        if let Some(max_tokens) = params.max_tokens {
            output.push_str(&format!("MaxTokens: {}\n", max_tokens));
        }
        if let Some(temp) = params.temperature {
            output.push_str(&format!("Temperature: {:.2}\n", temp));
        }
        if let Some(top_p) = params.top_p {
            output.push_str(&format!("TopP: {:.2}\n", top_p));
        }
        if let Some(top_k) = params.top_k {
            output.push_str(&format!("TopK: {:.2}\n", top_k));
        }
    }
    output.push('\n');

    // Prompts
    output.push_str("## Prompts\n");
    for prompt in ctx.prompts() {
        output.push_str(&format!("### {}\n{}\n\n", prompt.name, prompt.text));
    }

    // Tools
    output.push_str("## Tools\n");
    for tool in ctx.tools() {
        output.push_str(&inspect_tool(tool));
        output.push_str("\n\n");
    }

    // Messages
    output.push_str("## Messages\n");
    for msg in ctx.messages() {
        output.push_str(&inspect_message(msg));
        output.push_str("\n\n");
    }

    // CoTs
    output.push_str("## CoTs\n");
    for cot in ctx.cots() {
        output.push_str(cot);
        output.push('\n');
    }

    output
}

#[cfg(test)]
mod tests {
    use super::*;
    use schemars::JsonSchema;
    use serde::Deserialize;

    #[derive(Debug, JsonSchema, Deserialize)]
    struct TestArgs {
        query: String,
    }

    #[test]
    fn test_inspect_tool() {
        let tool = FuncTool::new::<TestArgs>("search", "Search for items");
        let output = inspect_tool(&tool);

        assert!(output.contains("search"));
        assert!(output.contains("Search for items"));
    }

    #[test]
    fn test_inspect_message() {
        let msg = Message::user_text("Hello, world!");
        let output = inspect_message(&msg);

        assert!(output.contains("user"));
        assert!(output.contains("Hello, world!"));
    }

    #[test]
    fn test_inspect_model_context() {
        let mut builder = ModelContextBuilder::new();
        builder.prompt_text("system", "You are helpful.");
        builder.user_text("user", "Hello");
        builder.add_tool(FuncTool::new::<TestArgs>("search", "Search"));

        let ctx = builder.build();
        let output = inspect_model_context(&ctx);

        assert!(output.contains("## Prompts"));
        assert!(output.contains("## Tools"));
        assert!(output.contains("## Messages"));
        assert!(output.contains("system"));
        assert!(output.contains("search"));
    }

    #[test]
    fn test_full_workflow() {
        // Test the full workflow without actual API calls
        let mut builder = ModelContextBuilder::new();

        // Add system prompt
        builder.prompt_text("system", "You are a helpful assistant.");

        // Add user message
        builder.user_text("user", "What's the weather?");

        // Add a tool
        let tool = FuncTool::new::<TestArgs>("search", "Search the web");
        builder.add_tool(tool);

        // Build context
        let ctx = builder.build();

        // Verify context
        assert_eq!(ctx.prompts().count(), 1);
        assert_eq!(ctx.messages().count(), 1);
        assert_eq!(ctx.tools().count(), 1);
    }
}
