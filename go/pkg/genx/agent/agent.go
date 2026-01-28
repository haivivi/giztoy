package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/agentcfg"
	"github.com/haivivi/giztoy/go/pkg/genx/match"
)

// EventType represents the type of agent event.
type EventType int

const (
	// EventChunk indicates a normal output chunk.
	EventChunk EventType = iota

	// EventEOF indicates the current round of output has ended.
	// The caller should provide new input via Input() and continue calling Next().
	EventEOF

	// EventClosed indicates the agent has completed its task or been closed.
	// This happens when a quit tool is executed or Close() is called.
	EventClosed

	// EventToolStart indicates a tool has started executing.
	EventToolStart

	// EventToolDone indicates a tool has completed successfully.
	EventToolDone

	// EventToolError indicates a tool execution failed.
	EventToolError

	// EventInterrupted indicates the agent was interrupted via Interrupt().
	EventInterrupted
)

// String returns the string representation of the event type.
func (t EventType) String() string {
	switch t {
	case EventChunk:
		return "chunk"
	case EventEOF:
		return "eof"
	case EventClosed:
		return "closed"
	case EventToolStart:
		return "tool_start"
	case EventToolDone:
		return "tool_done"
	case EventToolError:
		return "tool_error"
	case EventInterrupted:
		return "interrupted"
	default:
		return "unknown"
	}
}

// AgentEvent represents an event from Agent.Next().
type AgentEvent struct {
	// Type is the event type.
	Type EventType

	// Phase is the current phase of the agent (e.g., "matching", "executing", "thinking").
	// This helps callers understand what stage the agent is in.
	Phase string

	// AgentDef is the name of the agent definition that produced this event.
	// This is the agent's name as returned by AgentName() (e.g., "my_agent").
	// This is especially useful for identifying the source in calling/sub-agent scenarios.
	AgentDef string

	// AgentStateID is the unique state ID of the agent instance that produced this event.
	// Combined with AgentDef, this uniquely identifies the source agent.
	AgentStateID string

	// Chunk contains the message chunk (for EventChunk).
	Chunk *genx.MessageChunk

	// ToolCall contains the tool call info (for EventToolStart).
	ToolCall *genx.ToolCall

	// ToolResult contains the tool result (for EventToolDone).
	ToolResult *genx.ToolResult

	// ToolError contains the tool execution error (for EventToolError).
	ToolError error
}

// IsTerminal returns true if this event indicates the agent should stop.
func (e *AgentEvent) IsTerminal() bool {
	return e.Type == EventClosed || e.Type == EventInterrupted
}

// Agent is a multi-turn interaction coordinator that completes tasks through conversation.
type Agent interface {
	// Def returns the Agent definition.
	Def() agentcfg.Agent

	// State returns the agent's state interface (managed by Runtime).
	State() AgentState

	// StateID returns the state ID for persistence.
	StateID() string

	// Input receives user input.
	Input(contents genx.Contents) error

	// Interrupt interrupts current output.
	Interrupt() error

	// Next returns the next agent event.
	//
	// Event types:
	//   - EventChunk: Normal output chunk, continue reading.
	//   - EventEOF: Current round ended, call Input() to provide new input.
	//   - EventClosed: Agent completed (quit tool) or closed, stop reading.
	//   - EventToolStart: Tool execution started.
	//   - EventToolDone: Tool execution completed successfully.
	//   - EventToolError: Tool execution failed.
	//   - EventInterrupted: Agent was interrupted via Interrupt().
	//
	// After EventEOF, Next() will block until Input() is called.
	// After EventClosed or EventInterrupted, subsequent Next() calls return the same event.
	Next() (*AgentEvent, error)

	// Revert reverts the last round of conversation (last Input and its response).
	Revert() error

	// FormatHistory formats the agent's conversation history as a string.
	// This is useful for converting a sub-agent's conversation into a tool result.
	FormatHistory(ctx context.Context) string

	// Close closes the Agent.
	Close() error

	// CloseWithError closes the Agent with an error.
	CloseWithError(error) error
}

// ContextBuilder is the interface for Context Layer resources.
// Implementations can be simple PromptDef or richer types like Character (with voice, etc.)
// Returns genx.ModelContext which can be combined using genx.MultiModelContext.
type ContextBuilder interface {
	// BuildContext returns a ModelContext for the context layer.
	// ctx can be used for timeout control, passing user info, APM profiling, etc.
	BuildContext(ctx context.Context) genx.ModelContext
}

// Runtime provides capabilities needed for Agent runtime.
type Runtime interface {
	// Generator capability, routes to specific LLM by model name.
	genx.Generator

	// GetTool gets FuncTool by name.
	GetTool(ctx context.Context, name string) (*genx.FuncTool, error)

	// GetToolDef gets ToolDef by name.
	GetToolDef(ctx context.Context, name string) (agentcfg.Tool, error)

	// CreateToolFromDef creates FuncTool from a ToolDef.
	// Used for inline tool definitions in AgentDef.Tools.
	CreateToolFromDef(ctx context.Context, def agentcfg.Tool) (*genx.FuncTool, error)

	// GetAgentDef gets Agent definition by name.
	GetAgentDef(ctx context.Context, name string) (agentcfg.Agent, error)

	// GetContextBuilder gets Context Layer resource by name (e.g. character, context).
	// Returns ContextBuilder interface, implementations may contain more properties (e.g. voice).
	GetContextBuilder(ctx context.Context, name string) (ContextBuilder, error)

	// GetRule gets a match rule by name (e.g. "rule:play_song").
	GetRule(ctx context.Context, name string) (*match.Rule, error)

	// --- State Management ---

	// CreateReActState creates a new ReActState for a ReAct agent.
	// The state is automatically assigned a unique ID.
	// agentDef is the name of the AgentDef this state belongs to.
	// parentStateID is the ID of the parent agent state (empty for top-level agents).
	CreateReActState(ctx context.Context, agentDef string, parentStateID string) (ReActState, error)

	// CreateMatchState creates a new MatchState for a Match agent.
	// The state is automatically assigned a unique ID.
	// agentDef is the name of the AgentDef this state belongs to.
	// parentStateID is the ID of the parent agent state (empty for top-level agents).
	CreateMatchState(ctx context.Context, agentDef string, parentStateID string) (MatchState, error)

	// GetState retrieves an existing state by ID.
	// Returns nil if not found.
	GetState(ctx context.Context, id string) (AgentState, error)

	// DestroyState destroys a state.
	// If archive is true, the state's memory is compressed and archived before destruction.
	DestroyState(ctx context.Context, id string, archive bool) error

	// --- Agent Restoration ---

	// RestoreAgent restores an agent from its saved state.
	// The state contains the AgentDef name, so the runtime can recreate the agent.
	RestoreAgent(ctx context.Context, stateID string) (Agent, error)
}

// formatHistory formats an agent's conversation history as a string.
// This is useful for converting a sub-agent's conversation into a tool result.
func formatHistory(ctx context.Context, state AgentState) string {
	messages, err := state.LoadRecent(ctx)
	if err != nil {
		return fmt.Sprintf("error loading history: %v", err)
	}

	var sb strings.Builder
	for _, msg := range messages {
		fmt.Fprintf(&sb, "[%s]: %s\n", msg.Role, msg.Content)
	}
	return sb.String()
}
