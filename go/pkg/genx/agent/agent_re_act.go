package agent

import (
	"context"
	"fmt"
	"iter"
	"os"
	"sync"

	"github.com/haivivi/giztoy/pkg/genx"
	"github.com/haivivi/giztoy/pkg/genx/agentcfg"
)

var _ Agent = (*ReActAgent)(nil)

// ReActPhase represents the execution phase of a ReAct agent.
type ReActPhase string

const (
	ReActPhaseIdle     ReActPhase = ""         // Waiting for input
	ReActPhaseThinking ReActPhase = "thinking" // LLM is generating
	ReActPhaseTool     ReActPhase = "tool"     // Executing tool
	ReActPhaseAgent    ReActPhase = "agent"    // Calling sub-agent
	ReActPhaseFinished ReActPhase = "finished" // Agent completed
)

// ReActAgent is an Agent implementation based on the ReAct (Reasoning + Acting) pattern.
//
// # Overview
//
// ReActAgent implements the ReAct paradigm where an LLM reasons about a task and takes
// actions through tool calls in an iterative loop:
//
//	Input → Think → Act → Observe → Think → Act → ... → Done
//
// The agent continues the think-act-observe cycle until either:
//   - A quit/finish tool is called (configured via ToolRef.Quit)
//   - The LLM generates a final response without tool calls
//   - The agent is explicitly closed
//
// # Definition
//
// Define a ReAct agent in JSON/YAML:
//
//	{
//	  "type": "react",
//	  "name": "assistant",
//	  "prompt": "You are a helpful assistant.",
//	  "generator": {"model": "gpt-4o"},
//	  "tools": [
//	    {"$ref": "search"},
//	    {"$ref": "calculator"},
//	    {"$ref": "finish", "quit": true}
//	  ]
//	}
//
// Or using context_layers for more complex prompt configuration:
//
//	{
//	  "type": "react",
//	  "name": "assistant",
//	  "context_layers": [
//	    {"$ref": "system_prompt"},
//	    {"$this": ".prompt"},
//	    {"$mem": {"recent": 20, "summary": true}}
//	  ],
//	  "generator": {"model": "gpt-4o"},
//	  "tools": [...]
//	}
//
// # Execution Flow
//
//  1. Input: User provides text input via Input()
//  2. Context: Build ModelContext from prompts, memory, and previous messages
//  3. Generate: Call LLM to generate response (may include tool calls)
//  4. Stream: Emit EventChunk for each text chunk
//  5. Tool Call: If LLM requests tool call:
//     - Emit EventToolStart
//     - Execute tool
//     - Emit EventToolDone or EventToolError
//     - Store tool result in state
//     - Continue generation (go to step 3)
//  6. EOF: When generation ends without tool call, emit EventEOF
//  7. Quit: If quit tool is called, set finished=true and emit EventClosed
//
// # Tool Types
//
// ReActAgent supports various tool types:
//   - HTTP tools: Make HTTP requests
//   - Generator tools: Call LLM for sub-tasks
//   - Composite tools: Chain multiple tools
//   - Built-in tools: Custom Go functions
//
// # Memory Management
//
// Memory is configured via $mem in context_layers:
//
//		{"$mem": {"recent": 20, "summary": true, "query": true}}
//
//	  - recent: Load N most recent messages
//	  - summary: Include conversation summary (if available)
//	  - query: Use RAG to query relevant context
//
// # State Management
//
// ReActAgent maintains state via ReActState interface:
//   - messages: Conversation history (user, model, tool messages)
//   - phase: idle/thinking/tool/finished
//   - toolResults: Map of tool call results
//   - finished: Whether agent completed via quit tool
type ReActAgent struct {
	def *agentcfg.ReActAgent
	rt  Runtime

	// mu protects the following fields:
	//   - ctx, cancel (lifecycle management)
	//   - stream (LLM stream)
	//   - pendingText (accumulated response)
	//   - closed, interrupted, finished
	//   - inputReady channel operations
	//   - pendingToolEvent
	//
	// Note: 'state' is thread-safe (see ReActState interface) and does NOT require mu.
	// Note: 'mcb', 'quitTools', 'memOpts' are read-only after initialization and do NOT require mu.
	mu     sync.Mutex
	ctx    context.Context    // protected by mu
	cancel context.CancelFunc // protected by mu

	// state is managed by Runtime; thread-safe, no mu needed
	state ReActState

	// memOpts is the $mem configuration from context_layers (if any); read-only after init
	memOpts *agentcfg.MemoryOptions

	// mcb holds prompts, tools, cots (but NOT messages - those are in state); read-only after init
	mcb *genx.ModelContextBuilder

	// stream is the current LLM stream; protected by mu
	stream genx.Stream

	// quitTools contains tool names that trigger agent completion; read-only after init
	quitTools map[string]struct{}

	// pendingText is the accumulated model response in current round; protected by mu
	pendingText string

	// --- Lifecycle state (protected by mu) ---

	closed      bool
	interrupted bool

	// finished indicates this agent has completed its task (called quit/finish tool)
	finished bool

	// inputReady signals that Input() has been called after EOF
	inputReady chan struct{}

	// pendingToolEvent holds a tool event to be returned on next Next() call
	pendingToolEvent *AgentEvent
}

// NewReActAgent creates a new ReActAgent with a fresh state.
// parentStateID is the ID of the parent agent state (empty for top-level agents).
func NewReActAgent(ctx context.Context, def *agentcfg.ReActAgent, rt Runtime, parentStateID string) (*ReActAgent, error) {
	// Create state via Runtime with agent def name
	state, err := rt.CreateReActState(ctx, def.Name, parentStateID)
	if err != nil {
		return nil, fmt.Errorf("create react state: %w", err)
	}
	return NewReActAgentWithState(ctx, def, rt, state)
}

// NewReActAgentWithState creates a ReActAgent with an existing state.
// This is used for restoring agents from saved state.
func NewReActAgentWithState(ctx context.Context, def *agentcfg.ReActAgent, rt Runtime, state ReActState) (*ReActAgent, error) {
	ctx, cancel := context.WithCancel(ctx)
	mcb := &genx.ModelContextBuilder{}

	// Extract $mem config from context_layers (if any)
	var memOpts *agentcfg.MemoryOptions
	for _, layer := range def.ContextLayers {
		if layer.Mem != nil {
			memOpts = layer.Mem
			break
		}
	}
	// Default: load recent 100 messages if no $mem specified
	if memOpts == nil {
		memOpts = &agentcfg.MemoryOptions{Recent: 100}
	}

	// Build System Prompt from context layers
	promptMctx, err := buildContextLayers(ctx, def, rt)
	if err != nil {
		cancel()
		return nil, err
	}
	// Copy prompts and CoTs from context layers to mcb
	if promptMctx != nil {
		for p := range promptMctx.Prompts() {
			mcb.Prompts = append(mcb.Prompts, p)
		}
		for cot := range promptMctx.CoTs() {
			mcb.CoTs = append(mcb.CoTs, cot)
		}
	}

	// Load tools from def.Tools to mcb and track quit tools
	quitTools := make(map[string]struct{})
	for _, toolRef := range def.Tools {
		var tool *genx.FuncTool
		var err error
		var toolName string

		if toolRef.IsRef() {
			// $ref - load tool by reference
			toolName = toolRef.Ref
			tool, err = rt.GetTool(ctx, toolRef.Ref)
		} else if toolRef.Tool != nil {
			// Inline tool definition - create tool from ToolDef
			tool, err = rt.CreateToolFromDef(ctx, toolRef.Tool)
			if tool != nil {
				toolName = tool.Name
			}
		} else {
			cancel()
			return nil, fmt.Errorf("tool ref: neither $ref nor inline definition")
		}

		if err != nil {
			cancel()
			return nil, err
		}
		mcb.AddTool(tool)

		// Track quit tools
		if toolRef.Quit {
			quitTools[toolName] = struct{}{}
		}
	}

	return &ReActAgent{
		def:        def,
		rt:         rt,
		ctx:        ctx,
		cancel:     cancel,
		state:      state,
		memOpts:    memOpts,
		mcb:        mcb,
		quitTools:  quitTools,
		inputReady: make(chan struct{}, 1),
	}, nil
}

// buildContextLayers builds System Prompt based on context_layers.
// Returns a combined ModelContext from all layers using MultiModelContext.
func buildContextLayers(ctx context.Context, def *agentcfg.ReActAgent, rt Runtime) (genx.ModelContext, error) {
	// If no context_layers defined, use traditional approach
	if len(def.ContextLayers) == 0 {
		if def.Prompt != "" {
			mcb := &genx.ModelContextBuilder{
				Prompts: []*genx.Prompt{{Name: "system", Text: def.Prompt}},
			}
			return mcb.Build(), nil
		}
		return nil, nil
	}

	// Collect ModelContext from each layer
	layers := make([]genx.ModelContext, 0, len(def.ContextLayers))
	for _, layer := range def.ContextLayers {
		mctx, err := resolveContextLayer(ctx, def, rt, layer)
		if err != nil {
			return nil, err
		}
		if mctx != nil {
			layers = append(layers, mctx)
		}
	}

	if len(layers) == 0 {
		return nil, nil
	}
	if len(layers) == 1 {
		return layers[0], nil
	}
	return genx.ModelContexts(layers...), nil
}

// resolveContextLayer resolves a single context layer, returns genx.ModelContext.
func resolveContextLayer(ctx context.Context, def *agentcfg.ReActAgent, rt Runtime, layer agentcfg.ContextLayer) (genx.ModelContext, error) {
	// Inline prompt text (simple string form)
	if layer.Prompt != "" {
		mcb := &genx.ModelContextBuilder{
			Prompts: []*genx.Prompt{{Name: "inline", Text: layer.Prompt}},
		}
		return mcb.Build(), nil
	}

	// $this form: reference to current agent's field path
	if layer.This != "" {
		path := layer.This
		// Remove leading "."
		if len(path) > 0 && path[0] == '.' {
			path = path[1:]
		}
		text, err := resolveAgentField(def, path)
		if err != nil {
			return nil, err
		}
		if text == "" {
			return nil, nil
		}
		mcb := &genx.ModelContextBuilder{
			Prompts: []*genx.Prompt{{Name: layer.This, Text: text}},
		}
		return mcb.Build(), nil
	}

	// $ref form: reference to external resource
	if layer.Ref != "" {
		cb, err := rt.GetContextBuilder(ctx, layer.Ref)
		if err != nil {
			return nil, err
		}
		return cb.BuildContext(ctx), nil
	}

	// $env form: reference to environment variable
	if layer.Env != "" {
		text := os.Getenv(layer.Env)
		if text == "" {
			return nil, nil
		}
		mcb := &genx.ModelContextBuilder{
			Prompts: []*genx.Prompt{{Name: layer.Env, Text: text}},
		}
		return mcb.Build(), nil
	}

	return nil, nil
}

// resolveAgentField resolves field in agent definition.
func resolveAgentField(def *agentcfg.ReActAgent, field string) (string, error) {
	switch field {
	case "prompt":
		return def.Prompt, nil
	case "name":
		return def.Name, nil
	default:
		// TODO: support deep paths, e.g. generator.model
		return "", nil
	}
}

// Def returns the Agent definition.
func (a *ReActAgent) Def() agentcfg.Agent {
	return a.def
}

// tagEvent adds AgentDef and AgentStateID to the event.
// This identifies which agent instance produced the event.
func (a *ReActAgent) tagEvent(evt *AgentEvent) *AgentEvent {
	if evt != nil {
		evt.AgentDef = a.def.AgentName()
		evt.AgentStateID = a.StateID()
	}
	return evt
}

// IsFinished returns true if this agent has completed its task (called a finish tool).
func (a *ReActAgent) IsFinished() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.finished
}

// buildModelContext builds a ModelContext from state (messages) and mcb (prompts, tools).
// Uses $mem configuration from context_layers to determine what memory to include.
func (a *ReActAgent) buildModelContext() (genx.ModelContext, error) {
	// Use state.BuildMemoryContext to get memory-based context
	memCtx, err := a.state.BuildMemoryContext(a.ctx, *a.memOpts)
	if err != nil {
		return nil, fmt.Errorf("build memory context: %w", err)
	}

	// Collect messages from memory context
	var messages []*genx.Message
	for msg := range memCtx.Messages() {
		messages = append(messages, msg)
	}

	// Collect memory prompts (summary, query results)
	var memPrompts []*genx.Prompt
	for p := range memCtx.Prompts() {
		memPrompts = append(memPrompts, p)
	}

	// Build combined context: base (prompts, tools) + memory prompts + messages
	return &modelContextWithMemory{
		base:       a.mcb.Build(),
		memPrompts: memPrompts,
		messages:   messages,
	}, nil
}

// modelContextWithMemory wraps a base ModelContext with memory-based prompts and messages.
type modelContextWithMemory struct {
	base       genx.ModelContext
	memPrompts []*genx.Prompt
	messages   []*genx.Message
}

func (m *modelContextWithMemory) Prompts() iter.Seq[*genx.Prompt] {
	return func(yield func(*genx.Prompt) bool) {
		// First yield base prompts
		for p := range m.base.Prompts() {
			if !yield(p) {
				return
			}
		}
		// Then yield memory prompts (summary, query results)
		for _, p := range m.memPrompts {
			if !yield(p) {
				return
			}
		}
	}
}

func (m *modelContextWithMemory) Messages() iter.Seq[*genx.Message] {
	return func(yield func(*genx.Message) bool) {
		for _, msg := range m.messages {
			if !yield(msg) {
				return
			}
		}
	}
}

func (m *modelContextWithMemory) CoTs() iter.Seq[string] {
	return m.base.CoTs()
}

func (m *modelContextWithMemory) Tools() iter.Seq[genx.Tool] {
	return m.base.Tools()
}

func (m *modelContextWithMemory) Params() *genx.ModelParams {
	return m.base.Params()
}

// storeUserMessage stores a user message in state.
func (a *ReActAgent) storeUserMessage(contents genx.Contents) error {
	// Extract text content
	var text string
	for _, c := range contents {
		if t, ok := c.(genx.Text); ok {
			text = string(t)
			break
		}
	}

	return a.state.StoreMessage(a.ctx, agentcfg.Message{
		Role:    "user",
		Content: text,
	})
}

// storeModelText stores a model text response in state.
func (a *ReActAgent) storeModelText(text string) error {
	return a.state.StoreMessage(a.ctx, agentcfg.Message{
		Role:    "model",
		Content: text,
	})
}

// storeToolCall stores a tool call in state.
func (a *ReActAgent) storeToolCall(id, name, args string) error {
	return a.state.StoreMessage(a.ctx, agentcfg.Message{
		Role:         "model",
		ToolCallID:   id,
		ToolCallName: name,
		ToolCallArgs: args,
	})
}

// storeToolResult stores a tool result in state.
func (a *ReActAgent) storeToolResult(id, result string) error {
	return a.state.StoreMessage(a.ctx, agentcfg.Message{
		Role:         "tool",
		ToolResultID: id,
		Content:      result,
	})
}

// State returns the agent's state interface (managed by Runtime).
func (a *ReActAgent) State() AgentState {
	return a.state
}

// StateID returns the state ID for persistence.
func (a *ReActAgent) StateID() string {
	if a.state != nil {
		return a.state.ID()
	}
	return ""
}

// Input receives user input.
func (a *ReActAgent) Input(contents genx.Contents) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return ErrClosed
	}

	// Store user message to state
	if err := a.storeUserMessage(contents); err != nil {
		return fmt.Errorf("store user message: %w", err)
	}

	// Build model context and start generation
	model := a.getModel()
	mctx, err := a.buildModelContext()
	if err != nil {
		return err
	}
	stream, err := a.rt.GenerateStream(a.ctx, model, mctx)
	if err != nil {
		return err
	}
	a.stream = stream
	a.pendingText = "" // reset accumulated text

	// Signal that input is ready (unblock Next() if waiting)
	select {
	case a.inputReady <- struct{}{}:
	default:
		// Channel already has a signal, don't block
	}

	return nil
}

// getModel returns the configured model name.
func (a *ReActAgent) getModel() string {
	if !a.def.Generator.IsEmpty() && a.def.Generator.Generator != nil {
		return a.def.Generator.Generator.Model
	}
	return ""
}

// Interrupt interrupts the current output.
func (a *ReActAgent) Interrupt() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.interrupted = true

	if a.stream != nil {
		return a.stream.Close()
	}

	// Signal inputReady to unblock any waiting Next() call
	select {
	case a.inputReady <- struct{}{}:
	default:
	}

	return nil
}

// Revert reverts the last round of conversation.
func (a *ReActAgent) Revert() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return ErrClosed
	}

	// Close current stream
	if a.stream != nil {
		a.stream.Close()
		a.stream = nil
	}

	// Clear pending text
	a.pendingText = ""

	// Delegate to state
	return a.state.Revert(a.ctx)
}

// Next returns the next output chunk.
func (a *ReActAgent) Next() (*AgentEvent, error) {
	// Check terminal states and get pending event
	if evt := a.checkNextState(); evt != nil {
		return evt, nil
	}

	// Get or wait for stream
	stream, err := a.waitForStream()
	if err != nil {
		return nil, err
	}
	if stream == nil {
		return a.tagEvent(&AgentEvent{Type: EventEOF}), nil
	}

	// Process stream output
	return a.nextFromStream(stream)
}

// checkNextState checks terminal states and returns any pending event.
func (a *ReActAgent) checkNextState() *AgentEvent {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check terminal states
	if a.closed && a.finished {
		return a.tagEvent(&AgentEvent{Type: EventClosed})
	}
	if a.interrupted {
		return a.tagEvent(&AgentEvent{Type: EventInterrupted})
	}
	if a.closed {
		return a.tagEvent(&AgentEvent{Type: EventClosed})
	}

	// Check for pending tool event (already tagged)
	if a.pendingToolEvent != nil {
		evt := a.pendingToolEvent
		a.pendingToolEvent = nil
		return evt
	}

	return nil
}

// waitForStream gets the current stream or blocks waiting for input.
func (a *ReActAgent) waitForStream() (genx.Stream, error) {
	a.mu.Lock()
	stream := a.stream
	a.mu.Unlock()

	if stream != nil {
		return stream, nil
	}

	// Block waiting for Input() to be called
	select {
	case <-a.ctx.Done():
		return nil, a.ctx.Err()
	case <-a.inputReady:
		// Input was provided, get stream again
		a.mu.Lock()
		stream = a.stream
		a.mu.Unlock()
		return stream, nil
	}
}

// nextFromStream processes the next chunk from the generation stream.
func (a *ReActAgent) nextFromStream(stream genx.Stream) (*AgentEvent, error) {
	chunk, err := stream.Next()
	if err != nil {
		// Check if it's normal end (Done status)
		if state, ok := err.(*genx.State); ok && state.Status() == genx.StatusDone {
			return a.handleStreamEnd()
		}
		return nil, err
	}

	// Handle tool call
	if chunk.ToolCall != nil {
		return a.handleToolCallEvent(chunk.ToolCall)
	}

	// Accumulate text response
	if chunk.Part != nil {
		if t, ok := chunk.Part.(genx.Text); ok {
			a.mu.Lock()
			a.pendingText += string(t)
			a.mu.Unlock()
		}
	}

	return a.tagEvent(&AgentEvent{Type: EventChunk, Chunk: chunk}), nil
}

// handleStreamEnd handles stream completion.
func (a *ReActAgent) handleStreamEnd() (*AgentEvent, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Store accumulated text to state
	if a.pendingText != "" {
		if err := a.storeModelText(a.pendingText); err != nil {
			return nil, fmt.Errorf("store model text: %w", err)
		}
		a.pendingText = ""
	}
	a.stream = nil

	// Check if agent is finished (quit tool was called)
	if a.finished {
		return a.tagEvent(&AgentEvent{Type: EventClosed}), nil
	}
	return a.tagEvent(&AgentEvent{Type: EventEOF}), nil
}

// handleToolCallEvent handles a tool call from the stream.
func (a *ReActAgent) handleToolCallEvent(tc *genx.ToolCall) (*AgentEvent, error) {
	// Return tool start event first
	startEvt := a.tagEvent(&AgentEvent{
		Type:     EventToolStart,
		ToolCall: tc,
	})

	// Execute tool and prepare result event
	toolErr := a.handleToolCall(tc)

	var resultEvt *AgentEvent
	if toolErr != nil {
		resultEvt = a.tagEvent(&AgentEvent{
			Type:      EventToolError,
			ToolCall:  tc,
			ToolError: toolErr,
		})
	} else {
		resultEvt = a.tagEvent(&AgentEvent{
			Type:     EventToolDone,
			ToolCall: tc,
		})
	}

	// Store result event to return on next Next() call
	a.mu.Lock()
	a.pendingToolEvent = resultEvt
	a.mu.Unlock()

	return startEvt, nil
}

// handleToolCall handles tool call.
func (a *ReActAgent) handleToolCall(tc *genx.ToolCall) error {
	if tc.FuncCall == nil {
		return ErrInvalidToolCall
	}

	toolName := tc.FuncCall.Name
	toolID := tc.ID

	// Store pending text and tool call
	if err := a.storePendingTextAndToolCall(toolID, toolName, tc.FuncCall.Arguments); err != nil {
		return err
	}

	// Get and invoke tool (no lock needed - can be long-running)
	tool, err := a.rt.GetTool(a.ctx, toolName)
	if err != nil {
		// Failed to get tool, store error result
		if storeErr := a.storeToolResultSafe(toolID, "tool error: "+err.Error()); storeErr != nil {
			return fmt.Errorf("store tool error: %w", storeErr)
		}
	} else {
		// Call tool (no lock held - can be long-running)
		result, err := tool.Invoke(a.ctx, tc.FuncCall, tc.FuncCall.Arguments)

		if err != nil {
			if storeErr := a.storeToolResultSafe(toolID, "invoke error: "+err.Error()); storeErr != nil {
				return fmt.Errorf("store invoke error: %w", storeErr)
			}
		} else {
			// Store tool result
			if storeErr := a.storeToolResultSafe(toolID, formatOutput(result)); storeErr != nil {
				return fmt.Errorf("store tool result: %w", storeErr)
			}
		}
	}

	// Check quit and continue generation
	a.checkQuitTool(toolName)
	return a.continueGenerationSafe()
}

// storePendingTextAndToolCall stores any pending text and the tool call.
func (a *ReActAgent) storePendingTextAndToolCall(toolID, toolName, args string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pendingText != "" {
		if err := a.storeModelText(a.pendingText); err != nil {
			return fmt.Errorf("store model text: %w", err)
		}
		a.pendingText = ""
	}
	if err := a.storeToolCall(toolID, toolName, args); err != nil {
		return fmt.Errorf("store tool call: %w", err)
	}
	return nil
}

// storeToolResultSafe stores a tool result with proper locking.
func (a *ReActAgent) storeToolResultSafe(toolID, result string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.storeToolResult(toolID, result)
}

// checkQuitTool checks if the tool is a quit tool and sets the finished flag.
func (a *ReActAgent) checkQuitTool(toolName string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.quitTools[toolName]; ok {
		a.finished = true
	}
}

// continueGenerationSafe continues the generation after a tool call with proper locking.
func (a *ReActAgent) continueGenerationSafe() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.continueGeneration()
}

// continueGeneration builds context and starts a new generation stream.
// Note: caller must hold a.mu lock.
func (a *ReActAgent) continueGeneration() error {
	model := a.getModel()
	mctx, err := a.buildModelContext()
	if err != nil {
		return err
	}
	stream, err := a.rt.GenerateStream(a.ctx, model, mctx)
	if err != nil {
		return err
	}
	a.stream = stream
	return nil
}

// FormatHistory formats the agent's conversation history as a string.
func (a *ReActAgent) FormatHistory(ctx context.Context) string {
	return formatHistory(ctx, a.state)
}

// Close closes the Agent.
func (a *ReActAgent) Close() error {
	return a.CloseWithError(nil)
}

// CloseWithError closes the Agent with an error.
func (a *ReActAgent) CloseWithError(closeErr error) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true
	a.cancel()

	// Destroy state via Runtime
	if a.state != nil {
		// archive=true to preserve history before destruction
		if err := a.rt.DestroyState(a.ctx, a.state.ID(), true); err != nil {
			// Log but don't fail - state cleanup is best effort
			_ = err
		}
	}

	if a.stream != nil {
		if closeErr != nil {
			return a.stream.CloseWithError(closeErr)
		}
		return a.stream.Close()
	}
	return nil
}
