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
  - [x] 实现 `__builtin.http(request)` - HTTP 请求 (通过 curl)
  - [x] 实现 `__builtin.json_encode(value)` - JSON 编码
  - [x] 实现 `__builtin.json_decode(str)` - JSON 解码
  - [x] 实现 `__builtin.kvs_get(key)` - KVS 读取
  - [x] 实现 `__builtin.kvs_set(key, value)` - KVS 写入
  - [x] 实现 `__builtin.kvs_del(key)` - KVS 删除
  - [x] 实现 `__builtin.log(...)` - 日志输出
  - [x] 实现 `__builtin.env(key)` - 环境变量读取
  - [x] 实现 `require` 模块加载（从文件系统加载 `luau/libs/`）
  - [x] 编写 Bazel 构建规则

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

## 阶段三：Lua Context API

### 3.1 设计 Context API
- [ ] `ctx.generate(model, prompt)` - 调用 Generator
- [ ] `ctx.generate_json(model, prompt, schema)` - 生成 JSON
- [ ] `ctx.create_agent(name, config)` - 创建 子 Agent
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
