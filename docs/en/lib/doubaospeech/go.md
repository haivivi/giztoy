# Doubao Speech SDK - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/doubaospeech`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/doubaospeech)

## Clients

### Speech API Client

```go
type Client struct {
    // V1 Services (Classic)
    TTS *TTSService
    ASR *ASRService
    
    // V2 Services (BigModel)
    TTSV2 *TTSServiceV2
    ASRV2 *ASRServiceV2
    
    // Shared Services
    VoiceClone  *VoiceCloneService
    Realtime    *RealtimeService
    Meeting     *MeetingService
    Podcast     *PodcastService
    Translation *TranslationService
    Media       *MediaService
}
```

**Constructor:**

```go
// With API Key (recommended)
client := doubaospeech.NewClient("app-id",
    doubaospeech.WithAPIKey("your-api-key"),
    doubaospeech.WithCluster("volcano_tts"),
)

// With Bearer Token
client := doubaospeech.NewClient("app-id",
    doubaospeech.WithBearerToken("your-token"),
)

// With V2 API Key (for BigModel APIs)
client := doubaospeech.NewClient("app-id",
    doubaospeech.WithV2APIKey("access-key", "app-key"),
    doubaospeech.WithResourceID("seed-tts-2.0"),
)
```

### Console API Client

```go
console := doubaospeech.NewConsole("access-key", "secret-key")
```

## Services

### TTS V1 (Classic)

```go
// Synchronous
resp, err := client.TTS.Synthesize(ctx, &doubaospeech.TTSRequest{
    Text:      "你好，世界！",
    VoiceType: "zh_female_cancan",
})
// resp.Audio contains audio bytes

// Streaming (Go 1.23+ iter.Seq2)
for chunk, err := range client.TTS.SynthesizeStream(ctx, req) {
    if err != nil {
        return err
    }
    buf.Write(chunk.Audio)
}
```

### TTS V2 (BigModel)

```go
// Streaming HTTP (recommended)
for chunk, err := range client.TTSV2.Stream(ctx, &doubaospeech.TTSV2Request{
    Text:       "Hello, world!",
    VoiceType:  "zh_female_xiaohe_uranus_bigtts",
    ResourceID: "seed-tts-2.0",
}) {
    // Process chunk
}

// WebSocket Bidirectional
session, err := client.TTSV2.OpenSession(ctx, &doubaospeech.TTSV2SessionConfig{
    VoiceType:  "zh_female_xiaohe_uranus_bigtts",
    ResourceID: "seed-tts-2.0",
})
defer session.Close()

session.SendText(ctx, "First segment", false)
session.SendText(ctx, "Second segment", true)

for chunk, err := range session.Recv() {
    if err != nil {
        break
    }
    buf.Write(chunk.Audio)
    if chunk.IsLast {
        break
    }
}
```

**IMPORTANT:** Speaker voice must match Resource ID!

| Resource ID | Speaker Suffix Required |
|-------------|-------------------------|
| `seed-tts-2.0` | `*_uranus_bigtts` |
| `seed-tts-1.0` | `*_moon_bigtts` |

### ASR V1 (Classic)

```go
// One-sentence
resp, err := client.ASR.Recognize(ctx, &doubaospeech.ASRRequest{
    Audio:    audioData,
    Format:   "pcm",
    Language: "zh-CN",
})

// Streaming (WebSocket)
session, err := client.ASR.OpenStreamSession(ctx, &doubaospeech.StreamASRConfig{
    Format:     "pcm",
    SampleRate: 16000,
})
defer session.Close()

// Send audio chunks
session.SendAudio(ctx, audioData, false)
session.SendAudio(ctx, lastData, true)

// Receive results
for chunk, err := range session.Recv() {
    if err != nil {
        break
    }
    fmt.Println(chunk.Text)
}
```

### ASR V2 (BigModel)

```go
// Streaming (recommended)
session, err := client.ASRV2.OpenStreamSession(ctx, &doubaospeech.ASRV2Config{
    Format:     "pcm",
    SampleRate: 16000,
    Language:   "zh-CN",
    EnableITN:  true,
    EnablePunc: true,
})
defer session.Close()

// Send audio chunks
session.SendAudio(ctx, audioData, false)
session.SendAudio(ctx, lastData, true)

// Receive results
for chunk, err := range session.Recv() {
    if err != nil {
        break
    }
    fmt.Println(chunk.Text)
    if chunk.IsFinal {
        break
    }
}

// Async file recognition
result, err := client.ASRV2.SubmitAsync(ctx, &doubaospeech.ASRV2AsyncRequest{
    AudioURL: "https://example.com/audio.mp3",
    Format:   "mp3",
    Language: "zh-CN",
})
fmt.Println("Task ID:", result.TaskID)

// Query task status
status, err := client.ASRV2.QueryAsync(ctx, result.TaskID)
fmt.Println("Status:", status.Status, "Text:", status.Text)
```

### Voice Clone

```go
// Upload audio for training
result, err := client.VoiceClone.Upload(ctx, &doubaospeech.VoiceCloneRequest{
    AudioData: audioData,
    VoiceID:   "my-custom-voice",
})

// Check status
status, err := client.VoiceClone.GetStatus(ctx, "my-custom-voice")

// Activate voice
err := client.VoiceClone.Activate(ctx, "my-custom-voice")
```

### Realtime Dialogue

```go
session, err := client.Realtime.Connect(ctx, &doubaospeech.RealtimeConfig{
    Model: "speech-dialog-001",
})
defer session.Close()

// Send audio
session.SendAudio(audioData)

// Receive events
for event := range session.Events() {
    switch event.Type {
    case "asr_result":
        fmt.Println("User:", event.AsrResult.Text)
    case "tts_audio":
        play(event.TtsAudio)
    }
}
```

### Console API

```go
// List available voices
voices, err := console.ListSpeakers(ctx, &doubaospeech.ListSpeakersRequest{})

// List timbres
timbres, err := console.ListTimbres(ctx, &doubaospeech.ListTimbresRequest{})

// Check voice clone status
status, err := console.ListVoiceCloneStatus(ctx, &doubaospeech.ListVoiceCloneStatusRequest{
    VoiceID: "my-custom-voice",
})
```

## Options

| Option | Description |
|--------|-------------|
| `WithAPIKey(key)` | x-api-key authentication |
| `WithBearerToken(token)` | Bearer token authentication |
| `WithV2APIKey(access, app)` | V2/V3 API authentication |
| `WithCluster(cluster)` | Set cluster name (V1) |
| `WithResourceID(id)` | Set resource ID (V2) |
| `WithBaseURL(url)` | Custom HTTP base URL |
| `WithWebSocketURL(url)` | Custom WebSocket URL |
| `WithHTTPClient(client)` | Custom HTTP client |
| `WithTimeout(duration)` | Request timeout |
| `WithUserID(id)` | User identifier |

## Error Handling

```go
if err != nil {
    if e, ok := doubaospeech.AsError(err); ok {
        fmt.Printf("Error %d: %s\n", e.Code, e.Message)
        if e.IsRateLimit() {
            // Handle rate limiting
        }
    }
}
```
