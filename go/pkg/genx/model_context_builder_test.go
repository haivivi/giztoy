package genx

import (
	"slices"
	"testing"
)

func TestModelContextBuilder_Build(t *testing.T) {
	mcb := &ModelContextBuilder{
		Prompts: []*Prompt{
			{Name: "system", Text: "You are helpful"},
		},
		Messages: []*Message{
			{Role: RoleUser, Payload: Contents{Text("Hello")}},
		},
		CoTs: []string{"think step by step"},
		Params: &ModelParams{
			MaxTokens:   100,
			Temperature: 0.7,
		},
	}

	mctx := mcb.Build()

	// Verify prompts
	promptCount := 0
	for p := range mctx.Prompts() {
		if p.Name != "system" {
			t.Errorf("Prompt name = %q, want %q", p.Name, "system")
		}
		promptCount++
	}
	if promptCount != 1 {
		t.Errorf("Prompt count = %d, want 1", promptCount)
	}

	// Verify messages
	msgCount := 0
	for m := range mctx.Messages() {
		if m.Role != RoleUser {
			t.Errorf("Message role = %v, want %v", m.Role, RoleUser)
		}
		msgCount++
	}
	if msgCount != 1 {
		t.Errorf("Message count = %d, want 1", msgCount)
	}

	// Verify CoTs
	cotCount := 0
	for c := range mctx.CoTs() {
		if c != "think step by step" {
			t.Errorf("CoT = %q", c)
		}
		cotCount++
	}
	if cotCount != 1 {
		t.Errorf("CoT count = %d, want 1", cotCount)
	}

	// Verify params
	params := mctx.Params()
	if params.MaxTokens != 100 {
		t.Errorf("MaxTokens = %d, want 100", params.MaxTokens)
	}
}

func TestModelContextBuilder_AddPrompt(t *testing.T) {
	mcb := &ModelContextBuilder{}

	mcb.AddPrompt(&Prompt{Name: "system", Text: "Line 1"})
	mcb.AddPrompt(&Prompt{Name: "system", Text: "Line 2"})

	// Same name should merge
	if len(mcb.Prompts) != 1 {
		t.Fatalf("Prompt count = %d, want 1 (merged)", len(mcb.Prompts))
	}

	if mcb.Prompts[0].Text != "Line 1\nLine 2" {
		t.Errorf("Prompt text = %q, want merged text", mcb.Prompts[0].Text)
	}

	// Different name should not merge
	mcb.AddPrompt(&Prompt{Name: "assistant", Text: "I am an assistant"})
	if len(mcb.Prompts) != 2 {
		t.Fatalf("Prompt count = %d, want 2", len(mcb.Prompts))
	}
}

func TestModelContextBuilder_AddPrompt_EmptyInitial(t *testing.T) {
	mcb := &ModelContextBuilder{}

	mcb.AddPrompt(&Prompt{Name: "system", Text: ""})
	mcb.AddPrompt(&Prompt{Name: "system", Text: "Added text"})

	if mcb.Prompts[0].Text != "Added text" {
		t.Errorf("Prompt text = %q, want %q", mcb.Prompts[0].Text, "Added text")
	}
}

func TestModelContextBuilder_AddMessage(t *testing.T) {
	mcb := &ModelContextBuilder{}

	mcb.AddMessage(&Message{
		Role:    RoleUser,
		Name:    "user1",
		Payload: Contents{Text("Hello")},
	})

	mcb.AddMessage(&Message{
		Role:    RoleUser,
		Name:    "user1",
		Payload: Contents{Text(" World")},
	})

	// Same role and name with Contents should merge
	if len(mcb.Messages) != 1 {
		t.Fatalf("Message count = %d, want 1 (merged)", len(mcb.Messages))
	}

	contents := mcb.Messages[0].Payload.(Contents)
	if len(contents) != 2 {
		t.Errorf("Contents count = %d, want 2", len(contents))
	}
}

func TestModelContextBuilder_AddMessage_DifferentRole(t *testing.T) {
	mcb := &ModelContextBuilder{}

	mcb.AddMessage(&Message{
		Role:    RoleUser,
		Payload: Contents{Text("User message")},
	})

	mcb.AddMessage(&Message{
		Role:    RoleModel,
		Payload: Contents{Text("Model response")},
	})

	// Different role should not merge
	if len(mcb.Messages) != 2 {
		t.Fatalf("Message count = %d, want 2", len(mcb.Messages))
	}
}

func TestModelContextBuilder_AddMessage_NonContentsPayload(t *testing.T) {
	mcb := &ModelContextBuilder{}

	mcb.AddMessage(&Message{
		Role:    RoleModel,
		Payload: &ToolCall{ID: "call_1"},
	})

	mcb.AddMessage(&Message{
		Role:    RoleModel,
		Payload: Contents{Text("Text after tool call")},
	})

	// ToolCall payload should not merge
	if len(mcb.Messages) != 2 {
		t.Fatalf("Message count = %d, want 2", len(mcb.Messages))
	}
}

func TestModelContextBuilder_AddTool(t *testing.T) {
	mcb := &ModelContextBuilder{}

	tool1 := &FuncTool{Name: "tool1"}
	tool2 := &SearchWebTool{}

	mcb.AddTool(tool1)
	mcb.AddTool(tool2)

	if len(mcb.Tools) != 2 {
		t.Fatalf("Tool count = %d, want 2", len(mcb.Tools))
	}
}

func TestModelContextBuilder_SetCoT(t *testing.T) {
	mcb := &ModelContextBuilder{}

	mcb.SetCoT("Step 1", "Step 2")

	if len(mcb.CoTs) != 2 {
		t.Fatalf("CoT count = %d, want 2", len(mcb.CoTs))
	}

	if mcb.CoTs[0] != "Step 1" {
		t.Errorf("CoT[0] = %q, want %q", mcb.CoTs[0], "Step 1")
	}
}

func TestModelContextBuilder_SetCoT_NonString(t *testing.T) {
	mcb := &ModelContextBuilder{}

	// Non-string should be YAML marshaled
	mcb.SetCoT(map[string]int{"count": 42})

	if len(mcb.CoTs) != 1 {
		t.Fatalf("CoT count = %d, want 1", len(mcb.CoTs))
	}

	// Should contain YAML formatted data
	if mcb.CoTs[0] == "" {
		t.Error("CoT should not be empty")
	}
}

func TestModelContextBuilder_Prompt(t *testing.T) {
	mcb := &ModelContextBuilder{}

	err := mcb.Prompt("system", "config", map[string]string{"model": "gpt-4"})
	if err != nil {
		t.Fatalf("Prompt error: %v", err)
	}

	if len(mcb.Prompts) != 1 {
		t.Fatalf("Prompt count = %d, want 1", len(mcb.Prompts))
	}

	if mcb.Prompts[0].Name != "system" {
		t.Errorf("Prompt name = %q, want %q", mcb.Prompts[0].Name, "system")
	}
}

func TestModelContextBuilder_PromptText(t *testing.T) {
	mcb := &ModelContextBuilder{}

	mcb.PromptText("system", "You are a helpful assistant")

	if len(mcb.Prompts) != 1 {
		t.Fatalf("Prompt count = %d, want 1", len(mcb.Prompts))
	}

	if mcb.Prompts[0].Text != "You are a helpful assistant" {
		t.Errorf("Prompt text = %q", mcb.Prompts[0].Text)
	}
}

func TestModelContextBuilder_UserText(t *testing.T) {
	mcb := &ModelContextBuilder{}

	mcb.UserText("alice", "Hello!")

	if len(mcb.Messages) != 1 {
		t.Fatalf("Message count = %d, want 1", len(mcb.Messages))
	}

	msg := mcb.Messages[0]
	if msg.Role != RoleUser {
		t.Errorf("Role = %v, want %v", msg.Role, RoleUser)
	}
	if msg.Name != "alice" {
		t.Errorf("Name = %q, want %q", msg.Name, "alice")
	}

	contents := msg.Payload.(Contents)
	if len(contents) != 1 {
		t.Fatalf("Contents count = %d, want 1", len(contents))
	}
	if text, ok := contents[0].(Text); !ok || text != "Hello!" {
		t.Errorf("Text content = %v", contents[0])
	}
}

func TestModelContextBuilder_UserBlob(t *testing.T) {
	mcb := &ModelContextBuilder{}

	mcb.UserBlob("alice", "image/png", []byte{1, 2, 3})

	msg := mcb.Messages[0]
	contents := msg.Payload.(Contents)
	blob := contents[0].(*Blob)

	if blob.MIMEType != "image/png" {
		t.Errorf("MIMEType = %q, want %q", blob.MIMEType, "image/png")
	}
	if !slices.Equal(blob.Data, []byte{1, 2, 3}) {
		t.Errorf("Data = %v, want [1,2,3]", blob.Data)
	}
}

func TestModelContextBuilder_ModelText(t *testing.T) {
	mcb := &ModelContextBuilder{}

	mcb.ModelText("assistant", "Hello, how can I help?")

	msg := mcb.Messages[0]
	if msg.Role != RoleModel {
		t.Errorf("Role = %v, want %v", msg.Role, RoleModel)
	}
}

func TestModelContextBuilder_ModelBlob(t *testing.T) {
	mcb := &ModelContextBuilder{}

	mcb.ModelBlob("assistant", "audio/mp3", []byte{0xff, 0xfb})

	msg := mcb.Messages[0]
	if msg.Role != RoleModel {
		t.Errorf("Role = %v, want %v", msg.Role, RoleModel)
	}

	contents := msg.Payload.(Contents)
	blob := contents[0].(*Blob)
	if blob.MIMEType != "audio/mp3" {
		t.Errorf("MIMEType = %q, want %q", blob.MIMEType, "audio/mp3")
	}
}

func TestModelContextBuilder_AddToolCallResult(t *testing.T) {
	mcb := &ModelContextBuilder{}

	err := mcb.AddToolCallResult("get_weather", `{"city": "NYC"}`, `{"temp": 72}`)
	if err != nil {
		t.Fatalf("AddToolCallResult error: %v", err)
	}

	// Should add both tool call and result messages
	if len(mcb.Messages) != 2 {
		t.Fatalf("Message count = %d, want 2", len(mcb.Messages))
	}

	// First message should be tool call
	if _, ok := mcb.Messages[0].Payload.(*ToolCall); !ok {
		t.Errorf("First message should be ToolCall, got %T", mcb.Messages[0].Payload)
	}

	// Second message should be tool result
	if _, ok := mcb.Messages[1].Payload.(*ToolResult); !ok {
		t.Errorf("Second message should be ToolResult, got %T", mcb.Messages[1].Payload)
	}
}

func TestModelContextBuilder_AddToolCallResult_NonStringArgs(t *testing.T) {
	mcb := &ModelContextBuilder{}

	// Non-string arguments should be JSON marshaled
	err := mcb.AddToolCallResult("tool", map[string]int{"value": 42}, map[string]bool{"ok": true})
	if err != nil {
		t.Fatalf("AddToolCallResult error: %v", err)
	}

	if len(mcb.Messages) != 2 {
		t.Fatalf("Message count = %d, want 2", len(mcb.Messages))
	}
}

func TestModelContext_Tools(t *testing.T) {
	mcb := &ModelContextBuilder{}
	mcb.AddTool(&FuncTool{Name: "tool1"})
	mcb.AddTool(&FuncTool{Name: "tool2"})

	mctx := mcb.Build()

	toolCount := 0
	for range mctx.Tools() {
		toolCount++
	}

	if toolCount != 2 {
		t.Errorf("Tool count = %d, want 2", toolCount)
	}
}

func TestModelContext_EmptyBuilder(t *testing.T) {
	mcb := &ModelContextBuilder{}
	mctx := mcb.Build()

	// Empty iterators should not panic
	for range mctx.Prompts() {
		t.Error("Should have no prompts")
	}

	for range mctx.Messages() {
		t.Error("Should have no messages")
	}

	for range mctx.CoTs() {
		t.Error("Should have no CoTs")
	}

	for range mctx.Tools() {
		t.Error("Should have no tools")
	}

	if mctx.Params() != nil {
		t.Error("Params should be nil")
	}
}
