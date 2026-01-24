# MiniMax SDK - Rust Implementation

Crate: `giztoy-minimax`

📚 [Rust Documentation](https://docs.rs/giztoy-minimax)

## Client

```rust
pub struct Client {
    http: Arc<HttpClient>,
    config: ClientConfig,
}
```

**Constructor:**

```rust
use giztoy_minimax::{Client, BASE_URL_GLOBAL};

// Basic
let client = Client::new("api-key")?;

// With builder
let client = Client::builder("api-key")
    .base_url(BASE_URL_GLOBAL)
    .max_retries(5)
    .build()?;
```

**Builder Methods:**

| Method | Description |
|--------|-------------|
| `base_url(url)` | Custom API base URL |
| `max_retries(n)` | Max retry count (default: 3) |

## Services

Services are accessed via getter methods (returns new instance each call):

```rust
client.text()    // TextService
client.speech()  // SpeechService
client.voice()   // VoiceService
client.video()   // VideoService
client.image()   // ImageService
client.music()   // MusicService
client.file()    // FileService
```

### TextService

```rust
use giztoy_minimax::{ChatCompletionRequest, Message};

// Synchronous
let resp = client.text().create_chat_completion(&ChatCompletionRequest {
    model: "MiniMax-M2.1".to_string(),
    messages: vec![
        Message { role: "user".to_string(), content: "Hello!".to_string() },
    ],
    ..Default::default()
}).await?;

// Streaming
let stream = client.text().create_chat_completion_stream(&req).await?;
while let Some(chunk) = stream.next().await {
    let chunk = chunk?;
    if let Some(choice) = chunk.choices.first() {
        print!("{}", choice.delta.content);
    }
}
```

### SpeechService

```rust
use giztoy_minimax::{SpeechRequest, VoiceSetting};

// Synchronous
let resp = client.speech().synthesize(&SpeechRequest {
    model: "speech-2.6-hd".to_string(),
    text: "Hello, world!".to_string(),
    voice_setting: Some(VoiceSetting {
        voice_id: "male-qn-qingse".to_string(),
        ..Default::default()
    }),
    ..Default::default()
}).await?;
// resp.audio contains decoded bytes

// Streaming
let stream = client.speech().synthesize_stream(&req).await?;
while let Some(chunk) = stream.next().await {
    let chunk = chunk?;
    if let Some(audio) = chunk.audio {
        buf.extend(&audio);
    }
}

// Async (long text)
let task = client.speech().create_async_task(&AsyncSpeechRequest {
    // ...
}).await?;
let result = task.wait().await?;
```

### VoiceService

```rust
// List voices
let voices = client.voice().list().await?;

// Clone voice
let resp = client.voice().clone(&VoiceCloneRequest {
    file_id: "uploaded-file-id".to_string(),
    voice_id: "my-cloned-voice".to_string(),
}).await?;

// Design voice
let resp = client.voice().design(&VoiceDesignRequest {
    prompt: "A warm female voice...".to_string(),
    preview_text: "Hello, how can I help?".to_string(),
    ..Default::default()
}).await?;
```

### VideoService

```rust
// Text to video
let task = client.video().create_text_to_video(&TextToVideoRequest {
    model: "video-01".to_string(),
    prompt: "A cat playing piano".to_string(),
    ..Default::default()
}).await?;
let result = task.wait().await?;

// Image to video
let task = client.video().create_image_to_video(&ImageToVideoRequest {
    model: "video-01".to_string(),
    first_frame_image: "https://...".to_string(),
    ..Default::default()
}).await?;
```

### ImageService

```rust
let resp = client.image().generate(&ImageGenerateRequest {
    model: "image-01".to_string(),
    prompt: "A beautiful sunset".to_string(),
    ..Default::default()
}).await?;
```

### MusicService

```rust
let task = client.music().generate(&MusicRequest {
    prompt: "upbeat pop song".to_string(),
    lyrics: "[Verse]\nHello world...".to_string(),
    ..Default::default()
}).await?;
let result = task.wait().await?;
```

### FileService

```rust
// Upload
let resp = client.file().upload(file_path, FilePurpose::VoiceClone).await?;

// List
let files = client.file().list(Some(FilePurpose::VoiceClone)).await?;

// Download
let data = client.file().download(&file_id).await?;

// Delete
client.file().delete(&file_id).await?;
```

## Task Polling

```rust
let task = client.video().create_text_to_video(&req).await?;

// Default interval
let result = task.wait().await?;

// Custom interval
let result = task.wait_with_interval(Duration::from_secs(10)).await?;

// Manual polling
let status = task.query().await?;
if status.status == TaskStatus::Success {
    // ...
}
```

## Error Handling

```rust
use giztoy_minimax::{Error, Result};

match client.text().create_chat_completion(&req).await {
    Ok(resp) => { /* ... */ }
    Err(Error::Api { status_code, status_msg }) => {
        eprintln!("API Error: {} - {}", status_code, status_msg);
    }
    Err(Error::Http(e)) => {
        eprintln!("HTTP Error: {}", e);
    }
    Err(e) => {
        eprintln!("Error: {}", e);
    }
}
```

## HasModel Trait

For default model handling:

```rust
pub trait HasModel {
    fn model(&self) -> &str;
    fn set_model(&mut self, model: impl Into<String>);
    fn default_model() -> &'static str;
    fn apply_default_model(&mut self);
}
```

## Differences from Go

| Feature | Go | Rust |
|---------|----|----- |
| Client construction | `NewClient()` (panic on empty key) | `Client::new()` (returns Result) |
| Service access | Direct fields (`client.Text`) | Getter methods (`client.text()`) |
| Streaming | `iter.Seq2[T, error]` | `Stream<Item=Result<T>>` |
| Options | Functional options | Builder pattern |
| Error type | `*Error` with helper methods | `Error` enum |
