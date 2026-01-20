# 端到端实时语音大模型

## 原始文档链接

| 文档 | 链接 |
|------|------|
| 产品简介 | https://www.volcengine.com/docs/6561/xxx |
| API接入文档 | https://www.volcengine.com/docs/6561/1257584 |
| Android SDK | https://www.volcengine.com/docs/6561/xxx |
| iOS SDK | https://www.volcengine.com/docs/6561/xxx |

## 概述

端到端实时语音大模型提供实时语音交互能力，支持语音输入→大模型处理→语音输出的完整流程。

## 接口信息

| 项目 | 值 |
|------|------|
| 端点 | `WSS wss://openspeech.bytedance.com/api/v3/realtime/dialogue` |
| Resource ID | `volc.speech.dialog` |
| 协议 | WebSocket 二进制协议 |

## 认证方式

### Headers

| Header | 说明 | 必填 |
|--------|------|------|
| `X-Api-App-Id` | APP ID | ✅ |
| `X-Api-App-Key` | App Key | ✅ |
| `X-Api-Access-Key` | Access Token | ✅ |
| `X-Api-Resource-Id` | `volc.speech.dialog` | ✅ |
| `X-Api-Request-Id` | 请求 ID | |
| `X-Api-Connect-Id` | 连接 ID | |

## 交互流程

### 1. 建立连接

```go
header := http.Header{
    "X-Api-App-Id":      []string{appID},
    "X-Api-App-Key":     []string{appKey},
    "X-Api-Access-Key":  []string{token},
    "X-Api-Resource-Id": []string{"volc.speech.dialog"},
}
conn, _, _ := websocket.DefaultDialer.Dial(url, header)
```

### 2. 事件流程

```
Client                                Server
   |                                     |
   |--- StartConnection (1) ------------>|
   |<--- ConnectionStarted (50) ---------|
   |                                     |
   |--- StartSession (100) ------------->|
   |<--- SessionStarted (150) -----------|
   |                                     |
   |--- SendText/Audio (300/200) ------->|
   |<--- TTSAudioData (352) -------------|
   |<--- TTSSegmentEnd (351) ------------|
   |                                     |
   |--- FinishSession (102) ------------>|
   |<--- SessionFinish (154) ------------|
   |                                     |
   |--- FinishConnection (2) ----------->|
   |                                     |
```

## 事件定义

### 客户端事件

| Event ID | 名称 | 说明 |
|----------|------|------|
| 1 | StartConnection | 开始连接 |
| 2 | FinishConnection | 结束连接 |
| 100 | StartSession | 开始会话 |
| 102 | FinishSession | 结束会话 |
| 200 | AudioData | 发送音频数据 |
| 300 | SayHello | 触发初始 TTS |
| 500 | TTSText | 流式发送 TTS 文本 |
| 501 | UserText | 用户文本输入 |

### 服务端事件

| Event ID | 名称 | 说明 |
|----------|------|------|
| 50 | ConnectionStarted | 连接已建立 |
| 51 | ConnectionFailed | 连接失败 |
| 150 | SessionStarted | 会话已开始 |
| 153 | SessionFailed | 会话失败 |
| 154 | SessionFinish | 会话结束 |
| 250 | GenText | LLM 生成文本 |
| 251 | GenTextEnd | LLM 文本结束 |
| 350 | TTSSegmentStart | TTS 段开始 |
| 351 | TTSSegmentEnd | TTS 段结束 |
| 352 | TTSAudioData | TTS 音频数据 |
| 359 | TTSEnd | TTS 结束 |

## 二进制协议

### 消息格式

```
+--------+--------+--------+--------+--------+--------+
| 4 bits | 4 bits | 8 bits | 8 bits | 4 bytes| Payload|
| Version| HdrSz  | MsgType| Flags  | EventID|  ...   |
+--------+--------+--------+--------+--------+--------+
```

### Header 字段

| 字段 | 位置 | 说明 |
|------|------|------|
| Version | Byte[0] >> 4 | 协议版本（1）|
| HeaderSize | Byte[0] & 0x0F | Header 大小（4字节单位）|
| MessageType | Byte[1] | 消息类型 |
| Flags | Byte[2] | 标志位 |
| EventID | Byte[4:8] | 事件 ID |

### 消息类型

| MsgType | 说明 |
|---------|------|
| 0x01 | Full Client Request |
| 0x02 | Audio Only Client |
| 0x09 | Full Server Response |
| 0x0B | Audio Only Server |
| 0x0F | Error Response |

### 标志位

| Flag | 说明 |
|------|------|
| 0x01 | GZIP 压缩 |
| 0x02 | 最后一帧 |
| 0x04 | 包含 Event ID |

## 会话配置

### StartSession 请求

```json
{
    "event": 100,
    "req_params": {
        "mode": "standard",
        "audio_config": {
            "input": {
                "encoding": "pcm",
                "sample_rate": 16000
            },
            "output": {
                "encoding": "ogg_opus",
                "sample_rate": 24000
            }
        },
        "asr_config": {
            "language": "zh-CN"
        },
        "tts_config": {
            "speaker": "zh_female_cancan"
        },
        "llm_config": {
            "model": "doubao-pro",
            "system_prompt": "你是一个智能助手"
        }
    }
}
```

## 详细文档

- [API 接口详情](./api.md)
