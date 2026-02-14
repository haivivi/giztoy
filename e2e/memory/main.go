// Command memory-e2e tests the full memory compression pipeline with real LLMs.
//
// It creates a memory Host with an LLMCompressor backed by a registered
// segmentor, appends conversation messages, runs compression, and verifies
// that entities, relations, segments, and recall all work end-to-end.
//
// Usage:
//
//	memory-e2e -models <dir> -seg <model>
//	memory-e2e -models <dir> -seg <model> -verbose
//	memory-e2e -models <dir> -list
//
// Example:
//
//	memory-e2e -models ./testdata/models -seg seg/qwen-flash
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/haivivi/giztoy/go/pkg/genx/modelloader"
	"github.com/haivivi/giztoy/go/pkg/graph"
	"github.com/haivivi/giztoy/go/pkg/kv"
	"github.com/haivivi/giztoy/go/pkg/memory"
)

var (
	flagModelsDir = flag.String("models", "", "Models config directory (required for run)")
	flagSeg       = flag.String("seg", "", "Segmentor model pattern (e.g., seg/qwen-flash)")
	flagProf      = flag.String("prof", "", "Profiler model pattern (optional)")
	flagList      = flag.Bool("list", false, "List all available models")
	flagVerbose   = flag.Bool("verbose", false, "Verbose output (print HTTP request body)")
	flagGenerate  = flag.String("generate", "", "Generate test cases to directory (creates dir + tar.gz)")
	flagCases     = flag.String("cases", "", "Test cases tar.gz or directory to run")
	flagCase      = flag.String("case", "", "Run a specific case (name or prefix, default: all)")
)

func main() {
	flag.Parse()

	// Generate mode.
	if *flagGenerate != "" {
		if err := generateMemoryCases(*flagGenerate); err != nil {
			log.Fatalf("generate: %v", err)
		}
		return
	}

	// From here on we need -models.
	if *flagModelsDir == "" {
		printUsage()
		os.Exit(1)
	}

	modelloader.Verbose = *flagVerbose
	allModels, err := modelloader.LoadFromDir(*flagModelsDir)
	if err != nil {
		log.Fatalf("load models: %v", err)
	}

	if *flagList {
		fmt.Println("Available models:")
		for _, m := range allModels {
			fmt.Printf("  %s\n", m)
		}
		return
	}

	if *flagSeg == "" {
		printUsage()
		os.Exit(1)
	}

	// Verify the segmentor model is registered.
	segModel := *flagSeg
	found := false
	for _, m := range allModels {
		if m == segModel || strings.HasPrefix(m, segModel) {
			if m != segModel {
				segModel = m // use full name if prefix matched
			}
			found = true
			break
		}
	}
	if !found {
		log.Fatalf("segmentor model %q not found in registered models: %v", *flagSeg, allModels)
	}

	// Verify the profiler model is registered (if specified).
	profModel := *flagProf
	if profModel != "" {
		profFound := false
		for _, m := range allModels {
			if m == profModel || strings.HasPrefix(m, profModel) {
				if m != profModel {
					profModel = m
				}
				profFound = true
				break
			}
		}
		if !profFound {
			log.Fatalf("profiler model %q not found in registered models: %v", *flagProf, allModels)
		}
	}

	ctx := context.Background()

	// Data-driven mode: load cases from tar.gz or directory.
	if *flagCases != "" {
		if err := runCases(ctx, segModel, profModel, *flagCases, *flagCase); err != nil {
			log.Fatalf("FAIL: %v", err)
		}
		return
	}

	// Default: run built-in tests.
	if err := run(ctx, segModel, profModel); err != nil {
		log.Fatalf("FAIL: %v", err)
	}
	fmt.Println("\nAll checks passed.")
}

func run(ctx context.Context, segModel, profModel string) error {
	// Use unit separator so labels can contain ':'.
	const sep byte = 0x1F
	store := kv.NewMemory(&kv.Options{Separator: sep})

	// Create the LLM compressor.
	comp, err := memory.NewLLMCompressor(memory.LLMCompressorConfig{
		Segmentor: segModel,
		Profiler:  profModel,
	})
	if err != nil {
		return fmt.Errorf("create compressor: %w", err)
	}

	// Create the host with the compressor (no embedder — keyword/label only).
	host, err := memory.NewHost(ctx, memory.HostConfig{
		Store:      store,
		Compressor: comp,
		Separator:  sep,
	})
	if err != nil {
		return fmt.Errorf("create host: %w", err)
	}
	defer host.Close()

	// Open a memory for persona "小猫咪".
	mem, err := host.Open("cat_girl")
	if err != nil {
		return fmt.Errorf("open persona: %w", err)
	}

	fmt.Printf("=== Memory E2E Test ===\n")
	fmt.Printf("Segmentor: %s\n", segModel)
	if profModel != "" {
		fmt.Printf("Profiler:  %s\n", profModel)
	}
	fmt.Println()

	// --- Test 1: Conversation → Compress → Verify ---
	fmt.Println("--- Test 1: Dinosaur conversation ---")
	if err := testDinosaurConversation(ctx, mem); err != nil {
		return fmt.Errorf("test 1 (dinosaur): %w", err)
	}

	// --- Test 2: Second conversation, verify graph accumulation ---
	fmt.Println("\n--- Test 2: Cooking conversation ---")
	if err := testCookingConversation(ctx, mem); err != nil {
		return fmt.Errorf("test 2 (cooking): %w", err)
	}

	// --- Test 3: Recall across both conversations ---
	fmt.Println("\n--- Test 3: Recall ---")
	if err := testRecall(ctx, mem); err != nil {
		return fmt.Errorf("test 3 (recall): %w", err)
	}

	return nil
}

func testDinosaurConversation(ctx context.Context, mem *memory.Memory) error {
	conv := mem.OpenConversation("device-001", []string{"person:小明"})

	messages := []memory.Message{
		{Role: memory.RoleUser, Name: "小明", Content: "小猫咪！今天我看了一本恐龙的书，讲的是翼龙！", Timestamp: 1000},
		{Role: memory.RoleModel, Content: "哇，翼龙好酷啊！你知道翼龙会飞吗？", Timestamp: 2000},
		{Role: memory.RoleUser, Name: "小明", Content: "知道！书上说翼龙翅膀可以展开好几米呢！但是翼龙不算恐龙对吗？", Timestamp: 3000},
		{Role: memory.RoleModel, Content: "对的！翼龙属于翼龙目，和恐龙是亲戚，但不是同一类。就像猫和老虎是亲戚一样！", Timestamp: 4000},
		{Role: memory.RoleUser, Name: "小明", Content: "那鸟类是恐龙的后代吗？我姐姐小红说鸡是恐龙变的！", Timestamp: 5000},
		{Role: memory.RoleModel, Content: "小红说得对！现代鸟类确实是恐龙的后代。科学家发现很多恐龙其实有羽毛呢！", Timestamp: 6000},
		{Role: memory.RoleUser, Name: "小明", Content: "太酷了！我长大要当古生物学家，研究恐龙化石！", Timestamp: 7000},
		{Role: memory.RoleModel, Content: "好棒的梦想！当古生物学家可以去世界各地挖化石，一定很有趣！", Timestamp: 8000},
	}
	for _, msg := range messages {
		if err := conv.Append(ctx, msg); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}

	count, err := conv.Count(ctx)
	if err != nil {
		return fmt.Errorf("count: %w", err)
	}
	fmt.Printf("  Messages appended: %d\n", count)

	// Compress using the host's default compressor (nil = use default).
	fmt.Println("  Compressing with LLM...")
	if err := mem.Compress(ctx, conv, nil); err != nil {
		return fmt.Errorf("compress: %w", err)
	}

	// Verify conversation was cleared.
	count, err = conv.Count(ctx)
	if err != nil {
		return fmt.Errorf("count after compress: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("conversation should be cleared after compress, got %d messages", count)
	}
	fmt.Println("  Conversation cleared: OK")

	// Verify segments were stored.
	segments, err := mem.Index().RecentSegments(ctx, 10)
	if err != nil {
		return fmt.Errorf("recent segments: %w", err)
	}
	if len(segments) == 0 {
		return fmt.Errorf("expected at least one segment after compression, got 0")
	}
	fmt.Printf("  Segments stored: %d\n", len(segments))
	for i, seg := range segments {
		fmt.Printf("    %d. %s\n", i+1, seg.Summary)
		fmt.Printf("       keywords=%v labels=%v\n", seg.Keywords, seg.Labels)
	}

	// Verify entities were extracted into the graph.
	g := mem.Graph()
	entities := listEntities(ctx, g)
	if len(entities) == 0 {
		return fmt.Errorf("expected entities in graph after compression, got 0")
	}
	fmt.Printf("  Entities in graph: %d\n", len(entities))
	for _, e := range entities {
		fmt.Printf("    - %s %v\n", e.Label, e.Attrs)
	}

	// Verify relations.
	relations := listRelations(ctx, g, entities)
	fmt.Printf("  Relations: %d\n", len(relations))
	for _, r := range relations {
		fmt.Printf("    - %s -[%s]-> %s\n", r.from, r.relType, r.to)
	}

	return nil
}

func testCookingConversation(ctx context.Context, mem *memory.Memory) error {
	conv := mem.OpenConversation("device-001", []string{"person:妈妈", "person:小明"})

	messages := []memory.Message{
		{Role: memory.RoleUser, Name: "妈妈", Content: "小猫咪，今天我要教小明做蛋炒饭！", Timestamp: 20000},
		{Role: memory.RoleModel, Content: "好呀！蛋炒饭是小明最喜欢吃的！", Timestamp: 21000},
		{Role: memory.RoleUser, Name: "小明", Content: "我要放好多鸡蛋！", Timestamp: 22000},
		{Role: memory.RoleModel, Content: "哈哈，小明好期待呀！", Timestamp: 23000},
		{Role: memory.RoleUser, Name: "妈妈", Content: "周末我们还要做恐龙形状的饼干，小明最近迷恐龙嘛。", Timestamp: 24000},
		{Role: memory.RoleModel, Content: "恐龙饼干太有创意了！小明一定会很喜欢！", Timestamp: 25000},
	}
	for _, msg := range messages {
		if err := conv.Append(ctx, msg); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}

	count, err := conv.Count(ctx)
	if err != nil {
		return fmt.Errorf("count: %w", err)
	}
	fmt.Printf("  Messages appended: %d\n", count)

	fmt.Println("  Compressing with LLM...")
	if err := mem.Compress(ctx, conv, nil); err != nil {
		return fmt.Errorf("compress: %w", err)
	}

	// Verify segments accumulated.
	segments, err := mem.Index().RecentSegments(ctx, 10)
	if err != nil {
		return fmt.Errorf("recent segments: %w", err)
	}
	fmt.Printf("  Total segments now: %d\n", len(segments))
	for i, seg := range segments {
		fmt.Printf("    %d. %s\n", i+1, seg.Summary)
	}

	// Verify graph entities accumulated.
	g := mem.Graph()
	entities := listEntities(ctx, g)
	fmt.Printf("  Total entities now: %d\n", len(entities))
	for _, e := range entities {
		fmt.Printf("    - %s %v\n", e.Label, e.Attrs)
	}

	return nil
}

func testRecall(ctx context.Context, mem *memory.Memory) error {
	// Query 1: recall about dinosaurs from 小明's perspective.
	fmt.Println("  Query: person:小明 + 'dinosaurs'")
	result, err := mem.Recall(ctx, memory.RecallQuery{
		Labels: []string{"person:小明"},
		Text:   "恐龙",
		Limit:  5,
	})
	if err != nil {
		return fmt.Errorf("recall: %w", err)
	}

	fmt.Printf("  Entities found: %d\n", len(result.Entities))
	for _, e := range result.Entities {
		fmt.Printf("    - %s %v\n", e.Label, e.Attrs)
	}

	fmt.Printf("  Segments found: %d\n", len(result.Segments))
	for i, s := range result.Segments {
		fmt.Printf("    %d. [%.3f] %s\n", i+1, s.Score, s.Summary)
	}

	if len(result.Segments) == 0 {
		return fmt.Errorf("expected segments from recall, got 0")
	}

	// Query 2: recall about cooking.
	fmt.Println("\n  Query: 'cooking'")
	result2, err := mem.Recall(ctx, memory.RecallQuery{
		Text:  "做饭 蛋炒饭",
		Limit: 5,
	})
	if err != nil {
		return fmt.Errorf("recall cooking: %w", err)
	}
	fmt.Printf("  Segments found: %d\n", len(result2.Segments))
	for i, s := range result2.Segments {
		fmt.Printf("    %d. [%.3f] %s\n", i+1, s.Score, s.Summary)
	}

	return nil
}

// --- Helpers ---

type entityInfo struct {
	Label string
	Attrs map[string]any
}

type relationInfo struct {
	from    string
	to      string
	relType string
}

// listEntities returns all entities in the graph.
func listEntities(ctx context.Context, g graph.Graph) []entityInfo {
	var out []entityInfo
	for ent, err := range g.ListEntities(ctx, "") {
		if err != nil {
			break
		}
		out = append(out, entityInfo{Label: ent.Label, Attrs: ent.Attrs})
	}
	return out
}

// listRelations returns all unique relations for the given entities.
func listRelations(ctx context.Context, g graph.Graph, entities []entityInfo) []relationInfo {
	seen := make(map[string]bool)
	var out []relationInfo
	for _, e := range entities {
		rels, err := g.Relations(ctx, e.Label)
		if err != nil {
			continue
		}
		for _, r := range rels {
			key := r.From + "|" + r.RelType + "|" + r.To
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, relationInfo{from: r.From, to: r.To, relType: r.RelType})
		}
	}
	return out
}

func printUsage() {
	fmt.Println(`Memory E2E — Full memory compression pipeline test with real LLMs

Usage:
  memory-e2e -models <dir> -seg <model>                         Built-in tests
  memory-e2e -models <dir> -seg <model> -cases <tar.gz|dir>     Data-driven tests
  memory-e2e -models <dir> -seg <model> -cases <tar> -case m05  Run one case
  memory-e2e -models <dir> -list                                List models
  memory-e2e -generate <dir>                                    Generate test data

Options:
  -models <dir>      Models config directory (required for run)
  -seg <model>       Segmentor model pattern (e.g., seg/qwen-flash)
  -prof <model>      Profiler model pattern (optional)
  -cases <path>      Test cases tar.gz or directory
  -case <name>       Run specific case (name or prefix, default: all)
  -list              List all available models
  -verbose           Print HTTP request body
  -generate <dir>    Generate test cases to directory

Examples:
  memory-e2e -generate ./testdata/memory
  memory-e2e -models ./testdata/models -seg seg/qwen-flash
  memory-e2e -models ./testdata/models -seg seg/qwen-flash -cases ./testdata/memory.tar.gz
  memory-e2e -models ./testdata/models -seg seg/qwen-flash -cases ./testdata/memory.tar.gz -case m01`)
}
