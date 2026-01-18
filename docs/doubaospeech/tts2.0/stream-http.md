# 单向流式 HTTP 接口 (V3)

## 概述

单向流式 HTTP 接口支持流式输出音频，适用于实时播放场景。支持声音复刻和混音功能。

## 接口信息

| 项目 | 值 |
|------|------|
| 请求地址 | `https://openspeech.bytedance.com/api/v1/tts/stream` |
| 请求方式 | POST |
| 响应方式 | Chunked Transfer |

## 请求参数

### Headers

| 参数 | 必填 | 说明 |
|------|------|------|
| Content-Type | 是 | `application/json` |
| Authorization | 是 | `Bearer;{token}` |
| X-Request-Id | 否 | 请求 ID |

### Body

```json
{
  "app": {
    "appid": "your_appid",
    "cluster": "volcano_tts"
  },
  "user": {
    "uid": "user_001"
  },
  "audio": {
    "voice_type": "zh_female_cancan",
    "encoding": "mp3",
    "speed_ratio": 1.0,
    "volume_ratio": 1.0,
    "pitch_ratio": 1.0
  },
  "request": {
    "reqid": "uuid",
    "text": "你好，这是一段测试文本。",
    "text_type": "plain"
  }
}
```

## 响应格式

响应以 Chunked Transfer 方式返回，每个 chunk 是一个 JSON 对象：

### 音频数据块

```json
{
  "reqid": "xxx",
  "code": 3000,
  "message": "success",
  "sequence": 1,
  "data": "<base64编码的音频片段>"
}
```

### 结束标记

```json
{
  "reqid": "xxx",
  "code": 3000,
  "message": "success",
  "sequence": -1,
  "data": "",
  "addition": {
    "duration": "3456"
  }
}
```

## 响应字段

| 字段 | 说明 |
|------|------|
| sequence | 序列号，>0 表示数据块，-1 表示结束 |
| data | Base64 编码的音频数据 |
| addition.duration | 总音频时长（毫秒），仅在结束时返回 |

## Go 客户端示例

```go
resp, err := http.Post(url, "application/json", body)
if err != nil {
    return err
}
defer resp.Body.Close()

reader := bufio.NewReader(resp.Body)
for {
    line, err := reader.ReadBytes('\n')
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    
    var chunk StreamResponse
    if err := json.Unmarshal(line, &chunk); err != nil {
        continue
    }
    
    if chunk.Sequence == -1 {
        // 结束
        break
    }
    
    audioData, _ := base64.StdEncoding.DecodeString(chunk.Data)
    // 处理音频数据...
}
```
