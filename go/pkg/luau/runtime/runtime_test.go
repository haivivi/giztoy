package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

func TestThreadGlobalAccess(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	// Register a global function on main state
	err = state.RegisterFunc("testfunc", func(s *luau.State) int {
		s.PushString("hello from testfunc")
		return 1
	})
	if err != nil {
		t.Fatalf("Failed to register func: %v", err)
	}

	// Verify it exists in main state
	state.GetGlobal("testfunc")
	if state.IsNil(-1) {
		t.Error("testfunc is nil in main state")
	}
	if !state.IsFunction(-1) {
		t.Error("testfunc is not a function in main state")
	}
	state.Pop(1)

	// Create a thread
	thread, err := state.NewThread()
	if err != nil {
		t.Fatalf("Failed to create thread: %v", err)
	}

	// Check if global is accessible from thread
	thread.GetGlobal("testfunc")
	isNil := thread.IsNil(-1)
	isFunc := thread.IsFunction(-1)
	t.Logf("Thread - testfunc is nil: %v, is function: %v", isNil, isFunc)
	thread.Pop(1)

	if isNil {
		t.Error("testfunc is nil in thread - globals not shared!")
	}
	if !isFunc {
		t.Error("testfunc is not a function in thread")
	}

	// Try to run script that calls the function from thread
	source := `_G.result = testfunc()`
	bytecode, err := state.Compile(source, luau.OptO2)
	if err != nil {
		t.Fatalf("Compile error: %v", err)
	}

	err = thread.LoadBytecode(bytecode, "test")
	if err != nil {
		t.Fatalf("LoadBytecode error: %v", err)
	}

	status, _ := thread.Resume(0)
	t.Logf("Resume status: %v", status)

	if status != luau.CoStatusOK {
		errMsg := thread.ToString(-1)
		t.Errorf("Thread failed with status %v: %s", status, errMsg)
	}

	// Check result
	state.GetGlobal("result")
	result := state.ToString(-1)
	t.Logf("Result: %s", result)
	if result != "hello from testfunc" {
		t.Errorf("Expected 'hello from testfunc', got '%s'", result)
	}
}

func TestPromiseMethodsRegistered(t *testing.T) {
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

	// Check that promise methods are registered
	state.GetGlobal("__promise_await")
	if state.IsNil(-1) {
		t.Error("__promise_await is not registered")
	} else {
		t.Logf("__promise_await is nil: %v, is function: %v", state.IsNil(-1), state.IsFunction(-1))
	}
	state.Pop(1)

	state.GetGlobal("__promise_is_ready")
	if state.IsNil(-1) {
		t.Error("__promise_is_ready is not registered")
	} else {
		t.Logf("__promise_is_ready is nil: %v, is function: %v", state.IsNil(-1), state.IsFunction(-1))
	}
	state.Pop(1)

	// Create a thread and check access
	thread, err := state.NewThread()
	if err != nil {
		t.Fatalf("NewThread failed: %v", err)
	}

	thread.GetGlobal("__promise_await")
	if thread.IsNil(-1) {
		t.Error("__promise_await not accessible from thread")
	} else {
		t.Logf("Thread __promise_await is nil: %v, is function: %v", thread.IsNil(-1), thread.IsFunction(-1))
	}
	thread.Pop(1)

	// Now test pushPromiseObject directly
	promise := rt.promises.newPromise()
	rt.pushPromiseObject(thread.State, promise)

	// Check if the table has await method
	thread.GetField(-1, "await")
	if thread.IsNil(-1) {
		t.Error("Promise object doesn't have await method")
	} else {
		t.Logf("Promise.await is nil: %v, is function: %v", thread.IsNil(-1), thread.IsFunction(-1))
	}
	thread.Pop(1)

	thread.GetField(-1, "_id")
	t.Logf("Promise._id: %v", thread.ToInteger(-1))
	thread.Pop(1)
}

var _ = fmt.Sprintf // silence unused import

func TestNewWithOptionsBasic(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()

	ctx := context.WithValue(context.Background(), "test", "value")

	rt := NewWithOptions(state,
		WithContext(ctx),
		WithLibsDir("/tmp/libs"),
	)

	if rt.ctx != ctx {
		t.Error("context not set correctly")
	}
	if rt.libsDir != "/tmp/libs" {
		t.Error("libsDir not set correctly")
	}
}

type testCache struct {
	data map[string]any
}

func (m *testCache) Get(key string) (any, bool) {
	v, ok := m.data[key]
	return v, ok
}

func (m *testCache) Set(key string, value any, ttl time.Duration) {
	m.data[key] = value
}

func (m *testCache) Delete(key string) {
	delete(m.data, key)
}

func TestWithCacheOption(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()

	cache := &testCache{data: make(map[string]any)}
	rt := NewWithOptions(state, WithCache(cache))

	if rt.cache != cache {
		t.Error("cache not set correctly")
	}
}

func TestWithRuntimeContextOption(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()

	tc := NewToolContext()
	rt := NewWithOptions(state, WithRuntimeContext(tc))

	if rt.runtimeCtx != tc {
		t.Error("runtimeCtx not set correctly")
	}
}

func TestRuntimeSettersAndGetters(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()

	rt := New(state, nil)

	t.Run("SetCache", func(t *testing.T) {
		cache := &testCache{data: make(map[string]any)}
		rt.SetCache(cache)
		if rt.cache != cache {
			t.Error("cache not set")
		}
	})

	t.Run("SetContext", func(t *testing.T) {
		ctx := context.Background()
		rt.SetContext(ctx)
		if rt.ctx != ctx {
			t.Error("context not set")
		}
	})

	t.Run("SetRuntimeContext", func(t *testing.T) {
		tc := NewToolContext()
		rt.SetRuntimeContext(tc)
		if rt.GetRuntimeContext() != tc {
			t.Error("runtime context not set correctly")
		}
	})

	t.Run("SetLibsDir", func(t *testing.T) {
		rt.SetLibsDir("/new/path")
		if rt.libsDir != "/new/path" {
			t.Error("libsDir not set correctly")
		}
	})

	t.Run("State", func(t *testing.T) {
		if rt.State() != state {
			t.Error("State() should return underlying state")
		}
	})
}

func TestKVS(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()

	rt := New(state, nil)

	t.Run("set and get", func(t *testing.T) {
		rt.KVSSet("key1", "value1")
		v, ok := rt.KVSGet("key1")
		if !ok {
			t.Error("expected to find key")
		}
		if v != "value1" {
			t.Errorf("expected 'value1', got %v", v)
		}
	})

	t.Run("get non-existent", func(t *testing.T) {
		_, ok := rt.KVSGet("nonexistent")
		if ok {
			t.Error("expected not to find key")
		}
	})

	t.Run("delete", func(t *testing.T) {
		rt.KVSSet("key2", "value2")
		rt.KVSDel("key2")
		_, ok := rt.KVSGet("key2")
		if ok {
			t.Error("expected key to be deleted")
		}
	})
}

func TestInitAsync(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()

	rt := New(state, nil)
	rt.InitAsync()

	if rt.pendingOps == nil {
		t.Error("pendingOps should be initialized")
	}
	if rt.completedOps == nil {
		t.Error("completedOps should be initialized")
	}
	if rt.promises == nil {
		t.Error("promises should be initialized")
	}
}

func TestPrecompileModulesNoLibsDir(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()

	rt := New(state, nil)
	rt.libsDir = ""

	// Should return nil without error
	if err := rt.PrecompileModules(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestPrecompileModulesNonExistentDir(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()

	rt := New(state, nil)
	rt.libsDir = "/nonexistent/directory"

	// Should return nil without error
	if err := rt.PrecompileModules(); err != nil {
		t.Errorf("expected no error for non-existent dir, got %v", err)
	}
}

func TestPrecompileModulesSuccess(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	// Create temp directory with test module
	tmpDir := t.TempDir()
	testModule := filepath.Join(tmpDir, "test.luau")
	if err := os.WriteFile(testModule, []byte("return 42"), 0644); err != nil {
		t.Fatal(err)
	}

	rt := New(state, &Options{LibsDir: tmpDir})

	if err := rt.PrecompileModules(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Check bytecode was cached
	bc := rt.GetBytecode("test")
	if bc == nil {
		t.Error("expected bytecode to be cached")
	}
}

func TestPrecompileModulesInitModule(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	// Create temp directory with init module
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "mylib")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	initModule := filepath.Join(subDir, "init.luau")
	if err := os.WriteFile(initModule, []byte("return {value = 1}"), 0644); err != nil {
		t.Fatal(err)
	}

	rt := New(state, &Options{LibsDir: tmpDir})

	if err := rt.PrecompileModules(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Check init.luau is cached as "mylib" (not "mylib/init")
	bc := rt.GetBytecode("mylib")
	if bc == nil {
		t.Error("expected bytecode for 'mylib' to be cached")
	}
}

func TestPrecompileModulesCompileError(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	// Create temp directory with invalid module
	tmpDir := t.TempDir()
	badModule := filepath.Join(tmpDir, "bad.luau")
	if err := os.WriteFile(badModule, []byte("function invalid syntax"), 0644); err != nil {
		t.Fatal(err)
	}

	rt := New(state, &Options{LibsDir: tmpDir})

	err = rt.PrecompileModules()
	if err == nil {
		t.Error("expected compile error")
	}
}

func TestPrecompileModulesSkipDLuau(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	// Create temp directory with .d.luau type declaration file
	tmpDir := t.TempDir()
	typeDecl := filepath.Join(tmpDir, "types.d.luau")
	if err := os.WriteFile(typeDecl, []byte("export type MyType = { x: number }"), 0644); err != nil {
		t.Fatal(err)
	}

	rt := New(state, &Options{LibsDir: tmpDir})

	if err := rt.PrecompileModules(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// .d.luau files should be skipped
	bc := rt.GetBytecode("types.d")
	if bc != nil {
		t.Error("expected .d.luau to be skipped")
	}
}

func TestGetBytecodeNotFound(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()

	rt := New(state, nil)

	bc := rt.GetBytecode("nonexistent")
	if bc != nil {
		t.Error("expected nil for non-existent module")
	}
}

func TestRun(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)

	err = rt.Run("_G.testVar = 42", "test.luau")
	if err != nil {
		t.Errorf("Run failed: %v", err)
	}

	state.GetGlobal("testVar")
	if state.ToInteger(-1) != 42 {
		t.Error("expected testVar to be 42")
	}
}

func TestRunError(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)

	err = rt.Run("error('test error')", "test.luau")
	if err == nil {
		t.Error("expected error")
	}
}

func TestRunAsync(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	rt.InitAsync()

	err = rt.RunAsync("_G.asyncVar = 123", "async_test.luau")
	if err != nil {
		t.Errorf("RunAsync failed: %v", err)
	}
}

func TestRunAsyncCompileError(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	rt.InitAsync()

	err = rt.RunAsync("function invalid syntax", "test.luau")
	if err == nil {
		t.Error("expected compile error")
	}
}

func TestMakeMethodWrapper(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)

	// Create a simple function that returns the top value
	fn := func(s *luau.State) int {
		// After wrapper removes self, first arg should be "hello"
		s.PushString("processed")
		return 1
	}

	wrapped := rt.makeMethodWrapper(fn)

	// Register and call with self + arg
	state.RegisterFunc("testWrapper", wrapped)
	err = state.DoString(`
		local result = testWrapper({}, "arg1")
		_G.wrapperResult = result
	`)
	if err != nil {
		t.Fatalf("DoString failed: %v", err)
	}

	state.GetGlobal("wrapperResult")
	if state.ToString(-1) != "processed" {
		t.Error("wrapper did not work correctly")
	}
}

func TestRegisterBuiltins(t *testing.T) {
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

	// Verify __builtin table exists with expected functions
	state.GetGlobal("__builtin")
	if state.IsNil(-1) {
		t.Error("__builtin table not created")
	}

	state.GetField(-1, "json_encode")
	if state.IsNil(-1) {
		t.Error("__builtin.json_encode not registered")
	}
}

func TestRegisterContextFunctions(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	rt.RegisterBuiltins()

	tc := rt.CreateToolContext()
	tc.SetInput("test")

	if err := rt.RegisterContextFunctions(); err != nil {
		t.Fatalf("RegisterContextFunctions failed: %v", err)
	}

	// Verify 'rt' table exists with methods
	state.GetGlobal("rt")
	if state.IsNil(-1) {
		t.Error("rt table not created")
	}

	state.GetField(-1, "log")
	if state.IsNil(-1) {
		t.Error("rt.log not registered")
	}
}

func TestRegisterAll(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()
	state.OpenLibs()

	rt := New(state, nil)
	rt.CreateToolContext()

	if err := rt.RegisterAll(); err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	// Both __builtin and rt should exist
	state.GetGlobal("__builtin")
	if state.IsNil(-1) {
		t.Error("__builtin not registered")
	}
	state.Pop(1)

	state.GetGlobal("rt")
	if state.IsNil(-1) {
		t.Error("rt not registered")
	}
}
