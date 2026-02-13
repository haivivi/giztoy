package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/haivivi/giztoy/go/pkg/embed"
	"github.com/haivivi/giztoy/go/pkg/graph"
	"github.com/haivivi/giztoy/go/pkg/kv"
	"github.com/haivivi/giztoy/go/pkg/memory"
	"github.com/haivivi/giztoy/go/pkg/recall"
	"github.com/haivivi/giztoy/go/pkg/vecstore"
)

// memorySep is the KV separator for the memory system.
// Using ASCII Unit Separator (0x1F) so labels can contain ':'.
const memorySep byte = 0x1F

var (
	memDataDir string
	memAPIKey  string
	memPersona string
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Personal memory system (add, search, recall, entity)",
	Long: `Interact with the personal memory system.

The memory system uses DashScope embedding for semantic search. Set the API
key via --api-key flag or DASHSCOPE_API_KEY environment variable.

Data is stored locally in a badger database with HNSW vector index.

Examples:
  # Add a memory segment
  giztoy memory add "和小明聊了恐龙" --labels "person:小明,topic:恐龙" --keywords "恐龙"

  # Search by text
  giztoy memory search "恐龙" --limit 5

  # Search with label expansion
  giztoy memory recall "恐龙" --labels "person:小明"

  # Manage entities
  giztoy memory entity set "person:小明" '{"age":8,"likes":"恐龙"}'
  giztoy memory entity get "person:小明"

  # Add relations
  giztoy memory relation add "person:小明" "topic:恐龙" --type likes`,
}

var memAddCmd = &cobra.Command{
	Use:   "add <text>",
	Short: "Add a memory segment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text := args[0]
		labels, _ := cmd.Flags().GetString("labels")
		keywords, _ := cmd.Flags().GetString("keywords")

		env, err := openMemory()
		if err != nil {
			return err
		}
		defer env.close()

		seg := memory.SegmentInput{
			Summary: text,
		}
		if labels != "" {
			seg.Labels = splitComma(labels)
		}
		if keywords != "" {
			seg.Keywords = splitComma(keywords)
		}

		if err := env.mem.StoreSegment(cmd.Context(), seg); err != nil {
			return fmt.Errorf("store segment: %w", err)
		}

		if err := env.saveVec(); err != nil {
			return fmt.Errorf("save vector index: %w", err)
		}

		fmt.Printf("Stored segment: %q\n", text)
		if len(seg.Labels) > 0 {
			fmt.Printf("  labels:   %s\n", strings.Join(seg.Labels, ", "))
		}
		if len(seg.Keywords) > 0 {
			fmt.Printf("  keywords: %s\n", strings.Join(seg.Keywords, ", "))
		}
		return nil
	},
}

var memSearchCmd = &cobra.Command{
	Use:   "search <text>",
	Short: "Search memory segments by text",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text := args[0]
		labels, _ := cmd.Flags().GetString("labels")
		limit, _ := cmd.Flags().GetInt("limit")

		env, err := openMemory()
		if err != nil {
			return err
		}
		defer env.close()

		q := recall.SearchQuery{
			Text:  text,
			Limit: limit,
		}
		if labels != "" {
			q.Labels = splitComma(labels)
		}

		results, err := env.mem.Index().SearchSegments(cmd.Context(), q)
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}

		if len(results) == 0 {
			fmt.Println("No results found.")
			return nil
		}

		fmt.Printf("Found %d result(s):\n\n", len(results))
		for i, r := range results {
			fmt.Printf("  %d. [score=%.3f] %s\n", i+1, r.Score, r.Segment.Summary)
			if len(r.Segment.Labels) > 0 {
				fmt.Printf("     labels: %s\n", strings.Join(r.Segment.Labels, ", "))
			}
			if len(r.Segment.Keywords) > 0 {
				fmt.Printf("     keywords: %s\n", strings.Join(r.Segment.Keywords, ", "))
			}
			fmt.Printf("     time: %s\n", time.Unix(0, r.Segment.Timestamp).Format(time.RFC3339))
		}
		return nil
	},
}

var memRecallCmd = &cobra.Command{
	Use:   "recall <text>",
	Short: "Full recall: graph expansion + semantic search",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text := args[0]
		labels, _ := cmd.Flags().GetString("labels")
		limit, _ := cmd.Flags().GetInt("limit")
		hops, _ := cmd.Flags().GetInt("hops")

		env, err := openMemory()
		if err != nil {
			return err
		}
		defer env.close()

		q := memory.RecallQuery{
			Text:  text,
			Limit: limit,
			Hops:  hops,
		}
		if labels != "" {
			q.Labels = splitComma(labels)
		}

		result, err := env.mem.Recall(cmd.Context(), q)
		if err != nil {
			return fmt.Errorf("recall: %w", err)
		}

		// Entities.
		if len(result.Entities) > 0 {
			fmt.Printf("Entities (%d):\n", len(result.Entities))
			for _, e := range result.Entities {
				attrs := ""
				if len(e.Attrs) > 0 {
					b, _ := json.Marshal(e.Attrs)
					attrs = " " + string(b)
				}
				fmt.Printf("  - %s%s\n", e.Label, attrs)
			}
			fmt.Println()
		}

		// Segments.
		if len(result.Segments) > 0 {
			fmt.Printf("Segments (%d):\n", len(result.Segments))
			for i, s := range result.Segments {
				fmt.Printf("  %d. [score=%.3f] %s\n", i+1, s.Score, s.Summary)
				if len(s.Labels) > 0 {
					fmt.Printf("     labels: %s\n", strings.Join(s.Labels, ", "))
				}
			}
			fmt.Println()
		}

		// Summaries.
		if len(result.Summaries) > 0 {
			fmt.Printf("Summaries (%d):\n", len(result.Summaries))
			for _, s := range result.Summaries {
				fmt.Printf("  [%s] %s\n", s.Grain, s.Summary)
			}
		}

		if len(result.Entities) == 0 && len(result.Segments) == 0 && len(result.Summaries) == 0 {
			fmt.Println("No results found.")
		}
		return nil
	},
}

var memEntityCmd = &cobra.Command{
	Use:   "entity",
	Short: "Manage graph entities",
}

var memEntitySetCmd = &cobra.Command{
	Use:   "set <label> [attrs-json]",
	Short: "Create or update an entity",
	Long: `Create or update an entity in the persona's graph.

The label is the entity identifier (e.g., "person:小明", "topic:恐龙").
Attributes are optional JSON. If the entity already exists, attributes
are merged (existing keys preserved, new keys added, provided keys updated).

Examples:
  giztoy memory entity set "person:小明" '{"age":8,"likes":"恐龙"}'
  giztoy memory entity set "topic:恐龙"
  giztoy memory entity set "self" '{"name":"小猫咪","personality":"活泼"}'`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		label := args[0]
		var attrs map[string]any
		if len(args) > 1 {
			if err := json.Unmarshal([]byte(args[1]), &attrs); err != nil {
				return fmt.Errorf("invalid attrs JSON: %w", err)
			}
		}

		env, err := openMemory()
		if err != nil {
			return err
		}
		defer env.close()

		g := env.mem.Graph()
		ctx := cmd.Context()

		// Try merge first; create if not found.
		if attrs != nil {
			mergeErr := g.MergeAttrs(ctx, label, attrs)
			if mergeErr == nil {
				fmt.Printf("Updated entity %q\n", label)
				return nil
			}
			// Fall through to create.
		}

		if err := g.SetEntity(ctx, graph.Entity{Label: label, Attrs: attrs}); err != nil {
			return fmt.Errorf("set entity: %w", err)
		}
		fmt.Printf("Created entity %q\n", label)
		return nil
	},
}

var memEntityGetCmd = &cobra.Command{
	Use:   "get <label>",
	Short: "Get an entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		label := args[0]

		env, err := openMemory()
		if err != nil {
			return err
		}
		defer env.close()

		e, err := env.mem.Graph().GetEntity(cmd.Context(), label)
		if err != nil {
			return fmt.Errorf("get entity: %w", err)
		}

		b, err := json.MarshalIndent(e, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	},
}

var memRelationCmd = &cobra.Command{
	Use:   "relation",
	Short: "Manage graph relations",
}

var memRelationAddCmd = &cobra.Command{
	Use:   "add <from> <to>",
	Short: "Add a relation between entities",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		from, to := args[0], args[1]
		relType, _ := cmd.Flags().GetString("type")
		if relType == "" {
			return fmt.Errorf("--type is required")
		}

		env, err := openMemory()
		if err != nil {
			return err
		}
		defer env.close()

		if err := env.mem.Graph().AddRelation(cmd.Context(), graph.Relation{
			From: from, To: to, RelType: relType,
		}); err != nil {
			return fmt.Errorf("add relation: %w", err)
		}
		fmt.Printf("Added relation: %s --%s--> %s\n", from, relType, to)
		return nil
	},
}

var memDemoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Run a demo scenario with realistic data",
	Long: `Populate a temporary memory database with a realistic family scenario
and run several recall queries to demonstrate the system.

The demo creates an AI cat companion (小猫咪) that has been living with
a family for a week:

  小明 (8yo boy)  → likes dinosaurs, Lego, space
  小红 (6yo girl) → likes drawing, princess stories
  妈妈            → cooks, tells bedtime stories
  爸爸            → plays music, builds Lego

It stores 17 memory segments, 12 entity nodes, 14 relations, and then
runs recall queries from different perspectives.

No DashScope API key needed — uses the real system with keyword + label
scoring (no vector search in demo mode).`,
	RunE: runMemoryDemo,
}

func init() {
	// Global memory flags.
	memoryCmd.PersistentFlags().StringVar(&memDataDir, "data-dir", "", "data directory (default: ~/.local/share/giztoy/memory)")
	memoryCmd.PersistentFlags().StringVar(&memAPIKey, "api-key", "", "DashScope API key (or DASHSCOPE_API_KEY env)")
	memoryCmd.PersistentFlags().StringVar(&memPersona, "persona", "default", "persona ID")

	// add flags.
	memAddCmd.Flags().String("labels", "", "comma-separated labels (e.g., person:小明,topic:恐龙)")
	memAddCmd.Flags().String("keywords", "", "comma-separated keywords")

	// search flags.
	memSearchCmd.Flags().String("labels", "", "comma-separated label filter")
	memSearchCmd.Flags().Int("limit", 10, "max results")

	// recall flags.
	memRecallCmd.Flags().String("labels", "", "comma-separated seed labels for graph expansion")
	memRecallCmd.Flags().Int("limit", 10, "max segments")
	memRecallCmd.Flags().Int("hops", 2, "graph expansion hops")

	// relation flags.
	memRelationAddCmd.Flags().String("type", "", "relation type (e.g., likes, knows, sibling)")

	// Wire up.
	memEntityCmd.AddCommand(memEntitySetCmd, memEntityGetCmd)
	memRelationCmd.AddCommand(memRelationAddCmd)
	memoryCmd.AddCommand(memAddCmd, memSearchCmd, memRecallCmd, memEntityCmd, memRelationCmd, memDemoCmd)
	rootCmd.AddCommand(memoryCmd)
}

// ---------------------------------------------------------------------------
// Demo command implementation
// ---------------------------------------------------------------------------

func runMemoryDemo(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// Create a temporary directory for the demo database.
	tmpDir, err := os.MkdirTemp("", "giztoy-memory-demo-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Println("=== giztoy memory demo ===")
	fmt.Printf("Data dir: %s (temporary, deleted on exit)\n\n", tmpDir)

	// Open badger + memory host. No embedding API — keyword+label only.
	kvOpts := &kv.Options{Separator: memorySep}
	store, err := kv.NewBadger(kv.BadgerOptions{
		Dir: filepath.Join(tmpDir, "data"), Options: kvOpts, Logger: silentLogger{},
	})
	if err != nil {
		return fmt.Errorf("open badger: %w", err)
	}
	defer store.Close()

	host, err := memory.NewHost(ctx, memory.HostConfig{Store: store, Separator: memorySep})
	if err != nil {
		return fmt.Errorf("create host: %w", err)
	}
	defer host.Close()
	m := host.Open("cat_girl")
	g := m.Graph()
	lt := m.LongTerm()

	// ---- Step 1: Build entity graph ----
	fmt.Println("📌 Building entity graph...")

	type entityDef struct {
		label string
		attrs map[string]any
	}
	entities := []entityDef{
		{"self", map[string]any{"name": "小猫咪", "personality": "活泼好奇", "species": "虚拟猫猫"}},
		{"person:小明", map[string]any{"name": "小明", "age": float64(8), "gender": "男", "likes": "恐龙、乐高、太空"}},
		{"person:小红", map[string]any{"name": "小红", "age": float64(6), "gender": "女", "likes": "画画、公主故事"}},
		{"person:妈妈", map[string]any{"name": "妈妈", "role": "母亲", "good_at": "做饭、讲故事"}},
		{"person:爸爸", map[string]any{"name": "爸爸", "role": "父亲", "good_at": "音乐、搭乐高"}},
		{"topic:恐龙", nil},
		{"topic:画画", nil},
		{"topic:做饭", nil},
		{"topic:音乐", nil},
		{"topic:太空", nil},
		{"topic:公主故事", nil},
		{"topic:乐高", nil},
	}
	for _, e := range entities {
		if err := g.SetEntity(ctx, graph.Entity{Label: e.label, Attrs: e.attrs}); err != nil {
			return fmt.Errorf("set entity %q: %w", e.label, err)
		}
	}
	fmt.Printf("   %d entities created\n", len(entities))

	type relDef struct{ from, to, relType string }
	relations := []relDef{
		{"person:小明", "person:小红", "sibling"},
		{"person:小红", "person:小明", "sibling"},
		{"person:妈妈", "person:小明", "parent"},
		{"person:妈妈", "person:小红", "parent"},
		{"person:爸爸", "person:小明", "parent"},
		{"person:爸爸", "person:小红", "parent"},
		{"person:小明", "topic:恐龙", "likes"},
		{"person:小明", "topic:太空", "likes"},
		{"person:小明", "topic:乐高", "likes"},
		{"person:小红", "topic:画画", "likes"},
		{"person:小红", "topic:公主故事", "likes"},
		{"person:妈妈", "topic:做饭", "good_at"},
		{"person:爸爸", "topic:音乐", "good_at"},
		{"person:爸爸", "topic:乐高", "good_at"},
	}
	for _, r := range relations {
		if err := g.AddRelation(ctx, graph.Relation{From: r.from, To: r.to, RelType: r.relType}); err != nil {
			return fmt.Errorf("add relation: %w", err)
		}
	}
	fmt.Printf("   %d relations created\n\n", len(relations))

	// ---- Step 2: Store memory segments ----
	fmt.Println("📝 Storing memory segments (1 week of interactions)...")

	type segDef struct {
		summary  string
		keywords []string
		labels   []string
	}
	segments := []segDef{
		// Day 1: 小明 dinosaur session
		{"和小明聊了恐龙，他最喜欢霸王龙", []string{"恐龙", "霸王龙"}, []string{"person:小明", "topic:恐龙"}},
		{"小明问了很多恐龙的问题，还画了一只三角龙", []string{"恐龙", "三角龙", "画画"}, []string{"person:小明", "topic:恐龙", "topic:画画"}},
		{"小明说长大想当古生物学家", []string{"恐龙", "古生物学家", "梦想"}, []string{"person:小明", "topic:恐龙"}},
		{"给小明讲了恐龙灭绝的故事，他有点伤心", []string{"恐龙", "灭绝", "故事"}, []string{"person:小明", "topic:恐龙"}},
		// Day 2: 小红 drawing session
		{"小红画了一个公主城堡，涂了粉色和金色", []string{"画画", "公主", "城堡"}, []string{"person:小红", "topic:画画", "topic:公主故事"}},
		{"和小红一起编了一个公主和小猫的故事", []string{"公主", "小猫", "故事"}, []string{"person:小红", "topic:公主故事", "self"}},
		{"小红说她的公主会骑恐龙", []string{"公主", "恐龙"}, []string{"person:小红", "topic:公主故事", "topic:恐龙"}},
		// Day 3: 妈妈 cooking
		{"妈妈教我们做了蛋炒饭，小明吃了两碗", []string{"做饭", "蛋炒饭"}, []string{"person:妈妈", "person:小明", "topic:做饭"}},
		{"妈妈说周末要做恐龙形状的饼干", []string{"做饭", "恐龙", "饼干"}, []string{"person:妈妈", "topic:做饭", "topic:恐龙"}},
		// Day 4: 爸爸 music + Lego
		{"和爸爸一起听了古典音乐，小明跟着打节拍", []string{"音乐", "古典音乐"}, []string{"person:爸爸", "person:小明", "topic:音乐"}},
		{"爸爸和小明一起拼了一个恐龙乐高模型", []string{"乐高", "恐龙"}, []string{"person:爸爸", "person:小明", "topic:乐高", "topic:恐龙"}},
		// Day 5: Museum
		{"全家去了自然博物馆看恐龙化石，小明超兴奋", []string{"博物馆", "恐龙", "化石"}, []string{"person:小明", "person:小红", "person:妈妈", "person:爸爸", "topic:恐龙"}},
		{"小红在博物馆里画了好多恐龙素描", []string{"画画", "恐龙", "素描"}, []string{"person:小红", "topic:画画", "topic:恐龙"}},
		{"小明在天文馆看了星空投影，问了黑洞的问题", []string{"太空", "天文馆", "黑洞"}, []string{"person:小明", "topic:太空"}},
		// Day 6: Bedtime stories
		{"给小明讲了宇宙探险的睡前故事", []string{"太空", "故事", "睡前"}, []string{"person:小明", "topic:太空"}},
		{"给小红讲了小猫公主和恐龙的故事，她听得好开心", []string{"公主", "恐龙", "故事"}, []string{"person:小红", "topic:公主故事", "topic:恐龙", "self"}},
		// Day 7: Art class
		{"小红今天美术课画了全家福，画里还有我", []string{"画画", "全家福", "美术课"}, []string{"person:小红", "topic:画画", "self"}},
	}
	for _, s := range segments {
		if err := m.StoreSegment(ctx, memory.SegmentInput{
			Summary: s.summary, Keywords: s.keywords, Labels: s.labels,
		}); err != nil {
			return fmt.Errorf("store segment: %w", err)
		}
	}
	fmt.Printf("   %d segments stored\n\n", len(segments))

	// ---- Step 3: Set long-term summaries ----
	if err := lt.SetLifeSummary(ctx,
		"我是小猫咪，一只虚拟猫猫伙伴。和这个家庭在一起已经半年了。小明8岁最喜欢恐龙，小红6岁喜欢画画和公主故事。",
	); err != nil {
		return fmt.Errorf("set life summary: %w", err)
	}
	fmt.Println("📚 Life summary set.\n")

	// ---- Step 4: Run recall queries ----
	queries := []struct {
		name   string
		labels []string
		text   string
	}{
		{"小明喜欢什么？(从小明出发搜索\"恐龙\")", []string{"person:小明"}, "恐龙"},
		{"小红的画画回忆(从小红出发搜索\"画画\")", []string{"person:小红"}, "画画"},
		{"妈妈做了什么饭？(从妈妈出发搜索\"做饭\")", []string{"person:妈妈"}, "做饭"},
		{"爸爸和孩子玩了什么？(从爸爸出发搜索\"乐高\")", []string{"person:爸爸"}, "乐高"},
		{"所有恐龙相关的回忆(无标签搜索\"恐龙\")", nil, "恐龙"},
	}

	for i, q := range queries {
		fmt.Printf("🔍 Query %d: %s\n", i+1, q.name)
		fmt.Printf("   labels=%v text=%q\n", q.labels, q.text)

		result, err := m.Recall(ctx, memory.RecallQuery{
			Labels: q.labels, Text: q.text, Limit: 5, Hops: 2,
		})
		if err != nil {
			return fmt.Errorf("recall: %w", err)
		}

		if len(result.Entities) > 0 {
			fmt.Printf("   Entities (%d): ", len(result.Entities))
			names := make([]string, len(result.Entities))
			for j, e := range result.Entities {
				names[j] = e.Label
			}
			fmt.Println(strings.Join(names, ", "))
		}

		if len(result.Segments) > 0 {
			fmt.Printf("   Segments (%d):\n", len(result.Segments))
			for j, s := range result.Segments {
				fmt.Printf("     %d. [%.3f] %s\n", j+1, s.Score, s.Summary)
			}
		} else {
			fmt.Println("   (no segments)")
		}

		if len(result.Summaries) > 0 {
			for _, s := range result.Summaries {
				fmt.Printf("   [%s] %s\n", s.Grain, s.Summary)
			}
		}
		fmt.Println()
	}

	// ---- Step 5: Show graph expansion ----
	fmt.Println("🌐 Graph expansion from person:小明 (2 hops):")
	expanded, err := g.Expand(ctx, []string{"person:小明"}, 2)
	if err != nil {
		return fmt.Errorf("expand: %w", err)
	}
	fmt.Printf("   %s\n\n", strings.Join(expanded, ", "))

	// ---- Step 6: Show entity details ----
	fmt.Println("👤 Entity details:")
	for _, label := range []string{"person:小明", "person:小红", "person:妈妈", "person:爸爸", "self"} {
		ent, err := g.GetEntity(ctx, label)
		if err != nil {
			continue
		}
		b, _ := json.Marshal(ent.Attrs)
		fmt.Printf("   %s → %s\n", ent.Label, string(b))
	}
	fmt.Println()

	fmt.Println("=== demo complete ===")
	return nil
}

// ---------------------------------------------------------------------------
// memoryEnv: runtime environment for the memory CLI
// ---------------------------------------------------------------------------

type memoryEnv struct {
	mem     *memory.Memory
	host    *memory.Host
	store   *kv.Badger
	hnsw    *vecstore.HNSW
	dataDir string
}

func (e *memoryEnv) close() {
	if e.host != nil {
		_ = e.host.Close()
	}
	if e.store != nil {
		_ = e.store.Close()
	}
}

func (e *memoryEnv) saveVec() error {
	if e.hnsw == nil || e.hnsw.Len() == 0 {
		return nil
	}
	path := filepath.Join(e.dataDir, "hnsw.bin")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return e.hnsw.Save(f)
}

func openMemory() (*memoryEnv, error) {
	// Resolve data directory.
	dir := memDataDir
	if dir == "" {
		dir = os.Getenv("GIZTOY_MEMORY_DIR")
	}
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home dir: %w", err)
		}
		dir = filepath.Join(home, ".local", "share", "giztoy", "memory")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	// Open badger.
	kvOpts := &kv.Options{Separator: memorySep}
	store, err := kv.NewBadger(kv.BadgerOptions{
		Dir:     filepath.Join(dir, "data"),
		Options: kvOpts,
		Logger:  silentLogger{},
	})
	if err != nil {
		return nil, fmt.Errorf("open badger: %w", err)
	}

	// Resolve API key.
	apiKey := memAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("DASHSCOPE_API_KEY")
	}

	// Set up embedder + vecstore.
	var emb embed.Embedder
	if apiKey != "" {
		emb = embed.NewDashScope(apiKey)
		dim := emb.Dimension()

		// Try loading existing HNSW index.
		hnswPath := filepath.Join(dir, "hnsw.bin")
		var hnsw *vecstore.HNSW
		if f, err := os.Open(hnswPath); err == nil {
			hnsw, err = vecstore.LoadHNSW(f)
			f.Close()
			if err != nil {
				// Corrupted index — start fresh.
				fmt.Fprintf(os.Stderr, "Warning: corrupted HNSW index, starting fresh: %v\n", err)
				hnsw = nil
			}
		}
		if hnsw == nil {
			hnsw = vecstore.NewHNSW(vecstore.HNSWConfig{Dim: dim})
		}
		ctx := context.Background()
		host, err := memory.NewHost(ctx, memory.HostConfig{
			Store:     store,
			Vec:       hnsw,
			Embedder:  emb,
			Separator: memorySep,
		})
		if err != nil {
			store.Close()
			return nil, fmt.Errorf("create host: %w", err)
		}
		mem := host.Open(memPersona)
		return &memoryEnv{
			mem: mem, host: host, store: store, hnsw: hnsw, dataDir: dir,
		}, nil
	}

	// No API key — run without vector search.
	fmt.Fprintln(os.Stderr, "Warning: DASHSCOPE_API_KEY not set, vector search disabled (keyword + label only)")
	ctx := context.Background()
	host, err := memory.NewHost(ctx, memory.HostConfig{
		Store:     store,
		Separator: memorySep,
	})
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("create host: %w", err)
	}
	mem := host.Open(memPersona)
	return &memoryEnv{
		mem: mem, host: host, store: store, dataDir: dir,
	}, nil
}

// splitComma splits a comma-separated string, trimming whitespace.
func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// silentLogger suppresses badger output.
type silentLogger struct{}

func (silentLogger) Errorf(string, ...any)   {}
func (silentLogger) Warningf(string, ...any) {}
func (silentLogger) Infof(string, ...any)    {}
func (silentLogger) Debugf(string, ...any)   {}

// Ensure silentLogger implements badger.Logger at compile time.
var _ interface {
	Errorf(string, ...any)
	Warningf(string, ...any)
	Infof(string, ...any)
	Debugf(string, ...any)
} = silentLogger{}
