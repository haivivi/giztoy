package graph_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/graph"
	"github.com/haivivi/giztoy/go/pkg/kv"
)

func newTestGraph(t *testing.T) graph.Graph {
	t.Helper()
	store := kv.NewMemory(nil)
	t.Cleanup(func() { store.Close() })
	return graph.NewKVGraph(store, kv.Key{"test", "g"})
}

// --- Entity tests ---

func TestGetEntity_NotFound(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	_, err := g.GetEntity(ctx, "nobody")
	if !errors.Is(err, graph.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSetGetEntity(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	e := graph.Entity{
		Label: "Alice",
		Attrs: map[string]any{"age": float64(30), "role": "engineer"},
	}
	if err := g.SetEntity(ctx, e); err != nil {
		t.Fatalf("SetEntity: %v", err)
	}

	got, err := g.GetEntity(ctx, "Alice")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.Label != "Alice" {
		t.Fatalf("Label = %q, want %q", got.Label, "Alice")
	}
	if got.Attrs["role"] != "engineer" {
		t.Fatalf("Attrs[role] = %v, want engineer", got.Attrs["role"])
	}
	if got.Attrs["age"] != float64(30) {
		t.Fatalf("Attrs[age] = %v, want 30", got.Attrs["age"])
	}
}

func TestSetEntity_Overwrite(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	if err := g.SetEntity(ctx, graph.Entity{Label: "Bob", Attrs: map[string]any{"v": 1}}); err != nil {
		t.Fatal(err)
	}
	if err := g.SetEntity(ctx, graph.Entity{Label: "Bob", Attrs: map[string]any{"v": 2}}); err != nil {
		t.Fatal(err)
	}

	got, err := g.GetEntity(ctx, "Bob")
	if err != nil {
		t.Fatal(err)
	}
	// JSON numbers are float64.
	if got.Attrs["v"] != float64(2) {
		t.Fatalf("Attrs[v] = %v, want 2", got.Attrs["v"])
	}
}

func TestSetEntity_NoAttrs(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	if err := g.SetEntity(ctx, graph.Entity{Label: "Empty"}); err != nil {
		t.Fatal(err)
	}
	got, err := g.GetEntity(ctx, "Empty")
	if err != nil {
		t.Fatal(err)
	}
	if got.Label != "Empty" {
		t.Fatalf("Label = %q, want Empty", got.Label)
	}
}

func TestDeleteEntity(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	// Create entity with relations.
	if err := g.SetEntity(ctx, graph.Entity{Label: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := g.SetEntity(ctx, graph.Entity{Label: "B"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(ctx, graph.Relation{From: "A", To: "B", RelType: "knows"}); err != nil {
		t.Fatal(err)
	}

	// Delete A — should remove entity and its relations.
	if err := g.DeleteEntity(ctx, "A"); err != nil {
		t.Fatal(err)
	}

	_, err := g.GetEntity(ctx, "A")
	if !errors.Is(err, graph.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	// Relations involving A should be gone.
	rels, err := g.Relations(ctx, "B")
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 0 {
		t.Fatalf("expected 0 relations for B after deleting A, got %d", len(rels))
	}
}

func TestDeleteEntity_NonExistent(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	// Should not error.
	if err := g.DeleteEntity(ctx, "ghost"); err != nil {
		t.Fatalf("DeleteEntity non-existent: %v", err)
	}
}

func TestMergeAttrs(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	if err := g.SetEntity(ctx, graph.Entity{
		Label: "X",
		Attrs: map[string]any{"a": "1", "b": "2"},
	}); err != nil {
		t.Fatal(err)
	}

	// Merge: overwrite "b", add "c".
	if err := g.MergeAttrs(ctx, "X", map[string]any{"b": "updated", "c": "3"}); err != nil {
		t.Fatal(err)
	}

	got, err := g.GetEntity(ctx, "X")
	if err != nil {
		t.Fatal(err)
	}
	if got.Attrs["a"] != "1" {
		t.Fatalf("Attrs[a] = %v, want 1", got.Attrs["a"])
	}
	if got.Attrs["b"] != "updated" {
		t.Fatalf("Attrs[b] = %v, want updated", got.Attrs["b"])
	}
	if got.Attrs["c"] != "3" {
		t.Fatalf("Attrs[c] = %v, want 3", got.Attrs["c"])
	}
}

func TestMergeAttrs_NotFound(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	err := g.MergeAttrs(ctx, "ghost", map[string]any{"a": "1"})
	if !errors.Is(err, graph.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListEntities(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	for _, label := range []string{"Alice", "Alex", "Bob", "Charlie"} {
		if err := g.SetEntity(ctx, graph.Entity{Label: label}); err != nil {
			t.Fatal(err)
		}
	}

	// List all.
	var all []string
	for e, err := range g.ListEntities(ctx, "") {
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, e.Label)
	}
	want := []string{"Alex", "Alice", "Bob", "Charlie"}
	if !slices.Equal(all, want) {
		t.Fatalf("ListEntities('') = %v, want %v", all, want)
	}

	// List with prefix "Al".
	var filtered []string
	for e, err := range g.ListEntities(ctx, "Al") {
		if err != nil {
			t.Fatal(err)
		}
		filtered = append(filtered, e.Label)
	}
	wantFiltered := []string{"Alex", "Alice"}
	if !slices.Equal(filtered, wantFiltered) {
		t.Fatalf("ListEntities('Al') = %v, want %v", filtered, wantFiltered)
	}
}

// --- Relation tests ---

func TestAddAndGetRelations(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	if err := g.AddRelation(ctx, graph.Relation{From: "A", To: "B", RelType: "knows"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(ctx, graph.Relation{From: "A", To: "C", RelType: "works_with"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(ctx, graph.Relation{From: "D", To: "A", RelType: "manages"}); err != nil {
		t.Fatal(err)
	}

	// Relations for A: should include all 3 (as source or target).
	rels, err := g.Relations(ctx, "A")
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 3 {
		t.Fatalf("Relations(A) = %d, want 3", len(rels))
	}

	// Relations for B: only A->B.
	rels, err = g.Relations(ctx, "B")
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 1 {
		t.Fatalf("Relations(B) = %d, want 1", len(rels))
	}
	if rels[0].From != "A" || rels[0].To != "B" || rels[0].RelType != "knows" {
		t.Fatalf("unexpected relation: %+v", rels[0])
	}
}

func TestAddRelation_Idempotent(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	r := graph.Relation{From: "A", To: "B", RelType: "knows"}
	if err := g.AddRelation(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(ctx, r); err != nil {
		t.Fatal(err)
	}

	rels, err := g.Relations(ctx, "A")
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 1 {
		t.Fatalf("expected 1 relation after idempotent add, got %d", len(rels))
	}
}

func TestRelations_SelfLoop(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	// Self-loop: A -> A.
	if err := g.AddRelation(ctx, graph.Relation{From: "A", To: "A", RelType: "self"}); err != nil {
		t.Fatal(err)
	}
	// Plus a normal relation.
	if err := g.AddRelation(ctx, graph.Relation{From: "A", To: "B", RelType: "knows"}); err != nil {
		t.Fatal(err)
	}

	rels, err := g.Relations(ctx, "A")
	if err != nil {
		t.Fatal(err)
	}
	// Should have exactly 2: self-loop + A->B. No duplicates from self-loop.
	if len(rels) != 2 {
		t.Fatalf("expected 2 relations, got %d: %+v", len(rels), rels)
	}

	// Neighbors of A should include A (self-loop) and B.
	neighbors, err := g.Neighbors(ctx, "A")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"A", "B"}
	if !slices.Equal(neighbors, want) {
		t.Fatalf("Neighbors(A) = %v, want %v", neighbors, want)
	}
}

func TestRemoveRelation(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	if err := g.AddRelation(ctx, graph.Relation{From: "A", To: "B", RelType: "knows"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(ctx, graph.Relation{From: "A", To: "C", RelType: "knows"}); err != nil {
		t.Fatal(err)
	}

	// Remove A->B.
	if err := g.RemoveRelation(ctx, "A", "B", "knows"); err != nil {
		t.Fatal(err)
	}

	rels, err := g.Relations(ctx, "A")
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 1 {
		t.Fatalf("expected 1 relation after remove, got %d", len(rels))
	}
	if rels[0].To != "C" {
		t.Fatalf("expected remaining relation to C, got %+v", rels[0])
	}

	// Remove should also clean reverse index.
	rels, err = g.Relations(ctx, "B")
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 0 {
		t.Fatalf("expected 0 relations for B, got %d", len(rels))
	}
}

func TestRemoveRelation_NonExistent(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	// Should not error.
	if err := g.RemoveRelation(ctx, "X", "Y", "nope"); err != nil {
		t.Fatalf("RemoveRelation non-existent: %v", err)
	}
}

// --- Traversal tests ---

func TestNeighbors(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	//   A --knows--> B
	//   A --works_with--> C
	//   D --manages--> A
	if err := g.AddRelation(ctx, graph.Relation{From: "A", To: "B", RelType: "knows"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(ctx, graph.Relation{From: "A", To: "C", RelType: "works_with"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(ctx, graph.Relation{From: "D", To: "A", RelType: "manages"}); err != nil {
		t.Fatal(err)
	}

	// All neighbors of A.
	got, err := g.Neighbors(ctx, "A")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"B", "C", "D"}
	if !slices.Equal(got, want) {
		t.Fatalf("Neighbors(A) = %v, want %v", got, want)
	}

	// Neighbors of A filtered by "knows".
	got, err = g.Neighbors(ctx, "A", "knows")
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"B"}
	if !slices.Equal(got, want) {
		t.Fatalf("Neighbors(A, knows) = %v, want %v", got, want)
	}

	// Neighbors of B.
	got, err = g.Neighbors(ctx, "B")
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"A"}
	if !slices.Equal(got, want) {
		t.Fatalf("Neighbors(B) = %v, want %v", got, want)
	}
}

func TestNeighbors_MultipleRelTypes(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	if err := g.AddRelation(ctx, graph.Relation{From: "A", To: "B", RelType: "knows"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(ctx, graph.Relation{From: "A", To: "C", RelType: "works_with"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(ctx, graph.Relation{From: "A", To: "D", RelType: "manages"}); err != nil {
		t.Fatal(err)
	}

	got, err := g.Neighbors(ctx, "A", "knows", "manages")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"B", "D"}
	if !slices.Equal(got, want) {
		t.Fatalf("Neighbors(A, knows, manages) = %v, want %v", got, want)
	}
}

func TestExpand_ZeroHops(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	got, err := g.Expand(ctx, []string{"A", "B"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"A", "B"}
	if !slices.Equal(got, want) {
		t.Fatalf("Expand 0 hops = %v, want %v", got, want)
	}
}

func TestExpand_MultiHop(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	// Chain: A -> B -> C -> D -> E
	for _, r := range []graph.Relation{
		{From: "A", To: "B", RelType: "next"},
		{From: "B", To: "C", RelType: "next"},
		{From: "C", To: "D", RelType: "next"},
		{From: "D", To: "E", RelType: "next"},
	} {
		if err := g.AddRelation(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	// 1 hop from A: A + B (A->B outgoing, no incoming on A)
	got, err := g.Expand(ctx, []string{"A"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"A", "B"}
	if !slices.Equal(got, want) {
		t.Fatalf("Expand(A, 1) = %v, want %v", got, want)
	}

	// 2 hops from A: A + B + C (also A via B's incoming)
	got, err = g.Expand(ctx, []string{"A"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"A", "B", "C"}
	if !slices.Equal(got, want) {
		t.Fatalf("Expand(A, 2) = %v, want %v", got, want)
	}

	// 3 hops from A.
	got, err = g.Expand(ctx, []string{"A"}, 3)
	if err != nil {
		t.Fatal(err)
	}
	// A->B->C->D, but also reverse edges: B knows A, C knows B, etc.
	// So hop 1: A->{B}, hop 2: B->{A,C}, hop 3: C->{B,D} -> D is new.
	want = []string{"A", "B", "C", "D"}
	if !slices.Equal(got, want) {
		t.Fatalf("Expand(A, 3) = %v, want %v", got, want)
	}
}

func TestExpand_Graph(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	//    A
	//   / \
	//  B   C
	//   \ /
	//    D
	for _, r := range []graph.Relation{
		{From: "A", To: "B", RelType: "link"},
		{From: "A", To: "C", RelType: "link"},
		{From: "B", To: "D", RelType: "link"},
		{From: "C", To: "D", RelType: "link"},
	} {
		if err := g.AddRelation(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	got, err := g.Expand(ctx, []string{"A"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"A", "B", "C", "D"}
	if !slices.Equal(got, want) {
		t.Fatalf("Expand(A, 2) = %v, want %v", got, want)
	}
}

func TestExpand_MultipleSeeds(t *testing.T) {
	g := newTestGraph(t)
	ctx := context.Background()

	// Two disconnected pairs: A-B, C-D
	if err := g.AddRelation(ctx, graph.Relation{From: "A", To: "B", RelType: "link"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(ctx, graph.Relation{From: "C", To: "D", RelType: "link"}); err != nil {
		t.Fatal(err)
	}

	got, err := g.Expand(ctx, []string{"A", "C"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"A", "B", "C", "D"}
	if !slices.Equal(got, want) {
		t.Fatalf("Expand(A,C, 1) = %v, want %v", got, want)
	}
}

// --- Benchmarks ---

func setupBenchGraph(b *testing.B, nEntities, nRelations int) graph.Graph {
	b.Helper()
	store := kv.NewMemory(nil)
	g := graph.NewKVGraph(store, kv.Key{"bench", "g"})
	ctx := context.Background()

	for i := 0; i < nEntities; i++ {
		label := fmt.Sprintf("entity_%04d", i)
		if err := g.SetEntity(ctx, graph.Entity{
			Label: label,
			Attrs: map[string]any{"index": float64(i), "name": label},
		}); err != nil {
			b.Fatal(err)
		}
	}

	for i := 0; i < nRelations; i++ {
		from := fmt.Sprintf("entity_%04d", i%nEntities)
		to := fmt.Sprintf("entity_%04d", (i*7+3)%nEntities) // pseudo-random connections
		relType := "link"
		if i%3 == 0 {
			relType = "knows"
		}
		if err := g.AddRelation(ctx, graph.Relation{From: from, To: to, RelType: relType}); err != nil {
			b.Fatal(err)
		}
	}

	return g
}

func BenchmarkSetEntity(b *testing.B) {
	store := kv.NewMemory(nil)
	g := graph.NewKVGraph(store, kv.Key{"bench", "g"})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		label := fmt.Sprintf("entity_%d", i)
		_ = g.SetEntity(ctx, graph.Entity{
			Label: label,
			Attrs: map[string]any{"i": float64(i)},
		})
	}
}

func BenchmarkGetEntity(b *testing.B) {
	g := setupBenchGraph(b, 1000, 0)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		label := fmt.Sprintf("entity_%04d", i%1000)
		_, _ = g.GetEntity(ctx, label)
	}
}

func BenchmarkMergeAttrs(b *testing.B) {
	g := setupBenchGraph(b, 1000, 0)
	ctx := context.Background()
	attrs := map[string]any{"new_key": "new_value"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		label := fmt.Sprintf("entity_%04d", i%1000)
		_ = g.MergeAttrs(ctx, label, attrs)
	}
}

func BenchmarkAddRelation(b *testing.B) {
	store := kv.NewMemory(nil)
	g := graph.NewKVGraph(store, kv.Key{"bench", "g"})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.AddRelation(ctx, graph.Relation{
			From:    fmt.Sprintf("e_%d", i),
			To:      fmt.Sprintf("e_%d", i+1),
			RelType: "link",
		})
	}
}

func BenchmarkRelations(b *testing.B) {
	// Create a hub entity with many relations.
	store := kv.NewMemory(nil)
	g := graph.NewKVGraph(store, kv.Key{"bench", "g"})
	ctx := context.Background()

	hub := "hub"
	for i := 0; i < 100; i++ {
		_ = g.AddRelation(ctx, graph.Relation{
			From:    hub,
			To:      fmt.Sprintf("spoke_%d", i),
			RelType: "link",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.Relations(ctx, hub)
	}
}

func BenchmarkNeighbors(b *testing.B) {
	g := setupBenchGraph(b, 200, 1000)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		label := fmt.Sprintf("entity_%04d", i%200)
		_, _ = g.Neighbors(ctx, label)
	}
}

func BenchmarkNeighbors_Filtered(b *testing.B) {
	g := setupBenchGraph(b, 200, 1000)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		label := fmt.Sprintf("entity_%04d", i%200)
		_, _ = g.Neighbors(ctx, label, "knows")
	}
}

func BenchmarkExpand_1Hop(b *testing.B) {
	g := setupBenchGraph(b, 200, 1000)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		label := fmt.Sprintf("entity_%04d", i%200)
		_, _ = g.Expand(ctx, []string{label}, 1)
	}
}

func BenchmarkExpand_2Hops(b *testing.B) {
	g := setupBenchGraph(b, 200, 1000)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		label := fmt.Sprintf("entity_%04d", i%200)
		_, _ = g.Expand(ctx, []string{label}, 2)
	}
}

func BenchmarkListEntities(b *testing.B) {
	g := setupBenchGraph(b, 1000, 0)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, err := range g.ListEntities(ctx, "") {
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
