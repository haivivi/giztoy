//! Tool definitions for function calling.
//!
//! This module provides types for defining tools that can be called by LLMs.
//! The main type is [`FuncTool`], which uses JSON Schema to define parameters.

use schemars::JsonSchema;
use serde::de::DeserializeOwned;
use serde_json::Value as JsonValue;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use crate::types::{FuncCall, ToolCall};

/// A boxed future that is Send.
pub type BoxFuture<'a, T> = Pin<Box<dyn Future<Output = T> + Send + 'a>>;

/// A tool that can be used by an LLM.
pub trait Tool: Send + Sync {
    /// Get the name of this tool.
    fn name(&self) -> &str;

    /// Get the description of this tool.
    fn description(&self) -> &str;

    /// Get the JSON Schema for the tool's arguments (if applicable).
    /// Returns None for built-in tools like SearchWebTool.
    fn schema(&self) -> Option<&JsonValue> {
        None
    }

    /// Check if this is a function tool.
    fn is_func_tool(&self) -> bool {
        self.schema().is_some()
    }
}

/// A function tool with JSON Schema parameter definition.
pub struct FuncTool {
    /// Name of the tool
    pub name: String,
    /// Description of what the tool does
    pub description: String,
    /// JSON Schema for the argument (stored as JSON value)
    pub argument: JsonValue,
    /// The invoke function (if set)
    invoke_fn: Option<Arc<dyn Fn(String) -> BoxFuture<'static, anyhow::Result<String>> + Send + Sync>>,
}

impl std::fmt::Debug for FuncTool {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("FuncTool")
            .field("name", &self.name)
            .field("description", &self.description)
            .field("argument", &self.argument)
            .field("has_invoke", &self.invoke_fn.is_some())
            .finish()
    }
}

impl Tool for FuncTool {
    fn name(&self) -> &str {
        &self.name
    }

    fn description(&self) -> &str {
        &self.description
    }

    fn schema(&self) -> Option<&JsonValue> {
        Some(&self.argument)
    }
}

impl FuncTool {
    /// Create a new function tool from a type that implements JsonSchema.
    ///
    /// # Example
    ///
    /// ```
    /// use schemars::JsonSchema;
    /// use serde::Deserialize;
    /// use giztoy_genx::tool::FuncTool;
    ///
    /// #[derive(JsonSchema, Deserialize)]
    /// struct SearchArgs {
    ///     /// The search query
    ///     query: String,
    ///     /// Maximum results to return
    ///     limit: Option<u32>,
    /// }
    ///
    /// let tool = FuncTool::new::<SearchArgs>("search", "Search for items");
    /// ```
    pub fn new<T>(name: impl Into<String>, description: impl Into<String>) -> Self
    where
        T: JsonSchema,
    {
        let schema = schemars::schema_for!(T);
        let argument = serde_json::to_value(&schema).unwrap_or_default();

        Self {
            name: name.into(),
            description: description.into(),
            argument,
            invoke_fn: None,
        }
    }

    /// Create a new function tool with a handler function.
    ///
    /// # Example
    ///
    /// ```
    /// use schemars::JsonSchema;
    /// use serde::Deserialize;
    /// use giztoy_genx::tool::FuncTool;
    ///
    /// #[derive(JsonSchema, Deserialize)]
    /// struct SearchArgs {
    ///     query: String,
    /// }
    ///
    /// let tool = FuncTool::with_handler::<SearchArgs, _, _>(
    ///     "search",
    ///     "Search for items",
    ///     |args: SearchArgs| async move {
    ///         Ok(format!("Searched for: {}", args.query))
    ///     },
    /// );
    /// ```
    pub fn with_handler<T, F, Fut>(
        name: impl Into<String>,
        description: impl Into<String>,
        handler: F,
    ) -> Self
    where
        T: JsonSchema + DeserializeOwned + Send + 'static,
        F: Fn(T) -> Fut + Send + Sync + 'static,
        Fut: Future<Output = anyhow::Result<String>> + Send + 'static,
    {
        let handler = Arc::new(handler);
        let schema = schemars::schema_for!(T);
        let argument = serde_json::to_value(&schema).unwrap_or_default();

        Self {
            name: name.into(),
            description: description.into(),
            argument,
            invoke_fn: Some(Arc::new(move |args_json: String| {
                let handler = handler.clone();
                Box::pin(async move {
                    let args: T = serde_json::from_str(&args_json)
                        .map_err(|e| anyhow::anyhow!("Failed to parse arguments: {}", e))?;
                    handler(args).await
                })
            })),
        }
    }

    /// Create a function call for this tool with the given arguments.
    pub fn new_func_call(&self, arguments: impl Into<String>) -> FuncCall {
        FuncCall {
            name: self.name.clone(),
            arguments: arguments.into(),
        }
    }

    /// Create a tool call for this tool with the given ID and arguments.
    pub fn new_tool_call(&self, id: impl Into<String>, arguments: impl Into<String>) -> ToolCall {
        ToolCall::new(id, self.new_func_call(arguments))
    }

    /// Check if this tool has an invoke handler.
    pub fn has_invoke(&self) -> bool {
        self.invoke_fn.is_some()
    }

    /// Invoke this tool with JSON arguments.
    ///
    /// Returns an error if no handler is set or if invocation fails.
    pub async fn invoke(&self, args_json: &str) -> anyhow::Result<String> {
        match &self.invoke_fn {
            Some(f) => f(args_json.to_string()).await,
            None => Err(anyhow::anyhow!("No invoke handler set for tool: {}", self.name)),
        }
    }

    /// Invoke this tool with a FuncCall.
    pub async fn invoke_call(&self, call: &FuncCall) -> anyhow::Result<String> {
        if call.name != self.name {
            return Err(anyhow::anyhow!(
                "Function call name mismatch: expected {}, got {}",
                self.name,
                call.name
            ));
        }
        self.invoke(&call.arguments).await
    }

    /// Get the JSON Schema as a JSON value.
    pub fn schema_json(&self) -> &serde_json::Value {
        &self.argument
    }
}

impl Clone for FuncTool {
    fn clone(&self) -> Self {
        Self {
            name: self.name.clone(),
            description: self.description.clone(),
            argument: self.argument.clone(),
            invoke_fn: self.invoke_fn.clone(),
        }
    }
}

/// A search web tool (built-in tool type).
#[derive(Debug, Clone, Default)]
pub struct SearchWebTool;

impl Tool for SearchWebTool {
    fn name(&self) -> &str {
        "search_web"
    }

    fn description(&self) -> &str {
        "Search the web for information"
    }
}

/// Enum wrapper for different tool types.
#[derive(Debug)]
pub enum AnyTool {
    /// A function tool
    Func(FuncTool),
    /// A web search tool
    SearchWeb(SearchWebTool),
}

impl Tool for AnyTool {
    fn name(&self) -> &str {
        match self {
            AnyTool::Func(t) => t.name(),
            AnyTool::SearchWeb(t) => t.name(),
        }
    }

    fn description(&self) -> &str {
        match self {
            AnyTool::Func(t) => t.description(),
            AnyTool::SearchWeb(t) => t.description(),
        }
    }

    fn schema(&self) -> Option<&JsonValue> {
        match self {
            AnyTool::Func(t) => t.schema(),
            AnyTool::SearchWeb(t) => t.schema(),
        }
    }
}

impl From<FuncTool> for AnyTool {
    fn from(t: FuncTool) -> Self {
        AnyTool::Func(t)
    }
}

impl From<SearchWebTool> for AnyTool {
    fn from(t: SearchWebTool) -> Self {
        AnyTool::SearchWeb(t)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde::Deserialize;

    #[derive(Debug, JsonSchema, Deserialize)]
    struct TestArgs {
        /// The name parameter
        name: String,
        /// Optional count
        count: Option<i32>,
    }

    #[test]
    fn test_func_tool_new() {
        let tool = FuncTool::new::<TestArgs>("test_tool", "A test tool");

        assert_eq!(tool.name(), "test_tool");
        assert_eq!(tool.description(), "A test tool");
        assert!(!tool.has_invoke());

        let schema = tool.schema_json();
        assert!(schema.get("properties").is_some());
    }

    #[test]
    fn test_func_tool_schema_generation() {
        let tool = FuncTool::new::<TestArgs>("test", "Test");

        let schema = tool.schema_json();
        let props = schema.get("properties").unwrap();

        // Check that "name" is required and "count" is optional
        assert!(props.get("name").is_some());
        assert!(props.get("count").is_some());

        let required = schema.get("required").unwrap().as_array().unwrap();
        assert!(required.iter().any(|v| v.as_str() == Some("name")));
    }

    #[tokio::test]
    async fn test_func_tool_with_handler() {
        let tool = FuncTool::with_handler::<TestArgs, _, _>("test", "Test", |args: TestArgs| async move {
            Ok(format!("Hello, {}!", args.name))
        });

        assert!(tool.has_invoke());

        let result = tool.invoke(r#"{"name": "World"}"#).await.unwrap();
        assert_eq!(result, "Hello, World!");
    }

    #[tokio::test]
    async fn test_func_tool_invoke_without_handler() {
        let tool = FuncTool::new::<TestArgs>("test", "Test");

        let result = tool.invoke(r#"{"name": "World"}"#).await;
        assert!(result.is_err());
    }

    #[test]
    fn test_new_tool_call() {
        let tool = FuncTool::new::<TestArgs>("test", "Test");
        let call = tool.new_tool_call("call_123", r#"{"name": "test"}"#);

        assert_eq!(call.id, "call_123");
        assert_eq!(call.func_call.name, "test");
    }
}
