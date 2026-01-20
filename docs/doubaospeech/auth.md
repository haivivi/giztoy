# 鉴权方式

## 概述

豆包语音服务有两类 API，使用不同的认证方式：

| API 类型 | 认证方式 | 说明 |
|----------|----------|------|
| **Speech API** | Bearer Token / X-Api Headers | 语音合成、识别等核心服务 |
| **Console API** | Volcengine AK/SK 签名 | 控制台管理功能 |

---

## Speech API 认证

### 方式一：V3 API Headers 认证（推荐）

适用于 V3 版本的 API（`/api/v3/*`）

| Header | 说明 | 必填 |
|--------|------|------|
| `X-Api-App-Id` | APP ID | ✅ |
| `X-Api-Access-Key` | Access Token | ✅ |
| `X-Api-Resource-Id` | 资源 ID | ✅ |
| `X-Api-Request-Id` | 请求 ID | |
| `X-Api-Connect-Id` | 连接 ID | |

**示例：**

```http
POST /api/v3/tts/unidirectional HTTP/1.1
Host: openspeech.bytedance.com
X-Api-App-Id: 123456789
X-Api-Access-Key: your-access-token
X-Api-Resource-Id: seed-tts-2.0
Content-Type: application/json
```

### 方式二：Bearer Token 认证

适用于 V1/V2 版本的 API（`/api/v1/*`, `/api/v2/*`）

| Header | 格式 |
|--------|------|
| `Authorization` | `Bearer; {token}` |

> ⚠️ 注意：格式是 `Bearer;`（分号），不是 `Bearer `（空格）

**示例：**

```http
POST /api/v1/tts HTTP/1.1
Host: openspeech.bytedance.com
Authorization: Bearer; your-access-token
Content-Type: application/json
```

### 方式三：URL 参数认证

适用于 WebSocket 连接（V1/V2）

```
wss://openspeech.bytedance.com/api/v1/tts/ws_binary?appid={appid}&token={token}&cluster={cluster}
```

### 方式四：Request Body 认证

部分 V1 HTTP 接口需要在请求体中包含认证信息：

```json
{
    "app": {
        "appid": "123456789",
        "token": "your-access-token",
        "cluster": "volcano_tts"
    },
    ...
}
```

---

## Console API 认证

Console API 使用 **Volcengine OpenAPI V4 签名**（HMAC-SHA256）。

### 所需凭证

| 凭证 | 说明 | 获取方式 |
|------|------|----------|
| Access Key ID | AK | 火山引擎控制台 - 密钥管理 |
| Secret Access Key | SK | 火山引擎控制台 - 密钥管理 |

### 签名流程

1. 构建规范请求字符串（Canonical Request）
2. 构建待签名字符串（String to Sign）
3. 计算签名（Signature）
4. 构建 Authorization Header

### Authorization Header 格式

```
Authorization: HMAC-SHA256 Credential={AccessKeyId}/{Date}/{Region}/{Service}/request, SignedHeaders={SignedHeaders}, Signature={Signature}
```

### 示例

```http
POST / HTTP/1.1
Host: open.volcengineapi.com
Content-Type: application/json
X-Date: 20260119T100000Z
Authorization: HMAC-SHA256 Credential=AKLT.../20260119/cn-north-1/speech_saas_prod/request, SignedHeaders=content-type;host;x-date, Signature=xxx
X-Action: ListSpeakers
X-Version: 2025-05-20

{"AppId":"123456789"}
```

---

## 凭证获取

### 获取 APP ID 和 Access Token

1. 登录 [火山引擎控制台](https://console.volcengine.com/speech/app)
2. 创建或选择应用
3. 在应用详情页获取 APP ID 和 Access Token

### 获取 AK/SK

1. 登录 [火山引擎控制台](https://console.volcengine.com)
2. 点击右上角头像 - 密钥管理
3. 创建或获取 Access Key

---

## Resource ID 对照表

### TTS 服务

| Resource ID | 说明 |
|-------------|------|
| `seed-tts-1.0` | 大模型语音合成 1.0（字符版）|
| `seed-tts-1.0-concurr` | 大模型语音合成 1.0（并发版）|
| `seed-tts-2.0` | 大模型语音合成 2.0（字符版）|
| `seed-tts-2.0-concurr` | 大模型语音合成 2.0（并发版）|
| `seed-icl-1.0` | 声音复刻 1.0（字符版）|
| `seed-icl-2.0` | 声音复刻 2.0（字符版）|
| `volc.service_type.10029` | 大模型语音合成（旧 ID）|

### ASR 服务

| Resource ID | 说明 |
|-------------|------|
| `volc.bigasr.sauc.duration` | 大模型流式语音识别 |
| `volc.bigasr.auc.duration` | 大模型录音文件识别 |

### 其他服务

| Resource ID | 说明 |
|-------------|------|
| `volc.speech.dialog` | 端到端实时语音大模型 |
| `volc.megatts.podcast` | 播客合成 |
| `volc.megatts.simt` | 同声传译 |

---

## Cluster 对照表（V1 API）

| Cluster | 说明 |
|---------|------|
| `volcano_tts` | TTS 1.0 标准版 |
| `volcano_mega` | TTS 2.0 大模型版 |
| `volcano_icl` | 声音复刻 |
| `volcengine_input_common` | ASR 一句话识别 |
| `volcengine_streaming_common` | ASR 流式识别 |

---

## 常见问题

### Q: Bearer Token 格式是空格还是分号？

A: 是 `Bearer;`（分号后直接跟 token），不是 `Bearer `（空格）。这是火山引擎的特殊格式。

### Q: V1 和 V3 API 的区别？

A: V3 API 使用 `X-Api-*` Headers 认证，更标准化；V1 API 使用 Bearer Token 或 URL 参数认证。

### Q: Console API 和 Speech API 的 Token 一样吗？

A: 不一样。Speech API 使用 Access Token，Console API 使用 AK/SK。
