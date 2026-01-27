# Luau 脚本系统

GenX Agent 使用 [Luau](https://luau-lang.org/) 作为 Tool 和 Agent 的脚本语言。

## 为什么选择 Luau

| 特性 | Luau | 其他选项 |
|------|------|---------|
| **类型系统** | ✅ 渐进式类型 | Lua 5.x 无类型 |
| **性能** | ✅ 优化的字节码解释器 | QuickJS 较慢 |
| **嵌入设计** | ✅ 原生为嵌入设计 | TypeScript 需转译 |
| **安全** | ✅ 天然沙盒化 | 其他需额外处理 |
| **体积** | ~500 KB | V8 ~28MB |
| **维护** | ✅ 活跃（Roblox 团队）| LuaJIT 停滞 |

Luau 由 Roblox 开发，支撑 7000 万+ 日活用户。

## 两种执行模式

Luau 脚本有两种运行模式，具有不同的能力：

```
┌─────────────────────────────────────────────────────────────────┐
│                        Agent Runtime                             │
│                                                                  │
│  ┌──────────────┐                  ┌──────────────┐             │
│  │   LuaTool    │                  │  LuaAgent    │             │
│  │   (被动)     │                  │   (主动)      │             │
│  │              │                  │              │             │
│  │ invoke(ctx,  │                  │ ctx.recv()   │             │
│  │   args)      │                  │ ctx.emit()   │             │
│  │   -> result  │                  │              │             │
│  └──────────────┘                  └──────────────┘             │
└─────────────────────────────────────────────────────────────────┘
```

### Tool 模式

- **入口**：`invoke(ctx, args) -> result`
- **I/O**：参数输入，返回值输出
- **用途**：离散任务（天气查询、计算等）

### Agent 模式

- **入口**：`run(ctx)` 或 `on_input(ctx, input)`
- **I/O**：`ctx.recv()` 和 `ctx.emit()`
- **用途**：对话 Agent、流式处理器

## Context API

### 共享 API（Tool 和 Agent 都有）

```lua
-- ═══════════════════════════════════════════════════════════════
-- HTTP（异步，yield）
-- ═══════════════════════════════════════════════════════════════
ctx.http.get(url, opts?)        -- ⏳ async
ctx.http.post(url, opts?)       -- ⏳ async
ctx.http.request(method, url, opts?)  -- ⏳ async

-- 响应结构
-- {
--   status = 200,
--   headers = { ["Content-Type"] = "application/json" },
--   body = "...",
--   json = { ... }  -- 自动解析 JSON
-- }

-- ═══════════════════════════════════════════════════════════════
-- LLM 生成（异步，yield）
-- ═══════════════════════════════════════════════════════════════
ctx.generate(model, prompt, opts?)      -- ⏳ async，返回 string
ctx.generate_json(model, prompt, schema, opts?)  -- ⏳ async，返回 table

-- ═══════════════════════════════════════════════════════════════
-- 调用 Tool（异步，yield）
-- ═══════════════════════════════════════════════════════════════
ctx.invoke(tool_name, args)     -- ⏳ async

-- ═══════════════════════════════════════════════════════════════
-- 子 Agent 管理
-- ═══════════════════════════════════════════════════════════════
ctx.create_agent(name, config?) -- 🔄 sync（只创建，不等待）

agent:send(contents)            -- 🔄 sync（发送到输入 channel）
agent:iter()                    -- ⏳ async（迭代输出 chunk）
agent:collect()                 -- ⏳ async（收集全部输出）
agent:close()                   -- 🔄 sync

-- ═══════════════════════════════════════════════════════════════
-- Realtime 会话（语音/音频）
-- ═══════════════════════════════════════════════════════════════
ctx.realtime.connect(model, opts?)  -- ⏳ async（建立 WebSocket）

session:send_audio(data)        -- 🔄 sync
session:send_text(text)         -- 🔄 sync
session:wait_for(event_type)    -- ⏳ async
session:events()                -- ⏳ async（迭代事件）
session:cancel()                -- 🔄 sync
session:close()                 -- 🔄 sync

-- ═══════════════════════════════════════════════════════════════
-- Agent State（完整访问）
-- ═══════════════════════════════════════════════════════════════

-- Key-Value 状态（通过 metatable，触发 host 函数）
ctx.agent.state.key             -- 🔄 sync（读）
ctx.agent.state.key = value     -- 🔄 sync（写）
ctx.agent.state:keys()          -- 🔄 sync
ctx.agent.state:clear()         -- 🔄 sync
ctx.agent.state:all()           -- 🔄 sync

-- 消息历史
ctx.agent.history:recent(n?)    -- 🔄 sync（获取最近 N 条）
ctx.agent.history:append(msg)   -- 🔄 sync（添加消息）
ctx.agent.history:revert()      -- 🔄 sync（撤销最后一轮）

-- 长期记忆
ctx.agent.memory:summary()      -- 🔄 sync（获取摘要）
ctx.agent.memory:set_summary(s) -- 🔄 sync
ctx.agent.memory:query(q)       -- ⏳ async（RAG 查询）

-- Agent 信息（只读）
ctx.agent.name                  -- 🔄 sync
ctx.agent.model                 -- 🔄 sync
ctx.agent.state_id              -- 🔄 sync

-- ═══════════════════════════════════════════════════════════════
-- 运行时信息（只读）
-- ═══════════════════════════════════════════════════════════════
ctx.runtime.request_id          -- 🔄 sync
ctx.runtime.user_id             -- 🔄 sync
ctx.runtime.trace_id            -- 🔄 sync

-- ═══════════════════════════════════════════════════════════════
-- 日志
-- ═══════════════════════════════════════════════════════════════
ctx.log.debug(...)              -- 🔄 sync
ctx.log.info(...)               -- 🔄 sync
ctx.log.warn(...)               -- 🔄 sync
ctx.log.error(...)              -- 🔄 sync
```

### Tool 独有 API

```lua
-- 返回结果给调用者
-- 方式 1：直接 return
function invoke(ctx, args)
    return { result = "..." }
end

-- 方式 2：显式输出（用于提前返回）
function invoke(ctx, args)
    if args.invalid then
        ctx.output({ error = "invalid args" })
        return
    end
    return { result = "..." }
end

-- 方式 3：多轮交互的 Tool（等待额外输入）
function invoke(ctx, args)
    ctx.output({ status = "need_confirmation", data = args })
    local confirmation = ctx.input()  -- 等待用户确认
    if confirmation.confirmed then
        return { result = "done" }
    end
    return { result = "cancelled" }
end
```

### Agent 独有 API

```lua
-- I/O
ctx.recv()                      -- ⏳ async（等待输入，nil = 已关闭）
ctx.emit(chunk)                 -- 🔄 sync（发送输出 chunk）
                                --   chunk.eof = true 标记本轮结束
```

## 异步实现

Luau 使用协程实现并发。Host 函数 yield 回 Go/Rust，I/O 完成后 resume。

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
    │  (暂停)           │  select {                    │  (阻塞)
    │                   │      case <-readyChan:       │
    │                   │  }                           │
    │                   │                              │
    │                   │                              │  HTTP 完成
    │                   │◄─────────────────────────────│
    │                   │  readyChan <- result         │
    │                   │                              │
    │ resume(result)    │
    │◄──────────────────│
    │
    │ local resp = ...   -- 拿到结果，继续执行
    │
```

### 并行执行

```lua
-- 多个协程可以并发执行
local co1 = coroutine.create(function()
    return ctx.http.get("https://api1.com")  -- yield
end)

local co2 = coroutine.create(function()
    return ctx.http.get("https://api2.com")  -- yield
end)

-- 两个 HTTP 请求并行执行
coroutine.resume(co1)  -- 发起请求 1，yield
coroutine.resume(co2)  -- 发起请求 2，yield

-- Go 调度器管理完成和 resume
```

## 示例

### Tool：天气查询

```lua
function invoke(ctx, args)
    ctx.log.info("查询天气:", args.city)
    
    local resp = ctx.http.get("https://api.weather.com/v1", {
        query = { city = args.city }
    })
    
    if resp.status ~= 200 then
        return { error = "API 请求失败" }
    end
    
    -- 存储到 agent state 供后续引用
    ctx.agent.state.last_weather_query = args.city
    
    return {
        city = args.city,
        temperature = resp.json.temp,
        description = resp.json.desc
    }
end
```

### Agent：聊天机器人

```lua
function run(ctx)
    ctx.emit({ text = "你好！有什么可以帮助你的？" })
    ctx.emit({ eof = true })
    
    while true do
        local input = ctx.recv()
        if input == nil then break end
        
        -- 使用 LLM 生成响应
        local response = ctx.generate("gpt-4o", 
            "用户说: " .. input.text .. "\n请友好地回复:")
        
        ctx.emit({ text = response })
        ctx.emit({ eof = true })
    end
end
```

### Agent：Realtime + Match 的聊天处理器

```lua
-- 复杂的并行处理示例
function on_input(ctx, input)
    local asr_text = nil
    local match_result = nil
    local realtime_response = nil
    
    -- 协程 1：处理 realtime 模型
    local realtime_co = coroutine.create(function()
        local session = ctx.realtime.connect("gpt-4o-realtime")
        session:send_audio(input.audio)
        
        -- 等待 ASR 结果
        local event = session:wait_for("asr_done")
        asr_text = event.text
        
        return session:collect_response()
    end)
    
    -- 协程 2：ASR 完成后做意图匹配
    local match_co = coroutine.create(function()
        -- 等待 ASR 完成
        while asr_text == nil do
            coroutine.yield()
        end
        
        return ctx.invoke("intent_match", { text = asr_text })
    end)
    
    -- 运行两个协程（调度器处理并行）
    local ok1, result1 = coroutine.resume(realtime_co)
    local ok2, result2 = coroutine.resume(match_co)
    
    -- ... 调度器运行直到都完成 ...
    -- （实际实现中，调度器会在异步操作完成时 resume 协程）
    
    -- 协程完成后收集最终结果
    if coroutine.status(realtime_co) == "dead" then
        realtime_response = result1
    end
    if coroutine.status(match_co) == "dead" then
        match_result = result2
    end
    
    -- 有 match 结果用 match，否则用 realtime 响应
    if match_result and match_result.matched then
        local agent = ctx.create_agent(match_result.agent)
        agent:send(asr_text)
        for chunk in agent:iter() do
            ctx.emit(chunk)
        end
        agent:close()
    else
        for _, chunk in ipairs(realtime_response or {}) do
            ctx.emit(chunk)
        end
    end
    
    ctx.emit({ eof = true })
end
```

## API 汇总

| API | Tool | Agent | 同步/异步 |
|-----|:----:|:-----:|:--------:|
| `ctx.http.*` | ✅ | ✅ | ⏳ async |
| `ctx.generate*` | ✅ | ✅ | ⏳ async |
| `ctx.invoke()` | ✅ | ✅ | ⏳ async |
| `ctx.create_agent()` | ✅ | ✅ | 🔄 sync |
| `agent:iter/collect()` | ✅ | ✅ | ⏳ async |
| `ctx.realtime.*` | ✅ | ✅ | ⏳ async |
| `ctx.agent.state.*` | ✅ | ✅ | 🔄 sync |
| `ctx.agent.history.*` | ✅ | ✅ | 🔄 sync |
| `ctx.agent.memory.*` | ✅ | ✅ | 🔄/⏳ |
| `ctx.agent.name/model` | ✅ | ✅ | 🔄 sync |
| `ctx.runtime.*` | ✅ | ✅ | 🔄 sync |
| `ctx.log.*` | ✅ | ✅ | 🔄 sync |
| `ctx.input()` | ✅ | ❌ | ⏳ async |
| `ctx.output()` | ✅ | ❌ | 🔄 sync |
| `ctx.recv()` | ❌ | ✅ | ⏳ async |
| `ctx.emit()` | ❌ | ✅ | 🔄 sync |

**图例：**
- ⏳ async - yield 等待 I/O 完成后 resume
- 🔄 sync - 立即返回，不 yield

## 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        genx/luau 包                              │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                        Runner                             │   │
│  │  - StatePool（Luau State 池）                             │   │
│  │  - CompiledScripts（字节码缓存）                           │   │
│  │  - Scheduler（协程 + I/O 管理）                           │   │
│  └──────────────────────────────────────────────────────────┘   │
│                             │                                    │
│                             ▼                                    │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                    Luau State（从池中获取）                │   │
│  │                                                           │   │
│  │  Host Functions（通过 CGO 注册）：                        │   │
│  │    ctx.http.*      → HTTPGet/HTTPPost                    │   │
│  │    ctx.generate*   → Generate/GenerateJSON               │   │
│  │    ctx.agent.*     → AgentState 方法                     │   │
│  │    ctx.recv/emit   → I/O channel                         │   │
│  │                                                           │   │
│  └──────────────────────────────────────────────────────────┘   │
│                             │                                    │
│                             ▼                                    │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                  Luau C++ Runtime                         │   │
│  │  - VM（字节码执行）                                        │   │
│  │  - Compiler（源码 → 字节码）                               │   │
│  │  - Coroutine 支持（yield/resume）                         │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Go 接口

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

## 相关文档

- [Agent 框架概述](doc.md)
- [Agent 配置](../agentcfg/doc.md)
- [模式匹配](../match/doc.md)
