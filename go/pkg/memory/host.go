package memory

import (
	"sync"

	"github.com/haivivi/giztoy/go/pkg/embed"
	"github.com/haivivi/giztoy/go/pkg/kv"
	"github.com/haivivi/giztoy/go/pkg/recall"
	"github.com/haivivi/giztoy/go/pkg/vecstore"
)

// HostConfig configures a [Host].
type HostConfig struct {
	// Store is the shared KV store. Required.
	// The store must be created with the same Separator as configured here.
	// Each persona's data is isolated under "mem{sep}{id}{sep}..." prefixes.
	Store kv.Store

	// Vec is the shared vector index. Optional.
	// If nil, semantic vector search is disabled.
	Vec vecstore.Index

	// Embedder converts text to vectors. Optional.
	// If nil, semantic vector search is disabled.
	Embedder embed.Embedder

	// Separator is the KV key separator byte. It must match the Store's
	// configured separator. Labels (entity labels, segment labels) must not
	// contain this character.
	//
	// Zero means [kv.DefaultSeparator] (':'), which forbids ':' in labels.
	// For natural labels like "person:小明", use a non-printable separator
	// (e.g., '\x00') and create the KV store with the same separator.
	Separator byte
}

// Host is the process-level entry point for the memory system.
// It manages Memory instances for many personas, all sharing a single
// KV store, vector index, and embedder.
//
// Host is safe for concurrent use. Multiple goroutines can call Open
// simultaneously for different persona IDs.
type Host struct {
	cfg HostConfig

	mu       sync.Mutex
	memories map[string]*Memory
}

// NewHost creates a new Host. The KV store is required; vec and embedder
// are optional (nil disables vector search).
func NewHost(cfg HostConfig) *Host {
	if cfg.Store == nil {
		panic("memory: HostConfig.Store is required")
	}
	return &Host{
		cfg:      cfg,
		memories: make(map[string]*Memory),
	}
}

// Open returns a Memory for a persona. Creates the underlying recall.Index
// if this is the first call for the given ID. Subsequent calls with the
// same ID return the same Memory instance.
//
// The id should be a stable, unique identifier for the persona (e.g.,
// "cat_girl", "robot_boy"). It is used as the KV key prefix.
func (h *Host) Open(id string) *Memory {
	h.mu.Lock()
	defer h.mu.Unlock()

	if m, ok := h.memories[id]; ok {
		return m
	}

	idx := recall.NewIndex(recall.IndexConfig{
		Store:     h.cfg.Store,
		Embedder:  h.cfg.Embedder,
		Vec:       h.cfg.Vec,
		Prefix:    memPrefix(id),
		Separator: h.cfg.Separator,
	})

	m := newMemory(id, h.cfg.Store, idx)
	h.memories[id] = m
	return m
}

// Close releases all resources. After Close, the Host should not be used.
func (h *Host) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.memories = nil
	return nil
}
