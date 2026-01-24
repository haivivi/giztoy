# 控制台相关接口

## 原始文档

| 文档 | 链接 |
|------|------|
| API参考首页 | https://www.volcengine.com/docs/6561/1770994 |
| 音色 - ListBigModelTTSTimbres | https://www.volcengine.com/docs/6561/1770994 |
| 音色 - ListSpeakers (新接口) | https://www.volcengine.com/docs/6561/2160690 |
| 声音复刻 - ListMegaTTSTrainStatus | https://www.volcengine.com/docs/6561/1772920 |
| 声音复刻 - BatchListMegaTTSTrainStatus | https://www.volcengine.com/docs/6561/1772921 |
| api_key - ListAPIKeys | https://www.volcengine.com/docs/6561/1772905 |
| api_key - CreateAPIKey | https://www.volcengine.com/docs/6561/1772906 |
| service - ServiceStatus | https://www.volcengine.com/docs/6561/1772909 |
| 监控 - QuotaMonitoring | https://www.volcengine.com/docs/6561/1772924 |
| 监控 - UsageMonitoring | https://www.volcengine.com/docs/6561/1772925 |

> ⚠️ 如果本文档信息不准确，请以上述官方链接为准。

## 概述

控制台 API 用于管理语音服务的资源、配额、API Key 等。

**重要**: 控制台 API 与语音 API 使用**不同的鉴权方式**：

| API 类型 | Base URL | 鉴权方式 |
|----------|----------|----------|
| 语音 API | `openspeech.bytedance.com` | Bearer Token / API Key |
| 控制台 API | `open.volcengineapi.com` | **AK/SK 签名** |

## API 版本

> ⚠️ **重要**: 不同 API 使用不同的版本号！

| Action | Version | 说明 |
|--------|---------|------|
| `ListBigModelTTSTimbres` | **2025-05-20** | 大模型音色列表 |
| `ListSpeakers` | **2025-05-20** | 说话人列表（新接口） |
| `ListMegaTTSTrainStatus` | **2023-11-07** | 查询复刻状态 |
| `BatchListMegaTTSTrainStatus` | **2023-11-07** | 批量查询复刻状态 |

## 接口分类

### 音色管理

| 接口 | Version | 说明 |
|------|---------|------|
| ListBigModelTTSTimbres | 2025-05-20 | 获取大模型音色列表（ICL 复刻音色） |
| ListSpeakers | 2025-05-20 | 获取大模型音色列表（TTS 2.0 音色，推荐） |

### API Key 管理

| 接口 | 说明 |
|------|------|
| ListAPIKeys | 拉取 API Key 列表 |
| CreateAPIKey | 创建 API Key |
| DeleteAPIKey | 删除 API Key |
| UpdateAPIKey | 更新 API Key |

### 服务管理

| 接口 | 说明 |
|------|------|
| ServiceStatus | 查询服务状态 |
| PauseService | 暂停服务 |
| ResumeService | 重新启用服务 |
| ActivateService | 开通服务 |
| TerminateService | 停用服务 |

### 声音复刻管理

| 接口 | Version | 说明 |
|------|---------|------|
| ListMegaTTSTrainStatus | 2023-11-07 | 查询单个 SpeakerID 状态 |
| BatchListMegaTTSTrainStatus | 2023-11-07 | 分页查询所有 SpeakerID 状态 |
| OrderAccessResourcePacks | - | 音色下单 |
| RenewAccessResourcePacks | - | 音色续费 |

### 监控

| 接口 | 说明 |
|------|------|
| QuotaMonitoring | Quota 查询接口 |
| UsageMonitoring | 调用量查询接口 |

## 鉴权方式

控制台 API 使用火山引擎 OpenAPI 标准鉴权（HMAC-SHA256 V4 签名）。

### 获取凭证

1. 登录火山引擎控制台
2. 进入 IAM 访问控制 -> 密钥管理
3. 创建 Access Key（AK）和 Secret Key（SK）

### 请求示例

```bash
curl -X POST "https://open.volcengineapi.com/?Action=ListSpeakers&Version=2025-05-20" \
  -H "Content-Type: application/json" \
  -H "Authorization: HMAC-SHA256 Credential={AK}/{Date}/{Region}/speech_saas_prod/request, SignedHeaders=..., Signature=..." \
  -H "X-Date: 20260119T120000Z" \
  -H "Host: open.volcengineapi.com" \
  -H "X-Content-Sha256: {body-sha256}" \
  -d '{}'
```

### 签名参数

| 参数 | 值 |
|------|-----|
| Service | `speech_saas_prod` |
| Region | `cn-north-1` |
| Algorithm | `HMAC-SHA256` |

详见：[火山引擎 OpenAPI 鉴权](https://www.volcengine.com/docs/6369/65269)

## 详细接口文档

- [音色接口](./timbre.md)
- [API Key 接口](./apikey.md)
- [服务管理接口](./service.md)
- [监控接口](./monitoring.md)

## Go SDK 使用

```go
import "github.com/haivivi/giztoy/pkg/doubaospeech"

// 创建控制台客户端（使用 AK/SK）
console := doubaospeech.NewConsole(accessKey, secretKey)

// 列出 TTS 2.0 音色
speakers, err := console.ListSpeakers(ctx, &doubaospeech.ListSpeakersRequest{
    PageNumber: 1,
    PageSize:   20,
})

// 列出复刻音色
timbres, err := console.ListTimbres(ctx, &doubaospeech.ListTimbresRequest{})

// 查询复刻状态
status, err := console.ListVoiceCloneStatus(ctx, &doubaospeech.ListVoiceCloneStatusRequest{
    AppID: "your_app_id",
})
```
