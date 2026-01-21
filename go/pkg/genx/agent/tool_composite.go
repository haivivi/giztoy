package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/haivivi/giztoy/pkg/genx"
	"github.com/haivivi/giztoy/pkg/genx/agentcfg"
)

// CompositeTool creates composite tools that execute multiple tools in sequence.
//
// # Overview
//
// CompositeTool chains multiple tools together, passing output from one step
// as input to the next. It inherits the parameter schema from the first step's
// tool, allowing LLM to call it with the same arguments.
//
// # Definition
//
// Basic example with two HTTP calls:
//
//	tools:
//	  - type: composite
//	    name: get_weather_and_news
//	    description: "Get weather then fetch related news"
//	    steps:
//	      - id: weather
//	        tool:
//	          $ref: "get_weather"
//	      - id: news
//	        tool:
//	          $ref: "search_news"
//	        input_jq: '{"query": .steps.weather.condition}'
//
// # Steps
//
// Each step has:
//   - id: Unique identifier for referencing in subsequent steps
//   - tool: Either $ref to external tool or inline tool definition
//   - input_jq: (optional) JQ expression to transform input for this step
//
// # Input/Output Flow
//
//	┌─────────────────────────────────────────────────────────────────┐
//	│  Step 1 (first step)                                            │
//	│  - Receives raw arguments from LLM call directly                │
//	│  - input_jq is ignored (args come from LLM)                     │
//	│  - Output stored as: .steps.<id>                                │
//	├─────────────────────────────────────────────────────────────────┤
//	│  Step 2+ (subsequent steps)                                     │
//	│  - If input_jq specified: apply JQ to context                   │
//	│  - If no input_jq: use previous step's output as-is             │
//	│  - Output stored as: .steps.<id>                                │
//	└─────────────────────────────────────────────────────────────────┘
//
// # JQ Context
//
// The input_jq expression has access to:
//
//	{
//	  "input": <original LLM arguments>,
//	  "steps": {
//	    "<step_id>": <step output>,
//	    ...
//	  }
//	}
//
// # Examples
//
// Chain with data transformation:
//
//	steps:
//	  - id: search
//	    tool:
//	      $ref: "web_search"
//	  - id: summarize
//	    tool:
//	      type: generator
//	      model: gpt-4o
//	      mode: generate
//	      prompt: "Summarize the following search results"
//	    input_jq: '{"input": .steps.search.results | join("\n")}'
//
// Using previous output directly (no input_jq):
//
//	steps:
//	  - id: translate
//	    tool:
//	      $ref: "translate_to_english"
//	  - id: sentiment
//	    tool:
//	      $ref: "analyze_sentiment"
//	    # No input_jq: uses translate output directly
type CompositeTool struct {
	rt Runtime
}

// NewCompositeTool creates a CompositeTool instance.
func NewCompositeTool(rt Runtime) *CompositeTool {
	return &CompositeTool{rt: rt}
}

// compositeContext holds the execution context for composite tool steps.
// It is used as the input for jq expressions.
type compositeContext struct {
	Input any            // original input (parsed as JSON if possible)
	Steps map[string]any // step id -> output (parsed as JSON if possible)
}

// toMap converts the context to a map for jq processing.
func (c *compositeContext) toMap() map[string]any {
	return map[string]any{
		"input": c.Input,
		"steps": c.Steps,
	}
}

// parseJSONOrString tries to parse a string as JSON, returns original if not JSON.
func parseJSONOrString(s string) any {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s // Return as string if not valid JSON
	}
	return v
}

// CreateFuncTool creates a genx.FuncTool from agentcfg.CompositeTool.
// The composite tool inherits its parameters from the first step's tool,
// allowing LLM to call it with the same arguments as the first tool.
func (t *CompositeTool) CreateFuncTool(ctx context.Context, def *agentcfg.CompositeTool) (*genx.FuncTool, error) {
	if len(def.Steps) == 0 {
		return nil, fmt.Errorf("tool %s: at least one step is required", def.Name)
	}

	// Validate steps have IDs and tools
	for i, step := range def.Steps {
		if step.ID == "" {
			return nil, fmt.Errorf("tool %s: step %d missing id", def.Name, i)
		}
		if step.Tool.Ref == "" && step.Tool.Tool == nil {
			return nil, fmt.Errorf("tool %s: step %s must have tool.$ref or inline tool", def.Name, step.ID)
		}
	}

	// Get/create the first step's tool to inherit its schema
	firstStep := def.Steps[0]
	firstTool, err := t.getOrCreateTool(ctx, &firstStep.Tool)
	if err != nil {
		return nil, fmt.Errorf("tool %s: first step tool: %w", def.Name, err)
	}

	// Create a wrapper FuncTool that:
	// - Uses the first tool's argument schema
	// - Passes raw arguments to first step, then chains through subsequent steps
	tool := &genx.FuncTool{
		Name:        def.Name,
		Description: def.Description,
		Argument:    firstTool.Argument, // Inherit schema from first tool
		Invoke: func(ctx context.Context, call *genx.FuncCall, rawArgs string) (any, error) {
			return t.execute(ctx, def, firstTool, rawArgs)
		},
	}

	return tool, nil
}

// getOrCreateTool gets a tool by reference or creates an inline tool.
func (t *CompositeTool) getOrCreateTool(ctx context.Context, toolRef *agentcfg.ToolRef) (*genx.FuncTool, error) {
	if toolRef.IsRef() {
		return t.rt.GetTool(ctx, toolRef.Ref)
	}
	if toolRef.Tool != nil {
		return t.createInlineTool(ctx, toolRef.Tool)
	}
	return nil, fmt.Errorf("tool ref has neither $ref nor inline definition")
}

// execute executes the composite tool steps in sequence.
// The first step receives the raw arguments directly (same as LLM would call it).
// Subsequent steps can use input_jq to extract/transform data from context.
func (t *CompositeTool) execute(ctx context.Context, def *agentcfg.CompositeTool, firstTool *genx.FuncTool, rawArgs string) (any, error) {
	compCtx := &compositeContext{
		Input: parseJSONOrString(rawArgs), // Parse raw args as JSON if possible
		Steps: make(map[string]any),
	}

	var lastOutput any
	var lastOutputStr string

	for i, step := range def.Steps {
		var stepTool *genx.FuncTool
		var stepArgs string
		var err error

		if i == 0 {
			// First step: use the pre-loaded first tool with raw args from LLM
			// Note: InputJQ is ignored for first step since args come from LLM tool call
			stepTool = firstTool
			stepArgs = rawArgs
		} else {
			// Subsequent steps: get tool and resolve input
			stepTool, err = t.getOrCreateTool(ctx, &step.Tool)
			if err != nil {
				return nil, fmt.Errorf("step %s: get tool: %w", step.ID, err)
			}

			// If input_jq is specified, apply it to extract arguments
			if step.InputJQ != nil {
				stepArgs, err = step.InputJQ.Run(compCtx.toMap())
				if err != nil {
					return nil, fmt.Errorf("step %s: apply jq: %w", step.ID, err)
				}
			} else {
				// Default: use previous step's output as-is
				stepArgs = lastOutputStr
			}
		}

		// Execute step
		funcCall := stepTool.NewFuncCall(stepArgs)
		result, err := funcCall.Invoke(ctx)
		if err != nil {
			return nil, fmt.Errorf("step %s: invoke: %w", step.ID, err)
		}

		// Store result in context for subsequent steps
		lastOutputStr = formatOutput(result)
		compCtx.Steps[step.ID] = parseJSONOrString(lastOutputStr)
		lastOutput = result
	}

	return lastOutput, nil
}

// createInlineTool creates a FuncTool from an inline agentcfg.Tool.
func (t *CompositeTool) createInlineTool(ctx context.Context, def agentcfg.Tool) (*genx.FuncTool, error) {
	switch d := def.(type) {
	case *agentcfg.GeneratorTool:
		gt := NewGeneratorTool(t.rt)
		return gt.CreateFuncTool(ctx, d)
	case *agentcfg.HTTPTool:
		ht := NewHTTPTool(t.rt, nil)
		return ht.CreateFuncTool(d)
	default:
		return nil, fmt.Errorf("unsupported inline tool type: %T", def)
	}
}

// formatOutput converts result to string for context storage.
func formatOutput(result any) string {
	switch v := result.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("[marshal error: %v]", err)
		}
		return string(data)
	}
}
