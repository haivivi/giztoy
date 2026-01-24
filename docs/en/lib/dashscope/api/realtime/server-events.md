# 服务端事件

## 原始文档

- https://help.aliyun.com/zh/model-studio/server-events

> 如果本文档信息不完整，请访问上述链接获取最新内容。

---

## 概述

服务端事件是服务器发送给客户端的 WebSocket 消息，用于：
- 确认客户端操作
- 返回模型响应
- 报告错误

## 事件列表

### 会话相关

| 事件 | 说明 |
|------|------|
| session.created | 会话创建完成 |
| session.updated | 会话配置更新完成 |

### 输入缓冲区相关

| 事件 | 说明 |
|------|------|
| input_audio_buffer.committed | 音频缓冲区已提交 |
| input_audio_buffer.cleared | 音频缓冲区已清除 |
| input_audio_buffer.speech_started | VAD 检测到语音开始 |
| input_audio_buffer.speech_stopped | VAD 检测到语音结束 |

### 响应相关

| 事件 | 说明 |
|------|------|
| response.created | 响应创建开始 |
| response.done | 响应完成 |
| response.cancelled | 响应已取消 |
| response.audio.delta | 音频数据增量 |
| response.audio.done | 音频输出完成 |
| response.text.delta | 文本数据增量 |
| response.text.done | 文本输出完成 |

### 对话相关

| 事件 | 说明 |
|------|------|
| conversation.item.created | 对话项创建 |
| response.content_part.added | 内容部分添加 |

### 错误相关

| 事件 | 说明 |
|------|------|
| error | 错误事件 |

---

## session.created

会话创建完成时发送。

### 响应示例

```json
{
  "type": "session.created",
  "event_id": "event_xxx",
  "session": {
    "id": "session_xxx",
    "modalities": ["text", "audio"],
    "voice": "Chelsie",
    "input_audio_format": "pcm16",
    "output_audio_format": "pcm16"
  }
}
```

---

## session.updated

会话配置更新完成时发送（响应 `session.update`）。

### 响应示例

```json
{
  "type": "session.updated",
  "event_id": "event_xxx",
  "session": {
    "id": "session_xxx",
    "modalities": ["text", "audio"],
    "voice": "Chelsie",
    "instructions": "你是一个AI助手...",
    "turn_detection": {
      "type": "server_vad",
      "threshold": 0.5,
      "silence_duration_ms": 800
    }
  }
}
```

---

## input_audio_buffer.speech_started

VAD 检测到语音开始时发送（仅 VAD 模式）。

### 响应示例

```json
{
  "type": "input_audio_buffer.speech_started",
  "event_id": "event_xxx",
  "audio_start_ms": 1500
}
```

---

## input_audio_buffer.speech_stopped

VAD 检测到语音结束时发送（仅 VAD 模式）。

### 响应示例

```json
{
  "type": "input_audio_buffer.speech_stopped",
  "event_id": "event_xxx",
  "audio_end_ms": 3200
}
```

---

## input_audio_buffer.committed

音频缓冲区提交完成时发送。

### 响应示例

```json
{
  "type": "input_audio_buffer.committed",
  "event_id": "event_xxx",
  "item_id": "item_xxx"
}
```

---

## response.created

模型开始生成响应时发送。

### 响应示例

```json
{
  "type": "response.created",
  "event_id": "event_xxx",
  "response": {
    "id": "response_xxx",
    "status": "in_progress"
  }
}
```

---

## response.audio.delta

音频数据增量，包含 Base64 编码的音频片段。

### 响应示例

```json
{
  "type": "response.audio.delta",
  "event_id": "event_xxx",
  "response_id": "response_xxx",
  "item_id": "item_xxx",
  "content_index": 0,
  "delta": "UklGR..."
}
```

| 字段 | 说明 |
|------|------|
| delta | Base64 编码的音频数据 |
| content_index | 内容索引 |

---

## response.text.delta

文本数据增量。

### 响应示例

```json
{
  "type": "response.text.delta",
  "event_id": "event_xxx",
  "response_id": "response_xxx",
  "item_id": "item_xxx",
  "content_index": 0,
  "delta": "你好"
}
```

---

## response.done

响应生成完成时发送。

### 响应示例

```json
{
  "type": "response.done",
  "event_id": "event_xxx",
  "response": {
    "id": "response_xxx",
    "status": "completed",
    "usage": {
      "input_tokens": 100,
      "output_tokens": 50,
      "total_tokens": 150
    }
  }
}
```

---

## error

错误事件。

### 响应示例

```json
{
  "type": "error",
  "event_id": "event_xxx",
  "error": {
    "type": "invalid_request_error",
    "code": "invalid_value",
    "message": "Invalid value for parameter 'voice'"
  }
}
```

### 错误类型

| 类型 | 说明 |
|------|------|
| invalid_request_error | 请求参数无效 |
| authentication_error | 认证失败 |
| rate_limit_error | 超出速率限制 |
| server_error | 服务器内部错误 |
