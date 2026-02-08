package recall

import (
	"math"
	"sort"
	"sync"
)

// MemVec is an in-memory VectorIndex implementation using brute-force
// cosine distance. Intended for testing and small-scale use.
//
// It is safe for concurrent use.
type MemVec struct {
	mu      sync.RWMutex
	vectors map[string][]float32
}

// NewMemVec creates a new in-memory vector index.
func NewMemVec() *MemVec {
	return &MemVec{
		vectors: make(map[string][]float32),
	}
}

func (m *MemVec) Insert(id string, vector []float32) error {
	cp := make([]float32, len(vector))
	copy(cp, vector)
	m.mu.Lock()
	m.vectors[id] = cp
	m.mu.Unlock()
	return nil
}

func (m *MemVec) BatchInsert(ids []string, vectors [][]float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, id := range ids {
		cp := make([]float32, len(vectors[i]))
		copy(cp, vectors[i])
		m.vectors[id] = cp
	}
	return nil
}

func (m *MemVec) Flush() error {
	return nil // in-memory: always visible
}

func (m *MemVec) Search(query []float32, topK int) ([]VectorMatch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.vectors) == 0 || topK <= 0 {
		return nil, nil
	}

	type scored struct {
		id   string
		dist float32
	}
	results := make([]scored, 0, len(m.vectors))
	for id, vec := range m.vectors {
		d := cosineDistance(query, vec)
		results = append(results, scored{id: id, dist: d})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].dist < results[j].dist
	})

	if len(results) > topK {
		results = results[:topK]
	}

	matches := make([]VectorMatch, len(results))
	for i, r := range results {
		matches[i] = VectorMatch{ID: r.id, Distance: r.dist}
	}
	return matches, nil
}

func (m *MemVec) Delete(id string) error {
	m.mu.Lock()
	delete(m.vectors, id)
	m.mu.Unlock()
	return nil
}

func (m *MemVec) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.vectors)
}

func (m *MemVec) Close() error {
	return nil
}

// cosineDistance computes the cosine distance between two vectors.
// Returns a value in [0, 2] where 0 means identical direction and
// 2 means opposite direction. Returns 0 if either vector has zero norm.
func cosineDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		return 2 // maximum distance for mismatched dimensions
	}

	var dot, normA, normB float64
	for i := range a {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	similarity := dot / (math.Sqrt(normA) * math.Sqrt(normB))
	// Clamp to [-1, 1] to handle floating point errors.
	if similarity > 1 {
		similarity = 1
	}
	if similarity < -1 {
		similarity = -1
	}
	return float32(1 - similarity)
}
