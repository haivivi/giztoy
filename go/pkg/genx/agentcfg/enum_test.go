package agentcfg

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

// ========== ToolType Tests ==========

func TestToolType_IsValid(t *testing.T) {
	valid := []ToolType{"", ToolTypeBuiltIn, ToolTypeHTTP, ToolTypeGenerator, ToolTypeComposite, ToolTypeTextProcessor}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("ToolType(%q).IsValid() = false, want true", v)
		}
	}

	invalid := []ToolType{"unknown", "invalid", "foo"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("ToolType(%q).IsValid() = true, want false", v)
		}
	}
}

func TestToolType_UnmarshalJSON_Invalid(t *testing.T) {
	var tt ToolType
	err := json.Unmarshal([]byte(`"invalid_type"`), &tt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid tool type") {
		t.Errorf("error = %q, want contains 'invalid tool type'", err.Error())
	}
}

func TestToolType_UnmarshalMsgpack_Invalid(t *testing.T) {
	data, _ := msgpack.Marshal("invalid_type")
	var tt ToolType
	err := msgpack.Unmarshal(data, &tt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid tool type") {
		t.Errorf("error = %q, want contains 'invalid tool type'", err.Error())
	}
}

// ========== AgentType Tests ==========

func TestAgentType_IsValid(t *testing.T) {
	valid := []AgentType{"", AgentTypeReAct, AgentTypeMatch}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("AgentType(%q).IsValid() = false, want true", v)
		}
	}

	invalid := []AgentType{"unknown", "router", "foo"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("AgentType(%q).IsValid() = true, want false", v)
		}
	}
}

func TestAgentType_UnmarshalJSON_Invalid(t *testing.T) {
	var at AgentType
	err := json.Unmarshal([]byte(`"invalid_type"`), &at)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid agent type") {
		t.Errorf("error = %q, want contains 'invalid agent type'", err.Error())
	}
}

func TestAgentType_UnmarshalMsgpack_Invalid(t *testing.T) {
	data, _ := msgpack.Marshal("invalid_type")
	var at AgentType
	err := msgpack.Unmarshal(data, &at)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid agent type") {
		t.Errorf("error = %q, want contains 'invalid agent type'", err.Error())
	}
}

// ========== MessageRole Tests ==========

func TestMessageRole_IsValid(t *testing.T) {
	valid := []MessageRole{RoleUser, RoleModel, RoleTool}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("MessageRole(%q).IsValid() = false, want true", v)
		}
	}

	invalid := []MessageRole{"", "system", "assistant", "foo"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("MessageRole(%q).IsValid() = true, want false", v)
		}
	}
}

func TestMessageRole_UnmarshalJSON_Invalid(t *testing.T) {
	var mr MessageRole
	err := json.Unmarshal([]byte(`"invalid_role"`), &mr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid message role") {
		t.Errorf("error = %q, want contains 'invalid message role'", err.Error())
	}
}

func TestMessageRole_UnmarshalMsgpack_Invalid(t *testing.T) {
	data, _ := msgpack.Marshal("invalid_role")
	var mr MessageRole
	err := msgpack.Unmarshal(data, &mr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid message role") {
		t.Errorf("error = %q, want contains 'invalid message role'", err.Error())
	}
}

// ========== StateType Tests ==========

func TestStateType_IsValid(t *testing.T) {
	valid := []StateType{StateTypeReAct, StateTypeMatch}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("StateType(%q).IsValid() = false, want true", v)
		}
	}

	invalid := []StateType{"", "unknown", "foo"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("StateType(%q).IsValid() = true, want false", v)
		}
	}
}

func TestStateType_UnmarshalJSON_Invalid(t *testing.T) {
	var st StateType
	err := json.Unmarshal([]byte(`"invalid_type"`), &st)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid state type") {
		t.Errorf("error = %q, want contains 'invalid state type'", err.Error())
	}
}

func TestStateType_UnmarshalMsgpack_Invalid(t *testing.T) {
	data, _ := msgpack.Marshal("invalid_type")
	var st StateType
	err := msgpack.Unmarshal(data, &st)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid state type") {
		t.Errorf("error = %q, want contains 'invalid state type'", err.Error())
	}
}

// ========== GeneratorMode Tests ==========

func TestGeneratorMode_IsValid(t *testing.T) {
	valid := []GeneratorMode{"", GeneratorModeGenerate, GeneratorModeJSONOutput}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("GeneratorMode(%q).IsValid() = false, want true", v)
		}
	}

	invalid := []GeneratorMode{"unknown", "stream", "foo"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("GeneratorMode(%q).IsValid() = true, want false", v)
		}
	}
}

func TestGeneratorMode_UnmarshalJSON_Invalid(t *testing.T) {
	var gm GeneratorMode
	err := json.Unmarshal([]byte(`"invalid_mode"`), &gm)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid generator mode") {
		t.Errorf("error = %q, want contains 'invalid generator mode'", err.Error())
	}
}

func TestGeneratorMode_UnmarshalMsgpack_Invalid(t *testing.T) {
	data, _ := msgpack.Marshal("invalid_mode")
	var gm GeneratorMode
	err := msgpack.Unmarshal(data, &gm)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid generator mode") {
		t.Errorf("error = %q, want contains 'invalid generator mode'", err.Error())
	}
}

// ========== TextProcessorOutputMode Tests ==========

func TestTextProcessorOutputMode_IsValid(t *testing.T) {
	valid := []TextProcessorOutputMode{"", TextProcessorOutputText, TextProcessorOutputJSON}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("TextProcessorOutputMode(%q).IsValid() = false, want true", v)
		}
	}

	invalid := []TextProcessorOutputMode{"unknown", "xml", "foo"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("TextProcessorOutputMode(%q).IsValid() = true, want false", v)
		}
	}
}

func TestTextProcessorOutputMode_UnmarshalJSON_Invalid(t *testing.T) {
	var om TextProcessorOutputMode
	err := json.Unmarshal([]byte(`"invalid_mode"`), &om)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid text processor output mode") {
		t.Errorf("error = %q, want contains 'invalid text processor output mode'", err.Error())
	}
}

func TestTextProcessorOutputMode_UnmarshalMsgpack_Invalid(t *testing.T) {
	data, _ := msgpack.Marshal("invalid_mode")
	var om TextProcessorOutputMode
	err := msgpack.Unmarshal(data, &om)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid text processor output mode") {
		t.Errorf("error = %q, want contains 'invalid text processor output mode'", err.Error())
	}
}

// ========== HTTPMethod Tests ==========

func TestHTTPMethod_IsValid(t *testing.T) {
	valid := []HTTPMethod{HTTPMethodGET, HTTPMethodPOST, HTTPMethodPUT, HTTPMethodDELETE, HTTPMethodPATCH}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("HTTPMethod(%q).IsValid() = false, want true", v)
		}
	}

	invalid := []HTTPMethod{"", "HEAD", "OPTIONS", "CONNECT", "foo"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("HTTPMethod(%q).IsValid() = true, want false", v)
		}
	}
}

func TestHTTPMethod_UnmarshalJSON_Invalid(t *testing.T) {
	var hm HTTPMethod
	err := json.Unmarshal([]byte(`"HEAD"`), &hm)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid HTTP method") {
		t.Errorf("error = %q, want contains 'invalid HTTP method'", err.Error())
	}
}

func TestHTTPMethod_UnmarshalMsgpack_Invalid(t *testing.T) {
	data, _ := msgpack.Marshal("HEAD")
	var hm HTTPMethod
	err := msgpack.Unmarshal(data, &hm)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid HTTP method") {
		t.Errorf("error = %q, want contains 'invalid HTTP method'", err.Error())
	}
}

// ========== HTTPAuthType Tests ==========

func TestHTTPAuthType_IsValid(t *testing.T) {
	valid := []HTTPAuthType{HTTPAuthTypeBearer, HTTPAuthTypeBasic}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("HTTPAuthType(%q).IsValid() = false, want true", v)
		}
	}

	invalid := []HTTPAuthType{"", "digest", "oauth", "foo"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("HTTPAuthType(%q).IsValid() = true, want false", v)
		}
	}
}

func TestHTTPAuthType_UnmarshalJSON_Invalid(t *testing.T) {
	var at HTTPAuthType
	err := json.Unmarshal([]byte(`"digest"`), &at)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid HTTP auth type") {
		t.Errorf("error = %q, want contains 'invalid HTTP auth type'", err.Error())
	}
}

func TestHTTPAuthType_UnmarshalMsgpack_Invalid(t *testing.T) {
	data, _ := msgpack.Marshal("digest")
	var at HTTPAuthType
	err := msgpack.Unmarshal(data, &at)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid HTTP auth type") {
		t.Errorf("error = %q, want contains 'invalid HTTP auth type'", err.Error())
	}
}

// ========== CompositeMode Tests ==========

func TestCompositeMode_IsValid(t *testing.T) {
	valid := []CompositeMode{"", CompositeModeSeq}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("CompositeMode(%q).IsValid() = false, want true", v)
		}
	}

	invalid := []CompositeMode{"parallel", "unknown", "foo"}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("CompositeMode(%q).IsValid() = true, want false", v)
		}
	}
}

func TestCompositeMode_UnmarshalJSON_Invalid(t *testing.T) {
	var cm CompositeMode
	err := json.Unmarshal([]byte(`"parallel"`), &cm)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid composite mode") {
		t.Errorf("error = %q, want contains 'invalid composite mode'", err.Error())
	}
}

func TestCompositeMode_UnmarshalMsgpack_Invalid(t *testing.T) {
	data, _ := msgpack.Marshal("parallel")
	var cm CompositeMode
	err := msgpack.Unmarshal(data, &cm)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid composite mode") {
		t.Errorf("error = %q, want contains 'invalid composite mode'", err.Error())
	}
}

// ========== Enum Unmarshal Invalid Data Tests ==========

func TestToolType_UnmarshalJSON_InvalidData(t *testing.T) {
	var tt ToolType
	err := json.Unmarshal([]byte(`123`), &tt) // not a string
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestToolType_UnmarshalMsgpack_InvalidData(t *testing.T) {
	data, _ := msgpack.Marshal(123) // not a string
	var tt ToolType
	err := msgpack.Unmarshal(data, &tt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAgentType_UnmarshalJSON_InvalidData(t *testing.T) {
	var at AgentType
	err := json.Unmarshal([]byte(`123`), &at)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAgentType_UnmarshalMsgpack_InvalidData(t *testing.T) {
	data, _ := msgpack.Marshal(123)
	var at AgentType
	err := msgpack.Unmarshal(data, &at)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMessageRole_UnmarshalJSON_InvalidData(t *testing.T) {
	var mr MessageRole
	err := json.Unmarshal([]byte(`123`), &mr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMessageRole_UnmarshalMsgpack_InvalidData(t *testing.T) {
	data, _ := msgpack.Marshal(123)
	var mr MessageRole
	err := msgpack.Unmarshal(data, &mr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestStateType_UnmarshalJSON_InvalidData(t *testing.T) {
	var st StateType
	err := json.Unmarshal([]byte(`123`), &st)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestStateType_UnmarshalMsgpack_InvalidData(t *testing.T) {
	data, _ := msgpack.Marshal(123)
	var st StateType
	err := msgpack.Unmarshal(data, &st)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGeneratorMode_UnmarshalJSON_InvalidData(t *testing.T) {
	var gm GeneratorMode
	err := json.Unmarshal([]byte(`123`), &gm)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGeneratorMode_UnmarshalMsgpack_InvalidData(t *testing.T) {
	data, _ := msgpack.Marshal(123)
	var gm GeneratorMode
	err := msgpack.Unmarshal(data, &gm)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTextProcessorOutputMode_UnmarshalJSON_InvalidData(t *testing.T) {
	var om TextProcessorOutputMode
	err := json.Unmarshal([]byte(`123`), &om)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTextProcessorOutputMode_UnmarshalMsgpack_InvalidData(t *testing.T) {
	data, _ := msgpack.Marshal(123)
	var om TextProcessorOutputMode
	err := msgpack.Unmarshal(data, &om)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestHTTPMethod_UnmarshalJSON_InvalidData(t *testing.T) {
	var hm HTTPMethod
	err := json.Unmarshal([]byte(`123`), &hm)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestHTTPMethod_UnmarshalMsgpack_InvalidData(t *testing.T) {
	data, _ := msgpack.Marshal(123)
	var hm HTTPMethod
	err := msgpack.Unmarshal(data, &hm)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestHTTPAuthType_UnmarshalJSON_InvalidData(t *testing.T) {
	var at HTTPAuthType
	err := json.Unmarshal([]byte(`123`), &at)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestHTTPAuthType_UnmarshalMsgpack_InvalidData(t *testing.T) {
	data, _ := msgpack.Marshal(123)
	var at HTTPAuthType
	err := msgpack.Unmarshal(data, &at)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCompositeMode_UnmarshalJSON_InvalidData(t *testing.T) {
	var cm CompositeMode
	err := json.Unmarshal([]byte(`123`), &cm)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCompositeMode_UnmarshalMsgpack_InvalidData(t *testing.T) {
	data, _ := msgpack.Marshal(123)
	var cm CompositeMode
	err := msgpack.Unmarshal(data, &cm)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
