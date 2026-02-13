package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/haivivi/giztoy/go/pkg/genx/segmentors"
)

// TestCase is a single segmentor test case loaded from YAML.
type TestCase struct {
	Name     string   `json:"name" yaml:"name"`
	Desc     string   `json:"desc" yaml:"desc"`
	Messages []string `json:"messages" yaml:"messages"`
	Schema   *segmentors.Schema `json:"schema,omitempty" yaml:"schema,omitempty"`
	Expect   Expect   `json:"expect" yaml:"expect"`
	Tier     string   `json:"tier" yaml:"tier"` // set by loader: "simple", "complex", "long"
}

// Expect defines the expected extraction results for a test case.
type Expect struct {
	MinEntities     int                       `json:"min_entities,omitempty" yaml:"min_entities,omitempty"`
	EntitiesContain []string                  `json:"entities_contain,omitempty" yaml:"entities_contain,omitempty"`
	EntityAttrs     map[string]map[string]any `json:"entity_attrs,omitempty" yaml:"entity_attrs,omitempty"`
	RelationsContain []ExpectedRelation       `json:"relations_contain,omitempty" yaml:"relations_contain,omitempty"`
	SummaryContains []string                  `json:"summary_contains,omitempty" yaml:"summary_contains,omitempty"`
	LabelsContain   []string                  `json:"labels_contain,omitempty" yaml:"labels_contain,omitempty"`
	MinKeywords     int                       `json:"min_keywords,omitempty" yaml:"min_keywords,omitempty"`
}

// ExpectedRelation is an expected relation in the output.
type ExpectedRelation struct {
	From    string `json:"from" yaml:"from"`
	To      string `json:"to" yaml:"to"`
	RelType string `json:"rel_type" yaml:"rel_type"`
}

// ---------------------------------------------------------------------------
// Scoring
// ---------------------------------------------------------------------------

// Scores holds per-dimension scores for a single case.
type Scores struct {
	Entity   float64 `json:"entity_score"`
	Attr     float64 `json:"attr_score"`
	Relation float64 `json:"relation_score"`
	Summary  float64 `json:"summary_score"`
	Format   float64 `json:"format_score"`
	Overall  float64 `json:"overall"`
}

// Score weights.
const (
	weightEntity   = 0.30
	weightAttr     = 0.25
	weightRelation = 0.20
	weightSummary  = 0.15
	weightFormat   = 0.10
)

func computeScores(expect Expect, result *segmentors.Result) Scores {
	var s Scores
	s.Entity = scoreEntities(expect, result)
	s.Attr = scoreAttrs(expect, result)
	s.Relation = scoreRelations(expect, result)
	s.Summary = scoreSummary(expect, result)
	s.Format = scoreFormat(result)
	s.Overall = s.Entity*weightEntity +
		s.Attr*weightAttr +
		s.Relation*weightRelation +
		s.Summary*weightSummary +
		s.Format*weightFormat
	return s
}

// scoreEntities: fraction of expected entity labels found.
func scoreEntities(expect Expect, result *segmentors.Result) float64 {
	if len(expect.EntitiesContain) == 0 && expect.MinEntities == 0 {
		return 1.0 // nothing to check
	}

	actualLabels := make(map[string]bool)
	for _, e := range result.Entities {
		actualLabels[e.Label] = true
	}

	score := 1.0

	// Check entities_contain
	if len(expect.EntitiesContain) > 0 {
		found := 0
		for _, label := range expect.EntitiesContain {
			if actualLabels[label] {
				found++
			}
		}
		score = float64(found) / float64(len(expect.EntitiesContain))
	}

	// Check min_entities
	if expect.MinEntities > 0 && len(result.Entities) < expect.MinEntities {
		penalty := float64(len(result.Entities)) / float64(expect.MinEntities)
		score = math.Min(score, penalty)
	}

	return score
}

// scoreAttrs: fraction of expected entity attributes with correct values.
func scoreAttrs(expect Expect, result *segmentors.Result) float64 {
	if len(expect.EntityAttrs) == 0 {
		return 1.0
	}

	// Build map: label -> attrs
	actualAttrs := make(map[string]map[string]any)
	for _, e := range result.Entities {
		actualAttrs[e.Label] = e.Attrs
	}

	total := 0
	matched := 0
	for label, expectedAttrs := range expect.EntityAttrs {
		actual, ok := actualAttrs[label]
		for key, expectedVal := range expectedAttrs {
			total++
			if !ok {
				continue
			}
			actualVal, exists := actual[key]
			if !exists {
				continue
			}
			if attrValuesMatch(expectedVal, actualVal) {
				matched++
			}
		}
	}

	if total == 0 {
		return 1.0
	}
	return float64(matched) / float64(total)
}

// attrValuesMatch compares expected and actual attribute values loosely.
func attrValuesMatch(expected, actual any) bool {
	// Normalize both to strings for comparison.
	es := fmt.Sprintf("%v", expected)
	as := fmt.Sprintf("%v", actual)
	if es == as {
		return true
	}
	// Try numeric comparison (JSON numbers are float64).
	ef, eOK := toFloat64(expected)
	af, aOK := toFloat64(actual)
	if eOK && aOK {
		return math.Abs(ef-af) < 0.001
	}
	// Case-insensitive string match.
	return strings.EqualFold(es, as)
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// scoreRelations: fraction of expected relations found.
func scoreRelations(expect Expect, result *segmentors.Result) float64 {
	if len(expect.RelationsContain) == 0 {
		return 1.0
	}

	found := 0
	for _, er := range expect.RelationsContain {
		for _, ar := range result.Relations {
			if relationsMatch(er, ar) {
				found++
				break
			}
		}
	}
	return float64(found) / float64(len(expect.RelationsContain))
}

// relationsMatch checks if an actual relation matches an expected one.
// Allows fuzzy rel_type matching (e.g., "parent" matches "parent_of", "mother" etc.).
func relationsMatch(expected ExpectedRelation, actual segmentors.RelationOutput) bool {
	if actual.From != expected.From || actual.To != expected.To {
		// Also try reversed direction for symmetric relations like "sibling".
		reversed := actual.From == expected.To && actual.To == expected.From
		if !reversed {
			return false
		}
	}
	// Fuzzy rel_type: contains check.
	return strings.Contains(strings.ToLower(actual.RelType), strings.ToLower(expected.RelType)) ||
		strings.Contains(strings.ToLower(expected.RelType), strings.ToLower(actual.RelType))
}

// scoreSummary: fraction of expected keywords found in summary.
func scoreSummary(expect Expect, result *segmentors.Result) float64 {
	if len(expect.SummaryContains) == 0 && expect.MinKeywords == 0 {
		return 1.0
	}

	score := 1.0

	// Check summary_contains
	if len(expect.SummaryContains) > 0 {
		found := 0
		summary := strings.ToLower(result.Segment.Summary)
		for _, term := range expect.SummaryContains {
			if strings.Contains(summary, strings.ToLower(term)) {
				found++
			}
		}
		score = float64(found) / float64(len(expect.SummaryContains))
	}

	// Check min_keywords
	if expect.MinKeywords > 0 && len(result.Segment.Keywords) < expect.MinKeywords {
		penalty := float64(len(result.Segment.Keywords)) / float64(expect.MinKeywords)
		score = math.Min(score, penalty)
	}

	return score
}

// scoreFormat: 1.0 if all labels follow "type:name" format, 0.0 otherwise.
func scoreFormat(result *segmentors.Result) float64 {
	total := 0
	valid := 0

	for _, e := range result.Entities {
		total++
		if isValidLabel(e.Label) {
			valid++
		}
	}
	for _, l := range result.Segment.Labels {
		total++
		if isValidLabel(l) {
			valid++
		}
	}
	// Check no empty summaries.
	if strings.TrimSpace(result.Segment.Summary) == "" {
		return 0.0
	}

	if total == 0 {
		return 1.0
	}
	return float64(valid) / float64(total)
}

func isValidLabel(label string) bool {
	parts := strings.SplitN(label, ":", 2)
	if len(parts) != 2 {
		return false
	}
	// Type must be lowercase, non-empty.
	typ := parts[0]
	name := parts[1]
	return typ != "" && name != "" && typ == strings.ToLower(typ)
}

// ---------------------------------------------------------------------------
// Case Result / Model Result / Report
// ---------------------------------------------------------------------------

// CaseResult holds the result of running one test case.
type CaseResult struct {
	Name       string           `json:"name"`
	Tier       string           `json:"tier"`
	DurationMs int64            `json:"duration_ms"`
	Status     string           `json:"status"` // "pass", "fail", "error"
	Error      string           `json:"error,omitempty"`
	Scores     Scores           `json:"scores"`
	Result     *segmentors.Result `json:"result,omitempty"`
}

// ModelResult holds aggregate results for one model.
type ModelResult struct {
	Model        string             `json:"model"`
	Total        int                `json:"total"`
	Passed       int                `json:"passed"`
	Failed       int                `json:"failed"`
	Errors       int                `json:"errors"`
	PassRate     float64            `json:"pass_rate"`
	AvgScore     float64            `json:"avg_score"`
	ScoresByTier map[string]float64 `json:"scores_by_tier"`
	P50Ms        int64              `json:"p50_ms"`
	P95Ms        int64              `json:"p95_ms"`
	Cases        []CaseResult       `json:"cases"`
}

// BenchmarkReport is the full benchmark report.
type BenchmarkReport struct {
	Timestamp string        `json:"timestamp"`
	TestCount int           `json:"test_count"`
	Threshold float64       `json:"threshold"`
	Models    []ModelResult `json:"models"`
}

// passThreshold is the minimum overall score for a case to "pass".
const passThreshold = 0.7

// ---------------------------------------------------------------------------
// Runner
// ---------------------------------------------------------------------------

func runBenchmark(ctx context.Context, models []string, cases []TestCase, quiet bool) *BenchmarkReport {
	report := &BenchmarkReport{
		Timestamp: time.Now().Format(time.RFC3339),
		TestCount: len(cases),
		Threshold: passThreshold,
	}

	for _, model := range models {
		if ctx.Err() != nil {
			break
		}
		mr := runModel(ctx, model, cases, quiet)
		report.Models = append(report.Models, mr)
	}

	return report
}

func runModel(ctx context.Context, model string, cases []TestCase, quiet bool) ModelResult {
	if !quiet {
		fmt.Printf("\n=== Benchmarking: %s ===\n", model)
	}

	mr := ModelResult{
		Model:        model,
		Total:        len(cases),
		ScoresByTier: make(map[string]float64),
	}

	tierScores := make(map[string][]float64)
	var durations []int64
	var totalScore float64

	for i, tc := range cases {
		if ctx.Err() != nil {
			break
		}
		if !quiet {
			fmt.Printf("  [%d/%d] %s ... ", i+1, len(cases), tc.Name)
		}

		cr := runSingleCase(ctx, model, tc)
		mr.Cases = append(mr.Cases, cr)
		durations = append(durations, cr.DurationMs)
		totalScore += cr.Scores.Overall
		tierScores[tc.Tier] = append(tierScores[tc.Tier], cr.Scores.Overall)

		switch cr.Status {
		case "pass":
			mr.Passed++
			if !quiet {
				fmt.Printf("PASS (%.2f, %dms)\n", cr.Scores.Overall, cr.DurationMs)
			}
		case "fail":
			mr.Failed++
			if !quiet {
				fmt.Printf("FAIL (%.2f, %dms)\n", cr.Scores.Overall, cr.DurationMs)
			}
		case "error":
			mr.Errors++
			if !quiet {
				fmt.Printf("ERROR: %s\n", cr.Error)
			}
		}
	}

	if mr.Total > 0 {
		mr.PassRate = float64(mr.Passed) / float64(mr.Total) * 100
		mr.AvgScore = totalScore / float64(mr.Total)
		mr.P50Ms, mr.P95Ms = calcPercentiles(durations)
	}

	for tier, scores := range tierScores {
		sum := 0.0
		for _, s := range scores {
			sum += s
		}
		mr.ScoresByTier[tier] = sum / float64(len(scores))
	}

	return mr
}

func runSingleCase(ctx context.Context, model string, tc TestCase) CaseResult {
	cr := CaseResult{
		Name: tc.Name,
		Tier: tc.Tier,
	}

	input := segmentors.Input{
		Messages: tc.Messages,
		Schema:   tc.Schema,
	}

	start := time.Now()
	result, err := segmentors.Process(ctx, model, input)
	cr.DurationMs = time.Since(start).Milliseconds()

	if err != nil {
		cr.Status = "error"
		cr.Error = err.Error()
		return cr
	}

	cr.Result = result
	cr.Scores = computeScores(tc.Expect, result)

	if cr.Scores.Overall >= passThreshold {
		cr.Status = "pass"
	} else {
		cr.Status = "fail"
	}

	return cr
}

func calcPercentiles(durations []int64) (p50, p95 int64) {
	if len(durations) == 0 {
		return 0, 0
	}
	slices.Sort(durations)
	n := len(durations)
	p50 = durations[(n-1)*50/100]
	p95 = durations[(n-1)*95/100]
	return
}

func saveReport(report *BenchmarkReport, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func printSummary(report *BenchmarkReport) {
	fmt.Println("\n" + strings.Repeat("=", 120))
	fmt.Println("SEGTEST BENCHMARK SUMMARY")
	fmt.Println(strings.Repeat("=", 120))

	fmt.Printf("\n%-30s %6s %6s %6s %6s %8s %8s %8s %8s %8s %10s\n",
		"Model", "Total", "Pass", "Fail", "Err", "Simple", "Complex", "Long", "P50(ms)", "P95(ms)", "PassRate")
	fmt.Println(strings.Repeat("-", 120))

	for _, mr := range report.Models {
		fmt.Printf("%-30s %6d %6d %6d %6d %8.2f %8.2f %8.2f %8d %8d %9.1f%%\n",
			mr.Model, mr.Total, mr.Passed, mr.Failed, mr.Errors,
			mr.ScoresByTier["simple"], mr.ScoresByTier["complex"], mr.ScoresByTier["long"],
			mr.P50Ms, mr.P95Ms, mr.PassRate)
	}
	fmt.Println(strings.Repeat("-", 120))
}
