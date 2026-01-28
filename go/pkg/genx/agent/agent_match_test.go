package agent_test

import (
	"context"
	"os"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/agent"
	"github.com/haivivi/giztoy/go/pkg/genx/agentcfg"
	"github.com/haivivi/giztoy/go/pkg/genx/playground"
)

// mockMatchGenerator is a mock generator for match agent tests.
type mockMatchGenerator struct {
	responses    map[string]string // model -> response text
	matchResults map[string]string // model -> rule name for matching
}

func (g *mockMatchGenerator) GenerateStream(ctx context.Context, model string, mc genx.ModelContext) (genx.Stream, error) {
	// Check if this is a match request
	if g.matchResults != nil {
		if rule, ok := g.matchResults[model]; ok {
			return &mockMatchStream{response: rule + "\n"}, nil
		}
	}

	response := g.responses[model]
	if response == "" {
		response = "Mock response"
	}
	return &mockMatchStream{response: response}, nil
}

func (g *mockMatchGenerator) Invoke(ctx context.Context, model string, mc genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	return genx.Usage{}, nil, nil
}

type mockMatchStream struct {
	response string
	done     bool
}

func (s *mockMatchStream) Next() (*genx.MessageChunk, error) {
	if s.done {
		return nil, genx.Done(genx.Usage{})
	}
	s.done = true
	return &genx.MessageChunk{
		Role: genx.RoleModel,
		Part: genx.Text(s.response),
	}, nil
}

func (s *mockMatchStream) Close() error               { return nil }
func (s *mockMatchStream) CloseWithError(error) error { return nil }

func setupMatchAgentTestRuntime(t *testing.T) *playground.Runtime {
	return setupMatchAgentTestRuntimeWithMatchResult(t, "")
}

func setupMatchAgentTestRuntimeWithMatchResult(t *testing.T, matchResult string) *playground.Runtime {
	t.Helper()
	store := playground.NewStore(nil)
	if err := store.LoadReadonlyLayer("testdata", os.DirFS("testdata/agent_match_test")); err != nil {
		t.Fatalf("load testdata: %v", err)
	}

	mockGen := &mockMatchGenerator{
		responses: map[string]string{
			"test-model": "Hello! How can I help you today?",
		},
	}

	if matchResult != "" {
		mockGen.matchResults = map[string]string{
			"test-model": matchResult,
		}
	}

	return playground.NewRuntime(
		playground.WithStore(store),
		playground.WithGenerator(mockGen),
	)
}

func TestMatchAgent_LoadFromStore(t *testing.T) {
	ctx := context.Background()
	rt := setupMatchAgentTestRuntime(t)

	// Load match agent definition from store
	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	if matchDef == nil {
		t.Fatalf("expected MatchAgentDef, got %T", agentDef)
	}

	if matchDef.Name != "intent_router" {
		t.Errorf("Name = %q, want %q", matchDef.Name, "intent_router")
	}

	// Verify rules
	if len(matchDef.Rules) != 3 {
		t.Errorf("len(Rules) = %d, want 3", len(matchDef.Rules))
	}

	// First two rules are references
	if matchDef.Rules[0].Ref != "play_music" {
		t.Errorf("Rules[0].Ref = %q, want %q", matchDef.Rules[0].Ref, "play_music")
	}
	if matchDef.Rules[1].Ref != "weather_query" {
		t.Errorf("Rules[1].Ref = %q, want %q", matchDef.Rules[1].Ref, "weather_query")
	}

	// Third rule is inline
	if matchDef.Rules[2].Rule == nil {
		t.Fatal("Rules[2].Rule is nil")
	}
	if matchDef.Rules[2].Rule.Name != "greeting" {
		t.Errorf("Rules[2].Rule.Name = %q, want %q", matchDef.Rules[2].Rule.Name, "greeting")
	}

	// Verify routes
	if len(matchDef.Route) != 3 {
		t.Errorf("len(Route) = %d, want 3", len(matchDef.Route))
	}
}

func TestMatchAgent_LoadRulesFromStore(t *testing.T) {
	ctx := context.Background()
	rt := setupMatchAgentTestRuntime(t)

	// Load play_music rule
	rule, err := rt.GetRule(ctx, "play_music")
	if err != nil {
		t.Fatalf("GetRule error: %v", err)
	}

	if rule.Name != "play_music" {
		t.Errorf("Name = %q, want %q", rule.Name, "play_music")
	}
	if len(rule.Examples) == 0 {
		t.Error("Examples is empty")
	}

	// Load weather_query rule
	rule2, err := rt.GetRule(ctx, "weather_query")
	if err != nil {
		t.Fatalf("GetRule error: %v", err)
	}

	if rule2.Name != "weather_query" {
		t.Errorf("Name = %q, want %q", rule2.Name, "weather_query")
	}
}

func TestMatchAgent_LoadSubAgentsFromStore(t *testing.T) {
	ctx := context.Background()
	rt := setupMatchAgentTestRuntime(t)

	// Load music_agent
	agentDef, err := rt.GetAgentDef(ctx, "music_agent")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	reactDef := agentcfg.AsReActAgent(agentDef)
	if reactDef == nil {
		t.Fatalf("expected ReActAgentDef, got %T", agentDef)
	}

	if reactDef.Name != "music_agent" {
		t.Errorf("Name = %q, want %q", reactDef.Name, "music_agent")
	}
	if len(reactDef.Tools) != 1 {
		t.Errorf("len(Tools) = %d, want 1", len(reactDef.Tools))
	}
	if reactDef.Tools[0].Ref != "play_song" {
		t.Errorf("Tools[0].Ref = %q, want %q", reactDef.Tools[0].Ref, "play_song")
	}

	// Load weather_agent
	agentDef2, err := rt.GetAgentDef(ctx, "weather_agent")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	reactDef2 := agentcfg.AsReActAgent(agentDef2)
	if reactDef2 == nil {
		t.Fatalf("expected ReActAgentDef, got %T", agentDef2)
	}

	if reactDef2.Name != "weather_agent" {
		t.Errorf("Name = %q, want %q", reactDef2.Name, "weather_agent")
	}
}

func TestMatchAgent_LoadToolsFromStore(t *testing.T) {
	ctx := context.Background()
	rt := setupMatchAgentTestRuntime(t)

	// Load play_song tool
	toolDef, err := rt.GetToolDef(ctx, "play_song")
	if err != nil {
		t.Fatalf("GetToolDef error: %v", err)
	}

	builtinDef := agentcfg.AsBuiltInTool(toolDef)
	if builtinDef == nil {
		t.Fatalf("expected BuiltInToolDef, got %T", toolDef)
	}

	if builtinDef.Name != "play_song" {
		t.Errorf("Name = %q, want %q", builtinDef.Name, "play_song")
	}

	// Load get_weather tool
	toolDef2, err := rt.GetToolDef(ctx, "get_weather")
	if err != nil {
		t.Fatalf("GetToolDef error: %v", err)
	}

	builtinDef2 := agentcfg.AsBuiltInTool(toolDef2)
	if builtinDef2 == nil {
		t.Fatalf("expected BuiltInToolDef, got %T", toolDef2)
	}

	if builtinDef2.Name != "get_weather" {
		t.Errorf("Name = %q, want %q", builtinDef2.Name, "get_weather")
	}
}

func TestMatchAgentDef_RouteMapping(t *testing.T) {
	ctx := context.Background()
	rt := setupMatchAgentTestRuntime(t)

	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	if matchDef == nil {
		t.Fatalf("expected MatchAgentDef, got %T", agentDef)
	}

	// Build route map (rules -> route)
	routeMap := make(map[string]*agentcfg.MatchRoute)
	for i := range matchDef.Route {
		route := &matchDef.Route[i]
		for _, rule := range route.Rules {
			routeMap[rule] = route
		}
	}

	// Verify play_music route
	if route, ok := routeMap["play_music"]; ok {
		if route.Agent.Ref != "music_agent" {
			t.Errorf("play_music route agent = %q, want %q", route.Agent.Ref, "music_agent")
		}
	} else {
		t.Error("play_music route not found")
	}

	// Verify weather_query route
	if route, ok := routeMap["weather_query"]; ok {
		if route.Agent.Ref != "weather_agent" {
			t.Errorf("weather_query route agent = %q, want %q", route.Agent.Ref, "weather_agent")
		}
	} else {
		t.Error("weather_query route not found")
	}

	// Verify greeting route (inline agent)
	if route, ok := routeMap["greeting"]; ok {
		if route.Agent.Ref != "" {
			t.Errorf("greeting route should have inline agent, got ref = %q", route.Agent.Ref)
		}
		if route.Agent.Agent == nil {
			t.Error("greeting route inline AgentDef is nil")
		}
	} else {
		t.Error("greeting route not found")
	}
}

func TestMatchAgentDef_InlineRule(t *testing.T) {
	ctx := context.Background()
	rt := setupMatchAgentTestRuntime(t)

	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	if matchDef == nil {
		t.Fatalf("expected MatchAgentDef, got %T", agentDef)
	}

	// Find the inline greeting rule
	var greetingRule *agentcfg.RuleRef
	for i := range matchDef.Rules {
		if matchDef.Rules[i].Rule != nil && matchDef.Rules[i].Rule.Name == "greeting" {
			greetingRule = &matchDef.Rules[i]
			break
		}
	}

	if greetingRule == nil {
		t.Fatal("greeting rule not found")
	}

	if len(greetingRule.Rule.Examples) != 3 {
		t.Errorf("len(Examples) = %d, want 3", len(greetingRule.Rule.Examples))
	}
}

func TestMatchAgent_Create(t *testing.T) {
	ctx := context.Background()
	rt := setupMatchAgentTestRuntime(t)

	// Load match agent definition
	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	if matchDef == nil {
		t.Fatalf("expected MatchAgentDef, got %T", agentDef)
	}

	// Create MatchAgent
	matchAgent, err := agent.NewMatchAgent(ctx, matchDef, rt, "")
	if err != nil {
		t.Fatalf("NewMatchAgent error: %v", err)
	}
	defer matchAgent.Close()

	// Verify agent properties
	if matchAgent.Def() == nil {
		t.Error("Def() is nil")
	}
	if matchAgent.State() == nil {
		t.Error("State() is nil")
	}
	if matchAgent.StateID() == "" {
		t.Error("StateID() is empty")
	}
}

func TestMatchAgent_State(t *testing.T) {
	ctx := context.Background()
	rt := setupMatchAgentTestRuntime(t)

	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	matchAgent, err := agent.NewMatchAgent(ctx, matchDef, rt, "")
	if err != nil {
		t.Fatalf("NewMatchAgent error: %v", err)
	}
	defer matchAgent.Close()

	// Verify initial state
	state := matchAgent.State()
	if state == nil {
		t.Fatal("State() is nil")
	}

	// State should have agent def set
	if state.AgentDef() != "intent_router" {
		t.Errorf("AgentDef() = %q, want %q", state.AgentDef(), "intent_router")
	}
}

func TestMatchAgent_FormatHistory(t *testing.T) {
	ctx := context.Background()
	rt := setupMatchAgentTestRuntime(t)

	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	matchAgent, err := agent.NewMatchAgent(ctx, matchDef, rt, "")
	if err != nil {
		t.Fatalf("NewMatchAgent error: %v", err)
	}
	defer matchAgent.Close()

	// FormatHistory should not panic on empty state
	history := matchAgent.FormatHistory(ctx)
	if history == "" {
		// Empty history is expected for new agent
		t.Log("FormatHistory returned empty string (expected)")
	}
}

func TestMatchAgent_Close(t *testing.T) {
	ctx := context.Background()
	rt := setupMatchAgentTestRuntime(t)

	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	matchAgent, err := agent.NewMatchAgent(ctx, matchDef, rt, "")
	if err != nil {
		t.Fatalf("NewMatchAgent error: %v", err)
	}

	// Close should not error
	if err := matchAgent.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}

	// Close again should not error (idempotent)
	if err := matchAgent.Close(); err != nil {
		t.Errorf("Close again error: %v", err)
	}
}

func TestMatchAgent_NoMatch_ReturnsEOF(t *testing.T) {
	ctx := context.Background()
	// Use empty match result to simulate no match
	rt := setupMatchAgentTestRuntimeWithMatchResult(t, "")

	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	matchAgent, err := agent.NewMatchAgent(ctx, matchDef, rt, "")
	if err != nil {
		t.Fatalf("NewMatchAgent error: %v", err)
	}
	defer matchAgent.Close()

	// Send input
	if err := matchAgent.Input(genx.Contents{genx.Text("random text that won't match")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Get result - should be EOF since no match
	evt, err := matchAgent.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}

	if evt.Type != agent.EventEOF {
		t.Errorf("event type = %v, want EventEOF", evt.Type)
	}

	// Phase should be idle (empty string) for no match
	if evt.Phase != "" {
		t.Errorf("Phase = %q, want empty string (idle)", evt.Phase)
	}
}

func TestMatchAgent_Match_ExecutesSubAgent(t *testing.T) {
	ctx := context.Background()
	// Configure to match "greeting" rule
	rt := setupMatchAgentTestRuntimeWithMatchResult(t, "greeting")

	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	matchAgent, err := agent.NewMatchAgent(ctx, matchDef, rt, "")
	if err != nil {
		t.Fatalf("NewMatchAgent error: %v", err)
	}
	defer matchAgent.Close()

	// Send input that will match "greeting"
	if err := matchAgent.Input(genx.Contents{genx.Text("hello")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Collect events
	var events []*agent.AgentEvent
	for {
		evt, err := matchAgent.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		events = append(events, evt)

		// Stop on EOF or Closed
		if evt.Type == agent.EventEOF || evt.Type == agent.EventClosed {
			break
		}
	}

	if len(events) == 0 {
		t.Fatal("no events received")
	}

	// Should end with EOF (sub-agent needs input)
	lastEvt := events[len(events)-1]
	if lastEvt.Type != agent.EventEOF {
		t.Errorf("last event type = %v, want EventEOF", lastEvt.Type)
	}

	// Phase should be "executing" when sub-agent is running
	if lastEvt.Phase != "executing" {
		t.Errorf("last event Phase = %q, want %q", lastEvt.Phase, "executing")
	}

	t.Logf("Received %d events", len(events))
	for i, evt := range events {
		t.Logf("Event %d: Type=%v, Phase=%q", i, evt.Type, evt.Phase)
	}
}

// intentSwitchGenerator is a mock generator that can dynamically return different match results
// based on call count, to test intent switching scenarios.
type intentSwitchGenerator struct {
	matchCallCount int
	matchResults   []string // sequence of match results to return
	mu             chan struct{}
}

func newIntentSwitchGenerator(matchResults ...string) *intentSwitchGenerator {
	g := &intentSwitchGenerator{
		matchResults: matchResults,
		mu:           make(chan struct{}, 1),
	}
	g.mu <- struct{}{}
	return g
}

func (g *intentSwitchGenerator) GenerateStream(ctx context.Context, model string, mc genx.ModelContext) (genx.Stream, error) {
	<-g.mu
	defer func() { g.mu <- struct{}{} }()

	// Return match result in sequence
	if g.matchCallCount < len(g.matchResults) {
		result := g.matchResults[g.matchCallCount]
		g.matchCallCount++
		if result == "" {
			// Empty string means no match
			return &mockMatchStream{response: "NONE\n"}, nil
		}
		return &mockMatchStream{response: result + "\n"}, nil
	}

	// Default: return text response for sub-agent
	return &mockMatchStream{response: "Hello from sub-agent"}, nil
}

func (g *intentSwitchGenerator) Invoke(ctx context.Context, model string, mc genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	return genx.Usage{}, nil, nil
}

func setupIntentSwitchTestRuntime(t *testing.T, matchResults ...string) *playground.Runtime {
	t.Helper()
	store := playground.NewStore(nil)
	if err := store.LoadReadonlyLayer("testdata", os.DirFS("testdata/agent_match_test")); err != nil {
		t.Fatalf("load testdata: %v", err)
	}

	return playground.NewRuntime(
		playground.WithStore(store),
		playground.WithGenerator(newIntentSwitchGenerator(matchResults...)),
	)
}

// TestMatchAgent_IntentSwitch_ToDifferentRule tests switching from one rule to another.
func TestMatchAgent_IntentSwitch_ToDifferentRule(t *testing.T) {
	ctx := context.Background()
	// First match: greeting, Second match: play_music (different rule)
	rt := setupIntentSwitchTestRuntime(t, "greeting", "play_music")

	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	matchAgent, err := agent.NewMatchAgent(ctx, matchDef, rt, "")
	if err != nil {
		t.Fatalf("NewMatchAgent error: %v", err)
	}
	defer matchAgent.Close()

	// First input: matches "greeting"
	if err := matchAgent.Input(genx.Contents{genx.Text("hello")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Drain events until EOF (sub-agent needs input)
	for {
		evt, err := matchAgent.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if evt.Type == agent.EventEOF {
			break
		}
	}

	// Second input: matches "play_music" -> should switch intent
	if err := matchAgent.Input(genx.Contents{genx.Text("play some music")}); err != nil {
		t.Fatalf("Second Input error: %v", err)
	}

	// Verify we get events from the new agent
	var events []*agent.AgentEvent
	for {
		evt, err := matchAgent.Next()
		if err != nil {
			t.Fatalf("Next error after switch: %v", err)
		}
		events = append(events, evt)
		if evt.Type == agent.EventEOF || evt.Type == agent.EventClosed {
			break
		}
	}

	if len(events) == 0 {
		t.Fatal("no events received after intent switch")
	}

	t.Logf("Intent switch test: received %d events after switch", len(events))
	for i, evt := range events {
		t.Logf("Event %d: Type=%v, Phase=%q", i, evt.Type, evt.Phase)
	}
}

// TestMatchAgent_IntentSwitch_SameRuleNoSwitch tests that same rule does not trigger a switch.
func TestMatchAgent_IntentSwitch_SameRuleNoSwitch(t *testing.T) {
	ctx := context.Background()
	// Both matches return "greeting" (same rule)
	rt := setupIntentSwitchTestRuntime(t, "greeting", "greeting")

	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	matchAgent, err := agent.NewMatchAgent(ctx, matchDef, rt, "")
	if err != nil {
		t.Fatalf("NewMatchAgent error: %v", err)
	}
	defer matchAgent.Close()

	// First input: matches "greeting"
	if err := matchAgent.Input(genx.Contents{genx.Text("hello")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Drain events until EOF
	for {
		evt, err := matchAgent.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if evt.Type == agent.EventEOF {
			break
		}
	}

	// Second input: still matches "greeting" -> should NOT switch, continue with same agent
	if err := matchAgent.Input(genx.Contents{genx.Text("hi there")}); err != nil {
		t.Fatalf("Second Input error: %v", err)
	}

	// Get events - should continue with current sub-agent
	var events []*agent.AgentEvent
	for {
		evt, err := matchAgent.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		events = append(events, evt)
		// Phase should be "executing" (continuing with same agent)
		if evt.Type == agent.EventEOF || evt.Type == agent.EventClosed {
			break
		}
	}

	if len(events) == 0 {
		t.Fatal("no events received")
	}

	// Should still be in executing phase (same agent)
	lastEvt := events[len(events)-1]
	if lastEvt.Phase != "executing" {
		t.Errorf("Phase = %q, want %q (should continue with same agent)", lastEvt.Phase, "executing")
	}

	t.Logf("Same rule test: received %d events (no switch)", len(events))
}

// TestMatchAgent_IntentSwitch_NoMatchNoSwitch tests that no match does not trigger a switch.
func TestMatchAgent_IntentSwitch_NoMatchNoSwitch(t *testing.T) {
	ctx := context.Background()
	// First match: greeting, Second match: empty (no match)
	rt := setupIntentSwitchTestRuntime(t, "greeting", "")

	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	matchAgent, err := agent.NewMatchAgent(ctx, matchDef, rt, "")
	if err != nil {
		t.Fatalf("NewMatchAgent error: %v", err)
	}
	defer matchAgent.Close()

	// First input: matches "greeting"
	if err := matchAgent.Input(genx.Contents{genx.Text("hello")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Drain events until EOF
	for {
		evt, err := matchAgent.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if evt.Type == agent.EventEOF {
			break
		}
	}

	// Second input: no match -> should NOT switch, forward input to current agent
	if err := matchAgent.Input(genx.Contents{genx.Text("random gibberish")}); err != nil {
		t.Fatalf("Second Input error: %v", err)
	}

	// Get events - should continue with current sub-agent
	var events []*agent.AgentEvent
	for {
		evt, err := matchAgent.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		events = append(events, evt)
		if evt.Type == agent.EventEOF || evt.Type == agent.EventClosed {
			break
		}
	}

	if len(events) == 0 {
		t.Fatal("no events received")
	}

	// Should still be in executing phase (same agent handles unmatched input)
	lastEvt := events[len(events)-1]
	if lastEvt.Phase != "executing" {
		t.Errorf("Phase = %q, want %q (should continue with same agent)", lastEvt.Phase, "executing")
	}

	t.Logf("No match test: received %d events (no switch)", len(events))
}

// TestMatchAgent_Interrupt tests the Interrupt method.
func TestMatchAgent_Interrupt(t *testing.T) {
	ctx := context.Background()
	rt := setupMatchAgentTestRuntimeWithMatchResult(t, "greeting")

	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	matchAgent, err := agent.NewMatchAgent(ctx, matchDef, rt, "")
	if err != nil {
		t.Fatalf("NewMatchAgent error: %v", err)
	}
	defer matchAgent.Close()

	// Send input to start the agent
	if err := matchAgent.Input(genx.Contents{genx.Text("hello")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Drain events until EOF
	for {
		evt, err := matchAgent.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if evt.Type == agent.EventEOF {
			break
		}
	}

	// Now interrupt the agent
	err = matchAgent.Interrupt()
	if err != nil {
		t.Fatalf("Interrupt error: %v", err)
	}

	t.Log("Interrupt test passed")
}

// TestMatchAgent_Revert tests the Revert method.
func TestMatchAgent_Revert(t *testing.T) {
	ctx := context.Background()
	rt := setupMatchAgentTestRuntimeWithMatchResult(t, "greeting")

	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	matchAgent, err := agent.NewMatchAgent(ctx, matchDef, rt, "")
	if err != nil {
		t.Fatalf("NewMatchAgent error: %v", err)
	}
	defer matchAgent.Close()

	// Send input to start the agent
	if err := matchAgent.Input(genx.Contents{genx.Text("hello")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Drain events until EOF
	for {
		evt, err := matchAgent.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if evt.Type == agent.EventEOF {
			break
		}
	}

	// Now revert - should forward to the calling agent
	err = matchAgent.Revert()
	if err != nil {
		t.Fatalf("Revert error: %v", err)
	}

	t.Log("Revert test passed")
}

// TestMatchAgent_Revert_NoCalling tests Revert when there's no calling agent.
func TestMatchAgent_Revert_NoCalling(t *testing.T) {
	ctx := context.Background()
	// Use empty match result so no agent is started
	rt := setupMatchAgentTestRuntimeWithMatchResult(t, "")

	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	matchAgent, err := agent.NewMatchAgent(ctx, matchDef, rt, "")
	if err != nil {
		t.Fatalf("NewMatchAgent error: %v", err)
	}
	defer matchAgent.Close()

	// Send input that doesn't match anything
	if err := matchAgent.Input(genx.Contents{genx.Text("random")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Drain events until EOF (no match)
	for {
		evt, err := matchAgent.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if evt.Type == agent.EventEOF || evt.Type == agent.EventClosed {
			break
		}
	}

	// Revert with no calling agent - returns nil (no-op)
	err = matchAgent.Revert()
	if err != nil {
		t.Fatalf("Revert error: %v", err)
	}

	t.Log("Revert with no calling (no-op) test passed")
}

// TestMatchAgent_NoEventClosedFromSubAgent tests that when a sub-agent finishes (sends EventClosed),
// the MatchAgent (router) should NOT forward EventClosed to the caller.
// This is important because EventClosed from the router means the router itself is closed,
// not that a sub-agent has finished.
func TestMatchAgent_NoEventClosedFromSubAgent(t *testing.T) {
	ctx := context.Background()

	// Use setupMatchAgentTestRuntimeWithMatchResult which sets up matching for "greeting" rule
	// The greeting rule uses an inline agent that doesn't require builtin tools
	rt := setupMatchAgentTestRuntimeWithMatchResult(t, "greeting")

	// Load MatchAgent def
	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	if matchDef == nil {
		t.Fatal("Expected MatchAgentDef")
	}

	// Create MatchAgent
	ma, err := agent.NewMatchAgent(ctx, matchDef, rt, "")
	if err != nil {
		t.Fatalf("NewMatchAgent error: %v", err)
	}
	defer ma.Close()

	// Send input
	if err := ma.Input(genx.Contents{genx.Text("hello")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Collect all events
	var events []*agent.AgentEvent
	var sawEventClosed bool

	for {
		evt, err := ma.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}

		events = append(events, evt)
		t.Logf("Event %d: Type=%s, Phase=%s", len(events)-1, evt.Type, evt.Phase)

		if evt.Type == agent.EventClosed {
			sawEventClosed = true
			t.Errorf("Received EventClosed - router should NOT forward sub-agent's EventClosed")
		}

		if evt.Type == agent.EventEOF {
			break
		}

		// Safety limit
		if len(events) > 50 {
			t.Fatal("too many events")
		}
	}

	// Verify we did NOT see EventClosed
	if sawEventClosed {
		t.Error("MatchAgent forwarded EventClosed from sub-agent, which is incorrect")
	}

	// Verify we got EventEOF at the end
	lastEvent := events[len(events)-1]
	if lastEvent.Type != agent.EventEOF {
		t.Errorf("Last event Type = %s, want EventEOF", lastEvent.Type)
	}

	t.Logf("Test passed: received %d events, no EventClosed, ended with EventEOF", len(events))
}

// TestMatchAgentGenerator_Invoke tests the matchAgentGenerator.Invoke method.
// This method is part of the genx.Generator interface but is not currently used
// by the match package (which only uses GenerateStream).
func TestMatchAgentGenerator_Invoke(t *testing.T) {
	ctx := context.Background()

	// Create a mock generator that tracks Invoke calls
	invokeCallCount := 0
	var invokedModel string
	var invokedTool *genx.FuncTool

	mockGen := &mockInvokeTrackingGenerator{
		onInvoke: func(ctx context.Context, model string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
			invokeCallCount++
			invokedModel = model
			invokedTool = tool
			return genx.Usage{PromptTokenCount: 10, GeneratedTokenCount: 20}, &genx.FuncCall{
				Name:      "test_output",
				Arguments: `{"result": "success"}`,
			}, nil
		},
	}

	rt := playground.NewRuntime(
		playground.WithGenerator(mockGen),
	)

	// Create a test tool
	type testArgs struct {
		Input string `json:"input"`
	}
	testTool, err := genx.NewFuncTool[testArgs](
		"test_tool",
		"A test tool",
		genx.InvokeFunc[testArgs](func(ctx context.Context, call *genx.FuncCall, args testArgs) (any, error) {
			return "result", nil
		}),
	)
	if err != nil {
		t.Fatalf("NewFuncTool error: %v", err)
	}

	// Build a model context
	mcb := &genx.ModelContextBuilder{}
	mcb.UserText("", "test input")
	mctx := mcb.Build()

	// Call Invoke through the exported test function
	usage, funcCall, err := agent.InvokeMatchAgentGenerator(ctx, rt, "test-model", mctx, testTool)
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}

	// Verify the call was forwarded to the runtime
	if invokeCallCount != 1 {
		t.Errorf("Invoke call count = %d, want 1", invokeCallCount)
	}

	// Verify the model was passed correctly
	if invokedModel != "test-model" {
		t.Errorf("Invoked model = %q, want %q", invokedModel, "test-model")
	}

	// Verify the tool was passed correctly
	if invokedTool != testTool {
		t.Errorf("Invoked tool mismatch")
	}

	// Verify the return values
	if usage.PromptTokenCount != 10 || usage.GeneratedTokenCount != 20 {
		t.Errorf("Usage = %+v, want PromptTokenCount=10, GeneratedTokenCount=20", usage)
	}

	if funcCall == nil || funcCall.Name != "test_output" {
		t.Errorf("FuncCall = %+v, want Name=test_output", funcCall)
	}
}

// mockInvokeTrackingGenerator is a mock generator that tracks Invoke calls.
type mockInvokeTrackingGenerator struct {
	onInvoke func(ctx context.Context, model string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error)
}

func (g *mockInvokeTrackingGenerator) GenerateStream(ctx context.Context, model string, mctx genx.ModelContext) (genx.Stream, error) {
	return &mockMatchStream{response: "mock"}, nil
}

func (g *mockInvokeTrackingGenerator) Invoke(ctx context.Context, model string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	if g.onInvoke != nil {
		return g.onInvoke(ctx, model, mctx, tool)
	}
	return genx.Usage{}, nil, nil
}

// TestMatchAgent_SubAgent_ParentStateID tests that when MatchAgent creates a sub-agent,
// the sub-agent's ParentStateID is set to the MatchAgent's StateID.
func TestMatchAgent_SubAgent_ParentStateID(t *testing.T) {
	ctx := context.Background()
	// Configure to match "greeting" rule
	rt := setupMatchAgentTestRuntimeWithMatchResult(t, "greeting")

	agentDef, err := rt.GetAgentDef(ctx, "intent_router")
	if err != nil {
		t.Fatalf("GetAgentDef error: %v", err)
	}

	matchDef := agentcfg.AsMatchAgent(agentDef)
	matchAgent, err := agent.NewMatchAgent(ctx, matchDef, rt, "")
	if err != nil {
		t.Fatalf("NewMatchAgent error: %v", err)
	}
	defer matchAgent.Close()

	// Verify top-level MatchAgent has empty ParentStateID
	matchState := matchAgent.State()
	if matchState.ParentStateID() != "" {
		t.Errorf("top-level MatchAgent's ParentStateID should be empty, got %q", matchState.ParentStateID())
	}

	matchStateID := matchAgent.StateID()
	if matchStateID == "" {
		t.Fatal("MatchAgent should have a state ID")
	}

	// Send input that will trigger matching and sub-agent creation
	if err := matchAgent.Input(genx.Contents{genx.Text("hello")}); err != nil {
		t.Fatalf("Input error: %v", err)
	}

	// Read events until we get EOF (sub-agent is active)
	for {
		evt, err := matchAgent.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}

		// When we get EOF with "executing" phase, sub-agent should be active
		if evt.Type == agent.EventEOF && evt.Phase == "executing" {
			break
		}
		if evt.Type == agent.EventClosed {
			t.Fatal("unexpected EventClosed before sub-agent was activated")
		}
	}

	// Now get the calling sub-agent and verify its ParentStateID
	calling := matchAgent.GetCalling()
	if calling == nil {
		t.Fatal("expected calling sub-agent to be active")
	}

	subAgentState := calling.State()
	subAgentParentID := subAgentState.ParentStateID()

	if subAgentParentID != matchStateID {
		t.Errorf("sub-agent's ParentStateID = %q, want %q (MatchAgent's StateID)", subAgentParentID, matchStateID)
	}

	t.Logf("MatchAgent StateID: %s", matchStateID)
	t.Logf("Sub-agent ParentStateID: %s", subAgentParentID)
	t.Log("Call stack verified: sub-agent correctly references parent MatchAgent")
}
