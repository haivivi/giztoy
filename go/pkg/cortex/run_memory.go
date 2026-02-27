package cortex

import (
	"context"
	"fmt"

	"github.com/haivivi/giztoy/go/pkg/genx/labelers"
	"github.com/haivivi/giztoy/go/pkg/graph"
	"github.com/haivivi/giztoy/go/pkg/kv"
	"github.com/haivivi/giztoy/go/pkg/memory"
	"github.com/haivivi/giztoy/go/pkg/recall"
)

func init() {
	RegisterRunHandler("memory/create", runMemoryCreate)
	RegisterRunHandler("memory/add", runMemoryAdd)
	RegisterRunHandler("memory/search", runMemorySearch)
	RegisterRunHandler("memory/recall", runMemoryRecall)
	RegisterRunHandler("memory/entity/set", runMemoryEntitySet)
	RegisterRunHandler("memory/entity/get", runMemoryEntityGet)
	RegisterRunHandler("memory/entity/list", runMemoryEntityList)
	RegisterRunHandler("memory/entity/delete", runMemoryEntityDelete)
	RegisterRunHandler("memory/relation/add", runMemoryRelationAdd)
	RegisterRunHandler("memory/relation/list", runMemoryRelationList)
	RegisterRunHandler("memory/demo", runMemoryDemo)
}

const memorySep byte = 0x1F

func (c *Cortex) ensureMemoryHost(ctx context.Context) (*memory.Host, error) {
	c.memMu.Lock()
	defer c.memMu.Unlock()
	if c.memHost != nil {
		return c.memHost, nil
	}
	memKV := kv.NewMemory(&kv.Options{Separator: memorySep})
	host, err := memory.NewHost(ctx, memory.HostConfig{
		Store:     memKV,
		Separator: memorySep,
	})
	if err != nil {
		return nil, err
	}
	c.memHost = host
	return host, nil
}

func (c *Cortex) openMemory(ctx context.Context, persona string) (*memory.Memory, error) {
	host, err := c.ensureMemoryHost(ctx)
	if err != nil {
		return nil, err
	}
	return host.Open(persona)
}

func runMemoryCreate(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	name := task.GetString("name")
	if name == "" {
		return nil, fmt.Errorf("memory/create: missing 'name' field")
	}
	// "create" for memory just opens it (creates on first use)
	_, err := c.openMemory(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("memory create: %w", err)
	}
	return &RunResult{Kind: task.Kind, Status: "ok", Data: map[string]any{"persona": name}}, nil
}

func runMemoryAdd(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	persona := task.GetString("persona")
	if persona == "" {
		return nil, fmt.Errorf("memory/add: missing 'persona' field")
	}
	text := task.GetString("text")
	if text == "" {
		return nil, fmt.Errorf("memory/add: missing 'text' field")
	}

	mem, err := c.openMemory(ctx, persona)
	if err != nil {
		return nil, err
	}

	seg := memory.SegmentInput{Summary: text}
	if labels, ok := task.Fields["labels"].([]any); ok {
		for _, l := range labels {
			if s, ok := l.(string); ok {
				seg.Labels = append(seg.Labels, s)
			}
		}
	}
	if keywords, ok := task.Fields["keywords"].([]any); ok {
		for _, k := range keywords {
			if s, ok := k.(string); ok {
				seg.Keywords = append(seg.Keywords, s)
			}
		}
	}

	if err := mem.StoreSegment(ctx, seg, recall.Bucket1H); err != nil {
		return nil, fmt.Errorf("memory add: %w", err)
	}

	return &RunResult{Kind: task.Kind, Status: "ok", Data: map[string]any{"persona": persona, "text": text}}, nil
}

func runMemorySearch(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	persona := task.GetString("persona")
	text := task.GetString("text")
	if persona == "" || text == "" {
		return nil, fmt.Errorf("memory/search: missing 'persona' or 'text'")
	}

	mem, err := c.openMemory(ctx, persona)
	if err != nil {
		return nil, err
	}

	limit := task.GetInt("limit")
	if limit <= 0 {
		limit = 10
	}

	results, err := mem.Index().SearchSegments(ctx, recall.SearchQuery{Text: text, Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("memory search: %w", err)
	}

	var items []map[string]any
	for _, r := range results {
		items = append(items, map[string]any{
			"score":   r.Score,
			"summary": r.Segment.Summary,
			"labels":  r.Segment.Labels,
		})
	}
	return &RunResult{Kind: task.Kind, Status: "ok", Data: map[string]any{"results": items, "count": len(items)}}, nil
}

func runMemoryRecall(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	persona := task.GetString("persona")
	text := task.GetString("text")
	if persona == "" || text == "" {
		return nil, fmt.Errorf("memory/recall: missing 'persona' or 'text'")
	}

	mem, err := c.openMemory(ctx, persona)
	if err != nil {
		return nil, err
	}

	q := memory.RecallQuery{Text: text, Limit: task.GetInt("limit"), Hops: task.GetInt("hops")}
	if q.Limit <= 0 {
		q.Limit = 10
	}
	if q.Hops <= 0 {
		q.Hops = 2
	}
	if labels, ok := task.Fields["labels"].([]any); ok {
		for _, l := range labels {
			if s, ok := l.(string); ok {
				q.Labels = append(q.Labels, s)
			}
		}
	}

	labelerPattern := task.GetString("labeler")
	if labelerPattern != "" {
		candidates, err := collectCandidateLabels(ctx, mem.Graph())
		if err != nil {
			return nil, fmt.Errorf("memory recall: collect label candidates: %w", err)
		}

		result, err := labelers.Process(ctx, labelerPattern, labelers.Input{
			Text:       text,
			Candidates: candidates,
			TopK:       task.GetInt("label_top_k"),
		})
		if err != nil {
			return nil, fmt.Errorf("memory recall: labeler process: %w", err)
		}
		q.Labels = mergeLabels(q.Labels, result)
	}

	result, err := mem.Recall(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("memory recall: %w", err)
	}

	return &RunResult{Kind: task.Kind, Status: "ok", Data: map[string]any{
		"entities_count": len(result.Entities),
		"segments_count": len(result.Segments),
		"labels":         q.Labels,
	}}, nil
}

func collectCandidateLabels(ctx context.Context, g graph.Graph) ([]string, error) {
	var labels []string
	for ent, err := range g.ListEntities(ctx, "") {
		if err != nil {
			return nil, err
		}
		labels = append(labels, ent.Label)
	}
	return labels, nil
}

func mergeLabels(base []string, lr *labelers.Result) []string {
	if lr == nil || len(lr.Matches) == 0 {
		return base
	}
	seen := make(map[string]struct{}, len(base)+len(lr.Matches))
	out := make([]string, 0, len(base)+len(lr.Matches))
	for _, l := range base {
		if l == "" {
			continue
		}
		if _, ok := seen[l]; ok {
			continue
		}
		seen[l] = struct{}{}
		out = append(out, l)
	}
	for _, m := range lr.Matches {
		if m.Label == "" {
			continue
		}
		if _, ok := seen[m.Label]; ok {
			continue
		}
		seen[m.Label] = struct{}{}
		out = append(out, m.Label)
	}
	return out
}

func runMemoryEntitySet(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	persona := task.GetString("persona")
	label := task.GetString("label")
	if persona == "" || label == "" {
		return nil, fmt.Errorf("memory/entity/set: missing 'persona' or 'label'")
	}

	mem, err := c.openMemory(ctx, persona)
	if err != nil {
		return nil, err
	}

	attrs, _ := task.Fields["attrs"].(map[string]any)
	if err := mem.Graph().SetEntity(ctx, graph.Entity{Label: label, Attrs: attrs}); err != nil {
		return nil, fmt.Errorf("memory entity set: %w", err)
	}

	return &RunResult{Kind: task.Kind, Status: "ok", Data: map[string]any{"label": label}}, nil
}

func runMemoryEntityGet(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	persona := task.GetString("persona")
	label := task.GetString("label")
	if persona == "" || label == "" {
		return nil, fmt.Errorf("memory/entity/get: missing 'persona' or 'label'")
	}

	mem, err := c.openMemory(ctx, persona)
	if err != nil {
		return nil, err
	}

	entity, err := mem.Graph().GetEntity(ctx, label)
	if err != nil {
		return nil, fmt.Errorf("memory entity get: %w", err)
	}

	return &RunResult{Kind: task.Kind, Status: "ok", Data: map[string]any{
		"label": entity.Label,
		"attrs": entity.Attrs,
	}}, nil
}

func runMemoryEntityList(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	persona := task.GetString("persona")
	if persona == "" {
		return nil, fmt.Errorf("memory/entity/list: missing 'persona'")
	}

	mem, err := c.openMemory(ctx, persona)
	if err != nil {
		return nil, err
	}

	prefix := task.GetString("prefix")
	var entities []map[string]any
	for e, err := range mem.Graph().ListEntities(ctx, prefix) {
		if err != nil {
			return nil, fmt.Errorf("memory entity list: %w", err)
		}
		entities = append(entities, map[string]any{"label": e.Label, "attrs": e.Attrs})
	}

	return &RunResult{Kind: task.Kind, Status: "ok", Data: map[string]any{"entities": entities, "count": len(entities)}}, nil
}

func runMemoryEntityDelete(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	persona := task.GetString("persona")
	label := task.GetString("label")
	if persona == "" || label == "" {
		return nil, fmt.Errorf("memory/entity/delete: missing 'persona' or 'label'")
	}

	mem, err := c.openMemory(ctx, persona)
	if err != nil {
		return nil, err
	}

	if err := mem.Graph().DeleteEntity(ctx, label); err != nil {
		return nil, fmt.Errorf("memory entity delete: %w", err)
	}

	return &RunResult{Kind: task.Kind, Status: "ok", Data: map[string]any{"label": label}}, nil
}

func runMemoryRelationAdd(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	persona := task.GetString("persona")
	from := task.GetString("from")
	to := task.GetString("to")
	relType := task.GetString("rel_type")
	if persona == "" || from == "" || to == "" || relType == "" {
		return nil, fmt.Errorf("memory/relation/add: missing required fields")
	}

	mem, err := c.openMemory(ctx, persona)
	if err != nil {
		return nil, err
	}

	if err := mem.Graph().AddRelation(ctx, graph.Relation{From: from, To: to, RelType: relType}); err != nil {
		return nil, fmt.Errorf("memory relation add: %w", err)
	}

	return &RunResult{Kind: task.Kind, Status: "ok", Data: map[string]any{"from": from, "to": to, "type": relType}}, nil
}

func runMemoryRelationList(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	persona := task.GetString("persona")
	label := task.GetString("label")
	if persona == "" || label == "" {
		return nil, fmt.Errorf("memory/relation/list: missing 'persona' or 'label'")
	}

	mem, err := c.openMemory(ctx, persona)
	if err != nil {
		return nil, err
	}

	rels, err := mem.Graph().Relations(ctx, label)
	if err != nil {
		return nil, fmt.Errorf("memory relation list: %w", err)
	}

	var items []map[string]any
	for _, r := range rels {
		items = append(items, map[string]any{"from": r.From, "to": r.To, "type": r.RelType})
	}

	return &RunResult{Kind: task.Kind, Status: "ok", Data: map[string]any{"relations": items, "count": len(items)}}, nil
}

func runMemoryDemo(ctx context.Context, c *Cortex, task Document) (*RunResult, error) {
	persona := task.GetString("persona")
	if persona == "" {
		persona = "demo"
	}
	mem, err := c.openMemory(ctx, persona)
	if err != nil {
		return nil, err
	}
	g := mem.Graph()

	g.SetEntity(ctx, graph.Entity{Label: "person:小明", Attrs: map[string]any{"age": 8}})
	g.SetEntity(ctx, graph.Entity{Label: "topic:恐龙"})
	g.AddRelation(ctx, graph.Relation{From: "person:小明", To: "topic:恐龙", RelType: "likes"})

	mem.StoreSegment(ctx, memory.SegmentInput{
		Summary:  "和小明聊了恐龙",
		Labels:   []string{"person:小明", "topic:恐龙"},
		Keywords: []string{"恐龙", "霸王龙"},
	}, recall.Bucket1H)

	return &RunResult{Kind: task.Kind, Status: "ok", Data: map[string]any{
		"persona":  persona,
		"entities": 2,
		"segments": 1,
	}}, nil
}
