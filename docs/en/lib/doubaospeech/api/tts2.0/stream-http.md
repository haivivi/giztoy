# 单向流式 HTTP V3

## 原始文档

https://www.volcengine.com/docs/6561/1598757

## 接口功能

单向流式 API 为用户提供文本转语音的能力，支持多语种、多方言，通过 HTTP 协议流式输出音频。

## 端点

```
POST https://openspeech.bytedance.com/api/v3/tts/unidirectional
```

## 认证

### Request Headers

| Header | 说明 | 必填 | 示例 |
|--------|------|------|------|
| `X-Api-App-Id` | APP ID | ✅ | `123456789` |
| `X-Api-Access-Key` | Access Token | ✅ | `your-access-token` |
| `X-Api-Resource-Id` | 资源 ID | ✅ | `seed-tts-2.0` |
| `X-Api-Request-Id` | 请求 ID | | `uuid-xxx` |
| `X-Control-Require-Usage-Tokens-Return` | 返回用量 | | `*` 或 `text_words` |

### Resource ID 取值

| Resource ID | 说明 |
|-------------|------|
| `seed-tts-1.0` 或 `volc.service_type.10029` | 大模型 1.0（字符版）|
| `seed-tts-1.0-concurr` 或 `volc.service_type.10048` | 大模型 1.0（并发版）|
| `seed-tts-2.0` | 大模型 2.0（字符版）|
| `seed-icl-1.0` | 声音复刻 1.0（字符版）|
| `seed-icl-2.0` | 声音复刻 2.0（字符版）|

## 请求

### Content-Type

```
application/json
```

### Request Body

```json
{
    "user": {
        "uid": "12345"
    },
    "req_params": {
        "text": "要合成的文本",
        "speaker": "zh_female_shuangkuaisisi_moon_bigtts",
        "audio_params": {
            "format": "mp3",
            "sample_rate": 24000
        }
    }
}
```

### 参数说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `user.uid` | string | ✅ | 用户标识 |
| `req_params.text` | string | ✅ | 合成文本（双向流式不支持 SSML）|
| `req_params.speaker` | string | ✅ | 发音人 ID，见[音色列表](./voices.md) |
| `req_params.model` | string | | 模型版本：`seed-tts-1.1` |
| `req_params.audio_params` | object | ✅ | 音频参数 |
| `req_params.audio_params.format` | string | | `mp3`/`ogg_opus`/`pcm`，默认 `mp3` |
| `req_params.audio_params.sample_rate` | int | | 采样率：8000-48000，默认 24000 |
| `req_params.audio_params.bit_rate` | int | | 比特率：16000-160000 |
| `req_params.audio_params.speech_rate` | int | | 语速：-50 到 100（0=1.0x）|
| `req_params.audio_params.loudness_rate` | int | | 音量：-50 到 100（0=1.0x）|
| `req_params.audio_params.emotion` | string | | 情感：`angry`/`happy`/`sad` 等 |
| `req_params.audio_params.emotion_scale` | int | | 情感强度：1-5，默认 4 |
| `req_params.mix_speaker` | object | | 混音配置 |
| `req_params.addition` | object | | 附加参数 |

### 混音请求示例

使用混音时，`speaker` 必须设置为 `custom_mix_bigtts`：

```json
{
    "user": {
        "uid": "12345"
    },
    "req_params": {
        "text": "混音合成文本",
        "speaker": "custom_mix_bigtts",
        "audio_params": {
            "format": "mp3",
            "sample_rate": 24000
        },
        "mix_speaker": {
            "speakers": [
                {"source_speaker": "zh_male_bvlazysheep", "mix_factor": 0.3},
                {"source_speaker": "BV120_streaming", "mix_factor": 0.3},
                {"source_speaker": "zh_male_ahu_conversation_wvae_bigtts", "mix_factor": 0.4}
            ]
        }
    }
}
```

**混音限制：**
- 最多支持 3 个音色混合
- 混音因子之和必须等于 1
- 复刻音色需使用 `icl_` 开头的 speaker ID
- 风格差异较大的音色以 0.5-0.5 混合可能出现跳变

## 响应

### 流式响应格式

服务端以流式方式返回多条 JSON，每条以换行符分隔。

#### 音频数据

```json
{
    "code": 0,
    "message": "",
    "data": "base64-encoded-audio-chunk"
}
```

#### 文本数据（含时间戳）

```json
{
    "code": 0,
    "message": "",
    "data": null,
    "sentence": {
        "text": "其他人。",
        "words": [
            {"word": "其", "startTime": 0.205, "endTime": 0.315, "confidence": 0.85},
            {"word": "他", "startTime": 0.315, "endTime": 0.515, "confidence": 0.97},
            {"word": "人。", "startTime": 0.515, "endTime": 0.815, "confidence": 0.92}
        ]
    }
}
```

#### 结束响应

```json
{
    "code": 20000000,
    "message": "ok",
    "data": null,
    "usage": {"text_words": 10}
}
```

> `usage` 字段仅在请求 Header 中设置 `X-Control-Require-Usage-Tokens-Return` 时返回。

## 错误码

| Code | Message | 说明 |
|------|---------|------|
| 20000000 | ok | 合成结束成功 |
| 40402003 | TTSExceededTextLimit:exceed max limit | 文本长度超限 |
| 45000000 | speaker permission denied | 音色未授权或错误 |
| quota exceeded for types: concurrency | - | 并发数超限 |
| 55000000 | 服务端错误 | 通用错误 |

## 最佳实践

1. **连接复用**：使用 HTTP Keep-Alive 复用连接（火山服务端 keep-alive 时间为 1 分钟）
   
   ```python
   session = requests.Session()
   response = session.post(url, headers=headers, json=payload, stream=True)
   ```

2. **流式处理**：逐行读取 JSON 响应，解码 base64 音频数据后拼接播放

3. **错误处理**：检查每条响应的 `code` 字段，非 0 和非 20000000 均为错误

## 示例代码

### Python

```python
import requests
import base64

url = "https://openspeech.bytedance.com/api/v3/tts/unidirectional"
headers = {
    "X-Api-App-Id": "YOUR_APP_ID",
    "X-Api-Access-Key": "YOUR_ACCESS_TOKEN",
    "X-Api-Resource-Id": "seed-tts-2.0",
    "Content-Type": "application/json"
}
payload = {
    "user": {"uid": "test"},
    "req_params": {
        "text": "你好，世界！",
        "speaker": "zh_female_cancan",
        "audio_params": {"format": "mp3", "sample_rate": 24000}
    }
}

session = requests.Session()
response = session.post(url, headers=headers, json=payload, stream=True)

audio_data = b""
for line in response.iter_lines():
    if line:
        data = json.loads(line)
        if data.get("data"):
            audio_data += base64.b64decode(data["data"])
        if data.get("code") == 20000000:
            print("合成完成")
            break

with open("output.mp3", "wb") as f:
    f.write(audio_data)
```

### cURL

```bash
curl -X POST "https://openspeech.bytedance.com/api/v3/tts/unidirectional" \
  -H "X-Api-App-Id: YOUR_APP_ID" \
  -H "X-Api-Access-Key: YOUR_ACCESS_TOKEN" \
  -H "X-Api-Resource-Id: seed-tts-2.0" \
  -H "Content-Type: application/json" \
  -d '{
    "user": {"uid": "test"},
    "req_params": {
      "text": "你好，世界！",
      "speaker": "zh_female_cancan",
      "audio_params": {"format": "mp3", "sample_rate": 24000}
    }
  }'
```
