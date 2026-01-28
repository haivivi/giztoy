package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/agentcfg"
)

// GeneratorTool creates single-round LLM generation tools.
//
// # Overview
//
// GeneratorTool wraps an LLM call as a tool. When invoked, it takes user input,
// combines it with configured prompts, and generates output. It supports two modes:
//   - generate: Streaming text generation, returns plain text
//   - json_output: Structured JSON output with schema validation
//
// # Definition
//
// Simple form with inline prompt:
//
//	tools:
//	  - type: generator
//	    name: fortune_teller
//	    description: "Tell fortune based on user's birth info"
//	    model: qwen-turbo
//	    mode: generate
//	    prompt: |
//	      You are a fortune teller master.
//	      Analyze the user's info and predict their future.
//
// With context_layers for multi-source prompts:
//
//	tools:
//	  - type: generator
//	    name: story_writer
//	    description: "Generate a story"
//	    model: gpt-4o
//	    mode: json_output
//	    context_layers:
//	      - $ref: "character:storyteller"
//	      - $env: "EXTRA_PROMPT"
//	      - "inline prompt text"
//	    output_schema:
//	      type: object
//	      properties:
//	        title: { type: string }
//	        content: { type: string }
//
// # Context Layers
//
// context_layers is an array of prompt sources. Each layer can be one of:
//
//	┌─────────────┬────────────────────────────────────────────────────────┐
//	│ Syntax      │ Description                                            │
//	├─────────────┼────────────────────────────────────────────────────────┤
//	│ "text"      │ Inline prompt text (string form)                       │
//	│ $ref        │ Reference to external resource (e.g. "character:elsa") │
//	│ $env        │ Read prompt from environment variable                  │
//	│ $this       │ Reference to current tool's field (e.g. ".description")│
//	│ $mem        │ Inject memory context (summary, query, recent)         │
//	└─────────────┴────────────────────────────────────────────────────────┘
//
// Examples:
//
//	context_layers:
//	  # String form - inline prompt text
//	  - "You are a helpful assistant."
//
//	  # $ref - reference external resource by key
//	  - $ref: "character:elsa"
//
//	  # $env - read from environment variable
//	  - $env: "SYSTEM_PROMPT"
//
//	  # $this - reference current tool's field
//	  - $this: ".description"
//
//	  # $mem - inject memory (for agents with state)
//	  - $mem:
//	      summary: true   # include conversation summary
//	      recent: 10      # include last 10 messages
//
// # Input
//
// GeneratorTool accepts a single input field:
//
//	{ "input": "user's input text" }
//
// The input is appended as a user message after the configured prompts.
type GeneratorTool struct {
	rt Runtime
}

// NewGeneratorTool creates a GeneratorTool instance.
func NewGeneratorTool(rt Runtime) *GeneratorTool {
	return &GeneratorTool{rt: rt}
}

// CreateFuncTool creates a genx.FuncTool from agentcfg.GeneratorTool.
func (t *GeneratorTool) CreateFuncTool(ctx context.Context, def *agentcfg.GeneratorTool) (*genx.FuncTool, error) {
	if def.Model == "" {
		return nil, fmt.Errorf("tool %s: model is required", def.Name)
	}

	if def.Mode == "" {
		return nil, fmt.Errorf("tool %s: mode is required (generate or json_output)", def.Name)
	}

	// Build base model context from prompt/context_layers
	baseMCtx, err := t.buildModelContext(ctx, def)
	if err != nil {
		return nil, fmt.Errorf("tool %s: build context: %w", def.Name, err)
	}

	// Create FuncTool
	type generatorArgs struct {
		Input string `json:"input" description:"User input text"`
	}
	// Extract schema from wrapper
	var outputSchema *jsonschema.Schema
	if def.OutputSchema != nil {
		outputSchema = def.OutputSchema.Schema
	}
	tool, err := genx.NewFuncTool[generatorArgs](
		def.Name,
		def.Description,
		genx.InvokeFunc[generatorArgs](func(ctx context.Context, call *genx.FuncCall, args generatorArgs) (any, error) {
			return t.execute(ctx, baseMCtx, def.Model, def.Mode, outputSchema, args.Input)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("tool %s: %w", def.Name, err)
	}

	return tool, nil
}

// Execute executes the generator tool with a agentcfg.GeneratorTool and input.
// Used by result processor and other internal callers.
func (t *GeneratorTool) Execute(ctx context.Context, def *agentcfg.GeneratorTool, input any) (string, error) {
	if def.Model == "" {
		return "", fmt.Errorf("generator: model is required")
	}

	// Build base model context
	baseMCtx, err := t.buildModelContext(ctx, def)
	if err != nil {
		return "", fmt.Errorf("build context: %w", err)
	}

	// Convert input to string
	var inputStr string
	switch v := input.(type) {
	case string:
		inputStr = v
	case agentcfg.TextProcessorInput:
		inputStr = v.Content
	default:
		inputBytes, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("marshal input: %w", err)
		}
		inputStr = string(inputBytes)
	}

	// Execute in generate mode (result processor always returns text)
	result, err := t.execute(ctx, baseMCtx, def.Model, "generate", nil, inputStr)
	if err != nil {
		return "", err
	}
	if s, ok := result.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", result), nil
}

// buildModelContext builds ModelContext from generator config.
func (t *GeneratorTool) buildModelContext(ctx context.Context, def *agentcfg.GeneratorTool) (genx.ModelContext, error) {
	mcb := &genx.ModelContextBuilder{}

	// Simple prompt form
	if def.Prompt != "" {
		mcb.PromptText("", def.Prompt)
	}

	// Context layers (like AgentDef)
	for _, layer := range def.ContextLayers {
		// Inline prompt text
		if layer.Prompt != "" {
			mcb.Prompts = append(mcb.Prompts, &genx.Prompt{Name: "inline", Text: layer.Prompt})
			continue
		}

		// $ref - reference to external resource
		if layer.Ref != "" {
			cb, err := t.rt.GetContextBuilder(ctx, layer.Ref)
			if err != nil {
				return nil, err
			}
			mctx := cb.BuildContext(ctx)
			for p := range mctx.Prompts() {
				mcb.Prompts = append(mcb.Prompts, p)
			}
			continue
		}

		// $env - environment variable
		if layer.Env != "" {
			text := os.Getenv(layer.Env)
			if text != "" {
				mcb.Prompts = append(mcb.Prompts, &genx.Prompt{Name: layer.Env, Text: text})
			}
			continue
		}

		// $this - reference to current tool's field
		if layer.This != "" {
			text, err := resolveGeneratorField(def, layer.This)
			if err != nil {
				return nil, err
			}
			if text != "" {
				mcb.Prompts = append(mcb.Prompts, &genx.Prompt{Name: layer.This, Text: text})
			}
			continue
		}
	}

	return mcb.Build(), nil
}

// resolveGeneratorField resolves a field reference in agentcfg.GeneratorTool.
func resolveGeneratorField(def *agentcfg.GeneratorTool, field string) (string, error) {
	// Remove leading "."
	if len(field) > 0 && field[0] == '.' {
		field = field[1:]
	}
	switch field {
	case "prompt":
		return def.Prompt, nil
	case "name":
		return def.Name, nil
	case "description":
		return def.Description, nil
	case "model":
		return def.Model, nil
	case "mode":
		return string(def.Mode), nil
	default:
		return "", nil
	}
}

// BuildModelContextWithInput builds ModelContext from generator config with user input.
// This is a public method for use by text processor.
func (t *GeneratorTool) BuildModelContextWithInput(ctx context.Context, def *agentcfg.GeneratorTool, input string) (genx.ModelContext, error) {
	baseMCtx, err := t.buildModelContext(ctx, def)
	if err != nil {
		return nil, err
	}

	// Build full context with user input
	mcb := &genx.ModelContextBuilder{}

	// Add base prompts
	for p := range baseMCtx.Prompts() {
		mcb.Prompts = append(mcb.Prompts, p)
	}

	// Add user input
	mcb.UserText("", input)

	return mcb.Build(), nil
}

// execute executes the generator.
func (t *GeneratorTool) execute(ctx context.Context, baseMCtx genx.ModelContext, model string, mode agentcfg.GeneratorMode, outputSchema *jsonschema.Schema, input string) (any, error) {
	// Build full context with user input
	mcb := &genx.ModelContextBuilder{}

	// Add base prompts
	for p := range baseMCtx.Prompts() {
		mcb.Prompts = append(mcb.Prompts, p)
	}

	// Add user input
	mcb.UserText("", input)

	mctx := mcb.Build()

	switch mode {
	case agentcfg.GeneratorModeGenerate:
		return t.executeGenerate(ctx, model, mctx)
	case agentcfg.GeneratorModeJSONOutput:
		return t.executeJSONOutput(ctx, model, mctx, outputSchema)
	default:
		return nil, fmt.Errorf("unknown generator mode: %s", mode)
	}
}

// executeGenerate executes in generate mode (streaming text).
func (t *GeneratorTool) executeGenerate(ctx context.Context, model string, mctx genx.ModelContext) (string, error) {
	stream, err := t.rt.GenerateStream(ctx, model, mctx)
	if err != nil {
		return "", fmt.Errorf("generate stream: %w", err)
	}
	defer stream.Close()

	var sb strings.Builder
	for {
		chunk, err := stream.Next()
		if err != nil {
			if errors.Is(err, genx.ErrDone) {
				break
			}
			return "", fmt.Errorf("stream next: %w", err)
		}
		if chunk != nil && chunk.Part != nil {
			if text, ok := chunk.Part.(genx.Text); ok {
				sb.WriteString(string(text))
			}
		}
	}

	return sb.String(), nil
}

// executeJSONOutput executes in json_output mode (structured output).
func (t *GeneratorTool) executeJSONOutput(ctx context.Context, model string, mctx genx.ModelContext, outputSchema *jsonschema.Schema) (any, error) {
	// Create a FuncTool with the output schema
	tool := &genx.FuncTool{
		Name:        "output",
		Description: "Output structured result",
		Argument:    outputSchema,
		Invoke: func(ctx context.Context, call *genx.FuncCall, arg string) (any, error) {
			return arg, nil
		},
	}

	// Use Invoke to get structured output
	_, funcCall, err := t.rt.Invoke(ctx, model, mctx, tool)
	if err != nil {
		return nil, fmt.Errorf("invoke: %w", err)
	}

	if funcCall == nil {
		return nil, fmt.Errorf("no function call returned")
	}

	// Return the arguments as JSON result
	return funcCall.Arguments, nil
}
