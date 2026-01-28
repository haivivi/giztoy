package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// httpClient with timeout to prevent indefinite hangs
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// builtinHTTP implements __builtin.http(request) -> response
// In async mode: starts goroutine, yields, returns result when resumed
// request: { url: string, method: string?, headers: {[string]: string}?, body: string? }
// response: { status: number, headers: {[string]: string}, body: string, err: string? }
func (rt *Runtime) builtinHTTP(state *luau.State) int {
	// Check if argument is a table
	if !state.IsTable(1) {
		state.NewTable()
		state.PushString("request must be a table")
		state.SetField(-2, "err")
		return 1
	}

	// Read URL
	state.GetField(1, "url")
	urlStr := state.ToString(-1)
	state.Pop(1)

	if urlStr == "" {
		state.NewTable()
		state.PushString("url is required")
		state.SetField(-2, "err")
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

	// Check if we're in a thread context (can yield)
	if rt.currentThread != nil && state.IsYieldable() {
		// Async mode: start goroutine and yield
		return rt.asyncHTTP(state, method, urlStr, headers, body)
	}

	// Sync mode: execute HTTP directly (fallback for main thread)
	return rt.syncHTTP(state, method, urlStr, headers, body)
}

// asyncHTTP starts an HTTP request in a goroutine and yields.
func (rt *Runtime) asyncHTTP(state *luau.State, method, urlStr string, headers map[string]string, body string) int {
	// Generate request ID
	reqID := atomic.AddUint64(&rt.nextRequestID, 1)

	// Create result channel
	resultCh := make(chan HTTPResult, 1)

	// Register pending request
	rt.pendingMu.Lock()
	pending := &PendingRequest{
		ID:       reqID,
		Thread:   rt.currentThread,
		ResultCh: resultCh,
	}
	rt.pendingReqs[reqID] = pending
	rt.pendingMu.Unlock()

	// Start HTTP request in goroutine
	go func() {
		result := executeHTTP(method, urlStr, headers, body)
		resultCh <- result

		// Signal completion
		rt.completedReqs <- pending
	}()

	// Push request ID so we can match on resume
	state.PushInteger(int64(reqID))

	// Yield with 1 value (the request ID)
	return state.Yield(1)
}

// syncHTTP executes an HTTP request synchronously (blocking).
func (rt *Runtime) syncHTTP(state *luau.State, method, urlStr string, headers map[string]string, body string) int {
	result := executeHTTP(method, urlStr, headers, body)
	pushHTTPResult(state, result)
	return 1
}

// executeHTTP performs the actual HTTP request.
func executeHTTP(method, urlStr string, headers map[string]string, body string) HTTPResult {
	// Create HTTP request
	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}

	req, err := http.NewRequest(method, urlStr, bodyReader)
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
		return HTTPResult{Err: err}
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("[luau] warning: error reading response body: %v\n", err)
		// Continue with partial body if any
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

// pushHTTPResult pushes an HTTPResult onto the Luau stack.
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

// PollCompletedHTTP checks for completed HTTP requests and resumes their threads.
// Returns true if there are still pending requests.
func (rt *Runtime) PollCompletedHTTP() bool {
	select {
	case pending := <-rt.completedReqs:
		// Get result from channel
		result := <-pending.ResultCh

		// Remove from pending map
		rt.pendingMu.Lock()
		delete(rt.pendingReqs, pending.ID)
		hasPending := len(rt.pendingReqs) > 0
		rt.pendingMu.Unlock()

		// Push result onto thread's stack
		pushHTTPResult(pending.Thread.State, result)

		// Resume the thread with 1 result
		rt.currentThread = pending.Thread
		status, _ := pending.Thread.Resume(1)
		rt.currentThread = nil

		if status != luau.CoStatusOK && status != luau.CoStatusYield {
			// Thread error - get error message
			errMsg := pending.Thread.ToString(-1)
			fmt.Printf("[luau] thread error: %s\n", errMsg)
		}

		return hasPending || status == luau.CoStatusYield

	default:
		// No completed requests
		rt.pendingMu.Lock()
		hasPending := len(rt.pendingReqs) > 0
		rt.pendingMu.Unlock()
		return hasPending
	}
}

// HasPendingRequests returns true if there are pending HTTP requests.
func (rt *Runtime) HasPendingRequests() bool {
	rt.pendingMu.Lock()
	defer rt.pendingMu.Unlock()
	return len(rt.pendingReqs) > 0
}

