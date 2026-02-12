package vecid

import "sync"

// Store persists raw embeddings for re-clustering.
// Implementations must be safe for concurrent use.
//
// Use [NewMemoryStore] for in-memory storage (testing/ephemeral).
// For production, back with kv.Store or similar.
type Store interface {
	// Append stores an embedding. Returns a unique sequence ID.
	Append(emb []float32) (seq uint64, err error)

	// All returns all stored embeddings in insertion order.
	All() ([][]float32, error)

	// Len returns the count of stored embeddings.
	Len() (int, error)

	// Clear removes all stored embeddings.
	Clear() error
}

// MemoryStore is an in-memory Store implementation.
// Data is lost on restart. Suitable for testing or ephemeral use.
type MemoryStore struct {
	mu   sync.Mutex
	data [][]float32
	seq  uint64
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) Append(emb []float32) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]float32, len(emb))
	copy(cp, emb)
	s.data = append(s.data, cp)
	s.seq++
	return s.seq, nil
}

func (s *MemoryStore) All() ([][]float32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]float32, len(s.data))
	for i, v := range s.data {
		cp := make([]float32, len(v))
		copy(cp, v)
		out[i] = cp
	}
	return out, nil
}

func (s *MemoryStore) Len() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.data), nil
}

func (s *MemoryStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = nil
	s.seq = 0
	return nil
}
