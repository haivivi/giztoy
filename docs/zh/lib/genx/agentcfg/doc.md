# GenX Agent Configuration

Configuration parsing and serialization for agents and tools.

> **Note:** This package is Go-only. No Rust implementation exists.

## Design Goals

1. **Declarative Configuration**: Define agents/tools in YAML/JSON
2. **Reference System**: Support `$ref` for reusable components
3. **Validation**: Validate configuration at parse time
4. **Serialization**: Support JSON, YAML, and MessagePack

## Configuration Types

### Agent Types

| Type | Description | Configuration |
|------|-------------|---------------|
| `react` | ReAct pattern agent | `ReActAgent` |
| `match` | Router/matcher agent | `MatchAgent` |

### Tool Types

| Type | Description | Configuration |
|------|-------------|---------------|
| `http` | HTTP API tool | `HTTPTool` |
| `generator` | LLM generation tool | `GeneratorTool` |
| `composite` | Tool pipeline | `CompositeTool` |
| `text_processor` | Text manipulation | `TextProcessorTool` |

## Reference System

The `$ref` system allows reusing components:

```yaml
# Reference an agent
agent:
  $ref: agent:weather_assistant

# Reference a tool
tools:
  - $ref: tool:search
  - $ref: tool:calculator
```

Reference format: `{type}:{name}`

| Type | Description |
|------|-------------|
| `agent:{name}` | Reference to registered agent |
| `tool:{name}` | Reference to registered tool |
| `rule:{name}` | Reference to match rule |
| `prompt:{name}` | Reference to prompt template |

## Configuration Structure

### ReActAgent

```yaml
type: react
name: assistant
prompt: |
  You are a helpful assistant.
generator:
  model: gpt-4
  temperature: 0.7
context_layers:
  - type: env
    vars: ["USER_NAME"]
  - type: mem
    limit: 10
tools:
  - $ref: tool:search
    quit: false
  - $ref: tool:goodbye
    quit: true
```

### MatchAgent

```yaml
type: match
name: router
rules:
  - $ref: rule:weather
  - $ref: rule:music
route:
  - rules: [weather]
    agent:
      $ref: agent:weather_assistant
  - rules: [music]
    agent:
      type: react
      name: music_inline
      prompt: |
        You are a music assistant.
default:
  $ref: agent:chat
```

### HTTPTool

```yaml
type: http
name: weather_api
description: Get weather data
url: https://api.weather.com/v1/current
method: GET
headers:
  Authorization: "Bearer {{.api_key}}"
params:
  - name: city
    in: query
    required: true
  - name: units
    in: query
    default: "metric"
extract: .data.temperature
```

### GeneratorTool

```yaml
type: generator
name: summarize
description: Summarize text
prompt: |
  Summarize the following text in 2-3 sentences:
  {{.text}}
generator:
  model: gpt-3.5-turbo
```

### CompositeTool

```yaml
type: composite
name: search_and_summarize
description: Search and summarize
steps:
  - tool: search
    output_var: results
  - tool: summarize
    input_vars:
      text: results
```

## Validation

Configuration is validated during parsing:

- Required fields checked
- Type consistency verified
- References validated (at runtime)
- Enum values validated

## Related

- Agent framework: [../agent/](../agent/)
- Pattern matching: [../match/](../match/)
