# GenX Agent - TODO

## 阶段一：Luau 引入

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
- [x] 编写测试（40+ 测试用例，覆盖各种 Luau 脚本场景）
- [x] 编写 benchmark（执行/编译/栈操作/表操作/内存/实际场景）

### 1.4 Rust Binding
- [x] 选择方案：使用 C wrapper 的 FFI 绑定（与 Go 保持一致）
- [x] 创建 `rust/luau/` crate
- [x] 封装统一 API（State, Type, OptLevel）与 Go 保持一致
- [x] 编写测试（29 测试用例）
- [ ] 编写 benchmark（使用 criterion）- 待补充

---

## 阶段二：Lua Context API

### 2.1 设计 Context API
- [ ] `ctx.generate(model, prompt)` - 调用 Generator
- [ ] `ctx.generate_json(model, prompt, schema)` - 生成 JSON
- [ ] `ctx.create_agent(name, config)` - 创建子 Agent
- [ ] `ctx.http.get/post()` - HTTP 请求
- [ ] `ctx.state.xxx` - 状态读写
- [ ] `ctx.emit(chunk)` - 输出 MessageChunk

### 2.2 实现 LuaTool
- [ ] 创建 `go/pkg/genx/agent/tool_lua.go`
- [ ] 实现 `LuaTool` 结构体
- [ ] 实现 Invoke 方法（执行 Lua 脚本）
- [ ] 实现 ctx 注入
- [ ] 编写测试

---

## 阶段三：重构 Tool 系统

### 3.1 统一为 LuaTool
- [ ] 评估现有 tool 类型迁移方案
- [ ] 将 HTTPTool 逻辑迁移到 Lua（ctx.http）
- [ ] 将 GeneratorTool 逻辑迁移到 Lua（ctx.generate）
- [ ] 将 CompositeTool 逻辑迁移到 Lua（脚本流程控制）
- [ ] 更新 agentcfg 配置格式

### 3.2 清理旧代码
- [ ] 标记旧 tool 类型为 deprecated
- [ ] 迁移现有测试用例
- [ ] 移除旧实现（待确认）

---

## 阶段四：Agent I/O 语义优化

### 4.1 ReActAgent
- [ ] 确认 Tool 同步阻塞语义
- [ ] 移除任何 I/O 透传逻辑
- [ ] 更新文档

### 4.2 MatchAgent
- [ ] 保持透传语义（router 行为）
- [ ] 优化意图切换逻辑
- [ ] 更新文档

---

## 阶段五：RealtimeAgent

### 5.1 设计
- [ ] 定义 RealtimeAgent 接口
- [ ] 设计与 OpenAI/Gemini Realtime API 的映射

### 5.2 实现
- [ ] 创建 `go/pkg/genx/agent/agent_realtime.go`
- [ ] 实现 WebSocket 连接管理
- [ ] 实现 Input（audio/text MessageChunk）
- [ ] 实现 Next（转换 realtime event 为 AgentEvent）
- [ ] 实现 Interrupt
- [ ] 编写测试

---

## 阶段六：多路 Stream 支持

### 6.1 AgentStateID 分流
- [ ] 确认 AgentEvent.AgentStateID 设计
- [ ] 实现上层 Multiplexer（按 ID 分流）
- [ ] 支持多路 audio/text stream

---

## 文档更新

- [x] 更新 `docs/zh/lib/genx/agent/doc.md` - 整体架构
- [x] 更新 `docs/zh/lib/genx/agent/go.md` - Go 接口
- [x] 更新 `docs/zh/lib/genx/agent/issues.md` - 问题列表
- [x] 新增 `docs/zh/lib/genx/agent/luau.md` - Luau 脚本系统
- [ ] 新增 `docs/zh/lib/genx/agent/realtime.md` - RealtimeAgent（待实现后补充）
