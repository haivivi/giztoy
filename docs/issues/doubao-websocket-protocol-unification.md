# Issue: 统一 Doubao WebSocket V3 二进制协议实现

## 背景

`doubaospeech` 包中已有一个完整的二进制协议实现 `protocol.go`，定义了 V3 WebSocket 协议的完整结构，包括：

- Header 格式（version, message_type, flags, serialization, compression）
- 可选字段支持（sequence, event, sessionID, connectID）
- 压缩/解压（gzip）
- 完整的 marshal/unmarshal 方法

但是，多个 V3 API 的实现文件各自重复实现了协议编解码逻辑。

## 当前状态

| 文件 | V3 端点 | 协议实现方式 |
|------|---------|-------------|
| `realtime.go` | `/api/v3/realtime/dialogue` | ✅ 使用 `binaryProtocol` |
| `tts_v2.go` | `/api/v3/tts/bidirection` | ❌ 自己用 `uint32` 位运算构建 header |
| `podcast.go` | `/api/v3/sami/podcasttts` | ❌ 自己用 `[]byte` 构建 header |
| `asr_v2.go` | `/api/v3/sauc/bigmodel` | ❌ 自己用 `[]byte` 构建 header（带 sequence） |

## 协议格式参考

```
Header (4 bytes):
  - Byte 0: (4bits) version + (4bits) header_size
  - Byte 1: (4bits) message_type + (4bits) message_type_flags  
  - Byte 2: (4bits) serialization + (4bits) compression
  - Byte 3: reserved

Payload:
  - [optional] sequence (4 bytes) - 根据 flags 决定
  - [optional] event (4 bytes) - 根据 flags 决定
  - [optional] session_id (4 bytes len + data)
  - payload_size (4 bytes) + payload_data
```

### Message Types

| 类型 | 值 | 说明 |
|------|-----|------|
| Full Client Request | 0x01 | 完整客户端请求（JSON） |
| Audio Only Client | 0x02 | 纯音频数据 |
| Full Server Response | 0x09 | 完整服务端响应（JSON） |
| Audio Only Server | 0x0B | 纯音频响应 |
| Frontend Result | 0x0C | 前端结果（ASR 中间结果） |
| Error | 0x0F | 错误消息 |

### Message Type Flags

| Flag | 值 | 说明 |
|------|-----|------|
| No Sequence | 0x0 | 无 sequence 字段 |
| Positive Sequence | 0x1 | 有正向 sequence |
| Negative Sequence | 0x2 | 有负向 sequence |
| With Event | 0x4 | 有 event 字段 |

## 问题

1. **代码重复**：每个文件都有自己的 header 构建和解析逻辑
2. **不一致性**：不同文件使用不同的方式（uint32 位运算 vs []byte）
3. **维护困难**：协议变更需要修改多处代码
4. **容易出错**：手写 magic bytes 容易出错（如 `asr_v2.go` 的 sequence 处理）

## 建议方案

### 方案 A：扩展现有 `binaryProtocol`

让 `tts_v2.go`, `podcast.go`, `asr_v2.go` 都使用 `protocol.go` 中的 `binaryProtocol`：

```go
// 示例：asr_v2.go 使用 binaryProtocol
proto := newBinaryProtocol()
msg := &message{
    msgType: msgTypeFullClient,
    flags:   msgFlagPosSequence,  // ASR 需要 sequence
    sequence: 1,
    payload:  jsonPayload,
}
data, err := proto.marshal(msg)
```

### 方案 B：创建高层 API

在 `protocol.go` 中添加便捷方法：

```go
// 创建完整请求消息
func (p *binaryProtocol) NewFullRequest(payload []byte) *message
func (p *binaryProtocol) NewFullRequestWithSeq(payload []byte, seq int32) *message

// 创建音频消息
func (p *binaryProtocol) NewAudioOnly(audio []byte) *message
func (p *binaryProtocol) NewLastAudio(audio []byte) *message
```

## 实施步骤

1. [ ] 审查 `protocol.go` 是否完整覆盖所有 V3 API 的协议需求
2. [ ] 为缺失的 message type 和 flags 添加常量
3. [ ] 重构 `asr_v2.go` 使用 `binaryProtocol`
4. [ ] 重构 `tts_v2.go` 使用 `binaryProtocol`
5. [ ] 重构 `podcast.go` 使用 `binaryProtocol`
6. [ ] 添加单元测试验证协议编解码
7. [ ] 删除各文件中重复的协议代码

## 优先级

低 - 当前实现可以正常工作，这是代码质量改进

## 相关文件

- `go/pkg/doubaospeech/protocol.go` - 现有协议实现
- `go/pkg/doubaospeech/realtime.go` - 已使用 binaryProtocol
- `go/pkg/doubaospeech/tts_v2.go` - 待重构
- `go/pkg/doubaospeech/podcast.go` - 待重构
- `go/pkg/doubaospeech/asr_v2.go` - 待重构

## 参考

- 之前的实现：`/Users/idy/Work/haivivi/x/go/src/bytedance/speech/websocket.go`
- Doubao Speech API 文档
