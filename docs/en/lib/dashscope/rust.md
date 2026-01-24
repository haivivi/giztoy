# DashScope SDK - Rust Implementation

Crate: `giztoy-dashscope`

📚 [Rust Documentation](https://docs.rs/giztoy-dashscope)

## Client

```rust
pub struct Client {
    // Internal configuration
}

impl Client {
    pub fn realtime(&self) -> RealtimeService;
}
```

**Constructor:**

```rust
use giztoy_dashscope::{Client, DEFAULT_REALTIME_URL};

// Basic
let client = Client::new("sk-xxxxxxxx")?;

// With builder
let client = Client::builder("sk-xxxxxxxx")
    .workspace("ws-xxxxxxxx")
    .base_url("wss://dashscope-intl.aliyuncs.com/api-ws/v1/realtime")
    .build()?;
```

## RealtimeService

### Connect Session

```rust
use giztoy_dashscope::{RealtimeConfig, ModelQwenOmniTurboRealtimeLatest};

let session = client.realtime().connect(&RealtimeConfig {
    model: ModelQwenOmniTurboRealtimeLatest.to_string(),
    ..Default::default()
}).await?;
```

### Send Events

```rust
// Update session
session.update_session(&SessionUpdate {
    modalities: vec!["text".to_string(), "audio".to_string()],
    voice: Some("Cherry".to_string()),
    ..Default::default()
}).await?;

// Append audio
session.append_audio(&audio_data).await?;

// Commit audio
session.commit_audio().await?;

// Create response
session.create_response().await?;
```

### Receive Events

```rust
use giztoy_dashscope::ServerEvent;

while let Some(event) = session.recv().await {
    let event = event?;
    
    match event {
        ServerEvent::ResponseAudioDelta { delta, .. } => {
            // Play audio
            player.write(&delta)?;
        }
        ServerEvent::ResponseTextDelta { delta, .. } => {
            // Print text
            print!("{}", delta);
        }
        ServerEvent::ResponseDone { .. } => {
            // Complete
            break;
        }
        ServerEvent::Error { error } => {
            eprintln!("Error: {}", error.message);
        }
        _ => {}
    }
}
```

## Events

### Client Events (Send)

```rust
pub enum ClientEvent {
    SessionUpdate(SessionUpdate),
    InputAudioBufferAppend { audio: Vec<u8> },
    InputAudioBufferCommit,
    ResponseCreate(ResponseCreateOptions),
    ResponseCancel,
}
```

### Server Events (Receive)

```rust
pub enum ServerEvent {
    SessionCreated { session: SessionInfo },
    SessionUpdated { session: SessionInfo },
    ResponseCreated { response: ResponseInfo },
    ResponseAudioDelta { delta: Vec<u8> },
    ResponseTextDelta { delta: String },
    ResponseDone { response: ResponseInfo },
    Error { error: ErrorInfo },
    // ... more events
}
```

## Models

```rust
pub const MODEL_QWEN_OMNI_TURBO_REALTIME_LATEST: &str = "qwen-omni-turbo-realtime-latest";
pub const MODEL_QWEN3_OMNI_FLASH_REALTIME_LATEST: &str = "qwen3-omni-flash-realtime-latest";
```

## Error Handling

```rust
use giztoy_dashscope::{Error, Result};

match session.recv().await {
    Some(Ok(event)) => {
        // Process event
    }
    Some(Err(Error::WebSocket(e))) => {
        eprintln!("WebSocket error: {}", e);
    }
    Some(Err(Error::Api { code, message })) => {
        eprintln!("API error [{}]: {}", code, message);
    }
    None => {
        // Connection closed
    }
}
```

## Complete Example

```rust
use giztoy_dashscope::{Client, RealtimeConfig, ServerEvent};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let api_key = std::env::var("DASHSCOPE_API_KEY")?;
    let client = Client::new(&api_key)?;
    
    let session = client.realtime().connect(&RealtimeConfig {
        model: "qwen-omni-turbo-realtime-latest".to_string(),
        ..Default::default()
    }).await?;
    
    // Configure
    session.update_session(&SessionUpdate {
        voice: Some("Cherry".to_string()),
        ..Default::default()
    }).await?;
    
    // Send audio
    session.append_audio(&audio_data).await?;
    session.commit_audio().await?;
    session.create_response().await?;
    
    // Receive response
    while let Some(event) = session.recv().await {
        match event? {
            ServerEvent::ResponseAudioDelta { delta, .. } => {
                player.write(&delta)?;
            }
            ServerEvent::ResponseDone { .. } => break,
            _ => {}
        }
    }
    
    Ok(())
}
```

## Differences from Go

| Feature | Go | Rust |
|---------|----|----- |
| Event receiving | `iter.Seq2` (sync-like) | `async Stream` |
| Session lifetime | Manual `defer Close()` | Drop trait |
| Audio encoding | `[]byte` | `Vec<u8>` |
| WebSocket | gorilla/websocket | tokio-tungstenite |
