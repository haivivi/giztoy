package agentcfg

import (
	"encoding/json"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
	"gopkg.in/yaml.v3"
)

func TestJQExpr_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantExpr string
		wantErr  bool
	}{
		{
			name:     "simple expression",
			json:     `".data"`,
			wantExpr: ".data",
		},
		{
			name:     "complex expression",
			json:     `"{query: .query, limit: 10}"`,
			wantExpr: "{query: .query, limit: 10}",
		},
		{
			name:     "empty expression",
			json:     `""`,
			wantExpr: "",
		},
		{
			name:    "invalid jq",
			json:    `"invalid { jq"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var expr JQExpr
			err := json.Unmarshal([]byte(tt.json), &expr)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if expr.Expr != tt.wantExpr {
				t.Errorf("Expr = %q, want %q", expr.Expr, tt.wantExpr)
			}
			// Non-empty expressions should have Query set
			if tt.wantExpr != "" && expr.Query == nil {
				t.Error("Query is nil for non-empty expression")
			}
		})
	}
}

func TestJQExpr_Run(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		input   any
		want    string
		wantErr bool
	}{
		{
			name:  "simple field",
			expr:  ".name",
			input: map[string]any{"name": "test"},
			want:  `"test"`,
		},
		{
			name:  "nested field",
			expr:  ".data.value",
			input: map[string]any{"data": map[string]any{"value": 42}},
			want:  `42`,
		},
		{
			name:  "transform",
			expr:  "{x: .a, y: .b}",
			input: map[string]any{"a": 1, "b": 2},
			want:  `{"x":1,"y":2}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var expr JQExpr
			if err := json.Unmarshal([]byte(`"`+tt.expr+`"`), &expr); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			result, err := expr.Run(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if result != tt.want {
				t.Errorf("Run() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestJQExpr_MsgpackRoundtrip(t *testing.T) {
	original := JQExpr{Expr: ".data.value"}
	// Need to parse the expression
	if err := json.Unmarshal([]byte(`".data.value"`), &original); err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Encode
	data, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Decode
	var result JQExpr
	if err := msgpack.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if result.Expr != original.Expr {
		t.Errorf("Expr = %q, want %q", result.Expr, original.Expr)
	}
	if result.Query == nil {
		t.Error("Query is nil after decode")
	}
}

func TestJSONSchema_JSONRoundtrip(t *testing.T) {
	jsonStr := `{"type":"object","properties":{"name":{"type":"string"}}}`

	var schema JSONSchema
	if err := json.Unmarshal([]byte(jsonStr), &schema); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if schema.Schema == nil {
		t.Fatal("Schema is nil")
	}

	// Marshal back
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Should produce equivalent JSON
	var m1, m2 map[string]any
	json.Unmarshal([]byte(jsonStr), &m1)
	json.Unmarshal(data, &m2)

	if m1["type"] != m2["type"] {
		t.Errorf("type mismatch: %v vs %v", m1["type"], m2["type"])
	}
}

func TestJSONSchema_MsgpackRoundtrip(t *testing.T) {
	jsonStr := `{"type":"object","properties":{"name":{"type":"string"}}}`

	var original JSONSchema
	if err := json.Unmarshal([]byte(jsonStr), &original); err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Encode
	data, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Decode
	var result JSONSchema
	if err := msgpack.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if result.Schema == nil {
		t.Fatal("Schema is nil after decode")
	}
}

// ========== JQExpr Marshal Tests ==========

func TestJQExpr_MarshalJSON(t *testing.T) {
	var expr JQExpr
	if err := json.Unmarshal([]byte(`".data.value"`), &expr); err != nil {
		t.Fatalf("setup: %v", err)
	}

	data, err := json.Marshal(expr)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	if string(data) != `".data.value"` {
		t.Errorf("MarshalJSON = %s, want %q", data, ".data.value")
	}
}

func TestJQExpr_MarshalYAML(t *testing.T) {
	var expr JQExpr
	if err := json.Unmarshal([]byte(`".data.value"`), &expr); err != nil {
		t.Fatalf("setup: %v", err)
	}

	result, err := expr.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}

	if result != ".data.value" {
		t.Errorf("MarshalYAML = %v, want %q", result, ".data.value")
	}
}

func TestJQExpr_UnmarshalYAML(t *testing.T) {
	yamlData := `.data.value`

	var expr JQExpr
	if err := yaml.Unmarshal([]byte(yamlData), &expr); err != nil {
		t.Fatalf("UnmarshalYAML: %v", err)
	}

	if expr.Expr != ".data.value" {
		t.Errorf("Expr = %q, want %q", expr.Expr, ".data.value")
	}
	if expr.Query == nil {
		t.Error("Query is nil after UnmarshalYAML")
	}
}

func TestJQExpr_YAMLRoundtrip(t *testing.T) {
	var original JQExpr
	if err := json.Unmarshal([]byte(`"{query: .q, limit: 10}"`), &original); err != nil {
		t.Fatalf("setup: %v", err)
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded JQExpr
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Expr != original.Expr {
		t.Errorf("Expr = %q, want %q", decoded.Expr, original.Expr)
	}
}

// ========== JSONSchema Msgpack Nil Tests ==========

func TestJSONSchema_MarshalMsgpack_Nil(t *testing.T) {
	var schema JSONSchema // Schema is nil
	data, err := msgpack.Marshal(schema)
	if err != nil {
		t.Fatalf("MarshalMsgpack: %v", err)
	}

	var decoded JSONSchema
	if err := msgpack.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("UnmarshalMsgpack: %v", err)
	}

	if decoded.Schema != nil {
		t.Error("decoded Schema should be nil")
	}
}

// ========== JQExpr Run Error Tests ==========

func TestJQExpr_Run_Nil(t *testing.T) {
	var expr *JQExpr = nil
	result, err := expr.Run(map[string]any{"test": 1})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestJQExpr_Run_NoResult(t *testing.T) {
	var expr JQExpr
	if err := json.Unmarshal([]byte(`"empty"`), &expr); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// empty iterator produces no results
	_, err := expr.Run(map[string]any{})
	if err == nil || err.Error() != "jq expression returned no result" {
		t.Errorf("expected 'no result' error, got: %v", err)
	}
}

func TestJQExpr_Run_NilQuery(t *testing.T) {
	expr := &JQExpr{Expr: ".test"} // Query is nil
	result, err := expr.Run(map[string]any{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}


// ========== JQExpr YAML Error Tests ==========

func TestJQExpr_UnmarshalYAML_Invalid(t *testing.T) {
	yamlData := `"invalid { jq"`

	var expr JQExpr
	err := yaml.Unmarshal([]byte(yamlData), &expr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ========== JQExpr Msgpack Error Tests ==========

func TestJQExpr_DecodeMsgpack_Invalid(t *testing.T) {
	// Create msgpack with invalid jq expression
	data, _ := msgpack.Marshal("invalid { jq")

	var expr JQExpr
	err := msgpack.Unmarshal(data, &expr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestJQExpr_DecodeMsgpack_Empty(t *testing.T) {
	data, _ := msgpack.Marshal("")

	var expr JQExpr
	if err := msgpack.Unmarshal(data, &expr); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if expr.Expr != "" {
		t.Errorf("Expr = %q, want empty", expr.Expr)
	}
	if expr.Query != nil {
		t.Error("Query should be nil for empty expression")
	}
}

// ========== JSONSchema Msgpack Error Tests ==========

func TestJSONSchema_UnmarshalMsgpack_InvalidData(t *testing.T) {
	var schema JSONSchema
	err := msgpack.Unmarshal([]byte{0xff}, &schema)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ========== JQExpr Unmarshal Error Tests ==========

func TestJQExpr_UnmarshalJSON_InvalidData(t *testing.T) {
	var expr JQExpr
	err := json.Unmarshal([]byte(`{not a string}`), &expr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestJQExpr_DecodeMsgpack_InvalidData(t *testing.T) {
	var expr JQExpr
	err := msgpack.Unmarshal([]byte{0xff}, &expr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ========== JQExpr YAML Unmarshal Tests ==========

func TestJQExpr_UnmarshalYAML_Valid(t *testing.T) {
	var expr JQExpr
	err := yaml.Unmarshal([]byte(`".test"`), &expr)
	if err != nil {
		t.Fatalf("UnmarshalYAML: %v", err)
	}
	if expr.Expr != ".test" {
		t.Errorf("Expr = %q, want .test", expr.Expr)
	}
}

func TestJQExpr_UnmarshalYAML_InvalidData(t *testing.T) {
	var expr JQExpr
	err := yaml.Unmarshal([]byte(`[not a string]`), &expr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
