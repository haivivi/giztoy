# Luau Scripting System

GenX Agent uses [Luau](https://luau-lang.org/) as the scripting language for Tools and Agents.

## Why Luau

| Feature | Luau | Alternatives |
|---------|------|--------------|
| **Type System** | ✅ Gradual typing | Lua 5.x has none |
| **Performance** | ✅ Optimized bytecode interpreter | QuickJS slower |
| **Embedding** | ✅ Designed for embedding | TypeScript needs transpiling |
| **Safety** | ✅ Native sandboxing | Others need extra work |
| **Binary Size** | ~500 KB | V8 ~28MB |
| **Maintenance** | ✅ Active (Roblox team) | LuaJIT stalled |

Luau is developed by Roblox and powers 70M+ daily active users.

## Two Execution Modes

Luau scripts run in two modes with different capabilities:

```
┌─────────────────────────────────────────────────────────────────┐
│                        Agent Runtime                             │
│                                                                  │
│  ┌──────────────┐                  ┌──────────────┐             │
│  │   LuaTool    │                  │  LuaAgent    │             │
│  │  (Passive)   │                  │  (Active)    │             │
│  │              │                  │              │             │
│  │ invoke(ctx,  │                  │ ctx.recv()   │             │
│  │   args)      │                  │ ctx.emit()   │             │
│  │   -> result  │                  │              │             │
│  └──────────────┘                  └──────────────┘             │
└─────────────────────────────────────────────────────────────────┘
```

### Tool Mode

- **Entry**: `invoke(ctx, args) -> result`
- **I/O**: Arguments in, return value out
- **Use case**: Discrete tasks (weather lookup, calculations, etc.)

### Agent Mode

- **Entry**: `run(ctx)` or `on_input(ctx, input)`
- **I/O**: `ctx.recv()` and `ctx.emit()`
- **Use case**: Conversational agents, streaming processors

## Context API

### Shared API (Tool + Agent)

```lua
-- ═══════════════════════════════════════════════════════════════
-- HTTP (async, yields)
-- ═══════════════════════════════════════════════════════════════
ctx.http.get(url, opts?)        -- ⏳ async
ctx.http.post(url, opts?)       -- ⏳ async
ctx.http.request(method, url, opts?)  -- ⏳ async

-- Response structure
-- {
--   status = 200,
--   headers = { ["Content-Type"] = "application/json" },
--   body = "...",
--   json = { ... }  -- auto-parsed if JSON
-- }

-- ═══════════════════════════════════════════════════════════════
-- LLM Generation (async, yields)
-- ═══════════════════════════════════════════════════════════════
ctx.generate(model, prompt, opts?)      -- ⏳ async, returns string
ctx.generate_json(model, prompt, schema, opts?)  -- ⏳ async, returns table

-- ═══════════════════════════════════════════════════════════════
-- Tool Invocation (async, yields)
-- ═══════════════════════════════════════════════════════════════
ctx.invoke(tool_name, args)     -- ⏳ async

-- ═══════════════════════════════════════════════════════════════
-- Child Agent Management
-- ═══════════════════════════════════════════════════════════════
ctx.create_agent(name, config?) -- 🔄 sync (creates, doesn't wait)

agent:send(contents)            -- 🔄 sync (sends to input channel)
agent:iter()                    -- ⏳ async (iterate output chunks)
agent:collect()                 -- ⏳ async (collect all output)
agent:close()                   -- 🔄 sync

-- ═══════════════════════════════════════════════════════════════
-- Realtime Session (for voice/audio)
-- ═══════════════════════════════════════════════════════════════
ctx.realtime.connect(model, opts?)  -- ⏳ async (establish WebSocket)

session:send_audio(data)        -- 🔄 sync
session:send_text(text)         -- 🔄 sync
session:wait_for(event_type)    -- ⏳ async
session:events()                -- ⏳ async (iterate events)
session:cancel()                -- 🔄 sync
session:close()                 -- 🔄 sync

-- ═══════════════════════════════════════════════════════════════
-- Agent State (full access)
-- ═══════════════════════════════════════════════════════════════

-- Key-Value state (via metatable, triggers host functions)
ctx.agent.state.key             -- 🔄 sync (read)
ctx.agent.state.key = value     -- 🔄 sync (write)
ctx.agent.state:keys()          -- 🔄 sync
ctx.agent.state:clear()         -- 🔄 sync
ctx.agent.state:all()           -- 🔄 sync

-- Message history
ctx.agent.history:recent(n?)    -- 🔄 sync (get recent N messages)
ctx.agent.history:append(msg)   -- 🔄 sync (add message)
ctx.agent.history:revert()      -- 🔄 sync (undo last round)

-- Long-term memory
ctx.agent.memory:summary()      -- 🔄 sync (get summary)
ctx.agent.memory:set_summary(s) -- 🔄 sync
ctx.agent.memory:query(q)       -- ⏳ async (RAG query)

-- Agent info (read-only)
ctx.agent.name                  -- 🔄 sync
ctx.agent.model                 -- 🔄 sync
ctx.agent.state_id              -- 🔄 sync

-- ═══════════════════════════════════════════════════════════════
-- Runtime Info (read-only)
-- ═══════════════════════════════════════════════════════════════
ctx.runtime.request_id          -- 🔄 sync
ctx.runtime.user_id             -- 🔄 sync
ctx.runtime.trace_id            -- 🔄 sync

-- ═══════════════════════════════════════════════════════════════
-- Logging
-- ═══════════════════════════════════════════════════════════════
ctx.log.debug(...)              -- 🔄 sync
ctx.log.info(...)               -- 🔄 sync
ctx.log.warn(...)               -- 🔄 sync
ctx.log.error(...)              -- 🔄 sync
```

### Tool-Only API

```lua
-- Return result to caller
-- Option 1: Direct return
function invoke(ctx, args)
    return { result = "..." }
end

-- Option 2: Explicit output (for early return)
function invoke(ctx, args)
    if args.invalid then
        ctx.output({ error = "invalid args" })
        return
    end
    return { result = "..." }
end

-- Option 3: Multi-turn tool (wait for additional input)
function invoke(ctx, args)
    ctx.output({ status = "need_confirmation", data = args })
    local confirmation = ctx.input()  -- wait for user confirmation
    if confirmation.confirmed then
        return { result = "done" }
    end
    return { result = "cancelled" }
end
```

### Agent-Only API

```lua
-- I/O
ctx.recv()                      -- ⏳ async (wait for input, nil = closed)
ctx.emit(chunk)                 -- 🔄 sync (send output chunk)
                                --   chunk.eof = true marks end of turn
```

## Async Implementation

Luau uses coroutines for concurrency. Host functions yield to Go/Rust, which resumes after I/O completes.

```
Lua Coroutine         Go Scheduler              Go Goroutine
─────────────         ────────────              ────────────
    │
    │ ctx.http.get(url)
    │──────────────────►│
    │                   │  go func() {
    │                   │      http.Get(url)  ────────►│
    │   yield           │  }()                         │
    │◄──────────────────│                              │
    │                   │                              │
    │  (suspended)      │  select {                    │  (blocking)
    │                   │      case <-readyChan:       │
    │                   │  }                           │
    │                   │                              │
    │                   │                              │  HTTP done
    │                   │◄─────────────────────────────│
    │                   │  readyChan <- result         │
    │                   │                              │
    │ resume(result)    │
    │◄──────────────────│
    │
    │ local resp = ...   -- continue with result
    │
```

### Parallel Execution

```lua
-- Multiple coroutines can run concurrently
local co1 = coroutine.create(function()
    return ctx.http.get("https://api1.com")  -- yields
end)

local co2 = coroutine.create(function()
    return ctx.http.get("https://api2.com")  -- yields
end)

-- Both HTTP requests execute in parallel
coroutine.resume(co1)  -- starts request 1, yields
coroutine.resume(co2)  -- starts request 2, yields

-- Go scheduler manages completion and resumes appropriately
```

## Examples

### Tool: Weather Lookup

```lua
function invoke(ctx, args)
    ctx.log.info("Querying weather for:", args.city)
    
    local resp = ctx.http.get("https://api.weather.com/v1", {
        query = { city = args.city }
    })
    
    if resp.status ~= 200 then
        return { error = "API request failed" }
    end
    
    -- Store in agent state for future reference
    ctx.agent.state.last_weather_query = args.city
    
    return {
        city = args.city,
        temperature = resp.json.temp,
        description = resp.json.desc
    }
end
```

### Agent: Chat Bot

```lua
function run(ctx)
    ctx.emit({ text = "Hello! How can I help you?" })
    ctx.emit({ eof = true })
    
    while true do
        local input = ctx.recv()
        if input == nil then break end
        
        -- Generate response using LLM
        local response = ctx.generate("gpt-4o", 
            "User said: " .. input.text .. "\nRespond helpfully:")
        
        ctx.emit({ text = response })
        ctx.emit({ eof = true })
    end
end
```

### Agent: Chat Processor with Realtime + Match

```lua
-- Complex parallel processing example
function on_input(ctx, input)
    local asr_text = nil
    local match_result = nil
    local realtime_response = nil
    
    -- Coroutine 1: Handle realtime model
    local realtime_co = coroutine.create(function()
        local session = ctx.realtime.connect("gpt-4o-realtime")
        session:send_audio(input.audio)
        
        -- Wait for ASR result
        local event = session:wait_for("asr_done")
        asr_text = event.text
        
        return session:collect_response()
    end)
    
    -- Coroutine 2: Do intent matching after ASR
    local match_co = coroutine.create(function()
        -- Wait for ASR to complete
        while asr_text == nil do
            coroutine.yield()
        end
        
        return ctx.invoke("intent_match", { text = asr_text })
    end)
    
    -- Run both coroutines (scheduler handles parallelism)
    local ok1, result1 = coroutine.resume(realtime_co)
    local ok2, result2 = coroutine.resume(match_co)
    
    -- ... scheduler runs until both complete ...
    -- (In a real implementation, the scheduler would resume coroutines
    -- as their async operations complete)
    
    -- Collect final results after coroutines complete
    if coroutine.status(realtime_co) == "dead" then
        realtime_response = result1
    end
    if coroutine.status(match_co) == "dead" then
        match_result = result2
    end
    
    -- Use match result if available, otherwise use realtime response
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

## API Summary

| API | Tool | Agent | Sync/Async |
|-----|:----:|:-----:|:----------:|
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

**Legend:**
- ⏳ async - Yields, waits for I/O completion, then resumes
- 🔄 sync - Returns immediately, no yield

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        genx/luau Package                         │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                        Runner                             │   │
│  │  - StatePool (pooled Luau states)                        │   │
│  │  - CompiledScripts (bytecode cache)                      │   │
│  │  - Scheduler (coroutine + I/O management)                │   │
│  └──────────────────────────────────────────────────────────┘   │
│                             │                                    │
│                             ▼                                    │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                    Luau State (from pool)                 │   │
│  │                                                           │   │
│  │  Host Functions (registered via CGO):                    │   │
│  │    ctx.http.*      → HTTPGet/HTTPPost                    │   │
│  │    ctx.generate*   → Generate/GenerateJSON               │   │
│  │    ctx.agent.*     → AgentState methods                  │   │
│  │    ctx.recv/emit   → I/O channels                        │   │
│  │                                                           │   │
│  └──────────────────────────────────────────────────────────┘   │
│                             │                                    │
│                             ▼                                    │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                  Luau C++ Runtime                         │   │
│  │  - VM (bytecode execution)                               │   │
│  │  - Compiler (source → bytecode)                          │   │
│  │  - Coroutine support (yield/resume)                      │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Go Interface

```go
// ToolContext interface for Tool mode
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

// AgentContext interface for Agent mode
type AgentContext interface {
    ToolContext  // includes all Tool capabilities
    
    // I/O
    Recv() (*Contents, error)  // blocks until input or close
    Emit(chunk *MessageChunk) error
}
```

## Related

- [Agent Framework Overview](doc.md)
- [Agent Configuration](../agentcfg/doc.md)
- [Pattern Matching](../match/doc.md)
