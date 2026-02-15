// Package profilers provides a multiplexer and implementations for
// entity profile management — evolving profile schemas and updating
// structured entity profiles based on extracted metadata from conversations.
//
// A Profiler takes the raw extraction output of a [segmentors.Segmentor],
// the original conversation text, existing schemas, and existing profiles,
// and produces:
//   - Schema evolution: new fields or modifications to the entity schema.
//   - Profile updates: structured attribute values for each entity.
//   - Additional relations discovered during profile analysis.
//
// # Usage
//
//	profilers.Handle("qwen/turbo", profilers.NewGenX(profilers.Config{
//	    Generator: "qwen/turbo",
//	}))
//	result, err := profilers.Process(ctx, "qwen/turbo", profilers.Input{
//	    Messages:  messages,
//	    Extracted: segResult,
//	    Schema:    currentSchema,
//	    Profiles:  existingProfiles,
//	})
package profilers

import (
	"context"

	"github.com/haivivi/giztoy/go/pkg/genx/segmentors"
)

// Profiler evolves entity profile schemas and updates entity profiles
// based on segmentor output and original conversation context.
type Profiler interface {
	// Process analyzes the extracted metadata and original conversation to
	// produce schema changes and profile updates for entities.
	Process(ctx context.Context, input Input) (*Result, error)

	// Model returns the underlying LLM model identifier.
	Model() string
}

// Input is the input to a [Profiler].
type Input struct {
	// Messages is the original conversation text (same as segmentor input).
	// Provided so the profiler can reference the full context when
	// deciding on profile updates and schema changes.
	Messages []string

	// Extracted is the output from a [segmentors.Segmentor].
	// Contains the segment summary, extracted entities, and relations.
	Extracted *segmentors.Result

	// Schema is the current entity type schema.
	// The profiler may propose additions or modifications to this schema.
	// Can be nil if no schema exists yet.
	Schema *segmentors.Schema

	// Profiles contains the current profile data for known entities.
	// Keyed by entity label (e.g., "person:小明") → attribute map.
	// Can be nil for first-time processing.
	Profiles map[string]map[string]any
}

// Result is the output of a [Profiler].
type Result struct {
	// SchemaChanges are proposed modifications to the entity type schema.
	// The caller decides whether to accept these changes.
	SchemaChanges []SchemaChange

	// ProfileUpdates contains updated profile data for entities.
	// Keyed by entity label → attribute map.
	// Values are the new/updated attributes (not a full replacement;
	// the caller should merge with existing profiles).
	ProfileUpdates map[string]map[string]any

	// Relations are additional relations discovered during profile analysis
	// that the segmentor may have missed.
	Relations []segmentors.RelationOutput
}

// SchemaChange proposes a modification to the entity type schema.
type SchemaChange struct {
	// EntityType is the type prefix (e.g., "person", "topic").
	EntityType string `json:"entity_type"`

	// Field is the attribute name being added or modified.
	Field string `json:"field"`

	// Def is the attribute definition.
	Def segmentors.AttrDef `json:"def"`

	// Action is "add" for new fields or "modify" for changes to existing fields.
	Action string `json:"action"`
}

// Config configures a GenX profiler implementation.
type Config struct {
	// Generator is the pattern of the registered generator to use
	// for LLM calls (e.g., "qwen/turbo").
	Generator string `json:"generator" yaml:"generator"`

	// PromptVersion selects the prompt template variant. Default "v1".
	PromptVersion string `json:"prompt_version,omitempty" yaml:"prompt_version,omitempty"`
}
