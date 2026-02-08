package recall

import "context"

// Query specifies parameters for [Index.Search], the high-level combined
// search that expands labels via the graph before searching segments.
type Query struct {
	// Labels are seed entity labels (e.g., "person:Alice") to expand
	// through the graph before searching segments.
	Labels []string

	// Text is the search text used for vector embedding and keyword
	// matching.
	Text string

	// Hops controls how many graph traversal hops to expand from the
	// seed labels. Default is 2 if zero.
	Hops int

	// Limit is the maximum number of segments to return. Default is 10
	// if zero.
	Limit int
}

// Result holds the output of [Index.Search].
type Result struct {
	// Segments are the matching segments, scored and sorted by relevance.
	Segments []ScoredSegment

	// Expanded is the full set of entity labels after graph expansion.
	// Includes the original seed labels plus any discovered via traversal.
	Expanded []string
}

// Search performs a combined search: graph expansion followed by segment search.
//
// The flow:
//  1. Expand seed labels through the graph (breadth-first, up to Hops).
//  2. Build a SearchQuery with the expanded labels and query text.
//  3. Run SearchSegments to find matching segments.
//
// If no labels are provided, graph expansion is skipped and search is
// purely text-based (vector + keyword).
func (idx *Index) Search(ctx context.Context, q Query) (*Result, error) {
	hops := q.Hops
	if hops <= 0 {
		hops = 2
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 10
	}

	// Step 1: Graph expansion.
	var expanded []string
	if len(q.Labels) > 0 {
		var err error
		expanded, err = idx.graph.Expand(ctx, q.Labels, hops)
		if err != nil {
			return nil, err
		}
	}

	// Step 2: Build segment search query.
	sq := SearchQuery{
		Text:   q.Text,
		Labels: expanded,
		Limit:  limit,
	}

	// Step 3: Search segments.
	segments, err := idx.SearchSegments(ctx, sq)
	if err != nil {
		return nil, err
	}

	return &Result{
		Segments: segments,
		Expanded: expanded,
	}, nil
}
