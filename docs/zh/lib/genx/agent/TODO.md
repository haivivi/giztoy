# GenX Agent - TODO

## 阶段一：Luau 引入 ✅

### 1.1 引入 Luau 源码
- [x] 通过 Bazel http_archive 引入 luau-lang/luau (v0.706)
- [x] 编写 Bazel 构建规则 `third_party/luau/BUILD.bazel`
- [x] 编写下载规则 `extensions.bzl` 中的 `luau` extension

### 1.2 C Wrapper
- [x] 创建 `luau/c/luau_wrapper.h` - C 接口定义
- [x] 创建 `luau/c/luau_wrapper.cpp` - C++ 实现
- [x] 实现基础 API：new/close/dostring/compile/loadbytecode/pcall
- [x] 实现栈操作：push/to 各类型（nil/bool/number/string/table）
- [x] 实现函数注册：pushcfunction/register
- [x] 实现表操作：newtable/getfield/setfield/gettable/settable/next
- [x] 实现全局变量：getglobal/setglobal
- [x] 实现内存管理：memoryusage/gc
- [x] 实现调试工具：dumpstack/version

### 1.3 Go Binding
- [x] 创建 `go/pkg/luau/` 包
- [x] CGO 绑定 C wrapper
- [x] 封装 Go 友好的 API（State, Type, OptLevel）
- [x] 实现 RegisterFunc（Go 函数注册为 Luau 全局函数）
- [x] 编写测试（60+ 测试用例，含功能/内存/并发/边界）
- [x] 编写 benchmark（执行/编译/栈操作/表操作/内存/RegisterFunc）

### 1.4 Rust Binding
- [x] 选择方案：使用 C wrapper 的 FFI 绑定（与 Go 保持一致）
- [x] 创建 `rust/luau/` crate
- [x] 封装统一 API（State, Type, OptLevel）与 Go 保持一致
- [x] 实现 register_func（Rust 函数注册为 Luau 全局函数）
- [x] 编写测试（40+ 测试用例，含功能/内存/并发/边界）
- [x] 编写 benchmark（使用 criterion，执行/编译/RegisterFunc）

---

## 阶段二：Luau 库系统（SDK 架构）

### 2.1 Luau Runner
- [ ] **Go Runner** `go/cmd/luau/`
  - [ ] 创建基础 Runner 结构
  - [ ] 实现命令行参数解析
  - [ ] 实现脚本加载和执行
  - [ ] 实现错误处理和输出
- [ ] **Rust Runner** `rust/cmd/luau/`
  - [ ] 创建基础 Runner 结构
  - [ ] 实现命令行参数解析
  - [ ] 实现脚本加载和执行
  - [ ] 实现错误处理和输出
- [ ] 编写 Bazel 构建规则

### 2.2 核心 Native API
- [ ] **Go 实现**
  - [ ] `__native.http_request(url, method, headers, body)` - HTTP 请求
  - [ ] `__native.json_encode(value)` - JSON 编码
  - [ ] `__native.json_decode(str)` - JSON 解码
  - [ ] `__native.log(level, ...)` - 日志输出
  - [ ] `__native.sleep(ms)` - 等待（如需要）
- [ ] **Rust 实现** (与 Go 保持一致)
  - [ ] `__native.http_request`
  - [ ] `__native.json_encode`
  - [ ] `__native.json_decode`
  - [ ] `__native.log`
  - [ ] `__native.sleep`

### 2.3 模块加载器 (`require`)
- [ ] **Go 实现**
  - [ ] 实现自定义 `require` 函数
  - [ ] 支持模块缓存 (`package.loaded`)
  - [ ] 支持目录模块 (`libs/<name>/init.luau`)
  - [ ] 支持单文件模块 (`libs/<name>.luau`)
  - [ ] 支持内嵌资源加载（编译时 embed）
- [ ] **Rust 实现** (与 Go 保持一致)

### 2.4 Luau SDK（纯 Luau 代码，Go/Rust 共用）
- [ ] 创建 `libs/` 目录结构
- [ ] 实现 `libs/haivivi/init.luau` - 主入口
- [ ] 实现 `libs/haivivi/http.luau` - HTTP 客户端封装
- [ ] 实现 `libs/haivivi/context.luau` - Model Context Provider
- [ ] 实现 `libs/haivivi/agent.luau` - Agent 配置管理
- [ ] 实现 `libs/haivivi/music.luau` - 音乐播放控制
- [ ] 实现 `libs/haivivi/types.luau` - 类型定义

### 2.5 双向测试（同一套脚本测 Go 和 Rust）
- [ ] 创建 `tests/luau/` 目录
- [ ] **Native API 测试**
  - [ ] `test_native_http.luau`
  - [ ] `test_native_json.luau`
  - [ ] `test_native_log.luau`
- [ ] **模块加载测试**
  - [ ] `test_require.luau`
  - [ ] `test_require_cache.luau`
- [ ] **SDK 测试**
  - [ ] `test_sdk_http.luau`
  - [ ] `test_sdk_context.luau`
  - [ ] `test_sdk_agent.luau`
- [ ] **Bazel 集成**
  - [ ] 配置 Go Runner 测试 target
  - [ ] 配置 Rust Runner 测试 target
  - [ ] CI 自动双向验证

### 2.6 文档和示例
- [ ] 编写 SDK API 文档
- [ ] 编写 Model Context Provider 示例
- [ ] 编写 Agent 配置示例
- [ ] 编写音乐播放示例

---

## 阶段三：Lua Context API

### 3.1 设计 Context API
- [ ] `ctx.generate(model, prompt)` - 调用 Generator
- [ ] `ctx.generate_json(model, prompt, schema)` - 生成 JSON
- [ ] `ctx.create_agent(name, config)` - 创建子 Agent
- [ ] `ctx.http.get/post()` - HTTP 请求（复用阶段二的实现）
- [ ] `ctx.state.xxx` - 状态读写
- [ ] `ctx.emit(chunk)` - 输出 MessageChunk

### 3.2 实现 LuaTool
- [ ] 创建 `go/pkg/genx/agent/tool_lua.go`
- [ ] 实现 `LuaTool` 结构体
- [ ] 实现 Invoke 方法（执行 Lua 脚本）
- [ ] 实现 ctx 注入
- [ ] 编写测试

---

## 阶段四：重构 Tool 系统

### 4.1 统一为 LuaTool
- [ ] 评估现有 tool 类型迁移方案
- [ ] 将 HTTPTool 逻辑迁移到 Lua（ctx.http）
- [ ] 将 GeneratorTool 逻辑迁移到 Lua（ctx.generate）
- [ ] 将 CompositeTool 逻辑迁移到 Lua（脚本流程控制）
- [ ] 更新 agentcfg 配置格式

### 4.2 清理旧代码
- [ ] 标记旧 tool 类型为 deprecated
- [ ] 迁移现有测试用例
- [ ] 移除旧实现（待确认）

---

## 阶段五：Agent I/O 语义优化

### 5.1 ReActAgent
- [ ] 确认 Tool 同步阻塞语义
- [ ] 移除任何 I/O 透传逻辑
- [ ] 更新文档

### 5.2 MatchAgent
- [ ] 保持透传语义（router 行为）
- [ ] 优化意图切换逻辑
- [ ] 更新文档

---

## 阶段六：RealtimeAgent

### 6.1 设计
- [ ] 定义 RealtimeAgent 接口
- [ ] 设计与 OpenAI/Gemini Realtime API 的映射

### 6.2 实现
- [ ] 创建 `go/pkg/genx/agent/agent_realtime.go`
- [ ] 实现 WebSocket 连接管理
- [ ] 实现 Input（audio/text MessageChunk）
- [ ] 实现 Next（转换 realtime event 为 AgentEvent）
- [ ] 实现 Interrupt
- [ ] 编写测试

---

## 阶段七：多路 Stream 支持

### 7.1 AgentStateID 分流
- [ ] 确认 AgentEvent.AgentStateID 设计
- [ ] 实现上层 Multiplexer（按 ID 分流）
- [ ] 支持多路 audio/text stream

---

## 文档更新

- [x] 更新 `docs/zh/lib/genx/agent/doc.md` - 整体架构
- [x] 更新 `docs/zh/lib/genx/agent/go.md` - Go 接口
- [x] 更新 `docs/zh/lib/genx/agent/issues.md` - 问题列表
- [x] 新增 `docs/zh/lib/genx/agent/luau.md` - Luau 脚本系统（含库系统设计）
- [ ] 新增 `docs/zh/lib/genx/agent/realtime.md` - RealtimeAgent（待实现后补充）
