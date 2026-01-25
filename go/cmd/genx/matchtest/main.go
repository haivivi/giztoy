package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/haivivi/giztoy/pkg/genx/match"
	"github.com/haivivi/giztoy/pkg/genx/modelloader"
)

//go:embed rules/*
var embeddedRules embed.FS

var (
	flagModel     = flag.String("model", "", "Model pattern (e.g. gemini/flash, sf/, all)")
	flagList      = flag.Bool("list", false, "List all available models")
	flagRules     = flag.String("rules", "", "Rules directory (default: embedded rules)")
	flagTpl       = flag.String("tpl", "", "Custom prompt template file path")
	flagOutput    = flag.String("o", "", "Output JSON report to file")
	flagServe     = flag.String("serve", "", "Start HTTP server for live progress (e.g. :8080)")
	flagLoad      = flag.String("load", "", "Load existing report JSON(s) and serve (comma-separated for multiple files)")
	flagModelsDir = flag.String("models", "", "Models config directory (required)")
	flagQuiet     = flag.Bool("q", false, "Quiet mode (less output)")
	flagPrompt    = flag.Bool("prompt", false, "Print the generated system prompt and exit")
	flagVerbose   = flag.Bool("verbose", false, "Print HTTP request body for debugging")
)

// RuleFile represents a rule file (JSON or YAML) with optional tests.
type RuleFile struct {
	match.Rule `yaml:",inline"`
	Tests      []TestDef `json:"tests,omitempty" yaml:"tests,omitempty"`
}

// TestDef defines a test case within a rule file.
type TestDef struct {
	Input string            `json:"input" yaml:"input"`
	Args  map[string]string `json:"args" yaml:"args"`
}

func main() {
	flag.Parse()

	// Load and serve existing report(s)
	if *flagLoad != "" {
		paths := strings.Split(*flagLoad, ",")
		for i := range paths {
			paths[i] = strings.TrimSpace(paths[i])
		}
		report, err := loadReports(paths)
		if err != nil {
			log.Fatalf("load report: %v", err)
		}
		addr := *flagServe
		if addr == "" {
			addr = ":8080"
		}
		if err := startServer(addr, report); err != nil {
			log.Fatalf("server: %v", err)
		}
		return
	}

	// Print prompt mode - doesn't require model selection
	if *flagPrompt {
		// Load rules and compile to print prompt
		var ruleFiles []RuleFile
		var err error
		if *flagRules != "" {
			ruleFiles, err = loadRuleFilesFromDir(*flagRules)
		} else {
			ruleFiles, err = loadRuleFilesFromFS(embeddedRules, "rules")
		}
		if err != nil {
			log.Fatalf("load rules: %v", err)
		}
		var rules []*match.Rule
		for _, rf := range ruleFiles {
			rules = append(rules, &rf.Rule)
		}
		opts := loadCompileOptions()
		matcher, err := match.Compile(rules, opts...)
		if err != nil {
			log.Fatalf("compile failed: %v", err)
		}
		fmt.Println(matcher.SystemPrompt())
		return
	}

	// Models dir is required
	if *flagModelsDir == "" {
		printUsage()
		return
	}

	// Register models from configs
	modelloader.Verbose = *flagVerbose
	allModels, err := modelloader.LoadFromDir(*flagModelsDir)
	if err != nil {
		log.Fatalf("register models: %v", err)
	}

	// List models and exit
	if *flagList {
		fmt.Println("Available models:")
		for _, m := range allModels {
			fmt.Printf("  %s\n", m)
		}
		return
	}

	// Determine which models to test
	var models []string
	if *flagModel != "" {
		models = matchModels(*flagModel, allModels)
		if len(models) == 0 {
			log.Fatalf("no models matched pattern: %s", *flagModel)
		}
	} else {
		printUsage()
		return
	}

	if !*flagQuiet {
		fmt.Printf("=== Models Selected (%d) ===\n", len(models))
		for _, m := range models {
			fmt.Printf("  - %s\n", m)
		}
	}

	// Load rules and tests from JSON/YAML files
	var ruleFiles []RuleFile
	if *flagRules != "" {
		// Load from specified directory
		ruleFiles, err = loadRuleFilesFromDir(*flagRules)
	} else {
		// Load from embedded FS
		ruleFiles, err = loadRuleFilesFromFS(embeddedRules, "rules")
	}
	if err != nil {
		log.Fatalf("load rules: %v", err)
	}

	// Extract rules and tests
	var rules []*match.Rule
	var cases []TestCase
	for _, rf := range ruleFiles {
		rules = append(rules, &rf.Rule)
		for _, t := range rf.Tests {
			cases = append(cases, TestCase{
				Input: t.Input,
				Expected: ExpectedResult{
					Rule: rf.Name,
					Args: t.Args,
				},
			})
		}
	}

	// Sort rules by name
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Name < rules[j].Name
	})

	if !*flagQuiet {
		fmt.Printf("=== Loaded %d rules, %d tests ===\n", len(rules), len(cases))
		for _, r := range rules {
			fmt.Printf("  - %s (%d patterns, %d vars)\n", r.Name, len(r.Patterns), len(r.Vars))
		}
		fmt.Println()
	}

	// Compile rules
	opts := loadCompileOptions()
	matcher, err := match.Compile(rules, opts...)
	if err != nil {
		log.Fatalf("compile failed: %v", err)
	}

	if !*flagQuiet {
		fmt.Println("=== Compiled Successfully ===")
		fmt.Printf("\n=== Testing %d model(s) with %d test cases ===\n", len(models), len(cases))
	}

	ctx := context.Background()

	// Create runner
	runner := NewBenchmarkRunner(matcher, models, cases, len(rules))

	// If serve mode, start server first then run benchmark
	if *flagServe != "" {
		server := NewServer(*flagServe, runner)

		// Start server in background
		var wg sync.WaitGroup
		server.StartAsync(&wg)

		fmt.Printf("\nOpen http://%s in browser to view progress\n\n", *flagServe)
	}

	// Run benchmark
	report := runner.Run(ctx)

	// Print summary
	if !*flagQuiet {
		printSummary(report)
	}

	// Save to file
	if *flagOutput != "" {
		if err := saveReport(report, *flagOutput); err != nil {
			log.Fatalf("save report: %v", err)
		}
		fmt.Printf("\nReport saved to %s\n", *flagOutput)
	}

	// If serve mode, keep server running
	if *flagServe != "" {
		fmt.Printf("\nBenchmark complete. Server still running at http://%s\n", *flagServe)
		fmt.Println("Press Ctrl+C to stop")
		select {} // Block forever
	} else if *flagOutput != "" {
		// Print hint on how to view results
		fmt.Printf("\nTo view results in browser:\n  %s -load %s -serve :8080\n", os.Args[0], *flagOutput)
	}
}

// loadRuleFilesFromDir loads rule files recursively from a directory on disk
func loadRuleFilesFromDir(dir string) ([]RuleFile, error) {
	var ruleFiles []RuleFile

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		rf, err := parseRuleFile(data, ext)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		ruleFiles = append(ruleFiles, *rf)
		return nil
	})

	return ruleFiles, err
}

// loadRuleFilesFromFS loads rule files recursively from an embed.FS
func loadRuleFilesFromFS(fsys embed.FS, dir string) ([]RuleFile, error) {
	var ruleFiles []RuleFile

	err := fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			return nil
		}

		data, err := fsys.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		rf, err := parseRuleFile(data, ext)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		ruleFiles = append(ruleFiles, *rf)
		return nil
	})

	return ruleFiles, err
}

// parseRuleFile parses rule data based on file extension
func parseRuleFile(data []byte, ext string) (*RuleFile, error) {
	var rf RuleFile

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &rf); err != nil {
			return nil, fmt.Errorf("unmarshal yaml: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &rf); err != nil {
			return nil, fmt.Errorf("unmarshal json: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported extension: %s", ext)
	}

	return &rf, nil
}

// matchModels returns models matching the given pattern.
// Pattern can be:
//   - "all": all models
//   - exact model name: "gemini/flash"
//   - prefix: "sf/" (matches all starting with "sf/")
//   - multiple patterns: "sf/,openai/" (comma-separated)
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
			// Check exact match or prefix match
			if m == p || strings.HasPrefix(m, p) {
				matched = append(matched, m)
				seen[m] = true
			}
		}
	}

	return matched
}

// loadCompileOptions returns compile options based on flags.
func loadCompileOptions() []match.Option {
	var opts []match.Option

	if *flagTpl != "" {
		data, err := os.ReadFile(*flagTpl)
		if err != nil {
			log.Fatalf("read template %s: %v", *flagTpl, err)
		}
		opts = append(opts, match.WithTpl(string(data)))
	}

	return opts
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  matchtest -models <dir> -model <pattern>              Run benchmark")
	fmt.Println("  matchtest -models <dir> -model <pattern> -serve :8080 Run with web UI")
	fmt.Println("  matchtest -models <dir> -list                         List available models")
	fmt.Println("  matchtest -load <file.json> -serve :8080              View saved results")
	fmt.Println()
	fmt.Println("Model patterns:")
	fmt.Println("  -model gemini/flash            Exact model name")
	fmt.Println("  -model sf/                     All models starting with 'sf/'")
	fmt.Println("  -model sf/,openai/             Multiple prefixes (comma-separated)")
	fmt.Println("  -model all                     All registered models")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -models <dir>                  Models config directory (required)")
	fmt.Println("  -rules <dir>                   Rules directory (default: embedded)")
	fmt.Println("  -tpl <file.gotmpl>             Custom prompt template file")
	fmt.Println("  -o <file.json>                 Save results to JSON file")
	fmt.Println("  -serve :8080                   Start web server with live progress")
	fmt.Println("  -q                             Quiet mode (no console output)")
	fmt.Println("  -prompt                        Print system prompt and exit")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  matchtest -models ./models -model zhipu/ -serve :8080")
	fmt.Println("  matchtest -models ./models -model all -o results.json -serve :8080")
	fmt.Println("  matchtest -load results.json -serve :8080")
}
