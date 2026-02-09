package vecstore

import (
	"bytes"
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestHNSW creates an HNSW index with small parameters for fast tests.
func newTestHNSW(dim int) *HNSW {
	return NewHNSW(HNSWConfig{
		Dim:            dim,
		M:              8,
		EfConstruction: 64,
		EfSearch:       32,
	})
}

// randVec generates a random unit vector of the given dimension using rng.
func randVec(rng *rand.Rand, dim int) []float32 {
	v := make([]float32, dim)
	var norm float64
	for i := range v {
		x := float32(rng.NormFloat64())
		v[i] = x
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range v {
			v[i] /= float32(norm)
		}
	}
	return v
}

// bruteForceSearch returns the top-k IDs by brute-force cosine distance.
func bruteForceSearch(ids []string, vecs [][]float32, query []float32, topK int) []string {
	type scored struct {
		id   string
		dist float32
	}
	results := make([]scored, len(ids))
	for i, id := range ids {
		results[i] = scored{id: id, dist: CosineDistance(query, vecs[i])}
	}
	// Simple selection sort for small k — good enough for tests.
	for i := 0; i < topK && i < len(results); i++ {
		best := i
		for j := i + 1; j < len(results); j++ {
			if results[j].dist < results[best].dist {
				best = j
			}
		}
		results[i], results[best] = results[best], results[i]
	}
	n := topK
	if n > len(results) {
		n = len(results)
	}
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = results[i].id
	}
	return out
}

// ---------------------------------------------------------------------------
// Unit tests
// ---------------------------------------------------------------------------

func TestHNSWInsertAndSearch(t *testing.T) {
	h := newTestHNSW(4)

	_ = h.Insert("a", []float32{1, 0, 0, 0})
	_ = h.Insert("b", []float32{0, 1, 0, 0})
	_ = h.Insert("c", []float32{0.9, 0.1, 0, 0})

	matches, err := h.Search([]float32{1, 0, 0, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].ID != "a" {
		t.Errorf("top match = %q, want 'a'", matches[0].ID)
	}
	if matches[1].ID != "c" {
		t.Errorf("second match = %q, want 'c'", matches[1].ID)
	}
}

func TestHNSWBatchInsert(t *testing.T) {
	h := newTestHNSW(3)

	ids := []string{"a", "b", "c"}
	vecs := [][]float32{
		{1, 0, 0},
		{0, 1, 0},
		{0, 0, 1},
	}
	if err := h.BatchInsert(ids, vecs); err != nil {
		t.Fatal(err)
	}
	if h.Len() != 3 {
		t.Errorf("Len = %d, want 3", h.Len())
	}

	matches, err := h.Search([]float32{1, 0, 0}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != "a" {
		t.Errorf("expected match 'a', got %v", matches)
	}
}

func TestHNSWBatchInsertMismatch(t *testing.T) {
	h := newTestHNSW(3)
	err := h.BatchInsert([]string{"a", "b"}, [][]float32{{1, 0, 0}})
	if err == nil {
		t.Fatal("expected error for mismatched lengths")
	}
}

func TestHNSWDimensionMismatch(t *testing.T) {
	h := newTestHNSW(4)

	if err := h.Insert("a", []float32{1, 0, 0}); err == nil {
		t.Error("expected error for wrong dimension on Insert")
	}

	_ = h.Insert("b", []float32{1, 0, 0, 0})
	if _, err := h.Search([]float32{1, 0}, 1); err == nil {
		t.Error("expected error for wrong dimension on Search")
	}
}

func TestHNSWDelete(t *testing.T) {
	h := newTestHNSW(3)

	_ = h.Insert("a", []float32{1, 0, 0})
	_ = h.Insert("b", []float32{0, 1, 0})
	_ = h.Insert("c", []float32{0, 0, 1})

	if h.Len() != 3 {
		t.Fatalf("Len = %d, want 3", h.Len())
	}

	_ = h.Delete("b")
	if h.Len() != 2 {
		t.Errorf("Len after delete = %d, want 2", h.Len())
	}

	// Search should not return the deleted vector.
	matches, err := h.Search([]float32{0, 1, 0}, 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range matches {
		if m.ID == "b" {
			t.Error("deleted vector 'b' still returned in search")
		}
	}

	// Delete nonexistent — no error.
	if err := h.Delete("nonexistent"); err != nil {
		t.Fatal(err)
	}
}

func TestHNSWDeleteEntryPoint(t *testing.T) {
	h := newTestHNSW(3)

	_ = h.Insert("a", []float32{1, 0, 0})
	_ = h.Insert("b", []float32{0, 1, 0})

	// Delete both and verify the index becomes empty.
	_ = h.Delete("a")
	_ = h.Delete("b")
	if h.Len() != 0 {
		t.Fatalf("Len = %d, want 0", h.Len())
	}

	// Insert again after emptying.
	_ = h.Insert("c", []float32{0, 0, 1})
	matches, err := h.Search([]float32{0, 0, 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != "c" {
		t.Errorf("expected match 'c', got %v", matches)
	}
}

func TestHNSWUpdateExisting(t *testing.T) {
	h := newTestHNSW(3)

	_ = h.Insert("a", []float32{1, 0, 0})
	_ = h.Insert("b", []float32{0, 1, 0})

	// Update "a" to a new vector.
	_ = h.Insert("a", []float32{0, 0, 1})

	if h.Len() != 2 {
		t.Fatalf("Len = %d, want 2 (update should not increase count)", h.Len())
	}

	matches, err := h.Search([]float32{0, 0, 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != "a" {
		t.Errorf("expected updated 'a', got %v", matches)
	}
}

func TestHNSWSearchEmpty(t *testing.T) {
	h := newTestHNSW(3)
	matches, err := h.Search([]float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if matches != nil {
		t.Errorf("expected nil for empty index, got %v", matches)
	}
}

func TestHNSWSearchTopKZero(t *testing.T) {
	h := newTestHNSW(3)
	_ = h.Insert("a", []float32{1, 0, 0})

	matches, err := h.Search([]float32{1, 0, 0}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if matches != nil {
		t.Errorf("expected nil for topK=0, got %v", matches)
	}
}

func TestHNSWSingleNode(t *testing.T) {
	h := newTestHNSW(3)

	_ = h.Insert("only", []float32{0.5, 0.5, 0.5})
	matches, err := h.Search([]float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != "only" {
		t.Errorf("expected single match 'only', got %v", matches)
	}
}

func TestHNSWFlush(t *testing.T) {
	if err := newTestHNSW(3).Flush(); err != nil {
		t.Fatal(err)
	}
}

func TestHNSWClose(t *testing.T) {
	if err := newTestHNSW(3).Close(); err != nil {
		t.Fatal(err)
	}
}

func TestHNSWSetEfSearch(t *testing.T) {
	h := newTestHNSW(3)
	h.SetEfSearch(200)

	h.mu.RLock()
	ef := h.cfg.EfSearch
	h.mu.RUnlock()

	if ef != 200 {
		t.Errorf("EfSearch = %d, want 200", ef)
	}
}

func TestNewHNSWPanicsOnZeroDim(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for Dim=0")
		}
	}()
	NewHNSW(HNSWConfig{Dim: 0})
}

// ---------------------------------------------------------------------------
// Save / Load
// ---------------------------------------------------------------------------

func TestHNSWSaveLoad(t *testing.T) {
	h := newTestHNSW(4)

	_ = h.Insert("a", []float32{1, 0, 0, 0})
	_ = h.Insert("b", []float32{0, 1, 0, 0})
	_ = h.Insert("c", []float32{0, 0, 1, 0})
	_ = h.Delete("b")

	// Save.
	var buf bytes.Buffer
	if err := h.Save(&buf); err != nil {
		t.Fatal(err)
	}

	// Load.
	h2, err := LoadHNSW(&buf)
	if err != nil {
		t.Fatal(err)
	}

	// Verify metadata.
	if h2.Len() != h.Len() {
		t.Errorf("loaded Len = %d, want %d", h2.Len(), h.Len())
	}
	if h2.cfg.Dim != h.cfg.Dim {
		t.Errorf("loaded Dim = %d, want %d", h2.cfg.Dim, h.cfg.Dim)
	}

	// Verify search results match.
	query := []float32{1, 0, 0, 0}
	m1, _ := h.Search(query, 2)
	m2, _ := h2.Search(query, 2)

	if len(m1) != len(m2) {
		t.Fatalf("result count mismatch: original %d, loaded %d", len(m1), len(m2))
	}
	for i := range m1 {
		if m1[i].ID != m2[i].ID {
			t.Errorf("result[%d]: original %q, loaded %q", i, m1[i].ID, m2[i].ID)
		}
	}

	// Verify we can insert into the loaded index.
	if err := h2.Insert("d", []float32{0, 0, 0, 1}); err != nil {
		t.Fatal(err)
	}
	if h2.Len() != 3 {
		t.Errorf("Len after insert into loaded = %d, want 3", h2.Len())
	}
}

func TestHNSWSaveLoadEmpty(t *testing.T) {
	h := newTestHNSW(4)

	var buf bytes.Buffer
	if err := h.Save(&buf); err != nil {
		t.Fatal(err)
	}

	h2, err := LoadHNSW(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if h2.Len() != 0 {
		t.Errorf("loaded empty Len = %d, want 0", h2.Len())
	}

	// Verify it's usable.
	if err := h2.Insert("a", []float32{1, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
}

func TestLoadHNSWInvalidMagic(t *testing.T) {
	_, err := LoadHNSW(bytes.NewReader([]byte("NOPE")))
	if err == nil {
		t.Error("expected error for invalid magic")
	}
}

// ---------------------------------------------------------------------------
// Recall quality
// ---------------------------------------------------------------------------

func TestHNSWRecall(t *testing.T) {
	const (
		dim     = 32
		n       = 2000
		queries = 50
		topK    = 10
	)

	rng := rand.New(rand.NewPCG(42, 99))

	// Build index and keep a copy for brute-force.
	h := NewHNSW(HNSWConfig{
		Dim:            dim,
		M:              16,
		EfConstruction: 128,
		EfSearch:       64,
	})

	ids := make([]string, n)
	vecs := make([][]float32, n)
	for i := 0; i < n; i++ {
		ids[i] = fmt.Sprintf("v-%d", i)
		vecs[i] = randVec(rng, dim)
		if err := h.Insert(ids[i], vecs[i]); err != nil {
			t.Fatal(err)
		}
	}

	// Measure recall over random queries.
	totalRecall := 0.0
	for q := 0; q < queries; q++ {
		query := randVec(rng, dim)

		// Brute-force ground truth.
		truth := bruteForceSearch(ids, vecs, query, topK)
		truthSet := make(map[string]struct{}, topK)
		for _, id := range truth {
			truthSet[id] = struct{}{}
		}

		// HNSW result.
		matches, err := h.Search(query, topK)
		if err != nil {
			t.Fatal(err)
		}

		// Count hits.
		hits := 0
		for _, m := range matches {
			if _, ok := truthSet[m.ID]; ok {
				hits++
			}
		}
		totalRecall += float64(hits) / float64(topK)
	}

	avgRecall := totalRecall / float64(queries)
	t.Logf("average recall@%d over %d queries on %d vectors: %.3f", topK, queries, n, avgRecall)

	if avgRecall < 0.80 {
		t.Errorf("recall %.3f is below 0.80 threshold", avgRecall)
	}
}

// ---------------------------------------------------------------------------
// Concurrency
// ---------------------------------------------------------------------------

func TestHNSWConcurrent(t *testing.T) {
	const (
		dim        = 16
		numInserts = 200
		numSearches = 100
	)

	h := newTestHNSW(dim)
	rng := rand.New(rand.NewPCG(7, 13))

	// Pre-insert some vectors.
	for i := 0; i < 50; i++ {
		_ = h.Insert(fmt.Sprintf("pre-%d", i), randVec(rng, dim))
	}

	var wg sync.WaitGroup

	// Concurrent inserts.
	wg.Add(numInserts)
	for i := 0; i < numInserts; i++ {
		go func(i int) {
			defer wg.Done()
			localRng := rand.New(rand.NewPCG(uint64(i)*17, uint64(i)*31))
			_ = h.Insert(fmt.Sprintf("ins-%d", i), randVec(localRng, dim))
		}(i)
	}

	// Concurrent searches.
	wg.Add(numSearches)
	for i := 0; i < numSearches; i++ {
		go func(i int) {
			defer wg.Done()
			localRng := rand.New(rand.NewPCG(uint64(i)*41, uint64(i)*53))
			_, _ = h.Search(randVec(localRng, dim), 5)
		}(i)
	}

	// Concurrent deletes.
	wg.Add(20)
	for i := 0; i < 20; i++ {
		go func(i int) {
			defer wg.Done()
			_ = h.Delete(fmt.Sprintf("pre-%d", i))
		}(i)
	}

	wg.Wait()

	// Sanity check: the index should still work.
	_, err := h.Search(randVec(rng, dim), 5)
	if err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkHNSWInsert(b *testing.B) {
	const dim = 128
	rng := rand.New(rand.NewPCG(1, 2))

	// Pre-build an index with 1000 vectors.
	h := NewHNSW(HNSWConfig{Dim: dim, M: 16, EfConstruction: 100})
	for i := 0; i < 1000; i++ {
		_ = h.Insert(fmt.Sprintf("pre-%d", i), randVec(rng, dim))
	}

	// Benchmark inserting additional vectors.
	vecs := make([][]float32, b.N)
	for i := range vecs {
		vecs[i] = randVec(rng, dim)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.Insert(fmt.Sprintf("bench-%d", i), vecs[i])
	}
}

func BenchmarkHNSWSearch(b *testing.B) {
	const dim = 128
	rng := rand.New(rand.NewPCG(3, 4))

	h := NewHNSW(HNSWConfig{Dim: dim, M: 16, EfConstruction: 200, EfSearch: 50})
	for i := 0; i < 10000; i++ {
		_ = h.Insert(fmt.Sprintf("v-%d", i), randVec(rng, dim))
	}

	queries := make([][]float32, 1000)
	for i := range queries {
		queries[i] = randVec(rng, dim)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h.Search(queries[i%len(queries)], 10)
	}
}

func BenchmarkHNSWSearch_VaryEf(b *testing.B) {
	const dim = 128
	rng := rand.New(rand.NewPCG(5, 6))

	h := NewHNSW(HNSWConfig{Dim: dim, M: 16, EfConstruction: 200})
	for i := 0; i < 10000; i++ {
		_ = h.Insert(fmt.Sprintf("v-%d", i), randVec(rng, dim))
	}

	queries := make([][]float32, 100)
	for i := range queries {
		queries[i] = randVec(rng, dim)
	}

	for _, ef := range []int{10, 50, 100, 200} {
		b.Run(fmt.Sprintf("ef=%d", ef), func(b *testing.B) {
			h.SetEfSearch(ef)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = h.Search(queries[i%len(queries)], 10)
			}
		})
	}
}

func BenchmarkHNSWSaveLoad(b *testing.B) {
	const dim = 128
	rng := rand.New(rand.NewPCG(7, 8))

	h := NewHNSW(HNSWConfig{Dim: dim, M: 16, EfConstruction: 100})
	for i := 0; i < 5000; i++ {
		_ = h.Insert(fmt.Sprintf("v-%d", i), randVec(rng, dim))
	}

	b.Run("Save", func(b *testing.B) {
		var buf bytes.Buffer
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			_ = h.Save(&buf)
		}
		b.SetBytes(int64(buf.Len()))
	})

	var saved bytes.Buffer
	_ = h.Save(&saved)
	data := saved.Bytes()

	b.Run("Load", func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = LoadHNSW(bytes.NewReader(data))
		}
	})
}
