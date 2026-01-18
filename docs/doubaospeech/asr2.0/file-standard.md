# 大模型录音文件识别（标准版）

## 概述

适用于录音文件的离线转写，支持长音频。

## 接口列表

| 接口 | 路径 | 方法 | 说明 |
|------|------|------|------|
| 提交任务 | `/api/v1/asr/submit` | POST | 提交转写任务 |
| 查询任务 | `/api/v1/asr/query` | POST/GET | 查询任务状态 |

## 提交任务

### 请求

```
POST https://openspeech.bytedance.com/api/v1/asr/submit
```

### Headers

| 参数 | 必填 | 说明 |
|------|------|------|
| Content-Type | 是 | `application/json` |
| Authorization | 是 | `Bearer;{token}` |

### Body

```json
{
  "appid": "your_appid",
  "reqid": "uuid",
  "audio_url": "https://example.com/audio.mp3",
  "language": "zh-CN",
  "enable_itn": true,
  "enable_punc": true,
  "enable_speaker": true,
  "speaker_count": 2,
  "callback_url": "https://your-server.com/callback"
}
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| appid | string | 是 | 应用 ID |
| reqid | string | 是 | 请求 ID |
| audio_url | string | 是 | 音频文件 URL |
| language | string | 否 | 语言：`zh-CN`/`en-US` |
| enable_itn | bool | 否 | 是否进行逆文本正则化 |
| enable_punc | bool | 否 | 是否恢复标点 |
| enable_speaker | bool | 否 | 是否说话人分离 |
| speaker_count | int | 否 | 说话人数量（启用分离时） |
| callback_url | string | 否 | 回调 URL |

### 响应

```json
{
  "reqid": "uuid",
  "task_id": "task_xxxxx",
  "code": 0,
  "message": "success"
}
```

## 查询任务

### 请求

```
POST https://openspeech.bytedance.com/api/v1/asr/query
```

### Body

```json
{
  "appid": "your_appid",
  "reqid": "uuid"
}
```

### 响应 - 已完成

```json
{
  "reqid": "uuid",
  "task_id": "task_xxxxx",
  "status": "success",
  "code": 0,
  "result": {
    "text": "完整转写文本...",
    "duration": 125000,
    "utterances": [
      {
        "text": "你好",
        "start_time": 0,
        "end_time": 1000,
        "speaker": 0
      },
      {
        "text": "你好，请问有什么可以帮助您？",
        "start_time": 1200,
        "end_time": 3500,
        "speaker": 1
      }
    ]
  }
}
```

## 支持的音频格式

| 格式 | 扩展名 |
|------|--------|
| MP3 | .mp3 |
| WAV | .wav |
| M4A | .m4a |
| FLAC | .flac |
| OGG | .ogg |

## 限制

- 单文件最大 500MB
- 时长最大 5 小时
- 采样率支持 8kHz/16kHz
