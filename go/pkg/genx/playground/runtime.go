package playground

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/haivivi/giztoy/pkg/genx"
	"github.com/haivivi/giztoy/pkg/genx/agent"
	"github.com/haivivi/giztoy/pkg/genx/agentcfg"
	"github.com/haivivi/giztoy/pkg/genx/generators"
	"github.com/haivivi/giztoy/pkg/genx/match"
)

// Logger is an interface for logging runtime events.
type Logger interface {
	// Debug logs a debug message with optional key-value pairs.
	Debug(msg string, keysAndValues ...any)
	// Info logs an info message with optional key-value pairs.
	Info(msg string, keysAndValues ...any)
	// Error logs an error message with optional key-value pairs.
	Error(msg string, keysAndValues ...any)
}

// noopLogger is a logger that does nothing.
type noopLogger struct{}

func (noopLogger) Debug(msg string, keysAndValues ...any) {}
func (noopLogger) Info(msg string, keysAndValues ...any)  {}
func (noopLogger) Error(msg string, keysAndValues ...any) {}

// Resource type constants for Store lookup.
// Note: Using underscore instead of colon for Bazel compatibility (colon is reserved in Bazel labels).
const (
	TypeToolV1    = "tool_v1"
	TypeAgentV1   = "agent_v1"
	TypeContextV1 = "context_v1"
	TypeRuleV1    = "rule_v1"
	TypeStateV1   = "state_v1"
)

// resourcePrefixes contains all valid resource type prefixes for parseRef.
var resourcePrefixes = []string{
	"tool:",
	"agent:",
	"context:",
	"rule:",
	"state:",
}

// parseRef parses a ref string (e.g., "tool:play_music") and returns the name part.
// If the ref has a type prefix, it returns just the name; otherwise returns the input unchanged.
// Examples:
//   - "tool:play_music" -> "play_music"
//   - "agent:router" -> "router"
//   - "my_tool" -> "my_tool"
func parseRef(ref string) string {
	for _, prefix := range resourcePrefixes {
		if strings.HasPrefix(ref, prefix) {
			return ref[len(prefix):]
		}
	}
	return ref
}

// Runtime implements agent.Runtime for playground/testing purposes.
// It uses Store as a readonly storage layer and provides simple state management.
type Runtime struct {
	gen    genx.Generator
	store  *Store
	logger Logger

	// builtinTools stores pre-registered tools that take precedence over store lookup.
	builtinTools map[string]*genx.FuncTool

	mu     sync.RWMutex
	states map[string]agent.AgentState
}

// RuntimeOption is a functional option for configuring Runtime.
type RuntimeOption func(*Runtime)

// WithGenerator sets the generator for the runtime.
func WithGenerator(g genx.Generator) RuntimeOption {
	return func(r *Runtime) {
		r.gen = g
	}
}

// WithStore sets the store for the runtime.
func WithStore(s *Store) RuntimeOption {
	return func(r *Runtime) {
		r.store = s
	}
}

// WithBuiltinTools registers built-in tools that take precedence over store lookup.
func WithBuiltinTools(tools ...*genx.FuncTool) RuntimeOption {
	return func(r *Runtime) {
		if r.builtinTools == nil {
			r.builtinTools = make(map[string]*genx.FuncTool)
		}
		for _, tool := range tools {
			r.builtinTools[tool.Name] = tool
		}
	}
}

// WithLogger sets the logger for the runtime.
func WithLogger(l Logger) RuntimeOption {
	return func(r *Runtime) {
		r.logger = l
	}
}

// NewRuntime creates a new playground Runtime.
func NewRuntime(opts ...RuntimeOption) *Runtime {
	r := &Runtime{
		states: make(map[string]agent.AgentState),
		logger: noopLogger{},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// log returns the logger, never nil.
func (r *Runtime) log() Logger {
	if r.logger == nil {
		return noopLogger{}
	}
	return r.logger
}

// Store returns the underlying store.
func (r *Runtime) Store() *Store {
	return r.store
}

// --- Generator (embedded interface) ---

func (r *Runtime) generator() genx.Generator {
	if r.gen != nil {
		return r.gen
	}
	return generators.DefaultMux
}

func (r *Runtime) GenerateStream(ctx context.Context, model string, mctx genx.ModelContext) (genx.Stream, error) {
	return r.generator().GenerateStream(ctx, model, mctx)
}

func (r *Runtime) Invoke(ctx context.Context, model string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	return r.generator().Invoke(ctx, model, mctx, tool)
}

// --- Store Helpers ---

// getFromStore retrieves a value from store by prefixed key.
// The key format is "{type}/{name}", e.g. "tool_v1/my_tool".
// The type is determined by the key prefix, not by file content.
func (r *Runtime) getFromStore(name, resourceType string) (map[string]any, error) {
	if r.store == nil {
		return nil, fmt.Errorf("no store configured")
	}
	key := resourceType + "/" + name
	data, ok := r.store.Get(key)
	if !ok {
		return nil, fmt.Errorf("%q not found", key)
	}
	return data, nil
}

// marshalData converts map[string]any to JSON bytes for unmarshaling.
func marshalData(data map[string]any) ([]byte, error) {
	return json.Marshal(data)
}

// --- Tool Management ---

func (r *Runtime) GetTool(ctx context.Context, name string) (*genx.FuncTool, error) {
	// Handle ref format (e.g., "tool:play_music" -> "play_music")
	origName := name
	name = parseRef(name)
	r.log().Debug("GetTool", "ref", origName, "name", name)

	// Check built-in tools first
	if tool, ok := r.builtinTools[name]; ok {
		r.log().Debug("GetTool: found builtin", "name", name)
		return tool, nil
	}
	// In playground, GetTool requires CreateToolFromDef which needs actual implementation.
	// First get the ToolDef, then create FuncTool from it.
	def, err := r.GetToolDef(ctx, name)
	if err != nil {
		r.log().Error("GetTool: failed to get def", "name", name, "error", err)
		return nil, err
	}
	r.log().Debug("GetTool: got def", "name", name, "type", fmt.Sprintf("%T", def))
	return r.CreateToolFromDef(ctx, def)
}

func (r *Runtime) GetToolDef(ctx context.Context, name string) (agentcfg.Tool, error) {
	// Handle ref format (e.g., "tool:play_music" -> "play_music")
	name = parseRef(name)
	data, err := r.getFromStore(name, TypeToolV1)
	if err != nil {
		return nil, fmt.Errorf("get tool def: %w", err)
	}
	jsonData, err := marshalData(data)
	if err != nil {
		return nil, fmt.Errorf("marshal tool def data: %w", err)
	}
	def, err := agentcfg.UnmarshalTool(jsonData)
	if err != nil {
		return nil, fmt.Errorf("unmarshal tool def: %w", err)
	}
	return def, nil
}

func (r *Runtime) CreateToolFromDef(ctx context.Context, def agentcfg.Tool) (*genx.FuncTool, error) {
	r.log().Debug("CreateToolFromDef", "type", fmt.Sprintf("%T", def), "name", def.ToolName())

	switch d := def.(type) {
	case *agentcfg.BuiltInTool:
		// For built-in tools, look up in builtinTools map
		if tool, ok := r.builtinTools[d.Name]; ok {
			r.log().Debug("CreateToolFromDef: found builtin", "name", d.Name)
			return tool, nil
		}
		r.log().Error("CreateToolFromDef: builtin not found", "name", d.Name)
		return nil, fmt.Errorf("builtin tool %q not found", d.Name)

	case *agentcfg.HTTPTool:
		r.log().Debug("CreateToolFromDef: creating HTTP tool", "name", d.Name)
		httpTool := agent.NewHTTPTool(r, nil) // Use default HTTP client
		return httpTool.CreateFuncTool(d)

	case *agentcfg.GeneratorTool:
		r.log().Debug("CreateToolFromDef: creating Generator tool", "name", d.Name)
		genTool := agent.NewGeneratorTool(r)
		return genTool.CreateFuncTool(ctx, d)

	case *agentcfg.TextProcessorTool:
		r.log().Debug("CreateToolFromDef: creating TextProcessor tool", "name", d.Name)
		textTool := agent.NewTextProcessorTool(r)
		return textTool.CreateFuncTool(d)

	case *agentcfg.CompositeTool:
		r.log().Debug("CreateToolFromDef: creating Composite tool", "name", d.Name)
		compositeTool := agent.NewCompositeTool(r)
		return compositeTool.CreateFuncTool(ctx, d)

	default:
		r.log().Error("CreateToolFromDef: unsupported type", "type", fmt.Sprintf("%T", def))
		return nil, fmt.Errorf("unsupported tool type: %T", def)
	}
}

// --- Agent Management ---

func (r *Runtime) GetAgentDef(ctx context.Context, name string) (agentcfg.Agent, error) {
	// Handle ref format (e.g., "agent:router" -> "router")
	origName := name
	name = parseRef(name)
	r.log().Debug("GetAgentDef", "ref", origName, "name", name)

	data, err := r.getFromStore(name, TypeAgentV1)
	if err != nil {
		r.log().Error("GetAgentDef: not found", "name", name, "error", err)
		return nil, fmt.Errorf("get agent def: %w", err)
	}
	jsonData, err := marshalData(data)
	if err != nil {
		return nil, fmt.Errorf("marshal agent def data: %w", err)
	}
	def, err := agentcfg.UnmarshalAgent(jsonData)
	if err != nil {
		r.log().Error("GetAgentDef: unmarshal failed", "name", name, "error", err)
		return nil, fmt.Errorf("unmarshal agent def: %w", err)
	}
	r.log().Debug("GetAgentDef: loaded", "name", name, "type", fmt.Sprintf("%T", def))
	return def, nil
}

// --- Context Builder Management ---

// promptContextBuilder is a simple ContextBuilder that returns a prompt.
type promptContextBuilder struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

func (b *promptContextBuilder) BuildContext(ctx context.Context) genx.ModelContext {
	mcb := &genx.ModelContextBuilder{}
	mcb.PromptText(b.Name, b.Prompt)
	return mcb.Build()
}

func (r *Runtime) GetContextBuilder(ctx context.Context, name string) (agent.ContextBuilder, error) {
	// Handle ref format (e.g., "context:elsa" -> "elsa")
	name = parseRef(name)
	data, err := r.getFromStore(name, TypeContextV1)
	if err != nil {
		return nil, fmt.Errorf("get context builder: %w", err)
	}
	jsonData, err := marshalData(data)
	if err != nil {
		return nil, fmt.Errorf("marshal context builder data: %w", err)
	}
	var cb promptContextBuilder
	if err := json.Unmarshal(jsonData, &cb); err != nil {
		return nil, fmt.Errorf("unmarshal context builder: %w", err)
	}
	return &cb, nil
}

// --- Rule Management ---

func (r *Runtime) GetRule(ctx context.Context, name string) (*match.Rule, error) {
	// Handle ref format (e.g., "rule:play_music" -> "play_music")
	name = parseRef(name)
	data, err := r.getFromStore(name, TypeRuleV1)
	if err != nil {
		return nil, fmt.Errorf("get rule: %w", err)
	}
	jsonData, err := marshalData(data)
	if err != nil {
		return nil, fmt.Errorf("marshal rule data: %w", err)
	}
	var rule match.Rule
	if err := json.Unmarshal(jsonData, &rule); err != nil {
		return nil, fmt.Errorf("unmarshal rule: %w", err)
	}
	return &rule, nil
}

// --- State Management ---

func (r *Runtime) CreateReActState(ctx context.Context, agentDef string, parentStateID string) (agent.ReActState, error) {
	state := newReActState(parentStateID)
	state.base.setAgentDef(agentDef)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states[state.ID()] = state
	r.log().Debug("CreateReActState", "id", state.ID(), "agentDef", agentDef, "parentStateID", parentStateID)
	return state, nil
}

func (r *Runtime) CreateMatchState(ctx context.Context, agentDef string, parentStateID string) (agent.MatchState, error) {
	state := newMatchState(parentStateID)
	state.base.setAgentDef(agentDef)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states[state.ID()] = state
	r.log().Debug("CreateMatchState", "id", state.ID(), "agentDef", agentDef, "parentStateID", parentStateID)
	return state, nil
}

func (r *Runtime) GetState(ctx context.Context, id string) (agent.AgentState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, ok := r.states[id]
	if !ok {
		r.log().Debug("GetState: not found", "id", id)
		return nil, nil // Not found, return nil without error
	}
	r.log().Debug("GetState: found", "id", id, "type", fmt.Sprintf("%T", state))
	return state, nil
}

func (r *Runtime) DestroyState(ctx context.Context, id string, archive bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// For playground, we just delete the state
	// archive parameter is ignored
	r.log().Debug("DestroyState", "id", id, "archive", archive)
	delete(r.states, id)
	return nil
}

// --- State Serialization (playground extension) ---

// SaveState serializes a state to the writable layer of the store.
// The key format is "state_v1/{id}".
func (r *Runtime) SaveState(ctx context.Context, state agent.AgentState) error {
	if r.store == nil {
		return fmt.Errorf("no store configured")
	}

	var data []byte
	var err error

	switch s := state.(type) {
	case *ReActStateImpl:
		data, err = json.Marshal(s)
	case *MatchStateImpl:
		data, err = json.Marshal(s)
	default:
		return fmt.Errorf("unsupported state type: %T", state)
	}

	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("unmarshal to map: %w", err)
	}

	key := TypeStateV1 + "/" + state.ID()
	r.store.Set(key, m)
	return nil
}

// LoadState deserializes a state from the store.
// The key format is "state_v1/{id}".
func (r *Runtime) LoadState(ctx context.Context, id string) (agent.AgentState, error) {
	// Handle ref format (e.g., "state:music_playing" -> "music_playing")
	id = parseRef(id)
	data, err := r.getFromStore(id, TypeStateV1)
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	jsonData, err := marshalData(data)
	if err != nil {
		return nil, fmt.Errorf("marshal state data: %w", err)
	}

	// Check state_type to determine which type to unmarshal
	stateType, _ := data["state_type"].(string)
	switch stateType {
	case StateTypeReAct:
		var state ReActStateImpl
		if err := json.Unmarshal(jsonData, &state); err != nil {
			return nil, fmt.Errorf("unmarshal react state: %w", err)
		}
		// Register in memory
		r.mu.Lock()
		r.states[state.ID()] = &state
		r.mu.Unlock()
		return &state, nil

	case StateTypeMatch:
		var state MatchStateImpl
		if err := json.Unmarshal(jsonData, &state); err != nil {
			return nil, fmt.Errorf("unmarshal match state: %w", err)
		}
		// Register in memory
		r.mu.Lock()
		r.states[state.ID()] = &state
		r.mu.Unlock()
		return &state, nil

	default:
		return nil, fmt.Errorf("unknown state type: %q", stateType)
	}
}

// RestoreAgent restores an agent from its saved state.
func (r *Runtime) RestoreAgent(ctx context.Context, stateID string) (agent.Agent, error) {
	// Load state
	state, err := r.GetState(ctx, stateID)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}
	if state == nil {
		return nil, fmt.Errorf("state %q not found", stateID)
	}

	// Get agent def name from state
	defName := state.AgentDef()
	if defName == "" {
		return nil, fmt.Errorf("state %q has no agent def", stateID)
	}

	// Load agent def
	agentDef, err := r.GetAgentDef(ctx, defName)
	if err != nil {
		return nil, fmt.Errorf("get agent def %q: %w", defName, err)
	}

	// Create agent based on def type
	switch def := agentDef.(type) {
	case *agentcfg.ReActAgent:
		reactState, ok := state.(agent.ReActState)
		if !ok {
			return nil, fmt.Errorf("state type mismatch: expected ReActState, got %T", state)
		}
		return agent.NewReActAgentWithState(ctx, def, r, reactState)

	case *agentcfg.MatchAgent:
		matchState, ok := state.(agent.MatchState)
		if !ok {
			return nil, fmt.Errorf("state type mismatch: expected MatchState, got %T", state)
		}
		return agent.NewMatchAgentWithState(ctx, def, r, matchState)

	default:
		return nil, fmt.Errorf("unknown agent def type: %T", agentDef)
	}
}
