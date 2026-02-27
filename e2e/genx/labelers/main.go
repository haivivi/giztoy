// Command labelers provides end-to-end tests for the GenX labeler functionality.
//
// This test uses modelloader configs (same as other genx e2e tools) to register
// generators/labelers, then makes real LLM calls to verify label selection.
//
// Usage:
//
//	bazel run //e2e/genx/labelers -- -models <dir> -labeler <pattern>
//
// Example:
//
//	bazel run //e2e/genx/labelers -- -models ./testdata/models -labeler labeler/qwen-flash -v
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/haivivi/giztoy/go/pkg/genx/labelers"
	"github.com/haivivi/giztoy/go/pkg/genx/modelloader"
)

var verbose = flag.Bool("v", false, "verbose output")
var modelsDir = flag.String("models", "", "Models config directory (required)")
var labelerPattern = flag.String("labeler", "", "labeler pattern (required, e.g. labeler/qwen-flash)")
var list = flag.Bool("list", false, "list registered models and exit")

func main() {
	flag.Parse()

	fmt.Println("========================================")
	fmt.Println("   GenX Labeler E2E Test")
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

	if *verbose {
		fmt.Printf("Models dir: %s\n", *modelsDir)
		fmt.Printf("Labeler: %s\n", *labelerPattern)
		fmt.Println()
	}

	ctx := context.Background()
	if _, err := labelers.Get(*labelerPattern); err != nil {
		fmt.Printf("ERROR: labeler %q not found after modelloader registration: %v\n", *labelerPattern, err)
		return
	}

	// Run tests
	passed, failed := 0, 0

	tests := []struct {
		name     string
		testFunc func(context.Context) error
	}{
		{"BasicLabelSelection", testBasicLabelSelection},
		{"MultiLabelSelection", testMultiLabelSelection},
		{"TopKLimit", testTopKLimit},
		{"NoMatchScenario", testNoMatchScenario},
		{"WithAliases", testWithAliases},
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

func testBasicLabelSelection(ctx context.Context) error {
	candidates := []string{
		"person:小明",
		"person:小红",
		"topic:恐龙",
		"topic:编程",
		"place:北京",
	}

	result, err := labelers.Process(ctx, *labelerPattern, labelers.Input{
		Text:       "我昨天和小明聊了恐龙",
		Candidates: candidates,
	})
	if err != nil {
		return fmt.Errorf("process failed: %w", err)
	}

	if len(result.Matches) == 0 {
		return fmt.Errorf("expected at least one match, got none")
	}

	// Check that we got reasonable matches
	foundPerson := false
	for _, m := range result.Matches {
		if *verbose {
			fmt.Printf("  Match: %s (score: %.2f)\n", m.Label, m.Score)
		}
		if strings.HasPrefix(m.Label, "person:") {
			foundPerson = true
		}
		if m.Score < 0 || m.Score > 1 {
			return fmt.Errorf("invalid score %.2f for %s", m.Score, m.Label)
		}
	}

	if !foundPerson {
		return fmt.Errorf("expected at least one person label")
	}

	return nil
}

func testMultiLabelSelection(ctx context.Context) error {
	candidates := []string{
		"person:小明",
		"person:小红",
		"person:张三",
		"topic:恐龙",
		"topic:人工智能",
		"topic:画画",
		"place:北京",
		"place:上海",
	}

	result, err := labelers.Process(ctx, *labelerPattern, labelers.Input{
		Text:       "小红今天在上海画恐龙",
		Candidates: candidates,
	})
	if err != nil {
		return fmt.Errorf("process failed: %w", err)
	}

	if len(result.Matches) < 2 {
		return fmt.Errorf("expected at least 2 matches for multi-entity query, got %d", len(result.Matches))
	}

	if *verbose {
		for _, m := range result.Matches {
			fmt.Printf("  Match: %s (score: %.2f)\n", m.Label, m.Score)
		}
	}

	return nil
}

func testTopKLimit(ctx context.Context) error {
	candidates := []string{
		"person:小明",
		"person:小红",
		"topic:恐龙",
		"topic:编程",
		"place:北京",
	}

	result, err := labelers.Process(ctx, *labelerPattern, labelers.Input{
		Text:       "我昨天和小明聊了恐龙",
		Candidates: candidates,
		TopK:       2,
	})
	if err != nil {
		return fmt.Errorf("process failed: %w", err)
	}

	if len(result.Matches) > 2 {
		return fmt.Errorf("expected at most 2 matches with TopK=2, got %d", len(result.Matches))
	}

	if *verbose {
		fmt.Printf("  TopK=2 returned %d matches\n", len(result.Matches))
		for _, m := range result.Matches {
			fmt.Printf("    %s (%.2f)\n", m.Label, m.Score)
		}
	}

	return nil
}

func testNoMatchScenario(ctx context.Context) error {
	candidates := []string{
		"person:小明",
		"topic:编程",
		"place:北京",
	}

	result, err := labelers.Process(ctx, *labelerPattern, labelers.Input{
		Text:       "今天的天气真好啊", // Unrelated to candidates
		Candidates: candidates,
	})
	if err != nil {
		return fmt.Errorf("process failed: %w", err)
	}

	// It's okay to get no matches or low-confidence matches
	if *verbose {
		fmt.Printf("  Got %d matches for unrelated query\n", len(result.Matches))
		for _, m := range result.Matches {
			fmt.Printf("    %s (%.2f)\n", m.Label, m.Score)
		}
	}

	return nil
}

func testWithAliases(ctx context.Context) error {
	candidates := []string{
		"person:小明",
		"topic:恐龙",
	}

	aliases := map[string][]string{
		"person:小明": {"明明", "小明同学"},
	}

	result, err := labelers.Process(ctx, *labelerPattern, labelers.Input{
		Text:       "明明今天聊恐龙",
		Candidates: candidates,
		Aliases:    aliases,
	})
	if err != nil {
		return fmt.Errorf("process failed: %w", err)
	}

	if *verbose {
		fmt.Printf("  With aliases, got %d matches:\n", len(result.Matches))
		for _, m := range result.Matches {
			fmt.Printf("    %s (%.2f)\n", m.Label, m.Score)
		}
	}

	return nil
}

// Helpers
const (
	colorRed   = "\033[0;31m"
	colorGreen = "\033[0;32m"
	colorReset = "\033[0m"
)

func init() {
	// Prevent test timeout issues
	if t := os.Getenv("TEST_TIMEOUT"); t == "" {
		// Set a longer timeout for E2E tests
		time.AfterFunc(5*time.Minute, func() {
			fmt.Println("\nERROR: Test timeout (5 minutes)")
			os.Exit(1)
		})
	}
}
