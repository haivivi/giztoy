# 音色快速复刻 API (Voice Cloning)

> **官方文档**: https://platform.minimaxi.com/docs/api-reference/speech-voice-cloning

## 概述

音色快速复刻 API 允许用户上传音频文件来复刻音色，生成的音色可用于语音合成接口。

## 支持的模型

| 模型 | 特性 |
|------|------|
| speech-2.6-hd | 最新 HD 模型 |
| speech-2.6-turbo | 最新 Turbo 模型 |
| speech-02-hd | 出色的复刻相似度 |
| speech-02-turbo | 性能出色 |

## 接口说明

音色快速复刻包含 3 个接口：

1. **上传复刻音频** - 上传待复刻的音频文件
2. **上传示例音频** - 上传示例音频（可选，增强效果）
3. **快速复刻** - 执行复刻，生成 voice_id

## 上传复刻音频

### 端点

```
POST https://api.minimaxi.com/v1/voice_clone/upload
```

### 请求参数

使用 `multipart/form-data` 格式：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | file | 是 | 音频文件 |
| purpose | string | 是 | 固定为 `voice_clone` |

### 支持的音频格式

- mp3
- m4a
- wav

### 音频要求

- 时长建议 10-60 秒
- 清晰的人声，无背景音乐
- 单声道或双声道

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/voice_clone/upload \
  --header 'Authorization: Bearer <your_api_key>' \
  --form 'file=@voice_sample.mp3' \
  --form 'purpose=voice_clone'
```

### 响应格式

```json
{
  "file_id": "file_abc123",
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

## 上传示例音频

### 端点

```
POST https://api.minimaxi.com/v1/voice_clone/upload_demo
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | file | 是 | 示例音频文件 |
| purpose | string | 是 | 固定为 `voice_clone_demo` |

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/voice_clone/upload_demo \
  --header 'Authorization: Bearer <your_api_key>' \
  --form 'file=@demo_sample.mp3' \
  --form 'purpose=voice_clone_demo'
```

### 响应格式

```json
{
  "file_id": "file_demo456",
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

## 快速复刻

### 端点

```
POST https://api.minimaxi.com/v1/voice_clone
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_id | string | 是 | 复刻音频的 file_id |
| demo_file_id | string | 否 | 示例音频的 file_id |
| voice_id | string | 是 | 自定义的音色 ID |
| model | string | 否 | 模型版本，默认 speech-02-hd |
| text | string | 否 | 试听文本 |
| output_directory | string | 否 | 输出目录 |

### 请求示例

```bash
curl --request POST \
  --url https://api.minimaxi.com/v1/voice_clone \
  --header 'Authorization: Bearer <your_api_key>' \
  --header 'Content-Type: application/json' \
  --data '{
    "file_id": "file_abc123",
    "demo_file_id": "file_demo456",
    "voice_id": "my_custom_voice",
    "model": "speech-02-hd",
    "text": "这是一段试听文本"
  }'
```

### 响应格式

```json
{
  "voice_id": "my_custom_voice",
  "demo_audio": "<hex编码的试听音频>",
  "base_resp": {
    "status_code": 0,
    "status_msg": "success"
  }
}
```

## 使用复刻音色

复刻成功后，可在语音合成接口中使用：

```json
{
  "model": "speech-2.6-hd",
  "text": "使用复刻音色合成语音",
  "voice_setting": {
    "voice_id": "my_custom_voice"
  }
}
```

## 注意事项

1. **临时音色**: 复刻的音色为临时音色，若 **7 天内** 未被用于语音合成（试听不算），将被自动删除
2. **费用计算**: 复刻费用在首次使用语音合成时收取
3. **认证要求**: 需要完成实名认证或企业认证
4. **无状态设计**: 接口不存储用户数据
5. **音频质量**: 上传的音频质量直接影响复刻效果，建议使用清晰、无噪音的录音
