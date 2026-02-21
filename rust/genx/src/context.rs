//! Model context for LLM conversations.
//!
//! This module provides types for building and managing conversation context
//! that can be passed to LLM generators.

use serde::{Deserialize, Serialize};

use crate::tool::{AnyTool, FuncTool, Tool};
use crate::types::{FuncCall, Message, Part, Payload, Role, ToolCall, ToolResult};

/// A prompt with a name and text content.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Prompt {
    /// Name/identifier for this prompt
    pub name: String,
    /// Text content of the prompt
    pub text: String,
}

impl Prompt {
    /// Create a new prompt.
    pub fn new(name: impl Into<String>, text: impl Into<String>) -> Self {
        Self {
            name: name.into(),
            text: text.into(),
        }
    }
}

/// Parameters for model generation.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct ModelParams {
    /// Maximum number of tokens to generate
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_tokens: Option<i32>,

    /// Frequency penalty (0.0 to 2.0)
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub frequency_penalty: Option<f32>,

    /// Number of completions to generate
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub n: Option<i32>,

    /// Temperature (0.0 to 2.0)
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f32>,

    /// Top-p sampling (0.0 to 1.0)
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub top_p: Option<f32>,

    /// Presence penalty (0.0 to 2.0)
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub presence_penalty: Option<f32>,

    /// Top-k sampling
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub top_k: Option<f32>,
}

impl ModelParams {
    /// Create new default parameters.
    pub fn new() -> Self {
        Self::default()
    }

    /// Set max tokens.
    pub fn with_max_tokens(mut self, max_tokens: i32) -> Self {
        self.max_tokens = Some(max_tokens);
        self
    }

    /// Set temperature.
    pub fn with_temperature(mut self, temperature: f32) -> Self {
        self.temperature = Some(temperature);
        self
    }

    /// Set top-p.
    pub fn with_top_p(mut self, top_p: f32) -> Self {
        self.top_p = Some(top_p);
        self
    }

    /// Set top-k.
    pub fn with_top_k(mut self, top_k: f32) -> Self {
        self.top_k = Some(top_k);
        self
    }
}

/// Trait for accessing model context.
pub trait ModelContext: Send + Sync {
    /// Iterate over prompts.
    fn prompts(&self) -> Box<dyn Iterator<Item = &Prompt> + '_>;

    /// Iterate over messages.
    fn messages(&self) -> Box<dyn Iterator<Item = &Message> + '_>;

    /// Iterate over chain-of-thought entries.
    fn cots(&self) -> Box<dyn Iterator<Item = &str> + '_>;

    /// Iterate over tools.
    fn tools(&self) -> Box<dyn Iterator<Item = &dyn Tool> + '_>;

    /// Get model parameters.
    fn params(&self) -> Option<&ModelParams>;
}

/// Builder for constructing model context.
#[derive(Debug, Default)]
pub struct ModelContextBuilder {
    prompts: Vec<Prompt>,
    messages: Vec<Message>,
    cots: Vec<String>,
    tools: Vec<AnyTool>,
    params: Option<ModelParams>,
}

impl ModelContextBuilder {
    /// Create a new builder.
    pub fn new() -> Self {
        Self::default()
    }

    /// Build the model context.
    pub fn build(self) -> impl ModelContext {
        BuiltModelContext {
            prompts: self.prompts,
            messages: self.messages,
            cots: self.cots,
            tools: self.tools,
            params: self.params,
        }
    }

    /// Set model parameters.
    pub fn set_params(&mut self, params: ModelParams) -> &mut Self {
        self.params = Some(params);
        self
    }

    /// Set chain-of-thought entries.
    pub fn set_cot(&mut self, cots: Vec<String>) -> &mut Self {
        self.cots = cots;
        self
    }

    /// Add a prompt.
    pub fn add_prompt(&mut self, prompt: Prompt) -> &mut Self {
        // Merge with last prompt if same name
        if let Some(last) = self.prompts.last_mut()
            && last.name == prompt.name {
                if !last.text.is_empty() {
                    last.text.push('\n');
                }
                last.text.push_str(&prompt.text);
                return self;
            }
        self.prompts.push(prompt);
        self
    }

    /// Add a prompt with name and text.
    pub fn prompt_text(&mut self, name: impl Into<String>, text: impl Into<String>) -> &mut Self {
        self.add_prompt(Prompt::new(name, text))
    }

    /// Add a message.
    pub fn add_message(&mut self, msg: Message) -> &mut Self {
        // Try to merge with last message if same role/name and both are contents
        if let Some(last) = self.messages.last_mut()
            && let (Payload::Contents(last_contents), Payload::Contents(new_contents)) =
                (&mut last.payload, &msg.payload)
                && last.role == msg.role && last.name == msg.name {
                    last_contents.extend(new_contents.clone());
                    return self;
                }
        self.messages.push(msg);
        self
    }

    /// Add a user text message.
    pub fn user_text(&mut self, name: impl Into<String>, text: impl Into<String>) -> &mut Self {
        self.add_message(Message::with_name(Role::User, name, Payload::text(text)))
    }

    /// Add a user blob message.
    pub fn user_blob(
        &mut self,
        name: impl Into<String>,
        mime_type: impl Into<String>,
        data: impl Into<Vec<u8>>,
    ) -> &mut Self {
        self.add_message(Message::with_name(
            Role::User,
            name,
            Payload::Contents(vec![Part::blob(mime_type, data)]),
        ))
    }

    /// Add a model text message.
    pub fn model_text(&mut self, name: impl Into<String>, text: impl Into<String>) -> &mut Self {
        self.add_message(Message::with_name(Role::Model, name, Payload::text(text)))
    }

    /// Add a model blob message.
    pub fn model_blob(
        &mut self,
        name: impl Into<String>,
        mime_type: impl Into<String>,
        data: impl Into<Vec<u8>>,
    ) -> &mut Self {
        self.add_message(Message::with_name(
            Role::Model,
            name,
            Payload::Contents(vec![Part::blob(mime_type, data)]),
        ))
    }

    /// Add a tool call message.
    pub fn tool_call(
        &mut self,
        name: impl Into<String>,
        id: impl Into<String>,
        fn_name: impl Into<String>,
        arguments: impl Into<String>,
    ) -> &mut Self {
        self.messages.push(Message::with_name(
            Role::Model,
            name,
            Payload::ToolCall(ToolCall::new(
                id,
                FuncCall::new(fn_name, arguments),
            )),
        ));
        self
    }

    /// Add a tool result message.
    pub fn tool_result(
        &mut self,
        name: impl Into<String>,
        id: impl Into<String>,
        result: impl Into<String>,
    ) -> &mut Self {
        self.messages.push(Message::with_name(
            Role::Tool,
            name,
            Payload::ToolResult(ToolResult::new(id, result)),
        ));
        self
    }

    /// Add a tool call and its result.
    pub fn add_tool_call_result(
        &mut self,
        tool_name: impl Into<String>,
        arguments: impl Into<String>,
        result: impl Into<String>,
    ) -> &mut Self {
        let id = format!("call_{}", hex_string());
        let tool_name = tool_name.into();
        self.tool_call("", &id, &tool_name, arguments);
        self.tool_result("", &id, result);
        self
    }

    /// Add a function tool.
    pub fn add_tool(&mut self, tool: FuncTool) -> &mut Self {
        self.tools.push(AnyTool::Func(tool));
        self
    }

    /// Add any tool.
    pub fn add_any_tool(&mut self, tool: impl Into<AnyTool>) -> &mut Self {
        self.tools.push(tool.into());
        self
    }

    /// Get a reference to the tools.
    pub fn get_tools(&self) -> &[AnyTool] {
        &self.tools
    }

    /// Find a function tool by name.
    pub fn find_func_tool(&self, name: &str) -> Option<&FuncTool> {
        self.tools.iter().find_map(|t| match t {
            AnyTool::Func(f) if f.name == name => Some(f),
            _ => None,
        })
    }
}

/// Built model context.
struct BuiltModelContext {
    prompts: Vec<Prompt>,
    messages: Vec<Message>,
    cots: Vec<String>,
    tools: Vec<AnyTool>,
    params: Option<ModelParams>,
}

impl ModelContext for BuiltModelContext {
    fn prompts(&self) -> Box<dyn Iterator<Item = &Prompt> + '_> {
        Box::new(self.prompts.iter())
    }

    fn messages(&self) -> Box<dyn Iterator<Item = &Message> + '_> {
        Box::new(self.messages.iter())
    }

    fn cots(&self) -> Box<dyn Iterator<Item = &str> + '_> {
        Box::new(self.cots.iter().map(|s| s.as_str()))
    }

    fn tools(&self) -> Box<dyn Iterator<Item = &dyn Tool> + '_> {
        Box::new(self.tools.iter().map(|t| t as &dyn Tool))
    }

    fn params(&self) -> Option<&ModelParams> {
        self.params.as_ref()
    }
}

/// Multiple model contexts combined.
pub struct MultiModelContext {
    contexts: Vec<Box<dyn ModelContext>>,
}

impl MultiModelContext {
    /// Create from multiple contexts.
    pub fn new(contexts: Vec<Box<dyn ModelContext>>) -> Self {
        Self { contexts }
    }
}

impl ModelContext for MultiModelContext {
    fn prompts(&self) -> Box<dyn Iterator<Item = &Prompt> + '_> {
        Box::new(self.contexts.iter().flat_map(|c| c.prompts()))
    }

    fn messages(&self) -> Box<dyn Iterator<Item = &Message> + '_> {
        Box::new(self.contexts.iter().flat_map(|c| c.messages()))
    }

    fn cots(&self) -> Box<dyn Iterator<Item = &str> + '_> {
        Box::new(self.contexts.iter().flat_map(|c| c.cots()))
    }

    fn tools(&self) -> Box<dyn Iterator<Item = &dyn Tool> + '_> {
        Box::new(self.contexts.iter().flat_map(|c| c.tools()))
    }

    fn params(&self) -> Option<&ModelParams> {
        self.contexts.iter().find_map(|c| c.params())
    }
}

/// Generate a random hex string for IDs.
fn hex_string() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    format!("{:016x}", now)
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
    fn test_prompt_new() {
        let p = Prompt::new("system", "You are a helpful assistant.");
        assert_eq!(p.name, "system");
        assert_eq!(p.text, "You are a helpful assistant.");
    }

    #[test]
    fn test_model_params_builder() {
        let params = ModelParams::new()
            .with_max_tokens(1000)
            .with_temperature(0.7)
            .with_top_p(0.9);

        assert_eq!(params.max_tokens, Some(1000));
        assert_eq!(params.temperature, Some(0.7));
        assert_eq!(params.top_p, Some(0.9));
    }

    #[test]
    fn test_builder_prompt_merge() {
        let mut builder = ModelContextBuilder::new();
        builder.prompt_text("system", "Line 1");
        builder.prompt_text("system", "Line 2");
        builder.prompt_text("other", "Other");

        let ctx = builder.build();
        let prompts: Vec<_> = ctx.prompts().collect();

        assert_eq!(prompts.len(), 2);
        assert_eq!(prompts[0].text, "Line 1\nLine 2");
        assert_eq!(prompts[1].name, "other");
    }

    #[test]
    fn test_builder_message_merge() {
        let mut builder = ModelContextBuilder::new();
        builder.user_text("user1", "Hello");
        builder.user_text("user1", "World");
        builder.model_text("assistant", "Hi!");

        let ctx = builder.build();
        let messages: Vec<_> = ctx.messages().collect();

        assert_eq!(messages.len(), 2);
        // First message should have merged contents
        if let Payload::Contents(parts) = &messages[0].payload {
            assert_eq!(parts.len(), 2);
        } else {
            panic!("Expected Contents payload");
        }
    }

    #[test]
    fn test_builder_tools() {
        let mut builder = ModelContextBuilder::new();
        builder.add_tool(FuncTool::new::<TestArgs>("search", "Search for things"));

        let ctx = builder.build();
        let tools: Vec<_> = ctx.tools().collect();

        assert_eq!(tools.len(), 1);
        assert_eq!(tools[0].name(), "search");
    }

    #[test]
    fn test_tool_call_result() {
        let mut builder = ModelContextBuilder::new();
        builder.add_tool_call_result("search", r#"{"query":"test"}"#, "result");

        let ctx = builder.build();
        let messages: Vec<_> = ctx.messages().collect();

        assert_eq!(messages.len(), 2);
        assert!(messages[0].payload.is_tool_call());
        assert!(messages[1].payload.is_tool_result());
    }

    #[test]
    fn t_ctx_add_prompt_empty_initial() {
        let mut b = ModelContextBuilder::new();
        b.prompt_text("system", "");
        b.prompt_text("system", "Hello");
        let ctx = b.build();
        let prompts: Vec<_> = ctx.prompts().collect();
        assert_eq!(prompts.len(), 1);
        // Empty initial + merge = just "Hello" (no leading newline since initial is empty)
        assert_eq!(prompts[0].text, "Hello");
    }

    #[test]
    fn t_ctx_add_message_different_role() {
        let mut b = ModelContextBuilder::new();
        b.user_text("u", "Hello");
        b.model_text("m", "Hi");
        let ctx = b.build();
        assert_eq!(ctx.messages().count(), 2);
    }

    #[test]
    fn t_ctx_add_message_non_contents() {
        let mut b = ModelContextBuilder::new();
        b.user_text("u", "Hello");
        b.tool_call("m", "call_1", "search", "{}");
        let ctx = b.build();
        let msgs: Vec<_> = ctx.messages().collect();
        assert_eq!(msgs.len(), 2);
        assert!(msgs[1].payload.is_tool_call());
    }

    #[test]
    fn t_ctx_set_cot() {
        let mut b = ModelContextBuilder::new();
        b.set_cot(vec!["think step 1".into(), "think step 2".into()]);
        let ctx = b.build();
        let cots: Vec<_> = ctx.cots().collect();
        assert_eq!(cots.len(), 2);
        assert_eq!(cots[0], "think step 1");
    }

    #[test]
    fn t_ctx_user_blob() {
        let mut b = ModelContextBuilder::new();
        b.user_blob("u", "image/png", vec![1, 2, 3]);
        let ctx = b.build();
        let msgs: Vec<_> = ctx.messages().collect();
        assert_eq!(msgs.len(), 1);
        if let Payload::Contents(parts) = &msgs[0].payload {
            assert!(parts[0].is_blob());
        } else {
            panic!("expected contents");
        }
    }

    #[test]
    fn t_ctx_model_text() {
        let mut b = ModelContextBuilder::new();
        b.model_text("assistant", "I can help");
        let ctx = b.build();
        let msgs: Vec<_> = ctx.messages().collect();
        assert_eq!(msgs[0].role, Role::Model);
    }

    #[test]
    fn t_ctx_model_blob() {
        let mut b = ModelContextBuilder::new();
        b.model_blob("assistant", "audio/mp3", vec![4, 5, 6]);
        let ctx = b.build();
        let msgs: Vec<_> = ctx.messages().collect();
        assert_eq!(msgs[0].role, Role::Model);
    }

    #[test]
    fn t_ctx_empty_builder() {
        let b = ModelContextBuilder::new();
        let ctx = b.build();
        assert_eq!(ctx.prompts().count(), 0);
        assert_eq!(ctx.messages().count(), 0);
        assert_eq!(ctx.tools().count(), 0);
        assert_eq!(ctx.cots().count(), 0);
        assert!(ctx.params().is_none());
    }

    #[test]
    fn t_ctx_params() {
        let mut b = ModelContextBuilder::new();
        b.set_params(ModelParams::new().with_max_tokens(1000).with_temperature(0.5));
        let ctx = b.build();
        let params = ctx.params().unwrap();
        assert_eq!(params.max_tokens, Some(1000));
        assert_eq!(params.temperature, Some(0.5));
    }

    #[test]
    fn t_ctx_find_func_tool() {
        let mut b = ModelContextBuilder::new();
        b.add_tool(FuncTool::new::<TestArgs>("search", "Search"));
        assert!(b.find_func_tool("search").is_some());
        assert!(b.find_func_tool("missing").is_none());
    }
}
