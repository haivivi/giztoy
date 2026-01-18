# 控制台相关接口

## 原始文档

| 文档 | 链接 |
|------|------|
| API参考首页 | https://www.volcengine.com/docs/6561/1772902 |
| 音色 - ListBigModelTTSTimbres | https://www.volcengine.com/docs/6561/1772903 |
| 音色 - ListSpeakers | https://www.volcengine.com/docs/6561/1772904 |
| api_key - ListAPIKeys | https://www.volcengine.com/docs/6561/1772905 |
| api_key - CreateAPIKey | https://www.volcengine.com/docs/6561/1772906 |
| api_key - DeleteAPIKey | https://www.volcengine.com/docs/6561/1772907 |
| api_key - UpdateAPIKey | https://www.volcengine.com/docs/6561/1772908 |
| service - ServiceStatus | https://www.volcengine.com/docs/6561/1772909 |
| service - PauseService | https://www.volcengine.com/docs/6561/1772910 |
| service - ResumeService | https://www.volcengine.com/docs/6561/1772911 |
| service - ActivateService | https://www.volcengine.com/docs/6561/1772912 |
| service - TerminateService | https://www.volcengine.com/docs/6561/1772913 |
| 声音复刻 - ListMegaTTSTrainStatus | https://www.volcengine.com/docs/6561/1772920 |
| 声音复刻 - BatchListMegaTTSTrainStatus | https://www.volcengine.com/docs/6561/1772921 |
| 监控 - QuotaMonitoring | https://www.volcengine.com/docs/6561/1772924 |
| 监控 - UsageMonitoring | https://www.volcengine.com/docs/6561/1772925 |
| QPS/并发查询接口说明 | https://www.volcengine.com/docs/6561/1772926 |
| 调用量查询接口说明 | https://www.volcengine.com/docs/6561/1772927 |

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 概述

控制台相关接口用于管理语音服务的资源、配额、API Key 等。这些接口通常使用火山引擎 OpenAPI 标准鉴权方式。

## 接口分类

### 音色管理

| 接口 | 说明 |
|------|------|
| ListBigModelTTSTimbres | 获取大模型音色列表 |
| ListSpeakers | 获取大模型音色列表（新接口） |

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

### 资源包管理

| 接口 | 说明 |
|------|------|
| FormalizeResourcePacks | 转正资源包 |
| ResourcePacksStatus | 资源包状态信息 |
| AliasResourcePack | 更新音色资源别名 |
| OrderResourcePacks | 购买资源包 |

### 标签管理

| 接口 | 说明 |
|------|------|
| ListTagsForResources | 查询资源所附加的全部标签 |
| UntagResources | 对资源进行标签移除 |
| TagResources | 对资源进行标签附加 |

### 声音复刻管理

| 接口 | 说明 |
|------|------|
| ListMegaTTSTrainStatus | 查询 SpeakerID 状态信息 |
| BatchListMegaTTSTrainStatus | 分页查询 SpeakerID 状态 |
| OrderAccessResourcePacks | 音色下单 |
| RenewAccessResourcePacks | 音色续费 |
| ListMegaTTSByOrderID | 通过订单ID查询购买的音色 |

### 监控

| 接口 | 说明 |
|------|------|
| QuotaMonitoring | Quota 查询接口 |
| UsageMonitoring | 调用量查询接口 |

## 鉴权方式

控制台接口使用火山引擎 OpenAPI 标准鉴权，与语音 API 的鉴权方式不同。

详见：[火山引擎 OpenAPI 鉴权](https://www.volcengine.com/docs/6369/65269)

## 详细接口文档

- [音色接口](./timbre.md)
- [API Key 接口](./apikey.md)
- [服务管理接口](./service.md)
- [监控接口](./monitoring.md)
