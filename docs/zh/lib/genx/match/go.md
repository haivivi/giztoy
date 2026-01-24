# GenX Match - Go Implementation

Import: `github.com/haivivi/giztoy/pkg/genx/match`

📚 [Go Documentation](https://pkg.go.dev/github.com/haivivi/giztoy/pkg/genx/match)

## Rule Definition

### Rule Struct

```go
type Rule struct {
    Name     string         `yaml:"name"`
    Vars     map[string]Var `yaml:"vars,omitempty"`
    Patterns []Pattern      `yaml:"patterns"`
    Examples []Example      `yaml:"examples,omitempty"`
}

type Var struct {
    Label string `yaml:"label"`
    Type  string `yaml:"type"`  // "string", "int", "float", "bool"
}

type Pattern struct {
    Input  string
    Output string
}

type Example struct {
    Input  string
    Output string
}
```

### Creating Rules

```go
// Programmatically
rule := &match.Rule{
    Name: "music",
    Vars: map[string]match.Var{
        "title":  {Label: "歌曲名", Type: "string"},
        "artist": {Label: "歌手", Type: "string"},
    },
    Patterns: []match.Pattern{
        {Input: "播放歌曲"},
        {Input: "我想听[title]", Output: "title=[歌曲名]"},
        {Input: "我想听[artist]的[title]", Output: "artist=[歌手], title=[歌曲名]"},
    },
}

// From YAML
var rule match.Rule
err := yaml.Unmarshal(data, &rule)
```

## Compilation

```go
rules := []*match.Rule{weatherRule, musicRule, chatRule}

matcher, err := match.Compile(rules)
if err != nil {
    return err
}

// Optional: custom prompt template
matcher, err := match.Compile(rules, match.WithTpl(customTemplate))
```

## Matching

### Basic Match

```go
// Build model context with user input
mcb := &genx.ModelContextBuilder{}
mcb.UserText("user", "我想听周杰伦的稻香")
mctx := mcb.Build()

// Match against rules
for result, err := range matcher.Match(ctx, "gpt-4", mctx) {
    if err != nil {
        return err
    }
    
    fmt.Printf("Rule: %s\n", result.Rule)
    for name, arg := range result.Args {
        if arg.HasValue {
            fmt.Printf("  %s = %v\n", name, arg.Value)
        }
    }
}
```

### With Custom Generator

```go
gen := generators.NewOpenAIGenerator(apiKey)

for result, err := range matcher.Match(ctx, "gpt-4", mctx, 
    match.WithGenerator(gen)) {
    // ...
}
```

### Collect All Results

```go
results, err := match.Collect(matcher.Match(ctx, "gpt-4", mctx))
if err != nil {
    return err
}
for _, r := range results {
    fmt.Println(r.Rule)
}
```

## Result Structure

```go
type Result struct {
    // Rule is the matched rule name. Empty if no rule matched.
    Rule string
    
    // Args holds the extracted arguments, keyed by variable name.
    Args map[string]Arg
    
    // RawText holds the original line when no rule matched.
    RawText string
}

type Arg struct {
    // Value is the extracted value, typed according to Var.Type.
    Value any
    
    // Var is the variable definition from the rule.
    Var Var
    
    // HasValue indicates whether a value was successfully extracted.
    HasValue bool
}
```

## Processing Results

```go
for result, err := range matcher.Match(ctx, model, mctx) {
    if err != nil {
        return err
    }
    
    switch result.Rule {
    case "weather":
        handleWeather()
        
    case "music":
        title := ""
        artist := ""
        if arg, ok := result.Args["title"]; ok && arg.HasValue {
            title = arg.Value.(string)
        }
        if arg, ok := result.Args["artist"]; ok && arg.HasValue {
            artist = arg.Value.(string)
        }
        handleMusic(artist, title)
        
    case "":
        // No rule matched
        if result.RawText != "" {
            handleUnknown(result.RawText)
        }
    }
}
```

## YAML Rule Format

```yaml
# rule.yaml
name: music
vars:
  title:
    label: 歌曲名
    type: string
  artist:
    label: 歌手
    type: string
patterns:
  # Simple patterns (no variables)
  - 播放歌曲
  - 我想听歌
  
  # Patterns with variables (array format)
  - ["我想听[title]", "title=[歌曲名]"]
  - ["我想听[artist]的歌", "artist=[歌手]"]
  - ["我想听[artist]的[title]", "artist=[歌手], title=[歌曲名]"]

examples:
  - input: "我想听周杰伦的稻香"
    output: "music: artist=周杰伦, title=稻香"
  - input: "来首歌"
    output: "music"
```

## Debugging

```go
// Get the compiled system prompt
prompt := matcher.SystemPrompt()
fmt.Println(prompt)
```

## Custom Prompt Template

```go
customTpl := `
你是一个意图识别助手。
{{range .Rules}}
## {{.Name}}
{{range .Patterns}}
- {{.Input}} → {{.Output}}
{{end}}
{{end}}
`

matcher, err := match.Compile(rules, match.WithTpl(customTpl))
```
