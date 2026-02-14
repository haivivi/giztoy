// Package memory provides a personal memory system for AI personas.
//
// Each persona (virtual character) has an isolated [Memory] instance containing
// an entity-relation graph, time-bucketed segments (compressed memory fragments),
// and active conversations. The [Host] manages many Memory instances sharing a
// single KV store, embedding service, and vector index.
//
// # Architecture
//
// Memory sits above the [recall] package and adds persona-level semantics:
//
//   - Atom isolation: each persona gets its own [recall.Index] scoped by a
//     KV key prefix ("mem:{id}").
//   - Conversations: short-term message storage per device/session.
//   - Bucket-based compaction: segments at finer granularity are automatically
//     compacted into coarser buckets (1h → 1d → 1w → 1m → 3m → 6m → 1y → lt).
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
//	host, err := memory.NewHost(ctx, memory.HostConfig{
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
	// by relevance. Segments from all buckets are merged.
	Segments []ScoredSegment
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
// CompressPolicy: when to trigger auto-compression
// ---------------------------------------------------------------------------

// CompressPolicy controls when [Conversation.Append] triggers automatic
// compression, and when bucket compaction is triggered by [Memory.Compact].
//
// The same policy applies to all buckets uniformly. Compression fires when
// either threshold is reached.
//
// Zero values for both fields disables auto-compression — the caller must
// invoke [Memory.Compress] manually.
type CompressPolicy struct {
	// MaxChars triggers compression when the total character count of
	// pending (uncompressed) content reaches this value. Default 2000.
	MaxChars int

	// MaxMessages triggers compression when the number of pending
	// items (messages or segments) reaches this value. Default 50.
	MaxMessages int
}

// DefaultCompressPolicy returns the default compression policy.
// 32000 chars ≈ 16k tokens (Chinese ~2 bytes/char UTF-8).
func DefaultCompressPolicy() CompressPolicy {
	return CompressPolicy{MaxChars: 32000, MaxMessages: 30}
}

// shouldCompress reports whether the policy thresholds have been reached.
func (p CompressPolicy) shouldCompress(chars, msgs int) bool {
	if p.MaxChars <= 0 && p.MaxMessages <= 0 {
		return false // auto-compression disabled
	}
	if p.MaxChars > 0 && chars >= p.MaxChars {
		return true
	}
	if p.MaxMessages > 0 && msgs >= p.MaxMessages {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Compressor: upper-layer compression interface
// ---------------------------------------------------------------------------

// Compressor is implemented by the agent runtime to drive message compression
// and segment compaction. The memory package calls these methods when
// conversations exceed thresholds or when bucket compaction is triggered.
type Compressor interface {
	// CompressMessages compresses conversation messages into memory
	// segments and an updated summary.
	CompressMessages(ctx context.Context, messages []Message) (*CompressResult, error)

	// ExtractEntities extracts entity and relation updates from messages.
	ExtractEntities(ctx context.Context, messages []Message) (*EntityUpdate, error)

	// CompactSegments compresses multiple segment summaries into a single
	// new segment. Used for bucket compaction (e.g., merging hourly
	// segments into a daily segment). The input strings are the summaries
	// of the segments being compacted.
	CompactSegments(ctx context.Context, summaries []string) (*CompressResult, error)
}

// CompressResult holds the output of [Compressor.CompressMessages] or
// [Compressor.CompactSegments].
type CompressResult struct {
	// Segments are newly created memory fragments.
	Segments []SegmentInput

	// Summary is the updated summary text for the compacted segment.
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
