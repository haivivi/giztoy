# 同步语音合成 API (T2A)

> **官方文档**:
> - HTTP 接口: https://platform.minimaxi.com/docs/api-reference/speech-t2a-http
> - WebSocket 接口: https://platform.minimaxi.com/docs/api-reference/speech-t2a-ws

## 概述

同步语音合成 API 支持将文本转换为语音，单次最长处理 **10,000 字符**。支持 HTTP 和 WebSocket 两种调用方式。

## 支持的模型

| 模型 | 特性 |
|------|------|
| speech-2.6-hd | 最新 HD 模型，韵律表现出色，极致音质，生成更快更自然 |
| speech-2.6-turbo | 最新 Turbo 模型，音质优异，超低时延，响应更灵敏 |
| speech-02-hd | 出色的韵律、稳定性和复刻相似度，音质表现突出 |
| speech-02-turbo | 出色的韵律和稳定性，小语种能力加强，性能出色 |

## 支持的语言

支持 40 种语言：

| 语言 | 语言 | 语言 | 语言 |
|------|------|------|------|
| Chinese | English | Japanese | Korean |
| French | German | Spanish | Italian |
| Portuguese | Russian | Arabic | Thai |
| Vietnamese | Indonesian | Turkish | Dutch |
| Ukrainian | Polish | Romanian | Greek |
| Czech | Finnish | Hindi | Chinese,Yue (粤语) |
| ... | | | |

## HTTP 接口

### 端点

| 名称 | URL |
|------|-----|
| 主要地址 | `https://api.minimaxi.com/v1/t2a_v2` |
| 备用地址 | `https://api-bj.minimaxi.com/v1/t2a_v2` |

**方法**: POST

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型版本 |
| text | string | 是 | 需要合成的文本，限制 10,000 字符 |
| stream | boolean | 否 | 是否流式输出，默认 `false` |
| voice_setting | object | 否 | 语音设置 |
| audio_setting | object | 否 | 音频设置 |
| pronunciation_dict | object | 否 | 发音词典 |
| language_boost | string | 否 | 语言增强 |
| subtitle_enable | boolean | 否 | 是否开启字幕，默认 `false` |
| output_format | string | 否 | 输出格式：`hex`（默认）或 `url` |

### voice_setting 对象

| 参数 | 类型 | 默认值 | 范围 | 说明 |
|------|------|--------|------|------|
| voice_id | string | - | - | 音色 ID |
| speed | float | 1.0 | 0.5-2.0 | 语速 |
| vol | float | 1.0 | 0-10 | 音量 |
| pitch | int | 0 | -12 到 12 | 音调 |
| emotion | string | - | - | 情绪：`happy`, `sad`, `angry`, `fearful`, `disgusted`, `surprised`, `neutral` |

### audio_setting 对象

| 参数 | 类型 | 默认值 | 可选值 | 说明 |
|------|------|--------|--------|------|
| sample_rate | int | 32000 | 8000, 16000, 22050, 24000, 32000, 44100 | 采样率 |
| bitrate | int | 128000 | 32000, 64000, 128000, 256000 | 比特率 |
| format | string | mp3 | mp3, pcm, flac, wav | 音频格式 |
| channel | int | 1 | 1, 2 | 声道数 |

> **注意**: wav 格式仅在非流式输出下支持

### pronunciation_dict 对象

用于自定义发音：

```json
{
  "tone": [
    "处理/(chu3)(li3)",
    "危险/dangerous"
  ]
}
```

### language_boost 可选值

- 中文：`Chinese`, `Chinese,Yue`（粤语）
- 亚洲语言：`Japanese`, `Korean`, `Thai`, `Vietnamese`, `Indonesian`
- 欧洲语言：`English`, `French`, `German`, `Spanish`, `Italian`, `Portuguese`, `Russian`
- 其他：`Arabic`, `Turkish`, `Dutch`, `Ukrainian`, `Polish`, `Romanian`, `Greek`, `Czech`, `Finnish`, `Hindi`
- 自动检测：`auto`

### 停顿控制

在文本中使用 `<#x#>` 标记来控制停顿时间：
- `x` 为停顿时长（秒），范围 [0.01, 99.99]
- 示例：`你好<#1.5#>世界` 表示停顿 1.5 秒

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/t2a_v2 \
  --header 'Authorization: Bearer <your_api_key>' \
  --header 'Content-Type: application/json' \
  --data '{
    "model": "speech-2.6-hd",
    "text": "今天是不是很开心呀，当然了！",
    "stream": false,
    "voice_setting": {
      "voice_id": "male-qn-qingse",
      "speed": 1,
      "vol": 1,
      "pitch": 0,
      "emotion": "happy"
    },
    "audio_setting": {
      "sample_rate": 32000,
      "bitrate": 128000,
      "format": "mp3",
      "channel": 1
    }
  }'
```

### 响应格式

```json
{
  "data": {
    "audio": "<hex编码的音频数据或URL>",
    "status": 2
  },
  "extra_info": {
    "audio_length": 9900,
    "audio_sample_rate": 32000,
    "audio_size": 160323,
    "bitrate": 128000,
    "word_count": 52,
    "invisible_character_ratio": 0,
    "usage_characters": 26,
    "audio_format": "mp3",
    "audio_channel": 1
  },
  "trace_id": "01b8bf9bb7433cc75c18eee6cfa8fe21",
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

### 响应字段说明

#### data 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| audio | string | hex 编码的音频数据（或 URL，取决于 output_format） |
| status | int | 状态码，2 表示完成 |

#### extra_info 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| audio_length | int | 音频时长（毫秒） |
| audio_sample_rate | int | 采样率 |
| audio_size | int | 音频大小（字节） |
| bitrate | int | 比特率 |
| word_count | int | 字数 |
| usage_characters | int | 计费字符数 |
| audio_format | string | 音频格式 |
| audio_channel | int | 声道数 |

## WebSocket 接口

### 端点

```
wss://api.minimaxi.com/v1/t2a_v2/ws
```

### 连接参数

在 URL 中添加查询参数：

```
wss://api.minimaxi.com/v1/t2a_v2/ws?Authorization=Bearer%20<your_api_key>
```

### 消息格式

发送 JSON 格式的消息，参数与 HTTP 接口相同。

## 处理响应

### 解码 hex 音频数据

```go
// Go 示例
audioBytes, err := hex.DecodeString(response.Data.Audio)
if err != nil {
    return err
}
err = os.WriteFile("output.mp3", audioBytes, 0644)
```

```python
# Python 示例
audio_bytes = bytes.fromhex(response["data"]["audio"])
with open("output.mp3", "wb") as f:
    f.write(audio_bytes)
```

### 下载 URL 音频

当 `output_format` 为 `url` 时：

```python
import requests

response = requests.get(audio_url)
with open("output.mp3", "wb") as f:
    f.write(response.content)
```

## 注意事项

1. 当 `output_format` 为 `url` 时，返回的 URL 有效期为 **24 小时**
2. 流式场景仅支持返回 hex 格式
3. wav 格式仅在非流式输出下支持
4. 接口为无状态设计，不存储用户数据
