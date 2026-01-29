package luau

import (
	"context"
	"errors"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/agent"
	"github.com/haivivi/giztoy/go/pkg/genx/agentcfg"
	"github.com/haivivi/giztoy/go/pkg/genx/match"
)

// MockRuntime provides a mock implementation of agent.Runtime for testing.
type MockRuntime struct {
	mu sync.Mutex

	// GenerateStreamFunc is called when GenerateStream is invoked.
	GenerateStreamFunc func(ctx context.Context, model string, mctx genx.ModelContext) (genx.Stream, error)

	// InvokeFunc is called when Invoke is invoked.
	InvokeFunc func(ctx context.Context, model string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error)

	// GetToolFunc is called when GetTool is invoked.
	GetToolFunc func(ctx context.Context, name string) (*genx.FuncTool, error)

	// GetToolDefFunc is called when GetToolDef is invoked.
	GetToolDefFunc func(ctx context.Context, name string) (agentcfg.Tool, error)

	// CreateToolFromDefFunc is called when CreateToolFromDef is invoked.
	CreateToolFromDefFunc func(ctx context.Context, def agentcfg.Tool) (*genx.FuncTool, error)

	// GetAgentDefFunc is called when GetAgentDef is invoked.
	GetAgentDefFunc func(ctx context.Context, name string) (agentcfg.Agent, error)

	// GetContextBuilderFunc is called when GetContextBuilder is invoked.
	GetContextBuilderFunc func(ctx context.Context, name string) (agent.ContextBuilder, error)

	// GetRuleFunc is called when GetRule is invoked.
	GetRuleFunc func(ctx context.Context, name string) (*match.Rule, error)

	// States stores created states.
	States map[string]agent.AgentState
}

// NewMockRuntime creates a new MockRuntime with default implementations.
func NewMockRuntime() *MockRuntime {
	return &MockRuntime{
		States: make(map[string]agent.AgentState),
	}
}

// GenerateStream implements agent.Runtime.
func (m *MockRuntime) GenerateStream(ctx context.Context, model string, mctx genx.ModelContext) (genx.Stream, error) {
	if m.GenerateStreamFunc != nil {
		return m.GenerateStreamFunc(ctx, model, mctx)
	}
	return nil, errors.New("GenerateStreamFunc not set")
}

// Invoke implements agent.Runtime.
func (m *MockRuntime) Invoke(ctx context.Context, model string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	if m.InvokeFunc != nil {
		return m.InvokeFunc(ctx, model, mctx, tool)
	}
	return genx.Usage{}, nil, errors.New("InvokeFunc not set")
}

// GetTool implements agent.Runtime.
func (m *MockRuntime) GetTool(ctx context.Context, name string) (*genx.FuncTool, error) {
	if m.GetToolFunc != nil {
		return m.GetToolFunc(ctx, name)
	}
	return nil, errors.New("GetToolFunc not set")
}

// GetToolDef implements agent.Runtime.
func (m *MockRuntime) GetToolDef(ctx context.Context, name string) (agentcfg.Tool, error) {
	if m.GetToolDefFunc != nil {
		return m.GetToolDefFunc(ctx, name)
	}
	return nil, errors.New("GetToolDefFunc not set")
}

// CreateToolFromDef implements agent.Runtime.
func (m *MockRuntime) CreateToolFromDef(ctx context.Context, def agentcfg.Tool) (*genx.FuncTool, error) {
	if m.CreateToolFromDefFunc != nil {
		return m.CreateToolFromDefFunc(ctx, def)
	}
	return nil, errors.New("CreateToolFromDefFunc not set")
}

// GetAgentDef implements agent.Runtime.
func (m *MockRuntime) GetAgentDef(ctx context.Context, name string) (agentcfg.Agent, error) {
	if m.GetAgentDefFunc != nil {
		return m.GetAgentDefFunc(ctx, name)
	}
	return nil, errors.New("GetAgentDefFunc not set")
}

// GetContextBuilder implements agent.Runtime.
func (m *MockRuntime) GetContextBuilder(ctx context.Context, name string) (agent.ContextBuilder, error) {
	if m.GetContextBuilderFunc != nil {
		return m.GetContextBuilderFunc(ctx, name)
	}
	return nil, errors.New("GetContextBuilderFunc not set")
}

// GetRule implements agent.Runtime.
func (m *MockRuntime) GetRule(ctx context.Context, name string) (*match.Rule, error) {
	if m.GetRuleFunc != nil {
		return m.GetRuleFunc(ctx, name)
	}
	return nil, errors.New("GetRuleFunc not set")
}

// CreateReActState implements agent.Runtime.
func (m *MockRuntime) CreateReActState(ctx context.Context, agentDef string, parentStateID string) (agent.ReActState, error) {
	state := NewMockReActState(agentDef, parentStateID)
	m.mu.Lock()
	m.States[state.ID()] = state
	m.mu.Unlock()
	return state, nil
}

// CreateMatchState implements agent.Runtime.
func (m *MockRuntime) CreateMatchState(ctx context.Context, agentDef string, parentStateID string) (agent.MatchState, error) {
	state := NewMockMatchState(agentDef, parentStateID)
	m.mu.Lock()
	m.States[state.ID()] = state
	m.mu.Unlock()
	return state, nil
}

// GetState implements agent.Runtime.
func (m *MockRuntime) GetState(ctx context.Context, id string) (agent.AgentState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.States[id], nil
}

// DestroyState implements agent.Runtime.
func (m *MockRuntime) DestroyState(ctx context.Context, id string, archive bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.States, id)
	return nil
}

// RestoreAgent implements agent.Runtime.
func (m *MockRuntime) RestoreAgent(ctx context.Context, stateID string) (agent.Agent, error) {
	return nil, errors.New("RestoreAgent not implemented in mock")
}

// Ensure MockRuntime implements agent.Runtime.
var _ agent.Runtime = (*MockRuntime)(nil)

// MockState provides a mock implementation of agent.AgentState for testing.
type MockState struct {
	mu            sync.Mutex
	id            string
	agentDef      string
	parentStateID string
	messages      []agentcfg.Message
	summary       string
	properties    map[string]any
}

// NewMockState creates a new MockState.
func NewMockState(agentDef, parentStateID string) *MockState {
	return &MockState{
		id:            generateID(),
		agentDef:      agentDef,
		parentStateID: parentStateID,
		properties:    make(map[string]any),
	}
}

var mockIDCounter uint64
var mockIDMu sync.Mutex

func generateID() string {
	mockIDMu.Lock()
	defer mockIDMu.Unlock()
	mockIDCounter++
	return "mock-state-" + uintToString(mockIDCounter)
}

func uintToString(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// ID implements agent.AgentState.
func (s *MockState) ID() string {
	return s.id
}

// AgentDef implements agent.AgentState.
func (s *MockState) AgentDef() string {
	return s.agentDef
}

// ParentStateID implements agent.AgentState.
func (s *MockState) ParentStateID() string {
	return s.parentStateID
}

// LoadRecent implements agent.AgentState.
func (s *MockState) LoadRecent(ctx context.Context) ([]agentcfg.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]agentcfg.Message, len(s.messages))
	copy(result, s.messages)
	return result, nil
}

// StoreMessage implements agent.AgentState.
func (s *MockState) StoreMessage(ctx context.Context, msg agentcfg.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
	return nil
}

// Revert implements agent.AgentState.
func (s *MockState) Revert(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Find last user message and remove everything from there
	for i := len(s.messages) - 1; i >= 0; i-- {
		if s.messages[i].Role == agentcfg.RoleUser {
			s.messages = s.messages[:i]
			return nil
		}
	}
	return nil
}

// Summary implements agent.AgentState.
func (s *MockState) Summary(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.summary, nil
}

// SetSummary implements agent.AgentState.
func (s *MockState) SetSummary(ctx context.Context, summary string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summary = summary
	return nil
}

// Query implements agent.AgentState.
func (s *MockState) Query(ctx context.Context, query agentcfg.MemoryQuery) ([]agentcfg.MemorySegment, error) {
	// Mock returns empty results
	return nil, nil
}

// BuildMemoryContext implements agent.AgentState.
func (s *MockState) BuildMemoryContext(ctx context.Context, opts agentcfg.MemoryOptions) (genx.ModelContext, error) {
	// Mock returns nil
	return nil, nil
}

// Get implements agent.AgentState.
func (s *MockState) Get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.properties[key]
	return v, ok
}

// Set implements agent.AgentState.
func (s *MockState) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.properties[key] = value
}

// Delete implements agent.AgentState.
func (s *MockState) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.properties, key)
}

// Ensure MockState implements agent.AgentState.
var _ agent.AgentState = (*MockState)(nil)

// MockReActState provides a mock implementation of agent.ReActState for testing.
type MockReActState struct {
	*MockState
	phase       agent.ReActPhase
	toolResults []genx.ToolResult
	finished    bool
}

// NewMockReActState creates a new MockReActState.
func NewMockReActState(agentDef, parentStateID string) *MockReActState {
	return &MockReActState{
		MockState: NewMockState(agentDef, parentStateID),
	}
}

// Phase implements agent.ReActState.
func (s *MockReActState) Phase() agent.ReActPhase {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.phase
}

// SetPhase implements agent.ReActState.
func (s *MockReActState) SetPhase(phase agent.ReActPhase) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = phase
}

// ToolResults implements agent.ReActState.
func (s *MockReActState) ToolResults() []genx.ToolResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]genx.ToolResult, len(s.toolResults))
	copy(result, s.toolResults)
	return result
}

// SetToolResults implements agent.ReActState.
func (s *MockReActState) SetToolResults(results []genx.ToolResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolResults = make([]genx.ToolResult, len(results))
	copy(s.toolResults, results)
}

// ClearToolResults implements agent.ReActState.
func (s *MockReActState) ClearToolResults() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolResults = nil
}

// IsFinished implements agent.ReActState.
func (s *MockReActState) IsFinished() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finished
}

// SetFinished implements agent.ReActState.
func (s *MockReActState) SetFinished(finished bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finished = finished
}

// Ensure MockReActState implements agent.ReActState.
var _ agent.ReActState = (*MockReActState)(nil)

// MockMatchState provides a mock implementation of agent.MatchState for testing.
type MockMatchState struct {
	*MockState
	phase        agent.MatchAgentPhase
	input        string
	matches      []agent.MatchedIntent
	currentIndex int
	matched      bool
	callingState agent.ReActState
}

// NewMockMatchState creates a new MockMatchState.
func NewMockMatchState(agentDef, parentStateID string) *MockMatchState {
	return &MockMatchState{
		MockState: NewMockState(agentDef, parentStateID),
	}
}

// Phase implements agent.MatchState.
func (s *MockMatchState) Phase() agent.MatchAgentPhase {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.phase
}

// SetPhase implements agent.MatchState.
func (s *MockMatchState) SetPhase(phase agent.MatchAgentPhase) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = phase
}

// Input implements agent.MatchState.
func (s *MockMatchState) Input() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.input
}

// SetInput implements agent.MatchState.
func (s *MockMatchState) SetInput(input string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.input = input
}

// Matches implements agent.MatchState.
func (s *MockMatchState) Matches() []agent.MatchedIntent {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]agent.MatchedIntent, len(s.matches))
	copy(result, s.matches)
	return result
}

// SetMatches implements agent.MatchState.
func (s *MockMatchState) SetMatches(matches []agent.MatchedIntent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matches = make([]agent.MatchedIntent, len(matches))
	copy(s.matches, matches)
}

// CurrentIndex implements agent.MatchState.
func (s *MockMatchState) CurrentIndex() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentIndex
}

// SetCurrentIndex implements agent.MatchState.
func (s *MockMatchState) SetCurrentIndex(idx int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentIndex = idx
}

// Matched implements agent.MatchState.
func (s *MockMatchState) Matched() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.matched
}

// SetMatched implements agent.MatchState.
func (s *MockMatchState) SetMatched(matched bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matched = matched
}

// CallingState implements agent.MatchState.
func (s *MockMatchState) CallingState() agent.ReActState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callingState
}

// SetCallingState implements agent.MatchState.
func (s *MockMatchState) SetCallingState(state agent.ReActState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callingState = state
}

// Ensure MockMatchState implements agent.MatchState.
var _ agent.MatchState = (*MockMatchState)(nil)

// MockStream provides a mock implementation of genx.Stream for testing.
type MockStream struct {
	chunks []*genx.MessageChunk
	index  int
	closed bool
	err    error
}

// NewMockStream creates a new MockStream with the given chunks.
func NewMockStream(chunks ...*genx.MessageChunk) *MockStream {
	return &MockStream{chunks: chunks}
}

// NewMockTextStream creates a MockStream that yields text chunks.
func NewMockTextStream(texts ...string) *MockStream {
	chunks := make([]*genx.MessageChunk, len(texts))
	for i, text := range texts {
		chunks[i] = &genx.MessageChunk{
			Role: genx.RoleModel,
			Part: genx.Text(text),
		}
	}
	return NewMockStream(chunks...)
}

// Next implements genx.Stream.
func (s *MockStream) Next() (*genx.MessageChunk, error) {
	if s.closed {
		return nil, genx.ErrDone
	}
	if s.err != nil {
		return nil, s.err
	}
	if s.index >= len(s.chunks) {
		return nil, genx.ErrDone
	}
	chunk := s.chunks[s.index]
	s.index++
	return chunk, nil
}

// Close implements genx.Stream.
func (s *MockStream) Close() error {
	s.closed = true
	return nil
}

// CloseWithError implements genx.Stream.
func (s *MockStream) CloseWithError(err error) error {
	s.closed = true
	s.err = err
	return nil
}

// Ensure MockStream implements genx.Stream.
var _ genx.Stream = (*MockStream)(nil)

// TestLogger is a simple logger for tests that collects log messages.
type TestLogger struct {
	mu       sync.Mutex
	Messages []LogMessage
}

// LogMessage represents a log message.
type LogMessage struct {
	Level string
	Args  []any
}

// Log implements Logger.
func (l *TestLogger) Log(level string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Messages = append(l.Messages, LogMessage{Level: level, Args: args})
}

// Clear clears all collected messages.
func (l *TestLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Messages = nil
}

// Ensure TestLogger implements Logger.
var _ Logger = (*TestLogger)(nil)
