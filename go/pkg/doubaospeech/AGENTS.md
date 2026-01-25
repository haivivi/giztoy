# AGENTS.md - 豆包语音 SDK 开发指南

> 本文档面向 AI Agent（如 Claude、Cursor Agent），帮助理解豆包语音 SDK 的架构和常见问题。

## 快速参考

### ⚠️ 最重要的坑：音色与 Resource ID 必须匹配！

```
错误信息：{"code": 55000000, "message": "resource ID is mismatched with speaker related resource"}
含义：Resource ID 和音色后缀不匹配（不是"没开通"！）
```

| 音色后缀 | 必须使用的 Resource ID |
|---------|----------------------|
| `_uranus_bigtts` | `seed-tts-2.0` |
| `_moon_bigtts` | `seed-tts-1.0` |
| `_saturn_bigtts` | `seed-icl-*` 或 Podcast |
| `_v2_saturn_bigtts` | `volc.service_type.10050` (SAMI Podcast) |
| 无后缀 (如 `zh_female_cancan`) | V1 API `cluster=volcano_tts` |

### 已验证可用的组合

```go
// seed-tts-2.0 可用音色（_uranus_bigtts 后缀）
"zh_female_xiaohe_uranus_bigtts"     // ✅
"zh_female_vv_uranus_bigtts"         // ✅
"zh_male_taocheng_uranus_bigtts"     // ✅

// seed-tts-1.0 可用音色（_moon_bigtts 后缀）
"zh_female_shuangkuaisisi_moon_bigtts" // ✅

// SAMI Podcast 可用音色（_v2_saturn_bigtts 后缀）
"zh_male_dayixiansheng_v2_saturn_bigtts"   // ✅
"zh_female_mizaitongxue_v2_saturn_bigtts"  // ✅
```

---

## 服务架构

### 文件结构

```
go/pkg/doubaospeech/
├── client.go          # 客户端入口，认证配置
├── tts.go             # TTS V1 服务
├── tts_v2.go          # TTS V2 服务（HTTP + WebSocket）
├── asr.go             # ASR V1 服务
├── asr_v2.go          # ASR V2 服务（WebSocket + 文件）
├── podcast.go         # Podcast 服务（HTTP + SAMI WebSocket）
├── realtime.go        # Realtime 实时对话
├── voice.go           # 声音复刻
├── protocol.go        # V3 二进制协议（WebSocket 消息编解码）
├── console.go         # Console API（AK/SK 签名）
├── errors.go          # 错误类型
└── AGENTS.md          # 本文档
```

### 服务类型对照

| Go 服务 | API 版本 | 端点 | Resource ID |
|--------|---------|------|-------------|
| `client.TTS` | V1 | `/api/v1/tts` | cluster |
| `client.TTSV2.Stream()` | V2 HTTP | `/api/v3/tts/unidirectional` | `seed-tts-*` |
| `client.TTSV2.OpenSession()` | V2 WS 双向 | `/api/v3/tts/bidirection` | `seed-tts-*` |
| `client.ASR` | V1 | `/api/v1/asr` | cluster |
| `client.ASRV2.Stream()` | V2 WS | `/api/v3/sauc/bigmodel` | `volc.bigasr.sauc.duration` |
| `client.Podcast.Create()` | V1 HTTP | `/api/v1/podcast/*` | N/A |
| `client.Podcast.StreamSAMI()` | V3 WS | `/api/v3/sami/podcasttts` | `volc.service_type.10050` |
| `client.Realtime.Connect()` | V3 WS | `/api/v3/realtime/dialogue` | `volc.speech.dialog` |

---

## 认证方式

### V1 API（经典版）

```go
// Bearer Token
client := ds.NewClient(appID, ds.WithBearerToken(token))
// 请求头: Authorization: Bearer {token}
```

### V2/V3 API（大模型版）

```go
// 使用 Bearer Token
client := ds.NewClient(appID, ds.WithBearerToken(token))
// 请求头:
//   X-Api-App-Id: {app_id}
//   X-Api-Access-Key: {token}
//   X-Api-Resource-Id: {resource_id}  // 在请求中指定
```

### 特殊服务的固定 Header

| 服务 | 额外 Header | 固定值 | 代码位置 |
|------|------------|--------|---------|
| SAMI Podcast | `X-Api-App-Key` | `aGjiRDfUWi` | `podcast.go:607` |
| Realtime | `X-Api-App-Key` | `BYsHlwdHqc` | `client.go:283` |

### Console API（与语音 API 不同！）

```go
// 需要火山引擎 AK/SK
console := ds.NewConsole(accessKey, secretKey)
// 使用 HMAC-SHA256 签名
```

---

## Resource ID 对照表

### TTS 服务

| Resource ID | 别名 | 说明 | 音色要求 |
|-------------|------|------|---------|
| `seed-tts-1.0` | `volc.service_type.10029` | TTS 1.0 字符版 | `*_moon_bigtts` |
| `seed-tts-1.0-concurr` | `volc.service_type.10048` | TTS 1.0 并发版 | `*_moon_bigtts` |
| `seed-tts-2.0` | - | TTS 2.0 字符版 | `*_uranus_bigtts` |
| `seed-tts-2.0-concurr` | - | TTS 2.0 并发版 | `*_uranus_bigtts` |
| `seed-icl-1.0` | - | 声音复刻 1.0 | 复刻音色 |
| `seed-icl-2.0` | - | 声音复刻 2.0 | `*_saturn_bigtts` |

### 其他服务

| Resource ID | 说明 | 代码常量 |
|-------------|------|---------|
| `volc.bigasr.sauc.duration` | ASR 流式 | `ResourceASRStream` |
| `volc.bigasr.auc.duration` | ASR 文件 | `ResourceASRFile` |
| `volc.speech.dialog` | Realtime | `ResourceRealtime` |
| `volc.service_type.10050` | SAMI Podcast | 硬编码在 `podcast.go` |

---

## 二进制协议（protocol.go）

### V3 WebSocket 消息格式

```
Header (4 bytes):
  Byte 0: version (high nibble) | header_size (low nibble) = 0x11
  Byte 1: msg_type (high nibble) | flags (low nibble)
  Byte 2: serialization (high nibble) | compression (low nibble)
  Byte 3: reserved = 0x00

Optional fields (depends on flags):
  - sequence (4 bytes, if flags & 0x01)
  - event (4 bytes, if flags & 0x04)
  - session_id (4 bytes length + string, if event-based)

Payload:
  - payload_size (4 bytes)
  - payload (JSON or binary)
```

### 消息类型

```go
msgTypeFullClient      = 0x1  // 客户端完整请求
msgTypeAudioOnlyClient = 0x2  // 客户端仅音频
msgTypeFullServer      = 0x9  // 服务端完整响应
msgTypeAudioOnlyServer = 0xB  // 服务端仅音频
msgTypeError           = 0xF  // 错误响应
```

### 注意事项

- TTS V2 双向 WebSocket 使用事件驱动协议，不同于通用的 V3 协议
- SAMI Podcast 也使用事件驱动协议，事件码不同于 TTS V2
- ASR V2 的音频帧不包含序列号（即使有 `isLast` 标志）

---

## 常见错误及解决

### 1. Resource ID 与音色不匹配

```
Error: resource ID is mismatched with speaker related resource
```

**解决**：检查音色后缀是否与 Resource ID 对应，参考本文档开头的表格。

### 2. 服务未开通

```
Error: [resource_id=xxx] requested resource not granted
```

**解决**：在火山引擎控制台开通对应服务。

### 3. WebSocket 握手失败

```
Error: websocket: bad handshake, status=400
```

**可能原因**：
- 认证 Header 不正确
- Resource ID 缺失
- 端点 URL 错误

### 4. 连接异常关闭

```
Error: websocket: close 1006 (abnormal closure): unexpected EOF
```

**可能原因**：
- 服务端主动断开（检查服务端返回的错误信息）
- 协议格式不正确
- 超时

---

## CLI 命令结构（方案 A）

```
doubaospeech
├── tts
│   ├── v1
│   │   ├── synthesize          # TTS V1 同步
│   │   └── stream              # TTS V1 流式
│   └── v2
│       ├── stream              # TTS V2 HTTP 流式
│       ├── ws                  # TTS V2 WebSocket 单向
│       ├── bidirectional       # TTS V2 WebSocket 双向
│       └── async               # TTS V2 异步
├── asr
│   ├── v1
│   │   ├── recognize           # ASR V1 一句话
│   │   └── stream              # ASR V1 流式
│   └── v2
│       ├── stream              # ASR V2 流式
│       └── file                # ASR V2 文件
├── podcast
│   ├── http                    # Podcast HTTP
│   └── sami                    # SAMI Podcast WebSocket
├── realtime
│   └── interactive             # 实时对话
├── voice
│   ├── list                    # 列出音色
│   ├── clone                   # 声音复刻
│   └── status                  # 查询状态
└── config
    └── ...
```

---

## 开发检查清单

当修改豆包语音 SDK 时，请检查：

- [ ] Resource ID 与音色后缀是否匹配
- [ ] 认证 Header 是否正确（V1 vs V2 不同）
- [ ] 特殊服务是否需要固定 `X-Api-App-Key`
- [ ] WebSocket 协议是否正确（事件驱动 vs 通用 V3）
- [ ] 错误信息是否清晰（避免误导"未开通"）

---

## 相关文档

- 用户文档：`docs/zh/lib/doubaospeech/doc.md`
- API 文档：`docs/zh/lib/doubaospeech/api/`
- CLI 文档：`go/cmd/doubaospeech/README.md`
- 示例代码：`examples/go/doubaospeech/`
