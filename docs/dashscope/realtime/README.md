# 实时多模态 API (Qwen-Omni-Realtime)

## 原始文档

| 文档 | 链接 |
|------|------|
| 实时多模态概述 | https://help.aliyun.com/zh/model-studio/realtime |
| 客户端事件 | https://help.aliyun.com/zh/model-studio/client-events |
| 服务端事件 | https://help.aliyun.com/zh/model-studio/server-events |
| 音色列表 | https://help.aliyun.com/zh/model-studio/realtime#f9c68d860a3rs |

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 概述

Qwen-Omni-Realtime API 是阿里云百炼提供的实时多模态对话 API，支持：

- **语音输入/输出** - 实时语音对话
- **图像输入** - 支持发送图片进行视觉理解
- **文本输出** - 可选输出文本

基于 WebSocket 协议，支持双向实时通信。

## ⚠️ 与 OpenAI Realtime API 的关键差异

DashScope Realtime API 虽然事件格式类似 OpenAI，但有重要差异：

| 特性 | DashScope | OpenAI Realtime |
|------|-----------|-----------------|
| **response.create** | 必须包含顶层 `messages` 字段 | 使用已提交的音频/文本 |
| **响应格式** | `choices` 数组格式 | `response.text.delta` 等事件 |
| **instructions** | 仅用于角色设定，不作为 prompt | 可作为 system prompt |

### 正确的 response.create 格式

```json
{
  "event_id": "evt_1",
  "type": "response.create",
  "messages": [
    {"role": "system", "content": "你是一个AI助手"},
    {"role": "user", "content": "你好"}
  ]
}
```

### 响应格式（choices）

```json
{
  "choices": [{
    "finish_reason": "null",
    "message": {
      "role": "assistant",
      "content": [
        {"text": "你好"},
        {"audio": {"data": "base64..."}}
      ]
    }
  }]
}
```

## 支持的模型

| 模型 | 输出音频格式 | 默认音色 | 特点 |
|------|-------------|---------|------|
| Qwen3-Omni-Flash-Realtime | pcm24 | Cherry | 快速响应 |
| Qwen-Omni-Turbo-Realtime | pcm16 | Chelsie | 高质量 |

## WebSocket 连接

### 端点

```
wss://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-realtime/realtime
```

### 连接示例

```go
import "github.com/gorilla/websocket"

dialer := websocket.Dialer{}
header := http.Header{}
header.Set("Authorization", "Bearer "+apiKey)

conn, _, err := dialer.Dial(endpoint, header)
```

## 交互流程

```
Client                                  Server
  |                                        |
  |------ session.update ----------------->|  1. 配置会话
  |<----- session.created/updated ---------|
  |                                        |
  |------ input_audio_buffer.append ------>|  2. 发送音频
  |------ input_audio_buffer.append ------>|
  |                                        |
  |       (VAD 检测到语音结束)              |
  |<----- input_audio_buffer.speech_started|
  |<----- input_audio_buffer.speech_stopped|
  |<----- input_audio_buffer.committed ----|
  |                                        |
  |<----- response.created ----------------|  3. 模型响应
  |<----- response.audio.delta ------------|
  |<----- response.audio.delta ------------|
  |<----- response.done -------------------|
  |                                        |
```

## VAD 模式 vs 手动模式

### VAD 模式（默认）

服务端自动检测语音活动（Voice Activity Detection）：
- 自动检测语音开始/结束
- 自动提交音频缓冲区
- 自动创建模型响应

配置示例：
```json
{
  "turn_detection": {
    "type": "server_vad",
    "threshold": 0.5,
    "silence_duration_ms": 800
  }
}
```

### 手动模式

设置 `turn_detection: null` 禁用 VAD：
- 客户端需手动调用 `input_audio_buffer.commit`
- 客户端需手动调用 `response.create`

## 音频格式

### 输入音频
- 格式：pcm16（16-bit PCM）
- 采样率：根据模型要求
- 编码：Base64

### 输出音频
- Qwen3-Omni-Flash-Realtime：pcm24
- Qwen-Omni-Turbo-Realtime：pcm16

## 图像输入

支持实时发送图像（视频帧）：
- 格式：JPG/JPEG
- 分辨率：建议 480p-720p，最高 1080p
- 大小：< 500KB（Base64 编码前）
- 频率：建议 1 张/秒

## 详细文档

- [客户端事件](./client-events.md)
- [服务端事件](./server-events.md)
