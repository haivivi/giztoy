# 异步长文本接口

## 概述

异步长文本接口适用于大文本离线合成场景，支持最大 100,000 字符的文本。

## 接口列表

| 接口 | 路径 | 方法 | 说明 |
|------|------|------|------|
| 提交任务 | `/api/v1/tts_async/submit` | POST | 提交合成任务 |
| 查询任务 | `/api/v1/tts_async/query` | POST/GET | 查询任务状态 |

## 提交任务

### 请求

```
POST https://openspeech.bytedance.com/api/v1/tts_async/submit
```

### Headers

| 参数 | 必填 | 说明 |
|------|------|------|
| Content-Type | 是 | `application/json` |
| Authorization | 是 | `Bearer;{token}` |
| Resource-Id | 是 | 资源 ID |

### Body

```json
{
  "appid": "your_appid",
  "reqid": "unique_request_id",
  "text": "需要合成的长文本内容...",
  "voice_type": "zh_female_cancan",
  "format": "mp3",
  "sample_rate": 24000,
  "speed_ratio": 1.0,
  "volume_ratio": 1.0,
  "pitch_ratio": 1.0,
  "callback_url": "https://your-server.com/callback"
}
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| appid | string | 是 | 应用 ID |
| reqid | string | 是 | 请求 ID，20-64 字符，必须唯一 |
| text | string | 是 | 合成文本，最大 100,000 字符 |
| voice_type | string | 是 | 音色 ID |
| format | string | 否 | 音频格式：`mp3`/`wav`/`ogg_opus`/`pcm` |
| sample_rate | int | 否 | 采样率：8000/16000/22050/24000/44100/48000 |
| speed_ratio | float | 否 | 语速 [0.2, 3.0] |
| volume_ratio | float | 否 | 音量 [0.1, 3.0] |
| pitch_ratio | float | 否 | 音调 [0.1, 3.0] |
| callback_url | string | 否 | 回调 URL |

### 响应

```json
{
  "reqid": "unique_request_id",
  "task_id": "task_xxxxx",
  "code": 0,
  "message": "success"
}
```

---

## 查询任务

### 请求

```
POST https://openspeech.bytedance.com/api/v1/tts_async/query
```

### Body

```json
{
  "appid": "your_appid",
  "reqid": "unique_request_id"
}
```

### 响应 - 进行中

```json
{
  "reqid": "unique_request_id",
  "task_id": "task_xxxxx",
  "status": "running",
  "progress": 50,
  "code": 0,
  "message": "in progress"
}
```

### 响应 - 已完成

```json
{
  "reqid": "unique_request_id",
  "task_id": "task_xxxxx",
  "status": "success",
  "audio_url": "https://xxx.com/audio/xxx.mp3",
  "audio_duration": 125000,
  "audio_size": 2000000,
  "code": 0,
  "message": "success"
}
```

## 任务状态

| 状态 | 说明 |
|------|------|
| submitted | 已提交 |
| running | 处理中 |
| success | 成功 |
| failed | 失败 |

## 回调通知

如果设置了 `callback_url`，任务完成后会 POST：

```json
{
  "reqid": "unique_request_id",
  "task_id": "task_xxxxx",
  "status": "success",
  "audio_url": "https://xxx.com/audio/xxx.mp3",
  "audio_duration": 125000,
  "timestamp": 1704067200
}
```
