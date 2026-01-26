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
│   └── luau/                    # Luau 源码 (通过 Bazel http_archive 获取)
│       ├── BUILD.bazel          # 构建静态库
│       └── defs.bzl             # repository rule 定义
│
├── luau/
│   └── c/                       # C Wrapper
│       ├── BUILD.bazel
│       ├── luau_wrapper.h
│       └── luau_wrapper.cpp
│
├── go/pkg/luau/                 # Go Binding
│   ├── BUILD.bazel
│   ├── luau.go
│   ├── luau_test.go
│   └── doc.go
│
├── rust/luau/                   # Rust Binding
│   ├── BUILD.bazel
│   ├── Cargo.toml
│   └── src/
│       ├── lib.rs
│       ├── ffi.rs
│       └── tests.rs
│
└── go/pkg/genx/agent/
    └── tool_lua.go              # （规划中）LuaTool 实现
```

## 与其他语言的绑定

| 语言 | 方案 | 备注 |
|-----|------|-----|
| **Go** | CGO + C Wrapper | 自定义 crate `go/pkg/luau` |
| **Rust** | FFI + C Wrapper | 自定义 crate `rust/luau` (giztoy-luau) |
| **Zig** | ziglua with Luau | 现成库 |

## Luau 库系统（SDK 架构）

为了让 Luau 脚本能够访问 Haivivi API、配置 Agent 参数、实现 Model Context Provider 等高级功能，我们设计了一套分层的库系统。

### 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                   Luau Script (用户代码)                 │
│                                                         │
│   local haivivi = require("haivivi")                    │
│   local ctx = haivivi.context.create({                  │
│       model = "gpt-4",                                  │
│       temperature = 0.7                                 │
│   })                                                    │
│   haivivi.music.play("song_id")                         │
│                                                         │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│                 Luau SDK (纯 Luau 代码)                  │
│                                                         │
│   libs/haivivi/init.luau     -- 主入口                  │
│   libs/haivivi/context.luau  -- Model Context Provider  │
│   libs/haivivi/agent.luau    -- Agent 配置              │
│   libs/haivivi/music.luau    -- 音乐控制                │
│                                                         │
│   封装 Native API，提供友好的 Lua 风格接口              │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│             Native API (Go/Rust RegisterFunc)           │
│                                                         │
│   __native.context_create(opts)                         │
│   __native.context_set(ctx, key, value)                 │
│   __native.http_request(url, method, body)              │
│   __native.music_play(song_id)                          │
│   __native.log(level, message)                          │
│                                                         │
│   由 Go/Rust 通过 RegisterFunc 注入                     │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│                Go/Rust Implementation                    │
│                                                         │
│   实际的 HTTP 调用、数据库访问、音频控制等               │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 模块加载器 (`require`)

Luau 本身支持 `require`，但需要自定义 loader 来加载我们的 SDK 模块：

```go
// Go 端注册 require 函数
state.RegisterFunc("require", func(L *State) int {
    moduleName := L.ToString(1)
    
    // 1. 检查缓存 (package.loaded)
    // 2. 查找模块 (从 libs/ 目录或内嵌资源)
    // 3. 编译并执行模块脚本
    // 4. 缓存结果到 package.loaded
    // 5. 返回模块导出的 table
    
    return 1
})
```

模块搜索路径：
1. `package.loaded[name]` - 已加载的缓存
2. `libs/<name>/init.luau` - 目录模块
3. `libs/<name>.luau` - 单文件模块
4. 内嵌资源 (编译时嵌入的标准库)

### SDK 目录结构

```
libs/
├── haivivi/
│   ├── init.luau          -- 主入口，导出所有子模块
│   ├── http.luau          -- HTTP 客户端封装
│   ├── context.luau       -- Model Context Provider
│   ├── agent.luau         -- Agent 配置和管理
│   ├── music.luau         -- 音乐播放控制
│   └── types.luau         -- 类型定义
│
├── json/
│   └── init.luau          -- JSON 编码/解码
│
└── utils/
    ├── init.luau          -- 通用工具函数
    ├── string.luau        -- 字符串工具
    └── table.luau         -- 表工具
```

### Native API 设计

Native API 使用 `__native` 前缀，表示底层实现，不建议用户直接调用：

| API | 描述 | 返回值 |
|-----|------|--------|
| `__native.http_request(url, method, headers, body)` | HTTP 请求 | `{status, headers, body}` |
| `__native.json_encode(value)` | JSON 编码 | `string` |
| `__native.json_decode(str)` | JSON 解码 | `any` |
| `__native.context_create(opts)` | 创建 Context | `context_id` |
| `__native.context_set(id, key, value)` | 设置 Context 值 | `void` |
| `__native.context_get(id, key)` | 获取 Context 值 | `any` |
| `__native.agent_register(name, config)` | 注册 Agent | `void` |
| `__native.music_play(song_id, opts)` | 播放音乐 | `void` |
| `__native.music_stop()` | 停止播放 | `void` |
| `__native.log(level, ...)` | 日志输出 | `void` |
| `__native.sleep(ms)` | 异步等待 | `void` |

### SDK 封装示例

**haivivi/http.luau**:
```lua
local M = {}

function M.get(url: string, params: {[string]: any}?): HttpResponse
    local query = ""
    if params then
        local parts = {}
        for k, v in pairs(params) do
            table.insert(parts, k .. "=" .. tostring(v))
        end
        query = "?" .. table.concat(parts, "&")
    end
    
    local result = __native.http_request(url .. query, "GET", {}, nil)
    return {
        status = result.status,
        headers = result.headers,
        body = result.body,
        data = __native.json_decode(result.body)
    }
end

function M.post(url: string, opts: {headers: {[string]: string}?, body: any}?): HttpResponse
    local headers = opts and opts.headers or {}
    local body = opts and opts.body
    
    if type(body) == "table" then
        headers["Content-Type"] = "application/json"
        body = __native.json_encode(body)
    end
    
    local result = __native.http_request(url, "POST", headers, body)
    return {
        status = result.status,
        headers = result.headers,
        body = result.body,
        data = __native.json_decode(result.body)
    }
end

return M
```

**haivivi/context.luau**:
```lua
local M = {}

-- Context Provider 类
local Provider = {}
Provider.__index = Provider

function Provider.new(config: {name: string, description: string?})
    local self = setmetatable({}, Provider)
    self.name = config.name
    self.description = config.description or ""
    self._id = __native.context_create({
        name = config.name,
        description = config.description
    })
    return self
end

function Provider:set(key: string, value: any)
    __native.context_set(self._id, key, value)
end

function Provider:get(key: string): any
    return __native.context_get(self._id, key)
end

function Provider:get_context(query: string): {[string]: any}
    -- 由子类重写
    error("get_context must be implemented by subclass")
end

M.Provider = Provider

return M
```

### 使用示例

**实现一个 Model Context Provider**:
```lua
local haivivi = require("haivivi")
local json = require("json")

-- 创建自定义 Context Provider
local MyProvider = {}
setmetatable(MyProvider, { __index = haivivi.context.Provider })

function MyProvider.new()
    local self = haivivi.context.Provider.new({
        name = "my_provider",
        description = "自定义上下文提供器"
    })
    setmetatable(self, { __index = MyProvider })
    return self
end

function MyProvider:get_context(query: string)
    -- 调用 Haivivi API 获取上下文
    local response = haivivi.http.get("https://api.haivivi.com/v1/search", {
        query = query,
        limit = 10
    })
    
    if response.status ~= 200 then
        return { error = "API 调用失败" }
    end
    
    return {
        documents = response.data.results,
        metadata = {
            source = "haivivi",
            query = query,
            count = #response.data.results
        }
    }
end

-- 注册到 Agent 系统
local provider = MyProvider.new()
haivivi.agent.register_context_provider(provider)

-- 导出
return provider
```

**音乐播放控制**:
```lua
local haivivi = require("haivivi")

-- 播放音乐
haivivi.music.play("song_123", {
    volume = 0.8,
    loop = false
})

-- 暂停
haivivi.music.pause()

-- 恢复
haivivi.music.resume()

-- 停止
haivivi.music.stop()

-- 获取播放状态
local status = haivivi.music.status()
print(status.playing)   -- true/false
print(status.position)  -- 当前位置（秒）
print(status.duration)  -- 总时长（秒）
```

## 安全考虑

Luau 天然移除了不安全的功能：

- ❌ `io`, `os.execute` - 无文件/系统访问
- ❌ `loadfile`, `dofile` - 无动态加载
- ❌ `debug` 库大部分功能 - 无运行时修改
- ✅ 所有外部访问都通过 `ctx` 或 SDK - 可控、可审计

## 性能

| 场景 | 耗时 |
|------|------|
| 简单脚本执行 | < 1ms |
| 复杂逻辑 + 循环 | < 10ms |
| 带 HTTP 调用 | 取决于网络 |
| 带 Generator 调用 | 取决于 LLM |

Luau 脚本本身的执行开销可忽略，主要耗时在外部调用。

## Luau Runner

为了测试 Luau SDK 和运行 Luau 脚本，我们提供 Go 和 Rust 两个 Runner 实现。

### 架构

```
┌─────────────────────────────────────────────────────────┐
│                    Luau Runner                          │
│                                                         │
│  1. 创建 Luau State                                     │
│  2. 注入 Native API (__native.*)                        │
│  3. 注入模块加载器 (require)                            │
│  4. 执行用户脚本                                        │
│                                                         │
└─────────────────────────────────────────────────────────┘
                           │
        ┌──────────────────┴──────────────────┐
        ▼                                      ▼
┌───────────────────┐                ┌───────────────────┐
│    Go Runner      │                │   Rust Runner     │
│                   │                │                   │
│ go/cmd/luau/      │                │ rust/cmd/luau/    │
└───────────────────┘                └───────────────────┘
```

### 用法

```bash
# Go Runner
bazel run //go/cmd/luau -- script.luau

# Rust Runner  
bazel run //rust/cmd/luau -- script.luau

# 运行测试
bazel run //go/cmd/luau -- tests/luau/test_http.luau
```

### 双向测试

使用同一套 Luau 测试脚本，可以同时验证：

1. **Go/Rust Binding 正确性** - Native API 实现是否一致
2. **Luau SDK 正确性** - SDK 逻辑是否正确
3. **跨语言一致性** - Go 和 Rust 行为是否相同

```
tests/luau/
├── test_native_http.luau      # 测试 __native.http_request
├── test_native_json.luau      # 测试 __native.json_*
├── test_sdk_http.luau         # 测试 haivivi.http
├── test_sdk_context.luau      # 测试 haivivi.context
└── test_require.luau          # 测试模块加载
```

```bash
# 同一个测试，两个 Runner 都跑
bazel run //go/cmd/luau -- tests/luau/test_sdk_http.luau
bazel run //rust/cmd/luau -- tests/luau/test_sdk_http.luau

# CI 中自动双向验证
bazel test //tests/luau:all
```

### 测试示例

```lua
-- tests/luau/test_native_json.luau

-- 测试 JSON 编码
local encoded = __native.json_encode({ name = "test", value = 123 })
assert(type(encoded) == "string", "should return string")

-- 测试 JSON 解码
local decoded = __native.json_decode('{"foo": "bar", "num": 42}')
assert(decoded.foo == "bar", "should decode string")
assert(decoded.num == 42, "should decode number")

-- 测试往返
local original = { a = 1, b = "hello", c = true }
local roundtrip = __native.json_decode(__native.json_encode(original))
assert(roundtrip.a == 1)
assert(roundtrip.b == "hello")
assert(roundtrip.c == true)

print("✅ test_native_json passed")
```

```lua
-- tests/luau/test_sdk_http.luau

local http = require("haivivi/http")

-- 测试 GET 请求
local resp = http.get("https://httpbin.org/get", { foo = "bar" })
assert(resp.status == 200, "should return 200")
assert(resp.data.args.foo == "bar", "should have query param")

-- 测试 POST 请求
local resp2 = http.post("https://httpbin.org/post", {
    body = { message = "hello" }
})
assert(resp2.status == 200, "should return 200")
assert(resp2.data.json.message == "hello", "should have body")

print("✅ test_sdk_http passed")
```

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
