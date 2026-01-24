# MiniMax SDK - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/minimax`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/minimax)

## Client

```go
type Client struct {
    Text   *TextService
    Speech *SpeechService
    Voice  *VoiceService
    Video  *VideoService
    Image  *ImageService
    Music  *MusicService
    File   *FileService
}
```

**Constructor:**

```go
// Basic
client := minimax.NewClient("api-key")

// With options
client := minimax.NewClient("api-key",
    minimax.WithBaseURL(minimax.BaseURLGlobal),
    minimax.WithRetry(5),
    minimax.WithHTTPClient(&http.Client{Timeout: 60*time.Second}),
)
```

**Options:**

| Option | Description |
|--------|-------------|
| `WithBaseURL(url)` | Custom API base URL |
| `WithRetry(n)` | Max retry count (default: 3) |
| `WithHTTPClient(c)` | Custom http.Client |

## Services

### TextService

```go
// Synchronous
resp, err := client.Text.CreateChatCompletion(ctx, &minimax.ChatCompletionRequest{
    Model: "MiniMax-M2.1",
    Messages: []minimax.Message{
        {Role: "user", Content: "Hello!"},
    },
})

// Streaming (Go 1.23+ iter.Seq2)
for chunk, err := range client.Text.CreateChatCompletionStream(ctx, req) {
    if err != nil {
        return err
    }
    fmt.Print(chunk.Choices[0].Delta.Content)
}
```

### SpeechService

```go
// Synchronous
resp, err := client.Speech.Synthesize(ctx, &minimax.SpeechRequest{
    Model: "speech-2.6-hd",
    Text:  "Hello, world!",
    VoiceSetting: &minimax.VoiceSetting{
        VoiceID: "male-qn-qingse",
    },
})
// resp.Audio contains decoded audio bytes

// Streaming
for chunk, err := range client.Speech.SynthesizeStream(ctx, req) {
    if err != nil {
        return err
    }
    buf.Write(chunk.Audio)
}

// Async (long text)
task, err := client.Speech.CreateAsyncTask(ctx, &minimax.AsyncSpeechRequest{
    Model: "speech-2.6-hd",
    Text:  longText,
    // ...
})
result, err := task.Wait(ctx)
```

### VoiceService

```go
// List voices
voices, err := client.Voice.List(ctx)

// Clone voice
resp, err := client.Voice.Clone(ctx, &minimax.VoiceCloneRequest{
    FileID:  "uploaded-file-id",
    VoiceID: "my-cloned-voice",
})

// Design voice
resp, err := client.Voice.Design(ctx, &minimax.VoiceDesignRequest{
    Prompt:      "A warm female voice...",
    PreviewText: "Hello, how can I help?",
})
```

### VideoService

```go
// Text to video
task, err := client.Video.CreateTextToVideo(ctx, &minimax.TextToVideoRequest{
    Model:  "video-01",
    Prompt: "A cat playing piano",
})
result, err := task.Wait(ctx)
// result.FileID contains the video file ID

// Image to video
task, err := client.Video.CreateImageToVideo(ctx, &minimax.ImageToVideoRequest{
    Model:          "video-01",
    FirstFrameImage: "https://...",
})
```

### ImageService

```go
resp, err := client.Image.Generate(ctx, &minimax.ImageGenerateRequest{
    Model:  "image-01",
    Prompt: "A beautiful sunset",
})
// resp.Data[0].URL or resp.Data[0].B64JSON
```

### MusicService

```go
task, err := client.Music.Generate(ctx, &minimax.MusicRequest{
    Prompt: "upbeat pop song",
    Lyrics: "[Verse]\nHello world...",
})
result, err := task.Wait(ctx)
```

### FileService

```go
// Upload
resp, err := client.File.Upload(ctx, filePath, minimax.FilePurposeVoiceClone)

// List
files, err := client.File.List(ctx, &minimax.FileListRequest{
    Purpose: minimax.FilePurposeVoiceClone,
})

// Download
data, err := client.File.Download(ctx, fileID)

// Delete
err := client.File.Delete(ctx, fileID)
```

## Task Polling

```go
task, err := client.Video.CreateTextToVideo(ctx, req)
if err != nil {
    return err
}

// Default 5s interval
result, err := task.Wait(ctx)

// Custom interval
result, err := task.WaitWithInterval(ctx, 10*time.Second)

// Manual polling
status, err := task.Query(ctx)
if status.Status == minimax.TaskStatusSuccess {
    // ...
}
```

## Error Handling

```go
resp, err := client.Text.CreateChatCompletion(ctx, req)
if err != nil {
    if e, ok := minimax.AsError(err); ok {
        fmt.Printf("API Error: %d - %s\n", e.StatusCode, e.StatusMsg)
        if e.IsRateLimit() {
            // Wait and retry
        }
    }
    return err
}
```

## Streaming Internals

Uses SSE (Server-Sent Events):
- `iter.Seq2[T, error]` for Go 1.23+ range loops
- Auto-reconnect on transient errors (based on retry config)
- Hex audio decoding for speech streams
