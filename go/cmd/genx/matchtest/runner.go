package main

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/haivivi/giztoy/pkg/genx"
	"github.com/haivivi/giztoy/pkg/genx/match"
)

// RunnerStatus represents the overall benchmark status
type RunnerStatus struct {
	Status     string    `json:"status"` // "idle", "running", "done"
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Duration   string    `json:"duration,omitempty"`
}

// ModelProgress tracks progress for one model
type ModelProgress struct {
	Model   string  `json:"model"`
	Status  string  `json:"status"` // "pending", "running", "done", "error"
	Total   int     `json:"total"`
	Done    int     `json:"done"`
	Passed  int     `json:"passed"`
	Failed  int     `json:"failed"`
	Errors  int     `json:"errors"`
	Error   string  `json:"error,omitempty"`
	Percent float64 `json:"percent"`
}

// ProgressUpdate is sent to subscribers
type ProgressUpdate struct {
	Type   string         `json:"type"` // "model_start", "case_done", "model_done", "all_done"
	Model  string         `json:"model,omitempty"`
	Case   *CaseResult    `json:"case,omitempty"`
	Status *RunnerStatus  `json:"status,omitempty"`
	Models []ModelProgress `json:"models,omitempty"`
}

// BenchmarkRunner manages benchmark execution with progress tracking
type BenchmarkRunner struct {
	mu sync.RWMutex

	// Configuration
	matcher   *match.Matcher
	models    []string
	cases     []TestCase
	ruleCount int

	// State
	status    RunnerStatus
	progress  map[string]*ModelProgress
	report    *BenchmarkReport

	// Subscribers for progress updates
	subscribers []chan ProgressUpdate
	subMu       sync.Mutex
}

// NewBenchmarkRunner creates a new runner
func NewBenchmarkRunner(matcher *match.Matcher, models []string, cases []TestCase, ruleCount int) *BenchmarkRunner {
	progress := make(map[string]*ModelProgress)
	for _, m := range models {
		progress[m] = &ModelProgress{
			Model:  m,
			Status: "pending",
			Total:  len(cases),
		}
	}

	return &BenchmarkRunner{
		matcher:   matcher,
		models:    models,
		cases:     cases,
		ruleCount: ruleCount,
		status:    RunnerStatus{Status: "idle"},
		progress:  progress,
	}
}

// Subscribe returns a channel for progress updates
func (r *BenchmarkRunner) Subscribe() chan ProgressUpdate {
	r.subMu.Lock()
	defer r.subMu.Unlock()

	ch := make(chan ProgressUpdate, 100)
	r.subscribers = append(r.subscribers, ch)
	return ch
}

// Unsubscribe removes a subscriber
func (r *BenchmarkRunner) Unsubscribe(ch chan ProgressUpdate) {
	r.subMu.Lock()
	defer r.subMu.Unlock()

	for i, sub := range r.subscribers {
		if sub == ch {
			r.subscribers = append(r.subscribers[:i], r.subscribers[i+1:]...)
			close(ch)
			break
		}
	}
}

// broadcast sends update to all subscribers
func (r *BenchmarkRunner) broadcast(update ProgressUpdate) {
	r.subMu.Lock()
	defer r.subMu.Unlock()

	for _, ch := range r.subscribers {
		select {
		case ch <- update:
		default:
			// Skip slow subscribers
		}
	}
}

// GetStatus returns current status
func (r *BenchmarkRunner) GetStatus() RunnerStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

// GetProgress returns progress for all models
func (r *BenchmarkRunner) GetProgress() []ModelProgress {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ModelProgress, 0, len(r.models))
	for _, m := range r.models {
		if p, ok := r.progress[m]; ok {
			result = append(result, *p)
		}
	}
	return result
}

// GetReport returns the current report (may be partial if still running)
func (r *BenchmarkRunner) GetReport() *BenchmarkReport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.report
}

// Run starts the benchmark
func (r *BenchmarkRunner) Run(ctx context.Context) *BenchmarkReport {
	r.mu.Lock()
	r.status = RunnerStatus{
		Status:    "running",
		StartedAt: time.Now(),
	}
	r.report = &BenchmarkReport{
		Timestamp: time.Now().Format(time.RFC3339),
		RuleCount: r.ruleCount,
		TestCount: len(r.cases),
	}
	r.mu.Unlock()

	// Broadcast start
	r.broadcast(ProgressUpdate{
		Type:   "start",
		Status: &r.status,
		Models: r.GetProgress(),
	})

	// Run each model sequentially (could be parallel in future)
	for _, model := range r.models {
		if ctx.Err() != nil {
			break
		}
		r.runModel(ctx, model)
	}

	// Mark done
	r.mu.Lock()
	r.status.Status = "done"
	r.status.FinishedAt = time.Now()
	r.status.Duration = r.status.FinishedAt.Sub(r.status.StartedAt).Round(time.Millisecond).String()
	r.mu.Unlock()

	// Broadcast done
	r.broadcast(ProgressUpdate{
		Type:   "all_done",
		Status: &r.status,
		Models: r.GetProgress(),
	})

	return r.report
}

func (r *BenchmarkRunner) runModel(ctx context.Context, model string) {
	// Update status
	r.mu.Lock()
	r.progress[model].Status = "running"
	r.mu.Unlock()

	r.broadcast(ProgressUpdate{
		Type:   "model_start",
		Model:  model,
		Models: r.GetProgress(),
	})

	mr := ModelResult{
		Model:      model,
		TotalCases: len(r.cases),
	}

	var durations []int64

	for _, tc := range r.cases {
		if ctx.Err() != nil {
			break
		}

		cr := r.runSingleTest(ctx, model, tc)
		mr.Cases = append(mr.Cases, cr)
		durations = append(durations, cr.DurationMs)

		// Update progress
		r.mu.Lock()
		p := r.progress[model]
		p.Done++
		p.Percent = float64(p.Done) / float64(p.Total) * 100

		switch cr.Status {
		case "pass":
			mr.Passed++
			p.Passed++
		case "fail":
			mr.Failed++
			p.Failed++
		case "error":
			mr.Errors++
			p.Errors++
		}
		r.mu.Unlock()

		// Broadcast case done
		r.broadcast(ProgressUpdate{
			Type:   "case_done",
			Model:  model,
			Case:   &cr,
			Models: r.GetProgress(),
		})
	}

	// Calculate final stats
	if mr.TotalCases > 0 {
		mr.PassRate = float64(mr.Passed) / float64(mr.TotalCases) * 100
		mr.P50Ms, mr.P95Ms, mr.P99Ms = calcPercentiles(durations)
	}

	// Update progress and report
	r.mu.Lock()
	r.progress[model].Status = "done"
	r.report.Models = append(r.report.Models, mr)
	r.mu.Unlock()

	// Broadcast model done
	r.broadcast(ProgressUpdate{
		Type:   "model_done",
		Model:  model,
		Models: r.GetProgress(),
	})
}

func (r *BenchmarkRunner) runSingleTest(ctx context.Context, model string, tc TestCase) CaseResult {
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
	seq := r.matcher.Match(ctx, model, userCtx)
	results, err := match.Collect(seq)
	cr.DurationMs = time.Since(start).Milliseconds()

	if err != nil {
		cr.Status = "error"
		cr.Error = err.Error()
		return cr
	}

	// Find the first non-empty result
	var actual match.Result
	for _, res := range results {
		if res.Rule != "" {
			actual = res
			break
		}
	}

	// Convert to ActualResult
	cr.Actual.Rule = actual.Rule
	cr.Actual.Args = make(map[string]string)
	for k, v := range actual.Args {
		if v.HasValue {
			cr.Actual.Args[k] = formatValue(v.Value)
		}
	}

	// Compare results
	if compareResults(tc.Expected, actual, results) {
		cr.Status = "pass"
	} else {
		cr.Status = "fail"
	}

	return cr
}

func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	default:
		return string(mustJSON(v))
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
