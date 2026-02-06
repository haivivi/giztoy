package runtime

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// defaultHTTPTimeout is used when no timeout is specified in the request.
const defaultHTTPTimeout = 30 * time.Second

// httpClient without timeout - timeout is handled per-request via context.
var httpClient = &http.Client{}

// builtinHTTP implements __builtin.http(request) -> Promise
// request: { url: string, method: string?, headers: {[string]: string}?, body: string?, timeout: number? }
// timeout is in milliseconds, defaults to 30000 (30 seconds)
// Returns a Promise that resolves to: { status: number, headers: {[string]: string}, body: string, err: string? }
// Usage: local resp = rt:http({url = "...", timeout = 5000}):await()
func (rt *Runtime) builtinHTTP(state *luau.State) int {
	// Check if argument is a table
	if !state.IsTable(1) {
		// Return an immediately resolved promise with error
		promise := rt.promises.newPromise()
		promise.Resolve(map[string]any{"err": "request must be a table"})
		rt.pushPromiseObject(state, promise)
		return 1
	}

	// Read URL
	state.GetField(1, "url")
	urlStr := state.ToString(-1)
	state.Pop(1)

	if urlStr == "" {
		// Return an immediately resolved promise with error
		promise := rt.promises.newPromise()
		promise.Resolve(map[string]any{"err": "url is required"})
		rt.pushPromiseObject(state, promise)
		return 1
	}

	// Read method (default: GET)
	state.GetField(1, "method")
	method := state.ToString(-1)
	if method == "" {
		method = "GET"
	}
	state.Pop(1)

	// Read body
	state.GetField(1, "body")
	body := state.ToString(-1)
	state.Pop(1)

	// Read timeout (default: 30000ms)
	state.GetField(1, "timeout")
	timeoutMs := state.ToNumber(-1)
	state.Pop(1)

	timeout := defaultHTTPTimeout
	if timeoutMs > 0 {
		timeout = time.Duration(timeoutMs) * time.Millisecond
	}

	// Read headers into a map
	headers := make(map[string]string)
	state.GetField(1, "headers")
	if state.IsTable(-1) {
		state.PushNil()
		for state.Next(-2) {
			key := state.ToString(-2)
			value := state.ToString(-1)
			if key != "" && value != "" {
				headers[key] = value
			}
			state.Pop(1)
		}
	}
	state.Pop(1)

	// Create a promise for the HTTP request
	promise := rt.promises.newPromise()

	// Capture parent context for timeout
	parentCtx := rt.ctx

	// Start HTTP request in goroutine
	go func() {
		result := executeHTTP(parentCtx, method, urlStr, headers, body, timeout)
		// Resolve with the HTTP result as a map
		promise.Resolve(httpResultToMap(result))
	}()

	// Return the promise object
	rt.pushHTTPPromiseObject(state, promise, HTTPResult{})
	return 1
}

// httpResultToMap converts HTTPResult to a map for Lua.
func httpResultToMap(result HTTPResult) map[string]any {
	m := make(map[string]any)

	if result.Err != nil {
		m["status"] = 0
		m["err"] = result.Err.Error()
		return m
	}

	m["status"] = result.Status
	m["body"] = result.Body
	m["headers"] = result.Headers

	return m
}

// pushHTTPPromiseObject creates a Promise object with :await() method.
func (rt *Runtime) pushHTTPPromiseObject(state *luau.State, promise *Promise, _ HTTPResult) {
	rt.pushPromiseObject(state, promise)
}

// executeHTTP performs the actual HTTP request with timeout support.
func executeHTTP(parentCtx context.Context, method, urlStr string, headers map[string]string, body string, timeout time.Duration) HTTPResult {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	// Create HTTP request
	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return HTTPResult{Err: err}
	}

	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Execute request
	resp, err := httpClient.Do(req)
	if err != nil {
		// Check if it's a timeout error
		if ctx.Err() == context.DeadlineExceeded {
			return HTTPResult{Err: context.DeadlineExceeded}
		}
		return HTTPResult{Err: err}
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return HTTPResult{Err: fmt.Errorf("reading response body: %w", err)}
	}

	// Build response headers map
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	return HTTPResult{
		Status:  resp.StatusCode,
		Headers: respHeaders,
		Body:    string(respBody),
	}
}

// pushHTTPResult pushes an HTTPResult onto the Luau stack as a table.
// This is kept for direct result pushing (e.g., in tests).
func pushHTTPResult(state *luau.State, result HTTPResult) {
	state.NewTable()

	if result.Err != nil {
		state.PushNumber(0)
		state.SetField(-2, "status")
		state.PushString(result.Err.Error())
		state.SetField(-2, "err")
		return
	}

	state.PushNumber(float64(result.Status))
	state.SetField(-2, "status")

	state.PushString(result.Body)
	state.SetField(-2, "body")

	// Response headers
	state.NewTable()
	for k, v := range result.Headers {
		state.PushString(v)
		state.SetField(-2, k)
	}
	state.SetField(-2, "headers")
}

// HasPendingRequests is deprecated - use HasPendingOps instead.
// Kept for backward compatibility.
func (rt *Runtime) HasPendingRequests() bool {
	return rt.HasPendingOps()
}

// PollCompletedHTTP is deprecated - use PollCompleted instead.
// Kept for backward compatibility.
func (rt *Runtime) PollCompletedHTTP() bool {
	return rt.PollCompleted()
}
