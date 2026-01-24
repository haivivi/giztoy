# GenX - Rust Implementation

Crate: `giztoy-genx`

📚 [Rust Documentation](https://docs.rs/giztoy-genx)

## Status

The Rust implementation provides core abstractions but lacks the full agent framework available in Go.

| Feature | Go | Rust |
|---------|:--:|:----:|
| ModelContext | ✅ | ✅ |
| Generator trait | ✅ | ✅ |
| Streaming | ✅ | ✅ |
| FuncTool | ✅ | ✅ |
| OpenAI adapter | ✅ | ✅ |
| Gemini adapter | ✅ | ✅ |
| Agent framework | ✅ | ❌ |
| Configuration parser | ✅ | ❌ |
| Match patterns | ✅ | ❌ |

## Core Types

### Generator Trait

```rust
#[async_trait]
pub trait Generator: Send + Sync {
    async fn generate_stream(
        &self,
        model: &str,
        ctx: &dyn ModelContext,
    ) -> Result<Box<dyn Stream>, GenxError>;

    async fn invoke(
        &self,
        model: &str,
        ctx: &dyn ModelContext,
        tool: &FuncTool,
    ) -> Result<(Usage, FuncCall), GenxError>;
}
```

### ModelContext Trait

```rust
pub trait ModelContext: Send + Sync {
    fn prompts(&self) -> Box<dyn Iterator<Item = &Prompt> + '_>;
    fn messages(&self) -> Box<dyn Iterator<Item = &Message> + '_>;
    fn cots(&self) -> Box<dyn Iterator<Item = &str> + '_>;
    fn tools(&self) -> Box<dyn Iterator<Item = &dyn Tool> + '_>;
    fn params(&self) -> Option<&ModelParams>;
}
```

### Stream Trait

```rust
pub trait Stream: Send {
    fn next(&mut self) -> StreamResult;
    fn close(&mut self) -> Result<(), GenxError>;
    fn close_with_error(&mut self, err: GenxError) -> Result<(), GenxError>;
}
```

## ModelContextBuilder

```rust
use giztoy_genx::{ModelContextBuilder, FuncTool};
use schemars::JsonSchema;

#[derive(JsonSchema, serde::Deserialize)]
struct SearchArgs {
    query: String,
}

let mut builder = ModelContextBuilder::new();

// Add prompts
builder.prompt_text("system", "You are a helpful assistant.");

// Add messages
builder.user_text("user", "Hello!");
builder.assistant_text("assistant", "Hi there!");

// Add tools
builder.add_tool(FuncTool::new::<SearchArgs>("search", "Search the web"));

// Set parameters
builder.params(ModelParams {
    temperature: Some(0.7),
    max_tokens: Some(1000),
    ..Default::default()
});

let ctx = builder.build();
```

## FuncTool

```rust
use giztoy_genx::FuncTool;
use schemars::JsonSchema;
use serde::Deserialize;

#[derive(JsonSchema, Deserialize)]
struct WeatherArgs {
    city: String,
    #[serde(default)]
    units: Option<String>,
}

// Create tool with schema derived from type
let tool = FuncTool::new::<WeatherArgs>(
    "get_weather",
    "Get weather for a city"
);

// Access schema
println!("{}", tool.schema());
```

## Streaming

```rust
let mut stream = generator.generate_stream("gpt-4", &ctx).await?;

loop {
    match stream.next() {
        StreamResult::Chunk(chunk) => {
            if let Some(text) = chunk.text() {
                print!("{}", text);
            }
        }
        StreamResult::Done => break,
        StreamResult::Error(e) => return Err(e),
    }
}
```

## Message Types

```rust
use giztoy_genx::{Message, Contents, Part, Role};

// User text message
let msg = Message::user_text("Hello!");

// Assistant message with content
let msg = Message {
    role: Role::Assistant,
    name: None,
    payload: Payload::Contents(vec![
        Part::Text("Here's what I found:".to_string()),
    ]),
};

// Tool call
let msg = Message::tool_call(ToolCall {
    id: "call_123".to_string(),
    func_call: FuncCall {
        name: "search".to_string(),
        arguments: r#"{"query":"rust"}"#.to_string(),
    },
});

// Tool result
let msg = Message::tool_result(ToolResult {
    id: "call_123".to_string(),
    result: "Found 10 results".to_string(),
});
```

## Provider Adapters

### OpenAI

```rust
use giztoy_genx::openai::OpenAIGenerator;

let generator = OpenAIGenerator::new(api_key)
    .with_base_url("https://api.openai.com/v1");
```

### Gemini

```rust
use giztoy_genx::gemini::GeminiGenerator;

let generator = GeminiGenerator::new(api_key);
```

## Inspection

```rust
use giztoy_genx::{inspect_model_context, inspect_message, inspect_tool};

// Inspect context
println!("{}", inspect_model_context(&ctx));

// Inspect message
println!("{}", inspect_message(&msg));

// Inspect tool
println!("{}", inspect_tool(&tool));
```

## Error Types

```rust
use giztoy_genx::{GenxError, State, Status};

match result {
    Err(GenxError::Api { status, message }) => {
        eprintln!("API error: {} - {}", status, message);
    }
    Err(GenxError::Network(e)) => {
        eprintln!("Network error: {}", e);
    }
    Err(GenxError::Json(e)) => {
        eprintln!("JSON error: {}", e);
    }
    _ => {}
}
```

## Missing Features (vs Go)

The Rust implementation is missing:

1. **Agent Framework**: No ReActAgent, MatchAgent
2. **Configuration Parser**: No YAML/JSON config loading
3. **Match Patterns**: No intent matching system
4. **Tool Variants**: No GeneratorTool, HTTPTool, CompositeTool
5. **Runtime Interface**: No dependency injection system
6. **State Management**: No memory/state persistence

These would need to be implemented to reach feature parity with Go.
