# Jiutian - Go Implementation

## Status: Not Implemented

No native Go SDK for Jiutian API exists in this repository.

## Recommendation

Use OpenAI-compatible SDK since Jiutian API follows OpenAI chat completions format:

```go
import "github.com/sashabaranov/go-openai"

config := openai.DefaultConfig("sk-your-jiutian-token")
config.BaseURL = "https://ivs.chinamobiledevice.com:30100/v1"

client := openai.NewClientWithConfig(config)

resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model: "jiutian",
    Messages: []openai.ChatCompletionMessage{
        {
            Role:    "system",
            Content: "您好，我是中国移动的智能助理灵犀。",
        },
        {
            Role:    "user", 
            Content: "你是谁？",
        },
    },
})
```

## Device Protocol

For device-specific features (registration, heartbeat), implement HTTP client directly:

```go
// Device heartbeat
type HeartbeatRequest struct {
    DeviceID  string `json:"device_id"`
    ProductID string `json:"product_id"`
    Timestamp int64  `json:"timestamp"`
}

func sendHeartbeat(ctx context.Context, client *http.Client, req *HeartbeatRequest) error {
    // POST to /api/device/heartbeat
}
```

## Future Work

A native SDK could provide:
- Device registration/heartbeat management
- Token refresh handling
- Jiutian-specific features

See [api/device.md](./api/device.md) for device protocol details.
