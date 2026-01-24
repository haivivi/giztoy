# DashScope SDK - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/dashscope`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/dashscope)

## Client

```go
type Client struct {
    Realtime *RealtimeService
}
```

**Constructor:**

```go
// Basic
client := dashscope.NewClient("sk-xxxxxxxx")

// With workspace
client := dashscope.NewClient("sk-xxxxxxxx",
    dashscope.WithWorkspace("ws-xxxxxxxx"),
)

// Custom endpoint (international)
client := dashscope.NewClient("sk-xxxxxxxx",
    dashscope.WithBaseURL("wss://dashscope-intl.aliyuncs.com/api-ws/v1/realtime"),
)
```

**Options:**

| Option | Description |
|--------|-------------|
| `WithWorkspace(id)` | Workspace ID for isolation |
| `WithBaseURL(url)` | Custom WebSocket URL |
| `WithHTTPBaseURL(url)` | Custom HTTP URL |
| `WithHTTPClient(client)` | Custom HTTP client |

## RealtimeService

### Connect Session

```go
session, err := client.Realtime.Connect(ctx, &dashscope.RealtimeConfig{
    Model: dashscope.ModelQwenOmniTurboRealtimeLatest,
})
if err != nil {
    log.Fatal(err)
}
defer session.Close()
```

### Send Events

```go
// Update session configuration
session.UpdateSession(&dashscope.SessionUpdate{
    Modalities: []string{"text", "audio"},
    Voice: "Cherry",
    InputAudioFormat: "pcm16",
    OutputAudioFormat: "pcm16",
})

// Append audio data
session.AppendAudio(audioData)

// Commit audio (finalize input)
session.CommitAudio()

// Create response (start inference)
session.CreateResponse()

// Send text
session.AppendText("Hello!")

// Cancel response
session.CancelResponse()
```

### Receive Events

```go
// Using Go 1.23+ iter.Seq2
for event, err := range session.Events() {
    if err != nil {
        log.Fatal(err)
    }
    
    switch event.Type {
    case dashscope.EventResponseAudioDelta:
        // Audio chunk received
        play(event.Delta)
        
    case dashscope.EventResponseTextDelta:
        // Text chunk received
        fmt.Print(event.Delta)
        
    case dashscope.EventResponseDone:
        // Response complete
        
    case dashscope.EventError:
        // Error occurred
        log.Printf("Error: %s", event.Error.Message)
    }
}
```

## Events

### Client Events (Send)

| Event Type | Description |
|------------|-------------|
| `session.update` | Update session configuration |
| `input_audio_buffer.append` | Append audio data |
| `input_audio_buffer.commit` | Finalize audio input |
| `response.create` | Request response |
| `response.cancel` | Cancel current response |

### Server Events (Receive)

| Event Type | Description |
|------------|-------------|
| `session.created` | Session established |
| `session.updated` | Configuration updated |
| `response.created` | Response started |
| `response.audio.delta` | Audio chunk |
| `response.text.delta` | Text chunk |
| `response.done` | Response complete |
| `error` | Error occurred |

## Models

```go
const (
    ModelQwenOmniTurboRealtimeLatest  = "qwen-omni-turbo-realtime-latest"
    ModelQwen3OmniFlashRealtimeLatest = "qwen3-omni-flash-realtime-latest"
)
```

## Error Handling

```go
for event, err := range session.Events() {
    if err != nil {
        // Connection error
        log.Fatal(err)
    }
    
    if event.Type == dashscope.EventError {
        // API error
        log.Printf("API Error [%s]: %s", event.Error.Code, event.Error.Message)
    }
}
```

## Complete Example

```go
func main() {
    client := dashscope.NewClient(os.Getenv("DASHSCOPE_API_KEY"))
    
    session, err := client.Realtime.Connect(context.Background(), &dashscope.RealtimeConfig{
        Model: dashscope.ModelQwenOmniTurboRealtimeLatest,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer session.Close()
    
    // Configure session
    session.UpdateSession(&dashscope.SessionUpdate{
        Voice: "Cherry",
    })
    
    // Send audio (from microphone, etc.)
    session.AppendAudio(audioData)
    session.CommitAudio()
    session.CreateResponse()
    
    // Receive and play response
    for event, err := range session.Events() {
        if err != nil {
            break
        }
        if event.Type == dashscope.EventResponseAudioDelta {
            player.Write(event.Delta)
        }
    }
}
```
