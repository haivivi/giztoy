package agentcfg

import (
	"encoding/json"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

// ========== Tool JSON Marshal Tests ==========

func TestMarshalTool_BuiltIn_JSON(t *testing.T) {
	tool := &BuiltInTool{
		ToolBase: ToolBase{Name: "test_builtin", Type: ToolTypeBuiltIn, Description: "A test tool"},
		Params:   map[string]any{"key": "value"},
	}

	data, err := MarshalTool(tool)
	if err != nil {
		t.Fatalf("MarshalTool: %v", err)
	}

	got, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	builtin := AsBuiltInTool(got)
	if builtin == nil {
		t.Fatal("expected BuiltInTool")
	}
	if builtin.Name != tool.Name {
		t.Errorf("Name = %q, want %q", builtin.Name, tool.Name)
	}
	if builtin.Description != tool.Description {
		t.Errorf("Description = %q, want %q", builtin.Description, tool.Description)
	}
}

func TestMarshalTool_HTTP_JSON(t *testing.T) {
	tool := &HTTPTool{
		ToolBase: ToolBase{Name: "test_http", Type: ToolTypeHTTP},
		Method:   HTTPMethodGET,
		Endpoint: "https://api.example.com",
		Headers:  map[string]string{"X-API-Key": "secret"},
	}

	data, err := MarshalTool(tool)
	if err != nil {
		t.Fatalf("MarshalTool: %v", err)
	}

	got, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	http := AsHTTPTool(got)
	if http == nil {
		t.Fatal("expected HTTPTool")
	}
	if http.Name != tool.Name {
		t.Errorf("Name = %q, want %q", http.Name, tool.Name)
	}
	if http.Method != tool.Method {
		t.Errorf("Method = %q, want %q", http.Method, tool.Method)
	}
	if http.Endpoint != tool.Endpoint {
		t.Errorf("Endpoint = %q, want %q", http.Endpoint, tool.Endpoint)
	}
}

func TestMarshalTool_Generator_JSON(t *testing.T) {
	tool := &GeneratorTool{
		ToolBase: ToolBase{Name: "test_gen", Type: ToolTypeGenerator},
		Model:    "gpt-4o",
		Mode:     GeneratorModeGenerate,
		Prompt:   "You are helpful",
	}

	data, err := MarshalTool(tool)
	if err != nil {
		t.Fatalf("MarshalTool: %v", err)
	}

	got, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	gen := AsGeneratorTool(got)
	if gen == nil {
		t.Fatal("expected GeneratorTool")
	}
	if gen.Model != tool.Model {
		t.Errorf("Model = %q, want %q", gen.Model, tool.Model)
	}
	if gen.Mode != tool.Mode {
		t.Errorf("Mode = %q, want %q", gen.Mode, tool.Mode)
	}
}

func TestMarshalTool_Composite_JSON(t *testing.T) {
	tool := &CompositeTool{
		ToolBase: ToolBase{Name: "test_composite", Type: ToolTypeComposite},
		Mode:     CompositeModeSeq,
		Steps: []CompositeStep{
			{ID: "step1", Tool: ToolRef{Ref: "tool:helper"}},
		},
	}

	data, err := MarshalTool(tool)
	if err != nil {
		t.Fatalf("MarshalTool: %v", err)
	}

	got, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	comp := AsCompositeTool(got)
	if comp == nil {
		t.Fatal("expected CompositeTool")
	}
	if len(comp.Steps) != 1 {
		t.Errorf("Steps count = %d, want 1", len(comp.Steps))
	}
	if comp.Steps[0].ID != "step1" {
		t.Errorf("Steps[0].ID = %q, want %q", comp.Steps[0].ID, "step1")
	}
}

func TestMarshalTool_TextProcessor_JSON(t *testing.T) {
	tool := &TextProcessorTool{
		ToolBase: ToolBase{Name: "test_tp", Type: ToolTypeTextProcessor},
		Prompt:   "Summarize this",
		Model:    "gpt-4o",
	}

	data, err := MarshalTool(tool)
	if err != nil {
		t.Fatalf("MarshalTool: %v", err)
	}

	got, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	tp := AsTextProcessorTool(got)
	if tp == nil {
		t.Fatal("expected TextProcessorTool")
	}
	if tp.Prompt != tool.Prompt {
		t.Errorf("Prompt = %q, want %q", tp.Prompt, tool.Prompt)
	}
	if tp.Model != tool.Model {
		t.Errorf("Model = %q, want %q", tp.Model, tool.Model)
	}
}

// ========== ToolRef Marshal Tests ==========

func TestToolRef_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		ref  ToolRef
	}{
		{
			name: "ref only",
			ref:  ToolRef{Ref: "tool:helper"},
		},
		{
			name: "ref with quit",
			ref:  ToolRef{Ref: "tool:exit", Quit: true},
		},
		{
			name: "inline builtin",
			ref: ToolRef{
				Tool: &BuiltInTool{
					ToolBase: ToolBase{Name: "inline", Type: ToolTypeBuiltIn},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.ref)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var got ToolRef
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if got.Ref != tt.ref.Ref {
				t.Errorf("Ref = %q, want %q", got.Ref, tt.ref.Ref)
			}
			if got.Quit != tt.ref.Quit {
				t.Errorf("Quit = %v, want %v", got.Quit, tt.ref.Quit)
			}
		})
	}
}

// ========== Tool MsgPack Marshal Tests ==========

func TestMarshalTool_BuiltIn_Msgpack(t *testing.T) {
	original := &BuiltInTool{
		ToolBase: ToolBase{Name: "test_builtin", Type: ToolTypeBuiltIn, Description: "desc"},
		Params:   map[string]any{"key": "value"},
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded BuiltInTool
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, original.Name)
	}
}

func TestMarshalTool_HTTP_Msgpack(t *testing.T) {
	original := &HTTPTool{
		ToolBase: ToolBase{Name: "test_http", Type: ToolTypeHTTP},
		Method:   HTTPMethodPOST,
		Endpoint: "https://api.example.com",
		Headers:  map[string]string{"Content-Type": "application/json"},
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded HTTPTool
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Method != original.Method {
		t.Errorf("Method = %q, want %q", decoded.Method, original.Method)
	}
	if decoded.Endpoint != original.Endpoint {
		t.Errorf("Endpoint = %q, want %q", decoded.Endpoint, original.Endpoint)
	}
}

func TestMarshalTool_Generator_Msgpack(t *testing.T) {
	original := &GeneratorTool{
		ToolBase: ToolBase{Name: "test_gen", Type: ToolTypeGenerator},
		Model:    "gpt-4o",
		Mode:     GeneratorModeJSONOutput,
		Prompt:   "Generate JSON",
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded GeneratorTool
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Model != original.Model {
		t.Errorf("Model = %q, want %q", decoded.Model, original.Model)
	}
	if decoded.Mode != original.Mode {
		t.Errorf("Mode = %q, want %q", decoded.Mode, original.Mode)
	}
}

func TestMarshalTool_Composite_Msgpack(t *testing.T) {
	original := &CompositeTool{
		ToolBase: ToolBase{Name: "test_comp", Type: ToolTypeComposite},
		Mode:     CompositeModeSeq,
		Steps: []CompositeStep{
			{ID: "s1", Tool: ToolRef{Ref: "tool:a"}},
			{ID: "s2", Tool: ToolRef{Ref: "tool:b"}},
		},
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded CompositeTool
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, original.Name)
	}
	if len(decoded.Steps) != len(original.Steps) {
		t.Errorf("len(Steps) = %d, want %d", len(decoded.Steps), len(original.Steps))
	}
}

func TestMarshalTool_TextProcessor_Msgpack(t *testing.T) {
	original := &TextProcessorTool{
		ToolBase: ToolBase{Name: "test_tp", Type: ToolTypeTextProcessor},
		Prompt:   "Summarize",
		Model:    "gpt-4o",
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded TextProcessorTool
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Prompt != original.Prompt {
		t.Errorf("Prompt = %q, want %q", decoded.Prompt, original.Prompt)
	}
	if decoded.Model != original.Model {
		t.Errorf("Model = %q, want %q", decoded.Model, original.Model)
	}
}

func TestToolRef_MarshalMsgpack(t *testing.T) {
	tests := []struct {
		name     string
		original ToolRef
	}{
		{
			name:     "ref only",
			original: ToolRef{Ref: "tool:helper"},
		},
		{
			name:     "ref with quit",
			original: ToolRef{Ref: "tool:exit", Quit: true},
		},
		{
			name: "inline builtin",
			original: ToolRef{
				Tool: &BuiltInTool{
					ToolBase: ToolBase{Name: "inline", Type: ToolTypeBuiltIn},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packed, err := msgpack.Marshal(tt.original)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var decoded ToolRef
			if err := msgpack.Unmarshal(packed, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if decoded.Ref != tt.original.Ref {
				t.Errorf("Ref = %q, want %q", decoded.Ref, tt.original.Ref)
			}
			if decoded.Quit != tt.original.Quit {
				t.Errorf("Quit = %v, want %v", decoded.Quit, tt.original.Quit)
			}
		})
	}
}

// ========== ToolRef MarshalJSON Inline Tests ==========

func TestToolRef_MarshalJSON_Inline(t *testing.T) {
	ref := ToolRef{
		Tool: &HTTPTool{
			ToolBase: ToolBase{Name: "inline_http", Type: ToolTypeHTTP},
			Method:   HTTPMethodGET,
			Endpoint: "https://example.com",
		},
	}

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded["name"] != "inline_http" {
		t.Errorf("name = %v, want inline_http", decoded["name"])
	}
	if decoded["type"] != "http" {
		t.Errorf("type = %v, want http", decoded["type"])
	}
}

func TestToolRef_MarshalJSON_InlineWithQuit(t *testing.T) {
	ref := ToolRef{
		Quit: true,
		Tool: &HTTPTool{
			ToolBase: ToolBase{Name: "inline_http", Type: ToolTypeHTTP},
			Method:   HTTPMethodGET,
			Endpoint: "https://example.com",
		},
	}

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded["quit"] != true {
		t.Errorf("quit = %v, want true", decoded["quit"])
	}
}

// ========== ToolRef MarshalJSON Tests ==========

func TestToolRef_MarshalJSON_RefWithQuit(t *testing.T) {
	ref := ToolRef{
		Ref:  "tool:test",
		Quit: true,
	}

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded["$ref"] != "tool:test" {
		t.Errorf("$ref = %v, want tool:test", decoded["$ref"])
	}
	if decoded["quit"] != true {
		t.Errorf("quit = %v, want true", decoded["quit"])
	}
}

// ========== Tool Marshal Empty Tests ==========

func TestToolRef_MarshalJSON_Empty(t *testing.T) {
	ref := ToolRef{} // all fields are empty

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	// Should marshal to null or {}
	if string(data) != "null" && string(data) != "{}" {
		t.Errorf("MarshalJSON = %s, want null or {}", data)
	}
}

func TestToolRef_EncodeMsgpack_Empty(t *testing.T) {
	ref := ToolRef{} // all fields are empty

	data, err := msgpack.Marshal(ref)
	if err != nil {
		t.Fatalf("EncodeMsgpack: %v", err)
	}

	if len(data) == 0 {
		t.Error("data is empty")
	}
}

func TestGeneratorRef_MarshalJSON_Empty(t *testing.T) {
	ref := GeneratorRef{} // all fields are empty

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	// Should marshal to null or {}
	if string(data) != "null" && string(data) != "{}" {
		t.Errorf("MarshalJSON = %s, want null or {}", data)
	}
}

func TestTextProcessorToolRef_MarshalJSON_Empty(t *testing.T) {
	ref := TextProcessorToolRef{} // all fields are empty

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	// Should marshal to null or {}
	if string(data) != "null" && string(data) != "{}" {
		t.Errorf("MarshalJSON = %s, want null or {}", data)
	}
}

func TestGeneratorToolRef_MarshalJSON_Empty(t *testing.T) {
	ref := GeneratorToolRef{} // all fields are empty

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	// Should marshal to null or {}
	if string(data) != "null" && string(data) != "{}" {
		t.Errorf("MarshalJSON = %s, want null or {}", data)
	}
}
