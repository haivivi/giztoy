# DashScope CLI 示例

## 概述

这个目录包含 DashScope CLI 的测试脚本和示例配置文件。

## 使用方法

### 前置条件

1. 获取 DashScope API Key: https://bailian.console.aliyun.com/?apiKey=1
2. 配置 context:

```bash
# Go CLI
bazel run //go/cmd/dashscope -- config add-context dashscope_cn --api-key YOUR_API_KEY

# 或 Rust CLI
bazel run //rust/cmd/dashscope -- config add-context dashscope_cn --api-key YOUR_API_KEY
```

### 运行测试

```bash
# 快速测试 (检查 CLI 是否正常)
bazel run //examples/cmd/dashscope:run -- go quick
bazel run //examples/cmd/dashscope:run -- rust quick

# 同时测试 Go 和 Rust
bazel run //examples/cmd/dashscope:run -- both quick

# 运行全部测试 (需要音频文件)
bazel run //examples/cmd/dashscope:run -- go all
```

### 环境变量

- `DASHSCOPE_CONTEXT`: Context 名称 (默认: dashscope_cn)
- `DASHSCOPE_API_KEY`: API Key (如果设置，会自动创建 context)

## 配置文件

### omni-chat.yaml

Qwen-Omni-Realtime 实时语音对话配置:

```yaml
model: qwen-omni-turbo-realtime-latest
voice: Chelsie
input_audio_format: pcm16
output_audio_format: pcm16
modalities:
  - text
  - audio
enable_input_audio_transcription: true
turn_detection:
  type: server_vad
  threshold: 0.5
  silence_duration_ms: 800
```

## 音频格式

- **输入**: 16-bit PCM, 16kHz, mono
- **输出**: 16-bit PCM, 24kHz, mono (Turbo模型)

可以使用 ffmpeg 转换音频:

```bash
# 转换为输入格式
ffmpeg -i input.mp3 -ar 16000 -ac 1 -f s16le input.pcm

# 转换输出为 mp3
ffmpeg -f s16le -ar 24000 -ac 1 -i output.pcm output.mp3
```

## 支持的模型

| 模型 | 输出格式 | 默认音色 | 特点 |
|------|---------|---------|------|
| qwen-omni-turbo-realtime-latest | pcm16 | Chelsie | 高质量 |
| qwen3-omni-flash-realtime | pcm24 | Cherry | 快速响应 |

## 支持的音色

- Chelsie (默认)
- Cherry
- Serena
- Ethan
