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
//   - Vector cosine similarity (via [vecstore.Index] + [embed.Embedder])
//   - Keyword overlap between query terms and segment keywords
//   - Label overlap between query labels and segment labels
//
// [Index.Search] adds graph expansion before segment search: it calls
// [graph.Graph.Expand] on the seed labels to discover related entities,
// then searches segments using the expanded label set.
package recall

import (
	"time"
)

// Bucket identifies a time-granularity bucket for segment storage.
// Segments are organized into buckets based on the time span they cover.
// When a bucket exceeds its capacity (count or token threshold), the
// oldest segments are compacted into a coarser bucket.
//
// The bucket hierarchy:
//
//	1h → 1d → 1w → 1m → 3m → 6m → 1y → lt
//
// The "lt" (lifetime) bucket is terminal — it only grows, never compacted.
type Bucket string

const (
	Bucket1H Bucket = "1h" // 1 hour
	Bucket1D Bucket = "1d" // 1 day
	Bucket1W Bucket = "1w" // 1 week
	Bucket1M Bucket = "1m" // 1 month
	Bucket3M Bucket = "3m" // 3 months
	Bucket6M Bucket = "6m" // 6 months
	Bucket1Y Bucket = "1y" // 1 year
	BucketLT Bucket = "lt" // lifetime (terminal)
)

// AllBuckets lists all buckets in order from finest to coarsest.
var AllBuckets = []Bucket{Bucket1H, Bucket1D, Bucket1W, Bucket1M, Bucket3M, Bucket6M, Bucket1Y, BucketLT}

// CompactableBuckets lists all buckets that can be compacted (excludes lt).
var CompactableBuckets = []Bucket{Bucket1H, Bucket1D, Bucket1W, Bucket1M, Bucket3M, Bucket6M, Bucket1Y}

// BucketDuration returns the maximum time span a segment in this bucket
// should cover. BucketLT returns 0 (no upper bound).
func BucketDuration(b Bucket) time.Duration {
	switch b {
	case Bucket1H:
		return time.Hour
	case Bucket1D:
		return 24 * time.Hour
	case Bucket1W:
		return 7 * 24 * time.Hour
	case Bucket1M:
		return 30 * 24 * time.Hour
	case Bucket3M:
		return 90 * 24 * time.Hour
	case Bucket6M:
		return 180 * 24 * time.Hour
	case Bucket1Y:
		return 365 * 24 * time.Hour
	default:
		return 0
	}
}

// BucketForSpan returns the smallest bucket whose duration covers the
// given time span. If span exceeds 1 year, BucketLT is returned.
func BucketForSpan(span time.Duration) Bucket {
	for _, b := range AllBuckets {
		d := BucketDuration(b)
		if d == 0 || span <= d {
			return b
		}
	}
	return BucketLT
}

// Segment is a memory fragment stored in the index.
// Each segment carries a text summary, optional keywords and labels for
// filtering, a timestamp, and a bucket indicating its time granularity.
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

	// Timestamp is the Unix timestamp in nanoseconds of the latest event
	// covered by this segment. Used for ordering and time-range queries.
	Timestamp int64 `json:"ts" msgpack:"ts"`

	// Bucket identifies which time-granularity bucket this segment
	// belongs to. Set when the segment is stored. Defaults to Bucket1H
	// for segments produced by realtime conversation compression.
	Bucket Bucket `json:"bucket,omitempty" msgpack:"bucket,omitempty"`
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
