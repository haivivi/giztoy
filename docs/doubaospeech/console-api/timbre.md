# 音色接口

## 原始文档

- **ListBigModelTTSTimbres**: https://www.volcengine.com/docs/6561/1772903
- **ListSpeakers**: https://www.volcengine.com/docs/6561/1772904

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## ListBigModelTTSTimbres

获取大模型音色列表。

### 请求

```
GET https://open.volcengineapi.com/?Action=ListBigModelTTSTimbres&Version=2024-01-01
```

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
    "Version": "2024-01-01"
  },
  "Result": {
    "Total": 100,
    "Timbres": [
      {
        "TimbreId": "zh_female_cancan",
        "TimbreName": "灿灿",
        "Language": "zh",
        "Gender": "female",
        "Description": "甜美活泼"
      }
    ]
  }
}
```

---

## ListSpeakers

获取大模型音色列表（新接口，推荐使用）。

### 请求

```
GET https://open.volcengineapi.com/?Action=ListSpeakers&Version=2024-01-01
```

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
    "RequestId": "xxx"
  },
  "Result": {
    "Total": 100,
    "Speakers": [
      {
        "SpeakerId": "zh_female_cancan",
        "SpeakerName": "灿灿",
        "Language": "zh-CN",
        "Gender": "female",
        "SampleAudioUrl": "https://xxx/sample.mp3"
      }
    ]
  }
}
```
