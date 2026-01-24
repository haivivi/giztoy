# 一句话识别

## 原始文档

- **一句话识别**: https://www.volcengine.com/docs/6561/80818

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 概述

一句话识别适用于短音频场景，音频时长不超过 60 秒。

## 接口信息

| 项目 | 值 |
|------|------|
| 请求地址 | `https://openspeech.bytedance.com/api/v1/asr` |
| 请求方式 | POST |

## 请求参数

### Headers

| 参数 | 必填 | 说明 |
|------|------|------|
| Content-Type | 是 | `application/json` |
| Authorization | 是 | `Bearer;{token}` |

### Body

```json
{
  "app": {
    "appid": "your_appid",
    "cluster": "volcengine_streaming_common"
  },
  "user": {
    "uid": "user_001"
  },
  "audio": {
    "format": "mp3",
    "url": "https://example.com/audio.mp3"
  },
  "request": {
    "reqid": "uuid",
    "language": "zh-CN",
    "enable_itn": true,
    "enable_punc": true
  }
}
```

或使用 base64 音频：

```json
{
  "audio": {
    "format": "mp3",
    "data": "<base64编码的音频>"
  }
}
```

## 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| format | string | 是 | 音频格式：`mp3`/`wav`/`pcm` |
| url | string | 否 | 音频 URL（与 data 二选一） |
| data | string | 否 | Base64 音频（与 url 二选一） |
| language | string | 否 | 语言代码 |
| enable_itn | bool | 否 | 逆文本正则化 |
| enable_punc | bool | 否 | 标点恢复 |

## 响应格式

```json
{
  "reqid": "xxx",
  "code": 1000,
  "message": "success",
  "result": {
    "text": "你好，世界！",
    "duration": 2500
  }
}
```
