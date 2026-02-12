// Package memory provides a personal memory system for AI personas.
//
// Each persona (virtual character) has an isolated [Memory] instance containing
// an entity-relation graph, time-stamped segments (compressed memory fragments),
// multi-level time summaries, and active conversations. The [Host] manages many
// Memory instances sharing a single KV store, embedding service, and vector
// index.
//
// # Architecture
//
// Memory sits above the [recall] package and adds persona-level semantics:
//
//   - Atom isolation: each persona gets its own [recall.Index] scoped by a
//     KV key prefix ("mem:{id}").
//   - Conversations: short-term message storage per device/session.
//   - Long-term summaries: multi-granularity time compression (hour → life).
//   - Recall: combined graph expansion + segment search for context building.
//
// The package does not embed compression logic. An upper-layer [Compressor]
// (provided by the agent runtime) drives message compression into segments
// and entity extraction into the graph.
//
// # Dependency Direction
//
//	memory → recall → kv, embed, graph, vecstore
//
// memory does not depend on genx or voiceprint. It consumes voice labels
// as plain strings (e.g., "voice:A3F8") through the graph.
//
// # Separator and Labels
//
// Entity labels and segment labels are used as KV key segments. Labels
// must NOT contain the KV separator character. The default separator is
// ':' (kv.DefaultSeparator), which conflicts with natural labels like
// "person:小明".
//
// To use colon-namespaced labels, configure [HostConfig.Separator] to a
// non-printable byte such as 0x1F (ASCII Unit Separator) and create the
// KV store with the same separator:
//
//	store := kv.NewBadger(dir, &kv.Options{Separator: 0x1F})
//	host := memory.NewHost(memory.HostConfig{
//	    Store:     store,
//	    Separator: 0x1F,
//	})
//
// Then labels like "person:小明", "voice:A3F8" work naturally.
package memory

import (
	"context"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------------------------
// Message: conversation message
// ---------------------------------------------------------------------------

// Role identifies who produced a message.
type Role string

const (
	RoleUser  Role = "user"
	RoleModel Role = "model"
	RoleTool  Role = "tool"
)

// Message is a single conversation turn stored in short-term memory.
type Message struct {
	Role    Role   `json:"role" msgpack:"role"`
	Name    string `json:"name,omitempty" msgpack:"name,omitempty"`
	Content string `json:"content,omitempty" msgpack:"content,omitempty"`

	// Timestamp is the Unix timestamp in nanoseconds when this message
	// was created.
	Timestamp int64 `json:"ts" msgpack:"ts"`

	// Tool call fields (only used when Role == RoleTool).
	ToolCallID   string `json:"tc_id,omitempty" msgpack:"tc_id,omitempty"`
	ToolCallName string `json:"tc_name,omitempty" msgpack:"tc_name,omitempty"`
	ToolCallArgs string `json:"tc_args,omitempty" msgpack:"tc_args,omitempty"`
	ToolResultID string `json:"tr_id,omitempty" msgpack:"tr_id,omitempty"`
}

// ---------------------------------------------------------------------------
// LongTerm types: multi-level time compression
// ---------------------------------------------------------------------------

// Grain represents the time granularity for long-term summaries.
type Grain int

const (
	GrainHour Grain = iota
	GrainDay
	GrainWeek
	GrainMonth
	GrainYear
	GrainLife
)

// String returns a human-readable name for the grain.
func (g Grain) String() string {
	switch g {
	case GrainHour:
		return "hour"
	case GrainDay:
		return "day"
	case GrainWeek:
		return "week"
	case GrainMonth:
		return "month"
	case GrainYear:
		return "year"
	case GrainLife:
		return "life"
	default:
		return "unknown"
	}
}

// TimedSummary is a summary at a specific time granularity.
type TimedSummary struct {
	Grain   Grain    `json:"grain" msgpack:"grain"`
	Time    int64    `json:"time" msgpack:"time"` // Unix nanoseconds
	Labels  []string `json:"labels,omitempty" msgpack:"labels,omitempty"`
	Summary string   `json:"summary" msgpack:"summary"`
}

// ---------------------------------------------------------------------------
// Recall types: combined search query and result
// ---------------------------------------------------------------------------

// RecallQuery specifies parameters for [Memory.Recall].
type RecallQuery struct {
	// Labels are seed entity labels for the current context
	// (e.g., "person:Alice"). Graph expansion discovers related entities.
	Labels []string

	// Text is the current topic or question for semantic + keyword search.
	Text string

	// Hops controls graph traversal depth from seed labels. Default 2.
	Hops int

	// Limit is the maximum number of segments to return. Default 10.
	Limit int
}

// RecallResult holds the combined recall output.
type RecallResult struct {
	// Entities are the graph entities for the expanded label set.
	// Includes seed entities and those discovered via graph traversal.
	Entities []EntityInfo

	// Segments are the matching memory fragments, scored and sorted
	// by relevance.
	Segments []ScoredSegment

	// Summaries are relevant long-term summaries for the time window.
	Summaries []TimedSummary
}

// EntityInfo holds a graph entity's label and attributes for context building.
type EntityInfo struct {
	Label string         `json:"label"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// ScoredSegment pairs a segment summary with its relevance score.
type ScoredSegment struct {
	ID        string   `json:"id"`
	Summary   string   `json:"summary"`
	Keywords  []string `json:"keywords,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Timestamp int64    `json:"ts"`
	Score     float64  `json:"score"`
}

// ---------------------------------------------------------------------------
// Compressor: upper-layer compression interface
// ---------------------------------------------------------------------------

// Compressor is implemented by the agent runtime to drive message compression.
// The memory package calls these methods when conversations are closed or
// when message count exceeds a threshold.
type Compressor interface {
	// CompressMessages compresses old conversation messages into memory
	// segments and an updated summary.
	CompressMessages(ctx context.Context, messages []Message) (*CompressResult, error)

	// ExtractEntities extracts entity and relation updates from messages.
	ExtractEntities(ctx context.Context, messages []Message) (*EntityUpdate, error)
}

// CompressResult holds the output of [Compressor.CompressMessages].
type CompressResult struct {
	// Segments are newly created memory fragments.
	Segments []SegmentInput

	// Summary is the updated summary text for long-term storage.
	Summary string
}

// SegmentInput is the input for creating a new memory segment.
// The caller provides content; the memory system assigns ID and timestamp.
type SegmentInput struct {
	Summary  string
	Keywords []string
	Labels   []string
}

// EntityUpdate holds entity and relation changes extracted from messages.
type EntityUpdate struct {
	// Entities to create or update (merge attributes).
	Entities []EntityInput

	// Relations to add.
	Relations []RelationInput
}

// EntityInput specifies an entity to create or merge attributes into.
type EntityInput struct {
	Label string
	Attrs map[string]any
}

// RelationInput specifies a directed relation to add.
type RelationInput struct {
	From    string
	To      string
	RelType string
}

// ---------------------------------------------------------------------------
// Time helpers
// ---------------------------------------------------------------------------

// lastNano tracks the most recently returned timestamp to ensure monotonicity.
// If the wall clock hasn't advanced since the last call, the counter increments
// by 1 nanosecond. This prevents segment ID / KV key collisions when many
// segments are stored in rapid succession.
var lastNano atomic.Int64

// nowNano returns a monotonically increasing Unix nanosecond timestamp.
// Extracted as a variable to allow test injection.
var nowNano = func() int64 {
	now := time.Now().UnixNano()
	for {
		old := lastNano.Load()
		next := now
		if next <= old {
			next = old + 1
		}
		if lastNano.CompareAndSwap(old, next) {
			return next
		}
	}
}
