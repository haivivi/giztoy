package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/haivivi/giztoy/pkg/genx"
	"github.com/haivivi/giztoy/pkg/genx/agentcfg"
	"github.com/haivivi/giztoy/pkg/genx/match"
)

// MatchAgentPhase represents the current phase of the MatchAgent.
type MatchAgentPhase string

const (
	MatchPhaseIdle      MatchAgentPhase = ""         // Waiting for input
	MatchPhaseMatching  MatchAgentPhase = "matching" // Currently matching
	MatchPhaseExecuting MatchAgentPhase = "executing"
)

// roundtrip represents a single round of user interaction.
// It contains the context for cancellation and a channel for streaming results.
type roundtrip struct {
	ctx    context.Context
	cancel context.CancelFunc
	result chan roundtripEvent
	done   chan struct{} // closed when goroutine exits
}

// roundtripEvent is the result of a roundtrip operation.
type roundtripEvent struct {
	event *AgentEvent
	err   error
}

// MatchedIntent represents a single matched intent with its routing info.
type MatchedIntent struct {
	Rule     string         `json:"rule" msgpack:"rule"`
	Args     map[string]any `json:"args,omitzero" msgpack:"args,omitempty"`
	AgentRef string         `json:"agent_ref,omitzero" msgpack:"agent_ref,omitempty"`
	AgentDef agentcfg.Agent `json:"-" msgpack:"-"` // Inline agent def (not serialized)
}

// MatchAgent is an Agent that matches user input against rules and routes to sub-agents.
//
// # Overview
//
// MatchAgent uses an LLM-based intent matching system to classify user input and
// route it to the appropriate sub-agent. It acts as a router/dispatcher that:
//   - Matches user input against a set of rules (intent patterns)
//   - Routes matched intents to configured sub-agents
//   - Returns EOF when no rules match (allows caller to handle fallback)
//
// # Definition
//
// Define a match agent in JSON/YAML:
//
//	{
//	  "type": "match",
//	  "name": "intent_router",
//	  "rules": [
//	    {"$ref": "play_music"},
//	    {"$ref": "weather_query"},
//	    {"name": "greeting", "examples": [["hello"], ["hi"]]}
//	  ],
//	  "route": [
//	    {"rules": ["play_music"], "agent": {"$ref": "music_agent"}},
//	    {"rules": ["weather_query"], "agent": {"$ref": "weather_agent"}},
//	    {"rules": ["greeting"], "agent": {"name": "greeter", "prompt": "Greet warmly"}}
//	  ]
//	}
//
// # Execution Flow
//
//  1. Input: User provides text input via Input()
//  2. Match: LLM matches input against rules to identify intent(s)
//  3. Route: Matched intents are looked up in route map to find target agent
//  4. Execute: Sub-agent is created and receives the input
//  5. Stream: Events from sub-agent are forwarded to caller
//  6. Complete: When sub-agent finishes, next matched intent is processed (if any)
//  7. No Match: If no rules match, returns EOF immediately (caller handles fallback)
//
// # Intent Switching
//
// When a calling agent is active and new input arrives, MatchAgent checks in parallel:
//   - If new input matches a DIFFERENT rule → switch to new agent
//   - If new input matches the SAME rule → continue with current agent
//   - If no match → forward input to current agent
//
// # State Management
//
// MatchAgent maintains state via MatchState interface:
//   - phase: idle/matching/executing
//   - input: accumulated user input
//   - matches: list of matched intents
//   - currentIndex: current intent being executed
//   - callingState: serialized state of current sub-agent
type MatchAgent struct {
	def *agentcfg.MatchAgent
	rt  Runtime

	// mu protects the following fields:
	//   - ctx, cancel (lifecycle management)
	//   - calling (currently executing sub-agent)
	//   - currentRound (roundtrip context/channel)
	//   - closed
	//   - inputReady channel operations
	//
	// Note: 'state' is thread-safe (see MatchState interface) and does NOT require mu.
	// Note: 'matcher' and 'routeMap' are read-only after initialization and do NOT require mu.
	mu     sync.Mutex
	ctx    context.Context    // protected by mu
	cancel context.CancelFunc // protected by mu

	// state is managed by Runtime; thread-safe, no mu needed
	state MatchState

	// Runtime components (read-only after init, no mu needed)
	matcher  *match.Matcher
	routeMap map[string]*agentcfg.MatchRoute // rule name -> route config

	// calling is the currently executing sub-agent; protected by mu
	calling Agent

	// currentRound is the current roundtrip context/channel; protected by mu
	currentRound *roundtrip

	closed bool // protected by mu

	// inputReady signals that Input() has been called after EOF; protected by mu
	inputReady chan struct{}
}

// NewMatchAgent creates a new MatchAgent with a fresh state.
// parentStateID is the ID of the parent agent state (empty for top-level agents).
func NewMatchAgent(ctx context.Context, def *agentcfg.MatchAgent, rt Runtime, parentStateID string) (*MatchAgent, error) {
	// Create state via Runtime with agent def name
	state, err := rt.CreateMatchState(ctx, def.Name, parentStateID)
	if err != nil {
		return nil, fmt.Errorf("create match state: %w", err)
	}
	return NewMatchAgentWithState(ctx, def, rt, state)
}

// NewMatchAgentWithState creates a MatchAgent with an existing state.
// This is used for restoring agents from saved state.
func NewMatchAgentWithState(ctx context.Context, def *agentcfg.MatchAgent, rt Runtime, state MatchState) (*MatchAgent, error) {
	ctx, cancel := context.WithCancel(ctx)

	// Load all rules via runtime
	var rules []*match.Rule
	for _, ruleRef := range def.Rules {
		var rule *match.Rule
		var err error

		if ruleRef.Ref != "" {
			rule, err = rt.GetRule(ctx, ruleRef.Ref)
			if err != nil {
				cancel()
				return nil, fmt.Errorf("load rule %s: %w", ruleRef.Ref, err)
			}
		} else if ruleRef.Rule != nil {
			rule = ruleRef.Rule
		} else {
			continue
		}
		rules = append(rules, rule)
	}

	// Compile matcher
	matcher, err := match.Compile(rules)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("compile matcher: %w", err)
	}

	// Build route map: rule name -> route config
	routeMap := make(map[string]*agentcfg.MatchRoute)
	for i := range def.Route {
		route := &def.Route[i]
		for _, ruleName := range route.Rules {
			routeMap[ruleName] = route
		}
	}

	return &MatchAgent{
		def:        def,
		rt:         rt,
		ctx:        ctx,
		cancel:     cancel,
		state:      state,
		matcher:    matcher,
		routeMap:   routeMap,
		inputReady: make(chan struct{}, 1),
	}, nil
}

// Def returns the Agent definition.
func (a *MatchAgent) Def() agentcfg.Agent {
	return a.def
}

// State returns the current runtime state.
// State returns the agent's state interface (managed by Runtime).
func (a *MatchAgent) State() AgentState {
	return a.state
}

// StateID returns the state ID for persistence.
func (a *MatchAgent) StateID() string {
	if a.state != nil {
		return a.state.ID()
	}
	return ""
}

// tagEvent adds AgentDef and AgentStateID to the event.
// This identifies which agent instance produced the event.
func (a *MatchAgent) tagEvent(evt *AgentEvent) *AgentEvent {
	if evt != nil {
		evt.AgentDef = a.def.AgentName()
		evt.AgentStateID = a.StateID()
	}
	return evt
}

// Input receives user input.
func (a *MatchAgent) Input(contents genx.Contents) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return ErrClosed
	}

	// Extract text from contents
	var text string
	for _, c := range contents {
		if t, ok := c.(genx.Text); ok {
			text = string(t)
			break
		}
	}

	// Cancel previous roundtrip if any
	if a.currentRound != nil {
		a.currentRound.cancel()
		// Wait for goroutine to exit
		<-a.currentRound.done
		a.currentRound = nil
	}

	// Accumulate input (unless we have a calling agent and it's the same round)
	if a.state.Input() != "" && a.calling == nil {
		a.state.SetInput(a.state.Input() + " " + text)
	} else if a.calling == nil {
		a.state.SetInput(text)
	}

	// Create new roundtrip
	ctx, cancel := context.WithCancel(a.ctx)
	a.currentRound = &roundtrip{
		ctx:    ctx,
		cancel: cancel,
		result: make(chan roundtripEvent),
		done:   make(chan struct{}),
	}

	// Start roundtrip goroutine
	go a.runRoundtrip(a.currentRound, text, contents)

	// Signal that input is ready (unblock Next() if waiting)
	select {
	case a.inputReady <- struct{}{}:
	default:
		// Channel already has a signal, don't block
	}

	return nil
}

// Interrupt interrupts current output.
func (a *MatchAgent) Interrupt() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Cancel current roundtrip
	if a.currentRound != nil {
		a.currentRound.cancel()
		// Wait for goroutine to exit
		<-a.currentRound.done
		a.currentRound = nil
	}

	return nil
}

// Next returns the next output chunk.
func (a *MatchAgent) Next() (*AgentEvent, error) {
	for {
		a.mu.Lock()
		round := a.currentRound
		closed := a.closed
		a.mu.Unlock()

		if closed {
			return a.tagEvent(&AgentEvent{Type: EventClosed, Phase: string(MatchPhaseIdle)}), nil
		}

		if round == nil {
			// Block waiting for Input() to be called
			select {
			case <-a.ctx.Done():
				return nil, a.ctx.Err()
			case <-a.inputReady:
				// Input was provided, loop again to get the round
				continue
			}
		}

		// Read from roundtrip channel
		select {
		case <-a.ctx.Done():
			return nil, a.ctx.Err()
		case ev, ok := <-round.result:
			if !ok {
				// Channel closed, roundtrip done
				a.mu.Lock()
				a.currentRound = nil
				phase := string(a.state.Phase())
				a.mu.Unlock()
				return a.tagEvent(&AgentEvent{Type: EventEOF, Phase: phase}), nil
			}
			// Events from roundtrip (including sub-agent events) pass through unchanged
			// - Sub-agent events already have their own AgentDef/AgentStateID
			return ev.event, ev.err
		}
	}
}

// runRoundtrip runs a complete roundtrip: match + calling execution.
func (a *MatchAgent) runRoundtrip(round *roundtrip, inputText string, inputContents genx.Contents) {
	defer close(round.done)
	defer close(round.result)

	// Helper to send event
	send := func(evt *AgentEvent, err error) bool {
		select {
		case <-round.ctx.Done():
			return false
		case round.result <- roundtripEvent{event: evt, err: err}:
			return true
		}
	}

	// If we have a calling agent, check for intent switch in parallel
	if a.hasCalling() {
		a.runRoundtripWithCalling(round, inputText, inputContents, send)
		return
	}

	// No calling agent, perform fresh match
	a.runRoundtripFreshMatch(round, send)
}

// runRoundtripWithCalling handles the case when there's an active calling agent.
func (a *MatchAgent) runRoundtripWithCalling(round *roundtrip, inputText string, inputContents genx.Contents, send func(*AgentEvent, error) bool) {
	// Run match and calling in parallel
	matchDone := make(chan struct {
		switched bool
		err      error
	}, 1)

	go func() {
		switched, err := a.checkIntentSwitch(round.ctx, inputText, inputContents)
		matchDone <- struct {
			switched bool
			err      error
		}{switched, err}
	}()

	// Continue reading from calling agent until match completes or calling needs input
	for {
		calling := a.getCalling()
		if calling == nil {
			// Calling was cleared (e.g., by intent switch)
			return
		}

		evt, err := calling.Next()
		if err != nil {
			send(nil, err)
			return
		}

		// Check if match completed
		select {
		case result := <-matchDone:
			if result.err != nil {
				send(nil, result.err)
				return
			}
			if result.switched {
				// Intent switched, continue with new agent if exists
				if a.hasCalling() {
					a.runCallingLoop(round)
				}
				return
			}
			// No switch, continue with current calling

			// If we got EOF, forward input to calling agent
			if evt.Type == EventEOF {
				a.forwardInputToCalling(inputContents)
				a.runCallingLoop(round)
				return
			}
		default:
			// Match not done yet
		}

		if evt.Type == EventClosed {
			// Clear state (don't advance yet, we need to check match result first)
			a.clearCallingState()
			calling.Close()

			// Wait for match result
			result := <-matchDone
			if result.err != nil {
				send(nil, result.err)
				return
			}
			if result.switched {
				a.runCallingLoop(round)
				return
			}

			// Try to start next agent
			if a.tryAdvanceAndStartNext(send) {
				a.runCallingLoop(round)
			}
			return
		}

		if evt.Type == EventEOF {
			// Wait for match result
			result := <-matchDone
			if result.err != nil {
				send(nil, result.err)
				return
			}
			if result.switched {
				a.runCallingLoop(round)
				return
			}

			// Forward input to calling
			a.forwardInputToCalling(inputContents)
			a.runCallingLoop(round)
			return
		}

		// Pass through event (chunk, tool events, etc.)
		if !send(evt, nil) {
			return
		}
	}
}

// clearCallingState clears the calling agent state without advancing the index.
func (a *MatchAgent) clearCallingState() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calling = nil
	a.state.SetCallingState(nil)
}

// tryAdvanceAndStartNext advances the index and tries to start the next agent.
// Returns true if there's a next agent to run, false otherwise (resets to idle).
func (a *MatchAgent) tryAdvanceAndStartNext(send func(*AgentEvent, error) bool) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.state.SetCurrentIndex(a.state.CurrentIndex() + 1)

	if a.state.CurrentIndex() < len(a.state.Matches()) {
		if err := a.startNextAgent(); err != nil {
			send(nil, err)
			return false
		}
		return true
	}

	// All done - reset to idle
	a.state.SetPhase(MatchPhaseIdle)
	a.state.SetInput("")
	a.state.SetMatches(nil)
	a.state.SetMatched(false)
	a.state.SetCurrentIndex(0)
	return false
}

// runRoundtripFreshMatch handles the case when there's no active calling agent.
func (a *MatchAgent) runRoundtripFreshMatch(round *roundtrip, send func(*AgentEvent, error) bool) {
	// Perform matching
	if err := a.performFreshMatch(round.ctx); err != nil {
		send(nil, err)
		return
	}

	// Check match results and start first agent
	started, eof := a.checkMatchAndStart()
	if eof != nil {
		send(eof, nil)
		return
	}
	if !started {
		return
	}

	// Run calling loop
	a.runCallingLoop(round)
}

// performFreshMatch sets phase to matching and performs the match.
func (a *MatchAgent) performFreshMatch(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.state.SetPhase(MatchPhaseMatching)
	return a.doMatch(ctx)
}

// checkMatchAndStart checks match results and starts the first agent if needed.
// Returns (started bool, eofEvent *AgentEvent).
func (a *MatchAgent) checkMatchAndStart() (started bool, eofEvent *AgentEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// If no matches, return EOF to allow fallback handling by caller
	if len(a.state.Matches()) == 0 {
		a.state.SetMatched(false)
		a.state.SetPhase(MatchPhaseIdle)
		return false, a.tagEvent(&AgentEvent{Type: EventEOF, Phase: string(MatchPhaseIdle)})
	}

	// Start executing first match
	a.state.SetPhase(MatchPhaseExecuting)
	a.state.SetCurrentIndex(0)
	a.state.SetMatched(true)

	if err := a.startNextAgent(); err != nil {
		// Return error via eofEvent is not ideal, but keeps the signature simple
		// The caller should check for non-nil eofEvent first
		return false, nil
	}
	return true, nil
}

// runCallingLoop reads from the calling agent and sends to the roundtrip channel.
func (a *MatchAgent) runCallingLoop(round *roundtrip) {
	send := func(evt *AgentEvent, err error) bool {
		select {
		case <-round.ctx.Done():
			return false
		case round.result <- roundtripEvent{event: evt, err: err}:
			return true
		}
	}

	for {
		select {
		case <-round.ctx.Done():
			return
		default:
		}

		calling := a.getCalling()
		if calling == nil {
			return
		}

		evt, err := calling.Next()
		if err != nil {
			send(nil, err)
			return
		}

		if evt.Type == EventClosed {
			// Clear state and check if there are more matches
			hasMore := a.clearCallingAndAdvance()

			// Always close the finished agent (outside any lock)
			calling.Close()

			if hasMore {
				if err := a.tryStartNextAgent(); err != nil {
					send(nil, err)
					return
				}
				continue
			}

			// All done - reset to idle
			a.resetToIdle()
			return
		}

		if evt.Type == EventEOF {
			// Calling agent needs input
			send(a.tagEvent(&AgentEvent{Type: EventEOF, Phase: string(MatchPhaseExecuting)}), nil)
			return
		}

		// Pass through event (events from sub-agent already have their tags)
		if !send(evt, nil) {
			return
		}
	}
}

// getCalling returns the current calling agent (thread-safe).
func (a *MatchAgent) getCalling() Agent {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calling
}

// GetCalling returns the current calling sub-agent (thread-safe).
// Returns nil if no sub-agent is currently active.
// This is useful for inspecting the call stack during testing.
func (a *MatchAgent) GetCalling() Agent {
	return a.getCalling()
}

// hasCalling returns true if there is an active calling agent (thread-safe).
func (a *MatchAgent) hasCalling() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calling != nil
}

// clearCallingAndAdvance clears the calling state and advances the index.
// Returns true if there are more matches to execute.
func (a *MatchAgent) clearCallingAndAdvance() (hasMore bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calling = nil
	a.state.SetCallingState(nil)
	a.state.SetCurrentIndex(a.state.CurrentIndex() + 1)
	return a.state.CurrentIndex() < len(a.state.Matches())
}

// tryStartNextAgent attempts to start the next agent in the match list.
// Must be called when there are more matches (after clearCallingAndAdvance returns true).
func (a *MatchAgent) tryStartNextAgent() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.startNextAgent()
}

// resetToIdle resets the agent state to idle.
// State methods are thread-safe, no agent lock needed.
func (a *MatchAgent) resetToIdle() {
	a.state.SetPhase(MatchPhaseIdle)
	a.state.SetInput("")
	a.state.SetMatches(nil)
	a.state.SetMatched(false)
	a.state.SetCurrentIndex(0)
}

// forwardInputToCalling forwards input to the calling agent if it exists.
// Needs lock because it accesses a.calling which is agent state.
func (a *MatchAgent) forwardInputToCalling(contents genx.Contents) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.calling != nil {
		a.calling.Input(contents)
	}
}

// doMatch performs the matching using the LLM.
// Note: caller must hold a.mu.
func (a *MatchAgent) doMatch(ctx context.Context) error {
	// Get model name
	if a.def.Generator.IsEmpty() {
		return fmt.Errorf("generator.model is required")
	}
	var model string
	if a.def.Generator.IsRef() {
		return fmt.Errorf("generator $ref not yet supported")
	} else if a.def.Generator.Generator != nil {
		model = a.def.Generator.Generator.Model
	}
	if model == "" {
		return fmt.Errorf("generator.model is required")
	}

	// Build model context with user input
	mcb := &genx.ModelContextBuilder{}
	mcb.UserText("", a.state.Input())
	mc := mcb.Build()

	// Create generator adapter
	gen := &matchAgentGenerator{rt: a.rt, model: model}

	// Release lock during LLM call
	a.mu.Unlock()

	// Execute match - collect all results
	results, err := match.Collect(a.matcher.Match(ctx, model, mc, match.WithGenerator(gen)))

	// Re-acquire lock
	a.mu.Lock()

	if err != nil {
		return fmt.Errorf("match: %w", err)
	}

	// Clear previous matches
	a.state.SetMatches(nil)

	// Convert results to MatchedIntent
	var matches []MatchedIntent
	for _, r := range results {
		if r.Rule == "" {
			continue
		}

		// Convert Args
		args := make(map[string]any)
		for k, v := range r.Args {
			if v.HasValue {
				args[k] = v.Value
			}
		}

		// Look up route
		route := a.routeMap[r.Rule]
		if route == nil {
			continue // No route for this rule
		}

		intent := MatchedIntent{
			Rule: r.Rule,
			Args: args,
		}

		// Store agent reference or inline def
		if route.Agent.Ref != "" {
			intent.AgentRef = route.Agent.Ref
		} else if route.Agent.Agent != nil {
			intent.AgentDef = route.Agent.Agent
		}

		matches = append(matches, intent)
	}
	a.state.SetMatches(matches)

	return nil
}

// startNextAgent starts the next sub-agent in the execution queue.
// Note: caller must hold a.mu.
func (a *MatchAgent) startNextAgent() error {
	matches := a.state.Matches()
	if a.state.CurrentIndex() >= len(matches) {
		return nil
	}

	matched := matches[a.state.CurrentIndex()]

	// Resolve agent definition (use a.ctx, not round.ctx, so calling agent survives roundtrip cancellation)
	agentDef, err := a.resolveAgentDef(a.ctx, matched)
	if err != nil {
		return err
	}

	// Create agent based on type (use a.ctx so calling agent survives roundtrip cancellation)
	// Pass this agent's state ID as parent to build the call stack
	parentStateID := a.StateID()
	switch def := agentDef.(type) {
	case *agentcfg.ReActAgent:
		callingAgent, err := NewReActAgent(a.ctx, def, a.rt, parentStateID)
		if err != nil {
			return fmt.Errorf("create agent %s: %w", matched.Rule, err)
		}

		// Build input text with args
		inputText := a.state.Input()
		if len(matched.Args) > 0 {
			// Include args context in the input for the sub-agent
			inputText = fmt.Sprintf("%s\n(Extracted args: %v)", a.state.Input(), matched.Args)
		}

		if err := callingAgent.Input(genx.Contents{genx.Text(inputText)}); err != nil {
			return fmt.Errorf("input to agent: %w", err)
		}

		a.calling = callingAgent
		return nil

	case *agentcfg.MatchAgent:
		// Nested match agent not supported for now
		return fmt.Errorf("nested match agent not supported")

	default:
		return fmt.Errorf("unknown agent type: %T", agentDef)
	}
}

// resolveAgentDef resolves the agent definition from a matched intent.
func (a *MatchAgent) resolveAgentDef(ctx context.Context, matched MatchedIntent) (agentcfg.Agent, error) {
	if matched.AgentDef != nil {
		return matched.AgentDef, nil
	}

	if matched.AgentRef != "" {
		return a.rt.GetAgentDef(ctx, matched.AgentRef)
	}

	return nil, fmt.Errorf("no agent definition for rule %s", matched.Rule)
}

// checkIntentSwitch checks if input matches a new intent and switches if so.
// Returns true if intent was switched.
func (a *MatchAgent) checkIntentSwitch(ctx context.Context, inputText string, inputContents genx.Contents) (bool, error) {
	// Save current state and get current rule
	oldInput, oldMatches, currentRule := a.saveStateForIntentCheck()

	// Set new input for matching
	a.setInputForMatching(inputText)

	// Perform matching
	if err := a.performMatch(ctx); err != nil {
		a.restoreState(oldInput, oldMatches)
		return false, err
	}

	// Check match result
	return a.handleIntentMatchResult(oldInput, oldMatches, currentRule)
}

// saveStateForIntentCheck saves the current state before intent checking.
// State methods are thread-safe, no agent lock needed for read-only state access.
func (a *MatchAgent) saveStateForIntentCheck() (oldInput string, oldMatches []MatchedIntent, currentRule string) {
	oldInput = a.state.Input()
	oldMatches = a.state.Matches()
	if len(oldMatches) > 0 && a.state.CurrentIndex() < len(oldMatches) {
		currentRule = oldMatches[a.state.CurrentIndex()].Rule
	}
	return
}

// setInputForMatching sets the input for intent matching.
// State methods are thread-safe, no agent lock needed.
func (a *MatchAgent) setInputForMatching(inputText string) {
	a.state.SetInput(inputText)
	a.state.SetMatches(nil)
}

// performMatch performs the LLM matching.
// Needs lock because doMatch modifies agent state and releases/reacquires lock internally.
func (a *MatchAgent) performMatch(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.doMatch(ctx)
}

// restoreState restores the saved state.
// State methods are thread-safe, no agent lock needed.
func (a *MatchAgent) restoreState(oldInput string, oldMatches []MatchedIntent) {
	a.state.SetInput(oldInput)
	a.state.SetMatches(oldMatches)
}

// handleIntentMatchResult handles the result of intent matching.
// Returns true if intent was switched, false otherwise.
func (a *MatchAgent) handleIntentMatchResult(oldInput string, oldMatches []MatchedIntent, currentRule string) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	matches := a.state.Matches()
	if len(matches) == 0 {
		// No match, restore old state
		a.state.SetInput(oldInput)
		a.state.SetMatches(oldMatches)
		return false, nil
	}

	newRule := matches[0].Rule

	// If the new match is the SAME rule as current, don't switch!
	// This allows the current agent to continue handling follow-up inputs.
	if newRule == currentRule && currentRule != "" {
		a.state.SetInput(oldInput)
		a.state.SetMatches(oldMatches)
		return false, nil
	}

	// New intent detected! Switch to it.
	// Close current calling agent (outside lock to avoid deadlock)
	callingToClose := a.calling
	a.calling = nil
	a.state.SetCurrentIndex(0)
	a.state.SetCallingState(nil)

	// Start the new agent
	if err := a.startNextAgent(); err != nil {
		return false, err
	}

	// Close old calling agent after releasing lock (done via defer)
	if callingToClose != nil {
		// Schedule close after lock release
		go callingToClose.CloseWithError(nil)
	}

	return true, nil
}

// Revert reverts the last round of conversation.
func (a *MatchAgent) Revert() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.calling != nil {
		return a.calling.Revert()
	}

	return nil
}

// FormatHistory formats the agent's conversation history as a string.
func (a *MatchAgent) FormatHistory(ctx context.Context) string {
	return formatHistory(ctx, a.state)
}

// Close closes the Agent.
func (a *MatchAgent) Close() error {
	return a.CloseWithError(nil)
}

// CloseWithError closes the Agent with an error.
func (a *MatchAgent) CloseWithError(closeErr error) error {
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

	if a.calling != nil {
		a.calling.CloseWithError(closeErr)
	}

	return nil
}

// matchAgentGenerator wraps Runtime to use a specific model for matching.
type matchAgentGenerator struct {
	rt    Runtime
	model string
}

func (g *matchAgentGenerator) GenerateStream(ctx context.Context, _ string, mctx genx.ModelContext) (genx.Stream, error) {
	return g.rt.GenerateStream(ctx, g.model, mctx)
}

func (g *matchAgentGenerator) Invoke(ctx context.Context, _ string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	return g.rt.Invoke(ctx, g.model, mctx, tool)
}
