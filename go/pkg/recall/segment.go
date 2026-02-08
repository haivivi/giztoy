package recall

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/vmihailenco/msgpack/v5"
)

// Score fusion weights for combining search signals.
const (
	weightVector  = 0.5
	weightKeyword = 0.3
	weightLabel   = 0.2
)

// StoreSegment persists a segment in the index.
//
// It stores the segment data in KV (msgpack-encoded, keyed by timestamp),
// and if an embedder and vector index are configured, embeds the summary
// text and inserts the resulting vector.
func (idx *Index) StoreSegment(ctx context.Context, seg Segment) error {
	data, err := msgpack.Marshal(seg)
	if err != nil {
		return err
	}

	key := segmentKey(idx.prefix, seg.Timestamp)
	if err := idx.store.Set(ctx, key, data); err != nil {
		return err
	}

	// Index the vector if both embedder and vector index are available.
	if idx.embedder != nil && idx.vec != nil {
		vec, err := idx.embedder.Embed(ctx, seg.Summary)
		if err != nil {
			return err
		}
		if err := idx.vec.Insert(seg.ID, vec); err != nil {
			return err
		}
	}

	return nil
}

// DeleteSegment removes a segment by ID.
//
// Since segment keys are time-based (not ID-based), this scans all
// segments to find the matching ID. For typical per-persona indexes
// (hundreds to low thousands of segments) this is acceptable.
func (idx *Index) DeleteSegment(ctx context.Context, id string) error {
	prefix := segmentPrefix(idx.prefix)

	for entry, err := range idx.store.List(ctx, prefix) {
		if err != nil {
			return err
		}
		var seg Segment
		if err := msgpack.Unmarshal(entry.Value, &seg); err != nil {
			continue // skip malformed entries
		}
		if seg.ID == id {
			if err := idx.store.Delete(ctx, entry.Key); err != nil {
				return err
			}
			// Remove from vector index if configured.
			if idx.vec != nil {
				_ = idx.vec.Delete(id)
			}
			return nil
		}
	}

	return nil // not found is not an error
}

// GetSegment retrieves a segment by ID.
//
// Scans all segments to find the matching ID. Returns nil if not found.
func (idx *Index) GetSegment(ctx context.Context, id string) (*Segment, error) {
	prefix := segmentPrefix(idx.prefix)

	for entry, err := range idx.store.List(ctx, prefix) {
		if err != nil {
			return nil, err
		}
		var seg Segment
		if err := msgpack.Unmarshal(entry.Value, &seg); err != nil {
			continue
		}
		if seg.ID == id {
			return &seg, nil
		}
	}

	return nil, nil
}

// RecentSegments returns the n most recent segments, ordered newest first.
//
// KV keys are lexicographically ordered by date+timestamp, so we scan
// all segments and take the last n.
func (idx *Index) RecentSegments(ctx context.Context, n int) ([]Segment, error) {
	if n <= 0 {
		return nil, nil
	}

	prefix := segmentPrefix(idx.prefix)
	var all []Segment

	for entry, err := range idx.store.List(ctx, prefix) {
		if err != nil {
			return nil, err
		}
		var seg Segment
		if err := msgpack.Unmarshal(entry.Value, &seg); err != nil {
			continue
		}
		all = append(all, seg)
	}

	// KV list is ascending; reverse to get newest first.
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}

	if len(all) > n {
		all = all[:n]
	}
	return all, nil
}

// SearchSegments searches for segments matching the query using multi-signal
// scoring: vector similarity, keyword overlap, and label overlap.
//
// The scoring pipeline:
//  1. Collect all segments (with optional time filtering)
//  2. If embedder + vec are available: embed query text → vector search → distance scores
//  3. Keyword score: fraction of query terms found in segment keywords
//  4. Label score: fraction of segment labels overlapping with query labels
//  5. Fuse: 0.5*vector + 0.3*keyword + 0.2*label
//  6. Sort by score descending, return top Limit
func (idx *Index) SearchSegments(ctx context.Context, q SearchQuery) ([]ScoredSegment, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 10
	}

	// Step 1: Load all segments with time filtering.
	segments, err := idx.loadSegments(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(segments) == 0 {
		return nil, nil
	}

	// Step 2: Vector scores (ID → score in [0,1]).
	vecScores := make(map[string]float64)
	if idx.embedder != nil && idx.vec != nil && q.Text != "" && idx.vec.Len() > 0 {
		queryVec, err := idx.embedder.Embed(ctx, q.Text)
		if err != nil {
			return nil, err
		}
		// Request more candidates than limit to account for filtering.
		topK := limit * 3
		if topK < 20 {
			topK = 20
		}
		matches, err := idx.vec.Search(queryVec, topK)
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			// Convert distance to similarity score in [0,1].
			// Assuming cosine distance in [0,2], similarity = 1 - distance/2.
			sim := 1.0 - float64(m.Distance)/2.0
			if sim < 0 {
				sim = 0
			}
			vecScores[m.ID] = sim
		}
	}

	// Step 3+4+5: Score each segment and fuse signals.
	queryTerms := tokenize(q.Text)
	labelSet := toSet(q.Labels)
	hasVec := len(vecScores) > 0
	hasKeywords := len(queryTerms) > 0
	hasLabels := len(labelSet) > 0

	scored := make([]ScoredSegment, 0, len(segments))
	for _, seg := range segments {
		score := 0.0

		// Vector signal.
		if hasVec {
			if vs, ok := vecScores[seg.ID]; ok {
				score += weightVector * vs
			}
		}

		// Keyword signal: fraction of query terms found in segment keywords.
		if hasKeywords {
			score += weightKeyword * keywordScore(queryTerms, seg.Keywords)
		}

		// Label signal: fraction of segment labels in query label set.
		if hasLabels {
			score += weightLabel * labelScore(seg.Labels, labelSet)
		}

		// Skip zero-score segments unless no filters were applied.
		if score == 0 && (hasVec || hasKeywords || hasLabels) {
			continue
		}

		scored = append(scored, ScoredSegment{Segment: seg, Score: score})
	}

	// Step 6: Sort by score descending, then by timestamp descending.
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Segment.Timestamp > scored[j].Segment.Timestamp
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

// loadSegments scans all segments from KV, applying time and label filters.
func (idx *Index) loadSegments(ctx context.Context, q SearchQuery) ([]Segment, error) {
	prefix := segmentPrefix(idx.prefix)
	afterNs := int64(0)
	beforeNs := int64(math.MaxInt64)
	if !q.After.IsZero() {
		afterNs = q.After.UnixNano()
	}
	if !q.Before.IsZero() {
		beforeNs = q.Before.UnixNano()
	}

	labelSet := toSet(q.Labels)
	filterLabels := len(labelSet) > 0

	var segments []Segment
	for entry, err := range idx.store.List(ctx, prefix) {
		if err != nil {
			return nil, err
		}
		var seg Segment
		if err := msgpack.Unmarshal(entry.Value, &seg); err != nil {
			continue
		}

		// Time filter.
		if seg.Timestamp < afterNs || seg.Timestamp >= beforeNs {
			continue
		}

		// Label filter: segment must share at least one label.
		if filterLabels && !hasOverlap(seg.Labels, labelSet) {
			continue
		}

		segments = append(segments, seg)
	}
	return segments, nil
}

// keywordScore computes the fraction of query terms found in the segment's
// keywords. Both are compared in lowercase for case-insensitive matching.
func keywordScore(queryTerms []string, segKeywords []string) float64 {
	if len(queryTerms) == 0 {
		return 0
	}
	segSet := make(map[string]struct{}, len(segKeywords))
	for _, kw := range segKeywords {
		segSet[strings.ToLower(kw)] = struct{}{}
	}
	hits := 0
	for _, qt := range queryTerms {
		if _, ok := segSet[qt]; ok {
			hits++
		}
	}
	return float64(hits) / float64(len(queryTerms))
}

// labelScore computes the fraction of segment labels present in the query
// label set. Returns 0 if the segment has no labels.
func labelScore(segLabels []string, queryLabelSet map[string]struct{}) float64 {
	if len(segLabels) == 0 {
		return 0
	}
	hits := 0
	for _, l := range segLabels {
		if _, ok := queryLabelSet[l]; ok {
			hits++
		}
	}
	return float64(hits) / float64(len(segLabels))
}

// tokenize splits text into lowercase terms for keyword matching.
func tokenize(text string) []string {
	if text == "" {
		return nil
	}
	fields := strings.Fields(strings.ToLower(text))
	// Deduplicate.
	seen := make(map[string]struct{}, len(fields))
	result := make([]string, 0, len(fields))
	for _, f := range fields {
		if _, ok := seen[f]; !ok {
			seen[f] = struct{}{}
			result = append(result, f)
		}
	}
	return result
}

// toSet converts a string slice to a set (map).
func toSet(items []string) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(items))
	for _, item := range items {
		m[item] = struct{}{}
	}
	return m
}

// hasOverlap returns true if any element of items is in the set.
func hasOverlap(items []string, set map[string]struct{}) bool {
	for _, item := range items {
		if _, ok := set[item]; ok {
			return true
		}
	}
	return false
}

