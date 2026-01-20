# 大模型语音识别 2.0

## 原始文档链接

| 文档 | 链接 |
|------|------|
| 大模型流式语音识别 | https://www.volcengine.com/docs/6561/1354869 |
| 大模型录音文件识别标准版 | https://www.volcengine.com/docs/6561/1354868 |
| 大模型录音文件极速版 | https://www.volcengine.com/docs/6561/1631584 |
| 大模型录音文件闲时版 | https://www.volcengine.com/docs/6561/1840838 |

## 概述

大模型语音识别 2.0 基于大模型技术，提供更准确的语音识别能力。

## 接口列表

| 接口 | 端点 | Resource ID | 特点 |
|------|------|-------------|------|
| 流式识别 | `WSS /api/v3/sauc/bigmodel` | `volc.bigasr.sauc.duration` | 实时识别 |
| 录音文件标准版 | `POST /api/v3/asr/bigmodel/submit` | `volc.bigasr.auc.duration` | 准确度优先 |
| 录音文件极速版 | `POST /api/v3/asr/bigmodel_async/submit` | `volc.bigasr.auc.duration` | 速度优先 |
| 录音文件闲时版 | `POST /api/v3/asr/bigmodel_idle/submit` | `volc.bigasr.auc.duration` | 成本优先 |

## 认证方式

### V3 Headers 认证

| Header | 说明 | 必填 |
|--------|------|------|
| `X-Api-App-Id` | APP ID | ✅ |
| `X-Api-Access-Key` | Access Token | ✅ |
| `X-Api-Resource-Id` | 资源 ID | ✅ |

### Resource ID

| Resource ID | 说明 |
|-------------|------|
| `volc.bigasr.sauc.duration` | 大模型流式识别 |
| `volc.bigasr.auc.duration` | 大模型录音文件识别 |

## 流式识别 API

### 端点

```
WSS wss://openspeech.bytedance.com/api/v3/sauc/bigmodel
```

### 请求格式

使用二进制协议发送配置和音频数据。

### 配置请求

```json
{
    "user": {"uid": "user-id"},
    "audio": {
        "format": "pcm",
        "sample_rate": 16000,
        "channel": 1,
        "bits": 16
    },
    "request": {
        "reqid": "unique-request-id",
        "sequence": 1,
        "language": "zh-CN",
        "show_utterances": true,
        "result_type": "single"
    }
}
```

### 响应格式

```json
{
    "reqid": "request-id",
    "code": 1000,
    "sequence": 1,
    "result": {
        "text": "识别结果",
        "utterances": [
            {
                "text": "识别结果",
                "start_time": 0,
                "end_time": 1500,
                "words": [...]
            }
        ]
    }
}
```

## 录音文件识别 API

### 端点

```
POST https://openspeech.bytedance.com/api/v3/asr/bigmodel/submit
```

### 请求

```json
{
    "user": {"uid": "user-id"},
    "audio": {
        "format": "mp3",
        "url": "https://example.com/audio.mp3"
    },
    "request": {
        "reqid": "unique-request-id",
        "language": "zh-CN",
        "enable_itn": true,
        "enable_punc": true
    }
}
```

### 响应

```json
{
    "reqid": "request-id",
    "code": 1000,
    "message": "success",
    "result": {
        "text": "完整识别文本",
        "utterances": [...]
    }
}
```

## 详细文档

- [流式识别](./streaming.md)
- [录音文件识别标准版](./file-standard.md)
- [录音文件识别极速版](./file-fast.md)
