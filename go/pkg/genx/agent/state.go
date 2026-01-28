package agent

import (
	"context"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/agentcfg"
)

// AgentState is the interface for Agent runtime state management.
// It supports memory management capabilities: recent messages, long-term summary, and RAG queries.
// Different Runtime implementations can use memory-based or database-based storage.
//
// Thread Safety: All methods of AgentState (and its sub-interfaces ReActState, MatchState)
// MUST be thread-safe. Implementations should use internal synchronization (e.g., sync.Mutex)
// to protect concurrent access. This allows agents to call state methods without holding
// their own locks, simplifying the locking strategy.
type AgentState interface {
	// ID returns the unique identifier for this state.
	ID() string

	// AgentDef returns the name of the AgentDef this state belongs to.
	// This is set when the agent is created and used for restoration.
	AgentDef() string

	// ParentStateID returns the ID of the parent agent state (if any).
	// Returns empty string for top-level agents.
	// This forms a call stack that can be traversed to understand agent hierarchies.
	ParentStateID() string

	// --- Memory Management ---

	// LoadRecent loads the most recent messages.
	// The number of messages is determined by the state's configuration (MemoryOptions.Recent).
	LoadRecent(ctx context.Context) ([]agentcfg.Message, error)

	// StoreMessage stores a new message.
	// When storing a user message, state should mark the current position for Revert.
	StoreMessage(ctx context.Context, msg agentcfg.Message) error

	// Revert undoes the last round of conversation.
	// Removes all messages since the last user message (inclusive).
	Revert(ctx context.Context) error

	// Summary returns the long-term compressed summary.
	Summary(ctx context.Context) (string, error)

	// SetSummary updates the long-term summary.
	SetSummary(ctx context.Context, summary string) error

	// Query searches for relevant memory segments using RAG.
	// The runtime controls the result limit automatically.
	Query(ctx context.Context, query agentcfg.MemoryQuery) ([]agentcfg.MemorySegment, error)

	// BuildMemoryContext builds a ModelContext containing memory data.
	// - summary: injects long-term summary as a prompt
	// - query: injects RAG results as a prompt (auto-extracts keywords from recent messages)
	// - recent: injects recent N messages as Messages
	BuildMemoryContext(ctx context.Context, opts agentcfg.MemoryOptions) (genx.ModelContext, error)

	// --- State Properties ---
	// Generic key-value storage for agent-specific state.

	// Get retrieves a state property.
	Get(key string) (any, bool)

	// Set stores a state property.
	Set(key string, value any)

	// Delete removes a state property.
	Delete(key string)
}

// ReActState is the interface for ReAct agent state.
// It extends AgentState with ReAct-specific properties.
//
// Thread Safety: All methods MUST be thread-safe (see AgentState).
type ReActState interface {
	AgentState

	// Phase returns the current execution phase.
	Phase() ReActPhase

	// SetPhase sets the execution phase.
	SetPhase(phase ReActPhase)

	// ToolResults returns pending tool results.
	ToolResults() []genx.ToolResult

	// SetToolResults sets pending tool results.
	SetToolResults(results []genx.ToolResult)

	// ClearToolResults clears pending tool results.
	ClearToolResults()

	// IsFinished returns whether the agent has finished (called quit tool).
	IsFinished() bool

	// SetFinished marks the agent as finished.
	SetFinished(finished bool)
}

// MatchState is the interface for MatchAgent state.
// It extends AgentState with MatchAgent-specific properties.
//
// Thread Safety: All methods MUST be thread-safe (see AgentState).
type MatchState interface {
	AgentState

	// Phase returns the current execution phase.
	Phase() MatchAgentPhase

	// SetPhase sets the execution phase.
	SetPhase(phase MatchAgentPhase)

	// Input returns accumulated user input.
	Input() string

	// SetInput sets user input.
	SetInput(input string)

	// Matches returns the matched intents.
	Matches() []MatchedIntent

	// SetMatches sets matched intents.
	SetMatches(matches []MatchedIntent)

	// CurrentIndex returns the current executing match index.
	CurrentIndex() int

	// SetCurrentIndex sets the current index.
	SetCurrentIndex(idx int)

	// Matched returns whether matching has completed.
	Matched() bool

	// SetMatched sets the matched flag.
	SetMatched(matched bool)

	// CallingState returns the sub-agent's state (if any).
	CallingState() ReActState

	// SetCallingState sets the sub-agent's state.
	SetCallingState(state ReActState)
}
