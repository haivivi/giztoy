package memory

import (
	"context"
	"errors"
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/graph"
	"github.com/haivivi/giztoy/go/pkg/kv"
	"github.com/haivivi/giztoy/go/pkg/recall"
)

// Memory is the complete memory system for a single persona (virtual character).
// It provides:
//
//   - Graph: the persona's view of the world (entities + relations)
//   - Segments: compressed memory fragments organized in time-granularity buckets
//   - Conversations: active dialogue sessions
//   - Recall: combined graph expansion + segment search for LLM context
//   - Compaction: automatic cascading compression (1h → 1d → 1w → ... → lt)
//
// Each Memory is fully isolated — it owns a [recall.Index] scoped under
// a unique KV prefix "mem:{id}".
type Memory struct {
	id         string
	store      kv.Store
	index      *recall.Index
	compressor Compressor     // default compressor from Host, may be nil
	policy     CompressPolicy // auto-compression thresholds
}

func newMemory(id string, store kv.Store, index *recall.Index, compressor Compressor, policy CompressPolicy) *Memory {
	return &Memory{
		id:         id,
		store:      store,
		index:      index,
		compressor: compressor,
		policy:     policy,
	}
}

// ID returns the persona identifier.
func (m *Memory) ID() string { return m.id }

// Graph returns the entity-relation graph for this persona.
// Use it to query and update entities (people, topics, voice labels)
// and their relations.
func (m *Memory) Graph() graph.Graph { return m.index.Graph() }

// Index returns the underlying recall index for direct segment operations
// (store, delete, search). Most callers should use [Recall] instead.
func (m *Memory) Index() *recall.Index { return m.index }

// OpenConversation opens (or resumes) a conversation session.
// convID is typically a device ID or session ID. labels mark which entities
// are involved (e.g., ["person:Alice"]). Multiple calls with the same convID
// return independent handles to the same underlying data.
//
// If a [CompressPolicy] and [Compressor] are configured (via [HostConfig] or
// per-persona [OpenOption]), messages appended to the conversation are
// automatically compressed when thresholds are reached.
func (m *Memory) OpenConversation(convID string, labels []string) *Conversation {
	return newConversation(m, convID, labels)
}

// Recall performs combined retrieval: graph expansion + multi-signal segment
// search + entity attribute lookup.
//
// This is the primary method for building LLM context. The flow:
//  1. Expand seed labels through the graph (BFS, up to Hops).
//  2. Search segments across all buckets using expanded labels + query text.
//  3. Fetch entity attributes for all expanded labels.
func (m *Memory) Recall(ctx context.Context, q RecallQuery) (*RecallResult, error) {
	hops := q.Hops
	if hops <= 0 {
		hops = 2
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 10
	}

	// Step 1+2: delegate to recall.Index.Search for graph expansion + segment search.
	rResult, err := m.index.Search(ctx, recall.Query{
		Labels: q.Labels,
		Text:   q.Text,
		Hops:   hops,
		Limit:  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("memory: recall search: %w", err)
	}

	// Step 3: fetch entity attributes for expanded labels.
	var entities []EntityInfo
	for _, label := range rResult.Expanded {
		ent, err := m.index.Graph().GetEntity(ctx, label)
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("memory: get entity %q: %w", label, err)
		}
		entities = append(entities, EntityInfo{
			Label: ent.Label,
			Attrs: ent.Attrs,
		})
	}

	// Convert scored segments.
	segments := make([]ScoredSegment, len(rResult.Segments))
	for i, ss := range rResult.Segments {
		segments[i] = ScoredSegment{
			ID:        ss.Segment.ID,
			Summary:   ss.Segment.Summary,
			Keywords:  ss.Segment.Keywords,
			Labels:    ss.Segment.Labels,
			Timestamp: ss.Segment.Timestamp,
			Score:     ss.Score,
		}
	}

	return &RecallResult{
		Entities: entities,
		Segments: segments,
	}, nil
}

// InferLabels discovers entity labels from the persona's graph that are
// referenced in the given text. It is a convenience wrapper around
// [recall.Index.InferLabels].
//
// This is useful for automatically augmenting seed labels before calling
// [Recall]. For example, given "今天和小明聊了恐龙" and a graph containing
// "person:小明" and "topic:恐龙", InferLabels returns both labels.
//
// See [recall.InferConfig] for configuration options (attribute matching,
// minimum name length).
func (m *Memory) InferLabels(ctx context.Context, text string, cfg *recall.InferConfig) ([]string, error) {
	return m.index.InferLabels(ctx, text, cfg)
}

// StoreSegment stores a new segment in this persona's recall index.
// It generates an ID and timestamp, sets the bucket, and indexes the
// segment for search.
func (m *Memory) StoreSegment(ctx context.Context, input SegmentInput, bucket recall.Bucket) error {
	ts := nowNano()
	seg := recall.Segment{
		ID:        fmt.Sprintf("%s-%d", m.id, ts),
		Summary:   input.Summary,
		Keywords:  input.Keywords,
		Labels:    input.Labels,
		Timestamp: ts,
		Bucket:    bucket,
	}
	return m.index.StoreSegment(ctx, seg)
}

// ApplyEntityUpdate applies entity and relation updates from a compression
// result to the graph.
func (m *Memory) ApplyEntityUpdate(ctx context.Context, update *EntityUpdate) error {
	if update == nil {
		return nil
	}

	g := m.index.Graph()

	for _, e := range update.Entities {
		// Try to merge attributes into an existing entity.
		err := g.MergeAttrs(ctx, e.Label, e.Attrs)
		if err != nil {
			if !isNotFound(err) {
				return fmt.Errorf("memory: merge attrs %q: %w", e.Label, err)
			}
			// Entity doesn't exist — create it.
			if err := g.SetEntity(ctx, graph.Entity{
				Label: e.Label,
				Attrs: e.Attrs,
			}); err != nil {
				return fmt.Errorf("memory: set entity %q: %w", e.Label, err)
			}
		}
	}

	for _, r := range update.Relations {
		if err := g.AddRelation(ctx, graph.Relation{
			From:    r.From,
			To:      r.To,
			RelType: r.RelType,
		}); err != nil {
			return fmt.Errorf("memory: add relation %s→%s: %w", r.From, r.To, err)
		}
	}

	return nil
}

// Compress runs the full compression pipeline on a conversation:
//  1. Read all messages.
//  2. ExtractEntities → apply to graph.
//  3. CompressMessages → store segments in [recall.Bucket1H].
//  4. Clear the conversation.
//
// If compressor is non-nil, it is used for this call. Otherwise, the
// host's default compressor is used (see [HostConfig.Compressor]).
// If both are nil, Compress returns an error.
func (m *Memory) Compress(ctx context.Context, conv *Conversation, compressor Compressor) error {
	if compressor == nil {
		compressor = m.compressor
	}
	if compressor == nil {
		return fmt.Errorf("memory: compressor is nil (no per-call compressor and no default set in HostConfig)")
	}

	msgs, err := conv.All(ctx)
	if err != nil {
		return fmt.Errorf("memory: read messages: %w", err)
	}
	if len(msgs) == 0 {
		return nil
	}

	// Extract entities and update the graph.
	update, err := compressor.ExtractEntities(ctx, msgs)
	if err != nil {
		return fmt.Errorf("memory: extract entities: %w", err)
	}
	if err := m.ApplyEntityUpdate(ctx, update); err != nil {
		return err
	}

	// Compress messages into segments.
	result, err := compressor.CompressMessages(ctx, msgs)
	if err != nil {
		return fmt.Errorf("memory: compress messages: %w", err)
	}

	// Store the segments in the 1h bucket.
	for _, seg := range result.Segments {
		if err := m.StoreSegment(ctx, seg, recall.Bucket1H); err != nil {
			return fmt.Errorf("memory: store segment: %w", err)
		}
	}

	// Clear the conversation.
	if err := conv.Clear(ctx); err != nil {
		return fmt.Errorf("memory: clear conversation: %w", err)
	}

	return nil
}

// isNotFound checks if an error is a "not found" sentinel from graph or kv.
func isNotFound(err error) bool {
	return errors.Is(err, graph.ErrNotFound) || errors.Is(err, kv.ErrNotFound)
}
