package graph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"sort"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/kv"
)

// KV key layout (relative to the configured prefix):
//
//	{prefix}:e:{label}                  → JSON-encoded Entity.Attrs
//	{prefix}:r:{from}:{relType}:{to}   → empty (forward index)
//	{prefix}:ri:{to}:{relType}:{from}  → empty (reverse index)

// KVGraph is a Graph implementation backed by a kv.Store.
// All keys are scoped under a configurable prefix, allowing multiple
// independent graphs to share a single KV store.
type KVGraph struct {
	store  kv.Store
	prefix kv.Key
}

// NewKVGraph creates a new KVGraph using the given store and key prefix.
// The prefix is prepended to all keys, e.g. prefix = {"mem", "123", "g"}
// results in entity keys like "mem:123:g:e:Alice".
func NewKVGraph(store kv.Store, prefix kv.Key) *KVGraph {
	return &KVGraph{store: store, prefix: prefix}
}

// validateSegments checks that none of the given strings contain the KV
// separator character. Labels and relation types are used as kv.Key segments;
// if they contain the separator the encoded key would be corrupted.
func (g *KVGraph) validateSegments(segs ...string) error {
	sep := string(kv.DefaultSeparator)
	for _, s := range segs {
		if strings.Contains(s, sep) {
			return fmt.Errorf("%w: %q contains %q", ErrInvalidLabel, s, sep)
		}
	}
	return nil
}

// --- key helpers ---

func (g *KVGraph) entityKey(label string) kv.Key {
	k := make(kv.Key, len(g.prefix)+2)
	copy(k, g.prefix)
	k[len(g.prefix)] = "e"
	k[len(g.prefix)+1] = label
	return k
}

func (g *KVGraph) entityPrefix() kv.Key {
	k := make(kv.Key, len(g.prefix)+1)
	copy(k, g.prefix)
	k[len(g.prefix)] = "e"
	return k
}

func (g *KVGraph) fwdKey(from, relType, to string) kv.Key {
	k := make(kv.Key, len(g.prefix)+4)
	copy(k, g.prefix)
	k[len(g.prefix)] = "r"
	k[len(g.prefix)+1] = from
	k[len(g.prefix)+2] = relType
	k[len(g.prefix)+3] = to
	return k
}

func (g *KVGraph) fwdPrefix(from string) kv.Key {
	k := make(kv.Key, len(g.prefix)+2)
	copy(k, g.prefix)
	k[len(g.prefix)] = "r"
	k[len(g.prefix)+1] = from
	return k
}

func (g *KVGraph) revKey(to, relType, from string) kv.Key {
	k := make(kv.Key, len(g.prefix)+4)
	copy(k, g.prefix)
	k[len(g.prefix)] = "ri"
	k[len(g.prefix)+1] = to
	k[len(g.prefix)+2] = relType
	k[len(g.prefix)+3] = from
	return k
}

func (g *KVGraph) revPrefix(to string) kv.Key {
	k := make(kv.Key, len(g.prefix)+2)
	copy(k, g.prefix)
	k[len(g.prefix)] = "ri"
	k[len(g.prefix)+1] = to
	return k
}

// --- Entity operations ---

func (g *KVGraph) GetEntity(ctx context.Context, label string) (*Entity, error) {
	if err := g.validateSegments(label); err != nil {
		return nil, err
	}
	data, err := g.store.Get(ctx, g.entityKey(label))
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	e := &Entity{Label: label}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &e.Attrs); err != nil {
			return nil, err
		}
	}
	return e, nil
}

func (g *KVGraph) SetEntity(ctx context.Context, e Entity) error {
	if err := g.validateSegments(e.Label); err != nil {
		return err
	}
	data, err := json.Marshal(e.Attrs)
	if err != nil {
		return err
	}
	return g.store.Set(ctx, g.entityKey(e.Label), data)
}

func (g *KVGraph) DeleteEntity(ctx context.Context, label string) error {
	if err := g.validateSegments(label); err != nil {
		return err
	}
	// Collect all relation keys involving this entity.
	rels, err := g.Relations(ctx, label)
	if err != nil {
		return err
	}

	// Build a list of all keys to delete: entity + relation pairs.
	keys := make([]kv.Key, 0, 1+len(rels)*2)
	keys = append(keys, g.entityKey(label))
	for _, r := range rels {
		keys = append(keys, g.fwdKey(r.From, r.RelType, r.To))
		keys = append(keys, g.revKey(r.To, r.RelType, r.From))
	}

	return g.store.BatchDelete(ctx, keys)
}

func (g *KVGraph) MergeAttrs(ctx context.Context, label string, attrs map[string]any) error {
	if err := g.validateSegments(label); err != nil {
		return err
	}
	e, err := g.GetEntity(ctx, label)
	if err != nil {
		return err
	}
	if e.Attrs == nil {
		e.Attrs = make(map[string]any, len(attrs))
	}
	for k, v := range attrs {
		e.Attrs[k] = v
	}
	return g.SetEntity(ctx, *e)
}

func (g *KVGraph) ListEntities(ctx context.Context, prefix string) iter.Seq2[Entity, error] {
	// Always scan all entities and filter client-side. We cannot use a
	// more specific KV prefix because kv.List appends a separator, so
	// "e:Ali" would match "e:Ali:*" but not "e:Alice". Entity keys have
	// exactly one segment after "e", so client-side filtering is correct.
	kvPrefix := g.entityPrefix()

	return func(yield func(Entity, error) bool) {
		for entry, err := range g.store.List(ctx, kvPrefix) {
			if err != nil {
				yield(Entity{}, err)
				return
			}
			// Extract label: last segment of the key.
			label := entry.Key[len(entry.Key)-1]

			// Filter by prefix if requested.
			if prefix != "" && !hasPrefix(label, prefix) {
				continue
			}

			e := Entity{Label: label}
			if len(entry.Value) > 0 {
				if err := json.Unmarshal(entry.Value, &e.Attrs); err != nil {
					if !yield(Entity{}, err) {
						return
					}
					continue
				}
			}
			if !yield(e, nil) {
				return
			}
		}
	}
}

// hasPrefix checks if s starts with prefix.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// --- Relation operations ---

func (g *KVGraph) AddRelation(ctx context.Context, r Relation) error {
	if err := g.validateSegments(r.From, r.To, r.RelType); err != nil {
		return err
	}
	return g.store.BatchSet(ctx, []kv.Entry{
		{Key: g.fwdKey(r.From, r.RelType, r.To), Value: nil},
		{Key: g.revKey(r.To, r.RelType, r.From), Value: nil},
	})
}

func (g *KVGraph) RemoveRelation(ctx context.Context, from, to, relType string) error {
	if err := g.validateSegments(from, to, relType); err != nil {
		return err
	}
	return g.store.BatchDelete(ctx, []kv.Key{
		g.fwdKey(from, relType, to),
		g.revKey(to, relType, from),
	})
}

func (g *KVGraph) Relations(ctx context.Context, label string) ([]Relation, error) {
	if err := g.validateSegments(label); err != nil {
		return nil, err
	}
	var rels []Relation

	// Forward: relations where label is the source.
	for entry, err := range g.store.List(ctx, g.fwdPrefix(label)) {
		if err != nil {
			return nil, err
		}
		// Key: {prefix}:r:{from}:{relType}:{to}
		k := entry.Key
		plen := len(g.prefix)
		if len(k) != plen+4 {
			continue // malformed key, skip
		}
		rels = append(rels, Relation{
			From:    k[plen+1],
			RelType: k[plen+2],
			To:      k[plen+3],
		})
	}

	// Reverse: relations where label is the target.
	for entry, err := range g.store.List(ctx, g.revPrefix(label)) {
		if err != nil {
			return nil, err
		}
		// Key: {prefix}:ri:{to}:{relType}:{from}
		k := entry.Key
		plen := len(g.prefix)
		if len(k) != plen+4 {
			continue
		}
		from := k[plen+3]
		// Skip self-loops: already captured by the forward scan above.
		if from == label {
			continue
		}
		rels = append(rels, Relation{
			From:    from,
			RelType: k[plen+2],
			To:      k[plen+1],
		})
	}

	return rels, nil
}

// --- Traversal ---

func (g *KVGraph) Neighbors(ctx context.Context, label string, relTypes ...string) ([]string, error) {
	if err := g.validateSegments(label); err != nil {
		return nil, err
	}
	if err := g.validateSegments(relTypes...); err != nil {
		return nil, err
	}
	typeSet := make(map[string]struct{}, len(relTypes))
	for _, rt := range relTypes {
		typeSet[rt] = struct{}{}
	}
	filterType := len(typeSet) > 0

	seen := make(map[string]struct{})

	// Outgoing neighbors.
	for entry, err := range g.store.List(ctx, g.fwdPrefix(label)) {
		if err != nil {
			return nil, err
		}
		k := entry.Key
		plen := len(g.prefix)
		if len(k) != plen+4 {
			continue
		}
		relType := k[plen+2]
		to := k[plen+3]
		if filterType {
			if _, ok := typeSet[relType]; !ok {
				continue
			}
		}
		seen[to] = struct{}{}
	}

	// Incoming neighbors.
	for entry, err := range g.store.List(ctx, g.revPrefix(label)) {
		if err != nil {
			return nil, err
		}
		k := entry.Key
		plen := len(g.prefix)
		if len(k) != plen+4 {
			continue
		}
		relType := k[plen+2]
		from := k[plen+3]
		if filterType {
			if _, ok := typeSet[relType]; !ok {
				continue
			}
		}
		seen[from] = struct{}{}
	}

	result := make([]string, 0, len(seen))
	for lbl := range seen {
		result = append(result, lbl)
	}
	sort.Strings(result)
	return result, nil
}

func (g *KVGraph) Expand(ctx context.Context, labels []string, hops int) ([]string, error) {
	if err := g.validateSegments(labels...); err != nil {
		return nil, err
	}
	visited := make(map[string]struct{}, len(labels))
	for _, l := range labels {
		visited[l] = struct{}{}
	}

	frontier := make([]string, len(labels))
	copy(frontier, labels)

	for hop := 0; hop < hops && len(frontier) > 0; hop++ {
		var next []string
		for _, label := range frontier {
			neighbors, err := g.Neighbors(ctx, label)
			if err != nil {
				return nil, err
			}
			for _, n := range neighbors {
				if _, ok := visited[n]; !ok {
					visited[n] = struct{}{}
					next = append(next, n)
				}
			}
		}
		frontier = next
	}

	result := make([]string, 0, len(visited))
	for lbl := range visited {
		result = append(result, lbl)
	}
	sort.Strings(result)
	return result, nil
}
