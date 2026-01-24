# 同声传译 2.0 API

## 原始文档

- **API接入文档**: https://www.volcengine.com/docs/6561/1347897

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 接口信息

| 项目 | 值 |
|------|------|
| 请求地址 | `wss://openspeech.bytedance.com/api/v2/st` |
| 协议 | WebSocket |

## 认证

```
wss://openspeech.bytedance.com/api/v2/st?appid=xxx&token=xxx&cluster=xxx
```

## 开始会话

```json
{
  "app": {
    "appid": "your_appid",
    "cluster": "volcano_st"
  },
  "user": {
    "uid": "user_001"
  },
  "audio": {
    "format": "pcm",
    "sample_rate": 16000,
    "channel": 1,
    "bits": 16
  },
  "request": {
    "reqid": "uuid",
    "source_language": "zh",
    "target_language": "en",
    "enable_asr": true,
    "enable_tts": true,
    "tts_voice_type": "en_female_sweet"
  }
}
```

## 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| source_language | string | 是 | 源语言 |
| target_language | string | 是 | 目标语言 |
| enable_asr | bool | 否 | 是否返回 ASR 结果 |
| enable_tts | bool | 否 | 是否返回翻译后的语音 |
| tts_voice_type | string | 否 | TTS 音色（启用 TTS 时） |

## 发送音频

以二进制帧发送 PCM 音频数据。

## 响应格式

### ASR 结果

```json
{
  "type": "asr",
  "data": {
    "text": "你好，今天天气怎么样？",
    "is_final": true
  }
}
```

### 翻译结果

```json
{
  "type": "translation",
  "data": {
    "source_text": "你好，今天天气怎么样？",
    "target_text": "Hello, how is the weather today?",
    "is_final": true
  }
}
```

### TTS 音频

```json
{
  "type": "tts",
  "data": {
    "audio": "<base64编码的音频>",
    "sequence": 1
  }
}
```

## 流程示意

```
用户语音输入 → ASR识别 → 翻译 → TTS合成 → 翻译后语音输出
     ↓            ↓         ↓
   (PCM)    (源语言文本)  (目标语言文本)
```
