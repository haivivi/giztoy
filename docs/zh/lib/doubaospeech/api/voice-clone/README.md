# 声音复刻

## 原始文档链接

| 文档 | 链接 |
|------|------|
| 声音复刻API | https://www.volcengine.com/docs/6561/1305191 |
| 下单及使用指南 | https://www.volcengine.com/docs/6561/1829010 |
| 最佳实践 | https://www.volcengine.com/docs/6561/xxx |

## 概述

声音复刻服务允许用户上传音频样本，训练出定制化的音色，用于语音合成。

## 接口列表

| 接口 | 端点 | Cluster | 说明 |
|------|------|---------|------|
| 上传训练音频 | `POST /api/v1/mega_tts/audio/upload` | `volcano_icl` | 上传音频 |
| 查询训练状态 | `POST /api/v1/mega_tts/status` | `volcano_icl` | 查询进度 |
| 激活音色 | `POST /api/v1/mega_tts/audio/activate` | `volcano_icl` | 激活使用 |

## 认证方式

### V1 API

**Header 认证：**

```http
Authorization: Bearer; {token}
```

**请求体认证：**

```json
{
    "appid": "{appid}",
    "token": "{token}",
    "cluster": "volcano_icl"
}
```

## 训练流程

### 1. 上传训练音频

```
POST https://openspeech.bytedance.com/api/v1/mega_tts/audio/upload
```

**请求：**

```json
{
    "appid": "123456789",
    "token": "your-token",
    "cluster": "volcano_icl",
    "audio_format": "mp3",
    "audio_data": "base64-encoded-audio",
    "speaker_id": "S_custom_voice_001",
    "language": 0,
    "model_type": 1
}
```

**参数说明：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `appid` | string | ✅ | APP ID |
| `token` | string | ✅ | Access Token |
| `cluster` | string | ✅ | `volcano_icl` |
| `audio_format` | string | ✅ | 音频格式：`mp3`/`wav`/`pcm` |
| `audio_data` | string | ✅ | Base64 编码的音频数据 |
| `speaker_id` | string | ✅ | 自定义音色 ID，以 `S_` 开头 |
| `language` | int | | 语言：0=中文，1=英文 |
| `model_type` | int | | 模型类型：1=标准，2=Pro |

### 2. 查询训练状态

```
POST https://openspeech.bytedance.com/api/v1/mega_tts/status
```

**请求：**

```json
{
    "appid": "123456789",
    "token": "your-token",
    "cluster": "volcano_icl",
    "speaker_id": "S_custom_voice_001"
}
```

**响应：**

```json
{
    "BaseResp": {
        "StatusCode": 0,
        "StatusMessage": "success"
    },
    "speaker_id": "S_custom_voice_001",
    "status": 1,
    "demo_audio": "base64-audio"
}
```

**状态码：**

| status | 说明 |
|--------|------|
| 0 | 处理中 |
| 1 | 训练成功 |
| 2 | 训练失败 |
| 3 | 待激活 |

### 3. 激活音色

```
POST https://openspeech.bytedance.com/api/v1/mega_tts/audio/activate
```

**请求：**

```json
{
    "appid": "123456789",
    "token": "your-token",
    "cluster": "volcano_icl",
    "speaker_id": "S_custom_voice_001"
}
```

## 使用复刻音色

训练完成后，复刻音色可用于 TTS 合成：

**V1 API（volcano_icl cluster）：**

```json
{
    "app": {
        "appid": "123456789",
        "token": "your-token",
        "cluster": "volcano_icl"
    },
    "audio": {
        "voice_type": "S_custom_voice_001"
    },
    "request": {
        "text": "使用复刻音色合成文本"
    }
}
```

**V3 API（需要使用 icl_ 前缀）：**

```json
{
    "req_params": {
        "speaker": "icl_S_custom_voice_001",
        "text": "使用复刻音色合成文本"
    }
}
```

## 音频要求

| 项目 | 要求 |
|------|------|
| 时长 | 10-60 秒（推荐 30 秒以上）|
| 格式 | MP3/WAV/PCM |
| 采样率 | 16000 Hz 以上 |
| 内容 | 清晰语音，无背景音乐/噪音 |
| 语言 | 单一语言，与 language 参数匹配 |

## 最佳实践

1. **音频质量**：使用高质量麦克风录制，避免环境噪音
2. **语速稳定**：保持正常语速，避免过快或过慢
3. **情感中性**：使用中性情感录制，除非需要特定情感
4. **多样性**：录制内容包含多种句式和音节

## 详细文档

- [API 接口详情](./api.md)
