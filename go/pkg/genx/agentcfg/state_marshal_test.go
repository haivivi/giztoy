package agentcfg

import (
	"encoding/json"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

// ========== Message JSON Marshal Tests ==========

func TestMessage_MarshalJSON_User(t *testing.T) {
	msg := Message{
		Role:    RoleUser,
		Content: "Hello",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Role != msg.Role {
		t.Errorf("Role = %q, want %q", decoded.Role, msg.Role)
	}
	if decoded.Content != msg.Content {
		t.Errorf("Content = %q, want %q", decoded.Content, msg.Content)
	}
}

func TestMessage_MarshalJSON_ModelToolCall(t *testing.T) {
	msg := Message{
		Role:         RoleModel,
		ToolCallID:   "call_1",
		ToolCallName: "get_weather",
		ToolCallArgs: `{"city":"NYC"}`,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ToolCallID != msg.ToolCallID {
		t.Errorf("ToolCallID = %q, want %q", decoded.ToolCallID, msg.ToolCallID)
	}
	if decoded.ToolCallName != msg.ToolCallName {
		t.Errorf("ToolCallName = %q, want %q", decoded.ToolCallName, msg.ToolCallName)
	}
}

func TestMessage_MarshalJSON_ToolResult(t *testing.T) {
	msg := Message{
		Role:         RoleTool,
		Content:      "Sunny",
		ToolResultID: "call_1",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ToolResultID != msg.ToolResultID {
		t.Errorf("ToolResultID = %q, want %q", decoded.ToolResultID, msg.ToolResultID)
	}
}

// ========== State JSON Marshal Tests ==========

func TestState_MarshalJSON_ReAct(t *testing.T) {
	state := State{
		StateType: StateTypeReAct,
		ID:        "test_state",
		AgentDef:  "assistant",
		Phase:     "thinking",
		Messages: []Message{
			{Role: RoleUser, Content: "Hello"},
			{Role: RoleModel, Content: "Hi!"},
		},
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded State
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.StateType != state.StateType {
		t.Errorf("StateType = %q, want %q", decoded.StateType, state.StateType)
	}
	if decoded.ID != state.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, state.ID)
	}
	if len(decoded.Messages) != len(state.Messages) {
		t.Errorf("len(Messages) = %d, want %d", len(decoded.Messages), len(state.Messages))
	}
}

func TestState_MarshalJSON_Match(t *testing.T) {
	state := State{
		StateType: StateTypeMatch,
		ID:        "match_state",
		AgentDef:  "router",
		Input:     "Play music",
		Matched:   true,
		Matches: []MatchedIntent{
			{RuleName: "music_intent", AgentName: "music_agent"},
		},
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded State
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Input != state.Input {
		t.Errorf("Input = %q, want %q", decoded.Input, state.Input)
	}
	if decoded.Matched != state.Matched {
		t.Errorf("Matched = %v, want %v", decoded.Matched, state.Matched)
	}
	if len(decoded.Matches) != 1 {
		t.Errorf("len(Matches) = %d, want 1", len(decoded.Matches))
	}
}

func TestState_MarshalJSON_WithCallingState(t *testing.T) {
	state := State{
		StateType: StateTypeMatch,
		ID:        "parent",
		AgentDef:  "router",
		CallingState: &State{
			StateType: StateTypeReAct,
			ID:        "child",
			AgentDef:  "sub_agent",
		},
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded State
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.CallingState == nil {
		t.Fatal("CallingState is nil")
	}
	if decoded.CallingState.ID != "child" {
		t.Errorf("CallingState.ID = %q, want %q", decoded.CallingState.ID, "child")
	}
}

// ========== MemorySegment & MemoryQuery JSON Marshal Tests ==========

func TestMemorySegment_MarshalJSON(t *testing.T) {
	seg := MemorySegment{
		ID:        "seg_1",
		Summary:   "User asked about weather",
		Keywords:  []string{"weather", "NYC"},
		UnixEpoch: 1704067200,
	}

	data, err := json.Marshal(seg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded MemorySegment
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != seg.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, seg.ID)
	}
	if decoded.Summary != seg.Summary {
		t.Errorf("Summary = %q, want %q", decoded.Summary, seg.Summary)
	}
}

func TestMemoryQuery_MarshalJSON(t *testing.T) {
	query := MemoryQuery{
		Text:  "weather forecast",
		Year:  2024,
		Month: 1,
	}

	data, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded MemoryQuery
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Text != query.Text {
		t.Errorf("Text = %q, want %q", decoded.Text, query.Text)
	}
	if decoded.Year != query.Year {
		t.Errorf("Year = %d, want %d", decoded.Year, query.Year)
	}
}

func TestToolResult_MarshalJSON(t *testing.T) {
	result := ToolResult{
		ID:      "call_1",
		Name:    "get_weather",
		Content: "Sunny, 72°F",
		IsError: false,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded ToolResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != result.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, result.ID)
	}
	if decoded.Name != result.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, result.Name)
	}
}

func TestMatchedIntent_MarshalJSON(t *testing.T) {
	intent := MatchedIntent{
		RuleName:  "weather_intent",
		AgentName: "weather_agent",
	}

	data, err := json.Marshal(intent)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded MatchedIntent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.RuleName != intent.RuleName {
		t.Errorf("RuleName = %q, want %q", decoded.RuleName, intent.RuleName)
	}
}

// ========== State MsgPack Marshal Tests ==========

func TestMessage_MarshalMsgpack(t *testing.T) {
	tests := []struct {
		name     string
		original Message
	}{
		{
			name:     "user",
			original: Message{Role: RoleUser, Content: "Hello"},
		},
		{
			name:     "model text",
			original: Message{Role: RoleModel, Content: "Hi!"},
		},
		{
			name: "model tool call",
			original: Message{
				Role:         RoleModel,
				ToolCallID:   "call_1",
				ToolCallName: "search",
				ToolCallArgs: `{"q":"test"}`,
			},
		},
		{
			name: "tool result",
			original: Message{
				Role:         RoleTool,
				Content:      "result",
				ToolResultID: "call_1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packed, err := msgpack.Marshal(tt.original)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var decoded Message
			if err := msgpack.Unmarshal(packed, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if decoded.Role != tt.original.Role {
				t.Errorf("Role = %q, want %q", decoded.Role, tt.original.Role)
			}
			if decoded.Content != tt.original.Content {
				t.Errorf("Content = %q, want %q", decoded.Content, tt.original.Content)
			}
		})
	}
}

func TestState_MarshalMsgpack_ReAct(t *testing.T) {
	original := State{
		StateType: StateTypeReAct,
		ID:        "test",
		AgentDef:  "assistant",
		Phase:     "thinking",
		Messages: []Message{
			{Role: RoleUser, Content: "Hello"},
		},
		ToolResults: []ToolResult{
			{ID: "call_1", Name: "search", Content: "result"},
		},
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded State
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.StateType != original.StateType {
		t.Errorf("StateType = %q, want %q", decoded.StateType, original.StateType)
	}
	if decoded.ID != original.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, original.ID)
	}
	if len(decoded.Messages) != len(original.Messages) {
		t.Errorf("len(Messages) = %d, want %d", len(decoded.Messages), len(original.Messages))
	}
	if len(decoded.ToolResults) != len(original.ToolResults) {
		t.Errorf("len(ToolResults) = %d, want %d", len(decoded.ToolResults), len(original.ToolResults))
	}
}

func TestState_MarshalMsgpack_Match(t *testing.T) {
	original := State{
		StateType: StateTypeMatch,
		ID:        "match_test",
		AgentDef:  "router",
		Input:     "Play music",
		Matched:   true,
		Matches: []MatchedIntent{
			{RuleName: "music", AgentName: "player"},
		},
		CallingState: &State{
			StateType: StateTypeReAct,
			ID:        "sub",
			AgentDef:  "player",
		},
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded State
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.StateType != original.StateType {
		t.Errorf("StateType = %q, want %q", decoded.StateType, original.StateType)
	}
	if decoded.Input != original.Input {
		t.Errorf("Input = %q, want %q", decoded.Input, original.Input)
	}
	if decoded.CallingState == nil {
		t.Fatal("CallingState is nil")
	}
	if decoded.CallingState.ID != original.CallingState.ID {
		t.Errorf("CallingState.ID = %q, want %q", decoded.CallingState.ID, original.CallingState.ID)
	}
}

func TestMemorySegment_MarshalMsgpack(t *testing.T) {
	original := MemorySegment{
		ID:        "seg_1",
		Summary:   "Test summary",
		Keywords:  []string{"a", "b"},
		UnixEpoch: 1234567890,
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded MemorySegment
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, original.ID)
	}
}

func TestMemoryQuery_MarshalMsgpack(t *testing.T) {
	original := MemoryQuery{
		Text:  "test",
		Year:  2024,
		Month: 6,
		Day:   15,
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded MemoryQuery
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Text != original.Text {
		t.Errorf("Text = %q, want %q", decoded.Text, original.Text)
	}
}
