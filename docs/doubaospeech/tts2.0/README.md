# 大模型语音合成 2.0

## 原始文档链接

| 文档 | 链接 |
|------|------|
| 产品简介 | https://www.volcengine.com/docs/6561/1234523 |
| 能力介绍 | https://www.volcengine.com/docs/6561/1257584 |
| 单向流式 HTTP V3 | https://www.volcengine.com/docs/6561/1598757 |
| 单向流式 WebSocket V3 | https://www.volcengine.com/docs/6561/1719100 |
| 双向流式 WebSocket V3 | https://www.volcengine.com/docs/6561/1329505 |
| 异步长文本 | https://www.volcengine.com/docs/6561/1330194 |
| 音色列表 | https://www.volcengine.com/docs/6561/1257544 |
| SSML标记语言 | https://www.volcengine.com/docs/6561/1257543 |

## 概述

大模型语音合成 2.0 是基于大模型的语音合成服务，相比 1.0 版本具有更自然的音质和更丰富的功能。

## 接口列表

| 接口 | 端点 | Resource ID | 特点 |
|------|------|-------------|------|
| 单向流式 HTTP | `POST /api/v3/tts/unidirectional` | `seed-tts-2.0` | 简单易用，流式输出 |
| 单向流式 WebSocket | `WSS /api/v3/tts/unidirectional` | `seed-tts-2.0` | 低延迟流式 |
| 双向流式 WebSocket | `WSS /api/v3/tts/bidirection` | `seed-tts-2.0` | 实时交互 |
| 异步长文本 | `POST /api/v3/tts/async/submit` | `seed-tts-2.0-concurr` | 大文本离线合成 |

## 认证方式

### Headers 认证（推荐）

| Header | 说明 | 示例 |
|--------|------|------|
| `X-Api-App-Id` | APP ID | `123456789` |
| `X-Api-Access-Key` | Access Token | `your-token` |
| `X-Api-Resource-Id` | 资源 ID | `seed-tts-2.0` |
| `X-Api-Connect-Id` | 连接 ID（可选）| `uuid-xxx` |

### Resource ID 说明

| Resource ID | 说明 |
|-------------|------|
| `seed-tts-1.0` | 大模型 1.0（字符版）|
| `seed-tts-1.0-concurr` | 大模型 1.0（并发版）|
| `seed-tts-2.0` | 大模型 2.0（字符版）|
| `seed-tts-2.0-concurr` | 大模型 2.0（并发版）|
| `seed-icl-1.0` | 声音复刻 1.0（字符版）|
| `seed-icl-2.0` | 声音复刻 2.0（字符版）|

## 功能特性

- ✅ 大模型音色，效果更自然
- ✅ 支持声音复刻（ICL）
- ✅ 支持混音（Mix）- 最多 3 个音色
- ✅ 支持 SSML 标记
- ✅ 支持情感控制
- ✅ 支持多语种

## 请求参数

### 通用结构

```json
{
    "user": {
        "uid": "user-id"
    },
    "req_params": {
        "text": "要合成的文本",
        "speaker": "zh_female_cancan",
        "audio_params": {
            "format": "mp3",
            "sample_rate": 24000
        }
    }
}
```

### req_params 字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `text` | string | ✅ | 合成文本 |
| `speaker` | string | ✅ | 发音人 ID |
| `audio_params` | object | ✅ | 音频参数 |
| `audio_params.format` | string | | 格式：mp3/ogg_opus/pcm |
| `audio_params.sample_rate` | int | | 采样率：8000-48000 |
| `audio_params.speech_rate` | int | | 语速：-50 到 100 |
| `audio_params.loudness_rate` | int | | 音量：-50 到 100 |
| `audio_params.emotion` | string | | 情感：angry/happy/sad 等 |
| `model` | string | | 模型版本：seed-tts-1.1 |
| `mix_speaker` | object | | 混音配置（见下文）|

### 混音配置

使用混音时，`speaker` 必须设置为 `custom_mix_bigtts`：

```json
{
    "req_params": {
        "speaker": "custom_mix_bigtts",
        "mix_speaker": {
            "speakers": [
                {"source_speaker": "zh_male_xxx", "mix_factor": 0.3},
                {"source_speaker": "zh_female_xxx", "mix_factor": 0.7}
            ]
        }
    }
}
```

## 响应格式

### 音频响应

```json
{
    "code": 0,
    "message": "",
    "data": "base64-encoded-audio"
}
```

### 文本响应（含时间戳）

```json
{
    "code": 0,
    "message": "",
    "data": null,
    "sentence": {
        "text": "合成文本",
        "words": [
            {"word": "合", "startTime": 0.1, "endTime": 0.2, "confidence": 0.95}
        ]
    }
}
```

### 结束响应

```json
{
    "code": 20000000,
    "message": "ok",
    "data": null,
    "usage": {"text_words": 10}
}
```

## 错误码

| Code | Message | 说明 |
|------|---------|------|
| 20000000 | ok | 合成结束成功 |
| 40402003 | TTSExceededTextLimit | 文本长度超限 |
| 45000000 | speaker permission denied | 音色未授权 |
| 55000000 | 服务端错误 | 通用错误 |

## 详细文档

- [单向流式 HTTP V3](./stream-http.md)
- [单向流式 WebSocket V3](./stream-ws.md)
- [双向流式 WebSocket V3](./duplex-ws.md)
- [异步长文本](./async.md)
- [音色列表](./voices.md)
