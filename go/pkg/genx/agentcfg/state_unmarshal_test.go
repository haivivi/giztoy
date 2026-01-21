package agentcfg

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
	"gopkg.in/yaml.v3"
)

// ========== Message JSON Tests ==========

func TestMessage_JSON_User(t *testing.T) {
	data := loadTestFile(t, "testdata/state/message_user.json")

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if msg.Role != RoleUser {
		t.Errorf("Role = %q, want %q", msg.Role, RoleUser)
	}
	if msg.Content != "Hello, how are you?" {
		t.Errorf("Content = %q", msg.Content)
	}
}

func TestMessage_JSON_ModelText(t *testing.T) {
	data := loadTestFile(t, "testdata/state/message_model_text.json")

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if msg.Role != RoleModel {
		t.Errorf("Role = %q, want %q", msg.Role, RoleModel)
	}
	if msg.Content == "" {
		t.Error("Content is empty")
	}
}

func TestMessage_JSON_ModelToolCall(t *testing.T) {
	data := loadTestFile(t, "testdata/state/message_model_tool_call.json")

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if msg.Role != RoleModel {
		t.Errorf("Role = %q, want %q", msg.Role, RoleModel)
	}
	if msg.ToolCallID != "call_abc123" {
		t.Errorf("ToolCallID = %q, want %q", msg.ToolCallID, "call_abc123")
	}
	if msg.ToolCallName != "get_weather" {
		t.Errorf("ToolCallName = %q, want %q", msg.ToolCallName, "get_weather")
	}
	if msg.ToolCallArgs == "" {
		t.Error("ToolCallArgs is empty")
	}
}

func TestMessage_JSON_ToolResult(t *testing.T) {
	data := loadTestFile(t, "testdata/state/message_tool_result.json")

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if msg.Role != RoleTool {
		t.Errorf("Role = %q, want %q", msg.Role, RoleTool)
	}
	if msg.ToolResultID != "call_abc123" {
		t.Errorf("ToolResultID = %q, want %q", msg.ToolResultID, "call_abc123")
	}
	if msg.Content != "Sunny, 72°F" {
		t.Errorf("Content = %q", msg.Content)
	}
}

// ========== Message Error Tests ==========

func TestMessage_Error_InvalidRole(t *testing.T) {
	data := loadTestFile(t, "testdata/error/message_invalid_role.json")

	var msg Message
	err := json.Unmarshal(data, &msg)
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if !strings.Contains(err.Error(), "invalid message role") {
		t.Errorf("error = %q, want containing %q", err.Error(), "invalid message role")
	}
}

func TestMessage_Error_UserWithToolCall(t *testing.T) {
	data := loadTestFile(t, "testdata/error/message_user_with_tool_call.json")

	var msg Message
	err := json.Unmarshal(data, &msg)
	if err == nil {
		t.Fatal("expected error for user with tool call")
	}
	if !strings.Contains(err.Error(), "should not have tool call fields") {
		t.Errorf("error = %q, want containing %q", err.Error(), "should not have tool call fields")
	}
}

func TestMessage_Error_ToolNoResultID(t *testing.T) {
	data := loadTestFile(t, "testdata/error/message_tool_no_result_id.json")

	var msg Message
	err := json.Unmarshal(data, &msg)
	if err == nil {
		t.Fatal("expected error for tool without result id")
	}
	if !strings.Contains(err.Error(), "must have tool_result_id") {
		t.Errorf("error = %q, want containing %q", err.Error(), "must have tool_result_id")
	}
}

// ========== Message MsgPack Tests ==========

func TestMessage_MsgpackRoundtrip(t *testing.T) {
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
			original: Message{Role: RoleModel, Content: "Hi there!"},
		},
		{
			name: "model tool call",
			original: Message{
				Role:         RoleModel,
				ToolCallID:   "call_1",
				ToolCallName: "get_weather",
				ToolCallArgs: `{"city":"NYC"}`,
			},
		},
		{
			name: "tool result",
			original: Message{
				Role:         RoleTool,
				Content:      "Sunny",
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
			if decoded.ToolCallID != tt.original.ToolCallID {
				t.Errorf("ToolCallID = %q, want %q", decoded.ToolCallID, tt.original.ToolCallID)
			}
			if decoded.ToolResultID != tt.original.ToolResultID {
				t.Errorf("ToolResultID = %q, want %q", decoded.ToolResultID, tt.original.ToolResultID)
			}
		})
	}
}

// ========== State JSON Tests ==========

func loadStateTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func TestState_JSON_ReActMinimal(t *testing.T) {
	data := loadStateTestFile(t, "testdata/state/react_minimal.json")

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if state.StateType != StateTypeReAct {
		t.Errorf("StateType = %q, want %q", state.StateType, StateTypeReAct)
	}
	if state.ID != "react_minimal" {
		t.Errorf("ID = %q, want %q", state.ID, "react_minimal")
	}
	if state.AgentDef != "assistant" {
		t.Errorf("AgentDef = %q, want %q", state.AgentDef, "assistant")
	}
	if len(state.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(state.Messages))
	}
	if !state.IsReAct() {
		t.Error("IsReAct() = false, want true")
	}
}

func TestState_JSON_ReActWithToolCall(t *testing.T) {
	data := loadStateTestFile(t, "testdata/state/react_with_tool_call.json")

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(state.Messages) != 4 {
		t.Fatalf("len(Messages) = %d, want 4", len(state.Messages))
	}

	// Check tool call message
	if state.Messages[1].ToolCallID != "call_1" {
		t.Errorf("Messages[1].ToolCallID = %q, want %q", state.Messages[1].ToolCallID, "call_1")
	}
	if state.Messages[1].ToolCallName != "get_weather" {
		t.Errorf("Messages[1].ToolCallName = %q, want %q", state.Messages[1].ToolCallName, "get_weather")
	}

	// Check tool result message
	if state.Messages[2].ToolResultID != "call_1" {
		t.Errorf("Messages[2].ToolResultID = %q, want %q", state.Messages[2].ToolResultID, "call_1")
	}
}

func TestState_JSON_ReActPendingTool(t *testing.T) {
	data := loadStateTestFile(t, "testdata/state/react_pending_tool.json")

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if state.Phase != "tool_pending" {
		t.Errorf("Phase = %q, want %q", state.Phase, "tool_pending")
	}
	if len(state.ToolResults) != 1 {
		t.Fatalf("len(ToolResults) = %d, want 1", len(state.ToolResults))
	}
	if state.ToolResults[0].ID != "call_search" {
		t.Errorf("ToolResults[0].ID = %q, want %q", state.ToolResults[0].ID, "call_search")
	}
	if state.ToolResults[0].Name != "web_search" {
		t.Errorf("ToolResults[0].Name = %q, want %q", state.ToolResults[0].Name, "web_search")
	}
}

func TestState_JSON_MatchMinimal(t *testing.T) {
	data := loadStateTestFile(t, "testdata/state/match_minimal.json")

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if state.StateType != StateTypeMatch {
		t.Errorf("StateType = %q, want %q", state.StateType, StateTypeMatch)
	}
	if !state.IsMatch() {
		t.Error("IsMatch() = false, want true")
	}
	if state.Input != "Play some music" {
		t.Errorf("Input = %q, want %q", state.Input, "Play some music")
	}
	if state.Matched {
		t.Error("Matched = true, want false")
	}
}

func TestState_JSON_MatchWithCalling(t *testing.T) {
	data := loadStateTestFile(t, "testdata/state/match_with_calling.json")

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !state.Matched {
		t.Error("Matched = false, want true")
	}
	if len(state.Matches) != 1 {
		t.Fatalf("len(Matches) = %d, want 1", len(state.Matches))
	}
	if state.Matches[0].RuleName != "weather_intent" {
		t.Errorf("Matches[0].RuleName = %q, want %q", state.Matches[0].RuleName, "weather_intent")
	}
	if state.Matches[0].AgentName != "weather_agent" {
		t.Errorf("Matches[0].AgentName = %q, want %q", state.Matches[0].AgentName, "weather_agent")
	}

	// Check nested calling state
	if state.CallingState == nil {
		t.Fatal("CallingState is nil")
	}
	if state.CallingState.StateType != StateTypeReAct {
		t.Errorf("CallingState.StateType = %q, want %q", state.CallingState.StateType, StateTypeReAct)
	}
	if state.CallingState.ID != "sub_weather" {
		t.Errorf("CallingState.ID = %q, want %q", state.CallingState.ID, "sub_weather")
	}
}

// ========== State YAML Tests ==========

func loadYAMLStateFile(t *testing.T, path string) []byte {
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

func TestState_YAML_ReActMinimal(t *testing.T) {
	data := loadYAMLStateFile(t, "testdata/state/react_minimal.yaml")

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if state.StateType != StateTypeReAct {
		t.Errorf("StateType = %q, want %q", state.StateType, StateTypeReAct)
	}
	if len(state.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(state.Messages))
	}
}

func TestState_YAML_ReActWithToolCall(t *testing.T) {
	data := loadYAMLStateFile(t, "testdata/state/react_with_tool_call.yaml")

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(state.Messages) != 4 {
		t.Fatalf("len(Messages) = %d, want 4", len(state.Messages))
	}
	if state.Messages[1].ToolCallName != "get_weather" {
		t.Errorf("Messages[1].ToolCallName = %q", state.Messages[1].ToolCallName)
	}
}

func TestState_YAML_ReActPendingTool(t *testing.T) {
	data := loadYAMLStateFile(t, "testdata/state/react_pending_tool.yaml")

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(state.ToolResults) != 1 {
		t.Fatalf("len(ToolResults) = %d, want 1", len(state.ToolResults))
	}
}

func TestState_YAML_MatchMinimal(t *testing.T) {
	data := loadYAMLStateFile(t, "testdata/state/match_minimal.yaml")

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if state.StateType != StateTypeMatch {
		t.Errorf("StateType = %q, want %q", state.StateType, StateTypeMatch)
	}
}

func TestState_YAML_MatchWithCalling(t *testing.T) {
	data := loadYAMLStateFile(t, "testdata/state/match_with_calling.yaml")

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if state.CallingState == nil {
		t.Fatal("CallingState is nil")
	}
	if state.CallingState.AgentDef != "weather_agent" {
		t.Errorf("CallingState.AgentDef = %q", state.CallingState.AgentDef)
	}
}

// ========== State MsgPack Tests ==========

func TestState_MsgpackRoundtrip_ReAct(t *testing.T) {
	data := loadStateTestFile(t, "testdata/state/react_pending_tool.json")

	var original State
	if err := json.Unmarshal(data, &original); err != nil {
		t.Fatalf("Unmarshal: %v", err)
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
	if decoded.Phase != original.Phase {
		t.Errorf("Phase = %q, want %q", decoded.Phase, original.Phase)
	}
	if len(decoded.Messages) != len(original.Messages) {
		t.Errorf("len(Messages) = %d, want %d", len(decoded.Messages), len(original.Messages))
	}
	if len(decoded.ToolResults) != len(original.ToolResults) {
		t.Errorf("len(ToolResults) = %d, want %d", len(decoded.ToolResults), len(original.ToolResults))
	}
}

func TestState_MsgpackRoundtrip_Match(t *testing.T) {
	data := loadStateTestFile(t, "testdata/state/match_with_calling.json")

	var original State
	if err := json.Unmarshal(data, &original); err != nil {
		t.Fatalf("Unmarshal: %v", err)
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
	if decoded.Matched != original.Matched {
		t.Errorf("Matched = %v, want %v", decoded.Matched, original.Matched)
	}
	if len(decoded.Matches) != len(original.Matches) {
		t.Errorf("len(Matches) = %d, want %d", len(decoded.Matches), len(original.Matches))
	}
	if decoded.CallingState == nil {
		t.Fatal("CallingState is nil")
	}
	if decoded.CallingState.ID != original.CallingState.ID {
		t.Errorf("CallingState.ID = %q, want %q", decoded.CallingState.ID, original.CallingState.ID)
	}
}

// ========== MemorySegment & MemoryQuery Tests ==========

func TestMemorySegment_MsgpackRoundtrip(t *testing.T) {
	original := MemorySegment{
		ID:        "seg_1",
		Summary:   "User discussed weather in NYC",
		Keywords:  []string{"weather", "NYC", "sunny"},
		UnixEpoch: 1704067200,
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
	if decoded.Summary != original.Summary {
		t.Errorf("Summary = %q, want %q", decoded.Summary, original.Summary)
	}
	if len(decoded.Keywords) != len(original.Keywords) {
		t.Errorf("len(Keywords) = %d, want %d", len(decoded.Keywords), len(original.Keywords))
	}
}

func TestMemoryQuery_MsgpackRoundtrip(t *testing.T) {
	original := MemoryQuery{
		Text:  "weather forecast",
		Year:  2024,
		Month: 1,
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
	if decoded.Year != original.Year {
		t.Errorf("Year = %d, want %d", decoded.Year, original.Year)
	}
	if decoded.Month != original.Month {
		t.Errorf("Month = %d, want %d", decoded.Month, original.Month)
	}
}

func TestToolResult_MsgpackRoundtrip(t *testing.T) {
	original := ToolResult{
		ID:      "call_1",
		Name:    "get_weather",
		Content: "Sunny, 72°F",
		IsError: false,
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded ToolResult
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Name != original.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Content != original.Content {
		t.Errorf("Content = %q, want %q", decoded.Content, original.Content)
	}
	if decoded.IsError != original.IsError {
		t.Errorf("IsError = %v, want %v", decoded.IsError, original.IsError)
	}
}

func TestMatchedIntent_MsgpackRoundtrip(t *testing.T) {
	original := MatchedIntent{
		RuleName:  "weather_intent",
		AgentName: "weather_agent",
	}

	packed, err := msgpack.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded MatchedIntent
	if err := msgpack.Unmarshal(packed, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.RuleName != original.RuleName {
		t.Errorf("RuleName = %q, want %q", decoded.RuleName, original.RuleName)
	}
	if decoded.AgentName != original.AgentName {
		t.Errorf("AgentName = %q, want %q", decoded.AgentName, original.AgentName)
	}
}

// ========== Validate Error Tests ==========

func TestMessage_Validate_Error_UserWithToolCall(t *testing.T) {
	data := []byte(`{"role":"user","content":"test","tool_call_id":"call_1","tool_call_name":"foo"}`)
	var msg Message
	err := json.Unmarshal(data, &msg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "user message should not have tool call fields") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMessage_Validate_Error_UserWithToolResult(t *testing.T) {
	data := []byte(`{"role":"user","content":"test","tool_result_id":"result_1"}`)
	var msg Message
	err := json.Unmarshal(data, &msg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "user message should not have tool result fields") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMessage_Validate_Error_ModelToolCallNoName(t *testing.T) {
	data := []byte(`{"role":"model","tool_call_id":"call_1"}`)
	var msg Message
	err := json.Unmarshal(data, &msg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "model tool call message must have tool_call_name") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMessage_Validate_Error_ModelWithToolResult(t *testing.T) {
	data := []byte(`{"role":"model","content":"test","tool_result_id":"result_1"}`)
	var msg Message
	err := json.Unmarshal(data, &msg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "model message should not have tool result fields") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMessage_Validate_Error_ToolNoResultID(t *testing.T) {
	data := []byte(`{"role":"tool","content":"test"}`)
	var msg Message
	err := json.Unmarshal(data, &msg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "tool message must have tool_result_id") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMessage_Validate_Error_ToolWithToolCall(t *testing.T) {
	data := []byte(`{"role":"tool","content":"test","tool_result_id":"r1","tool_call_id":"call_1"}`)
	var msg Message
	err := json.Unmarshal(data, &msg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "tool message should not have tool call fields") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMemorySegment_Validate_Error_NoID(t *testing.T) {
	data := []byte(`{"summary":"test","unix_epoch":1234567890}`)
	var seg MemorySegment
	err := json.Unmarshal(data, &seg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "id is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMemorySegment_Validate_Error_NoSummary(t *testing.T) {
	data := []byte(`{"id":"seg_1","unix_epoch":1234567890}`)
	var seg MemorySegment
	err := json.Unmarshal(data, &seg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "summary is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMemorySegment_Validate_Error_NoUnixEpoch(t *testing.T) {
	data := []byte(`{"id":"seg_1","summary":"test"}`)
	var seg MemorySegment
	err := json.Unmarshal(data, &seg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unix_epoch is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMemoryQuery_Validate_Error_InvalidMonth(t *testing.T) {
	data := []byte(`{"month":13}`)
	var q MemoryQuery
	err := json.Unmarshal(data, &q)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "month must be 1-12") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMemoryQuery_Validate_Error_InvalidDay(t *testing.T) {
	data := []byte(`{"day":32}`)
	var q MemoryQuery
	err := json.Unmarshal(data, &q)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "day must be 1-31") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMemoryQuery_Validate_Error_InvalidHour(t *testing.T) {
	data := []byte(`{"hour":25}`)
	var q MemoryQuery
	err := json.Unmarshal(data, &q)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "hour must be 1-24") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestToolResult_Validate_Error_NoID(t *testing.T) {
	data := []byte(`{"name":"test","content":"foo"}`)
	var r ToolResult
	err := json.Unmarshal(data, &r)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "id is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestToolResult_Validate_Error_NoName(t *testing.T) {
	data := []byte(`{"id":"call_1","content":"foo"}`)
	var r ToolResult
	err := json.Unmarshal(data, &r)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMatchedIntent_Validate_Error_NoRuleName(t *testing.T) {
	data := []byte(`{"agent_name":"test"}`)
	var i MatchedIntent
	err := json.Unmarshal(data, &i)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "rule_name is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestMatchedIntent_Validate_Error_NoAgentName(t *testing.T) {
	data := []byte(`{"rule_name":"test"}`)
	var i MatchedIntent
	err := json.Unmarshal(data, &i)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "agent_name is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestState_Validate_Error_NoID(t *testing.T) {
	data := []byte(`{"state_type":"react","agent_def":"agent:test"}`)
	var s State
	err := json.Unmarshal(data, &s)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "id is required") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestState_Validate_Error_NoAgentDef(t *testing.T) {
	data := []byte(`{"state_type":"react","id":"state_1"}`)
	var s State
	err := json.Unmarshal(data, &s)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "agent_def is required") {
		t.Errorf("error = %q", err.Error())
	}
}

// ========== State Unmarshal Invalid Data Tests ==========

func TestMemorySegment_UnmarshalJSON_InvalidData(t *testing.T) {
	var seg MemorySegment
	err := json.Unmarshal([]byte(`{invalid`), &seg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMemorySegment_UnmarshalMsgpack_InvalidData(t *testing.T) {
	var seg MemorySegment
	err := msgpack.Unmarshal([]byte{0xff}, &seg) // invalid msgpack
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMemoryQuery_UnmarshalJSON_InvalidData(t *testing.T) {
	var q MemoryQuery
	err := json.Unmarshal([]byte(`{invalid`), &q)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMemoryQuery_UnmarshalMsgpack_InvalidData(t *testing.T) {
	var q MemoryQuery
	err := msgpack.Unmarshal([]byte{0xff}, &q)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestToolResult_UnmarshalJSON_InvalidData(t *testing.T) {
	var r ToolResult
	err := json.Unmarshal([]byte(`{invalid`), &r)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestToolResult_UnmarshalMsgpack_InvalidData(t *testing.T) {
	var r ToolResult
	err := msgpack.Unmarshal([]byte{0xff}, &r)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMatchedIntent_UnmarshalJSON_InvalidData(t *testing.T) {
	var i MatchedIntent
	err := json.Unmarshal([]byte(`{invalid`), &i)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMatchedIntent_UnmarshalMsgpack_InvalidData(t *testing.T) {
	var i MatchedIntent
	err := msgpack.Unmarshal([]byte{0xff}, &i)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestState_UnmarshalJSON_InvalidData(t *testing.T) {
	var s State
	err := json.Unmarshal([]byte(`{invalid`), &s)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestState_UnmarshalMsgpack_InvalidData(t *testing.T) {
	var s State
	err := msgpack.Unmarshal([]byte{0xff}, &s)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMessage_UnmarshalMsgpack_InvalidData(t *testing.T) {
	var m Message
	err := msgpack.Unmarshal([]byte{0xff}, &m)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ========== State Types Unmarshal JSON Invalid Data Tests ==========

func TestMemorySegment_UnmarshalJSON_InvalidJSON(t *testing.T) {
	var seg MemorySegment
	err := json.Unmarshal([]byte(`{"id":123}`), &seg) // id should be string
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMemoryQuery_UnmarshalJSON_InvalidJSON(t *testing.T) {
	var q MemoryQuery
	err := json.Unmarshal([]byte(`{"month":"invalid"}`), &q) // month should be number
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestToolResult_UnmarshalJSON_InvalidJSON(t *testing.T) {
	var r ToolResult
	err := json.Unmarshal([]byte(`{"id":123}`), &r) // id should be string
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMatchedIntent_UnmarshalJSON_InvalidJSON(t *testing.T) {
	var i MatchedIntent
	err := json.Unmarshal([]byte(`{"rule_name":123}`), &i) // should be string
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestState_UnmarshalJSON_InvalidJSON(t *testing.T) {
	var s State
	err := json.Unmarshal([]byte(`{"id":123}`), &s) // id should be string
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
