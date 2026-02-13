// Command segtest benchmarks segmentor extraction quality across models.
//
// Usage:
//
//	segtest -models <dir> -model <pattern> -cases <dir>           Run benchmark
//	segtest -models <dir> -model <pattern> -cases <dir> -o report.json
//	segtest -models <dir> -list                                   List models
//	segtest -generate <dir>                                       Generate long cases
//
// Model pattern works identically to matchtest: exact name, prefix, comma-separated, "all".
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/haivivi/giztoy/go/pkg/genx/modelloader"
)

var (
	flagModel     = flag.String("model", "", "Model pattern (e.g. qwen/turbo-latest, sf/, all)")
	flagList      = flag.Bool("list", false, "List all available segmentor models")
	flagModelsDir = flag.String("models", "", "Models config directory (required)")
	flagCasesDir  = flag.String("cases", "", "Test cases directory (required for benchmark)")
	flagOutput    = flag.String("o", "", "Output JSON report to file")
	flagQuiet     = flag.Bool("q", false, "Quiet mode (less output)")
	flagVerbose   = flag.Bool("verbose", false, "Print HTTP request body for debugging")
	flagGenerate  = flag.String("generate", "", "Generate long test cases to directory")
)

func main() {
	flag.Parse()

	// Generate mode.
	if *flagGenerate != "" {
		if err := generateLongCases(*flagGenerate); err != nil {
			log.Fatalf("generate: %v", err)
		}
		return
	}

	// Models dir is always required.
	if *flagModelsDir == "" {
		printUsage()
		os.Exit(1)
	}

	// Register models from configs.
	modelloader.Verbose = *flagVerbose
	allModels, err := modelloader.LoadFromDir(*flagModelsDir)
	if err != nil {
		log.Fatalf("register models: %v", err)
	}

	// List mode.
	if *flagList {
		fmt.Println("Available models:")
		for _, m := range allModels {
			fmt.Printf("  %s\n", m)
		}
		return
	}

	// Benchmark mode — need model and cases.
	if *flagModel == "" || *flagCasesDir == "" {
		printUsage()
		os.Exit(1)
	}

	// Determine which segmentor models to test.
	// segmentor model names are registered by modelloader from type:"segmentor" configs.
	// But we also accept generator model names and use them via segmentors.DefaultMux
	// if they were registered as segmentors (the modelloader registers them).
	models := matchModels(*flagModel, allModels)
	if len(models) == 0 {
		log.Fatalf("no models matched pattern: %s\navailable: %v", *flagModel, allModels)
	}

	if !*flagQuiet {
		fmt.Printf("=== Models Selected (%d) ===\n", len(models))
		for _, m := range models {
			fmt.Printf("  - %s\n", m)
		}
	}

	// Load test cases.
	cases, err := loadCases(*flagCasesDir)
	if err != nil {
		log.Fatalf("load cases: %v", err)
	}

	if !*flagQuiet {
		tierCounts := make(map[string]int)
		for _, c := range cases {
			tierCounts[c.Tier]++
		}
		fmt.Printf("=== Loaded %d test cases ===\n", len(cases))
		for tier, count := range tierCounts {
			fmt.Printf("  %s: %d\n", tier, count)
		}
		fmt.Println()
	}

	ctx := context.Background()
	report := runBenchmark(ctx, models, cases, *flagQuiet)

	if !*flagQuiet {
		printSummary(report)
	}

	if *flagOutput != "" {
		if err := saveReport(report, *flagOutput); err != nil {
			log.Fatalf("save report: %v", err)
		}
		fmt.Printf("\nReport saved to %s\n", *flagOutput)
	}
}

// ---------------------------------------------------------------------------
// Case loading
// ---------------------------------------------------------------------------

func loadCases(dir string) ([]TestCase, error) {
	var cases []TestCase

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		var tc TestCase
		if err := yaml.Unmarshal(data, &tc); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

		// Infer tier from parent directory name.
		if tc.Tier == "" {
			tc.Tier = inferTier(path)
		}

		cases = append(cases, tc)
		return nil
	})

	return cases, err
}

// inferTier determines the tier from the directory path.
// e.g., ".../cases/simple/s01.yaml" -> "simple"
func inferTier(path string) string {
	dir := filepath.Base(filepath.Dir(path))
	switch dir {
	case "simple", "complex", "long":
		return dir
	default:
		return "unknown"
	}
}

// ---------------------------------------------------------------------------
// Model matching (same as matchtest)
// ---------------------------------------------------------------------------

func matchModels(pattern string, allModels []string) []string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "all" {
		return allModels
	}

	patterns := strings.Split(pattern, ",")
	var matched []string
	seen := make(map[string]bool)

	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "all" {
			return allModels
		}
		for _, m := range allModels {
			if seen[m] {
				continue
			}
			if m == p || strings.HasPrefix(m, p) {
				matched = append(matched, m)
				seen[m] = true
			}
		}
	}

	return matched
}

func printUsage() {
	fmt.Println(`Segtest — Segmentor E2E Benchmark

Usage:
  segtest -models <dir> -model <pattern> -cases <dir>           Run benchmark
  segtest -models <dir> -model <pattern> -cases <dir> -o report.json
  segtest -models <dir> -list                                   List available models
  segtest -generate <dir>                                       Generate long cases

Model patterns:
  -model qwen/turbo-latest       Exact model name
  -model sf/                     All models starting with 'sf/'
  -model sf/,openai/             Multiple prefixes (comma-separated)
  -model all                     All registered models

Options:
  -models <dir>      Models config directory (required)
  -cases <dir>       Test cases directory (required for benchmark)
  -o <file.json>     Save results to JSON file
  -q                 Quiet mode
  -verbose           Print HTTP request body for debugging
  -generate <dir>    Generate 30 long test case YAMLs into <dir>

Examples:
  segtest -models ./testdata/models -model qwen/turbo-latest -cases ./testdata/segtest/cases
  segtest -models ./testdata/models -model all -cases ./testdata/segtest/cases -o report.json
  segtest -models ./testdata/models -list
  segtest -generate ./testdata/segtest/cases/long`)
}
