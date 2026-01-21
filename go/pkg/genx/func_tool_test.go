package genx

import (
	"context"
	"testing"
)

type TestArg struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestNewFuncTool_Basic(t *testing.T) {
	tool, err := NewFuncTool[TestArg]("test_tool", "A test tool")
	if err != nil {
		t.Fatalf("NewFuncTool error: %v", err)
	}

	if tool.Name != "test_tool" {
		t.Errorf("Name = %q, want %q", tool.Name, "test_tool")
	}

	if tool.Description != "A test tool" {
		t.Errorf("Description = %q, want %q", tool.Description, "A test tool")
	}

	if tool.Argument == nil {
		t.Error("Argument should not be nil")
	}

	// Verify isTool interface
	var _ Tool = tool
	tool.isTool() // Should not panic
}

func TestNewFuncTool_WithInvokeFunc(t *testing.T) {
	invokeFn := InvokeFunc[TestArg](func(ctx context.Context, call *FuncCall, arg TestArg) (any, error) {
		return map[string]any{
			"received_name":  arg.Name,
			"received_value": arg.Value,
		}, nil
	})

	tool, err := NewFuncTool[TestArg]("test_tool", "A test tool", invokeFn)
	if err != nil {
		t.Fatalf("NewFuncTool error: %v", err)
	}

	// Create a FuncCall and invoke
	call := tool.NewFuncCall(`{"name": "test", "value": 42}`)

	result, err := call.Invoke(context.Background())
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}

	if resultMap["received_name"] != "test" {
		t.Errorf("received_name = %v, want %q", resultMap["received_name"], "test")
	}

	if resultMap["received_value"] != 42 {
		t.Errorf("received_value = %v, want 42", resultMap["received_value"])
	}
}

func TestMustNewFuncTool(t *testing.T) {
	tool := MustNewFuncTool[TestArg]("must_tool", "Must tool description")

	if tool.Name != "must_tool" {
		t.Errorf("Name = %q, want %q", tool.Name, "must_tool")
	}
}

func TestFuncTool_NewFuncCall(t *testing.T) {
	tool, _ := NewFuncTool[TestArg]("test_tool", "Test tool")

	call := tool.NewFuncCall(`{"name": "foo", "value": 100}`)

	if call.Name != "test_tool" {
		t.Errorf("FuncCall.Name = %q, want %q", call.Name, "test_tool")
	}

	if call.Arguments != `{"name": "foo", "value": 100}` {
		t.Errorf("FuncCall.Arguments = %q", call.Arguments)
	}

	if call.tool != tool {
		t.Error("FuncCall.tool should reference the parent tool")
	}
}

func TestFuncTool_DefaultInvoke(t *testing.T) {
	// Default invoke function should unmarshal JSON and return the struct
	tool, _ := NewFuncTool[TestArg]("test_tool", "Test tool")

	call := tool.NewFuncCall(`{"name": "default", "value": 999}`)

	result, err := call.Invoke(context.Background())
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	arg, ok := result.(*TestArg)
	if !ok {
		t.Fatalf("result type = %T, want *TestArg", result)
	}

	if arg.Name != "default" {
		t.Errorf("arg.Name = %q, want %q", arg.Name, "default")
	}

	if arg.Value != 999 {
		t.Errorf("arg.Value = %d, want 999", arg.Value)
	}
}

func TestFuncCall_Invoke_NoTool(t *testing.T) {
	call := &FuncCall{
		Name:      "orphan_call",
		Arguments: `{}`,
		tool:      nil, // No tool associated
	}

	_, err := call.Invoke(context.Background())
	if err == nil {
		t.Error("Invoke should fail when tool is nil")
	}
}

func TestToolCall_Invoke(t *testing.T) {
	invokeFn := InvokeFunc[TestArg](func(ctx context.Context, call *FuncCall, arg TestArg) (any, error) {
		return "invoked", nil
	})

	tool, _ := NewFuncTool[TestArg]("test_tool", "Test", invokeFn)
	funcCall := tool.NewFuncCall(`{"name": "x", "value": 1}`)

	toolCall := &ToolCall{
		ID:       "call_123",
		FuncCall: funcCall,
	}

	result, err := toolCall.Invoke(context.Background())
	if err != nil {
		t.Fatalf("ToolCall.Invoke error: %v", err)
	}

	if result != "invoked" {
		t.Errorf("result = %v, want %q", result, "invoked")
	}
}

func TestToolCall_Invoke_NilFuncCall(t *testing.T) {
	toolCall := &ToolCall{
		ID:       "call_123",
		FuncCall: nil,
	}

	_, err := toolCall.Invoke(context.Background())
	if err == nil {
		t.Error("ToolCall.Invoke should fail when FuncCall is nil")
	}
}

func TestFuncTool_InvokeWithMalformedJSON(t *testing.T) {
	// Test that malformed JSON can be repaired
	invokeFn := InvokeFunc[TestArg](func(ctx context.Context, call *FuncCall, arg TestArg) (any, error) {
		return arg, nil
	})

	tool, _ := NewFuncTool[TestArg]("test_tool", "Test", invokeFn)

	// Malformed JSON with trailing comma
	call := tool.NewFuncCall(`{"name": "test", "value": 42,}`)

	result, err := call.Invoke(context.Background())
	if err != nil {
		t.Fatalf("Invoke should repair malformed JSON: %v", err)
	}

	arg, ok := result.(TestArg)
	if !ok {
		t.Fatalf("result type = %T, want TestArg", result)
	}

	if arg.Name != "test" {
		t.Errorf("arg.Name = %q, want %q", arg.Name, "test")
	}
}
