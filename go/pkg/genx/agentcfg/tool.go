package agentcfg

import (
	"encoding/json"
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

// Tool is the interface for all tool definitions.
type Tool interface {
	ToolName() string
	ToolDescription() string
	ToolType() ToolType
}

// ToolBase contains common fields for all tool types.
//
// Validation:
//   - Name: required, non-empty string
//   - Type: validated via ToolType unmarshal
type ToolBase struct {
	Name        string   `json:"name" msgpack:"name"`
	Type        ToolType `json:"type,omitzero" msgpack:"type,omitempty"`
	Description string   `json:"description,omitzero" msgpack:"description,omitempty"`
}

func (b *ToolBase) ToolName() string        { return b.Name }
func (b *ToolBase) ToolDescription() string { return b.Description }
func (b *ToolBase) ToolType() ToolType {
	if b.Type == "" {
		return ToolTypeBuiltIn
	}
	return b.Type
}

// ToolRef is a tool reference in Agent.
// Supports $ref to external tool or inline Tool.
//
// Validation:
//   - Must have exactly one of: Ref (reference) or Tool (inline definition)
type ToolRef struct {
	// Ref is a reference to external tool, e.g. "tool:play_music"
	Ref string `json:"$ref,omitzero" msgpack:"ref,omitempty"`
	// Quit indicates this tool triggers agent completion when called.
	// When a quit tool is executed, the agent will finish after generating
	// the final response and return EventClosed from Next().
	Quit bool `json:"quit,omitzero" msgpack:"quit,omitempty"`
	// Inline tool definition (fields flattened via embed)
	// Note: when Ref is set, this should be nil
	Tool `msgpack:"tool,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler for ToolRef.
func (t *ToolRef) UnmarshalJSON(data []byte) error {
	// First try to get $ref and quit
	var refWithQuit struct {
		Ref  string `json:"$ref"`
		Quit bool   `json:"quit"`
	}
	if err := json.Unmarshal(data, &refWithQuit); err == nil && refWithQuit.Ref != "" {
		t.Ref = refWithQuit.Ref
		t.Quit = refWithQuit.Quit
		return nil
	}

	// Parse as inline Tool (also check for quit)
	var quitOnly struct {
		Quit bool `json:"quit"`
	}
	_ = json.Unmarshal(data, &quitOnly)
	t.Quit = quitOnly.Quit

	def, err := UnmarshalTool(data)
	if err != nil {
		return err
	}
	t.Tool = def
	return nil
}

// MarshalJSON implements json.Marshaler for ToolRef.
func (t ToolRef) MarshalJSON() ([]byte, error) {
	if t.Ref != "" {
		m := map[string]any{"$ref": t.Ref}
		if t.Quit {
			m["quit"] = true
		}
		return json.Marshal(m)
	}
	if t.Tool != nil {
		// For inline tools, marshal the tool def and add quit if needed
		if t.Quit {
			data, err := json.Marshal(t.Tool)
			if err != nil {
				return nil, err
			}
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, err
			}
			m["quit"] = true
			return json.Marshal(m)
		}
		return json.Marshal(t.Tool)
	}
	return []byte("null"), nil
}

// IsRef returns true if this is a reference (not inline).
func (t *ToolRef) IsRef() bool {
	return t.Ref != ""
}

// toolRefMsgpack is the msgpack-friendly representation of ToolRef.
type toolRefMsgpack struct {
	Ref  string   `msgpack:"ref,omitempty"`
	Quit bool     `msgpack:"quit,omitempty"`
	Type ToolType `msgpack:"type,omitempty"` // tool type for polymorphic decoding
	Tool []byte   `msgpack:"tool,omitempty"` // msgpack-encoded tool definition
}

// EncodeMsgpack implements msgpack.CustomEncoder for ToolRef.
func (t ToolRef) EncodeMsgpack(enc *msgpack.Encoder) error {
	m := toolRefMsgpack{Ref: t.Ref, Quit: t.Quit}
	if t.Tool != nil {
		m.Type = t.Tool.ToolType()
		data, err := msgpack.Marshal(t.Tool)
		if err != nil {
			return err
		}
		m.Tool = data
	}
	return enc.Encode(m)
}

// DecodeMsgpack implements msgpack.CustomDecoder for ToolRef.
func (t *ToolRef) DecodeMsgpack(dec *msgpack.Decoder) error {
	var m toolRefMsgpack
	if err := dec.Decode(&m); err != nil {
		return err
	}
	t.Ref = m.Ref
	t.Quit = m.Quit
	if len(m.Tool) > 0 {
		var def Tool
		var err error
		switch m.Type {
		case ToolTypeBuiltIn, "":
			var d BuiltInTool
			err = msgpack.Unmarshal(m.Tool, &d)
			def = &d
		case ToolTypeHTTP:
			var d HTTPTool
			err = msgpack.Unmarshal(m.Tool, &d)
			def = &d
		case ToolTypeGenerator:
			var d GeneratorTool
			err = msgpack.Unmarshal(m.Tool, &d)
			def = &d
		case ToolTypeComposite:
			var d CompositeTool
			err = msgpack.Unmarshal(m.Tool, &d)
			def = &d
		case ToolTypeTextProcessor:
			var d TextProcessorTool
			err = msgpack.Unmarshal(m.Tool, &d)
			def = &d
		default:
			return fmt.Errorf("unknown tool type: %s", m.Type)
		}
		if err != nil {
			return err
		}
		t.Tool = def
	}
	return nil
}

// BuiltInTool is a built-in tool invoked by runtime.
//
// Validation:
//   - Inherits ToolBase validation (Name required)
type BuiltInTool struct {
	ToolBase `msgpack:",inline"`
	Params   map[string]any `json:"params,omitzero" msgpack:"params,omitempty"` // JSON Schema for function arguments
}
