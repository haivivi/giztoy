// Package vecstore provides a vector approximate nearest-neighbor (ANN)
// search interface and implementations.
//
// The [Index] interface defines the contract for vector storage and search.
// Implementations include an in-memory brute-force index for testing
// ([NewMemory]) and HNSW for production use (planned).
//
// This package follows the same pattern as [kv]: a generic interface with
// pluggable backends. For all-in-one deployment, use the built-in
// implementations. For distributed deployment, swap in a client that
// talks to Milvus, Qdrant, or similar.
package vecstore

// Index is the interface for approximate nearest-neighbor search over
// dense float32 vectors.
//
// All implementations must be safe for concurrent use.
type Index interface {
	// Insert adds or updates a vector with the given ID.
	Insert(id string, vector []float32) error

	// BatchInsert adds or updates multiple vectors at once.
	// ids and vectors must have the same length.
	BatchInsert(ids []string, vectors [][]float32) error

	// Search returns the top-k nearest vectors to the query.
	// Results are ordered by ascending distance (closest first).
	Search(query []float32, topK int) ([]Match, error)

	// Delete removes a vector by ID. No error if ID does not exist.
	Delete(id string) error

	// Len returns the number of vectors in the index.
	Len() int

	// Flush ensures all pending writes are visible to subsequent searches.
	// For in-memory implementations this is typically a no-op.
	Flush() error

	// Close releases resources held by the index.
	Close() error
}

// Match is a single result from a vector similarity search.
type Match struct {
	// ID is the identifier of the matched vector.
	ID string

	// Distance is the distance between the query and matched vector.
	// Lower values indicate higher similarity.
	Distance float32
}
