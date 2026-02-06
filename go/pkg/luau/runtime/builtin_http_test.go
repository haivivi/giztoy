package runtime

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

func TestExecuteHTTP_GET(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("X-Custom-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	}))
	defer server.Close()

	result := executeHTTP(context.Background(), "GET", server.URL, nil, "", defaultHTTPTimeout)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Status != 200 {
		t.Errorf("expected status 200, got %d", result.Status)
	}
	if result.Body != "hello world" {
		t.Errorf("expected 'hello world', got %s", result.Body)
	}
	if result.Headers["X-Custom-Header"] != "test-value" {
		t.Errorf("expected header 'test-value', got %s", result.Headers["X-Custom-Header"])
	}
}

func TestExecuteHTTP_POST(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Check headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer token123" {
			t.Errorf("expected Authorization header")
		}

		// Read body
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"name":"test"}` {
			t.Errorf("unexpected body: %s", body)
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":1}`))
	}))
	defer server.Close()

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer token123",
	}
	result := executeHTTP(context.Background(), "POST", server.URL, headers, `{"name":"test"}`, defaultHTTPTimeout)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Status != 201 {
		t.Errorf("expected status 201, got %d", result.Status)
	}
	if result.Body != `{"id":1}` {
		t.Errorf("expected '{\"id\":1}', got %s", result.Body)
	}
}

func TestExecuteHTTP_Error(t *testing.T) {
	t.Run("connection refused", func(t *testing.T) {
		// Use localhost with a port that's unlikely to be listening
		result := executeHTTP(context.Background(), "GET", "http://127.0.0.1:1", nil, "", 1*time.Second)
		if result.Err == nil {
			t.Error("expected error for refused connection")
		}
	})

	t.Run("invalid scheme", func(t *testing.T) {
		result := executeHTTP(context.Background(), "GET", "invalid://localhost", nil, "", defaultHTTPTimeout)
		if result.Err == nil {
			t.Error("expected error for invalid scheme")
		}
	})
}

func TestExecuteHTTP_Methods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != method {
					t.Errorf("expected %s, got %s", method, r.Method)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			result := executeHTTP(context.Background(), method, server.URL, nil, "", defaultHTTPTimeout)
			if result.Err != nil {
				t.Fatalf("unexpected error: %v", result.Err)
			}
			if result.Status != 200 {
				t.Errorf("expected status 200, got %d", result.Status)
			}
		})
	}
}

func TestPushHTTPResult(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()

	t.Run("success result", func(t *testing.T) {
		result := HTTPResult{
			Status: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"key":"value"}`,
		}

		pushHTTPResult(state, result)

		if !state.IsTable(-1) {
			t.Error("expected table on stack")
		}

		state.GetField(-1, "status")
		if int(state.ToNumber(-1)) != 200 {
			t.Errorf("expected status 200, got %v", state.ToNumber(-1))
		}
		state.Pop(1)

		state.GetField(-1, "body")
		if state.ToString(-1) != `{"key":"value"}` {
			t.Errorf("unexpected body: %s", state.ToString(-1))
		}
		state.Pop(1)

		state.GetField(-1, "headers")
		if !state.IsTable(-1) {
			t.Error("expected headers to be a table")
		}
		state.GetField(-1, "Content-Type")
		if state.ToString(-1) != "application/json" {
			t.Errorf("unexpected Content-Type: %s", state.ToString(-1))
		}
		state.Pop(3)
	})

	t.Run("error result", func(t *testing.T) {
		result := HTTPResult{
			Err: http.ErrServerClosed,
		}

		pushHTTPResult(state, result)

		if !state.IsTable(-1) {
			t.Error("expected table on stack")
		}

		state.GetField(-1, "status")
		if int(state.ToNumber(-1)) != 0 {
			t.Errorf("expected status 0 for error, got %v", state.ToNumber(-1))
		}
		state.Pop(1)

		state.GetField(-1, "err")
		if state.IsNil(-1) {
			t.Error("expected err field to be set")
		}
		state.Pop(2)
	})
}

func TestBuiltinHTTP_Async(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "success"})
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

	// Store URL in global for Lua access
	state.PushString(server.URL)
	state.SetGlobal("testURL")

	// First, test what rt:http returns
	err = rt.Run(`
		local promise = rt:http({ url = testURL, method = "GET" })
		_G.promise_type = type(promise)
		if type(promise) == "table" then
			_G.has_await = promise.await ~= nil
			_G.await_type = type(promise.await)
			_G.has_id = promise._id ~= nil
		end
	`, "test1")
	if err != nil {
		t.Fatalf("Run test1 failed: %v", err)
	}

	state.GetGlobal("promise_type")
	t.Logf("promise type: %s", state.ToString(-1))
	state.Pop(1)

	state.GetGlobal("has_await")
	t.Logf("has await: %v", state.ToBoolean(-1))
	state.Pop(1)

	state.GetGlobal("await_type")
	t.Logf("await type: %s", state.ToString(-1))
	state.Pop(1)

	state.GetGlobal("has_id")
	t.Logf("has _id: %v", state.ToBoolean(-1))
	state.Pop(1)

	// Use rt.Run() to enable coroutine-based async execution
	err = rt.Run(`
		local resp = rt:http({ url = testURL, method = "GET" }):await()
		_G.status = resp.status
		_G.body = resp.body
		_G.hasErr = resp.err ~= nil
	`, "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	state.GetGlobal("status")
	if int(state.ToNumber(-1)) != 200 {
		t.Errorf("expected status 200, got %v", state.ToNumber(-1))
	}
	state.Pop(1)

	state.GetGlobal("hasErr")
	if state.ToBoolean(-1) {
		t.Error("expected no error")
	}
	state.Pop(1)
}

func TestBuiltinHTTP_WithHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "test-header" {
			t.Errorf("expected X-Custom header, got %s", r.Header.Get("X-Custom"))
		}
		w.WriteHeader(http.StatusOK)
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

	err = rt.Run(`
		local resp = rt:http({
			url = testURL,
			method = "GET",
			headers = {
				["X-Custom"] = "test-header"
			}
		}):await()
		_G.status = resp.status
	`, "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	state.GetGlobal("status")
	if int(state.ToNumber(-1)) != 200 {
		t.Errorf("expected status 200, got %v", state.ToNumber(-1))
	}
}

func TestBuiltinHTTP_WithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"data":"test"}` {
			t.Errorf("unexpected body: %s", body)
		}
		w.WriteHeader(http.StatusOK)
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

	err = rt.Run(`
		local resp = rt:http({
			url = testURL,
			method = "POST",
			body = '{"data":"test"}'
		}):await()
		_G.status = resp.status
	`, "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	state.GetGlobal("status")
	if int(state.ToNumber(-1)) != 200 {
		t.Errorf("expected status 200, got %v", state.ToNumber(-1))
	}
}

func TestBuiltinHTTP_InvalidInput(t *testing.T) {
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

	t.Run("non-table argument", func(t *testing.T) {
		err := rt.Run(`
			local resp = rt:http("not a table"):await()
			_G.errMsg = resp.err
		`, "test")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		state.GetGlobal("errMsg")
		if state.IsNil(-1) {
			t.Error("expected error message")
		}
		errMsg := state.ToString(-1)
		if errMsg != "request must be a table" {
			t.Errorf("unexpected error: %s", errMsg)
		}
		state.Pop(1)
	})

	t.Run("missing url", func(t *testing.T) {
		err := rt.Run(`
			local resp = rt:http({ method = "GET" }):await()
			_G.errMsg = resp.err
		`, "test")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		state.GetGlobal("errMsg")
		if state.IsNil(-1) {
			t.Error("expected error message")
		}
		errMsg := state.ToString(-1)
		if errMsg != "url is required" {
			t.Errorf("unexpected error: %s", errMsg)
		}
		state.Pop(1)
	})

	t.Run("empty url", func(t *testing.T) {
		err := rt.Run(`
			local resp = rt:http({ url = "" }):await()
			_G.errMsg = resp.err
		`, "test")
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		state.GetGlobal("errMsg")
		errMsg := state.ToString(-1)
		if errMsg != "url is required" {
			t.Errorf("unexpected error: %s", errMsg)
		}
		state.Pop(1)
	})
}

func TestBuiltinHTTP_DefaultMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected default method GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
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

	// No method specified - should default to GET
	err = rt.Run(`
		local resp = rt:http({ url = testURL }):await()
		_G.status = resp.status
	`, "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	state.GetGlobal("status")
	if int(state.ToNumber(-1)) != 200 {
		t.Errorf("expected status 200, got %v", state.ToNumber(-1))
	}
}

func TestHasPendingOps(t *testing.T) {
	state, err := luau.New()
	if err != nil {
		t.Fatalf("luau.New failed: %v", err)
	}
	defer state.Close()

	rt := New(state, nil)
	rt.InitAsync()

	if rt.HasPendingOps() {
		t.Error("expected no pending ops initially")
	}
}

func TestExecuteHTTP_Timeout(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Request with very short timeout should fail
	result := executeHTTP(context.Background(), "GET", server.URL, nil, "", 50*time.Millisecond)
	if result.Err == nil {
		t.Error("expected timeout error")
	}
	if result.Err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", result.Err)
	}
}

func TestBuiltinHTTP_WithTimeout(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
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

	// Request with short timeout should fail
	err = rt.Run(`
		local resp = rt:http({ url = testURL, timeout = 50 }):await()
		_G.status = resp.status
		_G.hasErr = resp.err ~= nil
		_G.errMsg = resp.err or ""
	`, "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	state.GetGlobal("hasErr")
	if !state.ToBoolean(-1) {
		t.Error("expected timeout error")
	}
	state.Pop(1)

	state.GetGlobal("status")
	if int(state.ToNumber(-1)) != 0 {
		t.Errorf("expected status 0 for timeout, got %v", state.ToNumber(-1))
	}
	state.Pop(1)

	state.GetGlobal("errMsg")
	errMsg := state.ToString(-1)
	if errMsg != "context deadline exceeded" {
		t.Logf("error message: %s", errMsg)
	}
	state.Pop(1)
}
