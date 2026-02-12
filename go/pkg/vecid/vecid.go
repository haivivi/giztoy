// Package vecid assigns stable string IDs to embedding vectors via
// online nearest-centroid matching and offline DBSCAN re-clustering.
//
// It is generic — works with any embedding type (voice, face, text, etc.).
//
// # Usage
//
//	reg := vecid.New(vecid.Config{Dim: 512, Threshold: 0.5, Prefix: "speaker"}, nil)
//
//	// Online: returns best guess (may be unmatched until first Recluster)
//	id, conf, ok := reg.Identify(embedding)
//
//	// Offline: re-cluster all stored embeddings for better accuracy
//	n := reg.Recluster()
//
//	// Now Identify is more accurate
//	id, conf, ok = reg.Identify(newEmbedding)
//
// # Design
//
// Identify does NOT create new buckets — only Recluster does. This avoids
// the greedy merge / chain drift problem where sequential merging causes
// order-dependent results and cluster quality degradation.
package vecid

import (
	"fmt"
	"sync"
)

// Config controls registry behavior.
type Config struct {
	// Dim is the embedding dimension (e.g. 512 for voice, 1536 for text).
	Dim int

	// Threshold is the minimum cosine similarity to match a bucket.
	// Lower = more lenient (more merges), higher = stricter (more unknowns).
	// Default: 0.5.
	Threshold float32

	// MinSamples is the minimum number of samples to form a cluster
	// in DBSCAN. Default: 2.
	MinSamples int

	// Prefix is prepended to generated IDs (e.g. "speaker" → "speaker:001").
	Prefix string
}

func (c *Config) defaults() {
	if c.Threshold == 0 {
		c.Threshold = 0.5
	}
	if c.MinSamples == 0 {
		c.MinSamples = 2
	}
}

// Bucket represents a cluster of similar embeddings.
type Bucket struct {
	// ID is the stable identifier (e.g. "speaker:001").
	ID string

	// Centroid is the L2-normalized mean embedding of this cluster.
	Centroid []float32

	// Count is the number of embeddings in this cluster.
	Count int
}

// Registry assigns stable IDs to embedding vectors.
type Registry struct {
	mu      sync.RWMutex
	cfg     Config
	store   Store
	buckets []Bucket
	nextID  int
}

// New creates a Registry. If store is nil, an in-memory store is used.
func New(cfg Config, store Store) *Registry {
	cfg.defaults()
	if store == nil {
		store = NewMemoryStore()
	}
	return &Registry{
		cfg:   cfg,
		store: store,
	}
}

// Identify returns the best matching bucket ID for the embedding.
// The embedding is always stored for future re-clustering.
//
// Returns:
//   - id: the matched bucket ID, or "" if no match
//   - confidence: cosine similarity to the matched bucket (0 if unmatched)
//   - matched: true if a bucket was found above threshold
func (r *Registry) Identify(emb []float32) (id string, confidence float32, matched bool) {
	// Always store for future re-clustering.
	r.store.Append(emb)

	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.buckets) == 0 {
		return "", 0, false
	}

	bestSim := float32(-1)
	bestIdx := -1
	for i, b := range r.buckets {
		sim := cosineSim(emb, b.Centroid)
		if sim > bestSim {
			bestSim = sim
			bestIdx = i
		}
	}

	if bestIdx >= 0 && bestSim >= r.cfg.Threshold {
		return r.buckets[bestIdx].ID, bestSim, true
	}
	return "", 0, false
}

// Recluster re-runs DBSCAN on all stored embeddings.
// Updates bucket centroids and tries to preserve existing IDs.
// Returns the number of clusters found.
func (r *Registry) Recluster() int {
	embeddings, err := r.store.All()
	if err != nil || len(embeddings) == 0 {
		return 0
	}

	// Snapshot config under read lock to avoid race with SetThreshold.
	r.mu.RLock()
	threshold := r.cfg.Threshold
	minSamples := r.cfg.MinSamples
	dim := r.cfg.Dim
	r.mu.RUnlock()

	// L2-normalize all embeddings for cosine distance.
	normed := make([][]float32, len(embeddings))
	for i, emb := range embeddings {
		cp := make([]float32, len(emb))
		copy(cp, emb)
		l2Norm(cp)
		normed[i] = cp
	}

	// DBSCAN: eps = 1 - threshold (cosine distance).
	eps := 1.0 - threshold
	labels := dbscan(normed, float32(eps), minSamples)

	// Find max cluster label.
	maxLabel := 0
	for _, l := range labels {
		if l > maxLabel {
			maxLabel = l
		}
	}

	// Compute centroids for each cluster.
	newBuckets := make([]Bucket, 0, maxLabel)
	for c := 1; c <= maxLabel; c++ {
		centroid := make([]float32, dim)
		count := 0
		for i, l := range labels {
			if l == c {
				for d := range centroid {
					if d < len(normed[i]) {
						centroid[d] += normed[i][d]
					}
				}
				count++
			}
		}
		if count == 0 {
			continue
		}
		for d := range centroid {
			centroid[d] /= float32(count)
		}
		l2Norm(centroid)
		newBuckets = append(newBuckets, Bucket{
			Centroid: centroid,
			Count:    count,
		})
	}

	// Assign IDs: match new buckets to old ones by centroid similarity.
	r.mu.Lock()
	defer r.mu.Unlock()

	oldBuckets := r.buckets
	usedOldIDs := make(map[string]bool)

	for i := range newBuckets {
		bestSim := float32(-1)
		bestOldIdx := -1
		for j, old := range oldBuckets {
			if usedOldIDs[old.ID] {
				continue
			}
			sim := cosineSim(newBuckets[i].Centroid, old.Centroid)
			if sim > bestSim {
				bestSim = sim
				bestOldIdx = j
			}
		}
		if bestOldIdx >= 0 && bestSim >= threshold {
			// Reuse old ID.
			newBuckets[i].ID = oldBuckets[bestOldIdx].ID
			usedOldIDs[oldBuckets[bestOldIdx].ID] = true
		} else {
			// Assign new ID.
			newBuckets[i].ID = r.allocID()
		}
	}

	r.buckets = newBuckets
	return len(newBuckets)
}

// Buckets returns all current buckets.
func (r *Registry) Buckets() []Bucket {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Bucket, len(r.buckets))
	copy(out, r.buckets)
	return out
}

// BucketOf returns the bucket for a given ID, or nil if not found.
func (r *Registry) BucketOf(id string) *Bucket {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, b := range r.buckets {
		if b.ID == id {
			cp := b
			return &cp
		}
	}
	return nil
}

// SetThreshold adjusts matching strictness at runtime.
func (r *Registry) SetThreshold(t float32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cfg.Threshold = t
}

// Len returns the number of stored embeddings.
func (r *Registry) Len() int {
	n, _ := r.store.Len()
	return n
}

// Reset clears all stored embeddings and buckets.
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store.Clear()
	r.buckets = nil
	r.nextID = 0
}

// allocID generates the next ID. Must be called with mu held.
func (r *Registry) allocID() string {
	r.nextID++
	if r.cfg.Prefix != "" {
		return fmt.Sprintf("%s:%03d", r.cfg.Prefix, r.nextID)
	}
	return fmt.Sprintf("%03d", r.nextID)
}
