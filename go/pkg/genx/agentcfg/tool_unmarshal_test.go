package agentcfg

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
	"gopkg.in/yaml.v3"
)

func TestUnmarshalTool_HTTPGet(t *testing.T) {
	data := loadTestFile(t, "testdata/tool/http_get.json")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	if tool.ToolName() != "get_weather" {
		t.Errorf("ToolName() = %q, want %q", tool.ToolName(), "get_weather")
	}
	if tool.ToolDescription() != "Get weather information" {
		t.Errorf("ToolDescription() = %q, want %q", tool.ToolDescription(), "Get weather information")
	}
	if tool.ToolType() != ToolTypeHTTP {
		t.Errorf("ToolType() = %q, want %q", tool.ToolType(), ToolTypeHTTP)
	}

	http := AsHTTPTool(tool)
	if http == nil {
		t.Fatal("AsHTTPTool returned nil")
	}

	if http.Name != "get_weather" {
		t.Errorf("Name = %q, want %q", http.Name, "get_weather")
	}
	if http.Method != "GET" {
		t.Errorf("Method = %q, want %q", http.Method, "GET")
	}
	if http.Headers["X-API-Key"] != "${WEATHER_API_KEY}" {
		t.Errorf("Headers[X-API-Key] = %q, want %q", http.Headers["X-API-Key"], "${WEATHER_API_KEY}")
	}
}

func TestUnmarshalTool_HTTPPost(t *testing.T) {
	data := loadTestFile(t, "testdata/tool/http_post.json")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	http := AsHTTPTool(tool)
	if http == nil {
		t.Fatal("AsHTTPTool returned nil")
	}

	if http.ReqBodyJQ == nil {
		t.Fatal("ReqBodyJQ is nil")
	}
	if http.ReqBodyJQ.Expr != "{query: .query, limit: 10}" {
		t.Errorf("ReqBodyJQ.Expr = %q, want %q", http.ReqBodyJQ.Expr, "{query: .query, limit: 10}")
	}

	if http.RespBodyJQ == nil {
		t.Fatal("RespBodyJQ is nil")
	}
	if http.RespBodyJQ.Expr != ".results" {
		t.Errorf("RespBodyJQ.Expr = %q, want %q", http.RespBodyJQ.Expr, ".results")
	}
}

func TestUnmarshalTool_Generator(t *testing.T) {
	data := loadTestFile(t, "testdata/tool/generator.json")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	if tool.ToolType() != ToolTypeGenerator {
		t.Errorf("ToolType() = %q, want %q", tool.ToolType(), ToolTypeGenerator)
	}

	gen := AsGeneratorTool(tool)
	if gen == nil {
		t.Fatal("AsGeneratorTool returned nil")
	}

	if gen.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", gen.Model, "gpt-4o")
	}
	if gen.Mode != GeneratorModeGenerate {
		t.Errorf("Mode = %q, want %q", gen.Mode, GeneratorModeGenerate)
	}
}

func TestUnmarshalTool_Composite(t *testing.T) {
	data := loadTestFile(t, "testdata/tool/composite.json")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	if tool.ToolType() != ToolTypeComposite {
		t.Errorf("ToolType() = %q, want %q", tool.ToolType(), ToolTypeComposite)
	}

	comp := AsCompositeTool(tool)
	if comp == nil {
		t.Fatal("AsCompositeTool returned nil")
	}

	if len(comp.Steps) != 2 {
		t.Fatalf("len(Steps) = %d, want 2", len(comp.Steps))
	}

	if comp.Steps[0].ID != "search" {
		t.Errorf("Steps[0].ID = %q, want %q", comp.Steps[0].ID, "search")
	}
	if comp.Steps[1].ID != "summarize" {
		t.Errorf("Steps[1].ID = %q, want %q", comp.Steps[1].ID, "summarize")
	}
	if comp.Steps[1].InputJQ == nil {
		t.Fatal("Steps[1].InputJQ is nil")
	}
}

func TestUnmarshalTool_BuiltIn(t *testing.T) {
	data := []byte(`{"name": "echo", "description": "Echo input"}`)

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	if tool.ToolType() != ToolTypeBuiltIn {
		t.Errorf("ToolType() = %q, want %q", tool.ToolType(), ToolTypeBuiltIn)
	}

	builtin := AsBuiltInTool(tool)
	if builtin == nil {
		t.Fatal("AsBuiltInTool returned nil")
	}

	if builtin.Name != "echo" {
		t.Errorf("Name = %q, want %q", builtin.Name, "echo")
	}
}

func TestToolRef_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantRef string
		isRef   bool
		quit    bool
	}{
		{
			name:    "reference only",
			json:    `{"$ref": "tool:play_music"}`,
			wantRef: "tool:play_music",
			isRef:   true,
			quit:    false,
		},
		{
			name:    "reference with quit",
			json:    `{"$ref": "tool:stop", "quit": true}`,
			wantRef: "tool:stop",
			isRef:   true,
			quit:    true,
		},
		{
			name:  "inline tool",
			json:  `{"name": "inline", "description": "test"}`,
			isRef: false,
			quit:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ref ToolRef
			if err := json.Unmarshal([]byte(tt.json), &ref); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if ref.IsRef() != tt.isRef {
				t.Errorf("IsRef() = %v, want %v", ref.IsRef(), tt.isRef)
			}
			if tt.isRef && ref.Ref != tt.wantRef {
				t.Errorf("Ref = %q, want %q", ref.Ref, tt.wantRef)
			}
			if ref.Quit != tt.quit {
				t.Errorf("Quit = %v, want %v", ref.Quit, tt.quit)
			}
		})
	}
}

func TestTool_JSONRoundtrip(t *testing.T) {
	original := &HTTPTool{
		ToolBase: ToolBase{
			Name:        "test_http",
			Type:        ToolTypeHTTP,
			Description: "Test HTTP tool",
		},
		Method:   "POST",
		Endpoint: "https://example.com/api",
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Unmarshal
	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	result := AsHTTPTool(tool)
	if result == nil {
		t.Fatal("AsHTTPTool returned nil")
	}

	if result.Name != original.Name {
		t.Errorf("Name = %q, want %q", result.Name, original.Name)
	}
	if result.Method != original.Method {
		t.Errorf("Method = %q, want %q", result.Method, original.Method)
	}
	if result.Endpoint != original.Endpoint {
		t.Errorf("Endpoint = %q, want %q", result.Endpoint, original.Endpoint)
	}
}

// ========== YAML Tests ==========

// loadYAMLTestFile loads a YAML file and converts to JSON for UnmarshalTool.
func loadYAMLTestFile(t *testing.T, path string) []byte {
	t.Helper()
	yamlData := loadTestFile(t, path)

	var m any
	if err := yaml.Unmarshal(yamlData, &m); err != nil {
		t.Fatalf("yaml unmarshal %s: %v", path, err)
	}

	jsonData, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("json marshal %s: %v", path, err)
	}
	return jsonData
}

func TestUnmarshalTool_YAML_BuiltIn(t *testing.T) {
	data := loadYAMLTestFile(t, "testdata/tool/builtin.yaml")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	if tool.ToolType() != ToolTypeBuiltIn {
		t.Errorf("ToolType() = %q, want %q", tool.ToolType(), ToolTypeBuiltIn)
	}

	builtin := AsBuiltInTool(tool)
	if builtin == nil {
		t.Fatal("AsBuiltInTool returned nil")
	}

	if builtin.Name != "echo" {
		t.Errorf("Name = %q, want %q", builtin.Name, "echo")
	}
	if builtin.Params == nil {
		t.Error("Params is nil")
	}
}

func TestUnmarshalTool_YAML_HTTPGet(t *testing.T) {
	data := loadYAMLTestFile(t, "testdata/tool/http_get.yaml")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	http := AsHTTPTool(tool)
	if http == nil {
		t.Fatal("AsHTTPTool returned nil")
	}

	if http.Name != "get_weather" {
		t.Errorf("Name = %q, want %q", http.Name, "get_weather")
	}
	if http.Method != "GET" {
		t.Errorf("Method = %q, want %q", http.Method, "GET")
	}
}

func TestUnmarshalTool_YAML_HTTPPost(t *testing.T) {
	data := loadYAMLTestFile(t, "testdata/tool/http_post.yaml")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	http := AsHTTPTool(tool)
	if http == nil {
		t.Fatal("AsHTTPTool returned nil")
	}

	if http.ReqBodyJQ == nil {
		t.Fatal("ReqBodyJQ is nil")
	}
	if http.RespBodyJQ == nil {
		t.Fatal("RespBodyJQ is nil")
	}
}

func TestUnmarshalTool_YAML_HTTPWithAuth(t *testing.T) {
	data := loadYAMLTestFile(t, "testdata/tool/http_with_auth.yaml")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	http := AsHTTPTool(tool)
	if http == nil {
		t.Fatal("AsHTTPTool returned nil")
	}

	if http.Auth == nil {
		t.Fatal("Auth is nil")
	}
	if http.Auth.Type != "bearer" {
		t.Errorf("Auth.Type = %q, want %q", http.Auth.Type, "bearer")
	}
}

func TestUnmarshalTool_YAML_Generator(t *testing.T) {
	data := loadYAMLTestFile(t, "testdata/tool/generator.yaml")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	gen := AsGeneratorTool(tool)
	if gen == nil {
		t.Fatal("AsGeneratorTool returned nil")
	}

	if gen.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", gen.Model, "gpt-4o")
	}
}

func TestUnmarshalTool_YAML_GeneratorJSONOutput(t *testing.T) {
	data := loadYAMLTestFile(t, "testdata/tool/generator_json_output.yaml")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	gen := AsGeneratorTool(tool)
	if gen == nil {
		t.Fatal("AsGeneratorTool returned nil")
	}

	if gen.Mode != GeneratorModeJSONOutput {
		t.Errorf("Mode = %q, want %q", gen.Mode, GeneratorModeJSONOutput)
	}
	if gen.OutputSchema == nil {
		t.Error("OutputSchema is nil")
	}
}

func TestUnmarshalTool_YAML_GeneratorWithContext(t *testing.T) {
	data := loadYAMLTestFile(t, "testdata/tool/generator_with_context.yaml")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	gen := AsGeneratorTool(tool)
	if gen == nil {
		t.Fatal("AsGeneratorTool returned nil")
	}

	if len(gen.ContextLayers) != 2 {
		t.Errorf("len(ContextLayers) = %d, want 2", len(gen.ContextLayers))
	}
}

func TestUnmarshalTool_YAML_Composite(t *testing.T) {
	data := loadYAMLTestFile(t, "testdata/tool/composite.yaml")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	comp := AsCompositeTool(tool)
	if comp == nil {
		t.Fatal("AsCompositeTool returned nil")
	}

	if len(comp.Steps) != 2 {
		t.Fatalf("len(Steps) = %d, want 2", len(comp.Steps))
	}
}

func TestUnmarshalTool_YAML_CompositeInline(t *testing.T) {
	data := loadYAMLTestFile(t, "testdata/tool/composite_inline.yaml")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	comp := AsCompositeTool(tool)
	if comp == nil {
		t.Fatal("AsCompositeTool returned nil")
	}

	if len(comp.Steps) != 2 {
		t.Fatalf("len(Steps) = %d, want 2", len(comp.Steps))
	}

	// First step should have inline tool
	if comp.Steps[0].Tool.IsRef() {
		t.Error("Steps[0].Tool should be inline, not reference")
	}
}

// ========== TextProcessorTool Tests ==========

func TestUnmarshalTool_TextProcessor(t *testing.T) {
	data := loadTestFile(t, "testdata/tool/text_processor.json")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	if tool.ToolType() != ToolTypeTextProcessor {
		t.Errorf("ToolType() = %q, want %q", tool.ToolType(), ToolTypeTextProcessor)
	}

	tp := AsTextProcessorTool(tool)
	if tp == nil {
		t.Fatal("AsTextProcessorTool returned nil")
	}

	if tp.Name != "summarizer" {
		t.Errorf("Name = %q, want %q", tp.Name, "summarizer")
	}
	if tp.Model != "gpt-4o-mini" {
		t.Errorf("Model = %q, want %q", tp.Model, "gpt-4o-mini")
	}
	if tp.Prompt != "Please summarize the following conversation concisely." {
		t.Errorf("Prompt = %q", tp.Prompt)
	}
}

func TestUnmarshalTool_TextProcessorJSONOutput(t *testing.T) {
	data := loadTestFile(t, "testdata/tool/text_processor_json_output.json")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	tp := AsTextProcessorTool(tool)
	if tp == nil {
		t.Fatal("AsTextProcessorTool returned nil")
	}

	if tp.OutputMode != TextProcessorOutputJSON {
		t.Errorf("OutputMode = %q, want %q", tp.OutputMode, TextProcessorOutputJSON)
	}
	if tp.OutputSchema == nil {
		t.Error("OutputSchema is nil")
	}
}

func TestUnmarshalTool_YAML_TextProcessor(t *testing.T) {
	data := loadYAMLTestFile(t, "testdata/tool/text_processor.yaml")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	tp := AsTextProcessorTool(tool)
	if tp == nil {
		t.Fatal("AsTextProcessorTool returned nil")
	}

	if tp.Name != "summarizer" {
		t.Errorf("Name = %q, want %q", tp.Name, "summarizer")
	}
	if tp.Model != "gpt-4o-mini" {
		t.Errorf("Model = %q, want %q", tp.Model, "gpt-4o-mini")
	}
}

func TestUnmarshalTool_YAML_TextProcessorJSONOutput(t *testing.T) {
	data := loadYAMLTestFile(t, "testdata/tool/text_processor_json_output.yaml")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	tp := AsTextProcessorTool(tool)
	if tp == nil {
		t.Fatal("AsTextProcessorTool returned nil")
	}

	if tp.OutputMode != TextProcessorOutputJSON {
		t.Errorf("OutputMode = %q, want %q", tp.OutputMode, TextProcessorOutputJSON)
	}
	if tp.OutputSchema == nil {
		t.Error("OutputSchema is nil")
	}
}

// ========== MsgPack Tests ==========

func TestTool_MsgpackRoundtrip_BuiltIn(t *testing.T) {
	data := loadTestFile(t, "testdata/tool/builtin.json")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	original := AsBuiltInTool(tool)
	if original == nil {
		t.Fatal("AsBuiltInTool returned nil")
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("MsgPack Marshal: %v", err)
	}

	var decoded BuiltInTool
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("MsgPack Unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Description != original.Description {
		t.Errorf("Description = %q, want %q", decoded.Description, original.Description)
	}
	if decoded.Params == nil {
		t.Error("Params is nil")
	}
}

func TestTool_MsgpackRoundtrip_HTTP(t *testing.T) {
	data := loadTestFile(t, "testdata/tool/http_with_auth.json")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	original := AsHTTPTool(tool)
	if original == nil {
		t.Fatal("AsHTTPTool returned nil")
	}

	// MsgPack roundtrip
	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("MsgPack Marshal: %v", err)
	}

	var decoded HTTPTool
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("MsgPack Unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Method != original.Method {
		t.Errorf("Method = %q, want %q", decoded.Method, original.Method)
	}
	if decoded.Auth == nil {
		t.Fatal("Auth is nil")
	}
	if decoded.Auth.Type != original.Auth.Type {
		t.Errorf("Auth.Type = %q, want %q", decoded.Auth.Type, original.Auth.Type)
	}
}

func TestTool_MsgpackRoundtrip_Generator(t *testing.T) {
	data := loadTestFile(t, "testdata/tool/generator_json_output.json")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	original := AsGeneratorTool(tool)
	if original == nil {
		t.Fatal("AsGeneratorTool returned nil")
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("MsgPack Marshal: %v", err)
	}

	var decoded GeneratorTool
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("MsgPack Unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Model != original.Model {
		t.Errorf("Model = %q, want %q", decoded.Model, original.Model)
	}
	if decoded.Mode != original.Mode {
		t.Errorf("Mode = %q, want %q", decoded.Mode, original.Mode)
	}
}

func TestTool_MsgpackRoundtrip_Composite(t *testing.T) {
	data := loadTestFile(t, "testdata/tool/composite.json")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	original := AsCompositeTool(tool)
	if original == nil {
		t.Fatal("AsCompositeTool returned nil")
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("MsgPack Marshal: %v", err)
	}

	var decoded CompositeTool
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("MsgPack Unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, original.Name)
	}
	if len(decoded.Steps) != len(original.Steps) {
		t.Errorf("len(Steps) = %d, want %d", len(decoded.Steps), len(original.Steps))
	}
}

func TestTool_MsgpackRoundtrip_TextProcessor(t *testing.T) {
	data := loadTestFile(t, "testdata/tool/text_processor_json_output.json")

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	original := AsTextProcessorTool(tool)
	if original == nil {
		t.Fatal("AsTextProcessorTool returned nil")
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("MsgPack Marshal: %v", err)
	}

	var decoded TextProcessorTool
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("MsgPack Unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Model != original.Model {
		t.Errorf("Model = %q, want %q", decoded.Model, original.Model)
	}
	if decoded.OutputMode != original.OutputMode {
		t.Errorf("OutputMode = %q, want %q", decoded.OutputMode, original.OutputMode)
	}
	if decoded.OutputSchema == nil {
		t.Error("OutputSchema is nil")
	}
}

func TestToolRef_MsgpackRoundtrip(t *testing.T) {
	tests := []struct {
		name     string
		original ToolRef
	}{
		{
			name:     "ref only",
			original: ToolRef{Ref: "tool:play_music"},
		},
		{
			name:     "ref with quit",
			original: ToolRef{Ref: "tool:stop", Quit: true},
		},
		{
			name: "inline builtin",
			original: ToolRef{
				Tool: &BuiltInTool{
					ToolBase: ToolBase{Name: "echo", Type: ToolTypeBuiltIn},
				},
			},
		},
		{
			name: "inline http",
			original: ToolRef{
				Tool: &HTTPTool{
					ToolBase: ToolBase{Name: "get", Type: ToolTypeHTTP},
					Method:   "GET",
					Endpoint: "https://example.com",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packed, err := msgpack.Marshal(tt.original)
			if err != nil {
				t.Fatalf("MsgPack Marshal: %v", err)
			}

			var decoded ToolRef
			if err := msgpack.Unmarshal(packed, &decoded); err != nil {
				t.Fatalf("MsgPack Unmarshal: %v", err)
			}

			if decoded.Ref != tt.original.Ref {
				t.Errorf("Ref = %q, want %q", decoded.Ref, tt.original.Ref)
			}
			if decoded.Quit != tt.original.Quit {
				t.Errorf("Quit = %v, want %v", decoded.Quit, tt.original.Quit)
			}
			if (decoded.Tool == nil) != (tt.original.Tool == nil) {
				t.Errorf("Tool nil mismatch")
			}
			if tt.original.Tool != nil && decoded.Tool != nil {
				if decoded.Tool.ToolName() != tt.original.Tool.ToolName() {
					t.Errorf("Tool.Name = %q, want %q", decoded.Tool.ToolName(), tt.original.Tool.ToolName())
				}
			}
		})
	}
}

// ========== Error Tests ==========

func TestUnmarshalTool_Error_UnknownType(t *testing.T) {
	data := loadTestFile(t, "testdata/error/tool_unknown_type.json")

	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
	if !strings.Contains(err.Error(), "invalid tool type") {
		t.Errorf("error = %q, want containing %q", err.Error(), "invalid tool type")
	}
}

func TestUnmarshalTool_Error_GeneratorNoModel(t *testing.T) {
	data := loadTestFile(t, "testdata/error/tool_generator_no_model.json")

	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error for generator without model")
	}
	if !strings.Contains(err.Error(), "model is required") {
		t.Errorf("error = %q, want containing %q", err.Error(), "model is required")
	}
}

func TestUnmarshalTool_Error_CompositeNoSteps(t *testing.T) {
	data := loadTestFile(t, "testdata/error/tool_composite_no_steps.json")

	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error for composite without steps")
	}
	if !strings.Contains(err.Error(), "steps is required") {
		t.Errorf("error = %q, want containing %q", err.Error(), "steps is required")
	}
}

func TestUnmarshalTool_Error_InvalidJSON(t *testing.T) {
	data := loadTestFile(t, "testdata/error/invalid_json.json")

	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestUnmarshalTool_Error_InvalidJQ(t *testing.T) {
	data := loadTestFile(t, "testdata/error/invalid_jq.json")

	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error for invalid JQ")
	}
}

func TestUnmarshalTool_Error_TextProcessorNoPrompt(t *testing.T) {
	data := loadTestFile(t, "testdata/error/tool_text_processor_no_prompt.json")

	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error for text processor without prompt")
	}
	if !strings.Contains(err.Error(), "prompt is required") {
		t.Errorf("error = %q, want containing %q", err.Error(), "prompt is required")
	}
}

func TestUnmarshalTool_Error_TextProcessorNoModel(t *testing.T) {
	data := loadTestFile(t, "testdata/error/tool_text_processor_no_model.json")

	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error for text processor without model")
	}
	if !strings.Contains(err.Error(), "model is required") {
		t.Errorf("error = %q, want containing %q", err.Error(), "model is required")
	}
}

// ========== Additional Tool Validate Error Tests ==========

func TestHTTPTool_Validate_Error_NoName(t *testing.T) {
	data := []byte(`{"type":"http","endpoint":"https://example.com","method":"GET"}`)
	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestHTTPTool_Validate_Error_NoEndpoint(t *testing.T) {
	data := []byte(`{"type":"http","name":"test","method":"GET"}`)
	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "endpoint is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestHTTPTool_Validate_Error_BearerNoToken(t *testing.T) {
	data := []byte(`{"type":"http","name":"test","endpoint":"https://example.com","method":"GET","auth":{"type":"bearer"}}`)
	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "token is required for bearer") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestCompositeTool_Validate_Error_NoName(t *testing.T) {
	data := []byte(`{"type":"composite","steps":[{"id":"s1","tool":{"$ref":"tool:test"}}]}`)
	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestCompositeTool_Validate_Error_StepNoID(t *testing.T) {
	data := []byte(`{"type":"composite","name":"test","steps":[{"tool":{"$ref":"tool:test"}}]}`)
	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "id is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestCompositeTool_Validate_Error_DuplicateStepID(t *testing.T) {
	data := []byte(`{"type":"composite","name":"test","steps":[{"id":"s1","tool":{"$ref":"tool:a"}},{"id":"s1","tool":{"$ref":"tool:b"}}]}`)
	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate step id") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestGeneratorTool_Validate_Error_NoName(t *testing.T) {
	data := []byte(`{"type":"generator","model":"gpt-4o","mode":"generate"}`)
	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestGeneratorTool_Validate_Error_JSONOutputNoSchema(t *testing.T) {
	data := []byte(`{"type":"generator","name":"test","model":"gpt-4o","mode":"json_output"}`)
	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "output_schema is required for json_output mode") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestTextProcessorTool_Validate_Error_NoName(t *testing.T) {
	data := []byte(`{"type":"text_processor","prompt":"test","model":"gpt-4o"}`)
	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestTextProcessorTool_Validate_Error_JSONOutputNoSchema(t *testing.T) {
	data := []byte(`{"type":"text_processor","name":"test","prompt":"test","model":"gpt-4o","output_mode":"json"}`)
	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "output_schema is required for json output mode") {
		t.Errorf("error = %q", err.Error())
	}
}

// ========== ToolRef DecodeMsgpack Inline Tests ==========

func TestToolRef_MsgpackRoundtrip_Inline(t *testing.T) {
	original := ToolRef{
		Quit: true,
		Tool: &HTTPTool{
			ToolBase: ToolBase{Name: "inline_http", Type: ToolTypeHTTP},
			Method:   HTTPMethodGET,
			Endpoint: "https://example.com",
		},
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("MsgPack Marshal: %v", err)
	}

	var decoded ToolRef
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("MsgPack Unmarshal: %v", err)
	}

	if decoded.Quit != original.Quit {
		t.Errorf("Quit = %v, want %v", decoded.Quit, original.Quit)
	}
	if decoded.Tool == nil {
		t.Fatal("Tool is nil after decode")
	}
	if decoded.Tool.ToolName() != "inline_http" {
		t.Errorf("Tool.Name = %q, want %q", decoded.Tool.ToolName(), "inline_http")
	}
}

// ========== As Functions Tests (nil returns) ==========

func TestAsHTTPTool_Nil(t *testing.T) {
	tool := &GeneratorTool{ToolBase: ToolBase{Name: "gen"}}
	if AsHTTPTool(tool) != nil {
		t.Error("AsHTTPTool should return nil for GeneratorTool")
	}
}

func TestAsGeneratorTool_Nil(t *testing.T) {
	tool := &HTTPTool{ToolBase: ToolBase{Name: "http"}}
	if AsGeneratorTool(tool) != nil {
		t.Error("AsGeneratorTool should return nil for HTTPTool")
	}
}

func TestAsCompositeTool_Nil(t *testing.T) {
	tool := &HTTPTool{ToolBase: ToolBase{Name: "http"}}
	if AsCompositeTool(tool) != nil {
		t.Error("AsCompositeTool should return nil for HTTPTool")
	}
}

func TestAsTextProcessorTool_Nil(t *testing.T) {
	tool := &HTTPTool{ToolBase: ToolBase{Name: "http"}}
	if AsTextProcessorTool(tool) != nil {
		t.Error("AsTextProcessorTool should return nil for HTTPTool")
	}
}

func TestAsBuiltInTool_Nil(t *testing.T) {
	tool := &HTTPTool{ToolBase: ToolBase{Name: "http"}}
	if AsBuiltInTool(tool) != nil {
		t.Error("AsBuiltInTool should return nil for HTTPTool")
	}
}

// ========== UnmarshalTool Error Tests ==========

func TestUnmarshalTool_Error_InvalidType(t *testing.T) {
	data := []byte(`{"type":"invalid_type","name":"test"}`)
	_, err := UnmarshalTool(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid tool type") {
		t.Errorf("error = %q", err.Error())
	}
}

// ========== ToolRef DecodeMsgpack for each tool type ==========

func TestToolRef_MsgpackRoundtrip_InlineGenerator(t *testing.T) {
	original := ToolRef{
		Tool: &GeneratorTool{
			ToolBase: ToolBase{Name: "gen_tool", Type: ToolTypeGenerator},
			Model:    "gpt-4o",
			Mode:     GeneratorModeGenerate,
		},
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("MsgPack Marshal: %v", err)
	}

	var decoded ToolRef
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("MsgPack Unmarshal: %v", err)
	}

	if decoded.Tool == nil {
		t.Fatal("Tool is nil after decode")
	}
	if decoded.Tool.ToolType() != ToolTypeGenerator {
		t.Errorf("Tool.Type = %q, want %q", decoded.Tool.ToolType(), ToolTypeGenerator)
	}
}

func TestToolRef_MsgpackRoundtrip_InlineComposite(t *testing.T) {
	original := ToolRef{
		Tool: &CompositeTool{
			ToolBase: ToolBase{Name: "comp_tool", Type: ToolTypeComposite},
			Steps: []CompositeStep{
				{ID: "s1", Tool: ToolRef{Ref: "tool:step1"}},
			},
		},
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("MsgPack Marshal: %v", err)
	}

	var decoded ToolRef
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("MsgPack Unmarshal: %v", err)
	}

	if decoded.Tool == nil {
		t.Fatal("Tool is nil after decode")
	}
	if decoded.Tool.ToolType() != ToolTypeComposite {
		t.Errorf("Tool.Type = %q, want %q", decoded.Tool.ToolType(), ToolTypeComposite)
	}
}

func TestToolRef_MsgpackRoundtrip_InlineTextProcessor(t *testing.T) {
	original := ToolRef{
		Tool: &TextProcessorTool{
			ToolBase: ToolBase{Name: "text_tool", Type: ToolTypeTextProcessor},
			Prompt:   "Summarize",
			Model:    "gpt-4o-mini",
		},
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("MsgPack Marshal: %v", err)
	}

	var decoded ToolRef
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("MsgPack Unmarshal: %v", err)
	}

	if decoded.Tool == nil {
		t.Fatal("Tool is nil after decode")
	}
	if decoded.Tool.ToolType() != ToolTypeTextProcessor {
		t.Errorf("Tool.Type = %q, want %q", decoded.Tool.ToolType(), ToolTypeTextProcessor)
	}
}

func TestToolRef_MsgpackRoundtrip_InlineBuiltIn(t *testing.T) {
	original := ToolRef{
		Tool: &BuiltInTool{
			ToolBase: ToolBase{Name: "builtin_tool"},
		},
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("MsgPack Marshal: %v", err)
	}

	var decoded ToolRef
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("MsgPack Unmarshal: %v", err)
	}

	if decoded.Tool == nil {
		t.Fatal("Tool is nil after decode")
	}
	// BuiltIn type defaults to empty string or "built-in"
	if decoded.Tool.ToolType() != ToolTypeBuiltIn {
		t.Errorf("Tool.Type = %q, want %q", decoded.Tool.ToolType(), ToolTypeBuiltIn)
	}
}

// ========== UnmarshalTool Nested Format Tests ==========

func TestUnmarshalTool_HTTP_Nested(t *testing.T) {
	data := []byte(`{"type":"http","name":"test","http":{"name":"test","method":"GET","endpoint":"https://example.com"}}`)

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	http := AsHTTPTool(tool)
	if http == nil {
		t.Fatal("AsHTTPTool returned nil")
	}
	if http.Method != HTTPMethodGET {
		t.Errorf("Method = %q, want GET", http.Method)
	}
}

func TestUnmarshalTool_Generator_Nested(t *testing.T) {
	data := []byte(`{"type":"generator","name":"test","generator":{"name":"test","model":"gpt-4o","mode":"generate"}}`)

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	gen := AsGeneratorTool(tool)
	if gen == nil {
		t.Fatal("AsGeneratorTool returned nil")
	}
	if gen.Model != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o", gen.Model)
	}
}

func TestUnmarshalTool_Composite_Nested(t *testing.T) {
	data := []byte(`{"type":"composite","name":"test","composite":{"name":"test","steps":[{"id":"s1","tool":{"$ref":"tool:a"}}]}}`)

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	comp := AsCompositeTool(tool)
	if comp == nil {
		t.Fatal("AsCompositeTool returned nil")
	}
	if len(comp.Steps) != 1 {
		t.Errorf("len(Steps) = %d, want 1", len(comp.Steps))
	}
}

func TestUnmarshalTool_TextProcessor_Nested(t *testing.T) {
	data := []byte(`{"type":"text_processor","name":"test","text_processor":{"name":"test","prompt":"test","model":"gpt-4o"}}`)

	tool, err := UnmarshalTool(data)
	if err != nil {
		t.Fatalf("UnmarshalTool: %v", err)
	}

	text := AsTextProcessorTool(tool)
	if text == nil {
		t.Fatal("AsTextProcessorTool returned nil")
	}
	if text.Prompt != "test" {
		t.Errorf("Prompt = %q, want test", text.Prompt)
	}
}

// ========== Tool Unmarshal Invalid Data Tests ==========

func TestToolRef_UnmarshalJSON_InvalidData(t *testing.T) {
	var ref ToolRef
	err := json.Unmarshal([]byte(`{invalid`), &ref)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestToolRef_DecodeMsgpack_InvalidData(t *testing.T) {
	var ref ToolRef
	err := msgpack.Unmarshal([]byte{0xff}, &ref)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestHTTPTool_UnmarshalJSON_InvalidData(t *testing.T) {
	var tool HTTPTool
	err := json.Unmarshal([]byte(`{invalid`), &tool)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGeneratorTool_UnmarshalJSON_InvalidData(t *testing.T) {
	var tool GeneratorTool
	err := json.Unmarshal([]byte(`{invalid`), &tool)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCompositeTool_UnmarshalJSON_InvalidData(t *testing.T) {
	var tool CompositeTool
	err := json.Unmarshal([]byte(`{invalid`), &tool)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTextProcessorTool_UnmarshalJSON_InvalidData(t *testing.T) {
	var tool TextProcessorTool
	err := json.Unmarshal([]byte(`{invalid`), &tool)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ========== Tool Types Unmarshal JSON Invalid Data Tests ==========

func TestHTTPTool_UnmarshalJSON_InvalidJSON(t *testing.T) {
	var tool HTTPTool
	err := json.Unmarshal([]byte(`{"method":123}`), &tool) // method should be string
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGeneratorTool_UnmarshalJSON_InvalidJSON(t *testing.T) {
	var tool GeneratorTool
	err := json.Unmarshal([]byte(`{"mode":123}`), &tool) // mode should be string
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCompositeTool_UnmarshalJSON_InvalidJSON(t *testing.T) {
	var tool CompositeTool
	err := json.Unmarshal([]byte(`{"mode":123}`), &tool) // mode should be string
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTextProcessorTool_UnmarshalJSON_InvalidJSON(t *testing.T) {
	var tool TextProcessorTool
	err := json.Unmarshal([]byte(`{"output_mode":123}`), &tool) // should be string
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGeneratorRef_UnmarshalJSON_InvalidData(t *testing.T) {
	var ref GeneratorRef
	err := json.Unmarshal([]byte(`{invalid`), &ref)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGeneratorToolRef_UnmarshalJSON_InvalidData(t *testing.T) {
	var ref GeneratorToolRef
	err := json.Unmarshal([]byte(`{invalid`), &ref)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTextProcessorToolRef_UnmarshalJSON_InvalidData(t *testing.T) {
	var ref TextProcessorToolRef
	err := json.Unmarshal([]byte(`{invalid`), &ref)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ========== Tool Validate Error Tests ==========
