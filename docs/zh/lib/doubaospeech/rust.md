# Doubao Speech SDK - Rust Implementation

Crate: `giztoy-doubaospeech`

📚 [Rust Documentation](https://docs.rs/giztoy-doubaospeech)

## Clients

### Speech API Client

```rust
pub struct Client {
    // Internal HTTP/WebSocket clients
}

impl Client {
    // V1 Services (Classic)
    pub fn tts(&self) -> TtsService;
    pub fn asr(&self) -> AsrService;
    
    // V2 Services (BigModel)
    pub fn tts_v2(&self) -> TtsV2Service;
    pub fn asr_v2(&self) -> AsrV2Service;
    
    // Shared Services
    pub fn voice_clone(&self) -> VoiceCloneService;
    pub fn realtime(&self) -> RealtimeService;
    pub fn meeting(&self) -> MeetingService;
    pub fn podcast(&self) -> PodcastService;
    pub fn translation(&self) -> TranslationService;
    pub fn media(&self) -> MediaService;
}
```

**Constructor:**

```rust
use giztoy_doubaospeech::Client;

// With API Key (recommended)
let client = Client::builder("app-id")
    .api_key("your-api-key")
    .cluster("volcano_tts")
    .build()?;

// With Bearer Token
let client = Client::builder("app-id")
    .bearer_token("your-token")
    .build()?;

// With V2 API Key
let client = Client::builder("app-id")
    .v2_api_key("access-key", "app-key")
    .resource_id("seed-tts-2.0")
    .build()?;
```

### Console Client

```rust
use giztoy_doubaospeech::Console;

let console = Console::new("access-key", "secret-key");
```

## Services

### TTS V1 Service (Classic)

```rust
use giztoy_doubaospeech::{TtsRequest, TtsService};

// Synchronous
let response = client.tts().synthesize(&TtsRequest {
    text: "你好，世界！".to_string(),
    voice_type: "zh_female_cancan".to_string(),
    ..Default::default()
}).await?;
// response.audio contains bytes

// Streaming
let stream = client.tts().synthesize_stream(&req).await?;
while let Some(chunk) = stream.next().await {
    let chunk = chunk?;
    if let Some(audio) = chunk.audio {
        buf.extend(&audio);
    }
}
```

### TTS V2 Service (BigModel)

```rust
use giztoy_doubaospeech::{TtsV2Request, TtsV2Service};

// HTTP Streaming (recommended)
let req = TtsV2Request {
    text: "你好，世界！".to_string(),
    speaker: "zh_female_xiaohe_uranus_bigtts".to_string(),
    resource_id: Some("seed-tts-2.0".to_string()),
    format: Some("mp3".to_string()),
    sample_rate: Some(24000),
    ..Default::default()
};

let mut stream = client.tts_v2().stream(&req).await?;
while let Some(chunk) = stream.recv().await {
    let chunk = chunk?;
    if let Some(audio) = &chunk.audio {
        buf.extend(audio);
    }
}

// WebSocket Bidirectional
let mut session = client.tts_v2().bidirectional(&req).await?;
session.send_text("第一段文字", false).await?;
session.send_text("第二段文字", true).await?;

while let Some(chunk) = session.recv().await {
    let chunk = chunk?;
    if let Some(audio) = &chunk.audio {
        buf.extend(audio);
    }
    if chunk.is_last {
        break;
    }
}
session.close().await?;
```

**IMPORTANT:** Speaker voice must match Resource ID!

| Resource ID | Speaker Suffix Required |
|-------------|-------------------------|
| `seed-tts-2.0` | `*_uranus_bigtts` |
| `seed-tts-1.0` | `*_moon_bigtts` |

### ASR V1 Service (Classic)

```rust
use giztoy_doubaospeech::{OneSentenceRequest, StreamAsrConfig};

// One-sentence
let result = client.asr().recognize(&OneSentenceRequest {
    audio: audio_data,
    format: "pcm".to_string(),
    language: "zh-CN".to_string(),
    ..Default::default()
}).await?;

// Streaming
let session = client.asr().open_stream_session(&StreamAsrConfig {
    format: "pcm".to_string(),
    sample_rate: 16000,
    ..Default::default()
}).await?;

// Send audio
session.send_audio(&audio_data, false).await?;
session.send_audio(&last_data, true).await?;

// Receive results
while let Some(result) = session.recv().await {
    let chunk = result?;
    println!("Text: {}", chunk.text);
}
```

### ASR V2 Service (BigModel)

```rust
use giztoy_doubaospeech::{AsrV2Config, AsrV2AsyncRequest};

// Streaming (recommended)
let config = AsrV2Config {
    format: "pcm".to_string(),
    sample_rate: 16000,
    channels: 1,
    bits: 16,
    language: Some("zh-CN".to_string()),
    enable_itn: true,
    enable_punc: true,
    ..Default::default()
};

let mut session = client.asr_v2().open_stream_session(&config).await?;

// Send audio chunks
session.send_audio(&audio_data, false).await?;
session.send_audio(&last_data, true).await?;

// Receive results
while let Some(result) = session.recv().await {
    let chunk = result?;
    println!("Text: {}", chunk.text);
    if chunk.is_final {
        break;
    }
}
session.close().await?;

// Async file recognition
let result = client.asr_v2().submit_async(&AsrV2AsyncRequest {
    audio_url: Some("https://example.com/audio.mp3".to_string()),
    format: "mp3".to_string(),
    language: Some("zh-CN".to_string()),
    ..Default::default()
}).await?;
println!("Task ID: {}", result.task_id);

// Query task status
let status = client.asr_v2().query_async(&result.task_id).await?;
println!("Status: {}, Text: {}", status.status, status.text);
```

### Voice Clone Service

```rust
// Upload for training
let result = client.voice_clone().upload(&VoiceCloneTrainRequest {
    audio_data: audio_bytes,
    voice_id: "my-custom-voice".to_string(),
    ..Default::default()
}).await?;

// Check status
let status = client.voice_clone().get_status("my-custom-voice").await?;
```

### Realtime Service

```rust
use giztoy_doubaospeech::{RealtimeConfig, RealtimeEventType};

let session = client.realtime().connect(&RealtimeConfig {
    model: "speech-dialog-001".to_string(),
    ..Default::default()
}).await?;

// Send audio
session.send_audio(&audio_data).await?;

// Receive events
while let Some(event) = session.recv().await {
    let event = event?;
    match event.event_type {
        RealtimeEventType::AsrResult => {
            println!("User: {}", event.asr_result.text);
        }
        RealtimeEventType::TtsAudio => {
            play(&event.tts_audio);
        }
        _ => {}
    }
}
```

### Console API

```rust
use giztoy_doubaospeech::{Console, ListSpeakersRequest};

let console = Console::new("access-key", "secret-key");

// List speakers
let speakers = console.list_speakers(&ListSpeakersRequest::default()).await?;

// List timbres
let timbres = console.list_timbres(&ListTimbresRequest::default()).await?;
```

## Builder Options

| Method | Description |
|--------|-------------|
| `api_key(key)` | x-api-key authentication |
| `bearer_token(token)` | Bearer token authentication |
| `v2_api_key(access, app)` | V2/V3 API authentication |
| `cluster(cluster)` | Set cluster name (V1) |
| `resource_id(id)` | Set resource ID (V2) |
| `base_url(url)` | Custom HTTP base URL |
| `ws_url(url)` | Custom WebSocket URL |
| `timeout(duration)` | Request timeout |
| `user_id(id)` | User identifier |

## Error Handling

```rust
use giztoy_doubaospeech::{Error, Result};

match client.tts().synthesize(&req).await {
    Ok(resp) => { /* ... */ }
    Err(Error::Api { code, message }) => {
        eprintln!("API Error {}: {}", code, message);
    }
    Err(e) => {
        eprintln!("Error: {}", e);
    }
}
```

## Differences from Go

| Feature | Go | Rust |
|---------|----|----- |
| V1/V2 service access | Separate fields (`TTS`, `TTSV2`) | Separate methods (`tts()`, `tts_v2()`) |
| Streaming | `iter.Seq2` | `async fn recv()` |
| Session management | Manual `Close()` | Manual `close().await` |
| WebSocket | gorilla/websocket | tokio-tungstenite |
| Binary Protocol | `protocol.go` | `protocol.rs` |

Both implementations have **full feature parity**:
- TTS V1/V2 (HTTP stream, WS bidirectional)
- ASR V1/V2 (one-sentence, stream, file)
- Podcast (HTTP async, SAMI WebSocket)
- Realtime Dialogue
- Voice Clone
- Meeting/Media/Translation
