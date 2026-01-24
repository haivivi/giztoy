# GenX Agent Configuration - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/genx/agentcfg`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/genx/agentcfg)

## Agent Types

### Agent Interface

```go
type Agent interface {
    AgentName() string
    AgentType() AgentType
}
```

### AgentBase

```go
type AgentBase struct {
    Type          AgentType      `json:"type,omitzero"`
    Name          string         `json:"name"`
    Prompt        string         `json:"prompt,omitzero"`
    ContextLayers []ContextLayer `json:"context_layers,omitzero"`
    Generator     GeneratorRef   `json:"generator,omitzero"`
}
```

### ReActAgent

```go
type ReActAgent struct {
    AgentBase
    Tools []ToolRef `json:"tools,omitzero"`
}
```

### MatchAgent

```go
type MatchAgent struct {
    AgentBase
    Rules   []RuleRef    `json:"rules,omitzero"`
    Route   []MatchRoute `json:"route,omitzero"`
    Default *AgentRef    `json:"default,omitzero"`
}
```

## Tool Types

### ToolRef

```go
type ToolRef struct {
    Ref  string `json:"$ref,omitzero"`
    Quit bool   `json:"quit,omitzero"`
    Tool Tool   `json:"-"`  // Inline definition
}
```

### HTTPTool

```go
type HTTPTool struct {
    ToolBase
    URL       string         `json:"url"`
    Method    string         `json:"method,omitzero"`
    Headers   map[string]string `json:"headers,omitzero"`
    Params    []HTTPParam    `json:"params,omitzero"`
    Body      string         `json:"body,omitzero"`
    Extract   string         `json:"extract,omitzero"`  // jq expression
}

type HTTPParam struct {
    Name     string `json:"name"`
    In       string `json:"in"`  // "query", "path", "header"
    Required bool   `json:"required,omitzero"`
    Default  string `json:"default,omitzero"`
}
```

### GeneratorTool

```go
type GeneratorTool struct {
    ToolBase
    Prompt    string        `json:"prompt"`
    Generator GeneratorRef  `json:"generator"`
    Schema    string        `json:"schema,omitzero"`  // JSON Schema for output
}
```

### CompositeTool

```go
type CompositeTool struct {
    ToolBase
    Steps []CompositeStep `json:"steps"`
}

type CompositeStep struct {
    Tool      string            `json:"tool"`
    InputVars map[string]string `json:"input_vars,omitzero"`
    OutputVar string            `json:"output_var,omitzero"`
}
```

### TextProcessorTool

```go
type TextProcessorTool struct {
    ToolBase
    Prompt     string       `json:"prompt"`
    Generator  GeneratorRef `json:"generator"`
    OutputType string       `json:"output_type,omitzero"`  // "text", "json"
}
```

## Reference Types

### AgentRef

```go
type AgentRef struct {
    Ref   string `json:"$ref,omitzero"`
    Agent Agent  `json:"-"`  // Inline definition
}

func (a *AgentRef) IsRef() bool
func (a *AgentRef) IsEmpty() bool
```

### GeneratorRef

```go
type GeneratorRef struct {
    Ref    string           `json:"$ref,omitzero"`
    Config *GeneratorConfig `json:"-"`
}

type GeneratorConfig struct {
    Model       string  `json:"model"`
    Temperature float32 `json:"temperature,omitzero"`
    MaxTokens   int     `json:"max_tokens,omitzero"`
}
```

### RuleRef

```go
type RuleRef struct {
    Ref  string      `json:"$ref,omitzero"`
    Rule *match.Rule `json:"-"`
}
```

## Context Layers

```go
type ContextLayer interface {
    LayerType() ContextLayerType
}

type EnvContextLayer struct {
    Vars []string `json:"vars"`
}

type MemContextLayer struct {
    Limit int `json:"limit"`
}

type PromptContextLayer struct {
    Ref  string `json:"$ref,omitzero"`
    Text string `json:"text,omitzero"`
}
```

## Parsing

### Parse Agent

```go
// From JSON/YAML bytes
agent, err := agentcfg.UnmarshalAgent(data)

// Type assertion
if react := agentcfg.AsReActAgent(agent); react != nil {
    // Handle ReActAgent
}
if match := agentcfg.AsMatchAgent(agent); match != nil {
    // Handle MatchAgent
}
```

### Parse Tool

```go
tool, err := agentcfg.UnmarshalTool(data)
```

## Serialization

Supports JSON, YAML, and MessagePack:

```go
// JSON
data, err := json.Marshal(agent)
err = json.Unmarshal(data, &agent)

// YAML
data, err := yaml.Marshal(agent)
err = yaml.Unmarshal(data, &agent)

// MessagePack
data, err := msgpack.Marshal(agent)
err = msgpack.Unmarshal(data, &agent)
```

## Validation

Validation happens during unmarshal:

```go
// This will return error if validation fails
err := json.Unmarshal(data, &agent)
// err: "react agent: name is required"
```

Validation rules:
- Agent name is required
- Tool name and description required
- HTTPTool URL required
- GeneratorTool prompt and model required
- CompositeTool must have steps
