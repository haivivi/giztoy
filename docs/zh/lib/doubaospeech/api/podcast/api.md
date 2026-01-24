# 播客 API (WebSocket V3)

## 原始文档

- **播客API-websocket-v3协议**: https://www.volcengine.com/docs/6561/1356830

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 接口信息

| 项目 | 值 |
|------|------|
| 请求地址 | `wss://openspeech.bytedance.com/api/v3/tts/podcast` |
| 协议 | WebSocket |

## 认证

```
wss://openspeech.bytedance.com/api/v3/tts/podcast?appid=xxx&token=xxx&cluster=xxx
```

## 请求格式

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
    "encoding": "mp3",
    "sample_rate": 24000
  },
  "request": {
    "reqid": "uuid",
    "speakers": [
      {
        "name": "主持人A",
        "voice_type": "zh_male_yangguang"
      },
      {
        "name": "主持人B",
        "voice_type": "zh_female_cancan"
      }
    ],
    "dialogues": [
      {
        "speaker": "主持人A",
        "text": "大家好，欢迎收听今天的节目。"
      },
      {
        "speaker": "主持人B",
        "text": "是的，今天我们要聊的话题非常有趣。"
      },
      {
        "speaker": "主持人A",
        "text": "没错，让我们开始吧！"
      }
    ]
  }
}
```

## 参数说明

### speakers 数组

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 说话人名称，用于关联对话 |
| voice_type | string | 是 | 音色 ID |
| speed_ratio | float | 否 | 语速 |
| volume_ratio | float | 否 | 音量 |

### dialogues 数组

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| speaker | string | 是 | 说话人名称，需与 speakers 中定义的一致 |
| text | string | 是 | 对话内容 |
| emotion | string | 否 | 情感 |

## 响应格式

流式返回音频数据：

```json
{
  "reqid": "xxx",
  "code": 3000,
  "message": "success",
  "sequence": 1,
  "data": "<base64编码的音频>",
  "speaker": "主持人A",
  "dialogue_index": 0
}
```

### 结束标记

```json
{
  "reqid": "xxx",
  "code": 3000,
  "sequence": -1,
  "addition": {
    "duration": "15000",
    "total_dialogues": 3
  }
}
```
