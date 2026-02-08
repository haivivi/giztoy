package recall

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/graph"
	"github.com/haivivi/giztoy/go/pkg/kv"
)

// mockEmbedder returns deterministic vectors based on text content.
// Each unique text maps to a distinct direction in 4D space.
type mockEmbedder struct {
	dim     int
	vectors map[string][]float32
	calls   int
}

func newMockEmbedder() *mockEmbedder {
	return &mockEmbedder{
		dim: 4,
		vectors: map[string][]float32{
			"dinosaurs":       {1, 0, 0, 0},
			"space":           {0, 1, 0, 0},
			"cooking":         {0, 0, 1, 0},
			"dinosaur fossils": {0.9, 0.1, 0, 0}, // similar to dinosaurs
			"rocket launch":   {0.1, 0.9, 0, 0},  // similar to space
		},
	}
}

func (m *mockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	m.calls++
	if v, ok := m.vectors[text]; ok {
		return v, nil
	}
	// Default: zero vector (won't match anything well).
	return make([]float32, m.dim), nil
}

func (m *mockEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := m.Embed(context.Background(), t)
		if err != nil {
			return nil, err
		}
		vecs[i] = v
	}
	return vecs, nil
}

func (m *mockEmbedder) Dimension() int { return m.dim }

// helper to create a test index with all components.
func newTestIndex(t *testing.T) (*Index, *mockEmbedder) {
	t.Helper()
	store := kv.NewMemory(nil)
	emb := newMockEmbedder()
	vec := NewMemVec()

	idx := NewIndex(IndexConfig{
		Store:    store,
		Embedder: emb,
		Vec:      vec,
		Prefix:   kv.Key{"test"},
	})
	return idx, emb
}

// helper to create an index without vector search.
func newTestIndexNoVec(t *testing.T) *Index {
	t.Helper()
	store := kv.NewMemory(nil)
	return NewIndex(IndexConfig{
		Store:  store,
		Prefix: kv.Key{"test"},
	})
}

func TestStoreAndGetSegment(t *testing.T) {
	idx, _ := newTestIndex(t)
	ctx := context.Background()

	seg := Segment{
		ID:        "seg-001",
		Summary:   "dinosaurs",
		Keywords:  []string{"dinosaur", "fossil"},
		Labels:    []string{"Alice", "dinosaurs"},
		Timestamp: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC).UnixNano(),
	}

	if err := idx.StoreSegment(ctx, seg); err != nil {
		t.Fatalf("StoreSegment: %v", err)
	}

	got, err := idx.GetSegment(ctx, "seg-001")
	if err != nil {
		t.Fatalf("GetSegment: %v", err)
	}
	if got == nil {
		t.Fatal("GetSegment returned nil")
	}
	if got.ID != seg.ID {
		t.Errorf("ID = %q, want %q", got.ID, seg.ID)
	}
	if got.Summary != seg.Summary {
		t.Errorf("Summary = %q, want %q", got.Summary, seg.Summary)
	}
	if got.Timestamp != seg.Timestamp {
		t.Errorf("Timestamp = %d, want %d", got.Timestamp, seg.Timestamp)
	}
}

func TestDeleteSegment(t *testing.T) {
	idx, _ := newTestIndex(t)
	ctx := context.Background()

	seg := Segment{
		ID:        "seg-del",
		Summary:   "cooking",
		Timestamp: time.Now().UnixNano(),
	}
	if err := idx.StoreSegment(ctx, seg); err != nil {
		t.Fatalf("StoreSegment: %v", err)
	}

	if err := idx.DeleteSegment(ctx, "seg-del"); err != nil {
		t.Fatalf("DeleteSegment: %v", err)
	}

	got, err := idx.GetSegment(ctx, "seg-del")
	if err != nil {
		t.Fatalf("GetSegment after delete: %v", err)
	}
	if got != nil {
		t.Error("segment should be nil after delete")
	}
}

func TestDeleteSegmentNotFound(t *testing.T) {
	idx, _ := newTestIndex(t)
	ctx := context.Background()

	// Deleting a non-existent segment should not error.
	if err := idx.DeleteSegment(ctx, "nonexistent"); err != nil {
		t.Fatalf("DeleteSegment nonexistent: %v", err)
	}
}

func TestRecentSegments(t *testing.T) {
	idx, _ := newTestIndex(t)
	ctx := context.Background()

	base := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		seg := Segment{
			ID:        fmt.Sprintf("seg-%d", i),
			Summary:   "dinosaurs",
			Timestamp: base.Add(time.Duration(i) * time.Hour).UnixNano(),
		}
		if err := idx.StoreSegment(ctx, seg); err != nil {
			t.Fatalf("StoreSegment %d: %v", i, err)
		}
	}

	recent, err := idx.RecentSegments(ctx, 3)
	if err != nil {
		t.Fatalf("RecentSegments: %v", err)
	}
	if len(recent) != 3 {
		t.Fatalf("len = %d, want 3", len(recent))
	}

	// Should be newest first: seg-4, seg-3, seg-2.
	for i, want := range []string{"seg-4", "seg-3", "seg-2"} {
		if recent[i].ID != want {
			t.Errorf("recent[%d].ID = %q, want %q", i, recent[i].ID, want)
		}
	}
}

func TestRecentSegmentsEmpty(t *testing.T) {
	idx, _ := newTestIndex(t)
	ctx := context.Background()

	recent, err := idx.RecentSegments(ctx, 5)
	if err != nil {
		t.Fatalf("RecentSegments: %v", err)
	}
	if len(recent) != 0 {
		t.Errorf("expected empty, got %d", len(recent))
	}
}

func TestSearchSegmentsVector(t *testing.T) {
	idx, _ := newTestIndex(t)
	ctx := context.Background()

	// Store segments with different topics.
	segments := []Segment{
		{ID: "dino", Summary: "dinosaurs", Keywords: []string{"dinosaur"}, Timestamp: 1},
		{ID: "space", Summary: "space", Keywords: []string{"space"}, Timestamp: 2},
		{ID: "cook", Summary: "cooking", Keywords: []string{"cooking"}, Timestamp: 3},
	}
	for _, s := range segments {
		if err := idx.StoreSegment(ctx, s); err != nil {
			t.Fatalf("StoreSegment %s: %v", s.ID, err)
		}
	}

	// Search for "dinosaur fossils" — should rank "dino" highest via vector.
	results, err := idx.SearchSegments(ctx, SearchQuery{
		Text:  "dinosaur fossils",
		Limit: 3,
	})
	if err != nil {
		t.Fatalf("SearchSegments: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results, got none")
	}
	if results[0].Segment.ID != "dino" {
		t.Errorf("top result = %q, want 'dino'", results[0].Segment.ID)
	}
}

func TestSearchSegmentsKeywordOnly(t *testing.T) {
	idx := newTestIndexNoVec(t)
	ctx := context.Background()

	segments := []Segment{
		{ID: "s1", Summary: "chatted about dinosaurs", Keywords: []string{"dinosaur", "fossil"}, Timestamp: 1},
		{ID: "s2", Summary: "went to the park", Keywords: []string{"park", "outdoor"}, Timestamp: 2},
		{ID: "s3", Summary: "found a fossil", Keywords: []string{"fossil", "discovery"}, Timestamp: 3},
	}
	for _, s := range segments {
		if err := idx.StoreSegment(ctx, s); err != nil {
			t.Fatalf("StoreSegment: %v", err)
		}
	}

	results, err := idx.SearchSegments(ctx, SearchQuery{
		Text:  "fossil",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("SearchSegments: %v", err)
	}

	// Both s1 and s3 have "fossil" keyword.
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	ids := make(map[string]bool)
	for _, r := range results {
		ids[r.Segment.ID] = true
	}
	if !ids["s1"] || !ids["s3"] {
		t.Errorf("expected s1 and s3 in results, got %v", ids)
	}
}

func TestSearchSegmentsLabelFilter(t *testing.T) {
	idx := newTestIndexNoVec(t)
	ctx := context.Background()

	segments := []Segment{
		{ID: "s1", Summary: "hello", Labels: []string{"Alice"}, Keywords: []string{"hello"}, Timestamp: 1},
		{ID: "s2", Summary: "hello", Labels: []string{"Bob"}, Keywords: []string{"hello"}, Timestamp: 2},
		{ID: "s3", Summary: "hello", Labels: []string{"Alice", "Bob"}, Keywords: []string{"hello"}, Timestamp: 3},
	}
	for _, s := range segments {
		if err := idx.StoreSegment(ctx, s); err != nil {
			t.Fatalf("StoreSegment: %v", err)
		}
	}

	results, err := idx.SearchSegments(ctx, SearchQuery{
		Text:   "hello",
		Labels: []string{"Alice"},
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("SearchSegments: %v", err)
	}

	// Only s1 and s3 have label "Alice".
	ids := make(map[string]bool)
	for _, r := range results {
		ids[r.Segment.ID] = true
	}
	if ids["s2"] {
		t.Error("s2 should be filtered out (no matching label)")
	}
	if !ids["s1"] || !ids["s3"] {
		t.Errorf("expected s1 and s3, got %v", ids)
	}
}

func TestSearchSegmentsTimeFilter(t *testing.T) {
	idx := newTestIndexNoVec(t)
	ctx := context.Background()

	base := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		seg := Segment{
			ID:        fmt.Sprintf("s%d", i),
			Summary:   "event",
			Keywords:  []string{"event"},
			Timestamp: base.Add(time.Duration(i) * 24 * time.Hour).UnixNano(),
		}
		if err := idx.StoreSegment(ctx, seg); err != nil {
			t.Fatalf("StoreSegment: %v", err)
		}
	}

	// Search for events on June 2-4 only.
	results, err := idx.SearchSegments(ctx, SearchQuery{
		Text:   "event",
		After:  base.Add(1 * 24 * time.Hour),  // June 2
		Before: base.Add(4 * 24 * time.Hour),  // June 5 exclusive
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("SearchSegments: %v", err)
	}

	// Should include s1, s2, s3 (June 2, 3, 4).
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	ids := make(map[string]bool)
	for _, r := range results {
		ids[r.Segment.ID] = true
	}
	if !ids["s1"] || !ids["s2"] || !ids["s3"] {
		t.Errorf("expected s1,s2,s3, got %v", ids)
	}
}

func TestSearchWithGraphExpansion(t *testing.T) {
	idx, _ := newTestIndex(t)
	ctx := context.Background()

	// Set up graph: Alice --knows--> Bob, Bob --likes--> dinosaurs.
	// Note: graph entity labels cannot contain ':' (KV separator),
	// so we use plain names. Segment labels mirror graph entity labels.
	g := idx.Graph()
	if err := g.SetEntity(ctx, graph.Entity{Label: "Alice"}); err != nil {
		t.Fatal(err)
	}
	if err := g.SetEntity(ctx, graph.Entity{Label: "Bob"}); err != nil {
		t.Fatal(err)
	}
	if err := g.SetEntity(ctx, graph.Entity{Label: "dinosaurs"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(ctx, graph.Relation{From: "Alice", To: "Bob", RelType: "knows"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(ctx, graph.Relation{From: "Bob", To: "dinosaurs", RelType: "likes"}); err != nil {
		t.Fatal(err)
	}

	// Store segments with labels matching graph entities.
	segments := []Segment{
		{ID: "s-alice", Summary: "dinosaurs", Labels: []string{"Alice"}, Keywords: []string{"chat"}, Timestamp: 1},
		{ID: "s-bob-dino", Summary: "dinosaurs", Labels: []string{"Bob", "dinosaurs"}, Keywords: []string{"dinosaur"}, Timestamp: 2},
		{ID: "s-charlie", Summary: "dinosaurs", Labels: []string{"Charlie"}, Keywords: []string{"unrelated"}, Timestamp: 3},
	}
	for _, s := range segments {
		if err := idx.StoreSegment(ctx, s); err != nil {
			t.Fatalf("StoreSegment %s: %v", s.ID, err)
		}
	}

	// Search starting from Alice with 2 hops → expands to Alice, Bob, dinosaurs.
	result, err := idx.Search(ctx, Query{
		Labels: []string{"Alice"},
		Text:   "dinosaurs",
		Hops:   2,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	// Expanded labels should include Alice, Bob, and dinosaurs.
	expandedSet := toSet(result.Expanded)
	for _, want := range []string{"Alice", "Bob", "dinosaurs"} {
		if _, ok := expandedSet[want]; !ok {
			t.Errorf("expected %q in expanded labels, got %v", want, result.Expanded)
		}
	}

	// s-charlie should be filtered out (Charlie not in expanded set).
	for _, ss := range result.Segments {
		if ss.Segment.ID == "s-charlie" {
			t.Error("s-charlie should be filtered out by label expansion")
		}
	}

	// s-alice and s-bob-dino should be in results.
	ids := make(map[string]bool)
	for _, ss := range result.Segments {
		ids[ss.Segment.ID] = true
	}
	if !ids["s-alice"] || !ids["s-bob-dino"] {
		t.Errorf("expected s-alice and s-bob-dino in results, got %v", ids)
	}
}

func TestSearchNoLabels(t *testing.T) {
	idx, _ := newTestIndex(t)
	ctx := context.Background()

	seg := Segment{
		ID:        "s1",
		Summary:   "dinosaurs",
		Keywords:  []string{"dinosaur"},
		Timestamp: 1,
	}
	if err := idx.StoreSegment(ctx, seg); err != nil {
		t.Fatal(err)
	}

	// Search without labels — should still return results via text matching.
	result, err := idx.Search(ctx, Query{
		Text:  "dinosaurs",
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Expanded) != 0 {
		t.Errorf("expected no expanded labels, got %v", result.Expanded)
	}
	if len(result.Segments) == 0 {
		t.Error("expected at least one result from text-only search")
	}
}

func TestMemVecCosineDistance(t *testing.T) {
	tests := []struct {
		name string
		a, b []float32
		want float32
	}{
		{"identical", []float32{1, 0, 0}, []float32{1, 0, 0}, 0},
		{"orthogonal", []float32{1, 0, 0}, []float32{0, 1, 0}, 1},
		{"opposite", []float32{1, 0, 0}, []float32{-1, 0, 0}, 2},
		{"similar", []float32{1, 0.1, 0}, []float32{1, 0, 0}, 0.005},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineDistance(tt.a, tt.b)
			if diff := got - tt.want; diff > 0.01 || diff < -0.01 {
				t.Errorf("cosineDistance = %f, want ~%f", got, tt.want)
			}
		})
	}
}

func TestMemVecSearchTopK(t *testing.T) {
	vec := NewMemVec()

	_ = vec.Insert("a", []float32{1, 0, 0, 0})
	_ = vec.Insert("b", []float32{0, 1, 0, 0})
	_ = vec.Insert("c", []float32{0.9, 0.1, 0, 0})

	matches, err := vec.Search([]float32{1, 0, 0, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	// "a" should be closest (distance 0), then "c".
	if matches[0].ID != "a" {
		t.Errorf("top match = %q, want 'a'", matches[0].ID)
	}
	if matches[1].ID != "c" {
		t.Errorf("second match = %q, want 'c'", matches[1].ID)
	}
}

func TestKeyHelpers(t *testing.T) {
	prefix := kv.Key{"mem", "cat"}
	ts := time.Date(2025, 6, 15, 12, 30, 0, 0, time.UTC).UnixNano()

	key := segmentKey(prefix, ts)
	if len(key) != 5 {
		t.Fatalf("key len = %d, want 5", len(key))
	}
	if key[0] != "mem" || key[1] != "cat" || key[2] != "seg" || key[3] != "20250615" {
		t.Errorf("key = %v", key)
	}

	gotTs, err := parseSegmentKey(key, 2)
	if err != nil {
		t.Fatalf("parseSegmentKey: %v", err)
	}
	if gotTs != ts {
		t.Errorf("parsed ts = %d, want %d", gotTs, ts)
	}

	sp := segmentPrefix(prefix)
	if len(sp) != 3 || sp[2] != "seg" {
		t.Errorf("segmentPrefix = %v", sp)
	}

	gp := graphPrefix(prefix)
	if len(gp) != 3 || gp[2] != "g" {
		t.Errorf("graphPrefix = %v", gp)
	}
}

func TestSegmentDatePrefix(t *testing.T) {
	prefix := kv.Key{"mem", "cat"}
	d := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	dp := segmentDatePrefix(prefix, d)
	if len(dp) != 4 {
		t.Fatalf("len = %d, want 4", len(dp))
	}
	if dp[2] != "seg" || dp[3] != "20250615" {
		t.Errorf("segmentDatePrefix = %v", dp)
	}
}

func TestParseSegmentKeyErrors(t *testing.T) {
	// Too short.
	_, err := parseSegmentKey(kv.Key{"a", "b"}, 2)
	if err == nil {
		t.Error("expected error for short key")
	}

	// Wrong segment marker.
	_, err = parseSegmentKey(kv.Key{"a", "b", "notSeg", "20250615", "123"}, 2)
	if err == nil {
		t.Error("expected error for wrong segment marker")
	}

	// Invalid timestamp.
	_, err = parseSegmentKey(kv.Key{"a", "b", "seg", "20250615", "notanumber"}, 2)
	if err == nil {
		t.Error("expected error for invalid timestamp")
	}
}

func TestStoreSegmentNoEmbedder(t *testing.T) {
	idx := newTestIndexNoVec(t)
	ctx := context.Background()

	seg := Segment{
		ID:        "s1",
		Summary:   "hello",
		Timestamp: time.Now().UnixNano(),
	}
	if err := idx.StoreSegment(ctx, seg); err != nil {
		t.Fatalf("StoreSegment without embedder: %v", err)
	}

	got, err := idx.GetSegment(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != "s1" {
		t.Errorf("expected s1, got %v", got)
	}
}

func TestGetSegmentNotFound(t *testing.T) {
	idx, _ := newTestIndex(t)
	ctx := context.Background()

	got, err := idx.GetSegment(ctx, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestRecentSegmentsZero(t *testing.T) {
	idx, _ := newTestIndex(t)
	ctx := context.Background()

	got, err := idx.RecentSegments(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil for n=0, got %v", got)
	}
}

func TestSearchSegmentsDefaultLimit(t *testing.T) {
	idx := newTestIndexNoVec(t)
	ctx := context.Background()

	// Store 15 segments, search with Limit=0 (should default to 10).
	for i := 0; i < 15; i++ {
		seg := Segment{
			ID:        fmt.Sprintf("s%d", i),
			Summary:   "hello",
			Keywords:  []string{"hello"},
			Timestamp: int64(i + 1),
		}
		if err := idx.StoreSegment(ctx, seg); err != nil {
			t.Fatal(err)
		}
	}

	results, err := idx.SearchSegments(ctx, SearchQuery{Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 10 {
		t.Errorf("expected 10 (default limit), got %d", len(results))
	}
}

func TestSearchSegmentsNoMatchesNoFilter(t *testing.T) {
	idx := newTestIndexNoVec(t)
	ctx := context.Background()

	// Store a segment with no keywords or labels.
	seg := Segment{ID: "s1", Summary: "hello", Timestamp: 1}
	if err := idx.StoreSegment(ctx, seg); err != nil {
		t.Fatal(err)
	}

	// Search with no text, no labels — should return segments (no filter active).
	results, err := idx.SearchSegments(ctx, SearchQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result with no filters, got %d", len(results))
	}
}

func TestSearchDefaultHopsAndLimit(t *testing.T) {
	idx, _ := newTestIndex(t)
	ctx := context.Background()

	seg := Segment{
		ID: "s1", Summary: "dinosaurs", Keywords: []string{"dinosaur"},
		Labels: []string{"Alice"}, Timestamp: 1,
	}
	if err := idx.StoreSegment(ctx, seg); err != nil {
		t.Fatal(err)
	}

	// Hops=0 and Limit=0 should use defaults (2 and 10).
	result, err := idx.Search(ctx, Query{
		Labels: []string{"Alice"},
		Text:   "dinosaurs",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Segments) == 0 {
		t.Error("expected results with default hops/limit")
	}
}

func TestMemVecCosineDistanceEdgeCases(t *testing.T) {
	// Dimension mismatch.
	d := cosineDistance([]float32{1, 0}, []float32{1, 0, 0})
	if d != 2 {
		t.Errorf("dimension mismatch: got %f, want 2", d)
	}

	// Zero vector.
	d = cosineDistance([]float32{0, 0, 0}, []float32{1, 0, 0})
	if d != 0 {
		t.Errorf("zero vector: got %f, want 0", d)
	}
}

func TestMemVecClose(t *testing.T) {
	vec := NewMemVec()
	if err := vec.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMemVecSearchEmpty(t *testing.T) {
	vec := NewMemVec()
	matches, err := vec.Search([]float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if matches != nil {
		t.Errorf("expected nil for empty index, got %v", matches)
	}
}

func TestMemVecDelete(t *testing.T) {
	vec := NewMemVec()
	_ = vec.Insert("a", []float32{1, 0})
	if vec.Len() != 1 {
		t.Fatalf("Len = %d, want 1", vec.Len())
	}
	_ = vec.Delete("a")
	if vec.Len() != 0 {
		t.Errorf("Len after delete = %d, want 0", vec.Len())
	}
	// Delete nonexistent should not error.
	if err := vec.Delete("nonexistent"); err != nil {
		t.Fatal(err)
	}
}

func TestTokenize(t *testing.T) {
	// Deduplication.
	got := tokenize("hello world hello")
	if len(got) != 2 {
		t.Errorf("expected 2 unique tokens, got %d: %v", len(got), got)
	}
	// Empty.
	if tokenize("") != nil {
		t.Error("expected nil for empty string")
	}
}

func TestKeywordScoreEmpty(t *testing.T) {
	// No query terms.
	if s := keywordScore(nil, []string{"foo"}); s != 0 {
		t.Errorf("expected 0, got %f", s)
	}
}

func TestLabelScoreEmpty(t *testing.T) {
	// No segment labels.
	set := map[string]struct{}{"a": {}}
	if s := labelScore(nil, set); s != 0 {
		t.Errorf("expected 0, got %f", s)
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkStoreSegment(b *testing.B) {
	store := kv.NewMemory(nil)
	emb := newMockEmbedder()
	vec := NewMemVec()
	idx := NewIndex(IndexConfig{
		Store: store, Embedder: emb, Vec: vec, Prefix: kv.Key{"bench"},
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seg := Segment{
			ID:        fmt.Sprintf("seg-%d", i),
			Summary:   "dinosaurs",
			Keywords:  []string{"dinosaur", "fossil"},
			Labels:    []string{"Alice", "dinosaurs"},
			Timestamp: int64(i),
		}
		if err := idx.StoreSegment(ctx, seg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearchSegments(b *testing.B) {
	store := kv.NewMemory(nil)
	emb := newMockEmbedder()
	vec := NewMemVec()
	idx := NewIndex(IndexConfig{
		Store: store, Embedder: emb, Vec: vec, Prefix: kv.Key{"bench"},
	})
	ctx := context.Background()

	// Pre-populate 500 segments.
	for i := 0; i < 500; i++ {
		seg := Segment{
			ID:        fmt.Sprintf("seg-%d", i),
			Summary:   "dinosaurs",
			Keywords:  []string{"dinosaur", "fossil", "museum"},
			Labels:    []string{"Alice", "Bob"},
			Timestamp: int64(i),
		}
		if err := idx.StoreSegment(ctx, seg); err != nil {
			b.Fatal(err)
		}
	}

	q := SearchQuery{
		Text:   "dinosaur fossils",
		Labels: []string{"Alice"},
		Limit:  10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.SearchSegments(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemVecSearch(b *testing.B) {
	vec := NewMemVec()
	// Insert 1000 random-ish vectors.
	for i := 0; i < 1000; i++ {
		v := []float32{
			float32(i%7) / 7.0,
			float32(i%11) / 11.0,
			float32(i%13) / 13.0,
			float32(i%17) / 17.0,
		}
		_ = vec.Insert(fmt.Sprintf("v-%d", i), v)
	}

	query := []float32{0.5, 0.5, 0.5, 0.5}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = vec.Search(query, 10)
	}
}

func BenchmarkSearchCombined(b *testing.B) {
	store := kv.NewMemory(nil)
	emb := newMockEmbedder()
	vec := NewMemVec()
	idx := NewIndex(IndexConfig{
		Store: store, Embedder: emb, Vec: vec, Prefix: kv.Key{"bench"},
	})
	ctx := context.Background()

	// Set up a small graph.
	g := idx.Graph()
	_ = g.SetEntity(ctx, graph.Entity{Label: "Alice"})
	_ = g.SetEntity(ctx, graph.Entity{Label: "Bob"})
	_ = g.SetEntity(ctx, graph.Entity{Label: "dinosaurs"})
	_ = g.AddRelation(ctx, graph.Relation{From: "Alice", To: "Bob", RelType: "knows"})
	_ = g.AddRelation(ctx, graph.Relation{From: "Bob", To: "dinosaurs", RelType: "likes"})

	// Pre-populate 200 segments.
	for i := 0; i < 200; i++ {
		labels := []string{"Alice"}
		if i%3 == 0 {
			labels = append(labels, "Bob")
		}
		if i%5 == 0 {
			labels = append(labels, "dinosaurs")
		}
		seg := Segment{
			ID:        fmt.Sprintf("seg-%d", i),
			Summary:   "dinosaurs",
			Keywords:  []string{"dinosaur", "fossil"},
			Labels:    labels,
			Timestamp: int64(i),
		}
		if err := idx.StoreSegment(ctx, seg); err != nil {
			b.Fatal(err)
		}
	}

	q := Query{
		Labels: []string{"Alice"},
		Text:   "dinosaurs",
		Hops:   2,
		Limit:  10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}
