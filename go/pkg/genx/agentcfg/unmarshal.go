package agentcfg

import (
	"encoding/json"
	"fmt"
)

// toolRaw is the raw JSON structure for tool definitions.
// Supports nested format like: { type: "generator", generator: { model: ..., ... } }
type toolRaw struct {
	ToolBase
	Params        map[string]any     `json:"params,omitzero"`
	HTTP          *HTTPTool          `json:"http,omitzero"`
	Generator     *GeneratorTool     `json:"generator,omitzero"`
	Composite     *CompositeTool     `json:"composite,omitzero"`
	TextProcessor *TextProcessorTool `json:"text_processor,omitzero"`
}

// UnmarshalTool unmarshals JSON data into the appropriate Tool type.
// Supports both nested format (generator: { model: ... }) and flat format.
func UnmarshalTool(data []byte) (Tool, error) {
	var raw toolRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse tool: %w", err)
	}

	toolType := raw.Type
	if toolType == "" {
		toolType = ToolTypeBuiltIn
	}

	switch toolType {
	case ToolTypeHTTP:
		if raw.HTTP != nil {
			// Nested format: merge base into nested
			raw.HTTP.ToolBase = raw.ToolBase
			return raw.HTTP, nil
		}
		// Flat format: parse directly
		var t HTTPTool
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, fmt.Errorf("parse http tool: %w", err)
		}
		return &t, nil

	case ToolTypeGenerator:
		var t *GeneratorTool
		if raw.Generator != nil {
			raw.Generator.ToolBase = raw.ToolBase
			t = raw.Generator
		} else {
			t = &GeneratorTool{}
			if err := json.Unmarshal(data, t); err != nil {
				return nil, fmt.Errorf("parse generator tool: %w", err)
			}
		}
		if t.Model == "" {
			return nil, fmt.Errorf("tool %s: model is required", t.Name)
		}
		return t, nil

	case ToolTypeComposite:
		var t *CompositeTool
		if raw.Composite != nil {
			raw.Composite.ToolBase = raw.ToolBase
			t = raw.Composite
		} else {
			t = &CompositeTool{}
			if err := json.Unmarshal(data, t); err != nil {
				return nil, fmt.Errorf("parse composite tool: %w", err)
			}
		}
		if len(t.Steps) == 0 {
			return nil, fmt.Errorf("tool %s: steps is required", t.Name)
		}
		return t, nil

	case ToolTypeTextProcessor:
		var t *TextProcessorTool
		if raw.TextProcessor != nil {
			raw.TextProcessor.ToolBase = raw.ToolBase
			t = raw.TextProcessor
		} else {
			t = &TextProcessorTool{}
			if err := json.Unmarshal(data, t); err != nil {
				return nil, fmt.Errorf("parse text_processor tool: %w", err)
			}
		}
		if t.Prompt == "" {
			return nil, fmt.Errorf("tool %s: prompt is required", t.Name)
		}
		if t.Model == "" {
			return nil, fmt.Errorf("tool %s: model is required", t.Name)
		}
		return t, nil

	case ToolTypeBuiltIn:
		def := &BuiltInTool{
			ToolBase: raw.ToolBase,
			Params:   raw.Params,
		}
		return def, nil

	default:
		return nil, fmt.Errorf("unknown tool type: %s", toolType)
	}
}

// MarshalTool marshals a Tool to JSON.
func MarshalTool(def Tool) ([]byte, error) {
	return json.Marshal(def)
}

// Helper functions for type assertions

// AsHTTPTool returns the Tool as *HTTPTool if it is one, nil otherwise.
func AsHTTPTool(def Tool) *HTTPTool {
	if t, ok := def.(*HTTPTool); ok {
		return t
	}
	return nil
}

// AsGeneratorTool returns the Tool as *GeneratorTool if it is one, nil otherwise.
func AsGeneratorTool(def Tool) *GeneratorTool {
	if t, ok := def.(*GeneratorTool); ok {
		return t
	}
	return nil
}

// AsCompositeTool returns the Tool as *CompositeTool if it is one, nil otherwise.
func AsCompositeTool(def Tool) *CompositeTool {
	if t, ok := def.(*CompositeTool); ok {
		return t
	}
	return nil
}

// AsTextProcessorTool returns the Tool as *TextProcessorTool if it is one, nil otherwise.
func AsTextProcessorTool(def Tool) *TextProcessorTool {
	if t, ok := def.(*TextProcessorTool); ok {
		return t
	}
	return nil
}

// AsBuiltInTool returns the Tool as *BuiltInTool if it is one, nil otherwise.
func AsBuiltInTool(def Tool) *BuiltInTool {
	if t, ok := def.(*BuiltInTool); ok {
		return t
	}
	return nil
}
