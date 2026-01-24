# GenX - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/genx`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/genx)

## Packages

| Package | Description |
|---------|-------------|
| `genx` | Core types, interfaces, context builder |
| `genx/agent` | Agent framework (ReAct, Match) |
| `genx/agentcfg` | Configuration parsing (YAML/JSON) |
| `genx/match` | Intent matching patterns |
| `genx/generators` | Provider adapters (OpenAI, Gemini) |
| `genx/modelcontexts` | Pre-built contexts |
| `genx/playground` | Interactive testing |

## Core Types

### Generator Interface

```go
type Generator interface {
    GenerateStream(ctx context.Context, model string, mctx ModelContext) (Stream, error)
    Invoke(ctx context.Context, model string, mctx ModelContext, tool *FuncTool) (Usage, *FuncCall, error)
}
```

### ModelContext Interface

```go
type ModelContext interface {
    Prompts() iter.Seq[*Prompt]
    Messages() iter.Seq[*Message]
    CoTs() iter.Seq[string]
    Tools() iter.Seq[Tool]
    Params() *ModelParams
}
```

### Stream Interface

```go
type Stream interface {
    Next() (*MessageChunk, error)
    Close() error
    CloseWithError(error) error
}
```

## ModelContext Builder

```go
builder := genx.NewModelContextBuilder()

// Add prompts
builder.Prompt("system", "You are a helpful assistant.")

// Add messages
builder.UserText("Hello!")
builder.AssistantText("Hi there!")

// Add tools
builder.Tool(&genx.FuncTool{
    Name: "search",
    Description: "Search the web",
    Schema: `{"type":"object","properties":{"query":{"type":"string"}}}`,
})

// Set parameters
builder.Params(&genx.ModelParams{
    Temperature: 0.7,
    MaxTokens: 1000,
})

ctx := builder.Build()
```

## FuncTool

```go
// From schema
tool := &genx.FuncTool{
    Name: "get_weather",
    Description: "Get weather for a city",
    Schema: `{
        "type": "object",
        "properties": {
            "city": {"type": "string"},
            "units": {"type": "string", "enum": ["celsius", "fahrenheit"]}
        },
        "required": ["city"]
    }`,
}

// With executor
tool := genx.NewFuncToolWithExecutor(
    "search",
    "Search the web",
    schema,
    func(ctx context.Context, args json.RawMessage) (string, error) {
        var params SearchParams
        json.Unmarshal(args, &params)
        return doSearch(params.Query), nil
    },
)
```

## Streaming

```go
stream, err := generator.GenerateStream(ctx, "gpt-4", mctx)
if err != nil {
    return err
}
defer stream.Close()

for {
    chunk, err := stream.Next()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    fmt.Print(chunk.Part)
}
```

## Agent Framework

### ReActAgent

```go
import "github.com/haivivi/giztoy/pkg/genx/agent"

ag, err := agent.NewReActAgent(runtime, &agent.ReActConfig{
    Name: "assistant",
    Prompt: "You are a helpful assistant.",
    Generator: &agentcfg.GeneratorConfig{Model: "gpt-4"},
    Tools: []agentcfg.ToolRef{
        {Ref: "tool:search"},
        {Ref: "tool:calculator"},
    },
})
if err != nil {
    return err
}
defer ag.Close()

// Input
ag.Input(genx.Contents{genx.Text("What's 2+2?")})

// Event loop
for {
    evt, err := ag.Next()
    if err != nil {
        return err
    }
    switch evt.Type {
    case agent.EventChunk:
        fmt.Print(evt.Chunk.Part)
    case agent.EventEOF:
        // Waiting for input
        ag.Input(genx.Contents{genx.Text(readline())})
    case agent.EventClosed:
        return nil
    case agent.EventToolStart:
        fmt.Printf("Calling %s...\n", evt.ToolName)
    case agent.EventToolDone:
        fmt.Printf("Tool returned: %s\n", evt.ToolResult)
    case agent.EventToolError:
        fmt.Printf("Tool error: %v\n", evt.ToolError)
    }
}
```

### MatchAgent

```go
ag, err := agent.NewMatchAgent(runtime, &agent.MatchConfig{
    Name: "router",
    Rules: []match.Rule{
        {Name: "weather", Patterns: []string{"天气", "weather"}},
        {Name: "music", Patterns: []string{"播放", "play"}},
    },
    SubAgents: map[string]agentcfg.AgentRef{
        "weather": {Ref: "agent:weather_assistant"},
        "music":   {Ref: "agent:music_player"},
    },
})
```

## Configuration (agentcfg)

### Load from YAML

```go
import "github.com/haivivi/giztoy/pkg/genx/agentcfg"

cfg, err := agentcfg.LoadAgentFromFile("agent.yaml")

// Or from string
cfg, err := agentcfg.ParseAgent(yamlStr)
```

### Agent Config Example

```yaml
type: react
name: assistant
prompt: |
  You are a helpful coding assistant.
generator:
  model: gpt-4
  temperature: 0.7
tools:
  - $ref: tool:search
    quit: false
  - $ref: tool:goodbye
    quit: true
```

### Tool Config Example

```yaml
type: http
name: weather_api
description: Get weather data
url: https://api.weather.com/v1/current
method: GET
params:
  - name: city
    in: query
extract: .data.temperature
```

## Providers

### OpenAI

```go
import "github.com/haivivi/giztoy/pkg/genx/generators"

gen := generators.NewOpenAIGenerator(apiKey,
    generators.WithBaseURL("https://api.openai.com/v1"),
)
```

### Gemini

```go
gen := generators.NewGeminiGenerator(apiKey)
```

## Inspection

```go
// Inspect model context
output, _ := genx.InspectModelContext(mctx)
fmt.Println(output)

// Inspect message
fmt.Println(genx.InspectMessage(msg))

// Inspect tool
fmt.Println(genx.InspectTool(tool))
```
