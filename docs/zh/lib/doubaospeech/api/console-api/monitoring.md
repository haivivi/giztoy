# 监控接口

## 原始文档

| 接口 | 链接 |
|------|------|
| QuotaMonitoring | https://www.volcengine.com/docs/6561/1772924 |
| UsageMonitoring | https://www.volcengine.com/docs/6561/1772925 |
| QPS/并发查询接口说明 | https://www.volcengine.com/docs/6561/1772926 |
| 调用量查询接口说明 | https://www.volcengine.com/docs/6561/1772927 |

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## QuotaMonitoring

查询配额信息。

### 请求

```
GET https://open.volcengineapi.com/?Action=QuotaMonitoring&Version=2024-01-01
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| ServiceId | string | 否 | 服务 ID |

### 响应

```json
{
  "Result": {
    "Quotas": [
      {
        "ServiceId": "tts",
        "ServiceName": "语音合成",
        "QPS": {
          "Limit": 100,
          "Used": 45
        },
        "Concurrency": {
          "Limit": 50,
          "Used": 12
        },
        "CharacterQuota": {
          "Total": 10000000,
          "Used": 2500000,
          "Remaining": 7500000
        }
      }
    ]
  }
}
```

---

## UsageMonitoring

查询调用量统计。

### 请求

```
GET https://open.volcengineapi.com/?Action=UsageMonitoring&Version=2024-01-01
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| ServiceId | string | 否 | 服务 ID |
| StartTime | string | 是 | 开始时间 (RFC3339) |
| EndTime | string | 是 | 结束时间 (RFC3339) |
| Granularity | string | 否 | 粒度：`hour`/`day`/`month` |

### 响应

```json
{
  "Result": {
    "ServiceId": "tts",
    "StartTime": "2024-01-01T00:00:00Z",
    "EndTime": "2024-01-31T23:59:59Z",
    "Granularity": "day",
    "DataPoints": [
      {
        "Timestamp": "2024-01-01T00:00:00Z",
        "Requests": 10000,
        "Characters": 500000,
        "Duration": 3600000,
        "SuccessRate": 0.998
      },
      {
        "Timestamp": "2024-01-02T00:00:00Z",
        "Requests": 12000,
        "Characters": 600000,
        "Duration": 4200000,
        "SuccessRate": 0.999
      }
    ],
    "Summary": {
      "TotalRequests": 320000,
      "TotalCharacters": 15000000,
      "TotalDuration": 108000000,
      "AverageSuccessRate": 0.9985
    }
  }
}
```

---

## QPS/并发查询

实时查询当前 QPS 和并发使用情况。

### 请求

```
GET https://open.volcengineapi.com/?Action=QPSMonitoring&Version=2024-01-01
```

### 响应

```json
{
  "Result": {
    "CurrentQPS": 45,
    "MaxQPS": 100,
    "CurrentConcurrency": 12,
    "MaxConcurrency": 50,
    "Timestamp": "2024-01-15T10:30:00Z"
  }
}
```
