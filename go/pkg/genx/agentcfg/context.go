package agentcfg

import (
	"encoding/json"
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
	"gopkg.in/yaml.v3"
)

// ContextLayer defines a layer of Model Context.
// Supports two forms:
//   - string: inline prompt text
//   - object: $this, $ref, $env, $mem reference
//
// Example:
//
//	context_layers:
//	  - "You are a helpful assistant"  # inline prompt
//	  - $this: .prompt                 # reference to current agent's field
//	  - $ref: character:elsa           # reference to external resource
//	  - $env: SYSTEM_PROMPT            # reference to environment variable
//	  - $mem: { summary: true }        # inject memory context
//
// Validation:
//   - Must have exactly one of: Prompt, This, Ref, Env, or Mem (mutually exclusive)
type ContextLayer struct {
	// Inline prompt text (simple string form)
	Prompt string `json:"-" yaml:"-" msgpack:"prompt,omitempty"`

	// $this form: reference to current agent's field path
	This string `json:"$this,omitzero" yaml:"$this,omitempty" msgpack:"this,omitempty"`

	// $ref form: reference to external resource
	Ref string `json:"$ref,omitzero" yaml:"$ref,omitempty" msgpack:"ref,omitempty"`

	// $env form: reference to environment variable
	Env string `json:"$env,omitzero" yaml:"$env,omitempty" msgpack:"env,omitempty"`

	// $mem form: inject memory context (summary, query, recent messages)
	Mem *MemoryOptions `json:"$mem,omitzero" yaml:"$mem,omitempty" msgpack:"mem,omitempty"`
}

// MemoryOptions defines options for memory injection in context layers.
type MemoryOptions struct {
	Summary bool `json:"summary,omitzero" yaml:"summary,omitempty"`
	Query   bool `json:"query,omitzero" yaml:"query,omitempty"`
	Recent  int  `json:"recent,omitzero" yaml:"recent,omitempty"`
}

// validate checks if the ContextLayer has exactly one field set.
func (c *ContextLayer) validate() error {
	count := 0
	if c.Prompt != "" {
		count++
	}
	if c.This != "" {
		count++
	}
	if c.Ref != "" {
		count++
	}
	if c.Env != "" {
		count++
	}
	if c.Mem != nil {
		count++
	}
	if count == 0 {
		return fmt.Errorf("context layer: must have one of prompt, $this, $ref, $env, or $mem")
	}
	if count > 1 {
		return fmt.Errorf("context layer: must have only one of prompt, $this, $ref, $env, or $mem")
	}
	return nil
}

// UnmarshalJSON supports both string and object forms.
func (c *ContextLayer) UnmarshalJSON(data []byte) error {
	// Try to parse as string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		c.Prompt = s
		return c.validate()
	}

	// Parse as object
	type contextLayerAlias ContextLayer
	var alias contextLayerAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*c = ContextLayer(alias)
	return c.validate()
}

// UnmarshalYAML supports both string and object forms for YAML.
func (c *ContextLayer) UnmarshalYAML(value *yaml.Node) error {
	// Try to parse as string
	if value.Kind == yaml.ScalarNode {
		c.Prompt = value.Value
		return c.validate()
	}

	// Parse as object
	type contextLayerAlias ContextLayer
	var alias contextLayerAlias
	if err := value.Decode(&alias); err != nil {
		return err
	}
	*c = ContextLayer(alias)
	return c.validate()
}

// UnmarshalMsgpack implements msgpack.Unmarshaler with validation.
func (c *ContextLayer) UnmarshalMsgpack(data []byte) error {
	type Alias ContextLayer
	var alias Alias
	if err := msgpack.Unmarshal(data, &alias); err != nil {
		return err
	}
	*c = ContextLayer(alias)
	return c.validate()
}

// MarshalJSON serializes the ContextLayer.
func (c ContextLayer) MarshalJSON() ([]byte, error) {
	if c.Prompt != "" {
		return json.Marshal(c.Prompt)
	}
	type contextLayerAlias ContextLayer
	return json.Marshal(contextLayerAlias(c))
}
