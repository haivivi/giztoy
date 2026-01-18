# 异步长文本语音合成 API (T2A Async)

> **官方文档**: https://platform.minimaxi.com/docs/api-reference/speech-t2a-async

## 概述

异步长文本语音合成 API 支持处理最长 **1,000,000 字符** 的文本，采用异步任务模式。

## 支持的模型

| 模型 | 特性 |
|------|------|
| speech-2.6-hd | 最新 HD 模型，韵律表现出色 |
| speech-2.6-turbo | 最新 Turbo 模型，超低时延 |
| speech-02-hd | 出色的韵律和稳定性 |
| speech-02-turbo | 小语种能力加强 |

## 功能特性

- 支持最长 1,000,000 字符文本输入
- 支持上传文本文件（file_id）或直接传入文本
- 支持 100+ 系统音色和复刻音色
- 支持返回时间戳（字幕）
- 支持非法字符检测（超过 10% 报错）
- 生成结果通过文件管理 API 下载

## 接口说明

异步语音合成包含 2 个接口：

1. **创建异步语音合成任务** - 提交任务，获取 task_id
2. **查询语音生成任务状态** - 查询任务状态，获取 file_id

## 创建异步语音合成任务

### 端点

```
POST https://api.minimaxi.com/v1/t2a_async
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型版本 |
| text | string | 否* | 需要合成的文本，最长 1,000,000 字符 |
| file_id | string | 否* | 文本文件的 file_id |
| voice_setting | object | 否 | 语音设置 |
| audio_setting | object | 否 | 音频设置 |
| pronunciation_dict | object | 否 | 发音词典 |
| language_boost | string | 否 | 语言增强 |
| subtitle_enable | boolean | 否 | 是否开启字幕，默认 `false` |

> **注意**: `text` 和 `file_id` 二选一，必须提供其中一个

### voice_setting 对象

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| voice_id | string | - | 音色 ID |
| speed | float | 1.0 | 语速 (0.5-2.0) |
| vol | float | 1.0 | 音量 (0-10) |
| pitch | int | 0 | 音调 (-12 到 12) |
| emotion | string | - | 情绪 |

### audio_setting 对象

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| sample_rate | int | 32000 | 采样率 |
| bitrate | int | 128000 | 比特率 |
| format | string | mp3 | 音频格式：mp3, pcm, flac, wav |
| channel | int | 1 | 声道数 |

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/t2a_async \
  --header 'Authorization: Bearer <your_api_key>' \
  --header 'Content-Type: application/json' \
  --data '{
    "model": "speech-2.6-hd",
    "text": "这是一段很长的文本...",
    "voice_setting": {
      "voice_id": "male-qn-qingse",
      "speed": 1
    },
    "audio_setting": {
      "format": "mp3"
    },
    "subtitle_enable": true
  }'
```

### 响应格式

```json
{
  "task_id": "abc123456",
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

## 查询语音生成任务状态

### 端点

```
GET https://api.minimaxi.com/v1/t2a_async/{task_id}
```

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| task_id | string | 是 | 任务 ID |

### 请求示例

```bash
curl --request GET \
  --url https://api.minimaxi.com/v1/t2a_async/abc123456 \
  --header 'Authorization: Bearer <your_api_key>'
```

### 响应格式

```json
{
  "task_id": "abc123456",
  "status": "Success",
  "file_id": "file_xyz789",
  "extra_info": {
    "audio_length": 120000,
    "audio_sample_rate": 32000,
    "audio_size": 1920000,
    "bitrate": 128000,
    "word_count": 5000,
    "usage_characters": 5000,
    "audio_format": "mp3",
    "audio_channel": 1
  },
  "subtitle": {
    "segments": [
      {
        "start_time": 0,
        "end_time": 1500,
        "text": "这是第一句话"
      }
    ]
  },
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

### 任务状态

| 状态 | 说明 |
|------|------|
| Pending | 任务等待中 |
| Processing | 任务处理中 |
| Success | 任务完成 |
| Failed | 任务失败 |

## 下载音频文件

任务完成后，使用返回的 `file_id` 通过文件管理 API 下载：

```bash
curl --request GET \
  --url "https://api.minimaxi.com/v1/files/{file_id}/content" \
  --header 'Authorization: Bearer <your_api_key>' \
  --output output.mp3
```

## 注意事项

1. 返回的文件下载 URL 有效期为 **9 小时**（32,400 秒）
2. 非法字符比例超过 10% 时，接口会报错
3. 支持通过 `file_id` 上传文本文件，适合处理超长文本
4. 字幕功能返回每段文本的时间戳信息
