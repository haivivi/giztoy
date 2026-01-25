# GenX Agent - 已知问题与计划

## 🔴 重大变更计划

### AGT-001: Tool 系统重构为 Luau

**状态:** 计划中

**描述:**  
当前有多种 Tool 类型（GeneratorTool、HTTPTool、CompositeTool 等），计划统一为 Luau 脚本实现。

**变更内容:**
- 所有 Tool 统一用 Luau 脚本实现
- 提供 `ctx` API：generate、http、state、emit 等
- 现有 Tool 类型标记为 deprecated

**好处:**
- 统一的脚本能力
- 支持复杂流程控制
- 热更新友好
- 可测试性提升

**进度:** 见 [TODO.md](TODO.md)

---

### AGT-002: Agent I/O 语义明确化

**状态:** 计划中

**描述:**  
明确不同 Agent 类型的 I/O 语义：

| Agent 类型 | I/O 行为 |
|-----------|---------|
| ReActAgent | Tool 同步阻塞，不透传 |
| MatchAgent | 透传给子 Agent |
| RealtimeAgent | 双向流（新增） |

**当前问题:**
- ReActAgent 的 "agent tool" 语义不清晰
- 子 Agent "接管" 父 Agent I/O 的设计不够优雅

**解决方案:**
- ReActAgent：Tool 永远是同步的，复杂流程由 Luau 脚本控制
- MatchAgent：保持 Router 语义，透传子 Agent I/O
- 复杂多 Agent 协作由 Luau 脚本显式编排

---

### AGT-003: RealtimeAgent 实现

**状态:** 计划中

**描述:**  
新增 RealtimeAgent 类型，包装 OpenAI/Gemini Realtime API。

**功能:**
- 输入：MessageChunk（text/audio）
- 输出：AgentEvent（转换自 realtime events）
- 支持双向流式交互
- 支持 Interrupt

---

## 🟡 已知问题

### AGT-004: 部分内部错误使用 panic

**描述:**  
某些意外状态触发 panic 而非返回 error。

**影响:** 边缘情况可能导致应用崩溃。

**建议:** 将 panic 转换为 error 返回。

---

### AGT-005: 事件循环复杂度

**描述:**  
事件循环模式需要仔细处理所有事件类型。

**影响:** 客户端代码容易遗漏边缘情况。

**建议:** 提供 helper 函数或简化 API。

---

## 🔵 增强计划

### AGT-006: 状态持久化

**描述:**  
Agent 状态序列化/反序列化，支持重启恢复。

**当前状态:** 已实现基础的 State 接口，需要实现持久化存储。

---

### AGT-007: 调试工具

**描述:**  
增强对 Agent 推理和 Tool 选择的可观测性。

**计划:**
- Verbose 模式
- Step-through 调试
- Luau 脚本调试支持

---

### AGT-008: Tool 结果缓存

**描述:**  
相同输入的 Tool 调用可以缓存结果。

**计划:** 在 Luau ctx 中提供 cache API。

---

## ⚪ 设计亮点

### AGT-009: 事件系统设计良好

**描述:**  
基于事件的 API 提供了出色的控制能力：
- 流式输出块
- Tool 执行可见性
- AgentStateID 支持多路分流

---

### AGT-010: Quit Tool 模式

**描述:**  
优雅的 Agent 终止方式：
```yaml
tools:
  - $ref: tool:goodbye
    quit: true
```

---

### AGT-011: 多 Agent 路由

**描述:**  
MatchAgent 支持复杂的多技能架构：
```
Router → Weather Agent
       → Music Agent  
       → Chat Agent
```

---

### AGT-012: Luau 脚本系统（计划中）

**描述:**  
统一的脚本能力，支持：
- 调用 Generator
- 创建子 Agent
- HTTP 请求
- 状态管理
- 流式输出

详见 [luau.md](luau.md)

---

## 状态总结

| ID | 严重程度 | 状态 | 组件 |
|----|---------|------|------|
| AGT-001 | 🔴 重大 | 计划中 | Tool |
| AGT-002 | 🔴 重大 | 计划中 | Agent I/O |
| AGT-003 | 🔴 重大 | 计划中 | RealtimeAgent |
| AGT-004 | 🟡 次要 | 待修复 | Go |
| AGT-005 | 🟡 次要 | 待修复 | Go |
| AGT-006 | 🔵 增强 | 进行中 | State |
| AGT-007 | 🔵 增强 | 计划中 | Debug |
| AGT-008 | 🔵 增强 | 计划中 | Cache |
| AGT-009 | ⚪ 亮点 | N/A | Event |
| AGT-010 | ⚪ 亮点 | N/A | Quit |
| AGT-011 | ⚪ 亮点 | N/A | Match |
| AGT-012 | ⚪ 亮点 | 计划中 | Luau |

**整体状态:** 正在进行重大架构改进，统一 Tool 系统为 Luau 脚本，明确 Agent I/O 语义。
