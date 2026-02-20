package cortex

import (
	"fmt"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/kv"
)

// Schema defines validation rules and KV key layout for a kind.
type Schema struct {
	Kind     string
	Required []string
	Optional []string
	KeyFunc  func(fields map[string]any) kv.Key
	ValidateFn func(fields map[string]any) error // additional validation beyond required fields
}

// Validate checks that all required fields are present and non-empty,
// then runs any additional validation.
func (s *Schema) Validate(fields map[string]any) error {
	for _, req := range s.Required {
		val, ok := fields[req]
		if !ok {
			return fmt.Errorf("kind %q: missing required field %q", s.Kind, req)
		}
		if str, isStr := val.(string); isStr && str == "" {
			return fmt.Errorf("kind %q: field %q cannot be empty", s.Kind, req)
		}
	}
	if s.ValidateFn != nil {
		return s.ValidateFn(fields)
	}
	return nil
}

// Key returns the KV key for this document's fields.
func (s *Schema) Key(fields map[string]any) kv.Key {
	return s.KeyFunc(fields)
}

// SchemaRegistry holds all registered schemas.
type SchemaRegistry struct {
	schemas map[string]*Schema
}

// NewSchemaRegistry creates a registry with all built-in schemas.
func NewSchemaRegistry() *SchemaRegistry {
	r := &SchemaRegistry{schemas: make(map[string]*Schema)}
	registerBuiltinSchemas(r)
	return r
}

// Register adds a schema to the registry.
func (r *SchemaRegistry) Register(s *Schema) {
	r.schemas[s.Kind] = s
}

// Get returns the schema for a kind, or nil if not found.
func (r *SchemaRegistry) Get(kind string) *Schema {
	return r.schemas[kind]
}

// Kinds returns all registered kind names sorted.
func (r *SchemaRegistry) Kinds() []string {
	kinds := make([]string, 0, len(r.schemas))
	for k := range r.schemas {
		kinds = append(kinds, k)
	}
	return kinds
}

// KVPrefix returns the top-level KV prefix for a kind.
// For "creds/openai" → "creds", for "genx/generator" → "genx".
func KVPrefix(kind string) string {
	if i := strings.IndexByte(kind, '/'); i >= 0 {
		return kind[:i]
	}
	return kind
}

// validateCredFormat checks that a "cred" field has the format "service:name".
func validateCredFormat(fields map[string]any) error {
	cred, _ := fields["cred"].(string)
	if cred == "" {
		return nil
	}
	parts := strings.SplitN(cred, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("field 'cred' must be in format 'service:name', got %q", cred)
	}
	return nil
}

// validateTemperature checks temperature range 0-2.
func validateTemperature(fields map[string]any) error {
	if v, ok := fields["temperature"]; ok {
		var temp float64
		switch t := v.(type) {
		case float64:
			temp = t
		case int:
			temp = float64(t)
		default:
			return fmt.Errorf("field 'temperature' must be a number")
		}
		if temp < 0 || temp > 2 {
			return fmt.Errorf("field 'temperature' must be between 0 and 2, got %v", temp)
		}
	}
	return nil
}

// validateMaxTokens checks max_tokens > 0.
func validateMaxTokens(fields map[string]any) error {
	if v, ok := fields["max_tokens"]; ok {
		var n int
		switch t := v.(type) {
		case int:
			n = t
		case int64:
			n = int(t)
		case uint64:
			n = int(t)
		case float64:
			n = int(t)
		default:
			return fmt.Errorf("field 'max_tokens' must be a number")
		}
		if n <= 0 {
			return fmt.Errorf("field 'max_tokens' must be positive, got %d", n)
		}
	}
	return nil
}

// chainValidators runs multiple validators in sequence.
func chainValidators(validators ...func(map[string]any) error) func(map[string]any) error {
	return func(fields map[string]any) error {
		for _, v := range validators {
			if err := v(fields); err != nil {
				return err
			}
		}
		return nil
	}
}
