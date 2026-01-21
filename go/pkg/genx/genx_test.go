package genx

import (
	"strings"
	"testing"
)

func TestUsage_String(t *testing.T) {
	usage := Usage{
		PromptTokenCount:        100,
		CachedContentTokenCount: 50,
		GeneratedTokenCount:     200,
	}

	s := usage.String()

	// Should contain YAML formatted usage info
	if !strings.Contains(s, "Prompt") {
		t.Errorf("Usage.String() should contain 'Prompt', got: %s", s)
	}

	if !strings.Contains(s, "Cached") {
		t.Errorf("Usage.String() should contain 'Cached', got: %s", s)
	}

	if !strings.Contains(s, "Generated") {
		t.Errorf("Usage.String() should contain 'Generated', got: %s", s)
	}

	if !strings.Contains(s, "100") {
		t.Errorf("Usage.String() should contain prompt token count, got: %s", s)
	}
}

func TestInspectTool_FuncTool(t *testing.T) {
	tool := &FuncTool{
		Name:        "get_weather",
		Description: "Gets the current weather for a location",
	}

	result := InspectTool(tool)

	if !strings.Contains(result, "get_weather") {
		t.Errorf("InspectTool should contain tool name, got: %s", result)
	}

	if !strings.Contains(result, "Gets the current weather") {
		t.Errorf("InspectTool should contain description, got: %s", result)
	}
}

func TestInspectTool_SearchWebTool(t *testing.T) {
	tool := &SearchWebTool{}

	result := InspectTool(tool)

	if !strings.Contains(result, "SearchWebTool") {
		t.Errorf("InspectTool should identify SearchWebTool, got: %s", result)
	}
}

func TestInspectTool_UnknownTool(t *testing.T) {
	// Create a nil tool to test the default case
	result := InspectTool(nil)

	if result != "" {
		t.Errorf("InspectTool(nil) should return empty string, got: %s", result)
	}
}

func TestInspectMessage_Nil(t *testing.T) {
	result := InspectMessage(nil)

	if result != "" {
		t.Errorf("InspectMessage(nil) should return empty string, got: %s", result)
	}
}

func TestInspectMessage_WithContents(t *testing.T) {
	msg := &Message{
		Role:    RoleUser,
		Name:    "user1",
		Payload: Contents{Text("Hello, world!")},
	}

	result := InspectMessage(msg)

	if !strings.Contains(result, "user") {
		t.Errorf("InspectMessage should contain role, got: %s", result)
	}

	if !strings.Contains(result, "Hello, world!") {
		t.Errorf("InspectMessage should contain text content, got: %s", result)
	}
}

func TestInspectMessage_WithBlob(t *testing.T) {
	msg := &Message{
		Role: RoleUser,
		Name: "user1",
		Payload: Contents{
			&Blob{MIMEType: "image/png", Data: []byte{1, 2, 3, 4, 5}},
		},
	}

	result := InspectMessage(msg)

	if !strings.Contains(result, "image/png") {
		t.Errorf("InspectMessage should contain MIME type, got: %s", result)
	}

	if !strings.Contains(result, "[5]") {
		t.Errorf("InspectMessage should contain data length, got: %s", result)
	}
}

func TestInspectMessage_WithToolCall(t *testing.T) {
	msg := &Message{
		Role: RoleModel,
		Name: "assistant",
		Payload: &ToolCall{
			ID: "call_abc123",
			FuncCall: &FuncCall{
				Name:      "get_weather",
				Arguments: `{"location": "New York"}`,
			},
		},
	}

	result := InspectMessage(msg)

	if !strings.Contains(result, "call_abc123") {
		t.Errorf("InspectMessage should contain tool call ID, got: %s", result)
	}

	if !strings.Contains(result, "get_weather") {
		t.Errorf("InspectMessage should contain function name, got: %s", result)
	}
}

func TestInspectMessage_WithToolResult(t *testing.T) {
	msg := &Message{
		Role: RoleTool,
		Name: "tool",
		Payload: &ToolResult{
			ID:     "call_abc123",
			Result: `{"temperature": 72, "unit": "F"}`,
		},
	}

	result := InspectMessage(msg)

	if !strings.Contains(result, "call_abc123") {
		t.Errorf("InspectMessage should contain tool result ID, got: %s", result)
	}

	if !strings.Contains(result, "temperature") {
		t.Errorf("InspectMessage should contain result, got: %s", result)
	}
}

func TestSearchWebTool_isTool(t *testing.T) {
	tool := &SearchWebTool{}

	// Verify it implements Tool interface
	var _ Tool = tool
	tool.isTool() // Should not panic
}
