package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/haivivi/giztoy/go/pkg/graph"
	"github.com/haivivi/giztoy/go/pkg/kv"
	"github.com/haivivi/giztoy/go/pkg/memory"
)

// runCases loads test cases from a tar.gz (or directory), then runs each
// case through the full memory pipeline: append messages → compress → verify.
func runCases(ctx context.Context, segModel, profModel, casesPath, caseFilter string) error {
	// Resolve cases directory: unpack tar.gz if needed, or use directly.
	casesDir, cleanup, err := resolveCasesDir(casesPath)
	if err != nil {
		return fmt.Errorf("resolve cases: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Discover test case directories (each has a meta.yaml).
	cases, err := discoverCases(casesDir, caseFilter)
	if err != nil {
		return fmt.Errorf("discover cases: %w", err)
	}
	if len(cases) == 0 {
		return fmt.Errorf("no cases found in %s (filter=%q)", casesDir, caseFilter)
	}

	fmt.Printf("=== Memory E2E: %d cases ===\n", len(cases))
	fmt.Printf("Segmentor: %s\n", segModel)
	if profModel != "" {
		fmt.Printf("Profiler:  %s\n", profModel)
	}
	fmt.Println()

	passed, failed := 0, 0
	for _, tc := range cases {
		fmt.Printf("--- %s (%d msgs, %d convs) ---\n", tc.meta.Name, tc.meta.Total, tc.meta.Convs)
		start := time.Now()

		if err := runOneCase(ctx, segModel, profModel, tc); err != nil {
			fmt.Printf("  FAIL: %v\n", err)
			failed++
		} else {
			elapsed := time.Since(start)
			fmt.Printf("  PASS (%s)\n", elapsed.Round(time.Millisecond))
			passed++
		}
		fmt.Println()
	}

	fmt.Printf("=== Results: %d passed, %d failed, %d total ===\n", passed, failed, passed+failed)
	if failed > 0 {
		return fmt.Errorf("%d cases failed", failed)
	}
	return nil
}

// testCase holds a loaded test case ready to run.
type testCase struct {
	meta     Meta
	dir      string // directory containing conv_*.yaml files
	convFiles []string // sorted list of conv file paths
}

// ---------------------------------------------------------------------------
// Case discovery
// ---------------------------------------------------------------------------

func discoverCases(casesDir, filter string) ([]testCase, error) {
	var cases []testCase

	entries, err := os.ReadDir(casesDir)
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		dir := filepath.Join(casesDir, e.Name())
		metaPath := filepath.Join(dir, "meta.yaml")
		if _, err := os.Stat(metaPath); err != nil {
			continue
		}

		data, err := os.ReadFile(metaPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", metaPath, err)
		}
		var meta Meta
		if err := yaml.Unmarshal(data, &meta); err != nil {
			return nil, fmt.Errorf("parse %s: %w", metaPath, err)
		}

		// Apply filter.
		if filter != "" && filter != "all" {
			if meta.Name != filter && !strings.HasPrefix(meta.Name, filter) {
				continue
			}
		}

		// Find conv files.
		convFiles, err := findConvFiles(dir)
		if err != nil {
			return nil, fmt.Errorf("find convs in %s: %w", dir, err)
		}

		cases = append(cases, testCase{meta: meta, dir: dir, convFiles: convFiles})
	}

	sort.Slice(cases, func(i, j int) bool {
		return cases[i].meta.Name < cases[j].meta.Name
	})
	return cases, nil
}

func findConvFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "conv_") && (strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")) {
			files = append(files, filepath.Join(dir, name))
		}
	}
	sort.Strings(files) // conv_001.yaml, conv_002.yaml, ...
	return files, nil
}

// ---------------------------------------------------------------------------
// Run one case
// ---------------------------------------------------------------------------

func runOneCase(ctx context.Context, segModel, profModel string, tc testCase) error {
	const sep byte = 0x1F
	store := kv.NewMemory(&kv.Options{Separator: sep})

	comp, err := memory.NewLLMCompressor(memory.LLMCompressorConfig{
		Segmentor: segModel,
		Profiler:  profModel,
	})
	if err != nil {
		return fmt.Errorf("create compressor: %w", err)
	}

	host, err := memory.NewHost(ctx, memory.HostConfig{
		Store:      store,
		Compressor: comp,
		Separator:  sep,
	})
	if err != nil {
		return fmt.Errorf("create host: %w", err)
	}
	defer host.Close()

	persona := tc.meta.Persona
	if persona == "" {
		persona = "test"
	}
	mem, err := host.Open(persona)
	if err != nil {
		return fmt.Errorf("open persona %q: %w", persona, err)
	}

	// Process conversation files. Auto-compression is handled by
	// memory.Conversation.Append based on CompressPolicy thresholds.
	totalMsgs := 0
	conv := mem.OpenConversation("main", nil)

	for i, convPath := range tc.convFiles {
		data, err := os.ReadFile(convPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", convPath, err)
		}
		var cf ConvFile
		if err := yaml.Unmarshal(data, &cf); err != nil {
			return fmt.Errorf("parse %s: %w", convPath, err)
		}

		// Append messages — auto-compression fires when thresholds are hit.
		for j, msg := range cf.Messages {
			ts := int64((i*10000 + j + 1) * 1000)
			role := memory.RoleUser
			if msg.Role == "model" || msg.Role == "assistant" {
				role = memory.RoleModel
			}
			if err := conv.Append(ctx, memory.Message{
				Role:      role,
				Name:      msg.Name,
				Content:   msg.Content,
				Timestamp: ts,
			}); err != nil {
				return fmt.Errorf("append conv %d msg %d: %w", i+1, j+1, err)
			}
		}
		totalMsgs += len(cf.Messages)

		// Progress for large cases.
		if len(tc.convFiles) > 10 && (i+1)%10 == 0 {
			fmt.Printf("  ... processed %d/%d files (%d msgs)\n",
				i+1, len(tc.convFiles), totalMsgs)
		}
	}

	// Flush any remaining uncompressed messages.
	if err := mem.Compress(ctx, conv, nil); err != nil {
		return fmt.Errorf("compress final batch: %w", err)
	}

	fmt.Printf("  Processed %d files, %d messages\n", len(tc.convFiles), totalMsgs)

	// Verify expectations.
	return verifyExpect(ctx, mem, tc.meta.Expect)
}

// ---------------------------------------------------------------------------
// Verification
// ---------------------------------------------------------------------------

func verifyExpect(ctx context.Context, mem *memory.Memory, expect MetaExpect) error {
	g := mem.Graph()

	// Count entities.
	entities := listAllEntities(ctx, g)
	fmt.Printf("  Entities: %d", len(entities))
	if expect.MinEntities > 0 {
		if len(entities) < expect.MinEntities {
			fmt.Printf(" (FAIL: want >= %d)\n", expect.MinEntities)
			return fmt.Errorf("expected >= %d entities, got %d", expect.MinEntities, len(entities))
		}
		fmt.Printf(" (>= %d OK)", expect.MinEntities)
	}
	fmt.Println()

	// Print first 10 entities.
	for i, e := range entities {
		if i >= 10 {
			fmt.Printf("    ... and %d more\n", len(entities)-10)
			break
		}
		fmt.Printf("    - %s %v\n", e.Label, e.Attrs)
	}

	// Check entities_contain.
	if len(expect.EntitiesContain) > 0 {
		labelSet := make(map[string]bool)
		for _, e := range entities {
			labelSet[e.Label] = true
		}
		for _, want := range expect.EntitiesContain {
			if !labelSet[want] {
				return fmt.Errorf("expected entity %q not found in graph", want)
			}
		}
		fmt.Printf("  Entities contain %v: OK\n", expect.EntitiesContain)
	}

	// Count relations.
	allRels := listAllRelations(ctx, g, entities)
	fmt.Printf("  Relations: %d", len(allRels))
	if expect.MinRelations > 0 {
		if len(allRels) < expect.MinRelations {
			fmt.Printf(" (FAIL: want >= %d)\n", expect.MinRelations)
			return fmt.Errorf("expected >= %d relations, got %d", expect.MinRelations, len(allRels))
		}
		fmt.Printf(" (>= %d OK)", expect.MinRelations)
	}
	fmt.Println()
	for i, r := range allRels {
		if i >= 10 {
			fmt.Printf("    ... and %d more\n", len(allRels)-10)
			break
		}
		fmt.Printf("    - %s -[%s]-> %s\n", r.from, r.relType, r.to)
	}

	// Count segments.
	segments, err := mem.Index().RecentSegments(ctx, 10000)
	if err != nil {
		return fmt.Errorf("recent segments: %w", err)
	}
	fmt.Printf("  Segments: %d", len(segments))
	if expect.MinSegments > 0 {
		if len(segments) < expect.MinSegments {
			fmt.Printf(" (FAIL: want >= %d)\n", expect.MinSegments)
			return fmt.Errorf("expected >= %d segments, got %d", expect.MinSegments, len(segments))
		}
		fmt.Printf(" (>= %d OK)", expect.MinSegments)
	}
	fmt.Println()

	// Recall queries.
	for _, rq := range expect.Recall {
		result, err := mem.Recall(ctx, memory.RecallQuery{
			Labels: rq.Labels,
			Text:   rq.Text,
			Limit:  20,
		})
		if err != nil {
			return fmt.Errorf("recall %q: %w", rq.Text, err)
		}
		fmt.Printf("  Recall %q (labels=%v): %d results", rq.Text, rq.Labels, len(result.Segments))
		if rq.MinResults > 0 && len(result.Segments) < rq.MinResults {
			fmt.Printf(" (FAIL: want >= %d)\n", rq.MinResults)
			return fmt.Errorf("recall %q: expected >= %d results, got %d", rq.Text, rq.MinResults, len(result.Segments))
		}
		fmt.Println(" OK")
		for i, s := range result.Segments {
			if i >= 3 {
				break
			}
			fmt.Printf("    %d. [%.3f] %s\n", i+1, s.Score, s.Summary)
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func listAllEntities(ctx context.Context, g graph.Graph) []entityInfo {
	var out []entityInfo
	for ent, err := range g.ListEntities(ctx, "") {
		if err != nil {
			break
		}
		out = append(out, entityInfo{Label: ent.Label, Attrs: ent.Attrs})
	}
	return out
}

func listAllRelations(ctx context.Context, g graph.Graph, entities []entityInfo) []relationInfo {
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

// resolveCasesDir returns the directory containing test case subdirectories.
// If casesPath is a .tar.gz file, it extracts to a temp dir and returns a
// cleanup function. If it's a directory, returns it directly.
func resolveCasesDir(casesPath string) (string, func(), error) {
	info, err := os.Stat(casesPath)
	if err != nil {
		return "", nil, err
	}

	// If it's already a directory, use directly.
	if info.IsDir() {
		return casesPath, nil, nil
	}

	// Must be a tar.gz — extract to temp dir.
	tmpDir, err := os.MkdirTemp("", "memory-e2e-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(tmpDir) }

	if err := unpackTarGz(casesPath, tmpDir); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("unpack %s: %w", casesPath, err)
	}

	// The tar contains a top-level directory (e.g., "memory/").
	// Find it.
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		cleanup()
		return "", nil, err
	}

	// If there's exactly one subdirectory, use it as the cases dir.
	for _, e := range entries {
		if e.IsDir() {
			subDir := filepath.Join(tmpDir, e.Name())
			// Check if this dir has meta.yaml children (it's a case dir itself)
			// or if it contains case subdirectories.
			subEntries, err := os.ReadDir(subDir)
			if err == nil {
				for _, se := range subEntries {
					if se.IsDir() {
						metaPath := filepath.Join(subDir, se.Name(), "meta.yaml")
						if _, err := os.Stat(metaPath); err == nil {
							return subDir, cleanup, nil
						}
					}
				}
			}
		}
	}

	// Fallback: the tmpDir itself might be the cases dir.
	return tmpDir, cleanup, nil
}

func unpackTarGz(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, hdr.Name)

		// Prevent path traversal.
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			return fmt.Errorf("tar entry %q tries to escape dest dir", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}
	return nil
}
