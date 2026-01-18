# HTTP 接口（一次性合成-非流式）

## 原始文档

- **HTTP接口**: https://www.volcengine.com/docs/6561/79820

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 接口信息

| 项目 | 值 |
|------|------|
| 请求地址 | `https://openspeech.bytedance.com/api/v1/tts` |
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
    "token": "your_token",
    "cluster": "volcano_tts"
  },
  "user": {
    "uid": "user_001"
  },
  "audio": {
    "voice_type": "BV001_streaming",
    "encoding": "mp3",
    "speed_ratio": 1.0,
    "volume_ratio": 1.0,
    "pitch_ratio": 1.0
  },
  "request": {
    "reqid": "uuid",
    "text": "你好，世界！",
    "text_type": "plain",
    "operation": "query"
  }
}
```

## 响应格式

```json
{
  "reqid": "xxx",
  "code": 3000,
  "operation": "query",
  "message": "success",
  "sequence": -1,
  "data": "<base64编码的音频数据>",
  "addition": {
    "duration": "1234"
  }
}
```

## 音频解码

```go
audioBytes, err := base64.StdEncoding.DecodeString(response.Data)
if err != nil {
    return err
}
err = os.WriteFile("output.mp3", audioBytes, 0644)
```
