// Package recall provides a bottom-level search engine combining segment
// storage, entity-relation graphs, vector similarity search, and keyword
// matching into a unified search space.
//
// An [Index] represents a single search space (e.g., one persona's memory,
// one knowledge base, one news zone). The upper layer decides how to
// partition and isolate data by choosing the KV key prefix.
//
// # Search Signals
//
// [Index.SearchSegments] fuses three signals:
//
//   - Vector cosine similarity (via [VectorIndex] + [embed.Embedder])
//   - Keyword overlap between query terms and segment keywords
//   - Label overlap between query labels and segment labels
//
// [Index.Search] adds graph expansion before segment search: it calls
// [graph.Graph.Expand] on the seed labels to discover related entities,
// then searches segments using the expanded label set.
package recall

import "time"

// Segment is a memory fragment stored in the index.
// Each segment carries a text summary, optional keywords and labels for
// filtering, and a timestamp for time-range queries.
type Segment struct {
	// ID is the unique identifier for this segment.
	ID string `json:"id" msgpack:"id"`

	// Summary is the human-readable text content of this segment.
	Summary string `json:"summary" msgpack:"summary"`

	// Keywords are terms extracted from the segment for keyword matching.
	Keywords []string `json:"keywords,omitempty" msgpack:"keywords,omitempty"`

	// Labels tag this segment with entity references (e.g., "person:Alice",
	// "topic:dinosaurs"). Used for label-based filtering during search.
	Labels []string `json:"labels,omitempty" msgpack:"labels,omitempty"`

	// Timestamp is the Unix timestamp in nanoseconds when this segment
	// was created.
	Timestamp int64 `json:"ts" msgpack:"ts"`
}

// VectorIndex is the interface for approximate nearest-neighbor search
// over dense float32 vectors. Implementations include in-memory brute
// force (for testing) and HNSW (for production).
type VectorIndex interface {
	// Insert adds or updates a vector with the given ID.
	Insert(id string, vector []float32) error

	// Search returns the top-k nearest vectors to the query.
	// Results are ordered by ascending distance (closest first).
	Search(query []float32, topK int) ([]VectorMatch, error)

	// Delete removes a vector by ID. No error if ID does not exist.
	Delete(id string) error

	// Len returns the number of vectors in the index.
	Len() int

	// Close releases resources held by the index.
	Close() error
}

// VectorMatch is a single result from a vector similarity search.
type VectorMatch struct {
	// ID is the identifier of the matched vector.
	ID string

	// Distance is the distance between the query and matched vector.
	// Lower values indicate higher similarity.
	Distance float32
}

// SearchQuery specifies parameters for [Index.SearchSegments].
type SearchQuery struct {
	// Text is the query text used for both vector embedding and keyword
	// matching. If empty, only label filtering is applied.
	Text string

	// Labels filters segments by label overlap. Only segments sharing
	// at least one label with this set are returned. Empty means no
	// label filter.
	Labels []string

	// Limit is the maximum number of segments to return. Default is 10.
	Limit int

	// After filters segments to those created at or after this time.
	// Zero value means no lower bound.
	After time.Time

	// Before filters segments to those created before this time.
	// Zero value means no upper bound.
	Before time.Time
}

// ScoredSegment pairs a segment with its relevance score.
type ScoredSegment struct {
	Segment Segment `json:"segment"`
	Score   float64 `json:"score"`
}
