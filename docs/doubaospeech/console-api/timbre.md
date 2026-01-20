# 音色接口

## 原始文档

- **ListBigModelTTSTimbres**: https://www.volcengine.com/docs/6561/1770994
- **ListSpeakers**: https://www.volcengine.com/docs/6561/2160690

> ⚠️ 如果本文档信息不准确，请以上述官方链接为准。

## 鉴权方式

控制台 API 使用火山引擎 OpenAPI 标准鉴权（AK/SK 签名），与语音 API 的鉴权方式不同。

```
Authorization: HMAC-SHA256 Credential={AccessKey}/{Date}/{Region}/speech_saas_prod/request, SignedHeaders=..., Signature=...
```

---

## ListBigModelTTSTimbres

获取大模型音色列表（声音复刻 ICL 音色）。

### 请求

```
POST https://open.volcengineapi.com/?Action=ListBigModelTTSTimbres&Version=2025-05-20
Content-Type: application/json
```

**注意**: Version 是 `2025-05-20`，不是 `2024-01-01`！

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| PageNumber | int | 否 | 页码，默认 1 |
| PageSize | int | 否 | 每页数量，默认 20 |
| TimbreType | string | 否 | 音色类型筛选 |

### 响应

```json
{
  "ResponseMetadata": {
    "RequestId": "xxx",
    "Action": "ListBigModelTTSTimbres",
    "Version": "2025-05-20"
  },
  "Result": {
    "Timbres": [
      {
        "SpeakerID": "ICL_zh_female_liumengdie_v1_tob",
        "TimbreInfos": [
          {
            "SpeakerName": "清冷高雅",
            "Gender": "女",
            "Age": "青年",
            "Categories": [
              {"Category": "角色扮演"}
            ],
            "Emotions": [
              {
                "Emotion": "通用",
                "EmotionType": "general",
                "DemoText": "世间喧闹与我无关...",
                "DemoURL": "https://lf3-static.bytednsdoc.com/obj/..."
              }
            ]
          }
        ]
      }
    ]
  }
}
```

---

## ListSpeakers

获取大模型音色列表（新接口，推荐使用）。返回 TTS 2.0 可用的音色。

### 请求

```
POST https://open.volcengineapi.com/?Action=ListSpeakers&Version=2025-05-20
Content-Type: application/json
```

**注意**: Version 是 `2025-05-20`，不是 `2024-01-01`！

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| PageNumber | int | 否 | 页码 |
| PageSize | int | 否 | 每页数量 |
| SpeakerType | string | 否 | 音色类型 |
| Language | string | 否 | 语言筛选 |

### 响应

```json
{
  "ResponseMetadata": {
    "RequestId": "xxx",
    "Action": "ListSpeakers",
    "Version": "2025-05-20"
  },
  "Result": {
    "Total": 361,
    "Speakers": [
      {
        "ID": "7574011450654457882",
        "VoiceType": "zh_female_xiaohe_uranus_bigtts",
        "Name": "小何 2.0",
        "Avatar": "https://lf3-static.bytednsdoc.com/obj/.../小何.png",
        "Gender": "女",
        "Age": "青年",
        "Categories": [{"Categories": ["通用场景"]}],
        "CategoryKeys": ["1340b51f-f828-4383-8fd0-f62a750d6cea"],
        "TrialURL": "https://lf3-static.bytednsdoc.com/obj/..."
      }
    ]
  }
}
```

### VoiceType 字段说明

`VoiceType` 是在调用 TTS API 时使用的音色标识符，例如：

- `zh_female_xiaohe_uranus_bigtts` - 小何 2.0
- `zh_female_vv_uranus_bigtts` - Vivi 2.0
- `zh_male_taocheng_uranus_bigtts` - 小天 2.0

在 TTS 请求中使用：

```json
{
  "voice_type": "zh_female_xiaohe_uranus_bigtts",
  "text": "你好，世界！"
}
```
