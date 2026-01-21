package agentcfg

import (
	"encoding/json"
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

// Message represents a single message in conversation history.
// It stores both simple text content and tool call/result information.
//
// Role values:
//   - "user": User message
//   - "model": Model response or tool call
//   - "tool": Tool result
//
// Validation:
//   - Role: validated via MessageRole unmarshal
//   - When Role="user": ToolCallID, ToolCallName, ToolCallArgs, ToolResultID must be empty
//   - When Role="model" with tool call: ToolCallID and ToolCallName required; ToolResultID must be empty
//   - When Role="tool": ToolResultID required; ToolCallID, ToolCallName, ToolCallArgs must be empty
type Message struct {
	Role      MessageRole `json:"role" msgpack:"role"`
	Name      string      `json:"name,omitzero" msgpack:"name,omitempty"`
	Content   string      `json:"content,omitzero" msgpack:"content,omitempty"`
	UnixEpoch uint64      `json:"unix_epoch,omitzero" msgpack:"unix_epoch,omitempty"`

	// Tool call fields (populated when Role == "model" and this message represents a tool call)
	ToolCallID   string `json:"tool_call_id,omitzero" msgpack:"tool_call_id,omitempty"`
	ToolCallName string `json:"tool_call_name,omitzero" msgpack:"tool_call_name,omitempty"`
	ToolCallArgs string `json:"tool_call_args,omitzero" msgpack:"tool_call_args,omitempty"`

	// Tool result fields (populated when Role == "tool")
	ToolResultID string `json:"tool_result_id,omitzero" msgpack:"tool_result_id,omitempty"`
}

// validate checks if the Message fields are consistent with the Role.
// Note: Role enum validation is done by MessageRole.UnmarshalJSON/UnmarshalMsgpack.
func (m *Message) validate() error {
	switch m.Role {
	case RoleUser:
		// User messages should not have tool call or tool result fields
		if m.ToolCallID != "" || m.ToolCallName != "" || m.ToolCallArgs != "" {
			return fmt.Errorf("user message should not have tool call fields")
		}
		if m.ToolResultID != "" {
			return fmt.Errorf("user message should not have tool result fields")
		}

	case RoleModel:
		// Model messages can be text or tool call
		// If it's a tool call, ToolCallID and ToolCallName are required
		if m.ToolCallID != "" {
			if m.ToolCallName == "" {
				return fmt.Errorf("model tool call message must have tool_call_name")
			}
		}
		// Model messages should not have tool result fields
		if m.ToolResultID != "" {
			return fmt.Errorf("model message should not have tool result fields")
		}

	case RoleTool:
		// Tool messages must have ToolResultID
		if m.ToolResultID == "" {
			return fmt.Errorf("tool message must have tool_result_id")
		}
		// Tool messages should not have tool call fields
		if m.ToolCallID != "" || m.ToolCallName != "" || m.ToolCallArgs != "" {
			return fmt.Errorf("tool message should not have tool call fields")
		}
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (m *Message) UnmarshalJSON(data []byte) error {
	type Alias Message
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*m = Message(alias)
	return m.validate()
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (m *Message) UnmarshalMsgpack(data []byte) error {
	type Alias Message
	var alias Alias
	if err := msgpack.Unmarshal(data, &alias); err != nil {
		return err
	}
	*m = Message(alias)
	return m.validate()
}

// MemorySegment represents a compressed memory segment.
//
// Validation:
//   - ID: required, non-empty string
//   - Summary: required, non-empty string
//   - UnixEpoch: required, must be > 0
type MemorySegment struct {
	ID        string   `json:"id" msgpack:"id"`
	Summary   string   `json:"summary" msgpack:"summary"`
	Keywords  []string `json:"keywords,omitzero" msgpack:"keywords,omitempty"`
	UnixEpoch uint64   `json:"unix_epoch" msgpack:"unix_epoch"`
}

// validate checks if the MemorySegment fields are valid.
func (s *MemorySegment) validate() error {
	if s.ID == "" {
		return fmt.Errorf("memory segment: id is required")
	}
	if s.Summary == "" {
		return fmt.Errorf("memory segment %s: summary is required", s.ID)
	}
	if s.UnixEpoch == 0 {
		return fmt.Errorf("memory segment %s: unix_epoch is required", s.ID)
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (s *MemorySegment) UnmarshalJSON(data []byte) error {
	type Alias MemorySegment
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*s = MemorySegment(alias)
	return s.validate()
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (s *MemorySegment) UnmarshalMsgpack(data []byte) error {
	type Alias MemorySegment
	var alias Alias
	if err := msgpack.Unmarshal(data, &alias); err != nil {
		return err
	}
	*s = MemorySegment(alias)
	return s.validate()
}

// MemoryQuery defines query parameters for searching memory segments.
//
// Validation:
//   - Month: if set, must be 1-12
//   - Day: if set, must be 1-31
//   - Hour: if set, must be 1-24
type MemoryQuery struct {
	// Text is the search query text for semantic matching.
	Text string `json:"text,omitzero" msgpack:"text,omitempty"`

	// Time scope filters (all optional, can be combined).
	// When multiple fields are set, they form a time range filter.
	// Zero value means not set (no filter on that field).
	Year  int `json:"year,omitzero" msgpack:"year,omitempty"`   // e.g., 2024
	Month int `json:"month,omitzero" msgpack:"month,omitempty"` // 1-12
	Day   int `json:"day,omitzero" msgpack:"day,omitempty"`     // 1-31
	Hour  int `json:"hour,omitzero" msgpack:"hour,omitempty"`   // 1-24 (1=00:00-01:00, 24=23:00-24:00)
}

// validate checks if the MemoryQuery fields are valid.
func (q *MemoryQuery) validate() error {
	if q.Month != 0 && (q.Month < 1 || q.Month > 12) {
		return fmt.Errorf("memory query: month must be 1-12, got %d", q.Month)
	}
	if q.Day != 0 && (q.Day < 1 || q.Day > 31) {
		return fmt.Errorf("memory query: day must be 1-31, got %d", q.Day)
	}
	if q.Hour != 0 && (q.Hour < 1 || q.Hour > 24) {
		return fmt.Errorf("memory query: hour must be 1-24, got %d", q.Hour)
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (q *MemoryQuery) UnmarshalJSON(data []byte) error {
	type Alias MemoryQuery
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*q = MemoryQuery(alias)
	return q.validate()
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (q *MemoryQuery) UnmarshalMsgpack(data []byte) error {
	type Alias MemoryQuery
	var alias Alias
	if err := msgpack.Unmarshal(data, &alias); err != nil {
		return err
	}
	*q = MemoryQuery(alias)
	return q.validate()
}

// ToolResult represents a pending tool result in state.
//
// Validation:
//   - ID: required, non-empty string
//   - Name: required, non-empty string
type ToolResult struct {
	ID      string `json:"id" msgpack:"id"`
	Name    string `json:"name" msgpack:"name"`
	Content string `json:"content" msgpack:"content"`
	IsError bool   `json:"is_error,omitzero" msgpack:"is_error,omitempty"`
}

// validate checks if the ToolResult fields are valid.
func (r *ToolResult) validate() error {
	if r.ID == "" {
		return fmt.Errorf("tool result: id is required")
	}
	if r.Name == "" {
		return fmt.Errorf("tool result %s: name is required", r.ID)
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (r *ToolResult) UnmarshalJSON(data []byte) error {
	type Alias ToolResult
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*r = ToolResult(alias)
	return r.validate()
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (r *ToolResult) UnmarshalMsgpack(data []byte) error {
	type Alias ToolResult
	var alias Alias
	if err := msgpack.Unmarshal(data, &alias); err != nil {
		return err
	}
	*r = ToolResult(alias)
	return r.validate()
}

// MatchedIntent represents a matched intent in MatchAgent state.
//
// Validation:
//   - RuleName: required, non-empty string
//   - AgentName: required, non-empty string
type MatchedIntent struct {
	RuleName  string `json:"rule_name" msgpack:"rule_name"`
	AgentName string `json:"agent_name" msgpack:"agent_name"`
}

// validate checks if the MatchedIntent fields are valid.
func (i *MatchedIntent) validate() error {
	if i.RuleName == "" {
		return fmt.Errorf("matched intent: rule_name is required")
	}
	if i.AgentName == "" {
		return fmt.Errorf("matched intent: agent_name is required")
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (i *MatchedIntent) UnmarshalJSON(data []byte) error {
	type Alias MatchedIntent
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*i = MatchedIntent(alias)
	return i.validate()
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (i *MatchedIntent) UnmarshalMsgpack(data []byte) error {
	type Alias MatchedIntent
	var alias Alias
	if err := msgpack.Unmarshal(data, &alias); err != nil {
		return err
	}
	*i = MatchedIntent(alias)
	return i.validate()
}

// State is the serializable agent state configuration.
// It supports both ReAct and Match agent states.
//
// Validation:
//   - StateType: validated via StateType unmarshal
//   - ID: required, non-empty string
//   - AgentDef: required, non-empty string (agent reference)
type State struct {
	// Common fields
	StateType StateType `json:"state_type" msgpack:"state_type"`
	ID        string    `json:"id" msgpack:"id"`
	AgentDef  string    `json:"agent_def" msgpack:"agent_def"`
	Messages  []Message `json:"messages,omitzero" msgpack:"messages,omitempty"`
	Summary   string    `json:"summary,omitzero" msgpack:"summary,omitempty"`

	// ReAct-specific fields
	Phase       string       `json:"phase,omitzero" msgpack:"phase,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitzero" msgpack:"tool_results,omitempty"`
	Finished    bool         `json:"finished,omitzero" msgpack:"finished,omitempty"`

	// Match-specific fields
	Input        string          `json:"input,omitzero" msgpack:"input,omitempty"`
	Matches      []MatchedIntent `json:"matches,omitzero" msgpack:"matches,omitempty"`
	CurrentIndex int             `json:"current_index,omitzero" msgpack:"current_index,omitempty"`
	Matched      bool            `json:"matched,omitzero" msgpack:"matched,omitempty"`

	// CallingState is the sub-agent's state for Match agents (recursive)
	CallingState *State `json:"calling_state,omitzero" msgpack:"calling_state,omitempty"`

	// Properties is generic key-value storage for agent-specific state
	Properties map[string]any `json:"properties,omitzero" msgpack:"properties,omitempty"`
}

// validate checks if the State fields are valid.
func (s *State) validate() error {
	if s.ID == "" {
		return fmt.Errorf("state: id is required")
	}
	if s.AgentDef == "" {
		return fmt.Errorf("state %s: agent_def is required", s.ID)
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (s *State) UnmarshalJSON(data []byte) error {
	type Alias State
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*s = State(alias)
	return s.validate()
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (s *State) UnmarshalMsgpack(data []byte) error {
	type Alias State
	var alias Alias
	if err := msgpack.Unmarshal(data, &alias); err != nil {
		return err
	}
	*s = State(alias)
	return s.validate()
}

// IsReAct returns true if this is a ReAct state.
func (s *State) IsReAct() bool {
	return s.StateType == StateTypeReAct
}

// IsMatch returns true if this is a Match state.
func (s *State) IsMatch() bool {
	return s.StateType == StateTypeMatch
}
