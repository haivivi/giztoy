package genx

import (
	"errors"
	"testing"
)

func TestStatus_Constants(t *testing.T) {
	// Verify status constants are defined
	if StatusOK != 0 {
		t.Errorf("StatusOK = %d, want 0", StatusOK)
	}

	if StatusDone <= StatusOK {
		t.Error("StatusDone should be > StatusOK")
	}

	if StatusTruncated <= StatusDone {
		t.Error("StatusTruncated should be > StatusDone")
	}

	if StatusBlocked <= StatusTruncated {
		t.Error("StatusBlocked should be > StatusTruncated")
	}

	if StatusError <= StatusBlocked {
		t.Error("StatusError should be > StatusBlocked")
	}
}

func TestNewStreamBuilder(t *testing.T) {
	mcb := &ModelContextBuilder{}
	tool := MustNewFuncTool[TestArg]("test_tool", "Test tool")
	mcb.AddTool(tool)

	mctx := mcb.Build()

	sb := NewStreamBuilder(mctx, 10)
	if sb == nil {
		t.Fatal("NewStreamBuilder returned nil")
	}

	if sb.rb == nil {
		t.Error("StreamBuilder.rb should not be nil")
	}

	if len(sb.funcTools) != 1 {
		t.Errorf("funcTools count = %d, want 1", len(sb.funcTools))
	}

	if sb.funcTools["test_tool"] != tool {
		t.Error("funcTools should contain the added tool")
	}
}

func TestNewStreamBuilder_NoTools(t *testing.T) {
	mcb := &ModelContextBuilder{}
	mctx := mcb.Build()

	sb := NewStreamBuilder(mctx, 10)
	if sb == nil {
		t.Fatal("NewStreamBuilder returned nil")
	}

	if len(sb.funcTools) != 0 {
		t.Errorf("funcTools count = %d, want 0", len(sb.funcTools))
	}
}

func TestNewStreamBuilder_WithSearchWebTool(t *testing.T) {
	mcb := &ModelContextBuilder{}
	mcb.AddTool(&SearchWebTool{})
	mctx := mcb.Build()

	sb := NewStreamBuilder(mctx, 10)
	if sb == nil {
		t.Fatal("NewStreamBuilder returned nil")
	}

	// SearchWebTool should not be added to funcTools (only FuncTool is)
	if len(sb.funcTools) != 0 {
		t.Errorf("funcTools count = %d, want 0 (SearchWebTool is not a FuncTool)", len(sb.funcTools))
	}
}

func TestStreamBuilder_Done(t *testing.T) {
	mcb := &ModelContextBuilder{}
	mctx := mcb.Build()

	sb := NewStreamBuilder(mctx, 10)

	usage := Usage{
		PromptTokenCount:    100,
		GeneratedTokenCount: 50,
	}

	err := sb.Done(usage)
	if err != nil {
		t.Fatalf("Done error: %v", err)
	}

	// Get stream and verify it returns done state
	stream := sb.Stream()
	_, err = stream.Next()
	if !errors.Is(err, ErrDone) {
		t.Errorf("Stream should return ErrDone, got %v", err)
	}
}

func TestStreamBuilder_Truncated(t *testing.T) {
	mcb := &ModelContextBuilder{}
	mctx := mcb.Build()

	sb := NewStreamBuilder(mctx, 10)

	usage := Usage{GeneratedTokenCount: 4096}

	err := sb.Truncated(usage)
	if err != nil {
		t.Fatalf("Truncated error: %v", err)
	}

	stream := sb.Stream()
	_, err = stream.Next()

	var state *State
	if errors.As(err, &state) {
		if state.Status() != StatusTruncated {
			t.Errorf("Status = %v, want %v", state.Status(), StatusTruncated)
		}
	} else {
		t.Errorf("Expected *State error, got %T", err)
	}
}

func TestStreamBuilder_Blocked(t *testing.T) {
	mcb := &ModelContextBuilder{}
	mctx := mcb.Build()

	sb := NewStreamBuilder(mctx, 10)

	usage := Usage{PromptTokenCount: 100}

	err := sb.Blocked(usage, "content policy violation")
	if err != nil {
		t.Fatalf("Blocked error: %v", err)
	}

	stream := sb.Stream()
	_, err = stream.Next()

	var state *State
	if errors.As(err, &state) {
		if state.Status() != StatusBlocked {
			t.Errorf("Status = %v, want %v", state.Status(), StatusBlocked)
		}
	} else {
		t.Errorf("Expected *State error, got %T", err)
	}
}

func TestStreamBuilder_Unexpected(t *testing.T) {
	mcb := &ModelContextBuilder{}
	mctx := mcb.Build()

	sb := NewStreamBuilder(mctx, 10)

	usage := Usage{PromptTokenCount: 50}
	originalErr := errors.New("network error")

	err := sb.Unexpected(usage, originalErr)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	stream := sb.Stream()
	_, err = stream.Next()

	var state *State
	if errors.As(err, &state) {
		if state.Status() != StatusError {
			t.Errorf("Status = %v, want %v", state.Status(), StatusError)
		}
	} else {
		t.Errorf("Expected *State error, got %T", err)
	}
}

func TestStreamBuilder_Add(t *testing.T) {
	mcb := &ModelContextBuilder{}
	mctx := mcb.Build()

	sb := NewStreamBuilder(mctx, 10)

	chunks := []*MessageChunk{
		{Role: RoleModel, Part: Text("Hello")},
		{Role: RoleModel, Part: Text(" World")},
	}

	err := sb.Add(chunks...)
	if err != nil {
		t.Fatalf("Add error: %v", err)
	}

	sb.Done(Usage{})

	stream := sb.Stream()

	// Read first chunk
	chunk1, err := stream.Next()
	if err != nil {
		t.Fatalf("First Next error: %v", err)
	}
	if chunk1.Part.(Text) != "Hello" {
		t.Errorf("First chunk = %v, want 'Hello'", chunk1.Part)
	}

	// Read second chunk
	chunk2, err := stream.Next()
	if err != nil {
		t.Fatalf("Second Next error: %v", err)
	}
	if chunk2.Part.(Text) != " World" {
		t.Errorf("Second chunk = %v, want ' World'", chunk2.Part)
	}
}

func TestStreamBuilder_Add_WithToolCall(t *testing.T) {
	tool := MustNewFuncTool[TestArg]("get_weather", "Get weather")

	mcb := &ModelContextBuilder{}
	mcb.AddTool(tool)
	mctx := mcb.Build()

	sb := NewStreamBuilder(mctx, 10)

	chunk := &MessageChunk{
		Role: RoleModel,
		ToolCall: &ToolCall{
			ID:       "call_123",
			FuncCall: &FuncCall{Name: "get_weather", Arguments: `{"city": "NYC"}`},
		},
	}

	err := sb.Add(chunk)
	if err != nil {
		t.Fatalf("Add error: %v", err)
	}

	// Verify tool reference was set
	if chunk.ToolCall.FuncCall.tool != tool {
		t.Error("Tool reference should be set")
	}

	sb.Done(Usage{})
}

func TestStreamBuilder_Add_UnknownTool(t *testing.T) {
	mcb := &ModelContextBuilder{}
	// Don't add any tools
	mctx := mcb.Build()

	sb := NewStreamBuilder(mctx, 10)

	chunk := &MessageChunk{
		Role: RoleModel,
		ToolCall: &ToolCall{
			ID:       "call_123",
			FuncCall: &FuncCall{Name: "unknown_tool", Arguments: `{}`},
		},
	}

	// Should not error, just log warning and skip
	err := sb.Add(chunk)
	if err != nil {
		t.Fatalf("Add error: %v", err)
	}
}

func TestStreamBuilder_Abort(t *testing.T) {
	mcb := &ModelContextBuilder{}
	mctx := mcb.Build()

	sb := NewStreamBuilder(mctx, 10)

	abortErr := errors.New("aborted")
	err := sb.Abort(abortErr)
	if err != nil {
		t.Fatalf("Abort error: %v", err)
	}

	stream := sb.Stream()
	_, err = stream.Next()
	if !errors.Is(err, abortErr) {
		t.Errorf("Expected abort error, got %v", err)
	}
}

func TestStreamBuilder_Stream(t *testing.T) {
	mcb := &ModelContextBuilder{}
	mctx := mcb.Build()

	sb := NewStreamBuilder(mctx, 10)

	stream := sb.Stream()
	if stream == nil {
		t.Fatal("Stream returned nil")
	}

	// Verify it's the correct type
	if _, ok := stream.(*streamImpl); !ok {
		t.Errorf("Stream type = %T, want *streamImpl", stream)
	}
}

func TestStreamImpl_Close(t *testing.T) {
	mcb := &ModelContextBuilder{}
	mctx := mcb.Build()

	sb := NewStreamBuilder(mctx, 10)
	stream := sb.Stream()

	err := stream.Close()
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

func TestStreamImpl_CloseWithError(t *testing.T) {
	mcb := &ModelContextBuilder{}
	mctx := mcb.Build()

	sb := NewStreamBuilder(mctx, 10)
	stream := sb.Stream()

	customErr := errors.New("custom close error")
	err := stream.CloseWithError(customErr)
	if err != nil {
		t.Fatalf("CloseWithError error: %v", err)
	}
}
