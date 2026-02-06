# PR #59 Self-Review Issues

## go/pkg/luau/runtime/

| # | 优先级 | 文件:行 | 问题 | 说明 |
|---|--------|---------|------|------|
| 1 | P0 | stream.go:466 | stream 方法重复注册 | `pushStreamObject`/`pushBiStreamObject` 每次创建 stream 都 RegisterFunc + SetGlobal，应像 Promise 一样在 RegisterBuiltins 中预注册一次 |
| 2 | P1 | runtime.go:626,639 | fmt.Printf 在库代码中 | PrecompileModules 直接用 fmt.Printf 输出日志，库代码不应直接打印，应使用可配置 logger 或删除 |
| 3 | P1 | runtime.go:826-828 | thread error 只打印不传递 | processCompletedOp 中 thread 出错仅 fmt.Printf，调用方无感知 |
| 4 | P1 | builtin_timeout.go:84 | timeouts 延迟初始化 | timeouts registry 没有在 New() 中初始化（其他 registry 都有），应统一 |
| 5 | P2 | builtin_http.go:160-163 | ReadAll 错误被忽略 | resp body 读取失败时 comment 说 continue with partial body 但未告知调用方 |
| 6 | P2 | agent.go:176-178 | recover 处理 double close | CloseInput 用 recover 防 panic 不优雅，可用 inputClosed flag 替代 |

## go/pkg/genx/

| # | 优先级 | 文件:行 | 问题 | 说明 |
|---|--------|---------|------|------|
| 7 | P1 | cortex/stream.go:69-71 | inputStream.Write 静默丢帧 | channel full 时 drop frame 但不返回错误也无日志，调用方无法感知丢帧 |
| 8 | P1 | modelloader/config.go:41-43 | verboseTransport 使用 fmt.Printf | 调试日志用 fmt.Printf 直接输出，应使用 logger |
| 9 | P2 | stream_utils.go:31-32 | Split 硬编码 buffer 大小 100 | buffer.N 参数硬编码 100，大流量场景可能不够，建议可配置 |
| 10 | P2 | transformers/mux.go:17 | DefaultMux 全局变量 | 全局可变状态，并发注册可能有问题。不过 trie 本身可能线程安全，需确认 |
| 11 | P2 | cortex/atom.go 整体 | atom.mu 持锁范围大 | handleState 中持 mutex 调用 ensureSession 等可能阻塞的操作，可能影响 handleAudio 吞吐 |
| 12 | P0 | stream_id.go:27 | rand.Read 错误处理 | `rand.Read(randomBytes)` 忽略错误返回值。虽然 crypto/rand 极少失败，但 Go 惯例应检查 |

## go/pkg/chatgear/

| # | 优先级 | 文件:行 | 问题 | 说明 |
|---|--------|---------|------|------|
| 13 | P2 | listener.go:324-328 | accept channel 满时丢弃设备 | getOrCreatePort 中 acceptCh 满时仅打日志，新设备被静默丢弃。考虑到 cap(acceptCh)=32 且为异常场景，可接受但应关注 |
| 14 | P2 | port_client.go:8 | import "unsafe" | ClientPort 导入 unsafe 包，需确认是否必要（可能用于 int16/byte 转换） |
| 15 | OK | conn_pipe.go 整体 | ErrPipeBufferFull 改造 | 之前 review 已修复，Send* 方法 channel full 时返回 ErrPipeBufferFull 而非静默丢弃。设计合理 |
| 16 | OK | listener.go 整体 | net.Listener 模式 | Accept/Close 遵循 net.Listener 惯例，timeout checker 清理不活跃连接，设计良好 |
| 17 | OK | port_server.go 整体 | 三层 track 设计 | background/foreground/overlay 三层音频轨道 + mixer 设计清晰，自动 streaming 状态管理正确 |

## go/cmd/

| # | 优先级 | 文件:行 | 问题 | 说明 |
|---|--------|---------|------|------|
| 18 | OK | luau/main.go | CLI 结构清晰 | 三种 runtime mode (minimal/tool/agent)，flag 设计合理，已删除冗余 builtin_*.go 和 runtime.go |
| 19 | OK | cortextest/ | 完整 CLI 工具 | transformer 注册机制，支持 dashscope/doubao，signal handling 正确 |
| 20 | OK | geartest/ | 设备模拟器 | 支持多 context 配置，WebRTC 集成，TUI 交互 |

## go/pkg/luau/registry/

| # | 优先级 | 文件:行 | 问题 | 说明 |
|---|--------|---------|------|------|
| 21 | P2 | upstream.go:187 | DownloadPackage 无大小限制 | io.ReadAll 无 size limit，恶意上游可导致 OOM。考虑使用 io.LimitReader |
| 22 | P2 | require.go:39 | require 非线程安全 | requireStateSimple 在同一 luau.State 上操作 Lua API，但 Luau State 不支持并发。当前不是问题（单线程运行时），但 loadingMu/mu 暗示可能多线程使用 |
| 23 | OK | version.go | semver 实现完整 | ^/~/>=/<= 等约束全覆盖，FindBestMatch 正确取最高匹配版本 |
| 24 | OK | package.go | tarball 打包解包 | 支持 pkg.json manifest, checksum 校验, 路径规范化 |
| 25 | OK | memory.go | MemoryRegistry | upstream 级联查找，Store/Resolve 流程正确 |

## luau/ CGO (luau_wrapper.cpp, luau_wrapper.h, go/pkg/luau/luau.go)

| # | 优先级 | 文件:行 | 问题 | 说明 |
|---|--------|---------|------|------|
| 26 | OK | luau_wrapper.cpp:652-686 | luau_newthread 线程锚定 | lua_ref + parentRef/parentState 正确解决了 SIGBUS 问题，luau_close_thread 用 lua_unref 释放引用 |
| 27 | OK | luau_wrapper.cpp:682-683 | thread registry 覆盖 | 每个 thread 在自己的 registry 中存储 _luau_state 指向自己的 wrapper，确保外部回调获得正确 stack |
| 28 | P2 | luau_wrapper.cpp:583-586 | callback_id 截断风险 | pushinteger 用 `static_cast<int>` 转 uint32_t，若 Luau 的 lua_Integer 为 32-bit 有符号则高位丢失。实际不太可能因为 callback_id 通常远小于 2^31 |
| 29 | OK | luau.go:792-813 | Thread.Close 流程 | 先清理 Go 端 funcIDs，再调 C.luau_close_thread 释放 registry ref，最后 nil 化指针，流程正确 |

## go/pkg/ SDK 包 (audio, buffer, dashscope, doubaospeech, minimax, speech)

| # | 优先级 | 文件 | 问题 | 说明 |
|---|--------|------|------|------|
| 30 | OK | speech/ 全删除 | 旧 TTS/ASR 抽象层移除 | 功能已迁移到 genx/transformers，删除正确 |
| 31 | OK | audio/opusrt/ 全删除 | 旧 Opus 实时处理移除 | 功能已迁移到 genx/input/opus/，删除正确 |
| 32 | OK | audio/ 新增 codec/opus, pcm, resampler | 音频基础设施升级 | 独立模块化设计正确 |
| 33 | OK | buffer/ 增量改动 | Buffer 接口微调 | 常规改进 |

## examples/go/

| # | 优先级 | 文件 | 问题 | 说明 |
|---|--------|------|------|------|
| 34 | OK | speech/ 全删除 | 旧 speech 示例移除 | 正确 |
| 35 | OK | genx/transformers/ 新增 | 6 个 transformer 示例 | 覆盖 doubao ASR/TTS/realtime 和 dashscope realtime，结构合理 |
| 36 | OK | chatgear/ resampler 修复 | 使用 go/pkg/audio/resampler | 之前 review 已修复，不再用简单 decimation |
| 37 | OK | genx/luau_dialogue/ 已删除 | 误导性示例已清除 | 之前 review 确认删除 |

## testdata/ 和 luau/tests/

| # | 优先级 | 文件 | 问题 | 说明 |
|---|--------|------|------|------|
| 38 | OK | testdata/luau/runtime/ | 异步运行时测试数据 | 完整覆盖 async_http, async_timeout, async_combinators, generate, tts 等 |
| 39 | OK | testdata/models/ | model config 迁移 | 从 testdata/matchtest/models/ 迁移到 testdata/models/，增加了 tts/asr/realtime configs |
| 40 | OK | luau/tests/tool/ | 工具运行时测试 | test_basic, test_require, test_stream 覆盖 rt:* API |
| 41 | OK | luau/tests/agent/ | Agent 运行时测试 | test_basic 覆盖 recv/emit 模式 |

## rust/

| # | 优先级 | 文件 | 问题 | 说明 |
|---|--------|------|------|------|
| 42 | OK | rust/speech/ 全删除 | 旧 speech crate 移除 | Cargo.toml/Cargo.lock 已同步清理 |
| 43 | OK | rust/luau/runtime/ 新增 | Rust Luau 运行时库 | 从 rust/cmd/luau/ 提取为独立 crate，支持 async HTTP + event loop |
| 44 | OK | rust/cmd/luau/ 重构 | CLI 入口简化 | 依赖 rust/luau/runtime/ 而非内联实现 |

## Root & CI

| # | 优先级 | 文件 | 问题 | 说明 |
|---|--------|------|------|------|
| 45 | OK | .bazelrc | 缓存配置简化 | 移除 repository_cache 引用（随 embed 删除），保留 disk_cache |
| 46 | OK | .github/workflows/build-main.yaml | CI 脚本修复 | 修复 bazel-affected exit code 捕获逻辑 |
| 47 | OK | embed/ 全删除 | Zig 嵌入式代码移除 | 已确认移除，.bazelrc/.bazelignore 已同步 |
| 48 | OK | MODULE.bazel | 依赖更新 | 反映 speech/embed 移除和新 crate 添加 |
