package recall

import (
	"context"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/graph"
	"github.com/haivivi/giztoy/go/pkg/kv"
)

// newTestIndexCustomSep creates a test index with separator 0x1F so that
// labels containing ':' (e.g., "person:小明") work correctly.
func newTestIndexCustomSep(t *testing.T) *Index {
	t.Helper()
	store := kv.NewMemory(&kv.Options{Separator: 0x1F})
	return NewIndex(IndexConfig{
		Store:     store,
		Prefix:    kv.Key{"test"},
		Separator: 0x1F,
	})
}

func TestInferLabelsBasic(t *testing.T) {
	idx := newTestIndexCustomSep(t)
	ctx := context.Background()
	g := idx.Graph()

	entities := []graph.Entity{
		{Label: "person:小明"},
		{Label: "person:小红"},
		{Label: "topic:恐龙"},
		{Label: "place:北京"},
	}
	for _, e := range entities {
		if err := g.SetEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	labels, err := idx.InferLabels(ctx, "今天和小明聊了恐龙", nil)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{"person:小明": true, "topic:恐龙": true}
	if len(labels) != len(want) {
		t.Fatalf("got %v, want %v", labels, want)
	}
	for _, l := range labels {
		if !want[l] {
			t.Errorf("unexpected label %q in result", l)
		}
	}
}

func TestInferLabelsPlainLabels(t *testing.T) {
	idx := newTestIndexNoVec(t)
	ctx := context.Background()
	g := idx.Graph()

	for _, label := range []string{"Alice", "Bob", "dinosaurs"} {
		if err := g.SetEntity(ctx, graph.Entity{Label: label}); err != nil {
			t.Fatal(err)
		}
	}

	labels, err := idx.InferLabels(ctx, "Alice and Bob talked about dinosaurs", nil)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{"Alice": true, "Bob": true, "dinosaurs": true}
	if len(labels) != len(want) {
		t.Fatalf("got %v, want labels %v", labels, want)
	}
	for _, l := range labels {
		if !want[l] {
			t.Errorf("unexpected label %q", l)
		}
	}
}

func TestInferLabelsNoMatch(t *testing.T) {
	idx := newTestIndexCustomSep(t)
	ctx := context.Background()
	g := idx.Graph()

	if err := g.SetEntity(ctx, graph.Entity{Label: "person:小明"}); err != nil {
		t.Fatal(err)
	}

	labels, err := idx.InferLabels(ctx, "今天天气不错", nil)
	if err != nil {
		t.Fatal(err)
	}
	if labels != nil {
		t.Errorf("expected nil, got %v", labels)
	}
}

func TestInferLabelsEmptyText(t *testing.T) {
	idx := newTestIndexNoVec(t)
	ctx := context.Background()

	labels, err := idx.InferLabels(ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if labels != nil {
		t.Errorf("expected nil for empty text, got %v", labels)
	}
}

func TestInferLabelsEmptyGraph(t *testing.T) {
	idx := newTestIndexNoVec(t)
	ctx := context.Background()

	labels, err := idx.InferLabels(ctx, "hello world", nil)
	if err != nil {
		t.Fatal(err)
	}
	if labels != nil {
		t.Errorf("expected nil for empty graph, got %v", labels)
	}
}

func TestInferLabelsMinNameLen(t *testing.T) {
	idx := newTestIndexNoVec(t)
	ctx := context.Background()
	g := idx.Graph()

	// Single-character entity "A" should be skipped with default MinNameLen=2.
	if err := g.SetEntity(ctx, graph.Entity{Label: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := g.SetEntity(ctx, graph.Entity{Label: "Alice"}); err != nil {
		t.Fatal(err)
	}

	labels, err := idx.InferLabels(ctx, "A met Alice today", nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(labels) != 1 || labels[0] != "Alice" {
		t.Errorf("got %v, want [Alice]", labels)
	}

	// With MinNameLen=1, the single-char entity should match.
	labels, err = idx.InferLabels(ctx, "A met Alice today", &InferConfig{MinNameLen: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 2 {
		t.Errorf("got %v, want 2 labels with MinNameLen=1", labels)
	}
}

func TestInferLabelsMinNameLenTyped(t *testing.T) {
	idx := newTestIndexCustomSep(t)
	ctx := context.Background()
	g := idx.Graph()

	// "person:A" has display name "A" (1 rune), should be skipped.
	if err := g.SetEntity(ctx, graph.Entity{Label: "person:A"}); err != nil {
		t.Fatal(err)
	}
	if err := g.SetEntity(ctx, graph.Entity{Label: "person:Alice"}); err != nil {
		t.Fatal(err)
	}

	labels, err := idx.InferLabels(ctx, "A met Alice today", nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(labels) != 1 || labels[0] != "person:Alice" {
		t.Errorf("got %v, want [person:Alice]", labels)
	}
}

func TestInferLabelsAttrMatch(t *testing.T) {
	idx := newTestIndexCustomSep(t)
	ctx := context.Background()
	g := idx.Graph()

	if err := g.SetEntity(ctx, graph.Entity{
		Label: "person:张明",
		Attrs: map[string]any{
			"nickname": "小明",
			"age":      "8",
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Text mentions "小明" (nickname) but not "张明" (label name).
	cfg := &InferConfig{AttrKeys: []string{"nickname"}}
	labels, err := idx.InferLabels(ctx, "和小明玩了一天", cfg)
	if err != nil {
		t.Fatal(err)
	}

	if len(labels) != 1 || labels[0] != "person:张明" {
		t.Errorf("got %v, want [person:张明]", labels)
	}
}

func TestInferLabelsAttrArrayMatch(t *testing.T) {
	idx := newTestIndexCustomSep(t)
	ctx := context.Background()
	g := idx.Graph()

	if err := g.SetEntity(ctx, graph.Entity{
		Label: "person:张明",
		Attrs: map[string]any{
			"alias": []any{"小明", "明明"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	cfg := &InferConfig{AttrKeys: []string{"alias"}}
	labels, err := idx.InferLabels(ctx, "明明今天很开心", cfg)
	if err != nil {
		t.Fatal(err)
	}

	if len(labels) != 1 || labels[0] != "person:张明" {
		t.Errorf("got %v, want [person:张明]", labels)
	}
}

func TestInferLabelsDedup(t *testing.T) {
	idx := newTestIndexCustomSep(t)
	ctx := context.Background()
	g := idx.Graph()

	// Entity where both the label name and an attr match the text.
	if err := g.SetEntity(ctx, graph.Entity{
		Label: "person:小明",
		Attrs: map[string]any{"nickname": "小明"},
	}); err != nil {
		t.Fatal(err)
	}

	cfg := &InferConfig{AttrKeys: []string{"nickname"}}
	labels, err := idx.InferLabels(ctx, "和小明聊天", cfg)
	if err != nil {
		t.Fatal(err)
	}

	if len(labels) != 1 {
		t.Errorf("expected 1 result (deduped), got %v", labels)
	}
}

func TestInferLabelsSorted(t *testing.T) {
	idx := newTestIndexCustomSep(t)
	ctx := context.Background()
	g := idx.Graph()

	for _, label := range []string{"topic:恐龙", "person:小明", "place:北京"} {
		if err := g.SetEntity(ctx, graph.Entity{Label: label}); err != nil {
			t.Fatal(err)
		}
	}

	labels, err := idx.InferLabels(ctx, "小明去北京看恐龙展", nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(labels) != 3 {
		t.Fatalf("got %d labels, want 3", len(labels))
	}

	for i := 1; i < len(labels); i++ {
		if labels[i] < labels[i-1] {
			t.Errorf("labels not sorted: %v", labels)
			break
		}
	}
}

func TestInferLabelsVoiceLabel(t *testing.T) {
	idx := newTestIndexCustomSep(t)
	ctx := context.Background()
	g := idx.Graph()

	if err := g.SetEntity(ctx, graph.Entity{Label: "voice:A3F8"}); err != nil {
		t.Fatal(err)
	}

	labels, err := idx.InferLabels(ctx, "speaker A3F8 was talking", nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(labels) != 1 || labels[0] != "voice:A3F8" {
		t.Errorf("got %v, want [voice:A3F8]", labels)
	}
}

func TestInferLabelsCJKRuneLen(t *testing.T) {
	idx := newTestIndexCustomSep(t)
	ctx := context.Background()
	g := idx.Graph()

	// "明" is 1 rune (3 bytes UTF-8). Default MinNameLen=2, so it's skipped.
	if err := g.SetEntity(ctx, graph.Entity{Label: "person:明"}); err != nil {
		t.Fatal(err)
	}

	labels, err := idx.InferLabels(ctx, "明天去公园", nil)
	if err != nil {
		t.Fatal(err)
	}
	if labels != nil {
		t.Errorf("single-rune CJK entity should be skipped, got %v", labels)
	}

	// Two-rune CJK name should match.
	if err := g.SetEntity(ctx, graph.Entity{Label: "person:小明"}); err != nil {
		t.Fatal(err)
	}

	labels, err = idx.InferLabels(ctx, "和小明聊天", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 1 || labels[0] != "person:小明" {
		t.Errorf("got %v, want [person:小明]", labels)
	}
}

func TestDisplayName(t *testing.T) {
	tests := []struct {
		label string
		want  string
	}{
		{"person:小明", "小明"},
		{"topic:恐龙", "恐龙"},
		{"Alice", "Alice"},
		{"voice:A3F8", "A3F8"},
		{"a:b:c", "b:c"},
		// Edge cases: colon at boundary with nothing useful after it.
		// displayName returns the full label when there's no name part.
		{":", ":"},
		{"a:", "a:"},
	}
	for _, tt := range tests {
		got := displayName(tt.label)
		if got != tt.want {
			t.Errorf("displayName(%q) = %q, want %q", tt.label, got, tt.want)
		}
	}
}

func TestRuneLen(t *testing.T) {
	tests := []struct {
		s    string
		want int
	}{
		{"", 0},
		{"hello", 5},
		{"小明", 2},
		{"A3F8", 4},
		{"café", 4},
	}
	for _, tt := range tests {
		got := runeLen(tt.s)
		if got != tt.want {
			t.Errorf("runeLen(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}
