package agent_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/haivivi/giztoy/pkg/genx"
	"github.com/haivivi/giztoy/pkg/genx/agent"
	"github.com/haivivi/giztoy/pkg/genx/agentcfg"
	"github.com/haivivi/giztoy/pkg/genx/playground"
)

// createBuiltinTools creates mock tools for testing composite tool functionality.
func createBuiltinTools() []*genx.FuncTool {
	var tools []*genx.FuncTool

	// Echo tool - returns input as-is
	type echoArgs struct {
		Text string `json:"text" description:"Text to echo"`
	}
	echoTool, _ := genx.NewFuncTool[echoArgs](
		"echo",
		"Echo the input back",
		genx.InvokeFunc[echoArgs](func(ctx context.Context, call *genx.FuncCall, args echoArgs) (any, error) {
			return args.Text, nil
		}),
	)
	tools = append(tools, echoTool)

	// Uppercase tool - converts text to uppercase
	type uppercaseArgs struct {
		Text string `json:"text" description:"Text to uppercase"`
	}
	uppercaseTool, _ := genx.NewFuncTool[uppercaseArgs](
		"uppercase",
		"Convert text to uppercase",
		genx.InvokeFunc[uppercaseArgs](func(ctx context.Context, call *genx.FuncCall, args uppercaseArgs) (any, error) {
			return strings.ToUpper(args.Text), nil
		}),
	)
	tools = append(tools, uppercaseTool)

	// Concat tool - concatenates two strings
	type concatArgs struct {
		A string `json:"a" description:"First string"`
		B string `json:"b" description:"Second string"`
	}
	concatTool, _ := genx.NewFuncTool[concatArgs](
		"concat",
		"Concatenate two strings",
		genx.InvokeFunc[concatArgs](func(ctx context.Context, call *genx.FuncCall, args concatArgs) (any, error) {
			return args.A + args.B, nil
		}),
	)
	tools = append(tools, concatTool)

	// JSON output tool - returns structured data
	type jsonOutputArgs struct {
		Name  string `json:"name" description:"Name"`
		Value int    `json:"value" description:"Value"`
	}
	jsonOutputTool, _ := genx.NewFuncTool[jsonOutputArgs](
		"json_output",
		"Returns structured JSON",
		genx.InvokeFunc[jsonOutputArgs](func(ctx context.Context, call *genx.FuncCall, args jsonOutputArgs) (any, error) {
			return map[string]any{
				"name":   args.Name,
				"value":  args.Value,
				"double": args.Value * 2,
			}, nil
		}),
	)
	tools = append(tools, jsonOutputTool)

	// Extract tool - extracts a field from JSON input
	type extractArgs struct {
		Field string `json:"field" description:"Field to extract"`
	}
	extractTool, _ := genx.NewFuncTool[extractArgs](
		"extract",
		"Extract field from previous step",
		genx.InvokeFunc[extractArgs](func(ctx context.Context, call *genx.FuncCall, args extractArgs) (any, error) {
			return args.Field, nil
		}),
	)
	tools = append(tools, extractTool)

	return tools
}

func setupCompositeToolTestRuntime(t *testing.T) *playground.Runtime {
	t.Helper()
	// Load testdata with tool definitions (http, generator, text_processor)
	store := playground.NewStore(nil)
	if err := store.LoadReadonlyLayer("testdata", os.DirFS("testdata/tool_composite_test")); err != nil {
		t.Fatalf("load testdata: %v", err)
	}

	return playground.NewRuntime(
		playground.WithStore(store),
		playground.WithBuiltinTools(createBuiltinTools()...),
	)
}

func TestCompositeTool_SingleStep(t *testing.T) {
	ctx := context.Background()
	rt := setupCompositeToolTestRuntime(t)
	ct := agent.NewCompositeTool(rt)

	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "single_step",
			Description: "Single step composite",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID:   "step1",
				Tool: agentcfg.ToolRef{Ref: "echo"},
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	// Invoke with args
	call := tool.NewFuncCall(`{"text": "hello world"}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	if result != "hello world" {
		t.Errorf("result = %v, want %q", result, "hello world")
	}
}

func TestCompositeTool_TwoSteps_ChainOutput(t *testing.T) {
	ctx := context.Background()
	rt := setupCompositeToolTestRuntime(t)
	ct := agent.NewCompositeTool(rt)

	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "chain_output",
			Description: "Chain two steps",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID:   "echo",
				Tool: agentcfg.ToolRef{Ref: "echo"},
			},
			{
				ID:   "upper",
				Tool: agentcfg.ToolRef{Ref: "uppercase"},
				// Use input_jq to wrap the previous step's output
				// .steps.echo contains the string "hello"
				InputJQ: mustParseJQ(`{"text": .steps.echo}`),
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := tool.NewFuncCall(`{"text": "hello"}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	if result != "HELLO" {
		t.Errorf("result = %v, want %q", result, "HELLO")
	}
}

func TestCompositeTool_InputJQ_FromSteps(t *testing.T) {
	ctx := context.Background()
	rt := setupCompositeToolTestRuntime(t)
	ct := agent.NewCompositeTool(rt)

	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "jq_from_steps",
			Description: "Use input_jq to access steps",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID:   "first",
				Tool: agentcfg.ToolRef{Ref: "json_output"},
			},
			{
				ID:   "second",
				Tool: agentcfg.ToolRef{Ref: "echo"},
				// Access the "double" field from first step's output
				InputJQ: mustParseJQ(`{"text": (.steps.first.double | tostring)}`),
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := tool.NewFuncCall(`{"name": "test", "value": 21}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	if result != "42" {
		t.Errorf("result = %v, want %q", result, "42")
	}
}

func TestCompositeTool_InputJQ_FromInput(t *testing.T) {
	ctx := context.Background()
	rt := setupCompositeToolTestRuntime(t)
	ct := agent.NewCompositeTool(rt)

	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "jq_from_input",
			Description: "Use input_jq to access original input",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID:   "step1",
				Tool: agentcfg.ToolRef{Ref: "echo"},
			},
			{
				ID:   "step2",
				Tool: agentcfg.ToolRef{Ref: "concat"},
				// Concatenate original input text with step1 output
				InputJQ: mustParseJQ(`{"a": .input.text, "b": (" -> " + .steps.step1)}`),
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := tool.NewFuncCall(`{"text": "hello"}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	if result != "hello -> hello" {
		t.Errorf("result = %v, want %q", result, "hello -> hello")
	}
}

func TestCompositeTool_ThreeSteps(t *testing.T) {
	ctx := context.Background()
	rt := setupCompositeToolTestRuntime(t)
	ct := agent.NewCompositeTool(rt)

	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "three_steps",
			Description: "Three step chain",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID:   "echo",
				Tool: agentcfg.ToolRef{Ref: "echo"},
			},
			{
				ID:      "upper",
				Tool:    agentcfg.ToolRef{Ref: "uppercase"},
				InputJQ: mustParseJQ(`{"text": .steps.echo}`),
			},
			{
				ID:      "concat",
				Tool:    agentcfg.ToolRef{Ref: "concat"},
				InputJQ: mustParseJQ(`{"a": .steps.echo, "b": (" -> " + .steps.upper)}`),
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := tool.NewFuncCall(`{"text": "hello"}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	if result != "hello -> HELLO" {
		t.Errorf("result = %v, want %q", result, "hello -> HELLO")
	}
}

func TestCompositeTool_Error_NoSteps(t *testing.T) {
	ctx := context.Background()
	rt := setupCompositeToolTestRuntime(t)
	ct := agent.NewCompositeTool(rt)

	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "no_steps",
			Description: "No steps",
		},
		Steps: []agentcfg.CompositeStep{},
	}

	_, err := ct.CreateFuncTool(ctx, def)
	if err == nil {
		t.Fatal("expected error for no steps")
	}
	if !strings.Contains(err.Error(), "at least one step") {
		t.Errorf("error = %v, want to contain %q", err, "at least one step")
	}
}

func TestCompositeTool_Error_MissingStepID(t *testing.T) {
	ctx := context.Background()
	rt := setupCompositeToolTestRuntime(t)
	ct := agent.NewCompositeTool(rt)

	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "missing_id",
			Description: "Missing step ID",
		},
		Steps: []agentcfg.CompositeStep{
			{
				// Missing ID
				Tool: agentcfg.ToolRef{Ref: "echo"},
			},
		},
	}

	_, err := ct.CreateFuncTool(ctx, def)
	if err == nil {
		t.Fatal("expected error for missing step ID")
	}
	if !strings.Contains(err.Error(), "missing id") {
		t.Errorf("error = %v, want to contain %q", err, "missing id")
	}
}

func TestCompositeTool_Error_MissingTool(t *testing.T) {
	ctx := context.Background()
	rt := setupCompositeToolTestRuntime(t)
	ct := agent.NewCompositeTool(rt)

	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "missing_tool",
			Description: "Missing tool ref",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID:   "step1",
				Tool: agentcfg.ToolRef{}, // No ref or inline
			},
		},
	}

	_, err := ct.CreateFuncTool(ctx, def)
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
	if !strings.Contains(err.Error(), "must have tool.$ref") {
		t.Errorf("error = %v, want to contain %q", err, "must have tool.$ref")
	}
}

// mustParseJQ parses a JQ expression or panics.
func mustParseJQ(expr string) *agentcfg.JQExpr {
	jq := &agentcfg.JQExpr{}
	// Marshal the expression to proper JSON string first
	jsonBytes, err := json.Marshal(expr)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(jsonBytes, jq); err != nil {
		panic(err)
	}
	return jq
}

func TestCompositeTool_JSONOutput(t *testing.T) {
	ctx := context.Background()
	rt := setupCompositeToolTestRuntime(t)
	ct := agent.NewCompositeTool(rt)

	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "json_chain",
			Description: "Chain with JSON output",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID:   "json",
				Tool: agentcfg.ToolRef{Ref: "json_output"},
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := tool.NewFuncCall(`{"name": "test", "value": 10}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	// Result should be a map
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}

	if m["name"] != "test" {
		t.Errorf("result.name = %v, want %q", m["name"], "test")
	}
	// Note: JSON numbers are float64
	if v, ok := m["value"].(int); ok && v != 10 {
		t.Errorf("result.value = %v, want %d", m["value"], 10)
	}
	if v, ok := m["double"].(int); ok && v != 20 {
		t.Errorf("result.double = %v, want %d", m["double"], 20)
	}
}

func TestCompositeTool_StepOutputStoredAsJSON(t *testing.T) {
	ctx := context.Background()
	rt := setupCompositeToolTestRuntime(t)
	ct := agent.NewCompositeTool(rt)

	// First step returns JSON, second step accesses a specific field
	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "json_field_access",
			Description: "Access JSON field from previous step",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID:   "json",
				Tool: agentcfg.ToolRef{Ref: "json_output"},
			},
			{
				ID:   "extract",
				Tool: agentcfg.ToolRef{Ref: "echo"},
				// Access the "name" field from JSON output
				InputJQ: mustParseJQ(`{"text": .steps.json.name}`),
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := tool.NewFuncCall(`{"name": "extracted_value", "value": 5}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	if result != "extracted_value" {
		t.Errorf("result = %v, want %q", result, "extracted_value")
	}
}

// Helper to marshal for debugging
func toJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

// ============================================================================
// Tests for HTTP Tool, Generator Tool, Text Processor Tool in Composite
// ============================================================================

// mockHTTPServer creates a test HTTP server that responds based on the request.
func mockHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/data":
			// GET request - return mock data
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"id":    123,
					"name":  "test_item",
					"value": 42,
				},
			})
		case "/submit":
			// POST request - echo back with processing
			body, _ := io.ReadAll(r.Body)
			var input map[string]any
			json.Unmarshal(body, &input)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"response": map[string]any{
					"received": input,
					"status":   "processed",
				},
			})
		case "/search":
			// Search endpoint
			query := r.URL.Query().Get("q")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{"title": "Result 1 for " + query, "score": 0.95},
					{"title": "Result 2 for " + query, "score": 0.87},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

// mockCompositeGenerator is a mock generator for composite tool tests.
type mockCompositeGenerator struct {
	responses map[string]string // model -> response
}

func (g *mockCompositeGenerator) GenerateStream(ctx context.Context, model string, mc genx.ModelContext) (genx.Stream, error) {
	response := g.responses[model]
	if response == "" {
		response = "Mock response for " + model
	}
	return &mockCompositeStream{response: response}, nil
}

func (g *mockCompositeGenerator) Invoke(ctx context.Context, model string, mc genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	return genx.Usage{}, nil, nil
}

type mockCompositeStream struct {
	response string
	done     bool
}

func (s *mockCompositeStream) Next() (*genx.MessageChunk, error) {
	if s.done {
		return nil, genx.Done(genx.Usage{})
	}
	s.done = true
	return &genx.MessageChunk{
		Role: genx.RoleModel,
		Part: genx.Text(s.response),
	}, nil
}

func (s *mockCompositeStream) Close() error               { return nil }
func (s *mockCompositeStream) CloseWithError(error) error { return nil }

// setupHTTPTestRuntime creates a runtime with testdata loaded and HTTP tool endpoints overridden.
// It adds a new readonly layer to override only the endpoint field (merge mode).
func setupHTTPTestRuntime(t *testing.T, serverURL string, opts ...playground.RuntimeOption) *playground.Runtime {
	t.Helper()
	store := playground.NewStore(nil)
	if err := store.LoadReadonlyLayer("testdata", os.DirFS("testdata/tool_composite_test")); err != nil {
		t.Fatalf("load testdata: %v", err)
	}

	// Add overlay layer - only override endpoint field, other fields merge from base layer
	store.AddReadonlyLayer("mock_server", map[string]map[string]any{
		"tool:v1/mock_http":   {"endpoint": serverURL + "/data"},
		"tool:v1/mock_post":   {"endpoint": serverURL + "/submit"},
		"tool:v1/search_http": {"endpoint": serverURL + "/search?q=test"},
	})

	allOpts := []playground.RuntimeOption{
		playground.WithStore(store),
		playground.WithBuiltinTools(createBuiltinTools()...),
	}
	allOpts = append(allOpts, opts...)
	return playground.NewRuntime(allOpts...)
}

// TestCompositeTool_HTTPTool_SingleStep tests composite with a single HTTP tool step.
func TestCompositeTool_HTTPTool_SingleStep(t *testing.T) {
	ctx := context.Background()
	server := mockHTTPServer(t)
	defer server.Close()

	rt := setupHTTPTestRuntime(t, server.URL)
	ct := agent.NewCompositeTool(rt)

	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "http_single",
			Description: "Single HTTP step",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID:   "fetch",
				Tool: agentcfg.ToolRef{Ref: "mock_http"},
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := tool.NewFuncCall(`{}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	// Result should be the extracted "result" field from response
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if m["name"] != "test_item" {
		t.Errorf("result.name = %v, want %q", m["name"], "test_item")
	}
}

// TestCompositeTool_HTTPTool_ChainWithBuiltin tests HTTP tool chained with builtin.
func TestCompositeTool_HTTPTool_ChainWithBuiltin(t *testing.T) {
	ctx := context.Background()
	server := mockHTTPServer(t)
	defer server.Close()

	rt := setupHTTPTestRuntime(t, server.URL)
	ct := agent.NewCompositeTool(rt)

	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "http_chain",
			Description: "HTTP then builtin",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID:   "fetch",
				Tool: agentcfg.ToolRef{Ref: "mock_http"},
			},
			{
				ID:   "format",
				Tool: agentcfg.ToolRef{Ref: "echo"},
				// Extract the name from HTTP response and echo it
				InputJQ: mustParseJQ(`{"text": .steps.fetch.name}`),
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := tool.NewFuncCall(`{}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	if result != "test_item" {
		t.Errorf("result = %v, want %q", result, "test_item")
	}
}

// TestCompositeTool_HTTPTool_POST tests HTTP POST with request body transformation.
func TestCompositeTool_HTTPTool_POST(t *testing.T) {
	ctx := context.Background()
	server := mockHTTPServer(t)
	defer server.Close()

	rt := setupHTTPTestRuntime(t, server.URL)
	ct := agent.NewCompositeTool(rt)

	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "http_post",
			Description: "POST request",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID:   "submit",
				Tool: agentcfg.ToolRef{Ref: "mock_post"},
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := tool.NewFuncCall(`{"input": "test_data"}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if m["status"] != "processed" {
		t.Errorf("result.status = %v, want %q", m["status"], "processed")
	}
}

// TestCompositeTool_GeneratorTool tests composite with generator tool.
func TestCompositeTool_GeneratorTool(t *testing.T) {
	ctx := context.Background()

	store := playground.NewStore(nil)
	if err := store.LoadReadonlyLayer("testdata", os.DirFS("testdata/tool_composite_test")); err != nil {
		t.Fatalf("load testdata: %v", err)
	}

	mockGen := &mockCompositeGenerator{
		responses: map[string]string{
			"test-model": "This is a summarized version of the input.",
		},
	}

	rt := playground.NewRuntime(
		playground.WithStore(store),
		playground.WithGenerator(mockGen),
		playground.WithBuiltinTools(createBuiltinTools()...),
	)
	ct := agent.NewCompositeTool(rt)

	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "generator_step",
			Description: "Use generator tool",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID:   "summarize",
				Tool: agentcfg.ToolRef{Ref: "summarizer"},
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := tool.NewFuncCall(`{"input": "Long text that needs summarization..."}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	if !strings.Contains(resultStr, "summarized") {
		t.Errorf("result = %q, want to contain %q", resultStr, "summarized")
	}
}

// TestCompositeTool_TextProcessorTool tests composite with text processor tool.
func TestCompositeTool_TextProcessorTool(t *testing.T) {
	ctx := context.Background()

	store := playground.NewStore(nil)
	if err := store.LoadReadonlyLayer("testdata", os.DirFS("testdata/tool_composite_test")); err != nil {
		t.Fatalf("load testdata: %v", err)
	}

	mockGen := &mockCompositeGenerator{
		responses: map[string]string{
			"test-model": "Brief summary of the content.",
		},
	}

	rt := playground.NewRuntime(
		playground.WithStore(store),
		playground.WithGenerator(mockGen),
		playground.WithBuiltinTools(createBuiltinTools()...),
	)
	ct := agent.NewCompositeTool(rt)

	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "text_processor_step",
			Description: "Use text processor tool",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID:   "process",
				Tool: agentcfg.ToolRef{Ref: "text_summarizer"},
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := tool.NewFuncCall(`{"content": "This is the content to process."}`)
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

// TestCompositeTool_MixedToolTypes tests composite with multiple tool types.
func TestCompositeTool_MixedToolTypes(t *testing.T) {
	ctx := context.Background()
	server := mockHTTPServer(t)
	defer server.Close()

	mockGen := &mockCompositeGenerator{
		responses: map[string]string{
			"test-model": "Processed: test_item with value 42",
		},
	}

	rt := setupHTTPTestRuntime(t, server.URL, playground.WithGenerator(mockGen))
	ct := agent.NewCompositeTool(rt)

	// Chain: HTTP -> builtin uppercase -> builtin concat
	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "mixed_chain",
			Description: "Mix HTTP with builtin tools",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID:   "fetch",
				Tool: agentcfg.ToolRef{Ref: "mock_http"},
			},
			{
				ID:      "upper",
				Tool:    agentcfg.ToolRef{Ref: "uppercase"},
				InputJQ: mustParseJQ(`{"text": .steps.fetch.name}`),
			},
			{
				ID:      "result",
				Tool:    agentcfg.ToolRef{Ref: "concat"},
				InputJQ: mustParseJQ(`{"a": "Item: ", "b": .steps.upper}`),
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := tool.NewFuncCall(`{}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	if result != "Item: TEST_ITEM" {
		t.Errorf("result = %v, want %q", result, "Item: TEST_ITEM")
	}
}

// TestCompositeTool_ToolDefFromStore tests loading composite tool def from store.
func TestCompositeTool_ToolDefFromStore(t *testing.T) {
	ctx := context.Background()
	server := mockHTTPServer(t)
	defer server.Close()

	rt := setupHTTPTestRuntime(t, server.URL)

	// Get tool def from store
	toolDef, err := rt.GetToolDef(ctx, "search_http")
	if err != nil {
		t.Fatalf("GetToolDef error: %v", err)
	}

	httpDef := agentcfg.AsHTTPTool(toolDef)
	if httpDef == nil {
		t.Fatalf("expected HTTPToolDef, got %T", toolDef)
	}

	if httpDef.Method != "GET" {
		t.Errorf("Method = %q, want %q", httpDef.Method, "GET")
	}
}

// ============================================================================
// Tests for createInlineTool (inline tool definitions in composite steps)
// ============================================================================

// TestCompositeTool_InlineHTTPTool tests composite with inline HTTP tool definition.
func TestCompositeTool_InlineHTTPTool(t *testing.T) {
	ctx := context.Background()
	server := mockHTTPServer(t)
	defer server.Close()

	rt := setupCompositeToolTestRuntime(t)
	ct := agent.NewCompositeTool(rt)

	// Use inline HTTP tool definition instead of $ref
	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "inline_http",
			Description: "Composite with inline HTTP tool",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID: "fetch",
				Tool: agentcfg.ToolRef{
					Tool: &agentcfg.HTTPTool{
						ToolBase: agentcfg.ToolBase{
							Name:        "inline_get",
							Description: "Inline GET request",
							Type:        agentcfg.ToolTypeHTTP,
						},
						Method:   "GET",
						Endpoint: server.URL + "/data",
					},
				},
			},
			{
				ID:      "format",
				Tool:    agentcfg.ToolRef{Ref: "echo"},
				InputJQ: mustParseJQ(`{"text": .steps.fetch.result.name}`),
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := tool.NewFuncCall(`{}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	if result != "test_item" {
		t.Errorf("result = %v, want %q", result, "test_item")
	}
}

// TestCompositeTool_InlineGeneratorTool tests composite with inline generator tool definition.
func TestCompositeTool_InlineGeneratorTool(t *testing.T) {
	ctx := context.Background()

	mockGen := &mockCompositeGenerator{
		responses: map[string]string{
			"inline-model": "Inline generator output",
		},
	}

	store := playground.NewStore(nil)
	rt := playground.NewRuntime(
		playground.WithStore(store),
		playground.WithGenerator(mockGen),
		playground.WithBuiltinTools(createBuiltinTools()...),
	)
	ct := agent.NewCompositeTool(rt)

	// Use inline generator tool definition
	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "inline_generator",
			Description: "Composite with inline generator tool",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID: "generate",
				Tool: agentcfg.ToolRef{
					Tool: &agentcfg.GeneratorTool{
						ToolBase: agentcfg.ToolBase{
							Name:        "inline_gen",
							Description: "Inline generator",
							Type:        agentcfg.ToolTypeGenerator,
						},
						Model:  "inline-model",
						Mode:   "generate",
						Prompt: "Generate something useful",
					},
				},
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := tool.NewFuncCall(`{"input": "test input"}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	if resultStr != "Inline generator output" {
		t.Errorf("result = %q, want %q", resultStr, "Inline generator output")
	}
}

// TestCompositeTool_ChainInlineWithRef tests chaining inline and ref-based tools.
func TestCompositeTool_ChainInlineWithRef(t *testing.T) {
	ctx := context.Background()
	server := mockHTTPServer(t)
	defer server.Close()

	rt := setupHTTPTestRuntime(t, server.URL)
	ct := agent.NewCompositeTool(rt)

	// First step uses ref, second step uses inline
	def := &agentcfg.CompositeTool{
		ToolBase: agentcfg.ToolBase{
			Name:        "chain_inline_ref",
			Description: "Chain inline with ref tools",
		},
		Steps: []agentcfg.CompositeStep{
			{
				ID:   "fetch",
				Tool: agentcfg.ToolRef{Ref: "mock_http"}, // Reference to tool from store
			},
			{
				ID: "process",
				Tool: agentcfg.ToolRef{
					Tool: &agentcfg.HTTPTool{
						ToolBase: agentcfg.ToolBase{
							Name: "inline_post",
							Type: agentcfg.ToolTypeHTTP,
						},
						Method:   "POST",
						Endpoint: server.URL + "/submit",
					},
				},
				// Pass the fetched data to the POST request
				InputJQ: mustParseJQ(`.steps.fetch`),
			},
		},
	}

	tool, err := ct.CreateFuncTool(ctx, def)
	if err != nil {
		t.Fatalf("CreateFuncTool error: %v", err)
	}

	call := tool.NewFuncCall(`{}`)
	result, err := call.Invoke(ctx)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	// The /submit endpoint returns {"response": {"received": ..., "status": "processed"}}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	// Result is nested: response.status
	response, ok := m["response"].(map[string]any)
	if !ok {
		t.Fatalf("result.response type = %T, want map[string]any", m["response"])
	}
	if response["status"] != "processed" {
		t.Errorf("result.response.status = %v, want %q", response["status"], "processed")
	}
}
