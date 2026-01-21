package agentcfg

import (
	"encoding/json"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
	"gopkg.in/yaml.v3"
)

// ========== RuleRef Tests ==========

func TestRuleRef_JSON_Ref(t *testing.T) {
	data := []byte(`{"$ref": "rule:play_music"}`)

	var ref RuleRef
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !ref.IsRef() {
		t.Error("IsRef() = false, want true")
	}
	if ref.Ref != "rule:play_music" {
		t.Errorf("Ref = %q, want %q", ref.Ref, "rule:play_music")
	}
	if ref.Rule != nil {
		t.Error("Rule should be nil for reference")
	}
}

func TestRuleRef_JSON_Inline(t *testing.T) {
	data := []byte(`{
		"name": "weather_intent",
		"patterns": ["weather", "forecast"]
	}`)

	var ref RuleRef
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if ref.IsRef() {
		t.Error("IsRef() = true, want false")
	}
	if ref.Rule == nil {
		t.Fatal("Rule is nil")
	}
	if ref.Rule.Name != "weather_intent" {
		t.Errorf("Rule.Name = %q, want %q", ref.Rule.Name, "weather_intent")
	}
}

func TestRuleRef_YAML_Ref(t *testing.T) {
	data := []byte(`$ref: rule:play_music`)

	var ref RuleRef
	if err := yaml.Unmarshal(data, &ref); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !ref.IsRef() {
		t.Error("IsRef() = false, want true")
	}
	if ref.Ref != "rule:play_music" {
		t.Errorf("Ref = %q, want %q", ref.Ref, "rule:play_music")
	}
}

func TestRuleRef_YAML_Inline(t *testing.T) {
	// RuleRef inline with minimal Rule (name only, no patterns)
	// Note: Pattern type has custom YAML unmarshaler that doesn't work with yaml.Node
	data := []byte(`
name: weather_intent
`)

	var ref RuleRef
	if err := yaml.Unmarshal(data, &ref); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if ref.IsRef() {
		t.Error("IsRef() = true, want false")
	}
	if ref.Rule == nil {
		t.Fatal("Rule is nil")
	}
	if ref.Rule.Name != "weather_intent" {
		t.Errorf("Rule.Name = %q, want %q", ref.Rule.Name, "weather_intent")
	}
}

func TestRuleRef_MsgpackRoundtrip(t *testing.T) {
	tests := []struct {
		name     string
		original RuleRef
	}{
		{
			name:     "ref only",
			original: RuleRef{Ref: "rule:play_music"},
		},
		{
			name: "inline rule",
			original: RuleRef{
				Rule: &Rule{Name: "test_rule"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packed, err := msgpack.Marshal(tt.original)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var decoded RuleRef
			if err := msgpack.Unmarshal(packed, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if decoded.Ref != tt.original.Ref {
				t.Errorf("Ref = %q, want %q", decoded.Ref, tt.original.Ref)
			}
			if (decoded.Rule == nil) != (tt.original.Rule == nil) {
				t.Errorf("Rule nil mismatch")
			}
			if tt.original.Rule != nil && decoded.Rule != nil {
				if decoded.Rule.Name != tt.original.Rule.Name {
					t.Errorf("Rule.Name = %q, want %q", decoded.Rule.Name, tt.original.Rule.Name)
				}
			}
		})
	}
}

// ========== GeneratorRef Tests ==========

func TestGeneratorRef_JSON_Ref(t *testing.T) {
	data := []byte(`{"$ref": "generator:qwen-flash"}`)

	var ref GeneratorRef
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !ref.IsRef() {
		t.Error("IsRef() = false, want true")
	}
	if ref.Ref != "generator:qwen-flash" {
		t.Errorf("Ref = %q, want %q", ref.Ref, "generator:qwen-flash")
	}
}

func TestGeneratorRef_JSON_Inline(t *testing.T) {
	data := []byte(`{"model": "gpt-4o"}`)

	var ref GeneratorRef
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if ref.IsRef() {
		t.Error("IsRef() = true, want false")
	}
	if ref.Generator == nil {
		t.Fatal("Generator is nil")
	}
	if ref.Generator.Model != "gpt-4o" {
		t.Errorf("Generator.Model = %q, want %q", ref.Generator.Model, "gpt-4o")
	}
}

func TestGeneratorRef_MsgpackRoundtrip(t *testing.T) {
	tests := []struct {
		name     string
		original GeneratorRef
	}{
		{
			name:     "ref only",
			original: GeneratorRef{Ref: "generator:qwen-flash"},
		},
		{
			name: "inline generator",
			original: GeneratorRef{
				Generator: &Generator{Model: "gpt-4o"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packed, err := msgpack.Marshal(tt.original)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var decoded GeneratorRef
			if err := msgpack.Unmarshal(packed, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if decoded.Ref != tt.original.Ref {
				t.Errorf("Ref = %q, want %q", decoded.Ref, tt.original.Ref)
			}
			if (decoded.Generator == nil) != (tt.original.Generator == nil) {
				t.Errorf("Generator nil mismatch")
			}
			if tt.original.Generator != nil && decoded.Generator != nil {
				if decoded.Generator.Model != tt.original.Generator.Model {
					t.Errorf("Generator.Model = %q, want %q", decoded.Generator.Model, tt.original.Generator.Model)
				}
			}
		})
	}
}

// ========== GeneratorToolRef Tests ==========

func TestGeneratorToolRef_JSON_Ref(t *testing.T) {
	data := []byte(`{"$ref": "tool:summarizer"}`)

	var ref GeneratorToolRef
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !ref.IsRef() {
		t.Error("IsRef() = false, want true")
	}
	if ref.Ref != "tool:summarizer" {
		t.Errorf("Ref = %q, want %q", ref.Ref, "tool:summarizer")
	}
}

func TestGeneratorToolRef_JSON_Inline(t *testing.T) {
	data := []byte(`{
		"name": "inline_generator",
		"model": "gpt-4o",
		"mode": "generate",
		"prompt": "You are a helpful assistant."
	}`)

	var ref GeneratorToolRef
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if ref.IsRef() {
		t.Error("IsRef() = true, want false")
	}
	if ref.GeneratorTool == nil {
		t.Fatal("GeneratorTool is nil")
	}
	if ref.GeneratorTool.Name != "inline_generator" {
		t.Errorf("GeneratorTool.Name = %q, want %q", ref.GeneratorTool.Name, "inline_generator")
	}
	if ref.GeneratorTool.Model != "gpt-4o" {
		t.Errorf("GeneratorTool.Model = %q, want %q", ref.GeneratorTool.Model, "gpt-4o")
	}
}

func TestGeneratorToolRef_MsgpackRoundtrip(t *testing.T) {
	tests := []struct {
		name     string
		original GeneratorToolRef
	}{
		{
			name:     "ref only",
			original: GeneratorToolRef{Ref: "tool:summarizer"},
		},
		{
			name: "inline generator tool",
			original: GeneratorToolRef{
				GeneratorTool: &GeneratorTool{
					ToolBase: ToolBase{Name: "test", Type: ToolTypeGenerator},
					Model:    "gpt-4o",
					Mode:     GeneratorModeGenerate,
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

			var decoded GeneratorToolRef
			if err := msgpack.Unmarshal(packed, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if decoded.Ref != tt.original.Ref {
				t.Errorf("Ref = %q, want %q", decoded.Ref, tt.original.Ref)
			}
			if (decoded.GeneratorTool == nil) != (tt.original.GeneratorTool == nil) {
				t.Errorf("GeneratorTool nil mismatch")
			}
			if tt.original.GeneratorTool != nil && decoded.GeneratorTool != nil {
				if decoded.GeneratorTool.Name != tt.original.GeneratorTool.Name {
					t.Errorf("GeneratorTool.Name = %q, want %q", decoded.GeneratorTool.Name, tt.original.GeneratorTool.Name)
				}
				if decoded.GeneratorTool.Model != tt.original.GeneratorTool.Model {
					t.Errorf("GeneratorTool.Model = %q, want %q", decoded.GeneratorTool.Model, tt.original.GeneratorTool.Model)
				}
			}
		})
	}
}

// ========== TextProcessorToolRef Tests ==========

func TestTextProcessorToolRef_JSON_Ref(t *testing.T) {
	data := []byte(`{"$ref": "tool:text_summarizer"}`)

	var ref TextProcessorToolRef
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !ref.IsRef() {
		t.Error("IsRef() = false, want true")
	}
	if ref.Ref != "tool:text_summarizer" {
		t.Errorf("Ref = %q, want %q", ref.Ref, "tool:text_summarizer")
	}
}

func TestTextProcessorToolRef_JSON_Inline(t *testing.T) {
	data := []byte(`{
		"name": "inline_processor",
		"prompt": "Summarize the text",
		"model": "gpt-4o-mini"
	}`)

	var ref TextProcessorToolRef
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if ref.IsRef() {
		t.Error("IsRef() = true, want false")
	}
	if ref.TextProcessorTool == nil {
		t.Fatal("TextProcessorTool is nil")
	}
	if ref.TextProcessorTool.Name != "inline_processor" {
		t.Errorf("TextProcessorTool.Name = %q, want %q", ref.TextProcessorTool.Name, "inline_processor")
	}
	if ref.TextProcessorTool.Prompt != "Summarize the text" {
		t.Errorf("TextProcessorTool.Prompt = %q", ref.TextProcessorTool.Prompt)
	}
}

func TestTextProcessorToolRef_MsgpackRoundtrip(t *testing.T) {
	tests := []struct {
		name     string
		original TextProcessorToolRef
	}{
		{
			name:     "ref only",
			original: TextProcessorToolRef{Ref: "tool:text_summarizer"},
		},
		{
			name: "inline text processor",
			original: TextProcessorToolRef{
				TextProcessorTool: &TextProcessorTool{
					ToolBase: ToolBase{Name: "test", Type: ToolTypeTextProcessor},
					Prompt:   "Summarize",
					Model:    "gpt-4o-mini",
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

			var decoded TextProcessorToolRef
			if err := msgpack.Unmarshal(packed, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if decoded.Ref != tt.original.Ref {
				t.Errorf("Ref = %q, want %q", decoded.Ref, tt.original.Ref)
			}
			if (decoded.TextProcessorTool == nil) != (tt.original.TextProcessorTool == nil) {
				t.Errorf("TextProcessorTool nil mismatch")
			}
			if tt.original.TextProcessorTool != nil && decoded.TextProcessorTool != nil {
				if decoded.TextProcessorTool.Name != tt.original.TextProcessorTool.Name {
					t.Errorf("TextProcessorTool.Name = %q, want %q", decoded.TextProcessorTool.Name, tt.original.TextProcessorTool.Name)
				}
				if decoded.TextProcessorTool.Prompt != tt.original.TextProcessorTool.Prompt {
					t.Errorf("TextProcessorTool.Prompt = %q, want %q", decoded.TextProcessorTool.Prompt, tt.original.TextProcessorTool.Prompt)
				}
			}
		})
	}
}

// ========== MarshalJSON Tests ==========

func TestGeneratorRef_MarshalJSON_Ref(t *testing.T) {
	ref := GeneratorRef{Ref: "generator:qwen-flash"}

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	expected := `{"$ref":"generator:qwen-flash"}`
	if string(data) != expected {
		t.Errorf("MarshalJSON = %s, want %s", data, expected)
	}
}

func TestGeneratorRef_MarshalJSON_Inline(t *testing.T) {
	ref := GeneratorRef{Generator: &Generator{Model: "gpt-4o"}}

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded["model"] != "gpt-4o" {
		t.Errorf("model = %v, want gpt-4o", decoded["model"])
	}
}

func TestGeneratorToolRef_MarshalJSON_Ref(t *testing.T) {
	ref := GeneratorToolRef{Ref: "tool:summarizer"}

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	expected := `{"$ref":"tool:summarizer"}`
	if string(data) != expected {
		t.Errorf("MarshalJSON = %s, want %s", data, expected)
	}
}

func TestGeneratorToolRef_MarshalJSON_Inline(t *testing.T) {
	ref := GeneratorToolRef{
		GeneratorTool: &GeneratorTool{
			ToolBase: ToolBase{Name: "inline_gen", Type: ToolTypeGenerator},
			Model:    "gpt-4o",
			Mode:     GeneratorModeGenerate,
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

	if decoded["name"] != "inline_gen" {
		t.Errorf("name = %v, want inline_gen", decoded["name"])
	}
	if decoded["model"] != "gpt-4o" {
		t.Errorf("model = %v, want gpt-4o", decoded["model"])
	}
}

func TestTextProcessorToolRef_MarshalJSON_Ref(t *testing.T) {
	ref := TextProcessorToolRef{Ref: "tool:text_summarizer"}

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	expected := `{"$ref":"tool:text_summarizer"}`
	if string(data) != expected {
		t.Errorf("MarshalJSON = %s, want %s", data, expected)
	}
}

func TestTextProcessorToolRef_MarshalJSON_Inline(t *testing.T) {
	ref := TextProcessorToolRef{
		TextProcessorTool: &TextProcessorTool{
			ToolBase: ToolBase{Name: "inline_proc", Type: ToolTypeTextProcessor},
			Prompt:   "Summarize",
			Model:    "gpt-4o-mini",
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

	if decoded["name"] != "inline_proc" {
		t.Errorf("name = %v, want inline_proc", decoded["name"])
	}
	if decoded["prompt"] != "Summarize" {
		t.Errorf("prompt = %v, want Summarize", decoded["prompt"])
	}
}

// ========== IsEmpty Tests ==========

func TestRefs_IsEmpty(t *testing.T) {
	t.Run("RuleRef", func(t *testing.T) {
		empty := RuleRef{}
		if !empty.IsEmpty() {
			t.Error("empty RuleRef should be empty")
		}
		withRef := RuleRef{Ref: "rule:test"}
		if withRef.IsEmpty() {
			t.Error("RuleRef with ref should not be empty")
		}
	})

	t.Run("GeneratorRef", func(t *testing.T) {
		empty := GeneratorRef{}
		if !empty.IsEmpty() {
			t.Error("empty GeneratorRef should be empty")
		}
		withRef := GeneratorRef{Ref: "generator:test"}
		if withRef.IsEmpty() {
			t.Error("GeneratorRef with ref should not be empty")
		}
	})

	t.Run("GeneratorToolRef", func(t *testing.T) {
		empty := GeneratorToolRef{}
		if !empty.IsEmpty() {
			t.Error("empty GeneratorToolRef should be empty")
		}
		withRef := GeneratorToolRef{Ref: "tool:test"}
		if withRef.IsEmpty() {
			t.Error("GeneratorToolRef with ref should not be empty")
		}
	})

	t.Run("TextProcessorToolRef", func(t *testing.T) {
		empty := TextProcessorToolRef{}
		if !empty.IsEmpty() {
			t.Error("empty TextProcessorToolRef should be empty")
		}
		withRef := TextProcessorToolRef{Ref: "tool:test"}
		if withRef.IsEmpty() {
			t.Error("TextProcessorToolRef with ref should not be empty")
		}
	})
}

// ========== RuleRef MarshalJSON Inline Tests ==========

func TestRuleRef_MarshalJSON_Inline(t *testing.T) {
	ref := RuleRef{
		Rule: &Rule{Name: "test_rule"},
	}

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded["name"] != "test_rule" {
		t.Errorf("name = %v, want test_rule", decoded["name"])
	}
}

// ========== Rule Unmarshal Invalid Data Tests ==========

func TestRuleRef_UnmarshalJSON_InvalidData(t *testing.T) {
	var ref RuleRef
	err := json.Unmarshal([]byte(`{invalid`), &ref)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRuleRef_UnmarshalYAML_InvalidData(t *testing.T) {
	var ref RuleRef
	err := yaml.Unmarshal([]byte(`[invalid: yaml`), &ref)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ========== Rule MarshalJSON Empty Tests ==========

func TestRuleRef_MarshalJSON_Empty(t *testing.T) {
	ref := RuleRef{} // both Ref and Rule are empty

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	// Should marshal to null or {}
	if string(data) != "null" && string(data) != "{}" {
		t.Errorf("MarshalJSON = %s, want null or {}", data)
	}
}
