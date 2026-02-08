package recall

import (
	"github.com/haivivi/giztoy/go/pkg/embed"
	"github.com/haivivi/giztoy/go/pkg/graph"
	"github.com/haivivi/giztoy/go/pkg/kv"
)

// IndexConfig configures a new [Index].
type IndexConfig struct {
	// Store is the KV store used for segment and graph data.
	// Required.
	Store kv.Store

	// Embedder converts text to vectors for semantic search.
	// Optional: if nil, vector search is disabled and only keyword +
	// label matching is used.
	Embedder embed.Embedder

	// Vec is the vector index for approximate nearest-neighbor search.
	// Optional: if nil, vector search is disabled.
	Vec VectorIndex

	// Prefix is the KV key prefix that scopes all data for this index.
	// For example, kv.Key{"mem", "cat_girl"} isolates one persona's data.
	// All segment keys are stored under {Prefix}:seg:... and graph keys
	// under {Prefix}:g:...
	Prefix kv.Key
}

// Index is a single search space combining segment storage, an entity-relation
// graph, and multi-signal search (vector + keyword + label).
//
// Each Index is scoped under a KV prefix, allowing many independent indexes
// to share a single KV store. The upper layer (memory, knowledge, news)
// decides the prefix and isolation model.
type Index struct {
	store    kv.Store
	embedder embed.Embedder
	vec      VectorIndex
	graph    graph.Graph
	prefix   kv.Key
}

// NewIndex creates a new Index with the given configuration.
// The KV store is required; embedder and vec are optional (nil disables
// vector search).
//
// The graph is automatically created as a KVGraph scoped under
// {prefix}:g using the same KV store.
func NewIndex(cfg IndexConfig) *Index {
	return &Index{
		store:    cfg.Store,
		embedder: cfg.Embedder,
		vec:      cfg.Vec,
		graph:    graph.NewKVGraph(cfg.Store, graphPrefix(cfg.Prefix)),
		prefix:   cfg.Prefix,
	}
}

// Graph returns the entity-relation graph for this index.
// The graph is backed by the same KV store, scoped under {prefix}:g.
func (idx *Index) Graph() graph.Graph {
	return idx.graph
}
