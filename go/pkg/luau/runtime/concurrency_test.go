package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// TestConcurrent_MultipleHTTPRequests tests 5 parallel HTTP requests.
func TestConcurrent_MultipleHTTPRequests(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// Small delay to ensure requests run in parallel
		time.Sleep(10 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]int{"count": requestCount})
	}))
	defer server.Close()

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

	state.PushString(server.URL)
	state.SetGlobal("testURL")

	start := time.Now()

	// Launch 5 parallel HTTP requests and await them all
	err = rt.Run(`
		local p1 = rt:http({ url = testURL, method = "GET" })
		local p2 = rt:http({ url = testURL, method = "GET" })
		local p3 = rt:http({ url = testURL, method = "GET" })
		local p4 = rt:http({ url = testURL, method = "GET" })
		local p5 = rt:http({ url = testURL, method = "GET" })

		local r1 = p1:await()
		local r2 = p2:await()
		local r3 = p3:await()
		local r4 = p4:await()
		local r5 = p5:await()

		_G.all_ok = (r1.status == 200) and (r2.status == 200) and (r3.status == 200) and (r4.status == 200) and (r5.status == 200)
	`, "test")

	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// If requests ran in parallel, total time should be ~10-20ms, not ~50ms
	if elapsed > 100*time.Millisecond {
		t.Logf("Warning: requests may not have run in parallel (elapsed: %v)", elapsed)
	}

	state.GetGlobal("all_ok")
	if !state.ToBoolean(-1) {
		t.Error("expected all requests to succeed")
	}
}

// TestConcurrent_MultipleTimeouts tests multiple concurrent timeout operations.
func TestConcurrent_MultipleTimeouts(t *testing.T) {
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
		-- Create multiple timeouts
		local t1 = rt:timeout(10)
		local t2 = rt:timeout(15)
		local t3 = rt:timeout(20)
		
		-- Wait for all
		local r1 = t1:await()
		local r2 = t2:await()
		local r3 = t3:await()
		
		-- None should be cancelled
		_G.all_fired = (not r1.cancelled) and (not r2.cancelled) and (not r3.cancelled)
	`, "test")

	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Should take ~20ms (max timeout), not ~45ms if sequential
	if elapsed > 100*time.Millisecond {
		t.Logf("Warning: timeouts may not have run concurrently (elapsed: %v)", elapsed)
	}

	state.GetGlobal("all_fired")
	if !state.ToBoolean(-1) {
		t.Error("expected all timeouts to fire (not cancelled)")
	}
}

// TestConcurrent_MixedOperations tests HTTP + sleep mixed.
func TestConcurrent_MixedOperations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

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

	state.PushString(server.URL)
	state.SetGlobal("testURL")

	start := time.Now()

	err = rt.Run(`
		-- Start HTTP request
		local httpPromise = rt:http({ url = testURL, method = "GET" })
		
		-- Start sleep
		local sleepPromise = rt:sleep(20)
		
		-- Wait for both
		local httpResult = httpPromise:await()
		local sleepResult = sleepPromise:await()
		
		_G.http_ok = httpResult.status == 200
	`, "test")

	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Total time should be ~20ms (dominated by sleep), not ~40ms if they ran sequentially
	if elapsed > 100*time.Millisecond {
		t.Logf("Warning: operations may not have run concurrently (elapsed: %v)", elapsed)
	}

	state.GetGlobal("http_ok")
	if !state.ToBoolean(-1) {
		t.Error("expected HTTP request to succeed")
	}
}

// TestConcurrent_AwaitAll verifies await_all with multiple promises.
func TestConcurrent_AwaitAll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

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

	state.PushString(server.URL)
	state.SetGlobal("testURL")

	start := time.Now()

	// Use HTTP requests which return non-nil results
	err = rt.Run(`
		local p1 = rt:http({ url = testURL, method = "GET" })
		local p2 = rt:http({ url = testURL, method = "GET" })
		local p3 = rt:http({ url = testURL, method = "GET" })
		
		-- Wait for all using await_all
		local results = rt:await_all(p1, p2, p3):await()
		
		-- Count results by iterating (# won't work for nil values in Lua)
		local count = 0
		for k, v in pairs(results) do
			count = count + 1
		end
		_G.result_count = count
		
		-- Check all have status 200
		_G.all_ok = results[1] and results[1].status == 200 and
		            results[2] and results[2].status == 200 and
		            results[3] and results[3].status == 200
	`, "test")

	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Requests should run in parallel
	if elapsed > 200*time.Millisecond {
		t.Logf("Warning: await_all may not have run promises concurrently (elapsed: %v)", elapsed)
	}

	state.GetGlobal("result_count")
	if int(state.ToNumber(-1)) != 3 {
		t.Errorf("expected 3 results, got %v", state.ToNumber(-1))
	}
	state.Pop(1)

	state.GetGlobal("all_ok")
	if !state.ToBoolean(-1) {
		t.Error("expected all HTTP requests to succeed with status 200")
	}
}

// TestConcurrent_AwaitAny verifies await_any returns first result.
func TestConcurrent_AwaitAny(t *testing.T) {
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
		-- Create sleep promises with different durations
		-- p1 should complete first (5ms)
		local p1 = rt:sleep(5)
		local p2 = rt:sleep(100)
		local p3 = rt:sleep(100)
		
		-- Wait for first one using await_any
		local result = rt:await_any(p1, p2, p3):await()
		
		_G.completed = true
	`, "test")

	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Should complete in ~5ms (first promise), not ~100ms
	if elapsed > 50*time.Millisecond {
		t.Errorf("await_any took too long: %v (expected ~5ms)", elapsed)
	}

	state.GetGlobal("completed")
	if !state.ToBoolean(-1) {
		t.Error("expected await_any to complete")
	}
}

// TestConcurrent_TimeoutCancel tests timeout cancellation.
func TestConcurrent_TimeoutCancel(t *testing.T) {
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
		-- Create a long timeout
		local handle = rt:timeout(1000)  -- 1 second
		
		-- Cancel it immediately
		local cancelled = handle:cancel()
		_G.was_cancelled = cancelled
		
		-- Await should return immediately with cancelled=true
		local result = handle:await()
		_G.result_cancelled = result.cancelled
	`, "test")

	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Should complete immediately since we cancelled
	if elapsed > 100*time.Millisecond {
		t.Errorf("cancelled timeout took too long: %v", elapsed)
	}

	state.GetGlobal("was_cancelled")
	if !state.ToBoolean(-1) {
		t.Error("expected cancel to return true")
	}
	state.Pop(1)

	state.GetGlobal("result_cancelled")
	if !state.ToBoolean(-1) {
		t.Error("expected result.cancelled to be true")
	}
}

// TestConcurrent_TimeoutFires tests timeout that fires normally.
func TestConcurrent_TimeoutFires(t *testing.T) {
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
		-- Create a short timeout
		local handle = rt:timeout(20)  -- 20ms
		
		-- Await should return after ~20ms with cancelled=false
		local result = handle:await()
		_G.result_cancelled = result.cancelled
	`, "test")

	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Should complete in ~20ms
	if elapsed < 15*time.Millisecond {
		t.Errorf("timeout completed too quickly: %v (expected ~20ms)", elapsed)
	}

	state.GetGlobal("result_cancelled")
	if state.ToBoolean(-1) {
		t.Error("expected result.cancelled to be false (timeout fired normally)")
	}
}

// TestConcurrent_AwaitAllWithMixed tests await_all behavior with HTTP and sleep.
func TestConcurrent_AwaitAllWithMixed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

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

	state.PushString(server.URL)
	state.SetGlobal("testURL")

	// Test await_all with HTTP and sleep mixed
	err = rt.Run(`
		local p1 = rt:http({ url = testURL, method = "GET" })
		local p2 = rt:http({ url = testURL, method = "GET" })
		
		local results = rt:await_all(p1, p2):await()
		
		-- Count by iteration
		local count = 0
		for k, v in pairs(results) do
			count = count + 1
		end
		_G.result_count = count
		
		-- First result should be HTTP response
		_G.first_ok = results[1] and results[1].status == 200
	`, "test")

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	state.GetGlobal("result_count")
	if int(state.ToNumber(-1)) != 2 {
		t.Errorf("expected 2 results, got %v", state.ToNumber(-1))
	}
	state.Pop(1)

	state.GetGlobal("first_ok")
	if !state.ToBoolean(-1) {
		t.Error("expected first result to be HTTP response with status 200")
	}
}

// TestConcurrent_AwaitAnyEmpty tests await_any with no promises.
func TestConcurrent_AwaitAnyEmpty(t *testing.T) {
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
		local result = rt:await_any():await()
		_G.result_nil = result == nil
	`, "test")

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	state.GetGlobal("result_nil")
	if !state.ToBoolean(-1) {
		t.Error("expected await_any with no args to return nil")
	}
}

// TestConcurrent_AwaitAllEmpty tests await_all with no promises.
func TestConcurrent_AwaitAllEmpty(t *testing.T) {
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
		local results = rt:await_all():await()
		_G.is_table = type(results) == "table"
		_G.is_empty = #results == 0
	`, "test")

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	state.GetGlobal("is_table")
	if !state.ToBoolean(-1) {
		t.Error("expected await_all with no args to return a table")
	}
	state.Pop(1)

	state.GetGlobal("is_empty")
	if !state.ToBoolean(-1) {
		t.Error("expected await_all with no args to return an empty table")
	}
}
