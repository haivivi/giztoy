package agent_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/agent"
	"github.com/haivivi/giztoy/go/pkg/genx/agentcfg"
	"github.com/haivivi/giztoy/go/pkg/genx/playground"
)

// mockTextProcessorGenerator creates a mock generator for text processor tests.
type mockTextProcessorGenerator struct {
	responses map[string]string // model -> response
}

func (g *mockTextProcessorGenerator) GenerateStream(ctx context.Context, model string, mc genx.ModelContext) (genx.Stream, error) {
	response := g.responses[model]
	if response == "" {
		response = "Mock processed text for " + model
	}
	return &mockTextProcessorStream{response: response}, nil
}

func (g *mockTextProcessorGenerator) Invoke(ctx context.Context, model string, mc genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	// For JSON mode, return a function call with structured output
	response := g.responses[model]
	if response == "" {
		response = `{"title": "Mock Title", "summary": "Mock summary", "keywords": ["mock", "test"]}`
	}
	funcCall := tool.NewFuncCall(response)
	return genx.Usage{}, funcCall, nil
}

type mockTextProcessorStream struct {
	response string
	done     bool
}

func (s *mockTextProcessorStream) Next() (*genx.MessageChunk, error) {
	if s.done {
		return nil, genx.Done(genx.Usage{})
	}
	s.done = true
	return &genx.MessageChunk{
		Role: genx.RoleModel,
		Part: genx.Text(s.response),
	}, nil
}

func (s *mockTextProcessorStream) Close() error               { return nil }
func (s *mockTextProcessorStream) CloseWithError(error) error { return nil }

func setupTextProcessorTestRuntime(t *testing.T, gen genx.Generator) *playground.Runtime {
	t.Helper()
	store := playground.NewStore(nil)
	if err := store.LoadReadonlyLayer("testdata", os.DirFS("testdata/tool_text_processor_test")); err != nil {
		t.Fatalf("load testdata: %v", err)
	}

	return playground.NewRuntime(
		playground.WithStore(store),
		playground.WithGenerator(gen),
	)
}

func TestTextProcessorTool_TextMode(t *testing.T) {
	ctx := context.Background()
	mockGen := &mockTextProcessorGenerator{
		responses: map[string]string{
			"test-model": "This is a concise summary of the input text.",
		},
	}
	rt := setupTextProcessorTestRuntime(t, mockGen)

	// Get tool def from store
	toolDef, err := rt.GetToolDef(ctx, "summarizer")
	if err != nil {
		t.Fatalf("GetToolDef error: %v", err)
	}

	textDef := agentcfg.AsTextProcessorTool(toolDef)
	if textDef == nil {
		t.Fatalf("expected TextProcessorToolDef, got %T", toolDef)
	}

	if textDef.Name != "summarizer" {
		t.Errorf("Name = %q, want %q", textDef.Name, "summarizer")
	}
	if textDef.OutputMode != "text" {
		t.Errorf("OutputMode = %q, want %q", textDef.OutputMode, "text")
	}

	// Create and execute the tool
	textTool := agent.NewTextProcessorTool(rt)
	funcTool, err := textTool.CreateFuncTool(textDef)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := funcTool.NewFuncCall(`{"content": "Long text that needs summarization..."}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	if !strings.Contains(resultStr, "summary") {
		t.Errorf("result = %q, want to contain %q", resultStr, "summary")
	}
}

func TestTextProcessorTool_JSONMode(t *testing.T) {
	ctx := context.Background()
	mockGen := &mockTextProcessorGenerator{
		responses: map[string]string{
			"test-model": `{"title": "Extracted Title", "summary": "Extracted summary from text", "keywords": ["key1", "key2"]}`,
		},
	}
	rt := setupTextProcessorTestRuntime(t, mockGen)

	// Get tool def from store
	toolDef, err := rt.GetToolDef(ctx, "json_extractor")
	if err != nil {
		t.Fatalf("GetToolDef error: %v", err)
	}

	textDef := agentcfg.AsTextProcessorTool(toolDef)
	if textDef == nil {
		t.Fatalf("expected TextProcessorToolDef, got %T", toolDef)
	}

	if textDef.OutputMode != "json" {
		t.Errorf("OutputMode = %q, want %q", textDef.OutputMode, "json")
	}
	if textDef.OutputSchema == nil {
		t.Fatal("OutputSchema is nil")
	}

	// Create and execute the tool
	textTool := agent.NewTextProcessorTool(rt)
	funcTool, err := textTool.CreateFuncTool(textDef)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := funcTool.NewFuncCall(`{"content": "Some text with important information about AI and machine learning."}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	if !strings.Contains(resultStr, "Extracted Title") {
		t.Errorf("result = %q, want to contain %q", resultStr, "Extracted Title")
	}
}

func TestTextProcessorTool_GetToolFromStore(t *testing.T) {
	ctx := context.Background()
	mockGen := &mockTextProcessorGenerator{
		responses: map[string]string{
			"test-model": "Translated: Hello World",
		},
	}
	rt := setupTextProcessorTestRuntime(t, mockGen)

	// Get tool directly (uses CreateToolFromDef internally)
	tool, err := rt.GetTool(ctx, "translator")
	if err != nil {
		t.Fatalf("GetTool error: %v", err)
	}

	call := tool.NewFuncCall(`{"content": "你好世界"}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	if !strings.Contains(resultStr, "Translated") {
		t.Errorf("result = %q, want to contain %q", resultStr, "Translated")
	}
}

func TestTextProcessorTool_Execute_TextMode(t *testing.T) {
	ctx := context.Background()
	mockGen := &mockTextProcessorGenerator{
		responses: map[string]string{
			"test-model": "Summarized content here.",
		},
	}
	rt := setupTextProcessorTestRuntime(t, mockGen)

	textTool := agent.NewTextProcessorTool(rt)

	// Execute with inline definition
	ref := &agentcfg.TextProcessorToolRef{
		TextProcessorTool: &agentcfg.TextProcessorTool{
			ToolBase: agentcfg.ToolBase{
				Name:        "inline_processor",
				Description: "Inline text processor",
			},
			Model:      "test-model",
			Prompt:     "Process the text:",
			OutputMode: "text",
		},
	}

	result, err := textTool.Execute(ctx, ref, "Input text to process")
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if !strings.Contains(result, "Summarized") {
		t.Errorf("result = %q, want to contain %q", result, "Summarized")
	}
}

func TestTextProcessorTool_Execute_ByRef(t *testing.T) {
	ctx := context.Background()
	mockGen := &mockTextProcessorGenerator{
		responses: map[string]string{
			"test-model": "Summary from referenced processor.",
		},
	}
	rt := setupTextProcessorTestRuntime(t, mockGen)

	textTool := agent.NewTextProcessorTool(rt)

	// Execute by reference
	ref := &agentcfg.TextProcessorToolRef{
		Ref: "summarizer",
	}

	result, err := textTool.Execute(ctx, ref, "Input text to summarize")
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if !strings.Contains(result, "Summary") {
		t.Errorf("result = %q, want to contain %q", result, "Summary")
	}
}

func TestTextProcessorTool_Execute_NilRef(t *testing.T) {
	ctx := context.Background()
	mockGen := &mockTextProcessorGenerator{}
	rt := setupTextProcessorTestRuntime(t, mockGen)

	textTool := agent.NewTextProcessorTool(rt)

	// Execute with nil ref - should return content unchanged
	result, err := textTool.Execute(ctx, nil, "Original content")
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if result != "Original content" {
		t.Errorf("result = %q, want %q", result, "Original content")
	}
}

func TestTextProcessorTool_Execute_EmptyRef(t *testing.T) {
	ctx := context.Background()
	mockGen := &mockTextProcessorGenerator{}
	rt := setupTextProcessorTestRuntime(t, mockGen)

	textTool := agent.NewTextProcessorTool(rt)

	// Execute with empty ref - should return content unchanged
	ref := &agentcfg.TextProcessorToolRef{}
	result, err := textTool.Execute(ctx, ref, "Original content")
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if result != "Original content" {
		t.Errorf("result = %q, want %q", result, "Original content")
	}
}

func TestTextProcessorToolDef_FromStore(t *testing.T) {
	ctx := context.Background()
	mockGen := &mockTextProcessorGenerator{}
	rt := setupTextProcessorTestRuntime(t, mockGen)

	tests := []struct {
		name       string
		toolName   string
		wantMode   string
		wantSchema bool
	}{
		{"summarizer_text_mode", "summarizer", "text", false},
		{"json_extractor_json_mode", "json_extractor", "json", true},
		{"translator_text_mode", "translator", "text", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolDef, err := rt.GetToolDef(ctx, tt.toolName)
			if err != nil {
				t.Fatalf("GetToolDef error: %v", err)
			}

			textDef := agentcfg.AsTextProcessorTool(toolDef)
			if textDef == nil {
				t.Fatalf("expected TextProcessorToolDef, got %T", toolDef)
			}

			if string(textDef.OutputMode) != tt.wantMode {
				t.Errorf("OutputMode = %q, want %q", textDef.OutputMode, tt.wantMode)
			}

			hasSchema := textDef.OutputSchema != nil
			if hasSchema != tt.wantSchema {
				t.Errorf("has OutputSchema = %v, want %v", hasSchema, tt.wantSchema)
			}
		})
	}
}
