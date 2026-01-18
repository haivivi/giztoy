# 豆包语音妙记 API

## 原始文档

- **API接入文档**: https://www.volcengine.com/docs/6561/1352622

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 接口列表

| 接口 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 创建会议 | POST | `/api/v1/meeting/create` | 创建会议记录任务 |
| 上传音频 | POST | `/api/v1/meeting/upload` | 上传会议音频 |
| 实时转写 | WebSocket | `/api/v1/meeting/stream` | 实时语音转写 |
| 查询结果 | GET | `/api/v1/meeting/query` | 查询转写结果 |
| 生成纪要 | POST | `/api/v1/meeting/summary` | 生成会议纪要 |

## 创建会议

### 请求

```
POST https://openspeech.bytedance.com/api/v1/meeting/create
```

### Body

```json
{
  "appid": "your_appid",
  "meeting_id": "meeting_001",
  "title": "产品评审会议",
  "participants": [
    {"name": "张三", "voice_id": "voice_001"},
    {"name": "李四", "voice_id": "voice_002"}
  ],
  "language": "zh-CN",
  "enable_speaker_diarization": true,
  "enable_summary": true
}
```

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "meeting_id": "meeting_001",
    "session_id": "session_xxx",
    "status": "created"
  }
}
```

## 实时转写 (WebSocket)

### 连接

```
wss://openspeech.bytedance.com/api/v1/meeting/stream?session_id=xxx&token=xxx
```

### 发送音频

二进制帧发送 PCM 音频数据。

### 接收结果

```json
{
  "type": "transcript",
  "data": {
    "text": "我觉得这个方案可以",
    "speaker": "张三",
    "start_time": 12500,
    "end_time": 14200,
    "is_final": true
  }
}
```

## 生成会议纪要

### 请求

```
POST https://openspeech.bytedance.com/api/v1/meeting/summary
```

### Body

```json
{
  "appid": "your_appid",
  "meeting_id": "meeting_001",
  "summary_type": "detailed"
}
```

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "summary": "会议主题：产品评审...",
    "key_points": [
      "讨论了新功能的设计方案",
      "确定了下周的里程碑"
    ],
    "action_items": [
      {"assignee": "张三", "task": "完成原型设计", "deadline": "2024-01-20"},
      {"assignee": "李四", "task": "准备技术文档", "deadline": "2024-01-22"}
    ]
  }
}
```
