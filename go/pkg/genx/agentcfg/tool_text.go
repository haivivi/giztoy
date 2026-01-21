package agentcfg

import (
	"encoding/json"
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

// TextProcessorInput is the input for text processor tools.
type TextProcessorInput struct {
	// Content is the text content to process (e.g., conversation history)
	Content string `json:"content" msgpack:"content"`
}

// TextProcessorTool is a tool for processing text content using LLM.
// Used to process text (e.g., summarize conversation history).
//
// Validation:
//   - Inherits ToolBase validation (Name required)
//   - Prompt: required, non-empty string
//   - Model: required, non-empty string
//   - OutputMode: validated via TextProcessorOutputMode unmarshal
//   - OutputSchema: required when OutputMode is "json"
type TextProcessorTool struct {
	ToolBase `msgpack:",inline"`
	// Prompt is the system prompt for processing
	Prompt string `json:"prompt" msgpack:"prompt"`
	// Model specifies which model to use
	Model string `json:"model" msgpack:"model"`
	// OutputMode: "text" (plain text, default) or "json" (structured JSON)
	OutputMode TextProcessorOutputMode `json:"output_mode,omitzero" msgpack:"output_mode,omitempty"`
	// OutputSchema is the JSON schema for json output mode
	OutputSchema *JSONSchema `json:"output_schema,omitzero" msgpack:"output_schema,omitempty"`
}

// validate checks if the TextProcessorTool fields are valid.
func (t *TextProcessorTool) validate() error {
	if t.Name == "" {
		return fmt.Errorf("text_processor tool: name is required")
	}
	if t.Prompt == "" {
		return fmt.Errorf("tool %s: prompt is required", t.Name)
	}
	if t.Model == "" {
		return fmt.Errorf("tool %s: model is required", t.Name)
	}
	if t.OutputMode == TextProcessorOutputJSON && t.OutputSchema == nil {
		return fmt.Errorf("tool %s: output_schema is required for json output mode", t.Name)
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (t *TextProcessorTool) UnmarshalJSON(data []byte) error {
	type Alias TextProcessorTool
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*t = TextProcessorTool(alias)
	return t.validate()
}

// TextProcessorToolRef is a reference to or inline definition of TextProcessorTool.
//
// Validation:
//   - Must have exactly one of: Ref (reference) or TextProcessorTool (inline definition)
type TextProcessorToolRef struct {
	// Ref is a reference to existing text processor tool, e.g. "tool:summarizer"
	Ref string `json:"$ref,omitzero" msgpack:"ref,omitempty"`
	// Inline definition (fields flattened via embed)
	*TextProcessorTool `msgpack:"text_processor_tool,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler for TextProcessorToolRef.
func (r *TextProcessorToolRef) UnmarshalJSON(data []byte) error {
	// First try to get $ref
	var refOnly struct {
		Ref string `json:"$ref"`
	}
	if err := json.Unmarshal(data, &refOnly); err == nil && refOnly.Ref != "" {
		r.Ref = refOnly.Ref
		return nil
	}

	// Parse as inline TextProcessorTool
	var def TextProcessorTool
	if err := json.Unmarshal(data, &def); err != nil {
		return err
	}
	r.TextProcessorTool = &def
	return nil
}

// MarshalJSON implements json.Marshaler for TextProcessorToolRef.
func (r TextProcessorToolRef) MarshalJSON() ([]byte, error) {
	if r.Ref != "" {
		return json.Marshal(map[string]string{"$ref": r.Ref})
	}
	if r.TextProcessorTool != nil {
		return json.Marshal(r.TextProcessorTool)
	}
	return []byte("null"), nil
}

// IsRef returns true if this is a reference (not inline).
func (r *TextProcessorToolRef) IsRef() bool {
	return r.Ref != ""
}

// IsEmpty returns true if neither Ref nor inline definition is set.
func (r *TextProcessorToolRef) IsEmpty() bool {
	return r.Ref == "" && r.TextProcessorTool == nil
}

// textProcessorToolRefMsgpack is the msgpack-friendly representation of TextProcessorToolRef.
type textProcessorToolRefMsgpack struct {
	Ref  string `msgpack:"ref,omitempty"`
	Tool []byte `msgpack:"tool,omitempty"`
}

// EncodeMsgpack implements msgpack.CustomEncoder for TextProcessorToolRef.
func (r TextProcessorToolRef) EncodeMsgpack(enc *msgpack.Encoder) error {
	m := textProcessorToolRefMsgpack{Ref: r.Ref}
	if r.TextProcessorTool != nil {
		data, err := msgpack.Marshal(r.TextProcessorTool)
		if err != nil {
			return err
		}
		m.Tool = data
	}
	return enc.Encode(m)
}

// DecodeMsgpack implements msgpack.CustomDecoder for TextProcessorToolRef.
func (r *TextProcessorToolRef) DecodeMsgpack(dec *msgpack.Decoder) error {
	var m textProcessorToolRefMsgpack
	if err := dec.Decode(&m); err != nil {
		return err
	}
	r.Ref = m.Ref
	if len(m.Tool) > 0 {
		var tool TextProcessorTool
		if err := msgpack.Unmarshal(m.Tool, &tool); err != nil {
			return err
		}
		r.TextProcessorTool = &tool
	}
	return nil
}
