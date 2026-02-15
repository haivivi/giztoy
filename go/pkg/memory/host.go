package memory

import (
	"context"
	"encoding/json"
	"fmt"
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
	//
	// The embedder's Model() and Dimension() are persisted on first use.
	// Subsequent calls to NewHost with a different model or dimension will
	// return an error to prevent mixing incompatible vector spaces.
	Embedder embed.Embedder

	// Compressor is the shared [Compressor] for LLM-based conversation
	// compression. Optional. If set, all personas opened from this host
	// use it as the default compressor in [Memory.Compress].
	//
	// Callers may still pass a per-call compressor to [Memory.Compress],
	// which takes precedence over this default.
	//
	// Use [NewLLMCompressor] to create an LLM-backed compressor that
	// delegates to registered segmentors and profilers.
	Compressor Compressor

	// CompressPolicy controls when auto-compression is triggered during
	// [Conversation.Append]. If zero, [DefaultCompressPolicy] is used.
	// Set both MaxChars and MaxMessages to 0 to disable auto-compression.
	CompressPolicy CompressPolicy

	// Separator is the KV key separator byte. It must match the Store's
	// configured separator. Labels (entity labels, segment labels) must not
	// contain this character.
	//
	// Zero means [kv.DefaultSeparator] (':'), which forbids ':' in labels.
	// For natural labels like "person:小明", use a non-printable separator
	// (e.g., '\x1F') and create the KV store with the same separator.
	Separator byte
}

// embedMeta is persisted in KV to track which embedding model was used.
// On subsequent opens, the host verifies that the current Embedder matches.
type embedMeta struct {
	Model     string `json:"model"`
	Dimension int    `json:"dim"`
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

// NewHost creates a new Host and validates the embedding model consistency.
//
// If an Embedder is provided, NewHost checks the KV store for previously
// persisted model metadata. If found and the model name or dimension differs,
// NewHost returns an error. If not found, it persists the current model info.
//
// This ensures that vectors stored with one model are never searched with
// another, which would produce meaningless similarity scores.
func NewHost(ctx context.Context, cfg HostConfig) (*Host, error) {
	if cfg.Store == nil {
		return nil, fmt.Errorf("memory: HostConfig.Store is required")
	}

	if cfg.Embedder != nil {
		if err := checkEmbedMeta(ctx, cfg.Store, cfg.Embedder); err != nil {
			return nil, err
		}
	}

	return &Host{
		cfg:      cfg,
		memories: make(map[string]*Memory),
	}, nil
}

// OpenOption configures per-persona overrides when opening a [Memory].
type OpenOption func(*openConfig)

type openConfig struct {
	compressor     Compressor
	compressPolicy *CompressPolicy
	embedder       embed.Embedder
}

// WithCompressor overrides the host-level [Compressor] for this persona.
func WithCompressor(c Compressor) OpenOption {
	return func(o *openConfig) { o.compressor = c }
}

// WithCompressPolicy overrides the host-level [CompressPolicy] for this persona.
func WithCompressPolicy(p CompressPolicy) OpenOption {
	return func(o *openConfig) { o.compressPolicy = &p }
}

// WithEmbedder overrides the host-level [embed.Embedder] for this persona.
// The embedder's Dimension must match the host's configured embedder (if any)
// to ensure vector compatibility; Open returns an error on mismatch.
func WithEmbedder(e embed.Embedder) OpenOption {
	return func(o *openConfig) { o.embedder = e }
}

// Open returns a Memory for a persona. Creates the underlying recall.Index
// if this is the first call for the given ID. Subsequent calls with the
// same ID return the same Memory instance (options are ignored for already
// opened personas).
//
// The id should be a stable, unique identifier for the persona (e.g.,
// "cat_girl", "robot_boy"). It is used as the KV key prefix.
//
// Options override host-level defaults for this persona only. Use
// [WithCompressor], [WithCompressPolicy], or [WithEmbedder] to customize.
func (h *Host) Open(id string, opts ...OpenOption) (*Memory, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if m, ok := h.memories[id]; ok {
		return m, nil
	}

	// Apply options.
	var oc openConfig
	for _, opt := range opts {
		opt(&oc)
	}

	// Resolve embedder.
	emb := h.cfg.Embedder
	if oc.embedder != nil {
		// Validate dimension compatibility.
		if emb != nil && oc.embedder.Dimension() != emb.Dimension() {
			return nil, fmt.Errorf(
				"memory: Open %q: embedder dimension mismatch: host=%d, per-persona=%d",
				id, emb.Dimension(), oc.embedder.Dimension(),
			)
		}
		emb = oc.embedder
	}

	// Resolve compressor.
	compressor := h.cfg.Compressor
	if oc.compressor != nil {
		compressor = oc.compressor
	}

	// Resolve compress policy.
	policy := h.cfg.CompressPolicy
	if oc.compressPolicy != nil {
		policy = *oc.compressPolicy
	}
	if policy.MaxChars == 0 && policy.MaxMessages == 0 && compressor != nil {
		// Apply default policy when a compressor is configured but no
		// explicit policy was set (zero value).
		policy = DefaultCompressPolicy()
	}

	idx := recall.NewIndex(recall.IndexConfig{
		Store:     h.cfg.Store,
		Embedder:  emb,
		Vec:       h.cfg.Vec,
		Prefix:    memPrefix(id),
		Separator: h.cfg.Separator,
	})

	m := newMemory(id, h.cfg.Store, idx, compressor, policy)
	h.memories[id] = m
	return m, nil
}

// Close releases all resources. After Close, the Host should not be used.
func (h *Host) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.memories = nil
	return nil
}

// checkEmbedMeta verifies embedding model consistency. On first call it
// persists the current model; on subsequent calls it validates the stored
// model matches.
func checkEmbedMeta(ctx context.Context, store kv.Store, emb embed.Embedder) error {
	key := hostMetaKey("embed")

	current := embedMeta{
		Model:     emb.Model(),
		Dimension: emb.Dimension(),
	}

	data, err := store.Get(ctx, key)
	if err != nil {
		if err == kv.ErrNotFound {
			// First time — persist the metadata.
			return writeEmbedMeta(ctx, store, key, current)
		}
		return fmt.Errorf("memory: read embed metadata: %w", err)
	}

	// Existing metadata found — verify it matches.
	var stored embedMeta
	if err := json.Unmarshal(data, &stored); err != nil {
		return fmt.Errorf("memory: decode embed metadata: %w", err)
	}

	if stored.Model != current.Model {
		return fmt.Errorf(
			"memory: embed model mismatch: stored %q (dim=%d), current %q (dim=%d); "+
				"vectors from different models are incompatible — "+
				"either use the same model or rebuild the index",
			stored.Model, stored.Dimension, current.Model, current.Dimension,
		)
	}

	if stored.Dimension != current.Dimension {
		return fmt.Errorf(
			"memory: embed dimension mismatch for model %q: stored dim=%d, current dim=%d; "+
				"changing dimension requires rebuilding the index",
			current.Model, stored.Dimension, current.Dimension,
		)
	}

	return nil
}

func writeEmbedMeta(ctx context.Context, store kv.Store, key kv.Key, meta embedMeta) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("memory: encode embed metadata: %w", err)
	}
	if err := store.Set(ctx, key, data); err != nil {
		return fmt.Errorf("memory: write embed metadata: %w", err)
	}
	return nil
}
