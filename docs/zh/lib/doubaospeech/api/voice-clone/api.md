# 声音复刻 API

## 原始文档

- **声音复刻API**: https://www.volcengine.com/docs/6561/1305191
- **声音复刻下单及使用指南**: https://www.volcengine.com/docs/6561/1327328
- **查询SpeakerID状态 (Console API)**: https://www.volcengine.com/docs/6561/1772920

> ⚠️ 如果本文档信息不准确，请以上述官方链接为准。

## 概述

声音复刻支持以下类型：

| model_type | 说明 | cluster |
|------------|------|---------|
| 1 | 声音复刻 ICL 1.0 | `volcano_icl` |
| 2 | DiT 标准版（音色，不还原用户的风格） | `volcano_mega` |
| 3 | DiT 还原版（音色、还原用户口音、语速等风格） | `volcano_mega` |
| 4 | 声音复刻 ICL 2.0 | - |

## 接口列表

### 语音 API（openspeech.bytedance.com）

| 接口 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 上传音频训练 | POST | `/api/v1/mega_tts/audio/upload` | 上传音频创建音色 |
| 查询训练状态 | GET/POST | `/api/v1/mega_tts/status` | 查询单个任务状态 |

### 控制台 API（open.volcengineapi.com）

| 接口 | Action | Version | 说明 |
|------|--------|---------|------|
| 分页查询状态 | `ListMegaTTSTrainStatus` | 2023-11-07 | 查询单个 SpeakerID |
| 批量查询状态 | `BatchListMegaTTSTrainStatus` | 2023-11-07 | 分页查询所有复刻音色 |

> ⚠️ **注意**: 
> - `/api/v1/voice_clone/list` 端点**不存在**！列表功能请使用控制台 API。
> - 控制台 API 使用 AK/SK 签名认证，与语音 API 不同。

---

## 上传音频训练

### 请求

```
POST https://openspeech.bytedance.com/api/v1/mega_tts/audio/upload
Content-Type: multipart/form-data
Authorization: Bearer;{token}
```

### Form Data

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| appid | string | 是 | 应用 ID |
| speaker_id | string | 是 | 自定义音色 ID（建议格式：`S_xxxxxxxxx`） |
| audio_format | string | 是 | 音频格式：`wav`, `mp3`, `ogg`, `pcm` |
| audio_data | file | 是 | 音频文件（Base64 或二进制） |
| model_type | int | 否 | 复刻类型：1=ICL1.0, 2=DiT标准, 3=DiT还原, 4=ICL2.0 |
| language | string | 否 | 语言：`zh`, `en` |
| gender | string | 否 | 性别：`male`, `female` |
| text | string | 否 | 音频对应的文本（用于对齐） |

### 响应

```json
{
  "BaseResp": {
    "StatusCode": 0,
    "StatusMessage": "success"
  },
  "speaker_id": "S_xxx"
}
```

---

## 查询训练状态

### 请求

```
GET https://openspeech.bytedance.com/api/v1/mega_tts/status?appid={appid}&speaker_id={speaker_id}
Authorization: Bearer;{token}
```

### 响应

```json
{
  "BaseResp": {
    "StatusCode": 0,
    "StatusMessage": "success"
  },
  "speaker_id": "S_xxx",
  "status": "Success",
  "demo_audio": "https://..."
}
```

### 状态枚举

| 状态 | 说明 |
|------|------|
| Processing | 处理中 |
| Success | 成功 |
| Failed | 失败 |

---

## 批量查询状态（控制台 API）

使用控制台 API 批量查询所有复刻音色状态。

### 请求

```
POST https://open.volcengineapi.com/?Action=BatchListMegaTTSTrainStatus&Version=2023-11-07
Content-Type: application/json
Authorization: HMAC-SHA256 Credential=...
```

**注意**: 需要 AK/SK 签名认证！

### Body

```json
{
  "AppID": "9476442538",
  "PageNumber": 1,
  "PageSize": 20
}
```

### 响应

```json
{
  "ResponseMetadata": {
    "RequestId": "xxx",
    "Action": "BatchListMegaTTSTrainStatus",
    "Version": "2023-11-07"
  },
  "Result": {
    "Total": 35,
    "Statuses": [
      {
        "SpeakerID": "S_TR0rbVuI1",
        "InstanceNO": "S2S_Model_storage_g-e_W094e4FDP1aq",
        "IsActivatable": true,
        "State": "Success",
        "DemoAudio": "https://...",
        "Version": "V1",
        "CreateTime": 1761400817000,
        "ExpireTime": 1792943999000,
        "Alias": "小茧",
        "ResourceID": "seed-icl-1.0"
      }
    ]
  }
}
```

---

## 使用复刻音色

创建成功后，在 TTS 接口中使用：

```json
{
  "app": {
    "appid": "your_appid",
    "cluster": "volcano_icl"
  },
  "audio": {
    "voice_type": "S_TR0rbVuI1"
  },
  "request": {
    "text": "你好，这是我的专属音色"
  }
}
```

### Cluster 选择

| model_type | cluster |
|------------|---------|
| 1 (ICL 1.0) | `volcano_icl` |
| 2, 3 (DiT) | `volcano_mega` |
| 4 (ICL 2.0) | `volcano_icl_concurr` |

### Realtime API 中使用

在实时对话 API 中使用复刻音色：

```json
{
  "event": {
    "event": "start_connection",
    "speaker": {
      "speaker_id": "S_TR0rbVuI1"
    }
  }
}
```
