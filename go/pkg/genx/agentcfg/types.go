// Package agentcfg provides configuration definitions for agents and tools.
package agentcfg

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/itchyny/gojq"
	"github.com/vmihailenco/msgpack/v5"
	"gopkg.in/yaml.v3"
)

// JSONSchema wraps jsonschema.Schema with efficient msgpack serialization.
// Direct msgpack serialization of jsonschema.Schema produces large output (~2KB for simple schemas)
// because it serializes all zero-value fields. This wrapper converts to/from map[string]any
// via JSON for compact msgpack output (~80 bytes for the same schema).
type JSONSchema struct {
	*jsonschema.Schema
}

// MarshalJSON implements json.Marshaler.
func (s JSONSchema) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Schema)
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *JSONSchema) UnmarshalJSON(data []byte) error {
	s.Schema = &jsonschema.Schema{}
	return json.Unmarshal(data, s.Schema)
}

// MarshalMsgpack implements msgpack.Marshaler with compact output.
func (s JSONSchema) MarshalMsgpack() ([]byte, error) {
	if s.Schema == nil {
		return msgpack.Marshal(nil)
	}
	// Convert to JSON first (uses omitempty), then to any
	// Note: empty jsonschema.Schema marshals to JSON `true`, not `{}`
	jsonData, err := json.Marshal(s.Schema)
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(jsonData, &v); err != nil {
		return nil, err
	}
	return msgpack.Marshal(v)
}

// UnmarshalMsgpack implements msgpack.Unmarshaler.
func (s *JSONSchema) UnmarshalMsgpack(data []byte) error {
	var v any
	if err := msgpack.Unmarshal(data, &v); err != nil {
		return err
	}
	if v == nil {
		s.Schema = nil
		return nil
	}
	// Convert value to JSON, then to Schema
	jsonData, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.Schema = &jsonschema.Schema{}
	return json.Unmarshal(jsonData, s.Schema)
}

// JQExpr wraps a jq expression with pre-parsed query.
// The expression is parsed during deserialization to catch errors early
// and avoid repeated parsing at runtime.
type JQExpr struct {
	Expr  string      // original expression string
	Query *gojq.Query // pre-parsed query (not serialized)
}

// MarshalJSON implements json.Marshaler.
func (e JQExpr) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.Expr)
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *JQExpr) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &e.Expr); err != nil {
		return err
	}
	if e.Expr == "" {
		return nil
	}
	query, err := gojq.Parse(e.Expr)
	if err != nil {
		return fmt.Errorf("invalid jq expression %q: %w", e.Expr, err)
	}
	e.Query = query
	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (e JQExpr) MarshalYAML() (any, error) {
	return e.Expr, nil
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (e *JQExpr) UnmarshalYAML(node *yaml.Node) error {
	if err := node.Decode(&e.Expr); err != nil {
		return err
	}
	if e.Expr == "" {
		return nil
	}
	query, err := gojq.Parse(e.Expr)
	if err != nil {
		return fmt.Errorf("invalid jq expression %q: %w", e.Expr, err)
	}
	e.Query = query
	return nil
}

// EncodeMsgpack implements msgpack.Marshaler.
func (e JQExpr) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeString(e.Expr)
}

// DecodeMsgpack implements msgpack.Unmarshaler.
func (e *JQExpr) DecodeMsgpack(dec *msgpack.Decoder) error {
	expr, err := dec.DecodeString()
	if err != nil {
		return err
	}
	e.Expr = expr
	if expr == "" {
		return nil
	}
	query, err := gojq.Parse(expr)
	if err != nil {
		return fmt.Errorf("invalid jq expression %q: %w", expr, err)
	}
	e.Query = query
	return nil
}

// Run executes the jq query on the input and returns the first result as JSON string.
func (e *JQExpr) Run(input any) (string, error) {
	if e == nil || e.Query == nil {
		return "", nil
	}
	iter := e.Query.Run(input)
	v, ok := iter.Next()
	if !ok {
		return "", fmt.Errorf("jq expression returned no result")
	}
	if err, ok := v.(error); ok {
		return "", fmt.Errorf("jq error: %w", err)
	}
	result, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal jq result: %w", err)
	}
	return string(result), nil
}
