package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/haivivi/giztoy/go/pkg/genx"
	"github.com/haivivi/giztoy/go/pkg/genx/match"
)

// TestCase represents a single test case with input and expected output.
type TestCase struct {
	Input    string         `json:"input"`
	Expected ExpectedResult `json:"expected"`
}

// ExpectedResult is the expected match result.
type ExpectedResult struct {
	Rule string            `json:"rule"`
	Args map[string]string `json:"args,omitempty"`
}

// ActualResult is the actual match result (JSON-friendly).
type ActualResult struct {
	Rule string            `json:"rule"`
	Args map[string]string `json:"args,omitempty"`
}

// CaseResult holds the result of running one test case.
type CaseResult struct {
	Input      string       `json:"input"`
	Expected   ActualResult `json:"expected"`
	Actual     ActualResult `json:"actual"`
	DurationMs int64        `json:"duration_ms"`
	Status     string       `json:"status"` // "pass", "fail", "error"
	Error      string       `json:"error,omitempty"`
}

// ModelResult holds aggregate results for one model.
type ModelResult struct {
	Model      string       `json:"model"`
	TotalCases int          `json:"total_cases"`
	Passed     int          `json:"passed"`
	Failed     int          `json:"failed"`
	Errors     int          `json:"errors"`
	PassRate   float64      `json:"pass_rate"`
	P50Ms      int64        `json:"p50_ms"`
	P95Ms      int64        `json:"p95_ms"`
	P99Ms      int64        `json:"p99_ms"`
	Cases      []CaseResult `json:"cases"`
}

// BenchmarkReport is the full benchmark report.
type BenchmarkReport struct {
	Timestamp string        `json:"timestamp"`
	RuleCount int           `json:"rule_count"`
	TestCount int           `json:"test_count"`
	Models    []ModelResult `json:"models"`
}

func runBenchmark(ctx context.Context, matcher *match.Matcher, models []string, cases []TestCase, ruleCount int, quiet bool) *BenchmarkReport {
	report := &BenchmarkReport{
		Timestamp: time.Now().Format(time.RFC3339),
		RuleCount: ruleCount,
		TestCount: len(cases),
	}

	for _, model := range models {
		if !quiet {
			fmt.Printf("\n=== Benchmarking: %s ===\n", model)
		}

		mr := ModelResult{
			Model:      model,
			TotalCases: len(cases),
		}

		var durations []int64
		for i, tc := range cases {
			if !quiet {
				fmt.Printf("  [%d/%d] %s ... ", i+1, len(cases), tc.Input)
			}

			cr := runSingleTest(ctx, matcher, model, tc)
			mr.Cases = append(mr.Cases, cr)
			durations = append(durations, cr.DurationMs)

			switch cr.Status {
			case "pass":
				mr.Passed++
				if !quiet {
					fmt.Printf("PASS (%dms)\n", cr.DurationMs)
				}
			case "fail":
				mr.Failed++
				if !quiet {
					fmt.Printf("FAIL (got %s, want %s) (%dms)\n", cr.Actual.Rule, cr.Expected.Rule, cr.DurationMs)
				}
			case "error":
				mr.Errors++
				if !quiet {
					fmt.Printf("ERROR (%s)\n", cr.Error)
				}
			}
		}

		if mr.TotalCases > 0 {
			mr.PassRate = float64(mr.Passed) / float64(mr.TotalCases) * 100
			mr.P50Ms, mr.P95Ms, mr.P99Ms = calcPercentiles(durations)
		}
		report.Models = append(report.Models, mr)
	}

	return report
}

func runSingleTest(ctx context.Context, matcher *match.Matcher, model string, tc TestCase) CaseResult {
	cr := CaseResult{
		Input: tc.Input,
		Expected: ActualResult{
			Rule: tc.Expected.Rule,
			Args: tc.Expected.Args,
		},
	}

	// Create user context
	mcb := &genx.ModelContextBuilder{}
	mcb.UserText("", tc.Input)
	userCtx := mcb.Build()

	// Run match with timing
	start := time.Now()
	seq := matcher.Match(ctx, model, userCtx)
	results, err := match.Collect(seq)
	cr.DurationMs = time.Since(start).Milliseconds()

	if err != nil {
		cr.Status = "error"
		cr.Error = err.Error()
		return cr
	}

	// Find the first non-empty result
	var actual match.Result
	for _, r := range results {
		if r.Rule != "" {
			actual = r
			break
		}
	}

	// Convert to ActualResult
	cr.Actual.Rule = actual.Rule
	cr.Actual.Args = make(map[string]string)
	for k, v := range actual.Args {
		if v.HasValue {
			cr.Actual.Args[k] = fmt.Sprintf("%v", v.Value)
		}
	}

	// Compare results (with special handling for "nothing")
	if compareResults(tc.Expected, actual, results) {
		cr.Status = "pass"
	} else {
		cr.Status = "fail"
	}

	return cr
}

func compareResults(expected ExpectedResult, actual match.Result, results []match.Result) bool {
	// Special handling for "nothing":
	// - expected "nothing" matches actual empty (no rule matched)
	// - expected "nothing" matches actual "nothing"
	// - expected "nothing" matches raw text containing "nothing"
	if expected.Rule == "nothing" {
		if actual.Rule == "" || actual.Rule == "nothing" {
			return true
		}
		// Check raw text for "nothing"
		for _, r := range results {
			raw := strings.TrimSpace(strings.ToLower(r.RawText))
			if raw == "nothing" || strings.Contains(raw, "nothing") {
				return true
			}
		}
		return false
	}

	// Normal comparison
	if expected.Rule != actual.Rule {
		return false
	}

	for key, expectedVal := range expected.Args {
		arg, ok := actual.Args[key]
		if !ok || !arg.HasValue {
			return false
		}
		actualVal := fmt.Sprintf("%v", arg.Value)
		if !strings.EqualFold(strings.TrimSpace(expectedVal), strings.TrimSpace(actualVal)) {
			return false
		}
	}

	return true
}

// calcPercentiles calculates P50, P95, P99 from a slice of durations.
func calcPercentiles(durations []int64) (p50, p95, p99 int64) {
	if len(durations) == 0 {
		return 0, 0, 0
	}
	slices.Sort(durations)
	n := len(durations)
	// Use (n-1)*P/100 for more accurate percentile calculation
	p50 = durations[(n-1)*50/100]
	p95 = durations[(n-1)*95/100]
	p99 = durations[(n-1)*99/100]
	return
}

func saveReport(report *BenchmarkReport, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func loadReport(path string) (*BenchmarkReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var report BenchmarkReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

// loadReports loads multiple report files and merges them into one.
func loadReports(paths []string) (*BenchmarkReport, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no report files specified")
	}

	// Load first report as base
	merged, err := loadReport(paths[0])
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", paths[0], err)
	}

	// Merge subsequent reports
	for _, path := range paths[1:] {
		report, err := loadReport(path)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", path, err)
		}
		merged.Models = append(merged.Models, report.Models...)
		merged.RuleCount = max(merged.RuleCount, report.RuleCount)
		merged.TestCount = max(merged.TestCount, report.TestCount)
	}

	return merged, nil
}

func printSummary(report *BenchmarkReport) {
	fmt.Println("\n" + strings.Repeat("=", 110))
	fmt.Println("BENCHMARK SUMMARY")
	fmt.Println(strings.Repeat("=", 110))

	fmt.Printf("\n%-25s %8s %8s %8s %8s %8s %8s %8s %10s\n", "Model", "Total", "Passed", "Failed", "Errors", "P50(ms)", "P95(ms)", "P99(ms)", "PassRate")
	fmt.Println(strings.Repeat("-", 110))

	for _, mr := range report.Models {
		fmt.Printf("%-25s %8d %8d %8d %8d %8d %8d %8d %9.1f%%\n",
			mr.Model, mr.TotalCases, mr.Passed, mr.Failed, mr.Errors, mr.P50Ms, mr.P95Ms, mr.P99Ms, mr.PassRate)
	}
	fmt.Println(strings.Repeat("-", 110))
}
