package agentcfg

import (
	"encoding/json"

	"github.com/haivivi/giztoy/pkg/genx/match"
	"gopkg.in/yaml.v3"
)

// Rule is the definition of a matching rule.
// This is a type alias to match.Rule for consistency in the agentcfg package.
type Rule = match.Rule

// RuleRef is a reference to or inline definition of a matching rule.
// Supports $ref to external rule or inline Rule (match.Rule).
//
// Validation:
//   - Must have exactly one of: Ref (reference) or Rule (inline definition)
type RuleRef struct {
	// Ref is a reference to external rule, e.g. "rule:play_music"
	Ref string `json:"$ref,omitzero" msgpack:"ref,omitempty"`
	// Inline rule definition
	*Rule `msgpack:"rule,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler for RuleRef.
func (r *RuleRef) UnmarshalJSON(data []byte) error {
	// First try to get $ref
	var refOnly struct {
		Ref string `json:"$ref"`
	}
	if err := json.Unmarshal(data, &refOnly); err == nil && refOnly.Ref != "" {
		r.Ref = refOnly.Ref
		return nil
	}

	// Parse as inline Rule
	var rule Rule
	if err := json.Unmarshal(data, &rule); err != nil {
		return err
	}
	r.Rule = &rule
	return nil
}

// MarshalJSON implements json.Marshaler for RuleRef.
func (r RuleRef) MarshalJSON() ([]byte, error) {
	if r.Ref != "" {
		return json.Marshal(map[string]string{"$ref": r.Ref})
	}
	if r.Rule != nil {
		return json.Marshal(r.Rule)
	}
	return []byte("null"), nil
}

// IsRef returns true if this is a reference (not inline).
func (r *RuleRef) IsRef() bool {
	return r.Ref != ""
}

// IsEmpty returns true if neither ref nor inline rule is set.
func (r *RuleRef) IsEmpty() bool {
	return r.Ref == "" && r.Rule == nil
}

// UnmarshalYAML implements yaml.Unmarshaler for RuleRef.
func (r *RuleRef) UnmarshalYAML(value *yaml.Node) error {
	// First try to get $ref
	var refOnly struct {
		Ref string `yaml:"$ref"`
	}
	if err := value.Decode(&refOnly); err == nil && refOnly.Ref != "" {
		r.Ref = refOnly.Ref
		return nil
	}

	// Parse as inline Rule
	var rule Rule
	if err := value.Decode(&rule); err != nil {
		return err
	}
	r.Rule = &rule
	return nil
}
