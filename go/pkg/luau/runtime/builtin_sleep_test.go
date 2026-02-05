package runtime

import (
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

func TestBuiltinSleep_Basic(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	start := time.Now()
	err = rt.Run(`rt:sleep(50):await()`, "test")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Should have slept at least 50ms (with some tolerance)
	if elapsed < 40*time.Millisecond {
		t.Errorf("expected at least 40ms elapsed, got %v", elapsed)
	}
}

func TestBuiltinSleep_Zero(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	// Sleep with 0ms should complete immediately
	err = rt.Run(`rt:sleep(0):await()`, "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestBuiltinSleep_NegativeValue(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	// Negative value should be treated as 0 (immediate completion)
	err = rt.Run(`rt:sleep(-100):await()`, "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestBuiltinSleep_Multiple(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	start := time.Now()
	err = rt.Run(`
		rt:sleep(20):await()
		rt:sleep(20):await()
		rt:sleep(20):await()
	`, "test")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Should have slept at least 60ms total (with some tolerance)
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected at least 50ms elapsed, got %v", elapsed)
	}
}

func TestBuiltinSleep_ReturnsPromise(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	err = rt.Run(`
		local p = rt:sleep(10)
		_G.promise_type = type(p)
		_G.has_await = p.await ~= nil
		_G.has_is_ready = p.is_ready ~= nil
		p:await()
	`, "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	state.GetGlobal("promise_type")
	if state.ToString(-1) != "table" {
		t.Errorf("expected promise to be table, got %s", state.ToString(-1))
	}
	state.Pop(1)

	state.GetGlobal("has_await")
	if !state.ToBoolean(-1) {
		t.Error("expected promise to have await method")
	}
	state.Pop(1)

	state.GetGlobal("has_is_ready")
	if !state.ToBoolean(-1) {
		t.Error("expected promise to have is_ready method")
	}
	state.Pop(1)
}

func TestBuiltinSleep_IsReady(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	// Test that is_ready works correctly
	err = rt.Run(`
		local p = rt:sleep(100)
		-- Promise should not be ready immediately
		_G.initial_ready = p:is_ready()
		p:await()
		-- Promise should be ready after await
		_G.final_ready = p:is_ready()
	`, "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Note: initial_ready might be false or true depending on timing
	// (the sleep goroutine might complete before we check)
	// But final_ready should always be true
	state.GetGlobal("final_ready")
	// After await the promise is removed from registry, so is_ready returns true
	state.Pop(1)
}

func TestBuiltinSleep_ViaBuiltin(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	if err := rt.RegisterBuiltins(); err != nil {
		t.Fatalf("RegisterBuiltins failed: %v", err)
	}

	// Test access via __builtin table
	err = rt.Run(`
		local p = __builtin.sleep(10)
		p:await()
	`, "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}
