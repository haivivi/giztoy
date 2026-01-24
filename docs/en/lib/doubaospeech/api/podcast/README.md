# 语音播客大模型

## 原始文档链接

| 文档 | 链接 |
|------|------|
| 产品简介 | https://www.volcengine.com/docs/6561/xxx |
| 播客API-websocket-v3协议 | https://www.volcengine.com/docs/6561/1668014 |

## 概述

语音播客大模型支持生成多人对话式播客音频，适合生成新闻播报、故事叙述、对话节目等内容。

## 接口信息

| 项目 | 值 |
|------|------|
| 端点 | `WSS wss://openspeech.bytedance.com/api/v3/sami/podcasttts` |
| Resource ID | `volc.megatts.podcast` |
| 协议 | WebSocket V3 二进制协议 |

## 认证方式

### Headers

| Header | 说明 | 必填 |
|--------|------|------|
| `X-Api-App-Id` | APP ID | ✅ |
| `X-Api-Access-Key` | Access Token | ✅ |
| `X-Api-Resource-Id` | `volc.megatts.podcast` | ✅ |

## 请求格式

### Action 类型

| Action | 说明 |
|--------|------|
| 1 | Script（剧本生成）|
| 2 | NLP（对话生成）|
| 3 | TTS（文本转语音）|

### TTS 模式请求（Action=3）

```json
{
    "action": 3,
    "speaker_info": {
        "speaker1": "zh_female_cancan",
        "speaker2": "zh_male_yangguang"
    },
    "nlp_texts": [
        {"role": "speaker1", "text": "大家好，欢迎收听今天的节目。"},
        {"role": "speaker2", "text": "今天我们要聊一个有趣的话题。"}
    ],
    "audio_config": {
        "format": "mp3",
        "sample_rate": 24000
    }
}
```

### 剧本模式请求（Action=1）

```json
{
    "action": 1,
    "script": "这是一段关于人工智能的科普内容...",
    "speaker_info": {
        "host": "zh_female_cancan",
        "guest": "zh_male_yangguang"
    },
    "audio_config": {
        "format": "mp3"
    }
}
```

## 响应格式

### 音频数据

```json
{
    "event": 352,
    "data": "base64-encoded-audio"
}
```

### 完成响应

```json
{
    "event": 102,
    "message": "ok"
}
```

## 二进制协议

与实时对话 API 使用相同的 V3 二进制协议，详见 [realtime/api.md](../realtime/api.md)

## 支持的音色

播客支持所有 TTS 2.0 大模型音色，推荐使用多情感音色以获得更好的效果。

## 详细文档

- [API 接口详情](./api.md)
