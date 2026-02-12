package commands

import (
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
	memoryCmd.AddCommand(memAddCmd, memSearchCmd, memRecallCmd, memEntityCmd, memRelationCmd)
	rootCmd.AddCommand(memoryCmd)
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
		host := memory.NewHost(memory.HostConfig{
			Store:     store,
			Vec:       hnsw,
			Embedder:  emb,
			Separator: memorySep,
		})
		mem := host.Open(memPersona)
		return &memoryEnv{
			mem: mem, host: host, store: store, hnsw: hnsw, dataDir: dir,
		}, nil
	}

	// No API key — run without vector search.
	fmt.Fprintln(os.Stderr, "Warning: DASHSCOPE_API_KEY not set, vector search disabled (keyword + label only)")
	host := memory.NewHost(memory.HostConfig{
		Store:     store,
		Separator: memorySep,
	})
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
