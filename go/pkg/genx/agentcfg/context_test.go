package agentcfg

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
	"gopkg.in/yaml.v3"
)

func TestContextLayer_UnmarshalJSON_String(t *testing.T) {
	data := []byte(`"You are a helpful assistant."`)

	var layer ContextLayer
	if err := json.Unmarshal(data, &layer); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if layer.Prompt != "You are a helpful assistant." {
		t.Errorf("Prompt = %q, want %q", layer.Prompt, "You are a helpful assistant.")
	}
	if layer.This != "" || layer.Ref != "" || layer.Env != "" || layer.Mem != nil {
		t.Error("other fields should be empty for string form")
	}
}

func TestContextLayer_UnmarshalJSON_This(t *testing.T) {
	data := []byte(`{"$this": ".prompt"}`)

	var layer ContextLayer
	if err := json.Unmarshal(data, &layer); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if layer.This != ".prompt" {
		t.Errorf("This = %q, want %q", layer.This, ".prompt")
	}
}

func TestContextLayer_UnmarshalJSON_Ref(t *testing.T) {
	data := []byte(`{"$ref": "character:elsa"}`)

	var layer ContextLayer
	if err := json.Unmarshal(data, &layer); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if layer.Ref != "character:elsa" {
		t.Errorf("Ref = %q, want %q", layer.Ref, "character:elsa")
	}
}

func TestContextLayer_UnmarshalJSON_Env(t *testing.T) {
	data := []byte(`{"$env": "SYSTEM_PROMPT"}`)

	var layer ContextLayer
	if err := json.Unmarshal(data, &layer); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if layer.Env != "SYSTEM_PROMPT" {
		t.Errorf("Env = %q, want %q", layer.Env, "SYSTEM_PROMPT")
	}
}

func TestContextLayer_UnmarshalJSON_Mem(t *testing.T) {
	data := []byte(`{"$mem": {"summary": true, "recent": 5}}`)

	var layer ContextLayer
	if err := json.Unmarshal(data, &layer); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if layer.Mem == nil {
		t.Fatal("Mem is nil")
	}
	if !layer.Mem.Summary {
		t.Error("Mem.Summary = false, want true")
	}
	if layer.Mem.Recent != 5 {
		t.Errorf("Mem.Recent = %d, want 5", layer.Mem.Recent)
	}
}

func TestContextLayer_MarshalJSON_String(t *testing.T) {
	layer := ContextLayer{Prompt: "Test prompt"}

	data, err := json.Marshal(layer)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if string(data) != `"Test prompt"` {
		t.Errorf("got %s, want %q", string(data), `"Test prompt"`)
	}
}

func TestContextLayer_MarshalJSON_Object(t *testing.T) {
	layer := ContextLayer{Ref: "character:elsa"}

	data, err := json.Marshal(layer)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}

	if result["$ref"] != "character:elsa" {
		t.Errorf("$ref = %v, want %q", result["$ref"], "character:elsa")
	}
}

func TestContextLayers_InAgent(t *testing.T) {
	data := []byte(`{
		"name": "test",
		"context_layers": [
			"You are an AI assistant.",
			{"$ref": "personality:friendly"},
			{"$env": "CUSTOM_PROMPT"},
			{"$mem": {"summary": true}}
		]
	}`)

	agent, err := UnmarshalAgent(data)
	if err != nil {
		t.Fatalf("UnmarshalAgent: %v", err)
	}

	react := AsReActAgent(agent)
	if react == nil {
		t.Fatal("AsReActAgent returned nil")
	}

	if len(react.ContextLayers) != 4 {
		t.Fatalf("len(ContextLayers) = %d, want 4", len(react.ContextLayers))
	}

	// Check each layer type
	if react.ContextLayers[0].Prompt != "You are an AI assistant." {
		t.Errorf("layer[0].Prompt = %q", react.ContextLayers[0].Prompt)
	}
	if react.ContextLayers[1].Ref != "personality:friendly" {
		t.Errorf("layer[1].Ref = %q", react.ContextLayers[1].Ref)
	}
	if react.ContextLayers[2].Env != "CUSTOM_PROMPT" {
		t.Errorf("layer[2].Env = %q", react.ContextLayers[2].Env)
	}
	if react.ContextLayers[3].Mem == nil || !react.ContextLayers[3].Mem.Summary {
		t.Errorf("layer[3].Mem invalid")
	}
}

// ========== Testdata File Tests ==========

func loadContextTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func loadYAMLContextFile(t *testing.T, path string) []byte {
	t.Helper()
	yamlData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

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

func TestContextLayer_JSON_Testdata(t *testing.T) {
	tests := []struct {
		file     string
		validate func(t *testing.T, c *ContextLayer)
	}{
		{
			file: "testdata/context/this.json",
			validate: func(t *testing.T, c *ContextLayer) {
				if c.This != ".prompt" {
					t.Errorf("This = %q, want %q", c.This, ".prompt")
				}
			},
		},
		{
			file: "testdata/context/ref.json",
			validate: func(t *testing.T, c *ContextLayer) {
				if c.Ref != "character:elsa" {
					t.Errorf("Ref = %q, want %q", c.Ref, "character:elsa")
				}
			},
		},
		{
			file: "testdata/context/env.json",
			validate: func(t *testing.T, c *ContextLayer) {
				if c.Env != "SYSTEM_PROMPT" {
					t.Errorf("Env = %q, want %q", c.Env, "SYSTEM_PROMPT")
				}
			},
		},
		{
			file: "testdata/context/mem.json",
			validate: func(t *testing.T, c *ContextLayer) {
				if c.Mem == nil {
					t.Fatal("Mem is nil")
				}
				if !c.Mem.Summary {
					t.Errorf("Mem.Summary = %v, want true", c.Mem.Summary)
				}
				if !c.Mem.Query {
					t.Errorf("Mem.Query = %v, want true", c.Mem.Query)
				}
				if c.Mem.Recent != 100 {
					t.Errorf("Mem.Recent = %d, want 100", c.Mem.Recent)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			data := loadContextTestFile(t, tt.file)

			var c ContextLayer
			if err := json.Unmarshal(data, &c); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			tt.validate(t, &c)
		})
	}
}

// ========== YAML Tests ==========

func TestContextLayer_YAML_Testdata(t *testing.T) {
	tests := []struct {
		file     string
		validate func(t *testing.T, c *ContextLayer)
	}{
		{
			file: "testdata/context/this.yaml",
			validate: func(t *testing.T, c *ContextLayer) {
				if c.This != ".prompt" {
					t.Errorf("This = %q, want %q", c.This, ".prompt")
				}
			},
		},
		{
			file: "testdata/context/ref.yaml",
			validate: func(t *testing.T, c *ContextLayer) {
				if c.Ref != "character:elsa" {
					t.Errorf("Ref = %q, want %q", c.Ref, "character:elsa")
				}
			},
		},
		{
			file: "testdata/context/env.yaml",
			validate: func(t *testing.T, c *ContextLayer) {
				if c.Env != "SYSTEM_PROMPT" {
					t.Errorf("Env = %q, want %q", c.Env, "SYSTEM_PROMPT")
				}
			},
		},
		{
			file: "testdata/context/mem.yaml",
			validate: func(t *testing.T, c *ContextLayer) {
				if c.Mem == nil {
					t.Fatal("Mem is nil")
				}
				if !c.Mem.Summary {
					t.Errorf("Mem.Summary = %v, want true", c.Mem.Summary)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			data := loadYAMLContextFile(t, tt.file)

			var c ContextLayer
			if err := json.Unmarshal(data, &c); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			tt.validate(t, &c)
		})
	}
}

// ========== YAML Direct Unmarshal Tests ==========

func TestContextLayer_UnmarshalYAML_String(t *testing.T) {
	yamlData := `You are a helpful assistant.`

	var layer ContextLayer
	if err := yaml.Unmarshal([]byte(yamlData), &layer); err != nil {
		t.Fatalf("UnmarshalYAML: %v", err)
	}

	if layer.Prompt != "You are a helpful assistant." {
		t.Errorf("Prompt = %q, want %q", layer.Prompt, "You are a helpful assistant.")
	}
}

func TestContextLayer_UnmarshalYAML_This(t *testing.T) {
	yamlData := `$this: .prompt`

	var layer ContextLayer
	if err := yaml.Unmarshal([]byte(yamlData), &layer); err != nil {
		t.Fatalf("UnmarshalYAML: %v", err)
	}

	if layer.This != ".prompt" {
		t.Errorf("This = %q, want %q", layer.This, ".prompt")
	}
}

func TestContextLayer_UnmarshalYAML_Ref(t *testing.T) {
	yamlData := `$ref: character:elsa`

	var layer ContextLayer
	if err := yaml.Unmarshal([]byte(yamlData), &layer); err != nil {
		t.Fatalf("UnmarshalYAML: %v", err)
	}

	if layer.Ref != "character:elsa" {
		t.Errorf("Ref = %q, want %q", layer.Ref, "character:elsa")
	}
}

func TestContextLayer_UnmarshalYAML_Env(t *testing.T) {
	yamlData := `$env: SYSTEM_PROMPT`

	var layer ContextLayer
	if err := yaml.Unmarshal([]byte(yamlData), &layer); err != nil {
		t.Fatalf("UnmarshalYAML: %v", err)
	}

	if layer.Env != "SYSTEM_PROMPT" {
		t.Errorf("Env = %q, want %q", layer.Env, "SYSTEM_PROMPT")
	}
}

func TestContextLayer_UnmarshalYAML_Mem(t *testing.T) {
	yamlData := `$mem:
  summary: true
  recent: 50`

	var layer ContextLayer
	if err := yaml.Unmarshal([]byte(yamlData), &layer); err != nil {
		t.Fatalf("UnmarshalYAML: %v", err)
	}

	if layer.Mem == nil {
		t.Fatal("Mem is nil")
	}
	if !layer.Mem.Summary {
		t.Error("Mem.Summary = false, want true")
	}
	if layer.Mem.Recent != 50 {
		t.Errorf("Mem.Recent = %d, want 50", layer.Mem.Recent)
	}
}

// ========== MsgPack Tests ==========

func TestContextLayer_MsgpackRoundtrip(t *testing.T) {
	tests := []struct {
		name     string
		original ContextLayer
	}{
		{
			name:     "prompt",
			original: ContextLayer{Prompt: "You are a helpful assistant."},
		},
		{
			name:     "this",
			original: ContextLayer{This: ".prompt"},
		},
		{
			name:     "ref",
			original: ContextLayer{Ref: "character:elsa"},
		},
		{
			name:     "env",
			original: ContextLayer{Env: "SYSTEM_PROMPT"},
		},
		{
			name:     "mem",
			original: ContextLayer{Mem: &MemoryOptions{Summary: true, Query: true, Recent: 100}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packed, err := msgpack.Marshal(tt.original)
			if err != nil {
				t.Fatalf("MsgPack Marshal: %v", err)
			}

			var decoded ContextLayer
			if err := msgpack.Unmarshal(packed, &decoded); err != nil {
				t.Fatalf("MsgPack Unmarshal: %v", err)
			}

			if decoded.Prompt != tt.original.Prompt {
				t.Errorf("Prompt = %q, want %q", decoded.Prompt, tt.original.Prompt)
			}
			if decoded.This != tt.original.This {
				t.Errorf("This = %q, want %q", decoded.This, tt.original.This)
			}
			if decoded.Ref != tt.original.Ref {
				t.Errorf("Ref = %q, want %q", decoded.Ref, tt.original.Ref)
			}
			if decoded.Env != tt.original.Env {
				t.Errorf("Env = %q, want %q", decoded.Env, tt.original.Env)
			}
			if (decoded.Mem == nil) != (tt.original.Mem == nil) {
				t.Errorf("Mem nil mismatch")
			}
			if tt.original.Mem != nil && decoded.Mem != nil {
				if decoded.Mem.Summary != tt.original.Mem.Summary {
					t.Errorf("Mem.Summary = %v, want %v", decoded.Mem.Summary, tt.original.Mem.Summary)
				}
				if decoded.Mem.Recent != tt.original.Mem.Recent {
					t.Errorf("Mem.Recent = %d, want %d", decoded.Mem.Recent, tt.original.Mem.Recent)
				}
			}
		})
	}
}

// ========== ContextLayer Validate Error Tests ==========

func TestContextLayer_Validate_Error_MultipleFields(t *testing.T) {
	data := []byte(`{"$this":".prompt","$ref":"char:test"}`)
	var layer ContextLayer
	err := json.Unmarshal(data, &layer)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "must have") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestContextLayer_Validate_Error_Empty(t *testing.T) {
	data := []byte(`{}`)
	var layer ContextLayer
	err := json.Unmarshal(data, &layer)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "must have") {
		t.Errorf("error = %q", err.Error())
	}
}

// ========== Context Unmarshal Invalid Data Tests ==========

func TestContextLayer_UnmarshalJSON_InvalidData(t *testing.T) {
	var layer ContextLayer
	err := json.Unmarshal([]byte(`{invalid`), &layer)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestContextLayer_UnmarshalMsgpack_InvalidData(t *testing.T) {
	var layer ContextLayer
	err := msgpack.Unmarshal([]byte{0xff}, &layer)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
