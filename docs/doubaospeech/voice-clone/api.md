# 声音复刻 API

## 原始文档

- **声音复刻API**: https://www.volcengine.com/docs/6561/1329511

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 接口列表

| 接口 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 创建复刻任务 | POST | `/api/v1/voice_clone/submit` | 上传音频创建音色 |
| 查询复刻状态 | POST | `/api/v1/voice_clone/query` | 查询任务状态 |
| 获取音色列表 | GET | `/api/v1/voice_clone/list` | 获取已创建的音色 |
| 删除音色 | POST | `/api/v1/voice_clone/delete` | 删除复刻音色 |

## 创建复刻任务

### 请求

```
POST https://openspeech.bytedance.com/api/v1/voice_clone/submit
```

### Headers

| 参数 | 必填 | 说明 |
|------|------|------|
| Content-Type | 是 | `multipart/form-data` |
| Authorization | 是 | `Bearer;{token}` |

### Form Data

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| appid | string | 是 | 应用 ID |
| voice_name | string | 是 | 音色名称 |
| audio_file | file | 是 | 音频文件 |
| clone_type | string | 否 | 复刻类型：`fast`/`standard`/`professional`/`icl` |
| language | string | 否 | 语言：`zh`/`en` |
| gender | string | 否 | 性别：`male`/`female` |
| description | string | 否 | 音色描述 |

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "task_id": "task_xxx",
    "voice_id": "clone_xxx",
    "status": "processing"
  }
}
```

## 查询复刻状态

### 请求

```
POST https://openspeech.bytedance.com/api/v1/voice_clone/query
```

### Body

```json
{
  "appid": "your_appid",
  "task_id": "task_xxx"
}
```

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "task_id": "task_xxx",
    "voice_id": "clone_xxx",
    "status": "success",
    "voice_name": "我的音色",
    "created_at": "2024-01-15T10:30:00Z"
  }
}
```

### 状态枚举

| 状态 | 说明 |
|------|------|
| processing | 处理中 |
| success | 成功 |
| failed | 失败 |

## 使用复刻音色

创建成功后，在 TTS 接口中使用：

```json
{
  "app": {
    "appid": "your_appid",
    "cluster": "volcano_icl"
  },
  "audio": {
    "voice_type": "clone_xxx"
  },
  "request": {
    "text": "你好，这是我的专属音色"
  }
}
```

注意：使用复刻音色时，cluster 需设为 `volcano_icl`。
