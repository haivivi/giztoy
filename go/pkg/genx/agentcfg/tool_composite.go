package agentcfg

import (
	"encoding/json"
	"fmt"
)

// CompositeTool is a composite tool that executes multiple tools in sequence.
// Like GitHub Actions, each step's output is stored in a context for later steps to reference.
//
// Context reference syntax:
//   - .input - original user input
//   - .steps.<id> - output from step with given id
//
// Validation:
//   - Inherits ToolBase validation (Name required)
//   - Mode: validated via CompositeMode unmarshal
//   - Steps: required, non-empty array
//   - Each step ID must be unique within the tool
type CompositeTool struct {
	ToolBase `msgpack:",inline"`
	// Mode: "seq" (sequential execution, default)
	Mode CompositeMode `json:"mode,omitzero" msgpack:"mode,omitempty"`
	// Steps to execute in order
	Steps []CompositeStep `json:"steps" msgpack:"steps"`
}

// validate checks if the CompositeTool fields are valid.
func (t *CompositeTool) validate() error {
	if t.Name == "" {
		return fmt.Errorf("composite tool: name is required")
	}
	if len(t.Steps) == 0 {
		return fmt.Errorf("tool %s: steps is required", t.Name)
	}
	// Check for duplicate step IDs
	seen := make(map[string]struct{})
	for i, step := range t.Steps {
		if step.ID == "" {
			return fmt.Errorf("tool %s: step[%d].id is required", t.Name, i)
		}
		if _, exists := seen[step.ID]; exists {
			return fmt.Errorf("tool %s: duplicate step id %q", t.Name, step.ID)
		}
		seen[step.ID] = struct{}{}
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (t *CompositeTool) UnmarshalJSON(data []byte) error {
	type Alias CompositeTool
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*t = CompositeTool(alias)
	return t.validate()
}

// CompositeStep defines a single step in a composite tool.
//
// Validation:
//   - ID: required, non-empty string, must be unique within CompositeTool.Steps
//   - Tool: required, must have valid $ref or inline tool definition
type CompositeStep struct {
	// ID is the step identifier, used for referencing output in later steps
	ID string `json:"id" msgpack:"id"`
	// Tool is the tool to execute (reference or inline)
	Tool ToolRef `json:"tool" msgpack:"tool"`
	// InputJQ is a jq expression to extract/transform arguments from context.
	// The context object is: {"input": <raw_args>, "steps": {"step1": <output1>, ...}}
	// The jq result is used directly as the step's arguments JSON.
	// If empty, for step 0 uses raw input, for other steps uses previous step's output.
	InputJQ *JQExpr `json:"input_jq,omitzero" yaml:"input_jq,omitempty" msgpack:"input_jq,omitempty"`
}
