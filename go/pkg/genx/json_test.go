package genx

import (
	"testing"
)

func TestUnmarshalJSON_Valid(t *testing.T) {
	data := []byte(`{"name": "test", "value": 123}`)
	var result map[string]any

	err := unmarshalJSON(data, &result)
	if err != nil {
		t.Fatalf("unmarshalJSON error: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("name = %v, want %q", result["name"], "test")
	}

	if result["value"] != float64(123) {
		t.Errorf("value = %v, want 123", result["value"])
	}
}

func TestUnmarshalJSON_ValidArray(t *testing.T) {
	data := []byte(`[1, 2, 3]`)
	var result []int

	err := unmarshalJSON(data, &result)
	if err != nil {
		t.Fatalf("unmarshalJSON error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("len(result) = %d, want 3", len(result))
	}

	expected := []int{1, 2, 3}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("result[%d] = %d, want %d", i, result[i], v)
		}
	}
}

func TestUnmarshalJSON_MalformedJSON_TrailingComma(t *testing.T) {
	// JSON with trailing comma - invalid but repairable
	data := []byte(`{"name": "test", "value": 123,}`)
	var result map[string]any

	err := unmarshalJSON(data, &result)
	if err != nil {
		t.Fatalf("unmarshalJSON should repair trailing comma: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("name = %v, want %q", result["name"], "test")
	}
}

func TestUnmarshalJSON_MalformedJSON_MissingQuotes(t *testing.T) {
	// JSON with unquoted key - invalid but repairable
	data := []byte(`{name: "test"}`)
	var result map[string]any

	err := unmarshalJSON(data, &result)
	if err != nil {
		t.Fatalf("unmarshalJSON should repair unquoted key: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("name = %v, want %q", result["name"], "test")
	}
}

func TestUnmarshalJSON_MalformedJSON_SingleQuotes(t *testing.T) {
	// JSON with single quotes - invalid but repairable
	data := []byte(`{'name': 'test'}`)
	var result map[string]any

	err := unmarshalJSON(data, &result)
	if err != nil {
		t.Fatalf("unmarshalJSON should repair single quotes: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("name = %v, want %q", result["name"], "test")
	}
}

func TestUnmarshalJSON_TypeMismatch(t *testing.T) {
	// Valid JSON but wrong type
	data := []byte(`"string value"`)
	var result int

	err := unmarshalJSON(data, &result)
	if err == nil {
		t.Error("unmarshalJSON should fail on type mismatch")
	}
}

func TestUnmarshalJSON_EmptyObject(t *testing.T) {
	data := []byte(`{}`)
	var result map[string]any

	err := unmarshalJSON(data, &result)
	if err != nil {
		t.Fatalf("unmarshalJSON error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("len(result) = %d, want 0", len(result))
	}
}

func TestUnmarshalJSON_NestedObject(t *testing.T) {
	data := []byte(`{"outer": {"inner": "value"}}`)
	var result map[string]any

	err := unmarshalJSON(data, &result)
	if err != nil {
		t.Fatalf("unmarshalJSON error: %v", err)
	}

	outer, ok := result["outer"].(map[string]any)
	if !ok {
		t.Fatalf("outer type = %T, want map[string]any", result["outer"])
	}

	if outer["inner"] != "value" {
		t.Errorf("inner = %v, want %q", outer["inner"], "value")
	}
}

func TestHexString(t *testing.T) {
	s1 := hexString()
	s2 := hexString()

	// Should be 16 characters (8 bytes = 16 hex chars)
	if len(s1) != 16 {
		t.Errorf("hexString() length = %d, want 16", len(s1))
	}

	if len(s2) != 16 {
		t.Errorf("hexString() length = %d, want 16", len(s2))
	}

	// Should be different each time (with very high probability)
	if s1 == s2 {
		t.Error("hexString() should generate unique strings")
	}

	// Should be valid hex characters
	for _, c := range s1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("hexString() contains invalid character: %c", c)
		}
	}
}
