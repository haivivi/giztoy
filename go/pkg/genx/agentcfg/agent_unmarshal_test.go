package agentcfg

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
	"gopkg.in/yaml.v3"
)

func loadTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func TestUnmarshalAgent_ReActMinimal(t *testing.T) {
	data := loadTestFile(t, "testdata/agent/react_minimal.json")

	agent, err := UnmarshalAgent(data)
	if err != nil {
		t.Fatalf("UnmarshalAgent: %v", err)
	}

	if agent.AgentName() != "assistant" {
		t.Errorf("AgentName() = %q, want %q", agent.AgentName(), "assistant")
	}
	if agent.AgentType() != AgentTypeReAct {
		t.Errorf("AgentType() = %q, want %q", agent.AgentType(), AgentTypeReAct)
	}

	react := AsReActAgent(agent)
	if react == nil {
		t.Fatal("AsReActAgent returned nil")
	}
	if react.Prompt != "You are a helpful assistant." {
		t.Errorf("Prompt = %q, want %q", react.Prompt, "You are a helpful assistant.")
	}
}

func TestUnmarshalAgent_ReActWithTools(t *testing.T) {
	data := loadTestFile(t, "testdata/agent/react_with_tools.json")

	agent, err := UnmarshalAgent(data)
	if err != nil {
		t.Fatalf("UnmarshalAgent: %v", err)
	}

	react := AsReActAgent(agent)
	if react == nil {
		t.Fatal("AsReActAgent returned nil")
	}

	if len(react.Tools) != 2 {
		t.Fatalf("len(Tools) = %d, want 2", len(react.Tools))
	}

	// First tool: reference without quit
	if react.Tools[0].Ref != "play_music" {
		t.Errorf("Tools[0].Ref = %q, want %q", react.Tools[0].Ref, "play_music")
	}
	if react.Tools[0].Quit {
		t.Error("Tools[0].Quit = true, want false")
	}

	// Second tool: reference with quit
	if react.Tools[1].Ref != "stop_music" {
		t.Errorf("Tools[1].Ref = %q, want %q", react.Tools[1].Ref, "stop_music")
	}
	if !react.Tools[1].Quit {
		t.Error("Tools[1].Quit = false, want true")
	}
}

func TestUnmarshalAgent_MatchRouter(t *testing.T) {
	data := loadTestFile(t, "testdata/agent/match_router.json")

	agent, err := UnmarshalAgent(data)
	if err != nil {
		t.Fatalf("UnmarshalAgent: %v", err)
	}

	if agent.AgentName() != "router" {
		t.Errorf("AgentName() = %q, want %q", agent.AgentName(), "router")
	}
	if agent.AgentType() != AgentTypeMatch {
		t.Errorf("AgentType() = %q, want %q", agent.AgentType(), AgentTypeMatch)
	}

	match := AsMatchAgent(agent)
	if match == nil {
		t.Fatal("AsMatchAgent returned nil")
	}

	// Check rules
	if len(match.Rules) != 2 {
		t.Fatalf("len(Rules) = %d, want 2", len(match.Rules))
	}
	if match.Rules[0].Ref != "play_music_rule" {
		t.Errorf("Rules[0].Ref = %q, want %q", match.Rules[0].Ref, "play_music_rule")
	}
	if match.Rules[1].Rule == nil {
		t.Fatal("Rules[1].Rule is nil")
	}
	if match.Rules[1].Rule.Name != "weather" {
		t.Errorf("Rules[1].Rule.Name = %q, want %q", match.Rules[1].Rule.Name, "weather")
	}

	// Check routes
	if len(match.Route) != 2 {
		t.Fatalf("len(Route) = %d, want 2", len(match.Route))
	}

	// Check default
	if match.Default == nil {
		t.Fatal("Default is nil")
	}
	if match.Default.Ref != "fallback" {
		t.Errorf("Default.Ref = %q, want %q", match.Default.Ref, "fallback")
	}
}

func TestAgent_JSONRoundtrip(t *testing.T) {
	original := &ReActAgent{
		AgentBase: AgentBase{
			Name:   "test",
			Prompt: "Test prompt",
		},
		Tools: []ToolRef{
			{Ref: "tool1"},
			{Ref: "tool2", Quit: true},
		},
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Unmarshal
	agent, err := UnmarshalAgent(data)
	if err != nil {
		t.Fatalf("UnmarshalAgent: %v", err)
	}

	result := AsReActAgent(agent)
	if result == nil {
		t.Fatal("AsReActAgent returned nil")
	}

	if result.Name != original.Name {
		t.Errorf("Name = %q, want %q", result.Name, original.Name)
	}
	if result.Prompt != original.Prompt {
		t.Errorf("Prompt = %q, want %q", result.Prompt, original.Prompt)
	}
	if len(result.Tools) != len(original.Tools) {
		t.Fatalf("len(Tools) = %d, want %d", len(result.Tools), len(original.Tools))
	}
}

func TestAgentRef_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantRef string
		isRef   bool
	}{
		{
			name:    "reference",
			json:    `{"$ref": "agent:music_player"}`,
			wantRef: "agent:music_player",
			isRef:   true,
		},
		{
			name:  "inline",
			json:  `{"name": "inline_agent", "prompt": "test"}`,
			isRef: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ref AgentRef
			if err := json.Unmarshal([]byte(tt.json), &ref); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if ref.IsRef() != tt.isRef {
				t.Errorf("IsRef() = %v, want %v", ref.IsRef(), tt.isRef)
			}
			if tt.isRef && ref.Ref != tt.wantRef {
				t.Errorf("Ref = %q, want %q", ref.Ref, tt.wantRef)
			}
		})
	}
}

// ========== YAML Tests ==========

// loadYAMLAgentFile loads a YAML file and converts to JSON for UnmarshalAgent.
func loadYAMLAgentFile(t *testing.T, path string) []byte {
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

func TestUnmarshalAgent_YAML_ReActMinimal(t *testing.T) {
	data := loadYAMLAgentFile(t, "testdata/agent/react_minimal.yaml")

	agent, err := UnmarshalAgent(data)
	if err != nil {
		t.Fatalf("UnmarshalAgent: %v", err)
	}

	if agent.AgentName() != "assistant" {
		t.Errorf("AgentName() = %q, want %q", agent.AgentName(), "assistant")
	}
	if agent.AgentType() != AgentTypeReAct {
		t.Errorf("AgentType() = %q, want %q", agent.AgentType(), AgentTypeReAct)
	}
}

func TestUnmarshalAgent_YAML_ReActWithTools(t *testing.T) {
	data := loadYAMLAgentFile(t, "testdata/agent/react_with_tools.yaml")

	agent, err := UnmarshalAgent(data)
	if err != nil {
		t.Fatalf("UnmarshalAgent: %v", err)
	}

	react := AsReActAgent(agent)
	if react == nil {
		t.Fatal("AsReActAgent returned nil")
	}

	if len(react.Tools) != 2 {
		t.Fatalf("len(Tools) = %d, want 2", len(react.Tools))
	}
}

func TestUnmarshalAgent_YAML_ReActWithContext(t *testing.T) {
	data := loadYAMLAgentFile(t, "testdata/agent/react_with_context.yaml")

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

	// Check context layer types
	if react.ContextLayers[0].This != ".prompt" {
		t.Errorf("ContextLayers[0].This = %q, want %q", react.ContextLayers[0].This, ".prompt")
	}
	if react.ContextLayers[1].Ref != "character:elsa" {
		t.Errorf("ContextLayers[1].Ref = %q, want %q", react.ContextLayers[1].Ref, "character:elsa")
	}
	if react.ContextLayers[2].Env != "CUSTOM_CONTEXT" {
		t.Errorf("ContextLayers[2].Env = %q, want %q", react.ContextLayers[2].Env, "CUSTOM_CONTEXT")
	}
	if react.ContextLayers[3].Mem == nil {
		t.Error("ContextLayers[3].Mem is nil")
	}
}

func TestUnmarshalAgent_YAML_ReActFull(t *testing.T) {
	data := loadYAMLAgentFile(t, "testdata/agent/react_full.yaml")

	agent, err := UnmarshalAgent(data)
	if err != nil {
		t.Fatalf("UnmarshalAgent: %v", err)
	}

	react := AsReActAgent(agent)
	if react == nil {
		t.Fatal("AsReActAgent returned nil")
	}

	if react.Name != "full_agent" {
		t.Errorf("Name = %q, want %q", react.Name, "full_agent")
	}
	if react.Generator.IsEmpty() {
		t.Error("Generator is empty")
	}
	if len(react.ContextLayers) != 2 {
		t.Errorf("len(ContextLayers) = %d, want 2", len(react.ContextLayers))
	}
	if len(react.Tools) != 3 {
		t.Errorf("len(Tools) = %d, want 3", len(react.Tools))
	}
}

func TestUnmarshalAgent_YAML_MatchRouter(t *testing.T) {
	data := loadYAMLAgentFile(t, "testdata/agent/match_router.yaml")

	agent, err := UnmarshalAgent(data)
	if err != nil {
		t.Fatalf("UnmarshalAgent: %v", err)
	}

	if agent.AgentType() != AgentTypeMatch {
		t.Errorf("AgentType() = %q, want %q", agent.AgentType(), AgentTypeMatch)
	}

	match := AsMatchAgent(agent)
	if match == nil {
		t.Fatal("AsMatchAgent returned nil")
	}

	if len(match.Rules) != 2 {
		t.Fatalf("len(Rules) = %d, want 2", len(match.Rules))
	}
}

// ========== MsgPack Tests ==========

func TestReActAgent_MsgpackRoundtrip(t *testing.T) {
	data := loadTestFile(t, "testdata/agent/react_full.json")

	agent, err := UnmarshalAgent(data)
	if err != nil {
		t.Fatalf("UnmarshalAgent: %v", err)
	}

	original := AsReActAgent(agent)
	if original == nil {
		t.Fatal("AsReActAgent returned nil")
	}

	// MsgPack roundtrip
	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("MsgPack Marshal: %v", err)
	}

	var decoded ReActAgent
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("MsgPack Unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Prompt != original.Prompt {
		t.Errorf("Prompt = %q, want %q", decoded.Prompt, original.Prompt)
	}
	if len(decoded.Tools) != len(original.Tools) {
		t.Errorf("len(Tools) = %d, want %d", len(decoded.Tools), len(original.Tools))
	}
	if len(decoded.ContextLayers) != len(original.ContextLayers) {
		t.Errorf("len(ContextLayers) = %d, want %d", len(decoded.ContextLayers), len(original.ContextLayers))
	}
}

func TestMatchAgent_MsgpackRoundtrip(t *testing.T) {
	data := loadTestFile(t, "testdata/agent/match_router.json")

	agent, err := UnmarshalAgent(data)
	if err != nil {
		t.Fatalf("UnmarshalAgent: %v", err)
	}

	original := AsMatchAgent(agent)
	if original == nil {
		t.Fatal("AsMatchAgent returned nil")
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("MsgPack Marshal: %v", err)
	}

	var decoded MatchAgent
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("MsgPack Unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, original.Name)
	}
	if len(decoded.Rules) != len(original.Rules) {
		t.Errorf("len(Rules) = %d, want %d", len(decoded.Rules), len(original.Rules))
	}
	if len(decoded.Route) != len(original.Route) {
		t.Errorf("len(Route) = %d, want %d", len(decoded.Route), len(original.Route))
	}
}

func TestAgentRef_MsgpackRoundtrip(t *testing.T) {
	tests := []struct {
		name     string
		original AgentRef
	}{
		{
			name:     "ref only",
			original: AgentRef{Ref: "agent:music_player"},
		},
		{
			name: "inline react",
			original: AgentRef{
				Agent: &ReActAgent{
					AgentBase: AgentBase{Name: "inline", Prompt: "Test prompt"},
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

			var decoded AgentRef
			if err := msgpack.Unmarshal(packed, &decoded); err != nil {
				t.Fatalf("MsgPack Unmarshal: %v", err)
			}

			if decoded.Ref != tt.original.Ref {
				t.Errorf("Ref = %q, want %q", decoded.Ref, tt.original.Ref)
			}
			if (decoded.Agent == nil) != (tt.original.Agent == nil) {
				t.Errorf("Agent nil mismatch")
			}
			if tt.original.Agent != nil && decoded.Agent != nil {
				if decoded.Agent.AgentName() != tt.original.Agent.AgentName() {
					t.Errorf("Agent.Name = %q, want %q", decoded.Agent.AgentName(), tt.original.Agent.AgentName())
				}
			}
		})
	}
}

// ========== Validate Error Tests ==========

func TestReActAgent_Validate_Error_NoName(t *testing.T) {
	data := []byte(`{"type":"react","prompt":"Test prompt"}`)
	var agent ReActAgent
	err := json.Unmarshal(data, &agent)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMatchAgent_Validate_Error_NoName(t *testing.T) {
	data := []byte(`{"type":"match"}`)
	var agent MatchAgent
	err := json.Unmarshal(data, &agent)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMatchAgent_Validate_Error_RouteNoRules(t *testing.T) {
	data := []byte(`{
		"type": "match",
		"name": "router",
		"route": [
			{
				"rules": [],
				"agent": {"$ref": "agent:test"}
			}
		]
	}`)
	var agent MatchAgent
	err := json.Unmarshal(data, &agent)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "rules is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMatchAgent_Validate_Error_RouteNoAgent(t *testing.T) {
	data := []byte(`{
		"type": "match",
		"name": "router",
		"route": [
			{
				"rules": ["play_music"]
			}
		]
	}`)
	var agent MatchAgent
	err := json.Unmarshal(data, &agent)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "agent is required") {
		t.Errorf("error = %q", err.Error())
	}
}

// ========== As Functions Tests (nil returns) ==========

func TestAsReActAgent_Nil(t *testing.T) {
	agent := &MatchAgent{AgentBase: AgentBase{Name: "router"}}
	if AsReActAgent(agent) != nil {
		t.Error("AsReActAgent should return nil for MatchAgent")
	}
}

func TestAsMatchAgent_Nil(t *testing.T) {
	agent := &ReActAgent{AgentBase: AgentBase{Name: "assistant"}}
	if AsMatchAgent(agent) != nil {
		t.Error("AsMatchAgent should return nil for ReActAgent")
	}
}

// ========== AgentRef DecodeMsgpack Inline Tests ==========

func TestAgentRef_MsgpackRoundtrip_InlineMatch(t *testing.T) {
	original := AgentRef{
		Agent: &MatchAgent{
			AgentBase: AgentBase{Name: "router", Type: AgentTypeMatch},
		},
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("MsgPack Marshal: %v", err)
	}

	var decoded AgentRef
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("MsgPack Unmarshal: %v", err)
	}

	if decoded.Agent == nil {
		t.Fatal("Agent is nil after decode")
	}
	if decoded.Agent.AgentName() != "router" {
		t.Errorf("Agent.Name = %q, want %q", decoded.Agent.AgentName(), "router")
	}
}

// ========== AgentRef MarshalJSON Inline Tests ==========

func TestAgentRef_MarshalJSON_Inline(t *testing.T) {
	ref := AgentRef{
		Agent: &ReActAgent{
			AgentBase: AgentBase{Name: "inline_agent", Prompt: "Test"},
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

	if decoded["name"] != "inline_agent" {
		t.Errorf("name = %v, want inline_agent", decoded["name"])
	}
}

// ========== UnmarshalAgent Match Tests ==========

func TestUnmarshalAgent_Match(t *testing.T) {
	data := []byte(`{"type":"match","name":"router"}`)

	agent, err := UnmarshalAgent(data)
	if err != nil {
		t.Fatalf("UnmarshalAgent: %v", err)
	}

	match := AsMatchAgent(agent)
	if match == nil {
		t.Fatal("AsMatchAgent returned nil")
	}
	if match.Name != "router" {
		t.Errorf("Name = %q, want %q", match.Name, "router")
	}
}

// ========== UnmarshalAgent Error Tests ==========

func TestUnmarshalAgent_Error_InvalidType(t *testing.T) {
	data := []byte(`{"type":"invalid_type","name":"test"}`)
	_, err := UnmarshalAgent(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid agent type") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestUnmarshalAgent_Error_InvalidJSON(t *testing.T) {
	data := []byte(`{invalid`)
	_, err := UnmarshalAgent(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ========== Agent Unmarshal Invalid Data Tests ==========

func TestReActAgent_UnmarshalJSON_InvalidData(t *testing.T) {
	var agent ReActAgent
	err := json.Unmarshal([]byte(`{invalid`), &agent)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMatchAgent_UnmarshalJSON_InvalidData(t *testing.T) {
	var agent MatchAgent
	err := json.Unmarshal([]byte(`{invalid`), &agent)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAgentRef_UnmarshalJSON_InvalidData(t *testing.T) {
	var ref AgentRef
	err := json.Unmarshal([]byte(`{invalid`), &ref)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAgentRef_DecodeMsgpack_InvalidData(t *testing.T) {
	var ref AgentRef
	err := msgpack.Unmarshal([]byte{0xff}, &ref)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ========== UnmarshalAgent Parse Error Tests ==========

func TestUnmarshalAgent_Error_ParseMatch(t *testing.T) {
	// Invalid JSON for match agent (route is not an array)
	data := []byte(`{"type":"match","name":"router","route":"not_array"}`)
	_, err := UnmarshalAgent(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parse match agent") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestUnmarshalAgent_Error_ParseReAct(t *testing.T) {
	// Invalid JSON for react agent (tools is not an array)
	data := []byte(`{"type":"react","name":"assistant","tools":"not_array"}`)
	_, err := UnmarshalAgent(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parse react agent") {
		t.Errorf("error = %q", err.Error())
	}
}
