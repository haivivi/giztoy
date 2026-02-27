# 测试文档：Rust GenX Realtime Transformers 对齐（Doubao/DashScope）

## 测试策略

### 三层测试架构

| 测试类型 | 位置 | 运行方式 | 用途 |
|---------|------|---------|------|
| **单元测试** | `rust/genx/src/` 内联 | `cargo test --lib` | Mock 测试，验证事件映射和错误处理 |
| **集成测试** | `rust/genx/tests/` | `cargo test --test <name>` | 真实 API 调用，契约验证 |
| **E2E 二进制** | `e2e/genx/transformers/*/` | `bazel run //e2e/...` | 完整场景测试，对齐 Go |

### 测试框架
- **Rust**: 内置 `cargo test` + `tokio::test` 用于异步测试
- **Mock**: 自定义 WebSocket mock server（使用 `tokio-tungstenite` 或模拟 trait）
- **E2E**: Bazel 构建的 Rust 二进制程序，通过 `bazel run` 运行

### 测试目录结构

```
# 1. 单元测试（内联在实现文件中）
rust/genx/src/transformers/
├── doubao_realtime.rs          # #[cfg(test)] 模块
├── dashscope_realtime.rs       # #[cfg(test)] 模块
└── mod.rs

# 2. 集成测试（Cargo integration tests，真实 API）
rust/genx/tests/
├── e2e.rs                      # 已有：generator/segmentor/profiler 测试
└── realtime_contract.rs        # 新增：realtime transformer 契约测试

# 3. E2E 二进制（Bazel 构建，完整场景，对齐 Go）
e2e/genx/transformers/
├── doubao_realtime_basic/
│   ├── main.go                 # Go 版本
│   ├── main.rs                 # Rust 版本
│   └── BUILD.bazel             # go_binary + rust_binary
├── doubao_realtime_chat/
│   ├── main.go
│   ├── main.rs
│   └── BUILD.bazel
├── doubao_realtime_asr/
│   ├── main.go
│   ├── main.rs
│   └── BUILD.bazel
├── doubao_realtime_vad/
│   ├── main.go
│   ├── main.rs
│   └── BUILD.bazel
├── doubao_realtime_voice/
│   ├── main.go
│   ├── main.rs
│   └── BUILD.bazel
├── dashscope_realtime/
│   ├── main.go
│   ├── main.rs
│   └── BUILD.bazel
├── dashscope_realtime_chat/
│   ├── main.go
│   ├── main.rs
│   └── BUILD.bazel
└── internal/
    ├── lib.rs                  # 共享库
    ├── audio_track.rs
    ├── eos_to_silence.rs
    └── BUILD.bazel
```

### 运行命令

```bash
# ========== 1. 单元测试（Mock，无需 API Key）==========
bazel test //rust/genx:genx_test
cargo test -p giztoy-genx --lib

# ========== 2. 集成测试（真实 API，#[ignore]）==========
# Generator/Segmentor/Profiler 测试
DASHSCOPE_API_KEY=xxx cargo test -p giztoy-genx --test e2e -- --ignored

# Realtime Transformer 契约测试
DASHSCOPE_API_KEY=xxx cargo test -p giztoy-genx --test realtime_contract -- --ignored
DOUBAO_APP_ID=xxx DOUBAO_TOKEN=xxx cargo test -p giztoy-genx --test realtime_contract test_doubao_realtime_contract -- --ignored

# ========== 3. E2E 二进制（完整场景）==========
# Go 版本
bazel run //e2e/genx/transformers:doubao_realtime_basic

# Rust 版本
bazel run //e2e/genx/transformers:doubao_realtime_basic_rust
bazel run //e2e/genx/transformers:dashscope_realtime_rust -- --mode=basic
```
# 单元测试（内联在实现文件中）
rust/genx/src/transformers/
├── doubao_realtime.rs          # 实现 + 内联单元测试
├── dashscope_realtime.rs       # 实现 + 内联单元测试
└── mod.rs

# E2E 测试（Bazel 构建，与 Go E2E 目录一一对应）
e2e/genx/transformers/
├── doubao_realtime_basic/          # Bazel target: //e2e/genx/transformers:doubao_realtime_basic
│   ├── main.rs
│   └── BUILD.bazel
├── doubao_realtime_chat/           # Bazel target: //e2e/genx/transformers:doubao_realtime_chat
│   ├── main.rs
│   └── BUILD.bazel
├── doubao_realtime_asr/            # Bazel target: //e2e/genx/transformers:doubao_realtime_asr
│   ├── main.rs
│   └── BUILD.bazel
├── doubao_realtime_vad/            # Bazel target: //e2e/genx/transformers:doubao_realtime_vad
│   ├── main.rs
│   └── BUILD.bazel
├── doubao_realtime_voice/          # Bazel target: //e2e/genx/transformers:doubao_realtime_voice
│   ├── main.rs
│   └── BUILD.bazel
├── dashscope_realtime/             # Bazel target: //e2e/genx/transformers:dashscope_realtime
│   ├── main.rs
│   └── BUILD.bazel
├── dashscope_realtime_chat/        # Bazel target: //e2e/genx/transformers:dashscope_realtime_chat
│   ├── main.rs
│   └── BUILD.bazel
└── internal/                       # E2E 共享库（Bazel rust_library）
    ├── lib.rs
    ├── audio_track.rs
    ├── eos_to_silence.rs
    └── BUILD.bazel
```

### BUILD.bazel 示例
```python
# e2e/genx/transformers/doubao_realtime_basic/BUILD.bazel
load("@rules_rust//rust:defs.bzl", "rust_binary")

rust_binary(
    name = "doubao_realtime_basic",
    srcs = ["main.rs"],
    deps = [
        "//rust/genx:giztoy_genx",
        "//e2e/genx/transformers/internal:genx_e2e_internal",
        "@crates.io//:tokio",
        "@crates.io//:clap",
    ],
)
```

### 运行命令
```bash
# ========== 单元测试 ==========
# 运行所有单元测试（Mock，无需 API Key）
bazel test //rust/genx:genx_test
cargo test -p giztoy-genx --lib

# ========== E2E 测试 ==========
# Doubao Realtime 基本测试
bazel run //e2e/genx/transformers:doubao_realtime_basic

# DashScope Realtime 综合测试（支持多种模式）
bazel run //e2e/genx/transformers:dashscope_realtime -- --mode=basic
bazel run //e2e/genx/transformers:dashscope_realtime -- --mode=asr
bazel run //e2e/genx/transformers:dashscope_realtime -- --mode=vad

# Doubao Realtime VAD 测试
bazel run //e2e/genx/transformers:doubao_realtime_vad -- --vad-window=200
```

---

## 测试场景

### 场景 1：正常路径 - 全链路收发
**类型**: 单元测试  
**优先级**: P0  
**状态**: 📝 待实现

**测试内容**:
模拟完整的对话流程：
1. 客户端发送音频 chunk (带 BOS/EOS)
2. 服务端返回 ASR 文本（用户输入）
3. 服务端返回模型文本
4. 服务端返回音频数据（带 BOS/EOS）
5. 会话正常结束

**输入**:
```rust
// 模拟输入流
[
    MessageChunk::new_begin_of_stream("stream-1"),
    MessageChunk::blob(Role::User, "audio/pcm", vec![/* 音频数据 */]),
    MessageChunk::new_end_of_stream("audio/pcm"),
]
```

**预期输出顺序**:
1. `Role::User` + 转写文本（ASR 结果）
2. `Role::Model` + BOS 标记（TTS 开始）
3. `Role::Model` + 模型文本
4. `Role::Model` + 音频 blob
5. `Role::Model` + EOS 标记（TTS 结束）
6. `Ok(None)` - 流结束

**通过标准**:
- 输出事件顺序与预期完全一致
- 所有 chunk 的 `stream_id` 正确关联
- BOS/EOS 标记正确传递

**对应测试文件**: `rust/genx/src/transformers/doubao_realtime.rs::tests::test_full_conversation_flow`

---

### 场景 2：正常路径 - DashScope Realtime
**类型**: 单元测试  
**优先级**: P0  
**状态**: 📝 待实现

**测试内容**:
验证 DashScope 特定的事件映射：
- `input_audio_transcription.completed` → 用户文本 + EOS
- `response.created` → BOS
- `response.text.delta` → 模型文本
- `response.audio.delta` → 音频数据
- `response.audio.done` → EOS

**输入**: 同上

**预期输出**: 与 Doubao 场景语义一致（统一事件层）

**对应测试文件**: `rust/genx/src/transformers/dashscope_realtime.rs::tests::test_full_conversation_flow`

---

### 场景 3：边界条件 - 空输入流
**类型**: 单元测试  
**优先级**: P0  
**状态**: 📝 待实现

**测试内容**:
输入流直接返回 `Ok(None)`（无数据）。

**输入**:
```rust
// 空输入流
[] // 立即返回 Ok(None)
```

**预期输出**:
- `next()` 返回 `Ok(None)`
- 无 panic
- 后台任务优雅退出

**通过标准**:
- 测试通过且不 panic
- 资源正确释放

**对应测试文件**: `rust/genx/src/transformers/doubao_realtime.rs::tests::test_empty_input`

---

### 场景 4：边界条件 - 仅文本输入（无音频）
**类型**: 单元测试  
**优先级**: P1  
**状态**: 📝 待实现

**测试内容**:
输入流发送文本而非音频。

**输入**:
```rust
[
    MessageChunk::text(Role::User, "你好，请介绍一下自己"),
]
```

**预期输出**:
- 文本被正确发送到服务端
- 收到模型响应

**对应测试文件**: `rust/genx/src/transformers/doubao_realtime.rs::tests::test_text_input_only`

---

### 场景 5：错误处理 - 初始化失败
**类型**: 单元测试  
**优先级**: P0  
**状态**: 📝 待实现

**测试内容**:
模拟 WebSocket 连接失败或服务端拒绝握手。

**输入**:
```rust
// Mock server 返回 403 Forbidden
```

**预期输出**:
- `transform()` 返回 `Err(GenxError::Other(...))`
- 错误信息包含 "connection refused" 或 "handshake failed"
- 不启动后台任务

**通过标准**:
- 返回错误而非 panic
- 错误类型正确

**对应测试文件**: `rust/genx/src/transformers/doubao_realtime.rs::tests::test_init_connection_failed`

---

### 场景 6：错误处理 - 运行时断连
**类型**: 单元测试  
**优先级**: P0  
**状态**: 📝 待实现

**测试内容**:
会话进行中 WebSocket 连接中断。

**输入**:
```rust
// Mock server 在发送部分响应后断开连接
```

**预期输出**:
- 输出流通过 `next()` 返回错误
- 或发送 Error 事件后结束
- 错误可观测（不是 panic）

**通过标准**:
- 错误被正确传播到输出流
- 资源正确释放

**对应测试文件**: `rust/genx/src/transformers/doubao_realtime.rs::tests::test_runtime_disconnect`

---

### 场景 7：错误处理 - 服务端错误事件
**类型**: 单元测试  
**优先级**: P1  
**状态**: 📝 待实现

**测试内容**:
服务端发送错误事件（如 Doubao 的 `EventSessionFailed`）。

**输入**:
```rust
// Mock server 发送错误事件
{
    "type": EventSessionFailed,
    "error": { "code": 55000000, "message": "resource ID mismatch" }
}
```

**预期输出**:
- 错误映射到 `GenxError::Generation` 或 `GenxError::Other`
- 流终止

**对应测试文件**: `rust/genx/src/transformers/doubao_realtime.rs::tests::test_server_error_event`

---

### 场景 8：并发场景 - 多流并发
**类型**: 单元测试  
**优先级**: P1  
**状态**: 📝 待实现

**测试内容**:
多个 transformer 实例并发运行。

**输入**:
```rust
// 3 个独立的 transformer 实例，各自处理独立输入流
```

**预期输出**:
- 各流输出互不影响
- 无数据混淆

**对应测试文件**: `rust/genx/src/transformers/doubao_realtime.rs::tests::test_concurrent_streams`

---

### 场景 9：生命周期 - 上游关闭语义
**类型**: 单元测试  
**优先级**: P0  
**状态**: 📝 待实现

**测试内容**:
验证上游流关闭后，transformer 正确处理：
1. 发送 trailing silence（Doubao）
2. 等待服务端响应
3. 优雅关闭 WebSocket
4. 输出流结束

**输入**:
```rust
// 输入流正常结束（input.next() 返回 Ok(None)）
```

**预期输出**:
- 所有服务端响应被消费
- 输出流正确结束
- WebSocket 正确关闭

**对应测试文件**: `rust/genx/src/transformers/doubao_realtime.rs::tests::test_upstream_close_graceful`

---

### 场景 10：生命周期 - Drop 输出流
**类型**: 单元测试  
**优先级**: P1  
**状态**: 📝 待实现

**测试内容**:
消费端提前 drop 输出流。

**输入**:
```rust
// 消费端只读取一个 chunk 后就 drop 输出流
```

**预期输出**:
- 后台任务检测到发送失败，优雅退出
- 无 panic
- WebSocket 关闭

**对应测试文件**: `rust/genx/src/transformers/doubao_realtime.rs::tests::test_early_drop_output`

---

---

# E2E 测试（对齐 Go 实现）

参考 Go E2E 测试：`e2e/genx/transformers/`

## E2E 1: Doubao Realtime Basic
**对应 Go**: `e2e/genx/transformers/doubao_realtime_basic/main.go`  
**类型**: E2E  
**优先级**: P0  
**状态**: 📝 待实现

**测试内容**:
基本文本输入 → Doubao Realtime → 音频输出管道验证。

**环境变量要求**:
```bash
export DOUBAO_APP_ID=xxx
export DOUBAO_TOKEN=xxx
```

**命令行参数**:
```bash
--speaker    # TTS speaker voice (默认: zh_female_vv_jupiter_bigtts)
--timeout    # 测试超时 (默认: 2m)
```

**测试流程**:
1. 创建 Doubao client
2. 创建 DoubaoRealtime transformer（pcm_s16le @ 24kHz）
3. 初始化 portaudio 输出流
4. 循环处理测试句子：
   - 发送文本输入
   - 接收 ASR 结果（Role::User + Text）
   - 接收 LLM 文本（Role::Model + Text）
   - 接收音频数据（Role::Model + Blob），实时播放
   - 统计音频时长
5. 输出测试摘要

**测试句子**:
```rust
[
    "你好，请用一句话介绍自己。",
    "今天天气怎么样？",
    "给我讲一个笑话。",
]
```

**通过标准**:
- 所有句子处理成功
- 每个回合都有 LLM 文本响应
- 每个回合都有音频输出（>0 字节）
- 音频播放正常（无爆音/卡顿）

**对应文件**: `e2e/genx/transformers/doubao_realtime_basic/src/main.rs`

---

## E2E 2: Doubao Realtime Chat
**对应 Go**: `e2e/genx/transformers/doubao_realtime_chat/main.go`  
**类型**: E2E  
**优先级**: P0  
**状态**: 📝 待实现

**测试内容**:
两个 AI agent 进行文本对话，验证多轮交互。

**环境变量要求**:
```bash
export DOUBAO_APP_ID=xxx
export DOUBAO_TOKEN=xxx
```

**命令行参数**:
```bash
--rounds     # 对话轮数 (默认: 5)
--timeout    # 测试超时 (默认: 3m)
```

**测试流程**:
1. 创建两个 DoubaoRealtime transformer：
   - AI A: 小红，东北大妈人设
   - AI B: 小丽，上海阿姨人设
2. 初始化 portaudio 输出流
3. 开始对话循环（指定轮数）：
   - AI A 发送消息 → AI B 接收并回复
   - AI B 的回复 → AI A 接收并回复
   - 每轮收集 LLM 文本和音频
   - 实时播放音频
4. 输出对话记录和摘要

**通过标准**:
- 完成指定轮数的对话
- 每个 AI 都能正常接收和回复
- 响应符合人设特征
- 音频输出正常

**对应文件**: `e2e/genx/transformers/doubao_realtime_chat/src/main.rs`

---

## E2E 3: Doubao Realtime ASR
**对应 Go**: `e2e/genx/transformers/doubao_realtime_asr/main.go`  
**类型**: E2E  
**优先级**: P1  
**状态**: 📝 待实现

**测试内容**:
验证 ASR 转写功能（文本输入模式）。

**环境变量要求**: 同 E2E 1

**测试流程**:
1. 发送多个测试句子
2. 收集 ASR 结果（Role::User + Text）
3. 对比输入和 ASR 转写结果
4. 验证转写准确性

**测试句子**:
```rust
[
    "你好，我是小明。",
    "今天天气怎么样？",
    "请给我讲一个笑话。",
    "北京是中国的首都。",
    "我喜欢吃苹果和香蕉。",
]
```

**通过标准**:
- ASR 转写结果与输入文本语义一致
- 转写延迟 < 500ms

**对应文件**: `e2e/genx/transformers/doubao_realtime_asr/src/main.rs`

---

## E2E 4: Doubao Realtime VAD
**对应 Go**: `e2e/genx/transformers/doubao_realtime_vad/main.go`  
**类型**: E2E  
**优先级**: P1  
**状态**: 📝 待实现

**测试内容**:
验证 VAD (Voice Activity Detection) 配置效果。

**环境变量要求**:
```bash
export DOUBAO_APP_ID=xxx
export DOUBAO_TOKEN=xxx
export MINIMAX_API_KEY=xxx  # 用于 TTS 生成输入音频
```

**命令行参数**:
```bash
--vad-window    # VAD 检测窗口 (ms)，默认 200
                # 100-200ms: 快速响应，可能截断
                # 500-1000ms: 更容忍停顿，响应慢
```

**测试流程**:
1. 使用 MiniMax TTS 生成音频流
2. 在句子间插入 2 秒静音（模拟停顿）
3. 使用不同 vad-window 配置测试
4. 测量每轮对话的完成时间
5. 对比不同配置的效果

**通过标准**:
- 小窗口 (200ms): 快速检测结束，但可能过早截断
- 大窗口 (1000ms): 更容忍停顿，响应较慢
- 每轮对话正确分离

**对应文件**: `e2e/genx/transformers/doubao_realtime_vad/src/main.rs`

---

## E2E 5: Doubao Realtime Voice
**对应 Go**: `e2e/genx/transformers/doubao_realtime_voice/main.go`  
**类型**: E2E  
**优先级**: P1  
**状态**: 📝 待实现

**测试内容**:
验证不同音色的语音合成效果。

**环境变量要求**: 同 E2E 1

**命令行参数**:
```bash
--speaker    # 测试音色列表，逗号分隔
```

**测试流程**:
1. 使用不同 speaker 创建 transformer
2. 发送相同测试文本
3. 收集并保存音频文件
4. 播放对比（人工验证）

**对应文件**: `e2e/genx/transformers/doubao_realtime_voice/src/main.rs`

---

## E2E 6: DashScope Realtime（综合测试）
**对应 Go**: `e2e/genx/transformers/dashscope_realtime/main.go`  
**类型**: E2E  
**优先级**: P0  
**状态**: 📝 待实现

**测试内容**:
DashScope Realtime 综合功能测试，支持多种模式。

**环境变量要求**:
```bash
export DASHSCOPE_API_KEY=xxx  # 或 QWEN_API_KEY
export MINIMAX_API_KEY=xxx    # 用于 TTS 输入
```

**命令行参数**:
```bash
--mode       # 测试模式: basic | asr | vad | voice (默认: basic)
--voice      # 音色: Chelsie | Cherry | Serena | Ethan (默认: Chelsie)
--model      # 模型: turbo | flash (默认: turbo)
--output     # 音频输出目录 (默认: /tmp/dashscope_test)
--verify     # 使用 ASR 验证音频 (需要 DOUBAO_API_KEY)
```

### Mode: basic
**测试内容**: 基本流式管道验证  
**流程**:
1. MiniMax TTS 生成音频（16kHz PCM）
2. CompositeSeq 组合多个句子流
3. EOSToSilence 插入静音
4. DashScope Realtime 处理
5. AudioTrack 收集音频到 MP3 文件

**通过标准**:
- TTS 生成成功
- 管道正常连接
- 输出音频文件生成
- 音频时长合理（>0s）

### Mode: asr
**测试内容**: ASR 转写验证  
**配置**:
```rust
DashScopeRealtime::new(client)
    .with_enable_asr(true)
    .with_asr_model("qwen-audio-turbo")
```

**通过标准**:
- 收到 InputAudioTranscriptionCompleted 事件
- ASR 文本与输入语义一致

### Mode: vad
**测试内容**: 服务端 VAD 验证  
**配置**:
```rust
DashScopeRealtime::new(client)
    .with_enable_asr(true)
    .with_turn_detection(TurnDetection {
        type_: "server_vad",
        silence_duration_ms: 800,
        threshold: 0.5,
        prefix_padding_ms: 300,
    })
```

**通过标准**:
- VAD 正确检测语音结束
- 多轮对话正确分离

### Mode: voice
**测试内容**: 动态音色切换  
**流程**:
1. 第一回合使用 Chelsie 音色
2. 第一回合结束后，动态切换到 Cherry 音色
3. 同时修改 system instructions（切换人设）
4. 验证音色和人设切换效果

**通过标准**:
- UpdateSession 调用成功
- 第二回合音色变化可感知

**对应文件**: `e2e/genx/transformers/dashscope_realtime/src/main.rs`

---

## E2E 7: DashScope Realtime Chat
**对应 Go**: `e2e/genx/transformers/dashscope_realtime_chat/main.go`  
**类型**: E2E  
**优先级**: P1  
**状态**: 📝 待实现

**测试内容**:
DashScope 双 AI 并发对话测试。两个 AI 通过音频流实时交换消息。

**架构**:
```
TTS -> bufA -> AI_A(东北大妈) -> Tee(Track) -> filter(audio) -> bufB 
                                                        ↓
TTS -> bufA <- AI_B(上海小姐姐) <- Tee(Track) <- filter(audio) <- bufB
```

**环境变量要求**:
```bash
export DASHSCOPE_API_KEY=xxx  # 或 QWEN_API_KEY
export MINIMAX_API_KEY=xxx    # 用于初始 TTS
```

**命令行参数**:
```bash
--rounds     # 对话轮数 (默认: 4)
--timeout    # 测试超时 (默认: 10m)
```

**AI 人设**:
- **AI A (东北大妈 - Cherry 音色)**: 王大姐，说话带东北口音（"哎呀妈呀"、"老妹儿"、"整挺好"）
- **AI B (上海小姐姐 - Chelsie 音色)**: 小云，说话带上海腔调（"阿拉"、"侬"、"老好的"）

**测试流程**:
1. 使用 MiniMax TTS 生成初始消息（"你好呀，我是小云..."）
2. 创建两个 DashScope Realtime transformer（手动模式，无 VAD）
3. 创建 BufferStream 作为两个 AI 的输入缓冲
4. 启动三个并发任务：
   - 任务1: 发送初始 TTS 音频到 AI_A
   - 任务2: AI_A 输出 → 重采样(24kHz→16kHz) → 发送到 AI_B
   - 任务3: AI_B 输出 → 重采样(24kHz→16kHz) → 发送到 AI_A
5. 每轮对话检测音频 EOS 标记，统计轮数
6. 实时播放音频（portaudio）
7. 输出对话统计

**关键技术点**:
- 音频重采样：DashScope 输出 24kHz，输入需要 16kHz
- StreamID 管理：每轮对话生成新的 StreamID
- BOS/EOS 标记：正确传递流边界
- 并发控制：使用 WaitGroup 等待所有任务完成

**通过标准**:
- 完成指定轮数的对话
- AI_A 和 AI_B 都能正常接收和回复
- 响应符合人设特征（口音、用词）
- 音频播放正常，无重采样错误

**对应文件**: `e2e/genx/transformers/dashscope_realtime_chat/src/main.rs`

---

## E2E 测试共享组件

### AudioTrack
**对应 Go**: `e2e/genx/transformers/internal/audio_track.go`  
**功能**: 收集音频数据并保存到文件  
**Rust 实现**: `e2e/genx/transformers/internal/src/audio_track.rs`

### EOSToSilence
**对应 Go**: `e2e/genx/transformers/internal/eos_to_silence.go`  
**功能**: 在 EOS 后插入指定时长的静音  
**用途**: 模拟语音间的停顿，测试 VAD  
**Rust 实现**: `e2e/genx/transformers/internal/src/eos_to_silence.rs`

### BufferStream
**对应 Go**: `e2e/genx/transformers/dashscope_realtime_chat/main.go` (bufferStream)  
**功能**: 缓冲流，用于 AI 间对话的管道  
**Rust 实现**: `e2e/genx/transformers/internal/src/buffer_stream.rs`

---

## 场景 11：集成测试 - Realtime 最小契约
**类型**: 集成测试  
**优先级**: P0  
**状态**: 📝 待实现  
**标记**: `#[ignore]`

**测试内容**:
对齐 Go 实现的最小行为契约（需要真实 API Key）：
- Start → Input → Output → End 流程
- 事件序列与 Go 实现一致

**输入**:
真实音频数据（或预录制音频文件）

**预期输出**:
- 输出事件序列与 Go 契约一致
- 可以复现相同的行为

**验证方式**:
与 Go e2e 测试输出对比

**对应测试文件**: `rust/genx/tests/realtime_contract.rs::test_doubao_realtime_contract`

---

## 边界条件测试

### 边界 1：超长音频输入
**测试内容**: 持续发送大量音频数据（10MB+）  
**预期**: 流式处理，不占用过多内存

### 边界 2：极短音频输入
**测试内容**: 发送 1ms 音频数据  
**预期**: 正确处理，可能返回空转写

### 边界 3：快速 BOS/EOS
**测试内容**: 快速连续发送 BOS + EOS  
**预期**: 正确处理，无 panic

### 边界 4：无 StreamID
**测试内容**: chunk 没有设置 stream_id  
**预期**: 自动生成或使用默认值

---

## 异常场景测试

### 异常 1：非法音频格式
**触发条件**: 发送非 PCM 数据但声明为 PCM  
**预期行为**: 服务端返回错误，错误被传递

### 异常 2：认证失败
**触发条件**: 使用错误的 API Key  
**预期行为**: 初始化阶段返回错误

### 异常 3：网络超时
**触发条件**: 模拟网络延迟 > 30s  
**预期行为**: 返回超时错误

### 异常 4：服务端限流
**触发条件**: 快速发送大量请求  
**预期行为**: 接收限流错误，可重试

---

## 测试数据

### Mock WebSocket 事件序列

#### Doubao Realtime Mock 序列
```rust
// 模拟服务端事件序列
vec![
    // ConnectionStarted
    MockEvent::ConnectionStarted { connect_id: "conn-1" },
    // SessionStarted
    MockEvent::SessionStarted { session_id: "sess-1" },
    // ASR Response
    MockEvent::ASRResponse { text: "你好" },
    // ASR Ended
    MockEvent::ASREnded,
    // TTS Started (BOS)
    MockEvent::TTSStarted { content: "你好，有什么可以帮助你？" },
    // Chat Response
    MockEvent::ChatResponse { text: "你好，有什么可以帮助你？" },
    // Audio Data (multiple chunks)
    MockEvent::AudioReceived { data: vec![...] },
    MockEvent::AudioReceived { data: vec![...] },
    // TTS Finished (EOS)
    MockEvent::TTSFinished,
    // Session Ended
    MockEvent::SessionEnded,
]
```

#### DashScope Realtime Mock 序列
```rust
vec![
    MockEvent::SessionCreated { session_id: "sess-1" },
    MockEvent::InputAudioTranscriptionCompleted { transcript: "你好" },
    MockEvent::ResponseCreated,
    MockEvent::ResponseTextDelta { delta: "你好" },
    MockEvent::ResponseTextDelta { delta: "，有什么" },
    MockEvent::ResponseTextDelta { delta: "可以帮助你？" },
    MockEvent::ResponseTextDone,
    MockEvent::ResponseAudioDelta { data: vec![...] },
    MockEvent::ResponseAudioDone,
]
```

### 音频测试数据
- **silence_500ms.pcm**: 500ms 静音（16kHz, 16-bit, mono）
- **test_utterance.pcm**: 短语音片段
- 数据文件位置: `testdata/audio/`

---

## 已知问题与风险

### 风险 1：Provider 事件模型差异
**描述**: Doubao 和 DashScope 的事件类型和顺序不同  
**应对**: 统一事件映射表，在测试中逐项断言

### 风险 2：断线重连策略
**描述**: 不同 provider 的重连行为可能不同  
**应对**: 明确定义可重试错误集合，在测试中验证

### 风险 3：StreamID 关联
**描述**: 输入和输出的 StreamID 需要正确关联  
**应对**: 测试用例显式验证 stream_id 一致性

---

## 实现检查清单

### Doubao Realtime Transformer
- [ ] `transformers/doubao_realtime.rs` 文件创建
- [ ] `DoubaoRealtime` struct 实现 `Transformer` trait
- [ ] WebSocket 连接和握手
- [ ] 事件映射（Doubao → GenX）
- [ ] BOS/EOS 标记处理
- [ ] StreamID 队列管理
- [ ] 错误传播
- [ ] 优雅关闭

### DashScope Realtime Transformer
- [ ] `transformers/dashscope_realtime.rs` 文件创建
- [ ] `DashScopeRealtime` struct 实现 `Transformer` trait
- [ ] WebSocket 连接和 session.created 等待
- [ ] 事件映射（DashScope → GenX）
- [ ] 音频 rate limiting（100ms chunks）
- [ ] 其他同上

### Mux 集成
- [ ] 在 `transformers/mod.rs` 中暴露
- [ ] 注册到 `TransformerMux`
- [ ] pattern 命名与 `testdata/cmd/apply/genx-realtime.yaml` 一致

### 集成测试（`rust/genx/tests/`）
- [x] `e2e.rs` - 已有测试
- [x] `realtime_contract.rs` - 已创建（需要完善）

### E2E 二进制（`e2e/genx/transformers/`）
- [ ] `internal/` - E2E 共享库
  - [ ] `lib.rs`
  - [ ] `audio_track.rs`
  - [ ] `eos_to_silence.rs`
  - [ ] `BUILD.bazel` (rust_library)
- [x] `doubao_realtime_basic/main.rs` - ✅ 已创建
- [ ] `doubao_realtime_basic/BUILD.bazel` - 添加 rust_binary
- [ ] `doubao_realtime_chat/main.rs`
- [ ] `doubao_realtime_chat/BUILD.bazel`
- [ ] `doubao_realtime_asr/main.rs`
- [ ] `doubao_realtime_asr/BUILD.bazel`
- [ ] `doubao_realtime_vad/main.rs`
- [ ] `doubao_realtime_vad/BUILD.bazel`
- [ ] `doubao_realtime_voice/main.rs`
- [ ] `doubao_realtime_voice/BUILD.bazel`
- [ ] `dashscope_realtime/main.rs`
- [ ] `dashscope_realtime/BUILD.bazel`
- [ ] `dashscope_realtime_chat/main.rs`
- [ ] `dashscope_realtime_chat/BUILD.bazel`
- [ ] E2E 测试 README 文档

---

## 测试运行记录

### 2025-02-27 - 测试文档创建
- **完成工作**: 
  - 制定详细测试策略和测试场景
  - 单元测试场景：10 个核心场景
  - E2E 测试规划：7 个 E2E 程序（对齐 Go 结构）
- **测试场景数**: 
  - 单元测试: 10 个核心场景 + 多个边界/异常场景
  - E2E 测试: 7 个独立测试程序
- **下一步**: Developer 实现 transformer，然后运行测试验证

### 测试层次对照表

| 测试层次 | Go | Rust | 运行命令 | 状态 |
|---------|----|------|---------|------|
| **集成测试** | N/A | `rust/genx/tests/realtime_contract.rs` | `cargo test --test realtime_contract -- --ignored` | ✅ 已创建 |
| **E2E Basic** | `e2e/genx/transformers/doubao_realtime_basic/main.go` | `e2e/genx/transformers/doubao_realtime_basic/main.rs` | `bazel run //e2e/genx/transformers:doubao_realtime_basic_rust` | ✅ 已创建 |
| **E2E Chat** | `e2e/genx/transformers/doubao_realtime_chat/main.go` | `e2e/genx/transformers/doubao_realtime_chat/main.rs` | `bazel run //e2e/genx/transformers:doubao_realtime_chat_rust` | 📝 待实现 |
| **E2E ASR** | `e2e/genx/transformers/doubao_realtime_asr/main.go` | `e2e/genx/transformers/doubao_realtime_asr/main.rs` | `bazel run //e2e/genx/transformers:doubao_realtime_asr_rust` | 📝 待实现 |
| **E2E VAD** | `e2e/genx/transformers/doubao_realtime_vad/main.go` | `e2e/genx/transformers/doubao_realtime_vad/main.rs` | `bazel run //e2e/genx/transformers:doubao_realtime_vad_rust` | 📝 待实现 |
| **E2E Voice** | `e2e/genx/transformers/doubao_realtime_voice/main.go` | `e2e/genx/transformers/doubao_realtime_voice/main.rs` | `bazel run //e2e/genx/transformers:doubao_realtime_voice_rust` | 📝 待实现 |
| **E2E DashScope** | `e2e/genx/transformers/dashscope_realtime/main.go` | `e2e/genx/transformers/dashscope_realtime/main.rs` | `bazel run //e2e/genx/transformers:dashscope_realtime_rust` | 📝 待实现 |
| **E2E DashScope Chat** | `e2e/genx/transformers/dashscope_realtime_chat/main.go` | `e2e/genx/transformers/dashscope_realtime_chat/main.rs` | `bazel run //e2e/genx/transformers:dashscope_realtime_chat_rust` | 📝 待实现 |

---

## 附录：Go 契约参考

### Doubao Realtime 事件映射
| Doubao 事件 | GenX 输出 | 说明 |
|------------|----------|------|
| EventASRResponse | Role::User + Text | 转写文本 |
| EventTTSStarted | Role::Model + BOS | 音频流开始 |
| EventChatResponse | Role::Model + Text | 模型文本 |
| EventAudioReceived | Role::Model + Blob | 音频数据 |
| EventTTSFinished | Role::Model + EOS | 音频流结束 |

### DashScope Realtime 事件映射
| DashScope 事件 | GenX 输出 | 说明 |
|---------------|----------|------|
| InputAudioTranscriptionCompleted | Role::User + Text + EOS | ASR 完成 |
| ResponseCreated | Role::Model + BOS | 响应开始 |
| ResponseTextDelta | Role::Model + Text | 文本片段 |
| ResponseTextDone | Role::Model + EOS | 文本结束 |
| ResponseAudioDelta | Role::Model + Blob | 音频片段 |
| ResponseAudioDone | Role::Model + EOS | 音频结束 |

### 统一语义
- **输入**: 音频 Blob（PCM 16kHz/24kHz）或 Text
- **输出**: 
  - 转写文本（Role::User）
  - 模型文本（Role::Model）
  - 音频 Blob（Role::Model）
  - BOS/EOS 标记（StreamCtrl）
