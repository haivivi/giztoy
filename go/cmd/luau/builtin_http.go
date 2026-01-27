package main

import (
	"bytes"
	"io"
	"net/http"

	"github.com/haivivi/giztoy/pkg/luau"
)

// builtinHTTP implements __builtin.http(request) -> response
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
	url := state.ToString(-1)
	state.Pop(1)

	if url == "" {
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

	// Create HTTP request
	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return pushHTTPError(state, err)
	}

	// Read headers
	state.GetField(1, "headers")
	if state.IsTable(-1) {
		state.PushNil()
		for state.Next(-2) {
			key := state.ToString(-2)
			value := state.ToString(-1)
			if key != "" && value != "" {
				req.Header.Set(key, value)
			}
			state.Pop(1)
		}
	}
	state.Pop(1)

	// Execute request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return pushHTTPError(state, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Build response table
	state.NewTable()

	state.PushNumber(float64(resp.StatusCode))
	state.SetField(-2, "status")

	state.PushString(string(respBody))
	state.SetField(-2, "body")

	// Response headers
	state.NewTable()
	for k, v := range resp.Header {
		if len(v) > 0 {
			state.PushString(v[0])
			state.SetField(-2, k)
		}
	}
	state.SetField(-2, "headers")

	return 1
}

func pushHTTPError(state *luau.State, err error) int {
	state.NewTable()
	state.PushNumber(0)
	state.SetField(-2, "status")
	state.PushString(err.Error())
	state.SetField(-2, "err")
	return 1
}
