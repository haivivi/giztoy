# Jiutian - Rust Implementation

## Status: Not Implemented

No native Rust SDK for Jiutian API exists in this repository.

## Recommendation

Use OpenAI-compatible SDK since Jiutian API follows OpenAI chat completions format:

```rust
use async_openai::{Client, config::OpenAIConfig};

let config = OpenAIConfig::new()
    .with_api_key("sk-your-jiutian-token")
    .with_api_base("https://ivs.chinamobiledevice.com:30100/v1");

let client = Client::with_config(config);

let request = CreateChatCompletionRequestArgs::default()
    .model("jiutian")
    .messages([
        ChatCompletionRequestMessage::System(
            ChatCompletionRequestSystemMessageArgs::default()
                .content("您好，我是中国移动的智能助理灵犀。")
                .build()?
        ),
        ChatCompletionRequestMessage::User(
            ChatCompletionRequestUserMessageArgs::default()
                .content("你是谁？")
                .build()?
        ),
    ])
    .build()?;

let response = client.chat().create(request).await?;
```

## Device Protocol

For device-specific features, implement HTTP client using `reqwest`:

```rust
use serde::{Deserialize, Serialize};

#[derive(Serialize)]
struct HeartbeatRequest {
    device_id: String,
    product_id: String,
    timestamp: i64,
}

async fn send_heartbeat(
    client: &reqwest::Client,
    base_url: &str,
    req: &HeartbeatRequest,
) -> Result<(), Error> {
    client
        .post(format!("{}/api/device/heartbeat", base_url))
        .json(req)
        .send()
        .await?;
    Ok(())
}
```

## Future Work

A native SDK could provide:
- Device registration/heartbeat management
- Token refresh handling  
- Jiutian-specific features

See [api/device.md](./api/device.md) for device protocol details.
