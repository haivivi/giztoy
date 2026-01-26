# GenX Agent 框架

构建 LLM 驱动的自主代理框架。

## 设计目标

1. **统一的 Agent 架构**：用 Agent 系统统一处理不同类型的交互
2. **基于事件的 API**：对代理执行的细粒度控制，支持多路 Stream 分流
3. **Luau 脚本系统**：（规划中）使用 Luau 脚本实现 Tool 逻辑
4. **多 Agent 协作**：支持复杂的多 Agent 流程编排

## 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        Session                               │
│              管理连接、认证、多 Agent 生命周期                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                 Root Agent (MatchAgent)                      │
│                    意图匹配 + 路由                            │
└─────────────────────────────────────────────────────────────┘
              │              │              │
              ▼              ▼              ▼
       ReActAgent      ReActAgent     RealtimeAgent
       (聊天对话)       (故事生成)      (语音实时)
              │              │              │
              ▼              ▼              ▼
         AgentEvent     AgentEvent     AgentEvent
              │              │              │
              └──────────────┴──────────────┘
                              │
                              ▼
                  按 AgentStateID 分流多路 Stream
                         │         │
                         ▼         ▼
                   Text Stream  Audio Stream
```

## Agent 类型

### ReActAgent

实现推理和行动（ReAct）模式：
- 逐步思考用户请求
- 选择并执行 Tool（Luau 脚本）来完成任务
- Tool 执行是**同步阻塞**的，不透传 I/O

```
Input("查天气") → LLM 决定调用 tool → Tool 执行（阻塞）→ LLM 继续生成 → EventEOF
```

### MatchAgent

实现基于意图的路由：
- 将用户输入与预定义规则匹配
- 路由到适当的子 Agent
- **透传** 子 Agent 的 I/O（Router 语义）

```
Input("播放音乐") → 匹配 music 意图 → 路由到 MusicAgent → 透传 I/O
```

### RealtimeAgent

实现语音实时交互：
- 包装 OpenAI/Gemini Realtime API
- 输入输出都是 MessageChunk（text/audio）
- 双向流式交互

```
Input(AudioChunk) → Realtime API → AgentEvent(AudioChunk)
```

## Agent I/O 语义

| Agent 类型 | I/O 行为 | 原因 |
|-----------|---------|------|
| **ReActAgent** | Tool 同步阻塞，不透传 | Tool 是内部执行细节 |
| **MatchAgent** | 透传给子 Agent | Router 语义，本身不产内容 |
| **RealtimeAgent** | 双向流 MessageChunk | 语音实时场景 |

## 事件系统

Agent 通过事件进行通信：

| 事件 | 描述 |
|------|------|
| `EventChunk` | 输出块（text/audio） |
| `EventEOF` | 当前轮次结束，等待输入 |
| `EventClosed` | Agent 完成（quit tool 或 close） |
| `EventToolStart` | Tool 执行开始 |
| `EventToolDone` | Tool 成功完成 |
| `EventToolError` | Tool 执行失败 |
| `EventInterrupted` | Agent 被中断 |

### AgentEvent 结构

```go
type AgentEvent struct {
    Type         EventType
    AgentDef     string           // Agent 定义名称
    AgentStateID string           // Agent 实例状态 ID
    Chunk        *MessageChunk    // EventChunk
    ToolCall     *ToolCall        // EventToolStart
    ToolResult   *ToolResult      // EventToolDone
    ToolError    error            // EventToolError
}
```

**关键设计**：每个 Event 都带有 `AgentStateID`，上层可以按此 ID 将不同 Agent 的输出分流到不同的 Stream（如 text stream、audio stream、music stream）。

## Tool 系统

### 统一为 Luau 脚本

所有 Tool 都用 Luau 脚本实现：

```lua
-- tool: get_weather
function invoke(ctx, args)
    -- 发 HTTP 请求
    local resp = ctx.http.get("https://api.weather.com", {
        city = args.city
    })
    
    -- 可以调用 Generator
    local summary = ctx.generate("gpt-4o-mini", 
        "用一句话总结天气：" .. resp.body)
    
    return {
        city = args.city,
        temperature = resp.data.temp,
        summary = summary
    }
end
```

### Lua Context API

```lua
ctx = {
    -- LLM 生成
    generate = function(model, prompt) -> string,
    generate_json = function(model, prompt, schema) -> table,
    
    -- Agent 管理
    create_agent = function(name, config) -> Agent,
    
    -- HTTP 请求
    http = {
        get = function(url, params) -> Response,
        post = function(url, body) -> Response,
    },
    
    -- 状态管理（存储在父 Agent 的 state 中）
    state = { ... },
    
    -- 输出控制
    emit = function(chunk),  -- 流式输出 MessageChunk
}
```

### 复杂流程示例

```lua
-- tool: story_generator
function invoke(ctx, args)
    -- 创建作家 Agent
    local writer = ctx.create_agent("writer", {
        prompt = "你是故事作家...",
        model = "gpt-4o"
    })
    
    -- 生成大纲
    writer:input("写一个校园故事")
    local outline = writer:collect()
    ctx.state.outline = outline
    
    -- 为每个角色创建独立 Agent
    local characters = {}
    for _, char in ipairs(outline.characters) do
        characters[char.name] = ctx.create_agent("character", {
            prompt = string.format("你是 %s...", char.name),
            model = "gpt-4o"
        })
    end
    
    -- 创建导演 Agent
    local director = ctx.create_agent("director", {...})
    
    -- 逐章生成
    for _, chapter in ipairs(outline.chapters) do
        ctx.emit({ type = "chapter_start", title = chapter.title })
        
        while true do
            director:input("谁该说话了？")
            local decision = director:collect()
            
            if decision.action == "END" then break end
            
            local speaker = characters[decision.speaker]
            speaker:input(decision.instruction)
            
            for chunk in speaker:iter() do
                ctx.emit({
                    type = "dialog",
                    speaker = decision.speaker,
                    chunk = chunk
                })
            end
        end
    end
    
    return { status = "completed" }
end
```

## 配置格式

### Agent 配置

```yaml
type: react
name: assistant
prompt: |
  你是一个有帮助的助手。
generator:
  model: gpt-4o
tools:
  - $ref: tool:get_weather
  - $ref: tool:play_music
```

### Tool 配置

```yaml
name: get_weather
description: 查询天气
params:
  city: { type: string, description: 城市名 }
lua: |
  function invoke(ctx, args)
    local resp = ctx.http.get("https://api.weather.com", {city = args.city})
    return { temp = resp.data.temp }
  end

# 或引用外部文件
# lua_file: scripts/get_weather.lua
```

## 多路 Stream 分流

利用 `AgentEvent.AgentStateID` 实现多路分流：

```go
// 上层 Multiplexer
for {
    evt, _ := rootAgent.Next()
    
    switch evt.AgentStateID {
    case musicAgentState:
        musicStream.Write(evt.Chunk)
    case chatAgentState:
        textStream.Write(evt.Chunk)
    case realtimeAgentState:
        audioStream.Write(evt.Chunk)
    }
}
```

这样不同 Agent 的输出可以路由到不同的物理通道（如设备的不同音轨）。

## 相关文档

- Luau 脚本系统：[luau.md](luau.md)
- Go 实现：[go.md](go.md)
- 配置格式：[../agentcfg/](../agentcfg/)
- 模式匹配：[../match/](../match/)
- 待办事项：[TODO.md](TODO.md)
