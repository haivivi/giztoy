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

## 阶段二：Haivivi Luau SDK

### 2.1 目录结构

```
luau/
├── BUILD.bazel
├── c/                          # 现有 C wrapper
├── libs/                       # Luau SDK 库
│   └── haivivi/
│       ├── init.luau           # 主入口
│       ├── http.luau           # HTTP 客户端封装
│       ├── auth.luau           # Auth SDK
│       ├── pal.luau            # PAL SDK
│       └── aiot.luau           # AIOT SDK
└── tests/                      # 测试脚本
    └── haivivi/
        ├── test_auth.luau
        ├── test_pal.luau
        └── test_aiot.luau

testdata/luau/haivivi/          # 测试数据（Mock request/response）
├── auth/
│   ├── refresh_token_req.json
│   └── refresh_token_resp.json
├── pal/
│   ├── characters_list_resp.json
│   ├── voices_list_resp.json
│   └── virtual_devices_get_resp.json
└── aiot/
    ├── projects_get_resp.json
    └── gears_list_resp.json
```

### 2.2 临时 Runner（仅用于测试 SDK）

Go/Rust 仅提供最小的 builtin API，所有业务逻辑在 Luau 层实现。
Go 和 Rust Runner 并行开发，共用同一套测试数据和 Luau SDK。

- [x] **Go Runner** `go/cmd/luau/` ✅
  - [x] 实现 `__builtin.http(request)` - HTTP 请求
  - [x] 实现 `__builtin.json_encode(value)` - JSON 编码
  - [x] 实现 `__builtin.json_decode(str)` - JSON 解码
  - [x] 实现 `__builtin.kvs_get(key)` - KVS 读取
  - [x] 实现 `__builtin.kvs_set(key, value)` - KVS 写入
  - [x] 实现 `__builtin.kvs_del(key)` - KVS 删除
  - [x] 实现 `__builtin.log(...)` - 日志输出
  - [x] 实现 `__builtin.env(key)` - 环境变量读取
  - [x] 实现 `require` 模块加载（从文件系统加载 `luau/libs/`）
  - [x] 编写 Bazel 构建规则

- [x] **Rust Runner** `rust/cmd/luau/` ✅
  - [x] 实现 `__builtin.http(request)` - HTTP 请求 (通过 reqwest)
  - [x] 实现 `__builtin.json_encode(value)` - JSON 编码
  - [x] 实现 `__builtin.json_decode(str)` - JSON 解码
  - [x] 实现 `__builtin.kvs_get(key)` - KVS 读取
  - [x] 实现 `__builtin.kvs_set(key, value)` - KVS 写入
  - [x] 实现 `__builtin.kvs_del(key)` - KVS 删除
  - [x] 实现 `__builtin.log(...)` - 日志输出
  - [x] 实现 `__builtin.env(key)` - 环境变量读取
  - [x] 实现 `require` 模块加载（从文件系统加载 `luau/libs/`）
  - [x] 编写 Bazel 构建规则
  - [x] ✅ **HTTP 异步模式已实现**
    - 添加 `--async` / `-a` 命令行标志启用异步模式
    - 异步模式下 HTTP 请求使用协程 yield/resume，不阻塞其他请求
    - 同步模式（默认）使用 `block_in_place` + `block_on` 保持兼容

### 2.3 Haivivi SDK（纯 Luau 代码）✅

基于 Haivivi OpenAPI 实现的 SDK：

- [x] **HTTP 客户端** `luau/libs/haivivi/http.luau`
  - [x] 封装 `__builtin.http`
  - [x] 支持 base_url 配置
  - [x] 支持默认 headers
  - [x] 支持 auth token 注入
  - [x] 实现 GET/POST/PUT/DELETE/PATCH 方法
  - [x] 实现 query string 编码
  - [x] 实现错误处理

- [x] **Resource 抽象** `luau/libs/haivivi/resource.luau`
  - [x] 实现通用 ResourceCollection CRUD 封装
  - [x] 实现 list/get/create/update/delete 方法
  - [x] 实现 post_verb/get_verb/post_doc_verb 方法

- [x] **Auth SDK** `luau/libs/haivivi/auth.luau`
  - [x] 实现 `auth.new_client(base_url, key)`
  - [x] 实现 token 刷新逻辑（/me/@refresh）
  - [x] 使用 kvs 缓存 token
  - [x] 实现 `client:http_client()` 返回带认证的 HTTP 客户端
  - [x] Sessions 资源
  - [x] Users 资源
  - [x] Namespaces 资源

- [x] **PAL SDK** `luau/libs/haivivi/pal.luau`
  - [x] 实现 `pal.new_client(base_url, auth_client)`
  - [x] 实现 `refresh_token(key)` - 设备 token 刷新
  - [x] 实现 `setup(uat, eid, vid)` - 设备设置
  - [x] Characters 资源
  - [x] Voices 资源
  - [x] ChatTopics 资源
  - [x] VirtualDevices 资源
  - [x] Albums 资源
  - [x] Firmwares 资源
  - [x] Triggers 资源
  - [x] TTSModels 资源
  - [x] TunedLLMs 资源
  - [x] Memberships 资源
  - [x] Orders 资源
  - [x] Payments 资源
  - [x] Plans 资源
  - [x] Subscriptions 资源
  - [x] Tags 资源
  - [x] AccessPolicies 资源
  - [x] Achievements 资源
  - [x] AchievementTypes 资源
  - [x] AchievementProgresses 资源
  - [x] DeviceLogs 资源
  - [x] DeviceGiftCards 资源
  - [x] Campaigns 资源
  - [x] PresetPrompts 资源
  - [x] Reports 资源
  - [x] Series 资源

- [x] **AIOT SDK** `luau/libs/haivivi/aiot.luau`
  - [x] 实现 `aiot.new_client(base_url, auth_client)`
  - [x] Projects 资源（含 list/get/create/update/upsert/delete）
  - [x] Projects.key(key) 获取项目文档
  - [x] Gears 子资源（含 get_by_sn, sn, state, sign_token）
  - [x] Agents 子资源（含 register）

- [x] **主入口** `luau/libs/haivivi/init.luau`
  - [x] 导出所有模块（http, auth, pal, aiot, resource）

### 2.4 测试数据（Mock）✅

在 `testdata/luau/haivivi/` 准备 Mock 数据，用于单元测试：

- [x] **Auth Mock 数据**
  - [x] `auth/refresh_token_req.json` - 刷新 token 请求
  - [x] `auth/refresh_token_resp.json` - 刷新 token 响应

- [x] **PAL Mock 数据**
  - [x] `pal/characters_list_resp.json` - Characters 列表响应
  - [x] `pal/voices_list_resp.json` - Voices 列表响应
  - [x] `pal/virtual_devices_get_resp.json` - VirtualDevice 详情响应

- [x] **AIOT Mock 数据**
  - [x] `aiot/projects_get_resp.json` - Project 详情响应
  - [x] `aiot/gears_list_resp.json` - Gears 列表响应

### 2.5 测试（通过 Bazel 执行）✅

测试分两种模式：
1. **Mock 测试** - 使用 testdata 中的 mock 数据，不需要网络
2. **集成测试** - 使用 stage 环境 `https://api.stage.haivivi.cn`

- [x] **Auth 测试** `luau/tests/haivivi/test_auth.luau` (3/3 通过)
  - [x] 测试 token 刷新
  - [x] 测试 token 缓存
  - [x] 测试 HTTP client 创建

- [x] **PAL 测试** `luau/tests/haivivi/test_pal.luau` (5/5 通过)
  - [x] 测试 Characters.List
  - [x] 测试 Voices.List
  - [x] 测试 VirtualDevices.List
  - [x] 测试 ChatTopics.List
  - [x] 测试 Plans.List

- [x] **AIOT 测试** `luau/tests/haivivi/test_aiot.luau` (4/4 通过)
  - [x] 测试 Projects.List
  - [x] 测试 Projects.Key
  - [x] 测试 Gears.List
  - [x] 测试 Agents.List

- [x] **Bazel 集成** ✅
  - [x] 配置 `sh_test` 规则（Go Runner）
  - [x] 配置 `sh_test` 规则（Rust Runner）
  - [ ] CI 自动执行测试

---

## 阶段三：genx/luau Context API

### 3.1 两种执行模式

Luau 脚本有两种运行模式：

| 模式 | 入口函数 | I/O | 用途 |
|------|---------|-----|------|
| **Tool** | `invoke(ctx, args) -> result` | 参数进，return 出 | 离散任务 |
| **Agent** | `run(ctx)` 或 `on_input(ctx, input)` | `recv()/emit()` | 对话 Agent |

### 3.2 共享 API（Tool 和 Agent 都有）

- [ ] **HTTP**
  - [ ] `ctx.http.get(url, opts?)` - ⏳ async
  - [ ] `ctx.http.post(url, opts?)` - ⏳ async
  - [ ] `ctx.http.request(method, url, opts?)` - ⏳ async

- [ ] **LLM 生成**
  - [ ] `ctx.generate(model, prompt, opts?)` - ⏳ async
  - [ ] `ctx.generate_json(model, prompt, schema, opts?)` - ⏳ async

- [ ] **Tool 调用**
  - [ ] `ctx.invoke(tool_name, args)` - ⏳ async

- [ ] **子 Agent 管理**
  - [ ] `ctx.create_agent(name, config?)` - 🔄 sync
  - [ ] `agent:send(contents)` - 🔄 sync
  - [ ] `agent:iter()` - ⏳ async
  - [ ] `agent:collect()` - ⏳ async
  - [ ] `agent:close()` - 🔄 sync

- [ ] **Realtime 会话**
  - [ ] `ctx.realtime.connect(model, opts?)` - ⏳ async
  - [ ] `session:send_audio(data)` - 🔄 sync
  - [ ] `session:send_text(text)` - 🔄 sync
  - [ ] `session:wait_for(event_type)` - ⏳ async
  - [ ] `session:events()` - ⏳ async
  - [ ] `session:cancel()` - 🔄 sync
  - [ ] `session:close()` - 🔄 sync

- [ ] **Agent State（完整访问）**
  - [ ] `ctx.agent.state.key` - 🔄 sync（KV 读写，通过 metatable）
  - [ ] `ctx.agent.state:keys()` - 🔄 sync
  - [ ] `ctx.agent.state:clear()` - 🔄 sync
  - [ ] `ctx.agent.state:all()` - 🔄 sync
  - [ ] `ctx.agent.history:recent(n?)` - 🔄 sync
  - [ ] `ctx.agent.history:append(msg)` - 🔄 sync
  - [ ] `ctx.agent.history:revert()` - 🔄 sync
  - [ ] `ctx.agent.memory:summary()` - 🔄 sync
  - [ ] `ctx.agent.memory:set_summary(s)` - 🔄 sync
  - [ ] `ctx.agent.memory:query(q)` - ⏳ async

- [ ] **Agent 信息（只读）**
  - [ ] `ctx.agent.name` - 🔄 sync
  - [ ] `ctx.agent.model` - 🔄 sync
  - [ ] `ctx.agent.state_id` - 🔄 sync

- [ ] **运行时信息（只读）**
  - [ ] `ctx.runtime.request_id` - 🔄 sync
  - [ ] `ctx.runtime.user_id` - 🔄 sync
  - [ ] `ctx.runtime.trace_id` - 🔄 sync

- [ ] **日志**
  - [ ] `ctx.log.debug(...)` - 🔄 sync
  - [ ] `ctx.log.info(...)` - 🔄 sync
  - [ ] `ctx.log.warn(...)` - 🔄 sync
  - [ ] `ctx.log.error(...)` - 🔄 sync

### 3.3 Tool 独有 API

- [ ] `ctx.input()` - ⏳ async（等待输入，用于需要多轮交互的 Tool）
- [ ] `ctx.output(result)` - 🔄 sync（返回结果）
- [ ] 或直接 `return result`

### 3.4 Agent 独有 API

- [ ] `ctx.recv()` - ⏳ async（等待输入，nil = 已关闭）
- [ ] `ctx.emit(chunk)` - 🔄 sync（发送输出，chunk.eof=true 标记本轮结束）

### 3.5 异步实现（协程 + goroutine）

Host 函数需要支持 yield/resume 实现异步：

```
Lua 协程              Go 调度器                 Go goroutine
─────────             ─────────                 ─────────────
    │
    │ ctx.http.get(url)
    │──────────────────►│
    │                   │  go func() {
    │                   │      http.Get(url)  ────────►│
    │   yield           │  }()                         │
    │◄──────────────────│                              │
    │                   │                              │
    │  (暂停)           │  select {                    │
    │                   │      case <-readyChan:       │
    │                   │  }                           │
    │                   │                              │  HTTP 完成
    │                   │◄─────────────────────────────│
    │                   │  readyChan <- result         │
    │                   │                              │
    │ resume(result)    │
    │◄──────────────────│
    │
    │ local resp = ...   -- 继续执行
```

- [ ] 实现 Scheduler（管理协程 + I/O）
- [ ] 实现 Host 函数 yield（发起异步操作后立即 yield）
- [ ] 实现 goroutine 完成后通知调度器 resume
- [ ] 支持 Luau 协程并行执行（多个 HTTP 请求并行）

### 3.6 Go 接口设计

```go
// ToolContext Tool 模式接口
type ToolContext interface {
    Context() context.Context
    
    // HTTP
    HTTPGet(url string, opts *HTTPOptions) (*HTTPResponse, error)
    HTTPPost(url string, opts *HTTPOptions) (*HTTPResponse, error)
    
    // LLM
    Generate(model, prompt string, opts *GenerateOptions) (string, error)
    GenerateJSON(model, prompt string, schema any, opts *GenerateOptions) (any, error)
    
    // Tool
    Invoke(toolName string, args any) (any, error)
    
    // Agent State
    AgentStateGet(key string) (any, bool)
    AgentStateSet(key string, value any)
    AgentHistoryRecent(n int) ([]Message, error)
    AgentHistoryAppend(msg Message) error
    AgentHistoryRevert() error
    
    // Runtime
    RequestID() string
    UserID() string
    
    // Log
    Log(level string, args ...any)
}

// AgentContext Agent 模式接口
type AgentContext interface {
    ToolContext  // 包含所有 Tool 能力
    
    // I/O
    Recv() (*Contents, error)  // 阻塞等待输入或关闭
    Emit(chunk *MessageChunk) error
}
```

- [ ] 定义 `ToolContext` 接口
- [ ] 定义 `AgentContext` 接口
- [ ] 实现 `runtimeToolContext`（包装 agent.Runtime + AgentState）
- [ ] 实现 `runtimeAgentContext`

### 3.7 实现 LuaTool

- [ ] 创建 `go/pkg/genx/luau/` 包（独立，可单独测试）
- [ ] 实现 `Runner` 结构体
  - [ ] StatePool（Luau State 池化）
  - [ ] CompiledScripts（字节码缓存）
  - [ ] Scheduler（协程调度器）
- [ ] 实现 Host Functions 注册
- [ ] 实现 `Invoke(ctx, tc, script, args)` 方法
- [ ] 编写单元测试（Mock ToolContext）

### 3.8 实现 LuaAgent

- [ ] 创建 `go/pkg/genx/agent/agent_lua.go`
- [ ] 实现 `LuaAgent` 结构体
- [ ] 实现事件驱动入口（`on_start/on_input/on_close`）
- [ ] 实现主循环入口（`run`）
- [ ] 实现 `recv()` yield/resume 机制
- [ ] 实现 `emit()` channel 输出
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
- [x] 新增 `docs/en/lib/genx/agent/luau.md` - Luau 脚本系统（英文版）
- [ ] 新增 `docs/zh/lib/genx/agent/realtime.md` - RealtimeAgent（待实现后补充）

---

## Known Issues

### LUAU-001: Rust Luau Binding 缺少协程/Thread API

**状态**: ✅ 已修复 (PR #52)

**描述**: `rust/luau/` binding 之前没有实现 Luau 协程（Thread）相关的 API，导致 Rust runner 无法实现异步 yield/resume 机制。

**已实现的 API**（与 Go binding `go/pkg/luau/` 对齐）:

| API | Go binding | Rust binding |
|-----|:----------:|:------------:|
| `Thread` struct | ✅ | ✅ |
| `NewThread()` | ✅ | ✅ |
| `Resume(nargs)` | ✅ | ✅ |
| `Yield(nresults)` | ✅ | ✅ |
| `IsYieldable()` | ✅ | ✅ |
| `Status()` / `CoStatus` | ✅ | ✅ |

**修复内容**:
1. ✅ 在 `rust/luau/src/ffi.rs` 添加 FFI 绑定
2. ✅ 在 `rust/luau/src/lib.rs` 实现 `Thread` struct 和 `CoStatus` enum
3. ✅ 使用 `impl_lua_stack_ops!` 宏消除 State 和 Thread 的代码重复
4. ✅ 添加 12 个协程相关测试用例
5. ✅ `rust/cmd/luau/` 异步调度循环（`--async` 标志）
