# 经典版语音识别 1.0

## 原始文档链接

| 文档 | 链接 |
|------|------|
| 一句话识别 | https://www.volcengine.com/docs/6561/104897 |
| 流式语音识别 | https://www.volcengine.com/docs/6561/80816 |
| 录音文件识别标准版 | https://www.volcengine.com/docs/6561/80818 |
| 录音文件识别极速版 | https://www.volcengine.com/docs/6561/80820 |

## 概述

经典版语音识别 1.0 提供一句话识别、流式识别和录音文件识别等功能。

> ⚠️ 推荐使用 [大模型语音识别 2.0](../asr2.0/)，识别效果更好。

## 接口列表

| 接口 | 端点 | Cluster | 特点 |
|------|------|---------|------|
| 一句话识别 | `POST /api/v1/asr` | `volcengine_input_common` | 短音频 |
| 流式识别 | `WSS /api/v2/asr` | `volcengine_streaming_common` | 实时 |
| 录音文件标准版 | `POST /api/v1/asr/submit` | - | 准确度优先 |
| 录音文件极速版 | `POST /api/v1/asr/async/submit` | - | 速度优先 |

## 认证方式

### V1/V2 API

**Header 认证：**

```http
Authorization: Bearer; {token}
```

**请求体认证：**

```json
{
    "app": {
        "appid": "{appid}",
        "token": "{token}",
        "cluster": "{cluster}"
    }
}
```

## 一句话识别

### 端点

```
POST https://openspeech.bytedance.com/api/v1/asr
```

### 请求

```json
{
    "app": {
        "appid": "123456789",
        "token": "your-token",
        "cluster": "volcengine_input_common"
    },
    "user": {
        "uid": "user-id"
    },
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
    "result": "识别结果文本"
}
```

## 流式语音识别

### 端点

```
WSS wss://openspeech.bytedance.com/api/v2/asr
```

### 连接参数

```
?appid={appid}&token={token}&cluster=volcengine_streaming_common
```

### 二进制协议

使用自定义二进制协议发送音频数据，详见 [streaming.md](./streaming.md)

## 录音文件识别

### 提交任务

```
POST https://openspeech.bytedance.com/api/v1/asr/submit
```

### 查询结果

```
POST https://openspeech.bytedance.com/api/v1/asr/query
```

## 错误码

| Code | 说明 |
|------|------|
| 1000 | 成功 |
| 1001 | 参数错误 |
| 3001 | 资源未授权 |
| 3002 | 认证失败 |

## 详细文档

- [一句话识别](./one-sentence.md)
- [流式语音识别](./streaming.md)
- [录音文件识别标准版](./file-standard.md)
- [录音文件识别极速版](./file-fast.md)
