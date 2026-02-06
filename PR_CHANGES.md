# PR 变更说明

## 一、新增目录

### 1. `go/pkg/luau/runtime/`
**Luau 异步运行时核心实现**

- `promise.go` - Promise 类型和 await/is_ready 方法
- `builtin_sleep.go` - `rt:sleep(ms)` 异步睡眠
- `builtin_timeout.go` - `rt:timeout(ms)` 超时控制，支持取消
- `builtin_http.go` - `rt:http()` 异步 HTTP 请求，支持超时
- `runtime.go` - 事件循环、PendingOp 管理
- `agent.go` - Agent 上下文支持
- `stream.go` - 流处理
- `*_test.go` - 完整测试覆盖

### 2. `go/pkg/luau/registry/`
**Luau 注册表功能**

- `registry.go` - 注册表核心实现
- `memory.go` - 内存存储
- `package.go` - 包管理

### 3. `go/pkg/genx/transformers/`
**genx Transformer 实现**

- `dashscope_realtime.go` - DashScope 实时转换器
- `doubao_asr_sauc.go` - 豆包 ASR (SAUC 协议)
- `doubao_tts_icl_v2.go` - 豆包 TTS ICL v2
- `doubao_tts_seed_v2.go` - 豆包 TTS Seed v2
- `doubao_realtime.go` - 豆包实时转换器
- `minimax_tts.go` - MiniMax TTS
- `codec_mp3_to_ogg.go` - MP3 转 OGG 编解码
- `mux.go`, `mux_asr.go`, `mux_tts.go` - 多路复用器

### 4. `go/pkg/genx/cortex/`
**chatgear + transformer 桥接层**

- `atom.go` - Atom 核心，连接 ServerPort 和 Transformer
- `stream.go` - 流处理
- `doc.go` - 文档

用途：将 chatgear 设备音频流与 genx transformer 连接，处理状态机和音频编解码。

### 5. `go/pkg/genx/input/`
**输入处理**

- `jitter_buffer.go` - 抖动缓冲器
- `opus/` - Opus 编解码支持

### 6. `go/pkg/genx/luau/`
**genx 的 Luau 集成**

- `register.go` - 注册 genx 函数到 Luau runtime
- `runner_test.go` - 运行器测试

### 7. `go/cmd/cortextest/`
**cortex 测试 CLI**

用于测试 cortex.Atom 与各种 transformer 的组合。

### 8. `examples/go/chatgear/`
**chatgear 示例**

- `dashscope_realtime_server_port/` - DashScope 实时服务端示例
- `doubao_realtime_server_port/` - 豆包实时服务端示例

### 9. `examples/go/genx/transformers/`
**transformer 使用示例**

- `asr_vad/` - ASR + VAD 示例
- `audio/` - 音频处理示例
- `dashscope_realtime/` - DashScope 实时示例
- `doubao_realtime_asr/` - 豆包实时 ASR 示例

### 10. `examples/go/genx/luau_dialogue/`
**Luau 对话示例**

展示如何用 Luau 脚本控制对话流程。

### 11. `examples/rust/speech/`
**Rust Speech 示例**

Rust 语言的语音处理示例。

### 12. `rust/luau/runtime/`
**Rust Luau Runtime**

Rust 实现的 Luau 运行时（与 Go 版本对应）。

### 13. `luau/tests/`
**Luau 测试脚本**

- `agent/` - Agent 测试
- `genx/` - genx 集成测试
- `tool/` - Tool 测试

### 14. `testdata/luau/runtime/`
**运行时测试数据**

异步功能测试脚本：
- `async_http.luau` - HTTP 异步测试
- `async_timeout.luau` - 超时测试
- `async_combinators.luau` - await_all/await_any 测试
- `async_concurrent.luau` - 并发模式测试

### 15. `testdata/models/`
**模型配置文件**

替代原来的 `testdata/matchtest/models/`，新的模型配置格式。

---

## 二、删除的目录

### 1. `embed/`
嵌入式 Zig 代码（LED strip, ESP），不再维护。

### 2. `go/pkg/audio/opusrt/`
旧的 Opus 实时处理，已被 `go/pkg/genx/input/` 替代。

### 3. `go/pkg/speech/`
旧的 Speech 包，功能已迁移到 `go/pkg/genx/transformers/`。

### 4. `go/cmd/luau/builtin_*.go`
旧的 builtin 实现，已迁移到 `go/pkg/luau/runtime/`。

### 5. `rust/cmd/luau/src/builtin/`
Rust 旧 builtin，已迁移。

### 6. `examples/go/speech/`
旧的 speech 示例，已过时。

### 7. `testdata/matchtest/models/`
旧的模型配置，已迁移到 `testdata/models/`。

---

## 三、关键修改

### 1. `luau/c/luau_wrapper.cpp`
**CGO 线程稳定性修复**

- 添加 `parentRef` 和 `parentState` 字段
- `luau_newthread()` 现在将线程存储在 registry 中防止 GC
- `luau_close_thread()` 正确释放 registry 引用

### 2. `go/pkg/luau/luau.go`
**Thread.Close() 修复**

调用 `C.luau_close_thread()` 正确释放资源。

---

## 四、待清理（本次确认删除）

| 目录/文件 | 说明 |
|----------|------|
| `go/pkg/genx/luau/` | 冗余，与 runtime 功能重复 |
| `examples/rust/speech/` | Rust speech 示例，暂不更新 |
| `examples/rust/doubao_minimax/src/bin/luau_dialogue.rs` | 暂不更新 |
| `luau/tests/config.yaml.example` | 不再需要 |
| `rust/speech/` | Rust speech 包，暂不更新 |

---

## 五、暂停更新

| 目录 | 说明 |
|-----|------|
| `rust/luau/runtime/` | Rust Luau runtime，待 Go 版本稳定后再同步 |
| `rust/cmd/luau/` | Rust Luau CLI，同上 |

---

## 六、测试验证

所有测试通过：
- `//go/pkg/luau:luau_test`
- `//go/pkg/luau/registry:registry_test`
- `//go/pkg/luau/runtime:runtime_test`

压力测试通过（28万+迭代无崩溃）。
