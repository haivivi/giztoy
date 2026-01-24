# 经典版语音合成 1.0

## 原始文档链接

| 文档 | 链接 |
|------|------|
| HTTP接口（一次性合成） | https://www.volcengine.com/docs/6561/79820 |
| WebSocket接口 | https://www.volcengine.com/docs/6561/79821 |
| 参数基本说明 | https://www.volcengine.com/docs/6561/97465 |
| 发音人参数列表 | https://www.volcengine.com/docs/6561/1096680 |

## 概述

经典版语音合成 1.0 是传统的语音合成服务，提供 HTTP 和 WebSocket 两种接入方式。

> ⚠️ 推荐使用 [大模型语音合成 2.0](../tts2.0/)，效果更自然。

## 接口列表

| 接口 | 端点 | Cluster | 特点 |
|------|------|---------|------|
| HTTP 一次性合成 | `POST /api/v1/tts` | `volcano_tts` | 简单易用 |
| WebSocket 流式 | `WSS /api/v1/tts/ws_binary` | `volcano_tts` | 低延迟 |

## 认证方式

### HTTP 接口

**方式一：Header + Body**

```http
POST /api/v1/tts HTTP/1.1
Host: openspeech.bytedance.com
Authorization: Bearer; {token}
Content-Type: application/json

{
    "app": {
        "appid": "{appid}",
        "token": "{token}",
        "cluster": "volcano_tts"
    },
    ...
}
```

### WebSocket 接口

**URL 参数认证：**

```
wss://openspeech.bytedance.com/api/v1/tts/ws_binary?appid={appid}&token={token}&cluster={cluster}
```

## 请求参数

### HTTP 请求体

```json
{
    "app": {
        "appid": "123456789",
        "token": "your-token",
        "cluster": "volcano_tts"
    },
    "user": {
        "uid": "user-id"
    },
    "audio": {
        "voice_type": "BV001_streaming",
        "encoding": "mp3",
        "speed_ratio": 1.0,
        "volume_ratio": 1.0,
        "pitch_ratio": 1.0
    },
    "request": {
        "reqid": "unique-request-id",
        "text": "要合成的文本",
        "text_type": "plain",
        "operation": "query"
    }
}
```

### 参数说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `app.appid` | string | ✅ | APP ID |
| `app.token` | string | ✅ | Access Token |
| `app.cluster` | string | ✅ | 集群名：`volcano_tts` |
| `user.uid` | string | ✅ | 用户标识 |
| `audio.voice_type` | string | ✅ | 发音人 ID |
| `audio.encoding` | string | | `mp3`/`pcm`/`ogg_opus` |
| `audio.speed_ratio` | float | | 语速：0.5-2.0 |
| `audio.volume_ratio` | float | | 音量：0.5-2.0 |
| `audio.pitch_ratio` | float | | 音调：0.5-2.0 |
| `request.reqid` | string | ✅ | 请求 ID |
| `request.text` | string | ✅ | 合成文本 |
| `request.text_type` | string | | `plain`/`ssml` |
| `request.operation` | string | | `query`/`submit` |

## 响应格式

### HTTP 响应

```json
{
    "reqid": "request-id",
    "code": 3000,
    "message": "Success",
    "sequence": 1,
    "data": "base64-encoded-audio",
    "addition": {
        "duration": "1.5"
    }
}
```

### 错误响应

```json
{
    "reqid": "request-id",
    "code": 3001,
    "message": "[resource_id=volc.tts.default] requested resource not granted"
}
```

## 错误码

| Code | 说明 |
|------|------|
| 3000 | 成功 |
| 3001 | 资源未授权 |
| 3002 | 认证失败 |
| 3050 | 音色不存在 |

## 发音人列表

常用发音人 ID：

| Voice Type | 说明 |
|------------|------|
| `BV001_streaming` | 通用女声 |
| `BV002_streaming` | 通用男声 |
| `BV700_streaming` | 精品女声 |
| `BV701_streaming` | 精品男声 |

完整列表请参考[发音人参数列表](https://www.volcengine.com/docs/6561/1096680)

## 示例代码

### cURL

```bash
curl -X POST "https://openspeech.bytedance.com/api/v1/tts" \
  -H "Authorization: Bearer; YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "app": {
      "appid": "YOUR_APP_ID",
      "token": "YOUR_TOKEN",
      "cluster": "volcano_tts"
    },
    "user": {"uid": "test"},
    "audio": {
      "voice_type": "BV001_streaming",
      "encoding": "mp3"
    },
    "request": {
      "reqid": "test-001",
      "text": "你好，世界！",
      "text_type": "plain",
      "operation": "query"
    }
  }'
```

## 详细文档

- [HTTP接口](./http.md)
- [WebSocket接口](./websocket.md)
