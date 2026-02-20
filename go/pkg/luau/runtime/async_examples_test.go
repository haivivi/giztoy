package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// getTestdataPath returns the path to testdata/luau/runtime/ directory.
func getTestdataPath(t *testing.T) string {
	// Try bazel runfiles first
	if runfilesDir := os.Getenv("TEST_SRCDIR"); runfilesDir != "" {
		workspace := os.Getenv("TEST_WORKSPACE")
		if workspace == "" {
			workspace = "giztoy"
		}
		bazelPath := filepath.Join(runfilesDir, workspace, "testdata", "luau", "runtime")
		if _, err := os.Stat(bazelPath); err == nil {
			return bazelPath
		}
		// Try alternative workspace names
		for _, ws := range []string{"_main", "__main__"} {
			altPath := filepath.Join(runfilesDir, ws, "testdata", "luau", "runtime")
			if _, err := os.Stat(altPath); err == nil {
				return altPath
			}
		}
	}

	// Try relative path from workspace root
	candidates := []string{
		"testdata/luau/runtime",
		"../../../testdata/luau/runtime",
		"../../../../testdata/luau/runtime",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}

	t.Skip("testdata/luau/runtime not found")
	return ""
}

// runAsyncExample runs an async example script with the given input.
func runAsyncExample(t *testing.T, scriptName string, input map[string]any) (map[string]any, error) {
	t.Helper()

	testdataPath := getTestdataPath(t)
	scriptPath := filepath.Join(testdataPath, scriptName)

	source, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("Failed to read script %s: %v", scriptName, err)
	}

	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	// Create a tool context for rt:input() and rt:output()
	ctx := NewToolContext()
	if input != nil {
		ctx.SetInput(input)
	}

	rt := NewWithOptions(state, WithRuntimeContext(ctx))
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	err = rt.Run(string(source), scriptName)
	if err != nil {
		return nil, err
	}

	output, outputErr := ctx.GetOutput()
	if outputErr != nil {
		return nil, outputErr
	}
	if output == nil {
		return map[string]any{}, nil
	}

	if m, ok := output.(map[string]any); ok {
		return m, nil
	}

	return map[string]any{"raw": output}, nil
}

// TestAsyncTimeout_Basic tests basic timeout functionality.
func TestAsyncTimeout_Basic(t *testing.T) {
	result, err := runAsyncExample(t, "async_timeout.luau", map[string]any{
		"example": "basic",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result["fired"] != true {
		t.Errorf("expected fired=true, got %v", result)
	}
	if result["cancelled"] != false {
		t.Errorf("expected cancelled=false, got %v", result)
	}
}

// TestAsyncTimeout_Cancel tests timeout cancellation.
func TestAsyncTimeout_Cancel(t *testing.T) {
	result, err := runAsyncExample(t, "async_timeout.luau", map[string]any{
		"example": "cancel",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result["was_cancelled"] != true {
		t.Errorf("expected was_cancelled=true, got %v", result)
	}
	if result["result_cancelled"] != true {
		t.Errorf("expected result_cancelled=true, got %v", result)
	}
}

// TestAsyncTimeout_Multiple tests multiple concurrent timeouts.
func TestAsyncTimeout_Multiple(t *testing.T) {
	result, err := runAsyncExample(t, "async_timeout.luau", map[string]any{
		"example": "multiple",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result["all_fired"] != true {
		t.Errorf("expected all_fired=true, got %v", result)
	}
}

// TestAsyncTimeout_Deadline tests deadline pattern.
func TestAsyncTimeout_Deadline(t *testing.T) {
	result, err := runAsyncExample(t, "async_timeout.luau", map[string]any{
		"example": "deadline",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result["completed"] != true {
		t.Errorf("expected completed=true, got %v", result)
	}
	iterations, _ := result["iterations"].(float64)
	if iterations != 3 {
		t.Errorf("expected iterations=3, got %v", result["iterations"])
	}
}

// TestAsyncCombinators_All tests await_all combinator.
func TestAsyncCombinators_All(t *testing.T) {
	result, err := runAsyncExample(t, "async_combinators.luau", map[string]any{
		"example": "all",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	count, _ := result["result_count"].(float64)
	if count != 3 {
		t.Errorf("expected result_count=3, got %v", result["result_count"])
	}
}

// TestAsyncCombinators_Any tests await_any combinator.
func TestAsyncCombinators_Any(t *testing.T) {
	result, err := runAsyncExample(t, "async_combinators.luau", map[string]any{
		"example": "any",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result["has_result"] != true {
		t.Errorf("expected has_result=true, got %v", result)
	}
}

// TestAsyncCombinators_Empty tests empty combinators.
func TestAsyncCombinators_Empty(t *testing.T) {
	result, err := runAsyncExample(t, "async_combinators.luau", map[string]any{
		"example": "empty",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result["all_empty"] != true {
		t.Errorf("expected all_empty=true, got %v", result)
	}
	if result["any_nil"] != true {
		t.Errorf("expected any_nil=true, got %v", result)
	}
}

// TestAsyncConcurrent_FanOut tests fan-out pattern.
func TestAsyncConcurrent_FanOut(t *testing.T) {
	result, err := runAsyncExample(t, "async_concurrent.luau", map[string]any{
		"example": "fanout",
		"count":   float64(5),
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	completed, _ := result["completed"].(float64)
	if completed != 5 {
		t.Errorf("expected completed=5, got %v", result["completed"])
	}
}

// TestAsyncConcurrent_Staggered tests staggered start pattern.
func TestAsyncConcurrent_Staggered(t *testing.T) {
	result, err := runAsyncExample(t, "async_concurrent.luau", map[string]any{
		"example": "staggered",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result["pattern"] != "staggered" {
		t.Errorf("expected pattern=staggered, got %v", result)
	}
}

// TestAsyncConcurrent_Deadline tests deadline pattern with work.
func TestAsyncConcurrent_Deadline(t *testing.T) {
	result, err := runAsyncExample(t, "async_concurrent.luau", map[string]any{
		"example": "deadline",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	iterations, _ := result["work_iterations"].(float64)
	if iterations < 5 || iterations > 15 {
		t.Errorf("expected work_iterations between 5-15, got %v", iterations)
	}
}

// TestAsyncConcurrent_Mixed tests mixed operations.
func TestAsyncConcurrent_Mixed(t *testing.T) {
	result, err := runAsyncExample(t, "async_concurrent.luau", map[string]any{
		"example": "mixed",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	count, _ := result["result_count"].(float64)
	if count != 3 {
		t.Errorf("expected result_count=3, got %v", result["result_count"])
	}
}

// TestAsyncHTTP_WithServer tests HTTP examples with a mock server.
func TestAsyncHTTP_WithServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	testdataPath := getTestdataPath(t)
	scriptPath := filepath.Join(testdataPath, "async_http.luau")

	source, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("Failed to read script: %v", err)
	}

	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	ctx := NewToolContext()
	ctx.SetInput(map[string]any{
		"url":     server.URL,
		"example": "basic",
	})

	rt := NewWithOptions(state, WithRuntimeContext(ctx))
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	err = rt.Run(string(source), "async_http.luau")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output, outputErr := ctx.GetOutput()
	if outputErr != nil {
		t.Fatalf("Output error: %v", outputErr)
	}
	if output == nil {
		t.Fatal("expected output")
	}

	result, ok := output.(map[string]any)
	if !ok {
		t.Fatalf("expected map output, got %T", output)
	}

	status, _ := result["status"].(float64)
	if status != 200 {
		t.Errorf("expected status=200, got %v", result["status"])
	}
}

// TestAsyncHTTP_Concurrent tests concurrent HTTP requests.
func TestAsyncHTTP_Concurrent(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	testdataPath := getTestdataPath(t)
	scriptPath := filepath.Join(testdataPath, "async_http.luau")

	source, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("Failed to read script: %v", err)
	}

	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	ctx := NewToolContext()
	ctx.SetInput(map[string]any{
		"url":     server.URL,
		"example": "concurrent",
	})

	rt := NewWithOptions(state, WithRuntimeContext(ctx))
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	err = rt.Run(string(source), "async_http.luau")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	output, outputErr := ctx.GetOutput()
	if outputErr != nil {
		t.Fatalf("Output error: %v", outputErr)
	}
	if output == nil {
		t.Fatal("expected output")
	}

	result, ok := output.(map[string]any)
	if !ok {
		t.Fatalf("expected map output, got %T", output)
	}

	successCount, _ := result["success_count"].(float64)
	if successCount != 3 {
		t.Errorf("expected success_count=3, got %v", result["success_count"])
	}

	if rc := requestCount.Load(); rc != 3 {
		t.Errorf("expected 3 HTTP requests, got %d", rc)
	}
}
