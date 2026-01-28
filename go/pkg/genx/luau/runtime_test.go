package luau

import (
	"context"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/agentcfg"
)

func TestNewToolRuntime(t *testing.T) {
	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")
	logger := &TestLogger{}

	tr := NewToolRuntime(ctx, mockRT, mockState, &ToolRuntimeConfig{
		AgentName:  "test-agent",
		AgentModel: "qwen-turbo",
		Logger:     logger,
	})

	if tr == nil {
		t.Fatal("NewToolRuntime returned nil")
	}

	// Test Context
	if tr.Context() != ctx {
		t.Error("Context mismatch")
	}

	// Test AgentName
	if tr.AgentName() != "test-agent" {
		t.Errorf("AgentName = %q, want %q", tr.AgentName(), "test-agent")
	}

	// Test AgentModel
	if tr.AgentModel() != "qwen-turbo" {
		t.Errorf("AgentModel = %q, want %q", tr.AgentModel(), "qwen-turbo")
	}

	// Test StateID
	if tr.StateID() != mockState.ID() {
		t.Errorf("StateID = %q, want %q", tr.StateID(), mockState.ID())
	}
}

func TestToolRuntime_Generate(t *testing.T) {
	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")

	// Setup mock to return a text stream
	mockRT.GenerateStreamFunc = func(ctx context.Context, model string, mctx genx.ModelContext) (genx.Stream, error) {
		return NewMockTextStream("Hello", " ", "World", "!"), nil
	}

	tr := NewToolRuntime(ctx, mockRT, mockState, nil)

	result, err := tr.Generate("qwen-turbo", "Say hello")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if result != "Hello World!" {
		t.Errorf("Generate result = %q, want %q", result, "Hello World!")
	}
}

func TestToolRuntime_GenerateWithContext(t *testing.T) {
	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")

	mockRT.GenerateStreamFunc = func(ctx context.Context, model string, mctx genx.ModelContext) (genx.Stream, error) {
		return NewMockTextStream("Response"), nil
	}

	tr := NewToolRuntime(ctx, mockRT, mockState, nil)

	mcb := &genx.ModelContextBuilder{}
	mcb.PromptText("system", "You are helpful")
	mcb.UserText("", "Hello")

	result, err := tr.GenerateWithContext("qwen-turbo", mcb.Build())
	if err != nil {
		t.Fatalf("GenerateWithContext error: %v", err)
	}

	if result != "Response" {
		t.Errorf("GenerateWithContext result = %q, want %q", result, "Response")
	}
}

func TestToolRuntime_State(t *testing.T) {
	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")

	tr := NewToolRuntime(ctx, mockRT, mockState, nil)

	// Test StateSet and StateGet
	tr.StateSet("key1", "value1")
	tr.StateSet("key2", 42)

	v1, ok := tr.StateGet("key1")
	if !ok || v1 != "value1" {
		t.Errorf("StateGet(key1) = %v, %v; want value1, true", v1, ok)
	}

	v2, ok := tr.StateGet("key2")
	if !ok || v2 != 42 {
		t.Errorf("StateGet(key2) = %v, %v; want 42, true", v2, ok)
	}

	_, ok = tr.StateGet("nonexistent")
	if ok {
		t.Error("StateGet(nonexistent) should return false")
	}

	// Test StateDelete
	tr.StateSet("to_delete", "temp")
	v, ok := tr.StateGet("to_delete")
	if !ok || v != "temp" {
		t.Errorf("StateGet(to_delete) = %v, %v; want temp, true", v, ok)
	}

	tr.StateDelete("to_delete")
	_, ok = tr.StateGet("to_delete")
	if ok {
		t.Error("StateGet(to_delete) should return false after delete")
	}

	// Delete non-existent key should not panic
	tr.StateDelete("nonexistent")
}

func TestToolRuntime_History(t *testing.T) {
	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")

	tr := NewToolRuntime(ctx, mockRT, mockState, nil)

	// Append messages
	err := tr.HistoryAppend(agentcfg.Message{
		Role:    agentcfg.RoleUser,
		Content: "Hello",
	})
	if err != nil {
		t.Fatalf("HistoryAppend error: %v", err)
	}

	err = tr.HistoryAppend(agentcfg.Message{
		Role:    agentcfg.RoleModel,
		Content: "Hi there!",
	})
	if err != nil {
		t.Fatalf("HistoryAppend error: %v", err)
	}

	// Get recent messages
	messages, err := tr.HistoryRecent(0)
	if err != nil {
		t.Fatalf("HistoryRecent error: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("HistoryRecent count = %d, want 2", len(messages))
	}

	if messages[0].Content != "Hello" {
		t.Errorf("messages[0].Content = %q, want %q", messages[0].Content, "Hello")
	}

	// Test with limit
	messages, err = tr.HistoryRecent(1)
	if err != nil {
		t.Fatalf("HistoryRecent(1) error: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("HistoryRecent(1) count = %d, want 1", len(messages))
	}

	// The last message should be "Hi there!"
	if messages[0].Content != "Hi there!" {
		t.Errorf("messages[0].Content = %q, want %q", messages[0].Content, "Hi there!")
	}
}

func TestToolRuntime_HistoryRevert(t *testing.T) {
	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")

	tr := NewToolRuntime(ctx, mockRT, mockState, nil)

	// Add some messages
	tr.HistoryAppend(agentcfg.Message{Role: agentcfg.RoleUser, Content: "First"})
	tr.HistoryAppend(agentcfg.Message{Role: agentcfg.RoleModel, Content: "Response 1"})
	tr.HistoryAppend(agentcfg.Message{Role: agentcfg.RoleUser, Content: "Second"})
	tr.HistoryAppend(agentcfg.Message{Role: agentcfg.RoleModel, Content: "Response 2"})

	// Revert should remove from last user message
	err := tr.HistoryRevert()
	if err != nil {
		t.Fatalf("HistoryRevert error: %v", err)
	}

	messages, _ := tr.HistoryRecent(0)
	if len(messages) != 2 {
		t.Fatalf("After revert, message count = %d, want 2", len(messages))
	}

	if messages[1].Content != "Response 1" {
		t.Errorf("Last message = %q, want %q", messages[1].Content, "Response 1")
	}
}

func TestToolRuntime_Memory(t *testing.T) {
	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")

	tr := NewToolRuntime(ctx, mockRT, mockState, nil)

	// Test Summary
	summary, err := tr.MemorySummary()
	if err != nil {
		t.Fatalf("MemorySummary error: %v", err)
	}
	if summary != "" {
		t.Errorf("Initial summary = %q, want empty", summary)
	}

	// Set summary
	err = tr.MemorySetSummary("This is a test summary")
	if err != nil {
		t.Fatalf("MemorySetSummary error: %v", err)
	}

	summary, err = tr.MemorySummary()
	if err != nil {
		t.Fatalf("MemorySummary error: %v", err)
	}
	if summary != "This is a test summary" {
		t.Errorf("Summary = %q, want %q", summary, "This is a test summary")
	}

	// Test Query (mock returns empty)
	segments, err := tr.MemoryQuery("test query")
	if err != nil {
		t.Fatalf("MemoryQuery error: %v", err)
	}
	if segments != nil {
		t.Errorf("MemoryQuery segments = %v, want nil", segments)
	}
}

func TestToolRuntime_Log(t *testing.T) {
	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test-agent", "")
	logger := &TestLogger{}

	tr := NewToolRuntime(ctx, mockRT, mockState, &ToolRuntimeConfig{
		Logger: logger,
	})

	tr.Log("info", "test message", 123)

	if len(logger.Messages) != 1 {
		t.Fatalf("Log message count = %d, want 1", len(logger.Messages))
	}

	if logger.Messages[0].Level != "info" {
		t.Errorf("Log level = %q, want %q", logger.Messages[0].Level, "info")
	}
}

func TestToolRuntime_NilState(t *testing.T) {
	ctx := context.Background()
	mockRT := NewMockRuntime()

	tr := NewToolRuntime(ctx, mockRT, nil, nil)

	// State operations should return appropriate values when state is nil
	_, ok := tr.StateGet("key")
	if ok {
		t.Error("StateGet with nil state should return false")
	}

	// StateSet should not panic
	tr.StateSet("key", "value")

	// StateID should return empty
	if tr.StateID() != "" {
		t.Errorf("StateID with nil state = %q, want empty", tr.StateID())
	}

	// History operations should return error
	_, err := tr.HistoryRecent(0)
	if err == nil {
		t.Error("HistoryRecent with nil state should return error")
	}

	err = tr.HistoryAppend(agentcfg.Message{})
	if err == nil {
		t.Error("HistoryAppend with nil state should return error")
	}

	err = tr.HistoryRevert()
	if err == nil {
		t.Error("HistoryRevert with nil state should return error")
	}

	// Memory operations should return error
	_, err = tr.MemorySummary()
	if err == nil {
		t.Error("MemorySummary with nil state should return error")
	}

	err = tr.MemorySetSummary("test")
	if err == nil {
		t.Error("MemorySetSummary with nil state should return error")
	}

	_, err = tr.MemoryQuery("test")
	if err == nil {
		t.Error("MemoryQuery with nil state should return error")
	}
}
