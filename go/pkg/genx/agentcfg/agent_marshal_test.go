package agentcfg

import (
	"encoding/json"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

// ========== Agent JSON Marshal Tests ==========

func TestMarshalAgent_ReAct_JSON(t *testing.T) {
	agent := &ReActAgent{
		AgentBase: AgentBase{
			Name:   "test_agent",
			Type:   AgentTypeReAct,
			Prompt: "You are helpful",
		},
		Tools: []ToolRef{
			{Ref: "tool:helper"},
			{Ref: "tool:exit", Quit: true},
		},
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got, err := UnmarshalAgent(data)
	if err != nil {
		t.Fatalf("UnmarshalAgent: %v", err)
	}

	react := AsReActAgent(got)
	if react == nil {
		t.Fatal("expected ReActAgent")
	}
	if react.Name != agent.Name {
		t.Errorf("Name = %q, want %q", react.Name, agent.Name)
	}
	if react.Prompt != agent.Prompt {
		t.Errorf("Prompt = %q, want %q", react.Prompt, agent.Prompt)
	}
	if len(react.Tools) != 2 {
		t.Errorf("len(Tools) = %d, want 2", len(react.Tools))
	}
}

func TestMarshalAgent_Match_JSON(t *testing.T) {
	agent := &MatchAgent{
		AgentBase: AgentBase{
			Name: "router",
			Type: AgentTypeMatch,
		},
		Rules: []RuleRef{
			{Ref: "rule:intent1"},
		},
		Route: []MatchRoute{
			{Rules: []string{"intent1"}, Agent: AgentRef{Ref: "agent:sub"}},
		},
		Default: &AgentRef{Ref: "agent:fallback"},
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got, err := UnmarshalAgent(data)
	if err != nil {
		t.Fatalf("UnmarshalAgent: %v", err)
	}

	match := AsMatchAgent(got)
	if match == nil {
		t.Fatal("expected MatchAgent")
	}
	if match.Name != agent.Name {
		t.Errorf("Name = %q, want %q", match.Name, agent.Name)
	}
	if len(match.Rules) != 1 {
		t.Errorf("len(Rules) = %d, want 1", len(match.Rules))
	}
	if len(match.Route) != 1 {
		t.Errorf("len(Route) = %d, want 1", len(match.Route))
	}
}

func TestMarshalAgent_ReActWithContext_JSON(t *testing.T) {
	agent := &ReActAgent{
		AgentBase: AgentBase{
			Name: "ctx_agent",
			Type: AgentTypeReAct,
			ContextLayers: []ContextLayer{
				{Prompt: "System prompt"},
				{Ref: "character:elsa"},
				{Env: "EXTRA_CONTEXT"},
			},
		},
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got, err := UnmarshalAgent(data)
	if err != nil {
		t.Fatalf("UnmarshalAgent: %v", err)
	}

	react := AsReActAgent(got)
	if react == nil {
		t.Fatal("expected ReActAgent")
	}
	if len(react.ContextLayers) != 3 {
		t.Errorf("len(ContextLayers) = %d, want 3", len(react.ContextLayers))
	}
}

// ========== AgentRef Marshal Tests ==========

func TestAgentRef_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		ref  AgentRef
	}{
		{
			name: "ref only",
			ref:  AgentRef{Ref: "agent:sub"},
		},
		{
			name: "inline react",
			ref: AgentRef{
				Agent: &ReActAgent{
					AgentBase: AgentBase{Name: "inline", Type: AgentTypeReAct, Prompt: "test"},
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

			var got AgentRef
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if got.Ref != tt.ref.Ref {
				t.Errorf("Ref = %q, want %q", got.Ref, tt.ref.Ref)
			}
			if (got.Agent == nil) != (tt.ref.Agent == nil) {
				t.Error("Agent nil mismatch")
			}
		})
	}
}

// ========== Agent MsgPack Marshal Tests ==========

func TestMarshalAgent_ReAct_Msgpack(t *testing.T) {
	original := &ReActAgent{
		AgentBase: AgentBase{
			Name:   "test_agent",
			Type:   AgentTypeReAct,
			Prompt: "You are helpful",
		},
		Tools: []ToolRef{
			{Ref: "tool:helper"},
		},
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded ReActAgent
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
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
}

func TestMarshalAgent_Match_Msgpack(t *testing.T) {
	original := &MatchAgent{
		AgentBase: AgentBase{
			Name: "router",
			Type: AgentTypeMatch,
		},
		Rules: []RuleRef{
			{Ref: "rule:intent"},
		},
		Route: []MatchRoute{
			{Rules: []string{"intent"}, Agent: AgentRef{Ref: "agent:sub"}},
		},
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded MatchAgent
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, original.Name)
	}
	if len(decoded.Rules) != len(original.Rules) {
		t.Errorf("len(Rules) = %d, want %d", len(decoded.Rules), len(original.Rules))
	}
}

func TestAgentRef_MarshalMsgpack(t *testing.T) {
	tests := []struct {
		name     string
		original AgentRef
	}{
		{
			name:     "ref only",
			original: AgentRef{Ref: "agent:helper"},
		},
		{
			name: "inline react",
			original: AgentRef{
				Agent: &ReActAgent{
					AgentBase: AgentBase{Name: "inline", Type: AgentTypeReAct, Prompt: "test"},
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

			var decoded AgentRef
			if err := msgpack.Unmarshal(packed, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if decoded.Ref != tt.original.Ref {
				t.Errorf("Ref = %q, want %q", decoded.Ref, tt.original.Ref)
			}
			if (decoded.Agent == nil) != (tt.original.Agent == nil) {
				t.Error("Agent nil mismatch")
			}
		})
	}
}

// ========== AgentRef MarshalJSON Nil Tests ==========

func TestAgentRef_MarshalJSON_Empty(t *testing.T) {
	ref := AgentRef{} // both Ref and Agent are empty

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	// Should marshal to null or {}
	if string(data) != "null" && string(data) != "{}" {
		t.Errorf("MarshalJSON = %s, want null or {}", data)
	}
}
