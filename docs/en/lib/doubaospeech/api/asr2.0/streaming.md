# 大模型流式语音识别 API

## 接口信息

| 项目 | 值 |
|------|------|
| 请求地址 | `wss://openspeech.bytedance.com/api/v2/asr` |
| 协议 | WebSocket |

## 连接参数

```
wss://openspeech.bytedance.com/api/v2/asr?appid=xxx&token=xxx&cluster=volcengine_streaming_common
```

## 消息格式

### 开始识别

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
    "format": "pcm",
    "sample_rate": 16000,
    "channel": 1,
    "bits": 16
  },
  "request": {
    "reqid": "uuid",
    "workflow": "audio_in,resample,partition,vad,fe,decode,itn,nlu_punctuate",
    "show_utterances": true,
    "result_type": "full"
  }
}
```

### 发送音频

以二进制帧发送 PCM 音频数据，建议每帧 100-200ms。

### 结束识别

```json
{
  "request": {
    "reqid": "uuid",
    "command": "finish"
  }
}
```

## 响应格式

### 中间结果

```json
{
  "reqid": "uuid",
  "code": 1000,
  "message": "success",
  "result": {
    "text": "你好世",
    "is_final": false,
    "utterances": [
      {
        "text": "你好世",
        "start_time": 0,
        "end_time": 800
      }
    ]
  }
}
```

### 最终结果

```json
{
  "reqid": "uuid",
  "code": 1000,
  "message": "success",
  "result": {
    "text": "你好世界",
    "is_final": true,
    "utterances": [
      {
        "text": "你好世界",
        "start_time": 0,
        "end_time": 1200,
        "words": [
          {"text": "你", "start_time": 0, "end_time": 200},
          {"text": "好", "start_time": 200, "end_time": 400},
          {"text": "世", "start_time": 400, "end_time": 800},
          {"text": "界", "start_time": 800, "end_time": 1200}
        ]
      }
    ]
  }
}
```

## 参数说明

### audio 对象

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| format | string | 是 | 音频格式：`pcm`/`wav`/`mp3`/`ogg` |
| sample_rate | int | 是 | 采样率：8000/16000 |
| channel | int | 是 | 声道数：1/2 |
| bits | int | 是 | 位深：16 |

### request 对象

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| reqid | string | 是 | 请求 ID |
| workflow | string | 否 | 处理流程 |
| show_utterances | bool | 否 | 是否返回 utterances |
| result_type | string | 否 | 结果类型：`full`/`single` |
