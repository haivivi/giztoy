# GenX Agent - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/genx/agent`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/genx/agent)

## Agent Interface

```go
type Agent interface {
    Input(contents genx.Contents)
    Next() (*AgentEvent, error)
    Close() error
}
```

## AgentEvent

```go
type AgentEvent struct {
    Type       EventType
    Chunk      *genx.MessageChunk  // EventChunk
    ToolName   string              // EventToolStart/Done/Error
    ToolResult string              // EventToolDone
    ToolError  error               // EventToolError
}

type EventType int

const (
    EventChunk EventType = iota
    EventEOF
    EventClosed
    EventToolStart
    EventToolDone
    EventToolError
    EventInterrupted
)
```

## ReActAgent

```go
ag, err := agent.NewReActAgent(runtime, &agentcfg.ReActAgent{
    AgentBase: agentcfg.AgentBase{
        Name:   "assistant",
        Prompt: "You are a helpful assistant.",
        Generator: agentcfg.GeneratorRef{
            Config: &agentcfg.GeneratorConfig{
                Model: "gpt-4",
            },
        },
    },
    Tools: []agentcfg.ToolRef{
        {Ref: "tool:search"},
        {Ref: "tool:calculator"},
    },
})
if err != nil {
    return err
}
defer ag.Close()
```

## MatchAgent

```go
ag, err := agent.NewMatchAgent(runtime, &agentcfg.MatchAgent{
    AgentBase: agentcfg.AgentBase{
        Name: "router",
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
})
```

## Event Loop

```go
// Provide initial input
ag.Input(genx.Contents{genx.Text("Hello!")})

// Process events
for {
    evt, err := ag.Next()
    if err != nil {
        return err
    }
    
    switch evt.Type {
    case agent.EventChunk:
        // Output chunk
        fmt.Print(evt.Chunk.Part)
        
    case agent.EventEOF:
        // Round ended, provide new input
        fmt.Print("\n> ")
        input := readline()
        ag.Input(genx.Contents{genx.Text(input)})
        
    case agent.EventClosed:
        // Agent completed (quit tool or closed)
        return nil
        
    case agent.EventToolStart:
        fmt.Printf("[Calling %s...]\n", evt.ToolName)
        
    case agent.EventToolDone:
        fmt.Printf("[%s returned: %s]\n", evt.ToolName, evt.ToolResult)
        
    case agent.EventToolError:
        fmt.Printf("[%s error: %v]\n", evt.ToolName, evt.ToolError)
        
    case agent.EventInterrupted:
        return nil
    }
}
```

## Runtime Interface

```go
type Runtime interface {
    // Generator returns LLM generator for the given config
    Generator(cfg *agentcfg.GeneratorConfig) (genx.Generator, error)
    
    // Tool creates or retrieves a tool by reference
    Tool(ref string) (Tool, error)
    
    // Agent creates or retrieves an agent by reference
    Agent(ref string) (Agent, error)
    
    // State returns state manager for memory
    State() State
}
```

## Tool Interface

```go
type Tool interface {
    genx.FuncTool
    
    // Execute runs the tool with given arguments
    Execute(ctx context.Context, args json.RawMessage) (string, error)
}
```

## Built-in Tools

### GeneratorTool

LLM-based text/JSON generation:

```go
tool := &agent.GeneratorTool{
    Name:        "summarize",
    Description: "Summarize text",
    Prompt:      "Summarize the following:\n{{.text}}",
    Generator:   generatorConfig,
}
```

### HTTPTool

HTTP requests with jq extraction:

```go
tool := &agent.HTTPTool{
    Name:        "weather",
    Description: "Get weather data",
    URL:         "https://api.weather.com/v1/current",
    Method:      "GET",
    Params: []agent.HTTPParam{
        {Name: "city", In: "query"},
    },
    Extract: ".data.temperature",
}
```

### CompositeTool

Sequential tool pipeline:

```go
tool := &agent.CompositeTool{
    Name:        "search_and_summarize",
    Description: "Search and summarize results",
    Steps: []agent.CompositeStep{
        {Tool: "search", OutputVar: "results"},
        {Tool: "summarize", InputVars: map[string]string{"text": "results"}},
    },
}
```

### TextProcessorTool

Text manipulation:

```go
tool := &agent.TextProcessorTool{
    Name:        "extract_json",
    Description: "Extract JSON from text",
    Prompt:      "Extract JSON from: {{.input}}",
    OutputType:  "json",
}
```

## State Management

```go
type State interface {
    // Get retrieves value by key
    Get(key string) (any, bool)
    
    // Set stores value by key
    Set(key string, value any)
    
    // Memory returns conversation memory
    Memory() []genx.Message
    
    // AppendMemory adds message to memory
    AppendMemory(msg genx.Message)
}
```
