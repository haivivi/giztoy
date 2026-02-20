// Package segmentors provides a multiplexer and implementations for
// conversation segmentation — compressing a sequence of conversation messages
// into a structured segment with entity and relation extraction.
//
// A Segmentor takes raw conversation text and produces:
//   - A segment summary with keywords and entity labels.
//   - Extracted entities (people, topics, places, objects) with attributes.
//   - Discovered relations between entities.
//
// # Usage
//
//	segmentors.Handle("qwen/turbo", segmentors.NewGenX(segmentors.Config{
//	    Generator: "qwen/turbo",
//	}))
//	result, err := segmentors.Process(ctx, "qwen/turbo", segmentors.Input{
//	    Messages: []string{"user: 今天和小明聊了恐龙", "assistant: 小明最喜欢霸王龙"},
//	})
package segmentors

import "context"

// Segmentor compresses conversation messages into a structured segment
// with entity and relation extraction.
type Segmentor interface {
	// Process compresses the input messages into a single segment,
	// extracting entities and relations mentioned in the conversation.
	Process(ctx context.Context, input Input) (*Result, error)

	// Model returns the underlying LLM model identifier.
	Model() string
}

// Input is the input to a [Segmentor].
type Input struct {
	// Messages is the conversation text to compress.
	// Each element is a line of dialogue (e.g., "user: 你好", "assistant: 你好呀").
	// Plain text, no dependency on memory.Message.
	Messages []string `json:"messages"`

	// Schema is an optional hint describing entity types and expected attributes.
	// When provided, the LLM is guided to extract entities matching this schema,
	// but it can still discover entities beyond the schema.
	Schema *Schema `json:"schema,omitempty"`
}

// Result is the output of a [Segmentor].
type Result struct {
	// Segment is the compressed conversation fragment.
	Segment SegmentOutput `json:"segment"`

	// Entities are the entities mentioned or discovered in the conversation.
	Entities []EntityOutput `json:"entities"`

	// Relations are the relations between entities.
	Relations []RelationOutput `json:"relations"`
}

// SegmentOutput is a compressed conversation fragment.
type SegmentOutput struct {
	// Summary is a concise description of what happened in the conversation.
	Summary string `json:"summary"`

	// Keywords are important terms for keyword search.
	Keywords []string `json:"keywords"`

	// Labels are entity labels referenced by this segment (e.g., "person:小明").
	Labels []string `json:"labels"`
}

// EntityOutput is an entity extracted from the conversation.
type EntityOutput struct {
	// Label is the entity identifier with type prefix (e.g., "person:小明", "topic:恐龙").
	Label string `json:"label"`

	// Attrs are the attributes observed for this entity in the conversation.
	Attrs map[string]any `json:"attrs,omitempty"`
}

// RelationOutput is a directed relation between two entities.
type RelationOutput struct {
	// From is the source entity label.
	From string `json:"from"`

	// To is the target entity label.
	To string `json:"to"`

	// RelType is the relation type (e.g., "likes", "sibling", "parent").
	RelType string `json:"rel_type"`
}

// ---------------------------------------------------------------------------
// Schema: optional hint for entity extraction
// ---------------------------------------------------------------------------

// Schema describes the entity types and attributes the segmentor should
// look for. It serves as a hint — the LLM is guided but not restricted.
type Schema struct {
	// EntityTypes maps type prefixes (e.g., "person", "topic", "place")
	// to their expected schema.
	EntityTypes map[string]EntitySchema `json:"entity_types" yaml:"entity_types"`
}

// EntitySchema describes an entity type.
type EntitySchema struct {
	// Desc is a human-readable description of this entity type.
	Desc string `json:"desc" yaml:"desc"`

	// Attrs maps attribute names to their definitions.
	Attrs map[string]AttrDef `json:"attrs,omitempty" yaml:"attrs,omitempty"`
}

// AttrDef describes an entity attribute.
type AttrDef struct {
	// Type is the expected value type (e.g., "string", "int", "[]string", "bool").
	Type string `json:"type" yaml:"type"`

	// Desc is a human-readable description of the attribute.
	Desc string `json:"desc" yaml:"desc"`
}

// ---------------------------------------------------------------------------
// Config: for modelloader registration
// ---------------------------------------------------------------------------

// Config configures a GenX segmentor implementation.
type Config struct {
	// Generator is the pattern of the registered generator to use
	// for LLM calls (e.g., "qwen/turbo"). Must be registered in
	// generators.DefaultMux.
	Generator string `json:"generator" yaml:"generator"`

	// PromptVersion selects the prompt template variant. Default "v1".
	PromptVersion string `json:"prompt_version,omitempty" yaml:"prompt_version,omitempty"`
}
