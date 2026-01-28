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

// mockReActGenerator is a mock generator for ReAct agent tests.
// It can return text responses or tool calls based on configuration.
type mockReActGenerator struct {
	// responses maps model -> sequence of responses
	// Each response can be text or a tool call
	responses map[string][]mockResponse
	// callCount tracks how many times GenerateStream has been called per model
	callCount map[string]int
}

type mockResponse struct {
	text     string         // Text response
	toolCall *genx.ToolCall // Tool call (if not nil, this is a tool call response)
}

func newMockReActGenerator() *mockReActGenerator {
	return &mockReActGenerator{
		responses: make(map[string][]mockResponse),
		callCount: make(map[string]int),
	}
}

// WithTextResponse adds a text response for a model.
func (g *mockReActGenerator) WithTextResponse(model, text string) *mockReActGenerator {
	g.responses[model] = append(g.responses[model], mockResponse{text: text})
	return g
}

// WithToolCall adds a tool call response for a model.
func (g *mockReActGenerator) WithToolCall(model, toolID, toolName, args string) *mockReActGenerator {
	g.responses[model] = append(g.responses[model], mockResponse{
		toolCall: &genx.ToolCall{
			ID: toolID,
			FuncCall: &genx.FuncCall{
				Name:      toolName,
				Arguments: args,
			},
		},
	})
	return g
}

// WithTextAndToolCall adds a response with both text and a tool call.
// The text will be returned first, then the tool call.
func (g *mockReActGenerator) WithTextAndToolCall(model, text, toolID, toolName, args string) *mockReActGenerator {
	g.responses[model] = append(g.responses[model], mockResponse{
		text: text,
		toolCall: &genx.ToolCall{
			ID: toolID,
			FuncCall: &genx.FuncCall{
				Name:      toolName,
				Arguments: args,
			},
		},
	})
	return g
}

func (g *mockReActGenerator) GenerateStream(ctx context.Context, model string, mc genx.ModelContext) (genx.Stream, error) {
	// Get current call count and increment
	count := g.callCount[model]
	g.callCount[model]++

	// Get response for this call
	responses := g.responses[model]
	if count >= len(responses) {
		// No more responses, return empty
		return &mockReActStream{}, nil
	}

	resp := responses[count]
	return &mockReActStream{
		text:     resp.text,
		toolCall: resp.toolCall,
	}, nil
}

func (g *mockReActGenerator) Invoke(ctx context.Context, model string, mc genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	return genx.Usage{}, nil, nil
}

type mockReActStream struct {
	text     string
	toolCall *genx.ToolCall
	phase    int // 0: not started, 1: text sent, 2: tool call sent, 3: done
}

func (s *mockReActStream) Next() (*genx.MessageChunk, error) {
	switch s.phase {
	case 0:
		s.phase++
		if s.text != "" {
			return &genx.MessageChunk{
				Role: genx.RoleModel,
				Part: genx.Text(s.text),
			}, nil
		}
		fallthrough
	case 1:
		s.phase++
		if s.toolCall != nil {
			return &genx.MessageChunk{
				Role:     genx.RoleModel,
				ToolCall: s.toolCall,
			}, nil
		}
		fallthrough
	default:
		return nil, genx.Done(genx.Usage{})
	}
}

func (s *mockReActStream) Close() error               { return nil }
func (s *mockReActStream) CloseWithError(error) error { return nil }

// createReActBuiltinTools creates mock builtin tools for testing.
func createReActBuiltinTools() []*genx.FuncTool {
	var tools []*genx.FuncTool

	// Calculator tool
	type calcArgs struct {
		Expression string `json:"expression"`
	}
	calcTool, _ := genx.NewFuncTool[calcArgs](
		"calculator",
		"Perform basic math calculations",
		genx.InvokeFunc[calcArgs](func(ctx context.Context, call *genx.FuncCall, args calcArgs) (any, error) {
			// Simple mock: just return a fixed result
			return "42", nil
		}),
	)
	tools = append(tools, calcTool)

	// Search tool
	type searchArgs struct {
		Query string `json:"query"`
	}
	searchTool, _ := genx.NewFuncTool[searchArgs](
		"search",
		"Search for information",
		genx.InvokeFunc[searchArgs](func(ctx context.Context, call *genx.FuncCall, args searchArgs) (any, error) {
			return "Search result for: " + args.Query, nil
		}),
	)
	tools = append(tools, searchTool)

	// Finish tool (quit tool)
	type finishArgs struct {
		Answer string `json:"answer"`
	}
	finishTool, _ := genx.NewFuncTool[finishArgs](
		"finish",
		"Finish the task",
		genx.InvokeFunc[finishArgs](func(ctx context.Context, call *genx.FuncCall, args finishArgs) (any, error) {
			return args.Answer, nil
		}),
	)
	tools = append(tools, finishTool)

	return tools
}

func setupReActAgentTestRuntime(t *testing.T, mockGen *mockReActGenerator) *playground.Runtime {
	t.Helper()
	store := playground.NewStore(nil)
	if err := store.LoadReadonlyLayer("testdata", os.DirFS("testdata/agent_react_test")); err != nil {
		t.Fatalf("load testdata: %v", err)
	}

	return playground.NewRuntime(
		playground.WithStore(store),
		playground.WithGenerator(mockGen),
		playground.WithBuiltinTools(createReActBuiltinTools()...),
	)
}

func TestReActAgent_LoadFromStore(t *testing.T) {
	ctx := context.Background()
	mockGen := newMockReActGenerator().
		WithTextResponse("test-model", "Hello!")

	rt := setupReActAgentTestRuntime(t, mockGen)

	// Load agent definition from store
	agentDef, err := rt.GetAgentDef(ctx, "assistant")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	reactDef := agentcfg.AsReActAgent(agentDef)
	if reactDef == nil {
		t.Fatalf("expected ReActAgentDef, got %T", agentDef)
	}

	if reactDef.Name != "assistant" {
		t.Errorf("Name = %q, want %q", reactDef.Name, "assistant")
	}

	if reactDef.Prompt != "You are a helpful assistant." {
		t.Errorf("Prompt = %q, want %q", reactDef.Prompt, "You are a helpful assistant.")
	}

	// Verify tools
	if len(reactDef.Tools) != 3 {
		t.Errorf("len(Tools) = %d, want 3", len(reactDef.Tools))
	}
}

func TestReActAgent_Create(t *testing.T) {
	ctx := context.Background()
	mockGen := newMockReActGenerator().
		WithTextResponse("test-model", "Hello!")

	rt := setupReActAgentTestRuntime(t, mockGen)

	agentDef, err := rt.GetAgentDef(ctx, "assistant")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	reactDef := agentcfg.AsReActAgent(agentDef)
	reactAgent, err := agent.NewReActAgent(ctx, reactDef, rt, "")
	if err != nil {
		t.Fatalf("NewReActAgent error: %v", err)
	}
	defer reactAgent.Close()

	// Verify basic properties
	if reactAgent.Def() == nil {
		t.Error("Def() returned nil")
	}

	if reactAgent.State() == nil {
		t.Error("State() returned nil")
	}

	if reactAgent.StateID() == "" {
		t.Error("StateID() returned empty string")
	}
}

func TestReActAgent_SimpleTextResponse(t *testing.T) {
	ctx := context.Background()
	mockGen := newMockReActGenerator().
		WithTextResponse("test-model", "Hello! I'm here to help.")

	rt := setupReActAgentTestRuntime(t, mockGen)

	agentDef, err := rt.GetAgentDef(ctx, "simple_agent")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	reactDef := agentcfg.AsReActAgent(agentDef)
	reactAgent, err := agent.NewReActAgent(ctx, reactDef, rt, "")
	if err != nil {
		t.Fatalf("NewReActAgent error: %v", err)
	}
	defer reactAgent.Close()

	// Send input
	if err := reactAgent.Input(genx.Contents{genx.Text("Hi there")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Collect events
	var chunks []string
	var events []*agent.AgentEvent
	for {
		evt, err := reactAgent.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		events = append(events, evt)

		if evt.Type == agent.EventChunk && evt.Chunk != nil {
			if text, ok := evt.Chunk.Part.(genx.Text); ok {
				chunks = append(chunks, string(text))
			}
		}

		if evt.Type == agent.EventEOF || evt.Type == agent.EventClosed {
			break
		}
	}

	// Verify we got the text response
	fullText := strings.Join(chunks, "")
	if fullText != "Hello! I'm here to help." {
		t.Errorf("response = %q, want %q", fullText, "Hello! I'm here to help.")
	}

	// Last event should be EOF
	if events[len(events)-1].Type != agent.EventEOF {
		t.Errorf("last event = %v, want EventEOF", events[len(events)-1].Type)
	}
}

func TestReActAgent_ToolCall(t *testing.T) {
	ctx := context.Background()
	mockGen := newMockReActGenerator().
		// First call: tool call
		WithToolCall("test-model", "call-1", "calculator", `{"expression":"2+2"}`).
		// Second call (after tool result): text response
		WithTextResponse("test-model", "The answer is 42.")

	rt := setupReActAgentTestRuntime(t, mockGen)

	agentDef, err := rt.GetAgentDef(ctx, "assistant")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	reactDef := agentcfg.AsReActAgent(agentDef)
	reactAgent, err := agent.NewReActAgent(ctx, reactDef, rt, "")
	if err != nil {
		t.Fatalf("NewReActAgent error: %v", err)
	}
	defer reactAgent.Close()

	// Send input
	if err := reactAgent.Input(genx.Contents{genx.Text("What is 2+2?")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Collect events
	var events []*agent.AgentEvent
	var sawToolStart, sawToolDone bool
	for {
		evt, err := reactAgent.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		events = append(events, evt)

		switch evt.Type {
		case agent.EventToolStart:
			sawToolStart = true
			if evt.ToolCall == nil {
				t.Error("EventToolStart without ToolCall")
			} else if evt.ToolCall.FuncCall.Name != "calculator" {
				t.Errorf("ToolCall.Name = %q, want %q", evt.ToolCall.FuncCall.Name, "calculator")
			}
		case agent.EventToolDone:
			sawToolDone = true
		}

		if evt.Type == agent.EventEOF || evt.Type == agent.EventClosed {
			break
		}
	}

	if !sawToolStart {
		t.Error("did not see EventToolStart")
	}
	if !sawToolDone {
		t.Error("did not see EventToolDone")
	}

	t.Logf("Received %d events", len(events))
	for i, evt := range events {
		t.Logf("Event %d: Type=%v", i, evt.Type)
	}
}

func TestReActAgent_QuitTool(t *testing.T) {
	ctx := context.Background()
	mockGen := newMockReActGenerator().
		// Call finish tool (quit=true)
		WithToolCall("test-model", "call-1", "finish", `{"answer":"Done!"}`).
		// After quit tool, agent should be finished
		WithTextResponse("test-model", "")

	rt := setupReActAgentTestRuntime(t, mockGen)

	agentDef, err := rt.GetAgentDef(ctx, "assistant")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	reactDef := agentcfg.AsReActAgent(agentDef)
	reactAgent, err := agent.NewReActAgent(ctx, reactDef, rt, "")
	if err != nil {
		t.Fatalf("NewReActAgent error: %v", err)
	}
	defer reactAgent.Close()

	// Send input
	if err := reactAgent.Input(genx.Contents{genx.Text("Finish the task")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Collect events until closed
	var events []*agent.AgentEvent
	for {
		evt, err := reactAgent.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		events = append(events, evt)

		if evt.Type == agent.EventClosed {
			break
		}
		// Safety limit
		if len(events) > 20 {
			t.Fatal("too many events, expected EventClosed")
		}
	}

	// Verify agent is finished
	if !reactAgent.IsFinished() {
		t.Error("agent should be finished after quit tool")
	}

	// Last event should be EventClosed
	if events[len(events)-1].Type != agent.EventClosed {
		t.Errorf("last event = %v, want EventClosed", events[len(events)-1].Type)
	}
}

func TestReActAgent_MultipleToolCalls(t *testing.T) {
	ctx := context.Background()
	mockGen := newMockReActGenerator().
		// First: search
		WithToolCall("test-model", "call-1", "search", `{"query":"weather"}`).
		// Second: calculator
		WithToolCall("test-model", "call-2", "calculator", `{"expression":"100+50"}`).
		// Third: final response
		WithTextResponse("test-model", "Based on search and calculation, here's my answer.")

	rt := setupReActAgentTestRuntime(t, mockGen)

	agentDef, err := rt.GetAgentDef(ctx, "assistant")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	reactDef := agentcfg.AsReActAgent(agentDef)
	reactAgent, err := agent.NewReActAgent(ctx, reactDef, rt, "")
	if err != nil {
		t.Fatalf("NewReActAgent error: %v", err)
	}
	defer reactAgent.Close()

	// Send input
	if err := reactAgent.Input(genx.Contents{genx.Text("Do research")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Count tool calls
	toolStartCount := 0
	toolDoneCount := 0
	for {
		evt, err := reactAgent.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}

		switch evt.Type {
		case agent.EventToolStart:
			toolStartCount++
		case agent.EventToolDone:
			toolDoneCount++
		}

		if evt.Type == agent.EventEOF || evt.Type == agent.EventClosed {
			break
		}
	}

	if toolStartCount != 2 {
		t.Errorf("toolStartCount = %d, want 2", toolStartCount)
	}
	if toolDoneCount != 2 {
		t.Errorf("toolDoneCount = %d, want 2", toolDoneCount)
	}
}

func TestReActAgent_State(t *testing.T) {
	ctx := context.Background()
	mockGen := newMockReActGenerator().
		WithTextResponse("test-model", "Hello!")

	rt := setupReActAgentTestRuntime(t, mockGen)

	agentDef, err := rt.GetAgentDef(ctx, "simple_agent")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	reactDef := agentcfg.AsReActAgent(agentDef)
	reactAgent, err := agent.NewReActAgent(ctx, reactDef, rt, "")
	if err != nil {
		t.Fatalf("NewReActAgent error: %v", err)
	}
	defer reactAgent.Close()

	state := reactAgent.State()
	if state == nil {
		t.Fatal("State() returned nil")
	}

	// State should have ID
	if state.ID() == "" {
		t.Error("state.ID() is empty")
	}

	// State should have AgentDef
	if state.AgentDef() != "simple_agent" {
		t.Errorf("state.AgentDef() = %q, want %q", state.AgentDef(), "simple_agent")
	}
}

func TestReActAgent_FormatHistory(t *testing.T) {
	ctx := context.Background()
	mockGen := newMockReActGenerator().
		WithTextResponse("test-model", "Hello! How can I help?")

	rt := setupReActAgentTestRuntime(t, mockGen)

	agentDef, err := rt.GetAgentDef(ctx, "simple_agent")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	reactDef := agentcfg.AsReActAgent(agentDef)
	reactAgent, err := agent.NewReActAgent(ctx, reactDef, rt, "")
	if err != nil {
		t.Fatalf("NewReActAgent error: %v", err)
	}
	defer reactAgent.Close()

	// Send input and get response
	if err := reactAgent.Input(genx.Contents{genx.Text("Hi")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Drain events
	for {
		evt, err := reactAgent.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if evt.Type == agent.EventEOF || evt.Type == agent.EventClosed {
			break
		}
	}

	// Format history
	history := reactAgent.FormatHistory(ctx)

	// Should contain user message
	if !strings.Contains(history, "[user]:") {
		t.Error("history should contain user message")
	}

	// Should contain model response
	if !strings.Contains(history, "[model]:") {
		t.Error("history should contain model response")
	}

	t.Logf("History:\n%s", history)
}

func TestReActAgent_Close(t *testing.T) {
	ctx := context.Background()
	mockGen := newMockReActGenerator().
		WithTextResponse("test-model", "Hello!")

	rt := setupReActAgentTestRuntime(t, mockGen)

	agentDef, err := rt.GetAgentDef(ctx, "simple_agent")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	reactDef := agentcfg.AsReActAgent(agentDef)
	reactAgent, err := agent.NewReActAgent(ctx, reactDef, rt, "")
	if err != nil {
		t.Fatalf("NewReActAgent error: %v", err)
	}

	// Close should not error
	if err := reactAgent.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}

	// Close again should not error (idempotent)
	if err := reactAgent.Close(); err != nil {
		t.Errorf("Close again error: %v", err)
	}
}

func TestReActAgent_Interrupt(t *testing.T) {
	ctx := context.Background()
	mockGen := newMockReActGenerator().
		WithTextResponse("test-model", "Hello!")

	rt := setupReActAgentTestRuntime(t, mockGen)

	agentDef, err := rt.GetAgentDef(ctx, "simple_agent")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	reactDef := agentcfg.AsReActAgent(agentDef)
	reactAgent, err := agent.NewReActAgent(ctx, reactDef, rt, "")
	if err != nil {
		t.Fatalf("NewReActAgent error: %v", err)
	}
	defer reactAgent.Close()

	// Interrupt before any input
	if err := reactAgent.Interrupt(); err != nil {
		t.Errorf("Interrupt error: %v", err)
	}

	// Next should return interrupted
	evt, err := reactAgent.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}

	if evt.Type != agent.EventInterrupted {
		t.Errorf("event type = %v, want EventInterrupted", evt.Type)
	}
}

// TestAgentEvent_IsTerminal tests the IsTerminal method.
func TestAgentEvent_IsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		event    *agent.AgentEvent
		expected bool
	}{
		{"EventChunk", &agent.AgentEvent{Type: agent.EventChunk}, false},
		{"EventEOF", &agent.AgentEvent{Type: agent.EventEOF}, false},
		{"EventClosed", &agent.AgentEvent{Type: agent.EventClosed}, true},
		{"EventToolStart", &agent.AgentEvent{Type: agent.EventToolStart}, false},
		{"EventToolDone", &agent.AgentEvent{Type: agent.EventToolDone}, false},
		{"EventToolError", &agent.AgentEvent{Type: agent.EventToolError}, false},
		{"EventInterrupted", &agent.AgentEvent{Type: agent.EventInterrupted}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.IsTerminal(); got != tt.expected {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestReActAgent_ContextLayers_This tests $this context layer resolution.
func TestReActAgent_ContextLayers_This(t *testing.T) {
	ctx := context.Background()

	mockGen := newMockReActGenerator().
		WithTextResponse("test-model", "Hello!")

	store := playground.NewStore(nil)
	if err := store.LoadReadonlyLayer("testdata", os.DirFS("testdata/agent_react_test")); err != nil {
		t.Fatalf("load testdata: %v", err)
	}

	rt := playground.NewRuntime(
		playground.WithStore(store),
		playground.WithGenerator(mockGen),
		playground.WithBuiltinTools(createReActBuiltinTools()...),
	)

	// Create agent with $this context layer
	agentDef := &agentcfg.ReActAgent{
		AgentBase: agentcfg.AgentBase{
			Name:   "test_this",
			Type:   agentcfg.AgentTypeReAct,
			Prompt: "You are a helpful assistant.",
			Generator: agentcfg.GeneratorRef{
				Generator: &agentcfg.Generator{Model: "test-model"},
			},
			ContextLayers: []agentcfg.ContextLayer{
				{This: ".prompt"}, // Reference to agent's prompt field
			},
		},
		Tools: []agentcfg.ToolRef{
			{Ref: "finish", Quit: true},
		},
	}

	ag, err := agent.NewReActAgent(ctx, agentDef, rt, "")
	if err != nil {
		t.Fatalf("NewReActAgent error: %v", err)
	}
	defer ag.Close()

	if err := ag.Input(genx.Contents{genx.Text("Hi")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Consume events
	for {
		evt, err := ag.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if evt.Type == agent.EventEOF || evt.Type == agent.EventClosed {
			break
		}
	}
	// Test passes if no errors - $this context layer was resolved
}

// TestReActAgent_ContextLayers_Env tests $env context layer resolution.
func TestReActAgent_ContextLayers_Env(t *testing.T) {
	ctx := context.Background()

	// Set up test environment variable
	testEnvValue := "TEST_SYSTEM_PROMPT_VALUE_12345"
	os.Setenv("TEST_AGENT_PROMPT", testEnvValue)
	defer os.Unsetenv("TEST_AGENT_PROMPT")

	mockGen := newMockReActGenerator().
		WithTextResponse("test-model", "Hello!")

	store := playground.NewStore(nil)
	rt := playground.NewRuntime(
		playground.WithStore(store),
		playground.WithGenerator(mockGen),
		playground.WithBuiltinTools(createReActBuiltinTools()...),
	)

	// Create agent with $env context layer
	agentDef := &agentcfg.ReActAgent{
		AgentBase: agentcfg.AgentBase{
			Name: "test_env",
			Type: agentcfg.AgentTypeReAct,
			Generator: agentcfg.GeneratorRef{
				Generator: &agentcfg.Generator{Model: "test-model"},
			},
			ContextLayers: []agentcfg.ContextLayer{
				{Env: "TEST_AGENT_PROMPT"},
			},
		},
		Tools: []agentcfg.ToolRef{
			{Ref: "finish", Quit: true},
		},
	}

	ag, err := agent.NewReActAgent(ctx, agentDef, rt, "")
	if err != nil {
		t.Fatalf("NewReActAgent error: %v", err)
	}
	defer ag.Close()

	if err := ag.Input(genx.Contents{genx.Text("Hi")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Consume events - if no error, $env was resolved successfully
	for {
		evt, err := ag.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if evt.Type == agent.EventEOF || evt.Type == agent.EventClosed {
			break
		}
	}
}

// TestReActAgent_ParentStateID tests that top-level agents have empty ParentStateID.
func TestReActAgent_ParentStateID(t *testing.T) {
	ctx := context.Background()

	mockGen := newMockReActGenerator().
		WithTextResponse("test-model", "Hello!")

	rt := setupReActAgentTestRuntime(t, mockGen)

	agentDef, err := rt.GetAgentDef(ctx, "simple_agent")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	reactDef := agentcfg.AsReActAgent(agentDef)
	reactAgent, err := agent.NewReActAgent(ctx, reactDef, rt, "")
	if err != nil {
		t.Fatalf("NewReActAgent error: %v", err)
	}
	defer reactAgent.Close()

	// Top-level agent's state should have empty ParentStateID
	state := reactAgent.State()
	if state.ParentStateID() != "" {
		t.Errorf("top-level agent's ParentStateID should be empty, got %q", state.ParentStateID())
	}

	// Verify state ID is set
	if reactAgent.StateID() == "" {
		t.Error("agent should have a state ID")
	}
}
