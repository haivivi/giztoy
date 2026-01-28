package luau

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/genx"
)

// testdataDir returns the path to testdata/luau/runtime directory.
func testdataDir() string {
	// Method 1: Use the test binary's location to find runfiles
	exe, err := os.Executable()
	if err == nil {
		// Bazel puts runfiles next to the binary with .runfiles suffix
		runfilesDir := exe + ".runfiles"
		if _, err := os.Stat(runfilesDir); err == nil {
			candidate := filepath.Join(runfilesDir, "_main", "testdata", "luau", "runtime")
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	// Method 2: Try RUNFILES_DIR env var
	if runfiles := os.Getenv("RUNFILES_DIR"); runfiles != "" {
		candidate := filepath.Join(runfiles, "_main", "testdata", "luau", "runtime")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Method 3: Use caller's file location (works for go test ./...)
	_, file, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(file)
		// Go up to workspace root
		for {
			candidate := filepath.Join(dir, "testdata", "luau", "runtime")
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// Method 4: Find workspace root from cwd
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "MODULE.bazel")); err == nil {
			return filepath.Join(dir, "testdata", "luau", "runtime")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Fallback
	return "testdata/luau/runtime"
}

// readScript reads a Luau script from testdata.
func readScript(t *testing.T, name string) string {
	path := filepath.Join(testdataDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read script %s: %v", name, err)
	}
	return string(data)
}

func TestRunner_SimpleScript(t *testing.T) {
	runner, err := NewRunner(nil)
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}
	defer runner.Close()

	script := readScript(t, "simple_echo.luau")

	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test", "")

	result, err := runner.RunSource(ctx, script, mockRT, mockState, "hello")
	if err != nil {
		t.Fatalf("RunSource error: %v", err)
	}

	if result != "hello" {
		t.Errorf("result = %v, want %v", result, "hello")
	}
}

func TestRunner_TableInput(t *testing.T) {
	runner, err := NewRunner(nil)
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}
	defer runner.Close()

	script := readScript(t, "table_transform.luau")

	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test", "")

	input := map[string]any{
		"name":  "test",
		"count": int64(5),
	}

	result, err := runner.RunSource(ctx, script, mockRT, mockState, input)
	if err != nil {
		t.Fatalf("RunSource error: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}

	if resultMap["name"] != "test" {
		t.Errorf("result.name = %v, want %v", resultMap["name"], "test")
	}

	// Note: Lua numbers may come back as float64
	count, ok := resultMap["count"].(int64)
	if !ok {
		countF, ok := resultMap["count"].(float64)
		if !ok {
			t.Fatalf("result.count type = %T", resultMap["count"])
		}
		count = int64(countF)
	}
	if count != 6 {
		t.Errorf("result.count = %v, want 6", count)
	}
}

func TestRunner_Generate(t *testing.T) {
	runner, err := NewRunner(nil)
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}
	defer runner.Close()

	script := readScript(t, "generate.luau")

	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockRT.GenerateStreamFunc = func(ctx context.Context, model string, mctx genx.ModelContext) (genx.Stream, error) {
		return NewMockTextStream("Hello from LLM!"), nil
	}
	mockState := NewMockState("test", "")

	input := map[string]any{
		"model":  "qwen-turbo",
		"prompt": "Say hello",
	}

	result, err := runner.RunSource(ctx, script, mockRT, mockState, input)
	if err != nil {
		t.Fatalf("RunSource error: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}

	if resultMap["text"] != "Hello from LLM!" {
		t.Errorf("result.text = %v, want %v", resultMap["text"], "Hello from LLM!")
	}
}

func TestRunner_GenerateWithContext(t *testing.T) {
	runner, err := NewRunner(nil)
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}
	defer runner.Close()

	script := readScript(t, "generate_with_context.luau")

	ctx := context.Background()
	mockRT := NewMockRuntime()

	// Track what was passed to GenerateStream
	var capturedMctx genx.ModelContext
	mockRT.GenerateStreamFunc = func(ctx context.Context, model string, mctx genx.ModelContext) (genx.Stream, error) {
		capturedMctx = mctx
		return NewMockTextStream("3+3 equals 6."), nil
	}

	mockState := NewMockState("test", "")

	result, err := runner.RunSource(ctx, script, mockRT, mockState, nil)
	if err != nil {
		t.Fatalf("RunSource error: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}

	if resultMap["response"] != "3+3 equals 6." {
		t.Errorf("response = %v, want '3+3 equals 6.'", resultMap["response"])
	}

	// Verify the model context was built correctly
	if capturedMctx == nil {
		t.Fatal("ModelContext was not captured")
	}

	// Check prompts (system)
	promptCount := 0
	for p := range capturedMctx.Prompts() {
		if p.Name == "system" && p.Text == "You are a helpful math tutor." {
			promptCount++
		}
	}
	if promptCount != 1 {
		t.Errorf("expected 1 system prompt, got %d", promptCount)
	}

	// Check messages
	msgCount := 0
	for range capturedMctx.Messages() {
		msgCount++
	}
	if msgCount != 3 {
		t.Errorf("expected 3 messages in context, got %d", msgCount)
	}
}

func TestRunner_State(t *testing.T) {
	runner, err := NewRunner(nil)
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}
	defer runner.Close()

	script := readScript(t, "state_crud.luau")

	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test", "")

	result, err := runner.RunSource(ctx, script, mockRT, mockState, nil)
	if err != nil {
		t.Fatalf("RunSource error: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}

	// Verify state was set
	v, ok := mockState.Get("counter")
	if !ok {
		t.Error("state counter not set")
	} else if v != int64(42) {
		t.Errorf("state counter = %v, want 42", v)
	}

	v, ok = mockState.Get("name")
	if !ok {
		t.Error("state name not set")
	} else if v != "test" {
		t.Errorf("state name = %v, want test", v)
	}

	// Verify deleted key is gone
	_, ok = mockState.Get("to_delete")
	if ok {
		t.Error("state to_delete should have been deleted")
	}

	// Verify Luau got nil for deleted key
	if resultMap["deleted"] != nil {
		t.Errorf("result.deleted = %v, want nil", resultMap["deleted"])
	}
}

func TestRunner_History(t *testing.T) {
	runner, err := NewRunner(nil)
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}
	defer runner.Close()

	script := readScript(t, "history_append.luau")

	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test", "")

	result, err := runner.RunSource(ctx, script, mockRT, mockState, nil)
	if err != nil {
		t.Fatalf("RunSource error: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}

	// Check message count
	count, _ := resultMap["count"].(int64)
	if count == 0 {
		countF, _ := resultMap["count"].(float64)
		count = int64(countF)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	if resultMap["first_role"] != "user" {
		t.Errorf("first_role = %v, want user", resultMap["first_role"])
	}

	if resultMap["first_content"] != "Hello" {
		t.Errorf("first_content = %v, want Hello", resultMap["first_content"])
	}
}

func TestRunner_HistoryRevert(t *testing.T) {
	runner, err := NewRunner(nil)
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}
	defer runner.Close()

	script := readScript(t, "history_revert.luau")

	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test", "")

	result, err := runner.RunSource(ctx, script, mockRT, mockState, nil)
	if err != nil {
		t.Fatalf("RunSource error: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}

	// After revert, should have fewer messages
	count, _ := resultMap["count"].(int64)
	if count == 0 {
		countF, _ := resultMap["count"].(float64)
		count = int64(countF)
	}
	// MockState revert clears all messages after last user message marker
	if count > 2 {
		t.Errorf("count = %d, want <= 2 after revert", count)
	}
}

func TestRunner_Memory(t *testing.T) {
	runner, err := NewRunner(nil)
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}
	defer runner.Close()

	script := readScript(t, "memory.luau")

	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test", "")

	result, err := runner.RunSource(ctx, script, mockRT, mockState, nil)
	if err != nil {
		t.Fatalf("RunSource error: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}

	if resultMap["summary"] != "User likes coding in Go and Luau" {
		t.Errorf("summary = %v, want 'User likes coding in Go and Luau'", resultMap["summary"])
	}
}

func TestRunner_Log(t *testing.T) {
	logger := &TestLogger{}
	runner, err := NewRunner(&RunnerConfig{Logger: logger})
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}
	defer runner.Close()

	script := readScript(t, "log.luau")

	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test", "")

	_, err = runner.RunSource(ctx, script, mockRT, mockState, nil)
	if err != nil {
		t.Fatalf("RunSource error: %v", err)
	}

	if len(logger.Messages) != 2 {
		t.Fatalf("log message count = %d, want 2", len(logger.Messages))
	}

	if logger.Messages[0].Level != "info" {
		t.Errorf("log[0].level = %s, want info", logger.Messages[0].Level)
	}

	if logger.Messages[1].Level != "error" {
		t.Errorf("log[1].level = %s, want error", logger.Messages[1].Level)
	}
}

func TestRunner_OutputError(t *testing.T) {
	runner, err := NewRunner(nil)
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}
	defer runner.Close()

	script := readScript(t, "output_error.luau")

	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test", "")

	result, err := runner.RunSource(ctx, script, mockRT, mockState, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err.Error() != "something went wrong" {
		t.Errorf("error = %v, want 'something went wrong'", err)
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestRunner_NoOutput(t *testing.T) {
	runner, err := NewRunner(nil)
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}
	defer runner.Close()

	script := readScript(t, "no_output.luau")

	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test", "")

	_, err = runner.RunSource(ctx, script, mockRT, mockState, nil)
	if err != ErrNoOutput {
		t.Errorf("error = %v, want ErrNoOutput", err)
	}
}

func TestRunner_Compile(t *testing.T) {
	runner, err := NewRunner(nil)
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}
	defer runner.Close()

	script := readScript(t, "double.luau")

	// Compile the script
	err = runner.Compile("double", script)
	if err != nil {
		t.Fatalf("Compile error: %v", err)
	}

	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test", "")

	// Run the compiled script
	result, err := runner.Run(ctx, "double", mockRT, mockState, int64(21))
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// Lua returns numbers as float64
	resultNum, ok := result.(int64)
	if !ok {
		resultF, ok := result.(float64)
		if !ok {
			t.Fatalf("result type = %T", result)
		}
		resultNum = int64(resultF)
	}

	if resultNum != 42 {
		t.Errorf("result = %v, want 42", resultNum)
	}
}

func TestRunner_ScriptError(t *testing.T) {
	runner, err := NewRunner(nil)
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}
	defer runner.Close()

	script := readScript(t, "script_error.luau")

	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test", "")

	_, err = runner.RunSource(ctx, script, mockRT, mockState, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should be wrapped as ErrScriptError
	if !errors.Is(err, ErrScriptError) {
		t.Errorf("error = %v, want ErrScriptError", err)
	}
}

func TestRunner_ArrayInput(t *testing.T) {
	runner, err := NewRunner(nil)
	if err != nil {
		t.Fatalf("NewRunner error: %v", err)
	}
	defer runner.Close()

	script := readScript(t, "array_sum.luau")

	ctx := context.Background()
	mockRT := NewMockRuntime()
	mockState := NewMockState("test", "")

	input := []any{int64(1), int64(2), int64(3), int64(4), int64(5)}

	result, err := runner.RunSource(ctx, script, mockRT, mockState, input)
	if err != nil {
		t.Fatalf("RunSource error: %v", err)
	}

	// Result could be int64 or float64
	var sum int64
	switch v := result.(type) {
	case int64:
		sum = v
	case float64:
		sum = int64(v)
	default:
		t.Fatalf("result type = %T", result)
	}

	if sum != 15 {
		t.Errorf("sum = %d, want 15", sum)
	}
}
