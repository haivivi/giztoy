// Command labeler provides end-to-end tests for memory recall with labeler integration.
//
// This test creates a real memory instance, sets up entities and relations in the graph,
// then tests the complete flow:
//  1. Get candidate labels from memory graph
//  2. Call labeler to select matching labels
//  3. Use selected labels to perform recall
//
// Usage:
//
//	bazel run //e2e/memory/labeler -- [flags]
//
// Example:
//
//	bazel run //e2e/memory/labeler -- -models ./testdata/models -labeler labeler/qwen-flash -v
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/haivivi/giztoy/go/pkg/genx/labelers"
	"github.com/haivivi/giztoy/go/pkg/genx/modelloader"
	"github.com/haivivi/giztoy/go/pkg/graph"
	"github.com/haivivi/giztoy/go/pkg/kv"
	"github.com/haivivi/giztoy/go/pkg/memory"
	"github.com/haivivi/giztoy/go/pkg/recall"
)

var verbose = flag.Bool("v", false, "verbose output")
var modelsDir = flag.String("models", "", "Models config directory (required)")
var labelerPattern = flag.String("labeler", "", "labeler pattern (required, e.g. labeler/qwen-flash)")
var list = flag.Bool("list", false, "list registered models and exit")

func main() {
	flag.Parse()

	fmt.Println("========================================")
	fmt.Println("   Memory + Labeler E2E Test")
	fmt.Println("========================================")
	fmt.Println()

	if *modelsDir == "" {
		fmt.Println("ERROR: -models is required")
		flag.Usage()
		return
	}

	modelloader.Verbose = *verbose
	allModels, err := modelloader.LoadFromDir(*modelsDir)
	if err != nil {
		fmt.Printf("ERROR: load models: %v\n", err)
		return
	}

	if *list {
		fmt.Println("Registered models:")
		for _, m := range allModels {
			fmt.Printf("  %s\n", m)
		}
		return
	}

	if *labelerPattern == "" {
		fmt.Println("ERROR: -labeler is required")
		flag.Usage()
		return
	}

	if _, err := labelers.Get(*labelerPattern); err != nil {
		fmt.Printf("ERROR: labeler %q not found after modelloader registration: %v\n", *labelerPattern, err)
		return
	}

	if *verbose {
		fmt.Printf("Models dir: %s\n", *modelsDir)
		fmt.Printf("Labeler: %s\n", *labelerPattern)
		fmt.Println()
	}

	ctx := context.Background()

	// Run tests
	passed, failed := 0, 0

	tests := []struct {
		name     string
		testFunc func(context.Context) error
	}{
		{"BasicRecallWithLabeler", testBasicRecallWithLabeler},
		{"RecallWithoutLabeler", testRecallWithoutLabeler},
		{"MultiHopGraphExpansion", testMultiHopGraphExpansion},
		{"LabelerSelectionAccuracy", testLabelerSelectionAccuracy},
	}

	for _, tc := range tests {
		fmt.Printf("\n[Test] %s...\n", tc.name)
		if err := tc.testFunc(ctx); err != nil {
			fmt.Printf("  [FAIL] %s: %v\n", tc.name, err)
			failed++
		} else {
			fmt.Printf("  [PASS] %s\n", tc.name)
			passed++
		}
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("Results: %d passed, %d failed\n", passed, failed)
	fmt.Println("========================================")

	if failed > 0 {
		os.Exit(1)
	}
}

func createTestMemory(ctx context.Context, personaID string) (*memory.Memory, func(), error) {
	store := kv.NewMemory(&kv.Options{Separator: 0x1F})

	host, err := memory.NewHost(ctx, memory.HostConfig{
		Store:     store,
		Separator: 0x1F,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create host: %w", err)
	}

	mem, err := host.Open(personaID)
	if err != nil {
		host.Close()
		return nil, nil, fmt.Errorf("open memory: %w", err)
	}

	cleanup := func() {
		mem := mem
		host := host
		if mem != nil {
			// Memory doesn't have Close(), but host does
			_ = mem
		}
		if host != nil {
			host.Close()
		}
	}

	return mem, cleanup, nil
}

func testBasicRecallWithLabeler(ctx context.Context) error {
	mem, cleanup, err := createTestMemory(ctx, "e2e-test-persona-1")
	if err != nil {
		return err
	}
	defer cleanup()

	// Setup graph with entities
	g := mem.Graph()
	if err := g.SetEntity(ctx, graph.Entity{Label: "person:小明", Attrs: map[string]any{"name": "小明", "age": 8}}); err != nil {
		return fmt.Errorf("set entity: %w", err)
	}
	if err := g.SetEntity(ctx, graph.Entity{Label: "topic:恐龙", Attrs: map[string]any{"category": "science"}}); err != nil {
		return fmt.Errorf("set entity: %w", err)
	}
	if err := g.SetEntity(ctx, graph.Entity{Label: "topic:编程", Attrs: map[string]any{"category": "technology"}}); err != nil {
		return fmt.Errorf("set entity: %w", err)
	}
	if err := g.AddRelation(ctx, graph.Relation{From: "person:小明", To: "topic:恐龙", RelType: "likes"}); err != nil {
		return fmt.Errorf("add relation: %w", err)
	}

	// Store a memory segment
	if err := mem.StoreSegment(ctx, memory.SegmentInput{
		Summary:  "和小明聊了恐龙，他最喜欢霸王龙",
		Keywords: []string{"恐龙", "霸王龙", "小明"},
		Labels:   []string{"person:小明", "topic:恐龙"},
	}, recall.Bucket1H); err != nil {
		return fmt.Errorf("store segment: %w", err)
	}

	// Get candidate labels from graph
	// Note: ListEntities returns iter.Seq2, we collect all entity labels
	var candidates []string
	for e, err := range g.ListEntities(ctx, "") {
		if err != nil {
			return fmt.Errorf("list entities: %w", err)
		}
		candidates = append(candidates, e.Label)
	}

	if *verbose {
		fmt.Printf("  Candidates from graph: %v\n", candidates)
	}

	// Call labeler to select labels
	labelerResult, err := labelers.Process(ctx, *labelerPattern, labelers.Input{
		Text:       "小明喜欢什么恐龙",
		Candidates: candidates,
		TopK:       3,
	})
	if err != nil {
		return fmt.Errorf("labeler process: %w", err)
	}

	if len(labelerResult.Matches) == 0 {
		return fmt.Errorf("labeler returned no matches")
	}

	if *verbose {
		fmt.Printf("  Labeler selected:\n")
		for _, m := range labelerResult.Matches {
			fmt.Printf("    %s (%.2f)\n", m.Label, m.Score)
		}
	}

	// Use selected labels for recall
	selectedLabels := make([]string, len(labelerResult.Matches))
	for i, m := range labelerResult.Matches {
		selectedLabels[i] = m.Label
	}

	recallResult, err := mem.Recall(ctx, memory.RecallQuery{
		Text:   "恐龙",
		Labels: selectedLabels,
		Limit:  10,
		Hops:   2,
	})
	if err != nil {
		return fmt.Errorf("recall: %w", err)
	}

	if len(recallResult.Segments) == 0 {
		return fmt.Errorf("recall returned no segments")
	}

	if *verbose {
		fmt.Printf("  Recall returned %d segments\n", len(recallResult.Segments))
		for _, s := range recallResult.Segments {
			fmt.Printf("    - %s\n", s.Summary)
		}
	}

	return nil
}

func testRecallWithoutLabeler(ctx context.Context) error {
	mem, cleanup, err := createTestMemory(ctx, "e2e-test-persona-2")
	if err != nil {
		return err
	}
	defer cleanup()

	// Store a segment without setting up graph
	if err := mem.StoreSegment(ctx, memory.SegmentInput{
		Summary:  "今天天气很好",
		Keywords: []string{"天气"},
	}, recall.Bucket1H); err != nil {
		return fmt.Errorf("store segment: %w", err)
	}

	// Recall without labels (text-only search)
	result, err := mem.Recall(ctx, memory.RecallQuery{
		Text:   "天气",
		Labels: nil,
		Limit:  10,
	})
	if err != nil {
		return fmt.Errorf("recall: %w", err)
	}

	if len(result.Segments) == 0 {
		return fmt.Errorf("expected at least one segment from text-only search")
	}

	// Should have no expanded entities (no labels provided)
	if len(result.Entities) != 0 {
		return fmt.Errorf("expected no expanded entities, got %d", len(result.Entities))
	}

	if *verbose {
		fmt.Printf("  Text-only recall returned %d segments\n", len(result.Segments))
	}

	return nil
}

func testMultiHopGraphExpansion(ctx context.Context) error {
	mem, cleanup, err := createTestMemory(ctx, "e2e-test-persona-3")
	if err != nil {
		return err
	}
	defer cleanup()

	// Setup multi-hop graph:
	// Alice -> Bob -> Carol
	// Bob -> dinosaurs
	g := mem.Graph()
	if err := g.SetEntity(ctx, graph.Entity{Label: "person:Alice"}); err != nil {
		return err
	}
	if err := g.SetEntity(ctx, graph.Entity{Label: "person:Bob"}); err != nil {
		return err
	}
	if err := g.SetEntity(ctx, graph.Entity{Label: "person:Carol"}); err != nil {
		return err
	}
	if err := g.SetEntity(ctx, graph.Entity{Label: "topic:dinosaurs"}); err != nil {
		return err
	}
	if err := g.AddRelation(ctx, graph.Relation{From: "person:Alice", To: "person:Bob", RelType: "knows"}); err != nil {
		return err
	}
	if err := g.AddRelation(ctx, graph.Relation{From: "person:Bob", To: "person:Carol", RelType: "knows"}); err != nil {
		return err
	}
	if err := g.AddRelation(ctx, graph.Relation{From: "person:Bob", To: "topic:dinosaurs", RelType: "likes"}); err != nil {
		return err
	}

	// Store segments
	if err := mem.StoreSegment(ctx, memory.SegmentInput{
		Summary: "Carol talked about painting",
		Labels:  []string{"person:Carol"},
	}, recall.Bucket1H); err != nil {
		return err
	}
	if err := mem.StoreSegment(ctx, memory.SegmentInput{
		Summary: "Bob loves dinosaurs",
		Labels:  []string{"person:Bob", "topic:dinosaurs"},
	}, recall.Bucket1H); err != nil {
		return err
	}

	// Recall starting from Alice with 2 hops
	// Should expand to: Alice, Bob, Carol, dinosaurs
	result, err := mem.Recall(ctx, memory.RecallQuery{
		Text:   "dinosaurs",
		Labels: []string{"person:Alice"},
		Hops:   2,
		Limit:  10,
	})
	if err != nil {
		return fmt.Errorf("recall: %w", err)
	}

	// Should find Bob's dinosaur segment via graph expansion
	foundDinoSegment := false
	for _, s := range result.Segments {
		if s.Summary == "Bob loves dinosaurs" {
			foundDinoSegment = true
			break
		}
	}

	if !foundDinoSegment {
		return fmt.Errorf("expected to find Bob's dinosaur segment via 2-hop expansion")
	}

	// Should have expanded entities (Alice, Bob, Carol, dinosaurs)
	if len(result.Entities) < 3 {
		return fmt.Errorf("expected at least 3 expanded entities, got %d", len(result.Entities))
	}

	if *verbose {
		fmt.Printf("  2-hop expansion found %d entities and %d segments\n", len(result.Entities), len(result.Segments))
	}

	return nil
}

func testLabelerSelectionAccuracy(ctx context.Context) error {
	mem, cleanup, err := createTestMemory(ctx, "e2e-test-persona-4")
	if err != nil {
		return err
	}
	defer cleanup()

	// Setup specific entities
	g := mem.Graph()
	entities := []string{
		"person:小红",
		"person:小明",
		"topic:画画",
		"topic:编程",
		"place:上海",
		"place:北京",
	}
	for _, e := range entities {
		if err := g.SetEntity(ctx, graph.Entity{Label: e}); err != nil {
			return err
		}
	}

	// Test query specifically about 小红 and 画画
	query := "小红今天画了一只猫"
	result, err := labelers.Process(ctx, *labelerPattern, labelers.Input{
		Text:       query,
		Candidates: entities,
		TopK:       2,
	})
	if err != nil {
		return fmt.Errorf("labeler process: %w", err)
	}

	if len(result.Matches) == 0 {
		return fmt.Errorf("expected at least one match for specific query")
	}

	// Check if we got the right entities
	foundXiaohong := false
	foundDrawing := false
	for _, m := range result.Matches {
		if m.Label == "person:小红" {
			foundXiaohong = true
		}
		if m.Label == "topic:画画" {
			foundDrawing = true
		}
	}

	if *verbose {
		fmt.Printf("  Query: %s\n", query)
		fmt.Printf("  Selected: %v\n", result.Matches)
		fmt.Printf("  Found 小红: %v, Found 画画: %v\n", foundXiaohong, foundDrawing)
	}

	// Note: We don't fail if the labeler doesn't select exactly these,
	// as LLM behavior can vary. We just log it for manual inspection.
	if !foundXiaohong && !foundDrawing {
		fmt.Printf("  [WARN] Labeler did not select expected entities (小红/画画)\n")
	}

	return nil
}
