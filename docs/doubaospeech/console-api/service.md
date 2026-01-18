# 服务管理接口

## 原始文档

| 接口 | 链接 |
|------|------|
| ServiceStatus | https://www.volcengine.com/docs/6561/1772909 |
| PauseService | https://www.volcengine.com/docs/6561/1772910 |
| ResumeService | https://www.volcengine.com/docs/6561/1772911 |
| ActivateService | https://www.volcengine.com/docs/6561/1772912 |
| TerminateService | https://www.volcengine.com/docs/6561/1772913 |

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## ServiceStatus

查询服务状态。

### 请求

```
GET https://open.volcengineapi.com/?Action=ServiceStatus&Version=2024-01-01
```

### 响应

```json
{
  "Result": {
    "Status": "active",
    "ActivatedAt": "2024-01-01T00:00:00Z",
    "Services": [
      {
        "ServiceId": "tts",
        "ServiceName": "语音合成",
        "Status": "active"
      },
      {
        "ServiceId": "asr",
        "ServiceName": "语音识别",
        "Status": "active"
      }
    ]
  }
}
```

### 状态枚举

| 状态 | 说明 |
|------|------|
| active | 正常运行 |
| paused | 已暂停 |
| terminated | 已停用 |
| pending | 待开通 |

---

## ActivateService

开通服务。

### 请求

```
POST https://open.volcengineapi.com/?Action=ActivateService&Version=2024-01-01
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| ServiceId | string | 是 | 服务 ID |

---

## PauseService

暂停服务。

### 请求

```
POST https://open.volcengineapi.com/?Action=PauseService&Version=2024-01-01
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| ServiceId | string | 是 | 服务 ID |

---

## ResumeService

恢复已暂停的服务。

### 请求

```
POST https://open.volcengineapi.com/?Action=ResumeService&Version=2024-01-01
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| ServiceId | string | 是 | 服务 ID |

---

## TerminateService

停用服务。

### 请求

```
POST https://open.volcengineapi.com/?Action=TerminateService&Version=2024-01-01
```

### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| ServiceId | string | 是 | 服务 ID |

**警告**：停用服务后，相关资源将被清理，此操作不可逆。
