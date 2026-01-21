package agentcfg

import (
	"encoding/json"
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

// GeneratorTool is a single-round LLM generation tool.
//
// Validation:
//   - Inherits ToolBase validation (Name required)
//   - Model: required, non-empty string
//   - Mode: validated via GeneratorMode unmarshal
//   - OutputSchema: required when Mode is "json_output"
type GeneratorTool struct {
	ToolBase `msgpack:",inline"`
	// Prompt is the system prompt text (simple form)
	Prompt string `json:"prompt,omitzero" msgpack:"prompt,omitempty"`
	// ContextLayers for multi-layer prompt injection (like Agent)
	ContextLayers []ContextLayer `json:"context_layers,omitzero" msgpack:"context_layers,omitempty"`
	// Model specifies which model to use
	Model string `json:"model" msgpack:"model"`
	// Mode: "generate" (streaming text) or "json_output" (structured JSON)
	Mode GeneratorMode `json:"mode" msgpack:"mode"`
	// OutputSchema is the JSON schema for json_output mode
	OutputSchema *JSONSchema `json:"output_schema,omitzero" msgpack:"output_schema,omitempty"`
}

// validate checks if the GeneratorTool fields are valid.
func (t *GeneratorTool) validate() error {
	if t.Name == "" {
		return fmt.Errorf("generator tool: name is required")
	}
	if t.Model == "" {
		return fmt.Errorf("tool %s: model is required", t.Name)
	}
	if t.Mode == GeneratorModeJSONOutput && t.OutputSchema == nil {
		return fmt.Errorf("tool %s: output_schema is required for json_output mode", t.Name)
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (t *GeneratorTool) UnmarshalJSON(data []byte) error {
	type Alias GeneratorTool
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*t = GeneratorTool(alias)
	return t.validate()
}

// Generator defines generator configuration (for Agent).
//
// Validation:
//   - Model: required, non-empty string
type Generator struct {
	Model string `json:"model" msgpack:"model"` // model name, e.g. gpt-4o
}

// GeneratorRef is a reference to or inline definition of Generator.
// Supports $ref to external generator config or inline definition.
//
// Validation:
//   - Must have exactly one of: Ref (reference) or Generator (inline definition)
type GeneratorRef struct {
	// Ref is a reference to external generator, e.g. "generator:qwen-flash"
	Ref string `json:"$ref,omitzero" msgpack:"ref,omitempty"`
	// Inline generator definition (fields flattened via embed)
	*Generator `msgpack:"generator,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler for GeneratorRef.
func (g *GeneratorRef) UnmarshalJSON(data []byte) error {
	// First try to get $ref
	var refOnly struct {
		Ref string `json:"$ref"`
	}
	if err := json.Unmarshal(data, &refOnly); err == nil && refOnly.Ref != "" {
		g.Ref = refOnly.Ref
		return nil
	}

	// Parse as inline Generator
	var def Generator
	if err := json.Unmarshal(data, &def); err != nil {
		return err
	}
	g.Generator = &def
	return nil
}

// MarshalJSON implements json.Marshaler for GeneratorRef.
func (g GeneratorRef) MarshalJSON() ([]byte, error) {
	if g.Ref != "" {
		return json.Marshal(map[string]string{"$ref": g.Ref})
	}
	if g.Generator != nil {
		return json.Marshal(g.Generator)
	}
	return []byte("null"), nil
}

// IsRef returns true if this is a reference (not inline definition).
func (g *GeneratorRef) IsRef() bool {
	return g.Ref != ""
}

// IsEmpty returns true if neither ref nor inline definition is set.
func (g *GeneratorRef) IsEmpty() bool {
	return g.Ref == "" && g.Generator == nil
}

// GeneratorToolRef is a reference to or inline definition of GeneratorTool.
// Supports $ref to external generator tool or inline definition.
//
// Validation:
//   - Must have exactly one of: Ref (reference) or GeneratorTool (inline definition)
type GeneratorToolRef struct {
	// Ref is a reference to external generator tool, e.g. "tool:summarizer"
	Ref string `json:"$ref,omitzero" msgpack:"ref,omitempty"`
	// Inline generator tool definition (fields flattened via embed)
	*GeneratorTool `msgpack:"generator_tool,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler for GeneratorToolRef.
func (g *GeneratorToolRef) UnmarshalJSON(data []byte) error {
	// First try to get $ref
	var refOnly struct {
		Ref string `json:"$ref"`
	}
	if err := json.Unmarshal(data, &refOnly); err == nil && refOnly.Ref != "" {
		g.Ref = refOnly.Ref
		return nil
	}

	// Parse as inline GeneratorTool
	var def GeneratorTool
	if err := json.Unmarshal(data, &def); err != nil {
		return err
	}
	g.GeneratorTool = &def
	return nil
}

// MarshalJSON implements json.Marshaler for GeneratorToolRef.
func (g GeneratorToolRef) MarshalJSON() ([]byte, error) {
	if g.Ref != "" {
		return json.Marshal(map[string]string{"$ref": g.Ref})
	}
	if g.GeneratorTool != nil {
		return json.Marshal(g.GeneratorTool)
	}
	return []byte("null"), nil
}

// IsRef returns true if this is a reference (not inline definition).
func (g *GeneratorToolRef) IsRef() bool {
	return g.Ref != ""
}

// IsEmpty returns true if neither ref nor inline definition is set.
func (g *GeneratorToolRef) IsEmpty() bool {
	return g.Ref == "" && g.GeneratorTool == nil
}

// generatorToolRefMsgpack is the msgpack-friendly representation of GeneratorToolRef.
type generatorToolRefMsgpack struct {
	Ref  string `msgpack:"ref,omitempty"`
	Tool []byte `msgpack:"tool,omitempty"`
}

// EncodeMsgpack implements msgpack.CustomEncoder for GeneratorToolRef.
func (g GeneratorToolRef) EncodeMsgpack(enc *msgpack.Encoder) error {
	m := generatorToolRefMsgpack{Ref: g.Ref}
	if g.GeneratorTool != nil {
		data, err := msgpack.Marshal(g.GeneratorTool)
		if err != nil {
			return err
		}
		m.Tool = data
	}
	return enc.Encode(m)
}

// DecodeMsgpack implements msgpack.CustomDecoder for GeneratorToolRef.
func (g *GeneratorToolRef) DecodeMsgpack(dec *msgpack.Decoder) error {
	var m generatorToolRefMsgpack
	if err := dec.Decode(&m); err != nil {
		return err
	}
	g.Ref = m.Ref
	if len(m.Tool) > 0 {
		var tool GeneratorTool
		if err := msgpack.Unmarshal(m.Tool, &tool); err != nil {
			return err
		}
		g.GeneratorTool = &tool
	}
	return nil
}
