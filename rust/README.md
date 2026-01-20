# giztoy-rust

MiniMax API SDK and CLI for Rust.

## 特性

- **完整的 MiniMax API 支持**
  - 文本生成 (Chat Completions)
  - 语音合成 (TTS) - 同步和流式
  - 视频生成 (T2V, I2V)
  - 图像生成
  - 音乐生成
  - 声音管理 (克隆、设计)
  - 文件管理

- **同时支持 Cargo 和 Bazel 构建**
- **功能与 Go 版本 SDK 保持一致**
- **与 Go 版本共用 `~/.giztoy/minimax/` 配置目录**

## 安装

### 使用 Cargo

```bash
cd rust
cargo build --release
```

### 使用 Bazel

```bash
bazel build //rust:minimax
```

## SDK 使用示例

```rust
use giztoy::minimax::{Client, SpeechRequest, VoiceSetting, MODEL_SPEECH_26_HD, VOICE_FEMALE_SHAONV};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // 创建客户端
    let client = Client::new("your-api-key")?;
    
    // 语音合成
    let request = SpeechRequest {
        model: MODEL_SPEECH_26_HD.to_string(),
        text: "你好，世界！".to_string(),
        voice_setting: Some(VoiceSetting {
            voice_id: VOICE_FEMALE_SHAONV.to_string(),
            ..Default::default()
        }),
        ..Default::default()
    };
    
    let response = client.speech().synthesize(&request).await?;
    std::fs::write("output.mp3", &response.audio)?;
    
    Ok(())
}
```

## CLI 使用

CLI 与 Go 版本完全兼容，共用 `~/.giztoy/minimax/config.yaml` 配置文件。

### 配置管理

```bash
# 添加 context
minimax config add-context myctx --api-key YOUR_API_KEY

# 添加带有自定义 base-url 的 context
minimax config add-context prod --api-key YOUR_API_KEY --base-url https://api.minimaxi.chat

# 设置默认 context
minimax config use-context myctx

# 列出所有 contexts
minimax config list-contexts

# 查看当前配置
minimax config view

# 删除 context
minimax config delete-context old-ctx
```

### 语音合成

```bash
# 同步合成（使用请求文件）
minimax -c myctx speech synthesize -f speech.yaml -o output.mp3

# 流式合成
minimax -c myctx speech stream -f speech.yaml -o output.mp3

# 异步任务（用于长文本）
minimax -c myctx speech async -f long-text.yaml
```

请求文件示例 (`speech.yaml`):
```yaml
model: speech-2.6-hd
text: Hello, this is a test message.
voice_setting:
  voice_id: female-shaonv
  speed: 1.0
audio_setting:
  format: mp3
  sample_rate: 32000
```

### 文本生成

```bash
# 对话补全
minimax -c myctx text chat -f chat.yaml

# 流式输出
minimax -c myctx text stream -f chat.yaml

# JSON 输出（用于管道）
minimax -c myctx text chat -f chat.yaml --json | jq '.choices[0].message'
```

请求文件示例 (`chat.yaml`):
```yaml
model: MiniMax-M2.1
messages:
  - role: system
    content: You are a helpful assistant.
  - role: user
    content: What is 2+2?
max_tokens: 100
```

### 图像生成

```bash
minimax -c myctx image generate -f image.yaml
```

请求文件示例 (`image.yaml`):
```yaml
model: image-01
prompt: A beautiful sunset over mountains
aspect_ratio: "16:9"
n: 1
```

### 视频生成

```bash
# 文生视频
minimax -c myctx video t2v -f video-t2v.yaml

# 图生视频
minimax -c myctx video i2v -f video-i2v.yaml
```

### 声音管理

```bash
# 列出声音
minimax -c myctx voice list

# 上传克隆源音频
minimax -c myctx voice upload-clone-source --file voice-sample.mp3

# 克隆声音
minimax -c myctx voice clone -f voice-clone.yaml

# 设计声音
minimax -c myctx voice design -f voice-design.yaml
```

### 文件管理

```bash
# 上传文件
minimax -c myctx file upload --file audio.mp3 --purpose voice_clone

# 列出文件
minimax -c myctx file list

# 获取文件信息
minimax -c myctx file retrieve FILE_ID

# 删除文件
minimax -c myctx file delete FILE_ID
```

## 示例

运行示例：

```bash
export MINIMAX_API_KEY="your-api-key"

# 语音合成示例
cargo run --example minimax-speech

# 文本生成示例
cargo run --example minimax-text

# 图像生成示例
cargo run --example minimax-image
```

## 项目结构

```
rust/
├── Cargo.toml              # Cargo 配置
├── Cargo.lock              # 依赖锁定
├── BUILD.bazel             # Bazel 构建配置
├── src/
│   ├── lib.rs              # 库入口
│   ├── minimax/            # MiniMax SDK
│   │   ├── mod.rs
│   │   ├── client.rs       # API 客户端
│   │   ├── error.rs        # 错误处理
│   │   ├── http.rs         # HTTP 客户端
│   │   ├── types.rs        # 通用类型
│   │   ├── models.rs       # 模型常量
│   │   ├── speech.rs       # 语音服务
│   │   ├── text.rs         # 文本服务
│   │   ├── video.rs        # 视频服务
│   │   ├── image.rs        # 图像服务
│   │   ├── music.rs        # 音乐服务
│   │   ├── voice.rs        # 声音服务
│   │   ├── file.rs         # 文件服务
│   │   └── task.rs         # 异步任务
│   ├── cli/                # CLI 工具库
│   │   ├── mod.rs
│   │   ├── config.rs       # 配置管理（兼容 Go 版本）
│   │   └── output.rs       # 输出格式化
│   └── bin/
│       └── minimax/        # CLI 可执行程序
│           ├── main.rs
│           └── commands/
│               ├── mod.rs
│               ├── util.rs     # 工具函数
│               ├── config.rs
│               ├── speech.rs
│               ├── text.rs
│               ├── video.rs
│               ├── image.rs
│               ├── music.rs
│               ├── voice.rs
│               └── file.rs
└── examples/               # 示例代码
    └── minimax/            # MiniMax SDK 示例
        ├── speech.rs
        ├── text.rs
        └── image.rs
```

## 与 Go 版本的兼容性

Rust 版本与 Go 版本完全兼容：

1. **共用配置文件**: `~/.giztoy/minimax/config.yaml`
2. **相同的 CLI 命令结构**: `minimax config`, `minimax speech`, `minimax text`, etc.
3. **相同的请求文件格式**: YAML/JSON 请求文件可互用

| 功能 | Go | Rust |
|------|-----|------|
| 客户端 | `minimax.NewClient()` | `Client::new()` |
| 语音合成 | `client.Speech.Synthesize()` | `client.speech().synthesize()` |
| 流式语音 | `client.Speech.SynthesizeStream()` | `client.speech().synthesize_stream()` |
| 文本生成 | `client.Text.CreateChatCompletion()` | `client.text().create_chat_completion()` |
| 图像生成 | `client.Image.Generate()` | `client.image().generate()` |

## License

MIT
