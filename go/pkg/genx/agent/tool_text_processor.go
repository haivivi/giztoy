package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/agentcfg"
)

// TextProcessorTool processes text content using LLM.
//
// # Overview
//
// TextProcessorTool takes text content and processes it using an LLM.
// It supports two output modes:
//   - text: Streaming text generation, returns plain text
//   - json: Structured JSON output with schema validation
//
// # Definition
//
// Define a text_processor tool in YAML:
//
//	tools:
//	  - type: text_processor
//	    name: summarizer
//	    description: "Summarize conversation history"
//	    model: gemini-1.5-flash
//	    prompt: "Summarize the following content concisely:"
//	    output_mode: text
//
// # Usage
//
// Can be referenced via $ref in AgentToolDef.result_processor:
//
//	result_processor:
//	  $ref: tool:summarizer
type TextProcessorTool struct {
	rt Runtime
}

// NewTextProcessorTool creates a TextProcessorTool instance.
func NewTextProcessorTool(rt Runtime) *TextProcessorTool {
	return &TextProcessorTool{rt: rt}
}

// Execute processes the content using the text processor definition.
// Returns the processed result as a string.
func (t *TextProcessorTool) Execute(ctx context.Context, ref *agentcfg.TextProcessorToolRef, content string) (string, error) {
	if ref == nil || ref.IsEmpty() {
		return content, nil
	}

	// Reference to existing text processor tool - invoke directly
	if ref.Ref != "" {
		return t.executeByRef(ctx, ref.Ref, content)
	}

	// Inline text processor definition
	return t.executeInline(ctx, ref.TextProcessorTool, content)
}

// executeByRef executes a text processor by reference.
func (t *TextProcessorTool) executeByRef(ctx context.Context, ref string, content string) (string, error) {
	tool, err := t.rt.GetTool(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("get text processor tool %s: %w", ref, err)
	}
	return t.invokeToolWithInput(ctx, tool, content)
}

// executeInline executes an inline text processor definition.
func (t *TextProcessorTool) executeInline(ctx context.Context, def *agentcfg.TextProcessorTool, content string) (string, error) {
	if def == nil {
		return content, nil
	}

	// Build model context with prompt and input content
	mcb := &genx.ModelContextBuilder{}
	mcb.PromptText("system", def.Prompt)
	mcb.UserText("", content)
	mctx := mcb.Build()

	// Execute based on output mode
	outputMode := def.OutputMode
	if outputMode == "" {
		outputMode = agentcfg.TextProcessorOutputText
	}

	switch outputMode {
	case agentcfg.TextProcessorOutputText:
		return t.executeText(ctx, def.Model, mctx)
	case agentcfg.TextProcessorOutputJSON:
		var schema *jsonschema.Schema
		if def.OutputSchema != nil {
			schema = def.OutputSchema.Schema
		}
		return t.executeJSON(ctx, def.Model, mctx, schema)
	default:
		return "", fmt.Errorf("unknown output mode: %s", outputMode)
	}
}

// executeText executes in text mode using GenerateStream.
func (t *TextProcessorTool) executeText(ctx context.Context, model string, mctx genx.ModelContext) (string, error) {
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

// executeJSON executes in JSON mode using Invoke with schema.
func (t *TextProcessorTool) executeJSON(ctx context.Context, model string, mctx genx.ModelContext, outputSchema *jsonschema.Schema) (string, error) {
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
		return "", fmt.Errorf("invoke: %w", err)
	}

	if funcCall == nil {
		return "", fmt.Errorf("no function call returned")
	}

	// Return the arguments as JSON result
	return funcCall.Arguments, nil
}

// invokeToolWithInput invokes a tool with agentcfg.TextProcessorInput and returns string result.
func (t *TextProcessorTool) invokeToolWithInput(ctx context.Context, tool *genx.FuncTool, content string) (string, error) {
	input := agentcfg.TextProcessorInput{Content: content}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal input: %w", err)
	}

	funcCall := tool.NewFuncCall(string(inputJSON))
	result, err := funcCall.Invoke(ctx)
	if err != nil {
		return "", fmt.Errorf("invoke: %w", err)
	}

	// Convert result to string using formatOutput for consistent serialization
	return formatOutput(result), nil
}

// CreateFuncTool creates a genx.FuncTool from agentcfg.TextProcessorTool.
// This allows TextProcessorTool to be used as a standalone tool in the tool registry.
func (t *TextProcessorTool) CreateFuncTool(def *agentcfg.TextProcessorTool) (*genx.FuncTool, error) {
	tool, err := genx.NewFuncTool[agentcfg.TextProcessorInput](
		def.Name,
		def.Description,
		genx.InvokeFunc[agentcfg.TextProcessorInput](func(ctx context.Context, call *genx.FuncCall, input agentcfg.TextProcessorInput) (any, error) {
			ref := &agentcfg.TextProcessorToolRef{TextProcessorTool: def}
			return t.Execute(ctx, ref, input.Content)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("tool %s: %w", def.Name, err)
	}
	return tool, nil
}
