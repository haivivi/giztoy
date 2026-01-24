# 豆包语音（Doubao Speech）API 文档

## 原始文档

- **文档首页**: https://www.volcengine.com/docs/6561/162929
- **控制台**: https://console.volcengine.com/speech/app

> 如果本文档信息不完整，请访问上述链接获取最新内容。

## 产品体系

豆包语音分为两代产品：**大模型版（2.0）** 和 **经典版（1.0）**。推荐使用大模型版。

---

## 语音合成（TTS）

### 大模型语音合成 2.0

| 接口 | 端点 | Resource ID | 文档 |
|------|------|-------------|------|
| 单向流式 HTTP V3 | `POST /api/v3/tts/unidirectional` | `seed-tts-2.0` | [stream-http.md](./tts2.0/stream-http.md) |
| 单向流式 WebSocket V3 | `WSS /api/v3/tts/unidirectional` | `seed-tts-2.0` | [stream-ws.md](./tts2.0/stream-ws.md) |
| 双向流式 WebSocket V3 | `WSS /api/v3/tts/bidirection` | `seed-tts-2.0` | [duplex-ws.md](./tts2.0/duplex-ws.md) |
| 异步长文本 | `POST /api/v3/tts/async/submit` | `seed-tts-2.0-concurr` | [async.md](./tts2.0/async.md) |

**原始文档链接：**
- https://www.volcengine.com/docs/6561/1598757 (单向流式HTTP-V3)
- https://www.volcengine.com/docs/6561/1719100 (单向流式WebSocket-V3)
- https://www.volcengine.com/docs/6561/1329505 (双向流式WebSocket-V3)
- https://www.volcengine.com/docs/6561/1330194 (异步长文本)

### 经典版语音合成 1.0

| 接口 | 端点 | Cluster | 文档 |
|------|------|---------|------|
| HTTP 一次性合成 | `POST /api/v1/tts` | `volcano_tts` | [http.md](./tts1.0/http.md) |
| WebSocket 流式 | `WSS /api/v1/tts/ws_binary` | `volcano_tts` | [websocket.md](./tts1.0/websocket.md) |

**原始文档链接：**
- https://www.volcengine.com/docs/6561/79820 (HTTP接口)
- https://www.volcengine.com/docs/6561/79821 (WebSocket接口)
- https://www.volcengine.com/docs/6561/97465 (参数说明)

### 精品长文本语音合成

| 接口 | 端点 | 文档 |
|------|------|------|
| 异步长文本 | `POST /api/v1/long_tts/submit` | [long-tts.md](./tts1.0/long-tts.md) |

**原始文档链接：**
- https://www.volcengine.com/docs/6561/1096680

---

## 语音识别（ASR）

### 大模型语音识别 2.0

| 接口 | 端点 | Resource ID | 文档 |
|------|------|-------------|------|
| 流式识别 WebSocket | `WSS /api/v3/sauc/bigmodel` | `volc.bigasr.sauc.duration` | [streaming.md](./asr2.0/streaming.md) |
| 录音文件识别（标准版）| `POST /api/v3/asr/bigmodel/submit` | `volc.bigasr.auc.duration` | [file-standard.md](./asr2.0/file-standard.md) |
| 录音文件识别（极速版）| `POST /api/v3/asr/bigmodel_async/submit` | `volc.bigasr.auc.duration` | [file-fast.md](./asr2.0/file-fast.md) |

**原始文档链接：**
- https://www.volcengine.com/docs/6561/1354869 (大模型流式语音识别)
- https://www.volcengine.com/docs/6561/1354868 (大模型录音文件识别标准版)
- https://www.volcengine.com/docs/6561/1631584 (大模型录音文件极速版)
- https://www.volcengine.com/docs/6561/1840838 (大模型录音文件闲时版)

### 经典版语音识别 1.0

| 接口 | 端点 | Cluster | 文档 |
|------|------|---------|------|
| 一句话识别 | `POST /api/v1/asr` | `volcengine_input_common` | [one-sentence.md](./asr1.0/one-sentence.md) |
| 流式识别 | `WSS /api/v2/asr` | `volcengine_streaming_common` | [streaming.md](./asr1.0/streaming.md) |
| 录音文件标准版 | `POST /api/v1/asr/submit` | `volc.megatts.default` | [file-standard.md](./asr1.0/file-standard.md) |
| 录音文件极速版 | `POST /api/v1/asr/async/submit` | `volc.megatts.default` | [file-fast.md](./asr1.0/file-fast.md) |

**原始文档链接：**
- https://www.volcengine.com/docs/6561/104897 (一句话识别)
- https://www.volcengine.com/docs/6561/80816 (流式语音识别)
- https://www.volcengine.com/docs/6561/80818 (录音文件识别标准版)
- https://www.volcengine.com/docs/6561/80820 (录音文件识别极速版)

---

## 声音复刻

| 接口 | 端点 | Cluster | 文档 |
|------|------|---------|------|
| 训练提交 | `POST /api/v1/mega_tts/audio/upload` | `volcano_icl` | [api.md](./voice-clone/api.md) |
| 状态查询 | `POST /api/v1/mega_tts/status` | `volcano_icl` | [api.md](./voice-clone/api.md) |
| 激活音色 | `POST /api/v1/mega_tts/audio/activate` | `volcano_icl` | [api.md](./voice-clone/api.md) |

**原始文档链接：**
- https://www.volcengine.com/docs/6561/1305191 (声音复刻API)
- https://www.volcengine.com/docs/6561/1829010 (声音复刻下单及使用指南)

---

## 实时语音大模型

| 接口 | 端点 | Resource ID | 文档 |
|------|------|-------------|------|
| 实时对话 | `WSS /api/v3/realtime/dialogue` | `volc.speech.dialog` | [api.md](./realtime/api.md) |

**原始文档链接：**
- https://www.volcengine.com/docs/6561/1257584 (端到端实时语音大模型API)

---

## 播客合成

| 接口 | 端点 | Resource ID | 文档 |
|------|------|-------------|------|
| WebSocket V3 | `WSS /api/v3/sami/podcasttts` | `volc.megatts.podcast` | [api.md](./podcast/api.md) |

**原始文档链接：**
- https://www.volcengine.com/docs/6561/1668014 (播客API-websocket-v3协议)

---

## 同声传译

| 接口 | 端点 | Resource ID | 文档 |
|------|------|-------------|------|
| WebSocket V3 | `WSS /api/v3/saas/simt` | `volc.megatts.simt` | [api.md](./translation/api.md) |

**原始文档链接：**
- https://www.volcengine.com/docs/6561/xxx (同声传译2.0-API)

---

## 语音妙记（会议纪要）

| 接口 | 端点 | 文档 |
|------|------|------|
| 异步提交 | `POST /api/v1/meeting/submit` | [api.md](./meeting/api.md) |

**原始文档链接：**
- https://www.volcengine.com/docs/6561/xxx (豆包语音妙记-API)

---

## 音视频字幕

| 接口 | 端点 | 文档 |
|------|------|------|
| 字幕生成 | `POST /api/v1/subtitle/submit` | [subtitle.md](./media/subtitle.md) |
| 字幕打轴 | `POST /api/v1/subtitle/align` | [align.md](./media/align.md) |

**原始文档链接：**
- https://www.volcengine.com/docs/6561/192519 (音视频字幕生成)
- https://www.volcengine.com/docs/6561/113635 (自动字幕打轴)

---

## 控制台管理 API

| 接口 | 端点 | 认证方式 | 文档 |
|------|------|----------|------|
| 大模型音色列表 | `POST /ListBigModelTTSTimbres` | AK/SK | [timbre.md](./console-api/timbre.md) |
| 大模型音色列表(新) | `POST /ListSpeakers` | AK/SK | [timbre.md](./console-api/timbre.md) |
| API Key 管理 | `POST /ListAPIKeys` | AK/SK | [apikey.md](./console-api/apikey.md) |
| 服务状态管理 | `POST /ServiceStatus` | AK/SK | [service.md](./console-api/service.md) |
| 配额监控 | `POST /QuotaMonitoring` | AK/SK | [monitoring.md](./console-api/monitoring.md) |
| 声音复刻状态 | `POST /ListMegaTTSTrainStatus` | AK/SK | [voice-clone-status.md](./console-api/voice-clone-status.md) |

**原始文档链接：**
- https://www.volcengine.com/docs/6561/1770994 (ListBigModelTTSTimbres)
- https://www.volcengine.com/docs/6561/2160690 (ListSpeakers)

---

## 认证方式

### Speech API（语音服务）

语音服务使用以下认证方式：

| 认证方式 | Header | 适用场景 |
|----------|--------|----------|
| Access Token | `Authorization: Bearer; {token}` | HTTP/WebSocket V1-V2 |
| X-Api 认证 | `X-Api-App-Id`, `X-Api-Access-Key` | WebSocket V3 |
| Request Body | `app.token` | 部分 HTTP 接口 |

### Console API（控制台服务）

控制台 API 使用 **Volcengine OpenAPI AK/SK 签名认证**：

```
Authorization: HMAC-SHA256 Credential={AccessKeyId}/...
```

详见 [auth.md](./auth.md)

---

## 快速选择

| 需求 | 推荐接口 | 文档 |
|------|----------|------|
| 短文本实时合成 | TTS 2.0 单向流式 HTTP V3 | [stream-http.md](./tts2.0/stream-http.md) |
| 长文本批量合成 | TTS 2.0 异步接口 | [async.md](./tts2.0/async.md) |
| 实时语音交互 | 实时对话 API | [realtime/api.md](./realtime/api.md) |
| 定制音色 | 声音复刻 API | [voice-clone/api.md](./voice-clone/api.md) |
| 实时语音识别 | ASR 2.0 流式 | [asr2.0/streaming.md](./asr2.0/streaming.md) |
| 录音文件转写 | ASR 2.0 文件识别 | [asr2.0/file-standard.md](./asr2.0/file-standard.md) |
| 播客生成 | 播客 API | [podcast/api.md](./podcast/api.md) |
