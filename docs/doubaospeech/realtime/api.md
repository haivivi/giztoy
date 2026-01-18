# 端到端实时语音大模型 API

## 原始文档

- **API接入文档**: https://www.volcengine.com/docs/6561/1422282

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 接口信息

| 项目 | 值 |
|------|------|
| 请求地址 | `wss://openspeech.bytedance.com/api/v3/saas/chat` |
| 协议 | WebSocket |

## 认证

连接时通过 URL 参数传递认证信息：

```
wss://openspeech.bytedance.com/api/v3/saas/chat?appid=xxx&token=xxx&cluster=xxx
```

## 消息类型

### 客户端消息

| 类型 | 说明 |
|------|------|
| start | 开始会话 |
| audio | 发送音频数据 |
| text | 发送文本（可选） |
| finish | 结束会话 |
| cancel | 取消当前响应 |

### 服务端消息

| 类型 | 说明 |
|------|------|
| audio | 返回音频数据 |
| text | 返回文本（ASR/响应） |
| status | 状态更新 |
| error | 错误信息 |

## 开始会话

```json
{
  "type": "start",
  "data": {
    "session_id": "uuid",
    "config": {
      "asr": {
        "language": "zh-CN",
        "enable_punctuation": true
      },
      "tts": {
        "voice_type": "zh_female_cancan",
        "speed_ratio": 1.0
      },
      "llm": {
        "model": "doubao-chat",
        "system_prompt": "你是一个智能助手"
      }
    }
  }
}
```

## 发送音频

以二进制帧发送 PCM 音频数据：

- 格式：PCM
- 采样率：16000Hz
- 位深：16bit
- 声道：单声道

## 响应格式

### 文本响应（ASR 结果）

```json
{
  "type": "text",
  "data": {
    "role": "user",
    "content": "你好",
    "is_final": true
  }
}
```

### 文本响应（LLM 结果）

```json
{
  "type": "text",
  "data": {
    "role": "assistant",
    "content": "你好！有什么可以帮助你的吗？",
    "is_final": false
  }
}
```

### 音频响应

```json
{
  "type": "audio",
  "data": {
    "audio": "<base64编码的音频>",
    "sequence": 1
  }
}
```

## 状态流程

```
客户端                              服务端
   |                                   |
   |--- start (配置) ----------------->|
   |<-- status (ready) ----------------|
   |                                   |
   |--- audio (用户语音) ------------->|
   |<-- text (ASR结果) ----------------|
   |                                   |
   |<-- text (LLM响应，流式) ----------|
   |<-- audio (TTS音频，流式) ---------|
   |<-- status (turn_end) -------------|
   |                                   |
   |--- finish ----------------------->|
   |<-- status (session_end) ----------|
```
