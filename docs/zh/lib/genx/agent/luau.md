# Luau 脚本系统

GenX Agent 使用 [Luau](https://luau-lang.org/) 作为统一的 Tool 脚本语言。

## 为什么选择 Luau

| 特性 | Luau | 其他选项 |
|------|------|---------|
| **类型系统** | ✅ 渐进类型 | Lua 5.x 无类型 |
| **性能** | ✅ 优化的字节码解释器 + 可选 JIT | QuickJS 较慢 |
| **嵌入设计** | ✅ 原生为嵌入设计 | TypeScript 需转译 |
| **安全** | ✅ 天然沙盒化 | 其他需额外处理 |
| **体积** | ~500 KB | V8 28MB |

Luau 由 Roblox 开发维护，已被 Alan Wake 2、Warframe 等游戏采用。

## 架构

```
┌─────────────────────────────────────────────────┐
│                 Go / Rust                        │
│                                                  │
│  ┌───────────────────────────────────────────┐  │
│  │              LuaTool                       │  │
│  │  - 加载 Luau 脚本                          │  │
│  │  - 注入 ctx 对象                           │  │
│  │  - 执行 invoke 函数                        │  │
│  └───────────────────────────────────────────┘  │
│                      │                           │
│                      ▼                           │
│  ┌───────────────────────────────────────────┐  │
│  │           Luau C++ Runtime                 │  │
│  │  - VM (字节码执行)                          │  │
│  │  - Compiler (源码编译)                      │  │
│  │  - CodeGen (可选 JIT)                       │  │
│  └───────────────────────────────────────────┘  │
│                      │                           │
│                      ▼                           │
│  ┌───────────────────────────────────────────┐  │
│  │              C Wrapper                     │  │
│  │  luau_wrapper.h / luau_wrapper.cpp        │  │
│  └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

## Context API

每个 Tool 脚本通过 `ctx` 对象访问 Agent 能力：

### Generator 调用

```lua
-- 文本生成
local result: string = ctx.generate("gpt-4o", "写一首诗")

-- JSON 生成（带 schema 验证）
local data = ctx.generate_json("gpt-4o", "提取信息", {
    type = "object",
    properties = {
        name = { type = "string" },
        age = { type = "number" }
    }
})

-- 流式生成
for chunk in ctx.generate_stream("gpt-4o", "写一个故事") do
    ctx.emit({ text = chunk })
end
```

### Agent 管理

```lua
-- 创建子 Agent
local agent = ctx.create_agent("assistant", {
    prompt = "你是助手...",
    model = "gpt-4o",
    memory = { recent = 20 }  -- 可选：独立记忆
})

-- 输入
agent:input("你好")

-- 流式读取输出
for chunk in agent:iter() do
    ctx.emit(chunk)
end

-- 或收集全部输出
local result = agent:collect()

-- 检查是否需要更多输入
if agent:needs_input() then
    agent:input("继续")
end

-- 关闭
agent:close()
```

### HTTP 请求

```lua
-- GET 请求
local resp = ctx.http.get("https://api.example.com/data", {
    query = "test",
    limit = 10
})

-- POST 请求
local resp = ctx.http.post("https://api.example.com/submit", {
    headers = { ["Content-Type"] = "application/json" },
    body = { name = "test" }
})

-- 访问响应
print(resp.status)      -- 200
print(resp.body)        -- 响应体字符串
print(resp.data.field)  -- JSON 自动解析
```

### 状态管理

```lua
-- 状态存储在父 Agent 的 state 中，可持久化
ctx.state.counter = (ctx.state.counter or 0) + 1
ctx.state.last_query = args.query
ctx.state.results = { "a", "b", "c" }

-- 读取
local count = ctx.state.counter
```

### 输出控制

```lua
-- 发送 MessageChunk 到 Agent 输出流
ctx.emit({ text = "处理中..." })
ctx.emit({ type = "progress", percent = 50 })
ctx.emit({ audio = audio_data })
```

## 类型注解

Luau 支持可选的类型注解，提升代码质量：

```lua
-- 类型定义
type Character = {
    name: string,
    age: number,
    personality: string,
    speak: (self: Character, text: string) -> string
}

type WeatherResponse = {
    city: string,
    temperature: number,
    description: string
}

-- 函数类型注解
function invoke(ctx: Context, args: {city: string}): WeatherResponse
    local resp = ctx.http.get("https://api.weather.com", {
        city = args.city
    })
    
    return {
        city = args.city,
        temperature = resp.data.temp,
        description = resp.data.desc
    }
end
```

## Tool 定义格式

### 内联脚本

```yaml
name: get_weather
description: 查询指定城市的天气
params:
  type: object
  properties:
    city:
      type: string
      description: 城市名称
  required: [city]
lua: |
  function invoke(ctx, args)
    local resp = ctx.http.get("https://api.weather.com/v1", {
      city = args.city
    })
    return {
      temperature = resp.data.temp,
      description = resp.data.desc
    }
  end
```

### 外部文件

```yaml
name: story_generator
description: 生成多角色故事
params:
  type: object
  properties:
    theme:
      type: string
      description: 故事主题
    characters:
      type: integer
      description: 角色数量
lua_file: scripts/story_generator.lua
```

## 错误处理

```lua
function invoke(ctx, args)
    -- 使用 pcall 捕获错误
    local ok, result = pcall(function()
        return ctx.http.get("https://api.example.com")
    end)
    
    if not ok then
        -- 返回错误信息给 Agent
        return { error = "请求失败: " .. tostring(result) }
    end
    
    return result.data
end
```

## 项目结构

```
giztoy/
├── third_party/
│   └── luau/                    # Luau 源码 (git submodule)
│       ├── Ast/
│       ├── Compiler/
│       ├── VM/
│       └── CodeGen/
│
├── luau/
│   ├── BUILD.bazel
│   ├── c/                       # C Wrapper
│   │   ├── luau_wrapper.h
│   │   └── luau_wrapper.cpp
│   └── go/                      # Go Binding
│       ├── luau.go
│       └── luau_test.go
│
└── go/pkg/genx/agent/
    └── tool_lua.go              # LuaTool 实现
```

## 与其他语言的绑定

| 语言 | 方案 | 备注 |
|-----|------|-----|
| **Go** | CGO + C Wrapper | 自己实现 |
| **Rust** | mlua with luau feature | 现成库 |
| **Zig** | ziglua with Luau | 现成库 |

## 安全考虑

Luau 天然移除了不安全的功能：

- ❌ `io`, `os.execute` - 无文件/系统访问
- ❌ `loadfile`, `dofile` - 无动态加载
- ❌ `debug` 库大部分功能 - 无运行时修改
- ✅ 所有外部访问都通过 `ctx` - 可控、可审计

## 性能

| 场景 | 耗时 |
|------|------|
| 简单脚本执行 | < 1ms |
| 复杂逻辑 + 循环 | < 10ms |
| 带 HTTP 调用 | 取决于网络 |
| 带 Generator 调用 | 取决于 LLM |

Luau 脚本本身的执行开销可忽略，主要耗时在外部调用。

## 调试

```lua
-- 使用 print 输出调试信息（会记录到日志）
print("Debug:", args.city)
print("Response:", resp.data)

-- 使用 ctx.log 分级日志
ctx.log.debug("Processing...")
ctx.log.info("Result: ", result)
ctx.log.warn("Slow response")
ctx.log.error("Failed: ", err)
```
