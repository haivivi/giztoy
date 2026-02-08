# MiniMax API 示例

此目录包含 MiniMax API 的请求示例文件和测试脚本，供 Go 和 Rust CLI 共用。

## 目录结构

```
examples/minimax/
├── README.md               # 本文件
├── run.sh                  # 测试脚本（支持 Go 和 Rust）
└── commands/               # 请求文件目录
    ├── speech.yaml         # 语音合成请求
    ├── async-speech.yaml   # 异步语音合成请求
    ├── clone-source.yaml   # 用于克隆的源音频生成请求
    ├── chat.yaml           # 文本聊天请求
    ├── image.yaml          # 图片生成请求
    ├── video-t2v.yaml      # 文生视频请求
    ├── video-i2v.yaml      # 图生视频请求
    ├── music.yaml          # 音乐生成请求
    ├── voice-clone.yaml    # 声音克隆请求
    └── voice-design.yaml   # 声音设计请求
```

## 前置条件

1. 配置 API context:

```bash
# 使用 Go CLI
go run ./go/cmd/minimax/main.go config add-context minimax_cn --api-key YOUR_API_KEY

# 或使用 Rust CLI
./rust/target/release/minimax config add-context minimax_cn --api-key YOUR_API_KEY
```

2. 设置默认 context:

```bash
minimax config use-context minimax_cn
```

## 运行测试

### 使用 Bazel（推荐）

```bash
# 使用 Go CLI 运行全部测试
bazel run //examples/minimax:run -- go all

# 使用 Rust CLI 运行基础测试
bazel run //examples/minimax:run -- rust 1

# 同时使用 Go 和 Rust 运行快速测试
bazel run //examples/minimax:run -- both quick
```

### 直接运行脚本

```bash
cd examples/minimax

# 使用 Go CLI 运行全部测试
./run.sh go all

# 使用 Rust CLI 运行基础测试
./run.sh rust 1

# 同时使用 Go 和 Rust 运行快速测试
./run.sh both quick
```

### 测试级别

| 级别 | 说明 |
|------|------|
| 1 | 基础测试 (TTS, Chat) |
| 2 | 图片生成测试 |
| 3 | 流式测试 |
| 4 | 视频任务测试 |
| 5 | 声音管理测试 |
| 6 | 音色克隆测试 |
| 7 | 文件管理测试 |
| 8 | 音乐生成测试 |
| all | 全部测试 |
| quick | 快速测试 (1 + 5) |

### 手动运行示例

```bash
# 语音合成
minimax -c minimax_cn speech synthesize -f examples/minimax/commands/speech.yaml -o output.mp3

# 文本聊天
minimax -c minimax_cn text chat -f examples/minimax/commands/chat.yaml

# 图片生成
minimax -c minimax_cn image generate -f examples/minimax/commands/image.yaml
```

## 请求文件格式

所有请求文件使用 YAML 格式，与 API 文档中的 JSON 格式对应。

### 语音合成 (speech.yaml)

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

### 文本聊天 (chat.yaml)

```yaml
model: MiniMax-M2.1
messages:
  - role: system
    content: You are a helpful assistant.
  - role: user
    content: What is 2+2?
max_tokens: 100
```

### 图片生成 (image.yaml)

```yaml
model: image-01
prompt: A beautiful sunset over mountains
aspect_ratio: "16:9"
n: 1
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `MINIMAX_API_KEY` | API 密钥（用于自动创建 context） | - |
| `MINIMAX_CONTEXT` | 使用的 context 名称 | `minimax_cn` |

## 输出目录

测试脚本会将生成的文件保存到 `examples/minimax/output/` 目录：

- `speech_go.mp3` / `speech_rust.mp3` - 语音合成输出
- `speech_stream_go.mp3` / `speech_stream_rust.mp3` - 流式语音输出
- `music_go.mp3` / `music_rust.mp3` - 音乐生成输出
- `clone_source_go.mp3` / `clone_source_rust.mp3` - 克隆源音频
