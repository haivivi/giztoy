package agentcfg

import (
	"encoding/json"
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

// Agent is the interface for agent definitions.
type Agent interface {
	AgentName() string
	AgentType() AgentType
}

// AgentBase contains common fields for all agent types.
//
// Validation:
//   - Name: required, non-empty string
//   - Type: validated via AgentType unmarshal
type AgentBase struct {
	Type          AgentType      `json:"type,omitzero" msgpack:"type,omitempty"`
	Name          string         `json:"name" msgpack:"name"`
	Prompt        string         `json:"prompt,omitzero" msgpack:"prompt,omitempty"`
	ContextLayers []ContextLayer `json:"context_layers,omitzero" msgpack:"context_layers,omitempty"`
	Generator     GeneratorRef   `json:"generator,omitzero" msgpack:"generator,omitempty"`
}

// ReActAgent is the definition of a ReAct agent.
//
// Validation:
//   - Inherits AgentBase validation (Name required)
type ReActAgent struct {
	AgentBase `msgpack:",inline"`
	Tools     []ToolRef `json:"tools,omitzero" msgpack:"tools,omitempty"`
}

// AgentName returns the agent name.
func (d *ReActAgent) AgentName() string { return d.Name }

// AgentType returns the agent type.
func (d *ReActAgent) AgentType() AgentType {
	if d.Type == "" {
		return AgentTypeReAct
	}
	return d.Type
}

// validate checks if the ReActAgent fields are valid.
func (d *ReActAgent) validate() error {
	if d.Name == "" {
		return fmt.Errorf("react agent: name is required")
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (d *ReActAgent) UnmarshalJSON(data []byte) error {
	type Alias ReActAgent
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*d = ReActAgent(alias)
	return d.validate()
}

// MatchAgent is the definition of a match/router agent.
//
// Validation:
//   - Inherits AgentBase validation (Name required)
//   - Route: each MatchRoute.Rules must reference valid rule names from Rules
type MatchAgent struct {
	AgentBase `msgpack:",inline"`
	Rules     []RuleRef    `json:"rules,omitzero" msgpack:"rules,omitempty"`
	Route     []MatchRoute `json:"route,omitzero" msgpack:"route,omitempty"`
	Default   *AgentRef    `json:"default,omitzero" msgpack:"default,omitempty"` // Agent to use when no rules match
}

// AgentName returns the agent name.
func (d *MatchAgent) AgentName() string { return d.Name }

// AgentType returns the agent type.
func (d *MatchAgent) AgentType() AgentType { return AgentTypeMatch }

// validate checks if the MatchAgent fields are valid.
func (d *MatchAgent) validate() error {
	if d.Name == "" {
		return fmt.Errorf("match agent: name is required")
	}
	for i, route := range d.Route {
		if err := route.validate(); err != nil {
			return fmt.Errorf("agent %s: route[%d]: %w", d.Name, i, err)
		}
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (d *MatchAgent) UnmarshalJSON(data []byte) error {
	type Alias MatchAgent
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*d = MatchAgent(alias)
	return d.validate()
}

// MatchRoute defines routing from matched rules to an agent.
//
// Validation:
//   - Rules: required, non-empty array of rule names
//   - Agent: required, must have valid $ref or inline agent definition
type MatchRoute struct {
	Rules []string `json:"rules" msgpack:"rules"`
	Agent AgentRef `json:"agent" msgpack:"agent"`
}

// validate checks if the MatchRoute fields are valid.
func (r *MatchRoute) validate() error {
	if len(r.Rules) == 0 {
		return fmt.Errorf("rules is required")
	}
	if r.Agent.IsEmpty() {
		return fmt.Errorf("agent is required")
	}
	return nil
}

// AgentRef is a reference to or inline definition of an Agent.
// Supports either $ref or inline definition.
//
// Validation:
//   - Must have exactly one of: Ref (reference) or Agent (inline definition)
type AgentRef struct {
	// Ref is a reference to existing agent, e.g. "agent:music_player"
	Ref string `json:"$ref,omitzero" msgpack:"ref,omitempty"`
	// Inline definition (interface type, not embedded)
	Agent `json:"-" msgpack:"-"`
}

// UnmarshalJSON implements json.Unmarshaler for AgentRef.
func (a *AgentRef) UnmarshalJSON(data []byte) error {
	// First try to get $ref
	var refOnly struct {
		Ref string `json:"$ref"`
	}
	if err := json.Unmarshal(data, &refOnly); err == nil && refOnly.Ref != "" {
		a.Ref = refOnly.Ref
		return nil
	}

	// Parse as inline Agent using UnmarshalAgent
	def, err := UnmarshalAgent(data)
	if err != nil {
		return err
	}
	a.Agent = def
	return nil
}

// MarshalJSON implements json.Marshaler for AgentRef.
func (a AgentRef) MarshalJSON() ([]byte, error) {
	if a.Ref != "" {
		return json.Marshal(map[string]string{"$ref": a.Ref})
	}
	if a.Agent != nil {
		return json.Marshal(a.Agent)
	}
	return []byte("null"), nil
}

// IsRef returns true if this is a reference (not inline).
func (a *AgentRef) IsRef() bool {
	return a.Ref != ""
}

// IsEmpty returns true if neither ref nor inline definition is set.
func (a *AgentRef) IsEmpty() bool {
	return a.Ref == "" && a.Agent == nil
}

// EncodeMsgpack implements msgpack.CustomEncoder for AgentRef.
func (a AgentRef) EncodeMsgpack(enc *msgpack.Encoder) error {
	type agentMsg struct {
		Ref   string    `msgpack:"ref,omitempty"`
		Type  AgentType `msgpack:"type,omitempty"`
		Agent []byte    `msgpack:"agent,omitempty"`
	}
	m := agentMsg{Ref: a.Ref}
	if a.Agent != nil {
		m.Type = a.Agent.AgentType()
		data, err := msgpack.Marshal(a.Agent)
		if err != nil {
			return err
		}
		m.Agent = data
	}
	return enc.Encode(m)
}

// DecodeMsgpack implements msgpack.CustomDecoder for AgentRef.
func (a *AgentRef) DecodeMsgpack(dec *msgpack.Decoder) error {
	type agentMsg struct {
		Ref   string    `msgpack:"ref,omitempty"`
		Type  AgentType `msgpack:"type,omitempty"`
		Agent []byte    `msgpack:"agent,omitempty"`
	}
	var m agentMsg
	if err := dec.Decode(&m); err != nil {
		return err
	}
	a.Ref = m.Ref
	if len(m.Agent) > 0 {
		var def Agent
		var err error
		switch m.Type {
		case AgentTypeMatch:
			var d MatchAgent
			err = msgpack.Unmarshal(m.Agent, &d)
			def = &d
		case AgentTypeReAct, "":
			var d ReActAgent
			err = msgpack.Unmarshal(m.Agent, &d)
			def = &d
		default:
			return fmt.Errorf("unknown agent type: %q", m.Type)
		}
		if err != nil {
			return err
		}
		a.Agent = def
	}
	return nil
}

// UnmarshalAgent unmarshals JSON data into the appropriate Agent type.
func UnmarshalAgent(data []byte) (Agent, error) {
	// First get the type field
	var typeOnly struct {
		Type AgentType `json:"type"`
	}
	if err := json.Unmarshal(data, &typeOnly); err != nil {
		return nil, fmt.Errorf("parse agent type: %w", err)
	}

	switch typeOnly.Type {
	case AgentTypeMatch:
		var def MatchAgent
		if err := json.Unmarshal(data, &def); err != nil {
			return nil, fmt.Errorf("parse match agent: %w", err)
		}
		return &def, nil
	case AgentTypeReAct, "":
		// Default to ReAct
		var def ReActAgent
		if err := json.Unmarshal(data, &def); err != nil {
			return nil, fmt.Errorf("parse react agent: %w", err)
		}
		return &def, nil
	default:
		return nil, fmt.Errorf("unknown agent type: %q", typeOnly.Type)
	}
}

// AsReActAgent returns the Agent as *ReActAgent if it is one, nil otherwise.
func AsReActAgent(def Agent) *ReActAgent {
	if d, ok := def.(*ReActAgent); ok {
		return d
	}
	return nil
}

// AsMatchAgent returns the Agent as *MatchAgent if it is one, nil otherwise.
func AsMatchAgent(def Agent) *MatchAgent {
	if d, ok := def.(*MatchAgent); ok {
		return d
	}
	return nil
}
