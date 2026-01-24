# 音视频字幕生成

## 原始文档

- **音视频字幕生成**: https://www.volcengine.com/docs/6561/1251235

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 概述

自动为音视频文件生成带时间戳的字幕。

## 接口列表

| 接口 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 提交任务 | POST | `/api/v1/subtitle/submit` | 提交字幕生成任务 |
| 查询任务 | GET | `/api/v1/subtitle/query` | 查询任务状态 |

## 提交任务

### 请求

```
POST https://openspeech.bytedance.com/api/v1/subtitle/submit
```

### Body

```json
{
  "appid": "your_appid",
  "reqid": "uuid",
  "media_url": "https://example.com/video.mp4",
  "language": "zh-CN",
  "output_format": "srt",
  "enable_speaker": false,
  "callback_url": "https://your-server.com/callback"
}
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| media_url | string | 是 | 音视频文件 URL |
| language | string | 否 | 语言 |
| output_format | string | 否 | 输出格式：`srt`/`vtt`/`json` |
| enable_speaker | bool | 否 | 是否区分说话人 |

## 响应格式

### 任务完成

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "success",
    "subtitle_url": "https://xxx.com/subtitle.srt",
    "subtitle_content": "1\n00:00:01,000 --> 00:00:03,500\n你好，欢迎观看本视频\n\n2\n00:00:04,000 --> 00:00:07,000\n今天我们来讨论...",
    "duration": 125000
  }
}
```
