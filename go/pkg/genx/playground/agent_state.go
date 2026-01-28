package playground

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"sync"

	"github.com/google/uuid"
	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/agent"
	"github.com/haivivi/giztoy/go/pkg/genx/agentcfg"
)

// State type constants for serialization.
const (
	StateTypeReAct = "react"
	StateTypeMatch = "match"
)

// baseStateData is the serializable data of baseState.
type baseStateData struct {
	ID                 string             `json:"id"`
	AgentDef           string             `json:"agent_def,omitzero"`
	ParentStateID      string             `json:"parent_state_id,omitzero"`
	Messages           []agentcfg.Message `json:"messages,omitzero"`
	LastUserMessageIdx int                `json:"last_user_message_idx,omitzero"`
	Summary            string             `json:"summary,omitzero"`
	Properties         map[string]any     `json:"properties,omitzero"`
}

// baseState implements agent.AgentState.
type baseState struct {
	mu   sync.RWMutex
	data baseStateData
}

func newBaseState() *baseState {
	return &baseState{
		data: baseStateData{
			ID:         uuid.New().String(),
			Properties: make(map[string]any),
		},
	}
}

func (s *baseState) ID() string {
	return s.data.ID
}

func (s *baseState) AgentDef() string {
	return s.data.AgentDef
}

func (s *baseState) setAgentDef(name string) {
	s.data.AgentDef = name
}

func (s *baseState) ParentStateID() string {
	return s.data.ParentStateID
}

func (s *baseState) setParentStateID(id string) {
	s.data.ParentStateID = id
}

func (s *baseState) LoadRecent(ctx context.Context) ([]agentcfg.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy of messages
	result := make([]agentcfg.Message, len(s.data.Messages))
	copy(result, s.data.Messages)
	return result, nil
}

func (s *baseState) StoreMessage(ctx context.Context, msg agentcfg.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Mark revert point on user message (before any merge check)
	if msg.Role == "user" && (len(s.data.Messages) == 0 || s.data.Messages[len(s.data.Messages)-1].Role != "user") {
		// Only set revert point for first user message in a sequence
		s.data.LastUserMessageIdx = len(s.data.Messages)
	}

	// Try to merge with previous message (same role)
	if n := len(s.data.Messages); n > 0 && msg.Role == s.data.Messages[n-1].Role {
		last := &s.data.Messages[n-1]
		switch msg.Role {
		case "user":
			// Merge consecutive user messages
			last.Content += "\n" + msg.Content
			return nil
		case "model":
			if msg.ToolCallID == last.ToolCallID {
				if msg.ToolCallID == "" {
					last.Content += msg.Content // text message
				} else {
					last.ToolCallArgs += msg.ToolCallArgs // tool call
				}
				return nil
			}
		}
	}

	s.data.Messages = append(s.data.Messages, msg)
	return nil
}

func (s *baseState) Revert(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.LastUserMessageIdx < len(s.data.Messages) {
		s.data.Messages = s.data.Messages[:s.data.LastUserMessageIdx]
	}
	return nil
}

func (s *baseState) Summary(ctx context.Context) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Summary, nil
}

func (s *baseState) SetSummary(ctx context.Context, summary string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Summary = summary
	return nil
}

func (s *baseState) Query(ctx context.Context, query agentcfg.MemoryQuery) ([]agentcfg.MemorySegment, error) {
	// Simple implementation: no RAG support
	return nil, nil
}

func (s *baseState) BuildMemoryContext(ctx context.Context, opts agentcfg.MemoryOptions) (genx.ModelContext, error) {
	// Return an empty but non-nil ModelContext
	// This allows ReActAgent.buildModelContext to work without panicking
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect recent messages if requested
	var messages []*genx.Message
	if opts.Recent > 0 && len(s.data.Messages) > 0 {
		start := 0
		if len(s.data.Messages) > opts.Recent {
			start = len(s.data.Messages) - opts.Recent
		}
		for _, m := range s.data.Messages[start:] {
			messages = append(messages, convertMessage(&m))
		}
	}

	return &simpleMemoryContext{messages: messages}, nil
}

// convertMessage converts agentcfg.Message to genx.Message.
func convertMessage(m *agentcfg.Message) *genx.Message {
	msg := &genx.Message{
		Role: genx.Role(m.Role),
		Name: m.Name,
	}

	switch m.Role {
	case agentcfg.RoleUser, agentcfg.RoleModel:
		if m.ToolCallID != "" {
			// Tool call message
			msg.Payload = &genx.ToolCall{
				ID: m.ToolCallID,
				FuncCall: &genx.FuncCall{
					Name:      m.ToolCallName,
					Arguments: m.ToolCallArgs,
				},
			}
		} else if m.Content != "" {
			// Text message (Contents is a Payload)
			msg.Payload = genx.Contents{genx.Text(m.Content)}
		}
	case agentcfg.RoleTool:
		// Tool result
		msg.Payload = &genx.ToolResult{
			ID:     m.ToolResultID,
			Result: m.Content,
		}
	}

	return msg
}

// simpleMemoryContext is a simple implementation of genx.ModelContext
// that holds only messages (no prompts, no tools).
type simpleMemoryContext struct {
	messages []*genx.Message
}

func (c *simpleMemoryContext) Prompts() iter.Seq[*genx.Prompt] {
	return func(yield func(*genx.Prompt) bool) {}
}

func (c *simpleMemoryContext) Messages() iter.Seq[*genx.Message] {
	return func(yield func(*genx.Message) bool) {
		for _, m := range c.messages {
			if !yield(m) {
				return
			}
		}
	}
}

func (c *simpleMemoryContext) CoTs() iter.Seq[string] {
	return func(yield func(string) bool) {}
}

func (c *simpleMemoryContext) Tools() iter.Seq[genx.Tool] {
	return func(yield func(genx.Tool) bool) {}
}

func (c *simpleMemoryContext) Params() *genx.ModelParams {
	return nil
}

func (s *baseState) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data.Properties[key]
	return v, ok
}

func (s *baseState) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Properties == nil {
		s.data.Properties = make(map[string]any)
	}
	s.data.Properties[key] = value
}

func (s *baseState) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Properties, key)
}

// ReActStateData is the serializable data of ReActStateImpl.
type ReActStateData struct {
	StateType string `json:"state_type"`
	baseStateData
	Phase       agent.ReActPhase  `json:"phase,omitzero"`
	ToolResults []genx.ToolResult `json:"tool_results,omitzero"`
	Finished    bool              `json:"finished,omitzero"`
}

// ReActStateImpl implements agent.ReActState.
type ReActStateImpl struct {
	base        *baseState
	mu          sync.RWMutex
	phase       agent.ReActPhase
	toolResults []genx.ToolResult
	finished    bool
}

func newReActState(parentStateID string) *ReActStateImpl {
	s := &ReActStateImpl{
		base: newBaseState(),
	}
	s.base.setParentStateID(parentStateID)
	return s
}

// MarshalJSON implements json.Marshaler.
func (s *ReActStateImpl) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	s.base.mu.RLock()
	defer s.mu.RUnlock()
	defer s.base.mu.RUnlock()

	data := ReActStateData{
		StateType:     StateTypeReAct,
		baseStateData: s.base.data,
		Phase:         s.phase,
		ToolResults:   s.toolResults,
		Finished:      s.finished,
	}
	return json.Marshal(data)
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *ReActStateImpl) UnmarshalJSON(b []byte) error {
	var data ReActStateData
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	if data.StateType != StateTypeReAct {
		return fmt.Errorf("state type mismatch: got %q, want %q", data.StateType, StateTypeReAct)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.base == nil {
		s.base = &baseState{}
	}
	s.base.mu.Lock()
	defer s.base.mu.Unlock()

	s.base.data = data.baseStateData
	s.phase = data.Phase
	s.toolResults = data.ToolResults
	s.finished = data.Finished
	return nil
}

// Delegate baseState methods
func (s *ReActStateImpl) ID() string            { return s.base.ID() }
func (s *ReActStateImpl) AgentDef() string      { return s.base.AgentDef() }
func (s *ReActStateImpl) ParentStateID() string { return s.base.ParentStateID() }
func (s *ReActStateImpl) LoadRecent(ctx context.Context) ([]agentcfg.Message, error) {
	return s.base.LoadRecent(ctx)
}
func (s *ReActStateImpl) StoreMessage(ctx context.Context, msg agentcfg.Message) error {
	return s.base.StoreMessage(ctx, msg)
}
func (s *ReActStateImpl) Revert(ctx context.Context) error            { return s.base.Revert(ctx) }
func (s *ReActStateImpl) Summary(ctx context.Context) (string, error) { return s.base.Summary(ctx) }
func (s *ReActStateImpl) SetSummary(ctx context.Context, summary string) error {
	return s.base.SetSummary(ctx, summary)
}
func (s *ReActStateImpl) Query(ctx context.Context, query agentcfg.MemoryQuery) ([]agentcfg.MemorySegment, error) {
	return s.base.Query(ctx, query)
}
func (s *ReActStateImpl) BuildMemoryContext(ctx context.Context, opts agentcfg.MemoryOptions) (genx.ModelContext, error) {
	return s.base.BuildMemoryContext(ctx, opts)
}
func (s *ReActStateImpl) Get(key string) (any, bool) { return s.base.Get(key) }
func (s *ReActStateImpl) Set(key string, value any)  { s.base.Set(key, value) }
func (s *ReActStateImpl) Delete(key string)          { s.base.Delete(key) }

func (s *ReActStateImpl) Phase() agent.ReActPhase {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.phase
}

func (s *ReActStateImpl) SetPhase(phase agent.ReActPhase) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = phase
}

func (s *ReActStateImpl) ToolResults() []genx.ToolResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.toolResults
}

func (s *ReActStateImpl) SetToolResults(results []genx.ToolResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolResults = results
}

func (s *ReActStateImpl) ClearToolResults() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolResults = nil
}

func (s *ReActStateImpl) IsFinished() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.finished
}

func (s *ReActStateImpl) SetFinished(finished bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finished = finished
}

// MatchStateData is the serializable data of MatchStateImpl.
type MatchStateData struct {
	StateType string `json:"state_type"`
	baseStateData
	Phase        agent.MatchAgentPhase `json:"phase,omitzero"`
	Input        string                `json:"input,omitzero"`
	Matches      []agent.MatchedIntent `json:"matches,omitzero"`
	CurrentIndex int                   `json:"current_index,omitzero"`
	Matched      bool                  `json:"matched,omitzero"`
	CallingState *ReActStateData       `json:"calling_state,omitzero"`
}

// MatchStateImpl implements agent.MatchState.
type MatchStateImpl struct {
	base         *baseState
	mu           sync.RWMutex
	phase        agent.MatchAgentPhase
	input        string
	matches      []agent.MatchedIntent
	currentIndex int
	matched      bool
	callingState *ReActStateImpl
}

func newMatchState(parentStateID string) *MatchStateImpl {
	s := &MatchStateImpl{
		base: newBaseState(),
	}
	s.base.setParentStateID(parentStateID)
	return s
}

// MarshalJSON implements json.Marshaler.
func (s *MatchStateImpl) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	s.base.mu.RLock()
	defer s.mu.RUnlock()
	defer s.base.mu.RUnlock()

	data := MatchStateData{
		StateType:     StateTypeMatch,
		baseStateData: s.base.data,
		Phase:         s.phase,
		Input:         s.input,
		Matches:       s.matches,
		CurrentIndex:  s.currentIndex,
		Matched:       s.matched,
	}

	// Serialize calling state if present
	if s.callingState != nil {
		s.callingState.mu.RLock()
		s.callingState.base.mu.RLock()
		callingData := &ReActStateData{
			StateType:     StateTypeReAct,
			baseStateData: s.callingState.base.data,
			Phase:         s.callingState.phase,
			ToolResults:   s.callingState.toolResults,
			Finished:      s.callingState.finished,
		}
		s.callingState.base.mu.RUnlock()
		s.callingState.mu.RUnlock()
		data.CallingState = callingData
	}

	return json.Marshal(data)
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *MatchStateImpl) UnmarshalJSON(b []byte) error {
	var data MatchStateData
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	if data.StateType != StateTypeMatch {
		return fmt.Errorf("state type mismatch: got %q, want %q", data.StateType, StateTypeMatch)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.base == nil {
		s.base = &baseState{}
	}
	s.base.mu.Lock()
	defer s.base.mu.Unlock()

	s.base.data = data.baseStateData
	s.phase = data.Phase
	s.input = data.Input
	s.matches = data.Matches
	s.currentIndex = data.CurrentIndex
	s.matched = data.Matched

	// Deserialize calling state if present
	if data.CallingState != nil {
		s.callingState = &ReActStateImpl{
			base:        &baseState{data: data.CallingState.baseStateData},
			phase:       data.CallingState.Phase,
			toolResults: data.CallingState.ToolResults,
			finished:    data.CallingState.Finished,
		}
	}

	return nil
}

// Delegate baseState methods
func (s *MatchStateImpl) ID() string            { return s.base.ID() }
func (s *MatchStateImpl) AgentDef() string      { return s.base.AgentDef() }
func (s *MatchStateImpl) ParentStateID() string { return s.base.ParentStateID() }
func (s *MatchStateImpl) LoadRecent(ctx context.Context) ([]agentcfg.Message, error) {
	return s.base.LoadRecent(ctx)
}
func (s *MatchStateImpl) StoreMessage(ctx context.Context, msg agentcfg.Message) error {
	return s.base.StoreMessage(ctx, msg)
}
func (s *MatchStateImpl) Revert(ctx context.Context) error            { return s.base.Revert(ctx) }
func (s *MatchStateImpl) Summary(ctx context.Context) (string, error) { return s.base.Summary(ctx) }
func (s *MatchStateImpl) SetSummary(ctx context.Context, summary string) error {
	return s.base.SetSummary(ctx, summary)
}
func (s *MatchStateImpl) Query(ctx context.Context, query agentcfg.MemoryQuery) ([]agentcfg.MemorySegment, error) {
	return s.base.Query(ctx, query)
}
func (s *MatchStateImpl) BuildMemoryContext(ctx context.Context, opts agentcfg.MemoryOptions) (genx.ModelContext, error) {
	return s.base.BuildMemoryContext(ctx, opts)
}
func (s *MatchStateImpl) Get(key string) (any, bool) { return s.base.Get(key) }
func (s *MatchStateImpl) Set(key string, value any)  { s.base.Set(key, value) }
func (s *MatchStateImpl) Delete(key string)          { s.base.Delete(key) }

func (s *MatchStateImpl) Phase() agent.MatchAgentPhase {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.phase
}

func (s *MatchStateImpl) SetPhase(phase agent.MatchAgentPhase) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = phase
}

func (s *MatchStateImpl) Input() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.input
}

func (s *MatchStateImpl) SetInput(input string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.input = input
}

func (s *MatchStateImpl) Matches() []agent.MatchedIntent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.matches
}

func (s *MatchStateImpl) SetMatches(matches []agent.MatchedIntent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matches = matches
}

func (s *MatchStateImpl) CurrentIndex() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentIndex
}

func (s *MatchStateImpl) SetCurrentIndex(idx int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentIndex = idx
}

func (s *MatchStateImpl) Matched() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.matched
}

func (s *MatchStateImpl) SetMatched(matched bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matched = matched
}

func (s *MatchStateImpl) CallingState() agent.ReActState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.callingState == nil {
		return nil
	}
	return s.callingState
}

func (s *MatchStateImpl) SetCallingState(state agent.ReActState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state == nil {
		s.callingState = nil
		return
	}
	// Type assert to our implementation
	if impl, ok := state.(*ReActStateImpl); ok {
		s.callingState = impl
	}
}
