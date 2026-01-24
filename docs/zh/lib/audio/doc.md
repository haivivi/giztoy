# 音频包

用于语音和多媒体应用的音频处理框架。

## 设计目标

1. **实时处理**：低延迟音频混音、编码和流传输
2. **格式灵活**：支持常见音频格式（PCM、Opus、MP3、OGG）
3. **跨平台**：提供原生库的 FFI 绑定（libopus、libsoxr、lame）
4. **流式优先**：专为连续音频流设计，而非仅针对文件

## 架构

```mermaid
graph TB
    subgraph audio["audio/"]
        subgraph row1[" "]
            pcm["pcm/<br/>- 格式<br/>- 分块<br/>- 混音"]
            codec["codec/<br/>- opus/<br/>- mp3/<br/>- ogg/"]
            resampler["resampler/<br/>- soxr<br/>- 格式<br/>- 转换"]
        end
        subgraph row2[" "]
            opusrt["opusrt/<br/>- 缓冲<br/>- 实时<br/>- OGG 读写"]
            songs["songs/<br/>- 目录<br/>- 音符<br/>- PCM 生成"]
            portaudio["portaudio/<br/>(仅 Go)<br/>- 流<br/>- 设备"]
        end
    end
```

## 子模块

| 模块 | 描述 | Go | Rust |
|--------|-------------|:--:|:----:|
| [pcm/](./pcm/doc.md) | PCM 格式、分块、混音 | ✅ | ✅ |
| [codec/](./codec/doc.md) | 音频编解码器（Opus、MP3、OGG） | ✅ | ✅ |
| [resampler/](./resampler/doc.md) | 采样率转换（soxr） | ✅ | ✅ |
| [opusrt/](./opusrt/doc.md) | 实时 Opus 流传输 | ✅ | ⚠️ |
| [songs/](./songs/doc.md) | 内置旋律 | ✅ | ✅ |
| [portaudio/](./portaudio/doc.md) | 音频 I/O 设备 | ✅ | ❌ |

## 音频格式

### PCM 格式（预定义）

| 格式 | 采样率 | 声道 | 位深 |
|--------|-------------|----------|-----------|
| `L16Mono16K` | 16000 Hz | 1 | 16 位 |
| `L16Mono24K` | 24000 Hz | 1 | 16 位 |
| `L16Mono48K` | 48000 Hz | 1 | 16 位 |

### 编解码器支持

| 编解码器 | 编码 | 解码 | 容器 |
|-------|--------|--------|-----------|
| Opus | ✅ | ✅ | Raw、OGG |
| MP3 | ✅ | ✅ | Raw |
| OGG | N/A | N/A | 仅容器 |

## 常见工作流

### 语音聊天（低延迟）

```mermaid
flowchart LR
    A[麦克风] --> B[PCM 16kHz]
    B --> C[Opus 编码]
    C --> D[网络]
    D --> E[Opus 解码]
    E --> F[混音器]
    F --> G[扬声器]
```

### 语音合成播放

```mermaid
flowchart LR
    A[API 响应<br/>Base64 MP3] --> B[MP3 解码]
    B --> C[重采样<br/>24K→16K]
    C --> D[混音器]
    D --> E[扬声器]
```

### 音频录制

```mermaid
flowchart LR
    A[PCM 流] --> B[Opus 编码]
    B --> C[OGG 写入器]
    C --> D[文件]
```

## 原生依赖

| 库 | 用途 | 构建系统 |
|---------|---------|--------------|
| libopus | Opus 编解码器 | pkg-config / Bazel |
| libsoxr | 重采样 | pkg-config / Bazel |
| lame | MP3 编码 | Bazel（内置） |
| minimp3 | MP3 解码 | Bazel（内置） |
| libogg | OGG 容器 | pkg-config / Bazel |
| portaudio | 音频 I/O | pkg-config / Bazel |

## 示例目录

- `examples/go/audio/` - Go 音频示例
- `examples/rust/audio/` - Rust 音频示例

## 相关包

- `buffer` - 用于音频数据缓冲
- `speech` - 高级语音合成/识别
- `minimax`、`doubaospeech` - 返回音频的 TTS/ASR API
