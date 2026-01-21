package agentcfg

import (
	"encoding/json"
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

// ToolType defines the type of a tool.
type ToolType string

// Tool type constants.
const (
	ToolTypeBuiltIn       ToolType = "built-in"       // default, invoked by runtime
	ToolTypeHTTP          ToolType = "http"           // HTTP request tool
	ToolTypeGenerator     ToolType = "generator"      // single-round LLM generation tool
	ToolTypeComposite     ToolType = "composite"      // sequential tool composition
	ToolTypeTextProcessor ToolType = "text_processor" // text processor tool
)

var validToolTypes = map[string]struct{}{
	string(ToolTypeBuiltIn):       {},
	string(ToolTypeHTTP):          {},
	string(ToolTypeGenerator):     {},
	string(ToolTypeComposite):     {},
	string(ToolTypeTextProcessor): {},
}

// IsValid returns true if the tool type is valid.
func (t ToolType) IsValid() bool {
	if t == "" {
		return true // empty defaults to built-in
	}
	_, ok := validToolTypes[string(t)]
	return ok
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (t *ToolType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	tt := ToolType(s)
	if !tt.IsValid() {
		return fmt.Errorf("invalid tool type: %q", s)
	}
	*t = tt
	return nil
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (t *ToolType) UnmarshalMsgpack(data []byte) error {
	var s string
	if err := msgpack.Unmarshal(data, &s); err != nil {
		return err
	}
	tt := ToolType(s)
	if !tt.IsValid() {
		return fmt.Errorf("invalid tool type: %q", s)
	}
	*t = tt
	return nil
}

// AgentType defines the type of an agent.
type AgentType string

// Agent type constants.
const (
	AgentTypeReAct AgentType = "react" // default, can be omitted
	AgentTypeMatch AgentType = "match" // router/match agent
)

var validAgentTypes = map[string]struct{}{
	string(AgentTypeReAct): {},
	string(AgentTypeMatch): {},
}

// IsValid returns true if the agent type is valid.
func (t AgentType) IsValid() bool {
	if t == "" {
		return true // empty defaults to react
	}
	_, ok := validAgentTypes[string(t)]
	return ok
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (t *AgentType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	at := AgentType(s)
	if !at.IsValid() {
		return fmt.Errorf("invalid agent type: %q", s)
	}
	*t = at
	return nil
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (t *AgentType) UnmarshalMsgpack(data []byte) error {
	var s string
	if err := msgpack.Unmarshal(data, &s); err != nil {
		return err
	}
	at := AgentType(s)
	if !at.IsValid() {
		return fmt.Errorf("invalid agent type: %q", s)
	}
	*t = at
	return nil
}

// MessageRole defines the role of a message.
type MessageRole string

// Message role constants.
const (
	RoleUser  MessageRole = "user"
	RoleModel MessageRole = "model"
	RoleTool  MessageRole = "tool"
)

var validMessageRoles = map[string]struct{}{
	string(RoleUser):  {},
	string(RoleModel): {},
	string(RoleTool):  {},
}

// IsValid returns true if the message role is valid.
func (r MessageRole) IsValid() bool {
	_, ok := validMessageRoles[string(r)]
	return ok
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (r *MessageRole) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	mr := MessageRole(s)
	if !mr.IsValid() {
		return fmt.Errorf("invalid message role: %q (must be %q, %q, or %q)", s, RoleUser, RoleModel, RoleTool)
	}
	*r = mr
	return nil
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (r *MessageRole) UnmarshalMsgpack(data []byte) error {
	var s string
	if err := msgpack.Unmarshal(data, &s); err != nil {
		return err
	}
	mr := MessageRole(s)
	if !mr.IsValid() {
		return fmt.Errorf("invalid message role: %q (must be %q, %q, or %q)", s, RoleUser, RoleModel, RoleTool)
	}
	*r = mr
	return nil
}

// StateType defines the type of an agent state.
type StateType string

// State type constants.
const (
	StateTypeReAct StateType = "react"
	StateTypeMatch StateType = "match"
)

var validStateTypes = map[string]struct{}{
	string(StateTypeReAct): {},
	string(StateTypeMatch): {},
}

// IsValid returns true if the state type is valid.
func (t StateType) IsValid() bool {
	_, ok := validStateTypes[string(t)]
	return ok
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (t *StateType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	st := StateType(s)
	if !st.IsValid() {
		return fmt.Errorf("invalid state type: %q (must be %q or %q)", s, StateTypeReAct, StateTypeMatch)
	}
	*t = st
	return nil
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (t *StateType) UnmarshalMsgpack(data []byte) error {
	var s string
	if err := msgpack.Unmarshal(data, &s); err != nil {
		return err
	}
	st := StateType(s)
	if !st.IsValid() {
		return fmt.Errorf("invalid state type: %q (must be %q or %q)", s, StateTypeReAct, StateTypeMatch)
	}
	*t = st
	return nil
}

// GeneratorMode defines the mode of a generator tool.
type GeneratorMode string

// Generator mode constants.
const (
	GeneratorModeGenerate   GeneratorMode = "generate"    // streaming text generation
	GeneratorModeJSONOutput GeneratorMode = "json_output" // structured JSON output
)

var validGeneratorModes = map[string]struct{}{
	string(GeneratorModeGenerate):   {},
	string(GeneratorModeJSONOutput): {},
}

// IsValid returns true if the generator mode is valid.
func (m GeneratorMode) IsValid() bool {
	if m == "" {
		return true // empty defaults to generate
	}
	_, ok := validGeneratorModes[string(m)]
	return ok
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (m *GeneratorMode) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	gm := GeneratorMode(s)
	if !gm.IsValid() {
		return fmt.Errorf("invalid generator mode: %q (must be %q or %q)", s, GeneratorModeGenerate, GeneratorModeJSONOutput)
	}
	*m = gm
	return nil
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (m *GeneratorMode) UnmarshalMsgpack(data []byte) error {
	var s string
	if err := msgpack.Unmarshal(data, &s); err != nil {
		return err
	}
	gm := GeneratorMode(s)
	if !gm.IsValid() {
		return fmt.Errorf("invalid generator mode: %q (must be %q or %q)", s, GeneratorModeGenerate, GeneratorModeJSONOutput)
	}
	*m = gm
	return nil
}

// TextProcessorOutputMode defines the output mode of a text processor tool.
type TextProcessorOutputMode string

// Text processor output mode constants.
const (
	TextProcessorOutputText TextProcessorOutputMode = "text" // plain text output (default)
	TextProcessorOutputJSON TextProcessorOutputMode = "json" // JSON structured output
)

var validTextProcessorOutputModes = map[string]struct{}{
	string(TextProcessorOutputText): {},
	string(TextProcessorOutputJSON): {},
}

// IsValid returns true if the output mode is valid.
func (m TextProcessorOutputMode) IsValid() bool {
	if m == "" {
		return true // empty defaults to text
	}
	_, ok := validTextProcessorOutputModes[string(m)]
	return ok
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (m *TextProcessorOutputMode) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	om := TextProcessorOutputMode(s)
	if !om.IsValid() {
		return fmt.Errorf("invalid text processor output mode: %q (must be %q or %q)", s, TextProcessorOutputText, TextProcessorOutputJSON)
	}
	*m = om
	return nil
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (m *TextProcessorOutputMode) UnmarshalMsgpack(data []byte) error {
	var s string
	if err := msgpack.Unmarshal(data, &s); err != nil {
		return err
	}
	om := TextProcessorOutputMode(s)
	if !om.IsValid() {
		return fmt.Errorf("invalid text processor output mode: %q (must be %q or %q)", s, TextProcessorOutputText, TextProcessorOutputJSON)
	}
	*m = om
	return nil
}

// HTTPMethod defines the HTTP method for HTTP tools.
type HTTPMethod string

// HTTP method constants (only supported methods).
const (
	HTTPMethodGET    HTTPMethod = "GET"
	HTTPMethodPOST   HTTPMethod = "POST"
	HTTPMethodPUT    HTTPMethod = "PUT"
	HTTPMethodDELETE HTTPMethod = "DELETE"
	HTTPMethodPATCH  HTTPMethod = "PATCH"
)

var validHTTPMethods = map[string]struct{}{
	string(HTTPMethodGET):    {},
	string(HTTPMethodPOST):   {},
	string(HTTPMethodPUT):    {},
	string(HTTPMethodDELETE): {},
	string(HTTPMethodPATCH):  {},
}

// IsValid returns true if the HTTP method is valid.
func (m HTTPMethod) IsValid() bool {
	_, ok := validHTTPMethods[string(m)]
	return ok
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (m *HTTPMethod) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	hm := HTTPMethod(s)
	if !hm.IsValid() {
		return fmt.Errorf("invalid HTTP method: %q (must be GET, POST, PUT, DELETE, or PATCH)", s)
	}
	*m = hm
	return nil
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (m *HTTPMethod) UnmarshalMsgpack(data []byte) error {
	var s string
	if err := msgpack.Unmarshal(data, &s); err != nil {
		return err
	}
	hm := HTTPMethod(s)
	if !hm.IsValid() {
		return fmt.Errorf("invalid HTTP method: %q (must be GET, POST, PUT, DELETE, or PATCH)", s)
	}
	*m = hm
	return nil
}

// HTTPAuthType defines the authentication type for HTTP tools.
type HTTPAuthType string

// HTTP auth type constants.
const (
	HTTPAuthTypeBearer HTTPAuthType = "bearer"
	HTTPAuthTypeBasic  HTTPAuthType = "basic"
)

var validHTTPAuthTypes = map[string]struct{}{
	string(HTTPAuthTypeBearer): {},
	string(HTTPAuthTypeBasic):  {},
}

// IsValid returns true if the HTTP auth type is valid.
func (t HTTPAuthType) IsValid() bool {
	_, ok := validHTTPAuthTypes[string(t)]
	return ok
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (t *HTTPAuthType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	at := HTTPAuthType(s)
	if !at.IsValid() {
		return fmt.Errorf("invalid HTTP auth type: %q (must be %q or %q)", s, HTTPAuthTypeBearer, HTTPAuthTypeBasic)
	}
	*t = at
	return nil
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (t *HTTPAuthType) UnmarshalMsgpack(data []byte) error {
	var s string
	if err := msgpack.Unmarshal(data, &s); err != nil {
		return err
	}
	at := HTTPAuthType(s)
	if !at.IsValid() {
		return fmt.Errorf("invalid HTTP auth type: %q (must be %q or %q)", s, HTTPAuthTypeBearer, HTTPAuthTypeBasic)
	}
	*t = at
	return nil
}

// CompositeMode defines the execution mode of a composite tool.
type CompositeMode string

// Composite mode constants.
const (
	CompositeModeSeq CompositeMode = "seq" // sequential execution (default)
)

var validCompositeModes = map[string]struct{}{
	string(CompositeModeSeq): {},
}

// IsValid returns true if the composite mode is valid.
func (m CompositeMode) IsValid() bool {
	if m == "" {
		return true // empty defaults to seq
	}
	_, ok := validCompositeModes[string(m)]
	return ok
}

// UnmarshalJSON implements json.Unmarshaler with validation.
func (m *CompositeMode) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	cm := CompositeMode(s)
	if !cm.IsValid() {
		return fmt.Errorf("invalid composite mode: %q (must be %q)", s, CompositeModeSeq)
	}
	*m = cm
	return nil
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (m *CompositeMode) UnmarshalMsgpack(data []byte) error {
	var s string
	if err := msgpack.Unmarshal(data, &s); err != nil {
		return err
	}
	cm := CompositeMode(s)
	if !cm.IsValid() {
		return fmt.Errorf("invalid composite mode: %q (must be %q)", s, CompositeModeSeq)
	}
	*m = cm
	return nil
}
