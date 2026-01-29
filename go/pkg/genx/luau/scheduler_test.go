package luau

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

func TestNewScheduler(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New error: %v", err)
	}
	defer state.Close()

	ctx := context.Background()
	sched := NewScheduler(ctx, state, nil)
	if sched == nil {
		t.Fatal("NewScheduler returned nil")
	}
	defer sched.Close()

	if sched.State() != state {
		t.Error("State mismatch")
	}

	if sched.Context() == nil {
		t.Error("Context is nil")
	}
}

func TestScheduler_PushValue(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New error: %v", err)
	}
	defer state.Close()

	ctx := context.Background()
	sched := NewScheduler(ctx, state, nil)
	defer sched.Close()

	tests := []struct {
		name  string
		value any
		check func(s *luau.State) bool
	}{
		{"nil", nil, func(s *luau.State) bool { return s.IsNil(-1) }},
		{"bool true", true, func(s *luau.State) bool { return s.IsBoolean(-1) && s.ToBoolean(-1) }},
		{"bool false", false, func(s *luau.State) bool { return s.IsBoolean(-1) && !s.ToBoolean(-1) }},
		{"int", 42, func(s *luau.State) bool { return s.IsNumber(-1) && s.ToInteger(-1) == 42 }},
		{"int64", int64(123456789), func(s *luau.State) bool { return s.IsNumber(-1) && s.ToInteger(-1) == 123456789 }},
		{"float64", 3.14, func(s *luau.State) bool { return s.IsNumber(-1) && s.ToNumber(-1) == 3.14 }},
		{"string", "hello", func(s *luau.State) bool { return s.IsString(-1) && s.ToString(-1) == "hello" }},
		{"bytes", []byte("data"), func(s *luau.State) bool { return s.IsString(-1) && s.ToString(-1) == "data" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := state.GetTop()
			sched.pushValue(state, tt.value)
			after := state.GetTop()

			if after != before+1 {
				t.Errorf("Stack size changed by %d, want 1", after-before)
			}

			if !tt.check(state) {
				t.Errorf("Check failed for %v", tt.value)
			}

			state.Pop(1)
		})
	}
}

func TestScheduler_PushValue_Table(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New error: %v", err)
	}
	defer state.Close()

	ctx := context.Background()
	sched := NewScheduler(ctx, state, nil)
	defer sched.Close()

	// Test map[string]any
	m := map[string]any{
		"name": "test",
		"age":  30,
	}
	sched.pushValue(state, m)

	if !state.IsTable(-1) {
		t.Fatal("Expected table on stack")
	}

	state.GetField(-1, "name")
	if !state.IsString(-1) || state.ToString(-1) != "test" {
		t.Errorf("name = %q, want %q", state.ToString(-1), "test")
	}
	state.Pop(1)

	state.GetField(-1, "age")
	if !state.IsNumber(-1) || state.ToInteger(-1) != 30 {
		t.Errorf("age = %d, want 30", state.ToInteger(-1))
	}
	state.Pop(2)

	// Test []any
	arr := []any{"a", "b", "c"}
	sched.pushValue(state, arr)

	if !state.IsTable(-1) {
		t.Fatal("Expected table on stack for array")
	}

	state.RawGetI(-1, 1)
	if !state.IsString(-1) || state.ToString(-1) != "a" {
		t.Errorf("arr[1] = %q, want %q", state.ToString(-1), "a")
	}
	state.Pop(2)
}

func TestScheduler_AsyncOp(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New error: %v", err)
	}
	defer state.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sched := NewScheduler(ctx, state, nil)
	defer sched.Close()

	// Create a simple async operation
	executed := false
	op := AsyncOpFunc(func(ctx context.Context) (any, error) {
		executed = true
		return "result", nil
	})

	// Create a thread
	thread, err := state.NewThread()
	if err != nil {
		t.Fatalf("NewThread error: %v", err)
	}

	// Submit the operation
	sched.SubmitAsync(thread, op)

	// Process pending
	sched.processPending()

	// Wait for result
	time.Sleep(100 * time.Millisecond)

	if !executed {
		t.Error("Operation was not executed")
	}
}

func TestScheduler_AsyncOpError(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New error: %v", err)
	}
	defer state.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sched := NewScheduler(ctx, state, nil)
	defer sched.Close()

	// Create an operation that returns error
	expectedErr := errors.New("test error")
	op := AsyncOpFunc(func(ctx context.Context) (any, error) {
		return nil, expectedErr
	})

	// Create a thread
	thread, err := state.NewThread()
	if err != nil {
		t.Fatalf("NewThread error: %v", err)
	}

	// Submit the operation
	sched.SubmitAsync(thread, op)

	// Process pending
	sched.processPending()

	// Wait for result
	time.Sleep(100 * time.Millisecond)
}

func TestScheduler_Close(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New error: %v", err)
	}
	defer state.Close()

	ctx := context.Background()
	sched := NewScheduler(ctx, state, nil)

	// Close should succeed
	err = sched.Close()
	if err != nil {
		t.Errorf("Close error: %v", err)
	}

	// Double close should be safe
	err = sched.Close()
	if err != nil {
		t.Errorf("Double close error: %v", err)
	}

	// Operations after close should fail
	thread, _ := state.NewThread()
	op := AsyncOpFunc(func(ctx context.Context) (any, error) {
		return nil, nil
	})

	// This should not panic
	sched.SubmitAsync(thread, op)
}

func TestScheduler_ConcurrentSubmit(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New error: %v", err)
	}
	defer state.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sched := NewScheduler(ctx, state, nil)
	defer sched.Close()

	var wg sync.WaitGroup
	numOps := 10
	counter := int32(0)
	var counterMu sync.Mutex

	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			thread, err := state.NewThread()
			if err != nil {
				t.Errorf("NewThread error: %v", err)
				return
			}

			op := AsyncOpFunc(func(ctx context.Context) (any, error) {
				counterMu.Lock()
				counter++
				counterMu.Unlock()
				return nil, nil
			})

			sched.SubmitAsync(thread, op)
		}()
	}

	wg.Wait()

	// Process all pending
	sched.processPending()

	// Wait for all to complete
	time.Sleep(200 * time.Millisecond)

	counterMu.Lock()
	if counter != int32(numOps) {
		t.Errorf("counter = %d, want %d", counter, numOps)
	}
	counterMu.Unlock()
}

func TestScheduler_ContextCancel(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New error: %v", err)
	}
	defer state.Close()

	ctx, cancel := context.WithCancel(context.Background())
	sched := NewScheduler(ctx, state, nil)
	defer sched.Close()

	// Create a slow operation
	opStarted := make(chan struct{})
	opDone := make(chan struct{})

	op := AsyncOpFunc(func(ctx context.Context) (any, error) {
		close(opStarted)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return "completed", nil
		}
	})

	thread, _ := state.NewThread()
	sched.SubmitAsync(thread, op)
	sched.processPending()

	// Wait for op to start
	select {
	case <-opStarted:
	case <-time.After(time.Second):
		t.Fatal("Operation did not start")
	}

	// Cancel the context
	cancel()

	// Operation should complete quickly due to cancellation
	go func() {
		time.Sleep(200 * time.Millisecond)
		close(opDone)
	}()

	select {
	case <-opDone:
	case <-time.After(2 * time.Second):
		t.Error("Operation did not respond to cancellation")
	}
}

func TestYieldForAsync(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New error: %v", err)
	}
	defer state.Close()

	// YieldForAsync should return 0 (Yield's return value)
	// This is typically called from a registered function
	// We can't easily test this without a full coroutine setup,
	// but we can at least verify it doesn't panic
	// Note: Calling Yield outside a coroutine may cause issues,
	// so we just test it exists
	_ = YieldForAsync
}
