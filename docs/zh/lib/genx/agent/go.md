# GenX Agent - Go 实现

Import: `github.com/haivivi/giztoy/pkg/genx/agent`

## Agent 接口

```go
type Agent interface {
    // Def 返回 Agent 定义
    Def() agentcfg.Agent

    // State 返回 Agent 状态接口
    State() AgentState

    // StateID 返回状态 ID（用于持久化和分流）
    StateID() string

    // Input 接收用户输入
    Input(contents genx.Contents) error

    // Interrupt 中断当前输出
    Interrupt() error

    // Next 返回下一个事件
    Next() (*AgentEvent, error)

    // Revert 撤销最后一轮对话
    Revert() error

    // FormatHistory 格式化对话历史
    FormatHistory(ctx context.Context) string

    // Close 关闭 Agent
    Close() error
    CloseWithError(error) error
}
```

## AgentEvent

```go
type EventType int

const (
    EventChunk EventType = iota      // 输出块
    EventEOF                          // 轮次结束，等待输入
    EventClosed                       // Agent 完成
    EventToolStart                    // Tool 开始执行
    EventToolDone                     // Tool 完成
    EventToolError                    // Tool 失败
    EventInterrupted                  // 被中断
)

type AgentEvent struct {
    Type         EventType
    Phase        string               // 当前阶段
    AgentDef     string               // Agent 定义名称
    AgentStateID string               // Agent 实例 ID（用于分流）
    Chunk        *genx.MessageChunk   // EventChunk
    ToolCall     *genx.ToolCall       // EventToolStart
    ToolResult   *genx.ToolResult     // EventToolDone
    ToolError    error                // EventToolError
}

// IsTerminal 判断是否为终止事件
func (e *AgentEvent) IsTerminal() bool {
    return e.Type == EventClosed || e.Type == EventInterrupted
}
```

## ReActAgent

```go
// 创建 ReActAgent
agent, err := agent.NewReActAgent(ctx, &agentcfg.ReActAgent{
    Name:   "assistant",
    Prompt: "你是一个有帮助的助手。",
    Generator: agentcfg.GeneratorRef{
        Generator: &agentcfg.Generator{Model: "gpt-4o"},
    },
    Tools: []agentcfg.ToolRef{
        {Ref: "tool:get_weather"},
        {Ref: "tool:search", Quit: true},  // quit tool
    },
}, runtime, "")  // parentStateID 为空表示顶层 Agent

defer agent.Close()
```

## MatchAgent

```go
agent, err := agent.NewMatchAgent(ctx, &agentcfg.MatchAgent{
    Name: "router",
    Generator: agentcfg.GeneratorRef{
        Generator: &agentcfg.Generator{Model: "gpt-4o"},
    },
    Rules: []agentcfg.RuleRef{
        {Ref: "rule:weather"},
        {Ref: "rule:music"},
    },
    Route: []agentcfg.MatchRoute{
        {
            Rules: []string{"weather"},
            Agent: agentcfg.AgentRef{Ref: "agent:weather_assistant"},
        },
        {
            Rules: []string{"music"},
            Agent: agentcfg.AgentRef{Ref: "agent:music_player"},
        },
    },
}, runtime, "")
```

## 事件循环

```go
// 提供初始输入
agent.Input(genx.Contents{genx.Text("你好！")})

// 处理事件
for {
    evt, err := agent.Next()
    if err != nil {
        return err
    }

    switch evt.Type {
    case agent.EventChunk:
        // 输出块 - 可按 AgentStateID 分流
        fmt.Printf("[%s] %s", evt.AgentStateID, evt.Chunk.Part)

    case agent.EventEOF:
        // 轮次结束，提供新输入
        fmt.Print("\n> ")
        input := readline()
        agent.Input(genx.Contents{genx.Text(input)})

    case agent.EventClosed:
        // Agent 完成（quit tool 或 closed）
        return nil

    case agent.EventToolStart:
        fmt.Printf("[Tool: %s 开始]\n", evt.ToolCall.FuncCall.Name)

    case agent.EventToolDone:
        fmt.Printf("[Tool: %s 完成]\n", evt.ToolCall.FuncCall.Name)

    case agent.EventToolError:
        fmt.Printf("[Tool 错误: %v]\n", evt.ToolError)

    case agent.EventInterrupted:
        return nil
    }
}
```

## 多路 Stream 分流

```go
// 根据 AgentStateID 分流到不同 Stream
streams := make(map[string]*Stream)

for {
    evt, _ := rootAgent.Next()
    
    if evt.Type == agent.EventChunk {
        stream, ok := streams[evt.AgentStateID]
        if !ok {
            stream = NewStream(evt.AgentStateID)
            streams[evt.AgentStateID] = stream
        }
        stream.Write(evt.Chunk)
    }
}
```

## Runtime 接口

```go
type Runtime interface {
    // Generator 能力
    genx.Generator

    // GetTool 获取 Tool
    GetTool(ctx context.Context, name string) (*genx.FuncTool, error)

    // GetToolDef 获取 Tool 定义
    GetToolDef(ctx context.Context, name string) (agentcfg.Tool, error)

    // CreateToolFromDef 从定义创建 Tool
    CreateToolFromDef(ctx context.Context, def agentcfg.Tool) (*genx.FuncTool, error)

    // GetAgentDef 获取 Agent 定义
    GetAgentDef(ctx context.Context, name string) (agentcfg.Agent, error)

    // GetContextBuilder 获取 Context 构建器
    GetContextBuilder(ctx context.Context, name string) (ContextBuilder, error)

    // GetRule 获取匹配规则
    GetRule(ctx context.Context, name string) (*match.Rule, error)

    // --- 状态管理 ---

    // CreateReActState 创建 ReAct 状态
    CreateReActState(ctx context.Context, agentDef, parentStateID string) (ReActState, error)

    // CreateMatchState 创建 Match 状态
    CreateMatchState(ctx context.Context, agentDef, parentStateID string) (MatchState, error)

    // GetState 获取状态
    GetState(ctx context.Context, id string) (AgentState, error)

    // DestroyState 销毁状态
    DestroyState(ctx context.Context, id string, archive bool) error

    // RestoreAgent 从状态恢复 Agent
    RestoreAgent(ctx context.Context, stateID string) (Agent, error)
}
```

## LuaTool（计划中）

```go
// LuaTool 执行 Luau 脚本
type LuaTool struct {
    Name        string
    Description string
    Params      *jsonschema.Schema
    Script      string     // 内联脚本
    ScriptFile  string     // 外部脚本文件
}

// Invoke 执行脚本
func (t *LuaTool) Invoke(ctx context.Context, call *genx.FuncCall, args string) (any, error) {
    // 1. 创建 Luau 状态
    L := luau.New()
    defer L.Close()

    // 2. 注入 ctx 对象
    t.injectContext(L, ctx)

    // 3. 加载并执行脚本
    if err := L.DoString(t.Script); err != nil {
        return nil, err
    }

    // 4. 调用 invoke 函数
    return L.Call("invoke", args)
}
```

## 状态接口

```go
// AgentState 基础状态接口
type AgentState interface {
    ID() string
    AgentDef() string
    ParentStateID() string
    LoadRecent(ctx context.Context) ([]agentcfg.Message, error)
    StoreMessage(ctx context.Context, msg agentcfg.Message) error
    Revert(ctx context.Context) error
    BuildMemoryContext(ctx context.Context, opts agentcfg.MemoryOptions) (genx.ModelContext, error)
}

// ReActState ReAct 专用状态
type ReActState interface {
    AgentState
    Phase() ReActPhase
    SetPhase(phase ReActPhase)
    Finished() bool
    SetFinished(finished bool)
}

// MatchState Match 专用状态
type MatchState interface {
    AgentState
    Phase() MatchAgentPhase
    SetPhase(phase MatchAgentPhase)
    Input() string
    SetInput(input string)
    Matches() []MatchedIntent
    SetMatches(matches []MatchedIntent)
    CurrentIndex() int
    SetCurrentIndex(index int)
    Matched() bool
    SetMatched(matched bool)
    CallingState() []byte
    SetCallingState(state []byte)
}
```
