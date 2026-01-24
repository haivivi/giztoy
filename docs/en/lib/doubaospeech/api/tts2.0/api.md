# 大模型语音合成 API

## 接口信息

| 项目 | 值 |
|------|------|
| 请求地址 | `https://openspeech.bytedance.com/api/v1/tts` |
| 请求方式 | POST |
| 协议 | HTTPS |

## 请求参数

### Headers

| 参数 | 必填 | 说明 |
|------|------|------|
| Content-Type | 是 | `application/json` |
| Authorization | 是 | `Bearer;{token}` |

### Body

```json
{
  "app": {
    "appid": "string",
    "token": "string",
    "cluster": "string"
  },
  "user": {
    "uid": "string"
  },
  "audio": {
    "voice_type": "string",
    "encoding": "string",
    "speed_ratio": 1.0,
    "volume_ratio": 1.0,
    "pitch_ratio": 1.0,
    "emotion": "string",
    "language": "string"
  },
  "request": {
    "reqid": "string",
    "text": "string",
    "text_type": "plain",
    "operation": "query",
    "silence_duration": 0,
    "with_frontend": true,
    "frontend_type": "unitTson"
  }
}
```

## 参数说明

### app 对象

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| appid | string | 是 | 应用 ID |
| token | string | 是 | 访问令牌（可放 Header） |
| cluster | string | 是 | 集群：`volcano_tts` / `volcano_mega` / `volcano_icl` |

### user 对象

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| uid | string | 是 | 用户标识，用于追踪 |

### audio 对象

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| voice_type | string | 是 | - | 音色 ID |
| encoding | string | 否 | mp3 | 编码格式：`mp3`/`wav`/`pcm`/`ogg_opus` |
| speed_ratio | float | 否 | 1.0 | 语速，范围 [0.2, 3.0] |
| volume_ratio | float | 否 | 1.0 | 音量，范围 [0.1, 3.0] |
| pitch_ratio | float | 否 | 1.0 | 音调，范围 [0.1, 3.0] |
| emotion | string | 否 | - | 情感：`happy`/`sad`/`angry` 等 |
| language | string | 否 | - | 语言标识 |

### request 对象

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| reqid | string | 是 | 请求 ID，建议 UUID |
| text | string | 是 | 合成文本，最大 10000 字符 |
| text_type | string | 否 | 文本类型：`plain`/`ssml` |
| operation | string | 否 | 操作类型：`query`/`submit` |
| silence_duration | int | 否 | 句尾静音时长（毫秒） |

## 响应格式

### 成功响应

```json
{
  "reqid": "xxx",
  "code": 3000,
  "message": "success",
  "sequence": 1,
  "data": "<base64编码的音频数据>",
  "addition": {
    "duration": "1234"
  }
}
```

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| reqid | string | 请求 ID |
| code | int | 状态码，3000 表示成功 |
| message | string | 状态消息 |
| sequence | int | 序列号（流式时使用） |
| data | string | Base64 编码的音频数据 |
| addition.duration | string | 音频时长（毫秒） |

## 状态码

| 码值 | 说明 |
|------|------|
| 3000 | 成功 |
| 3001 | 参数错误 |
| 3002 | 认证失败 |
| 3003 | 频率限制 |
| 3004 | 余额不足 |
| 3005 | 服务内部错误 |
