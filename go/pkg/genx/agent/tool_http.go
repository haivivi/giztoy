package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/haivivi/giztoy/pkg/genx"
	"github.com/haivivi/giztoy/pkg/genx/agentcfg"
)

// Default max response size: 1MB
const defaultMaxResponseSizeMB = 1

// HTTPTool is the runtime instance for HTTP tools.
// Created once at cortex startup, shared by all HTTP tool definitions.
type HTTPTool struct {
	rt     Runtime
	client *http.Client
}

// NewHTTPTool creates an HTTP tool instance.
// The client can be customized by cortex for connection pooling, timeouts, etc.
func NewHTTPTool(rt Runtime, client *http.Client) *HTTPTool {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPTool{rt: rt, client: client}
}

// CreateFuncTool creates a genx.FuncTool from agentcfg.HTTPTool.
func (t *HTTPTool) CreateFuncTool(def *agentcfg.HTTPTool) (*genx.FuncTool, error) {
	// Create FuncTool with map[string]any as argument type
	tool, err := genx.NewFuncTool[map[string]any](
		def.Name,
		def.Description,
		genx.InvokeFunc[map[string]any](func(ctx context.Context, call *genx.FuncCall, args map[string]any) (any, error) {
			return t.execute(ctx, def, args)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("tool %s: %w", def.Name, err)
	}

	return tool, nil
}

// Execute executes the HTTP request and returns the result.
// argsJSON is the raw JSON string from FuncCall.Arguments.
func (t *HTTPTool) Execute(ctx context.Context, def *agentcfg.HTTPTool, argsJSON string) (any, error) {
	var args map[string]any
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return nil, fmt.Errorf("parse args: %w", err)
		}
	}
	return t.execute(ctx, def, args)
}

// execute executes the HTTP request with parsed arguments.
func (t *HTTPTool) execute(ctx context.Context, def *agentcfg.HTTPTool, args map[string]any) (any, error) {
	// Build request body
	var reqBody io.Reader
	if def.ReqBodyJQ != nil {
		// Use jq to transform args into request body
		result, err := def.ReqBodyJQ.Run(args)
		if err != nil {
			return nil, fmt.Errorf("build request body: %w", err)
		}
		reqBody = bytes.NewReader([]byte(result))
	} else {
		// Default: use args as request body
		data, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("marshal args: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	// Expand environment variables in endpoint
	endpoint := expandEnvVars(def.Endpoint)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, string(def.Method), endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set default Content-Type
	req.Header.Set("Content-Type", "application/json")

	// Add custom headers
	for key, value := range def.Headers {
		req.Header.Set(key, expandEnvVars(value))
	}

	// Add Bearer authentication
	if def.Auth != nil && def.Auth.Type == "bearer" {
		token := expandEnvVars(def.Auth.Token)
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Execute request
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	// Determine max response size (convert MB to bytes)
	maxSizeMB := def.MaxResponseSizeMB
	if maxSizeMB <= 0 {
		maxSizeMB = defaultMaxResponseSizeMB
	}
	maxSize := maxSizeMB << 20 // MB to bytes

	// Limit response body size
	limitedBody := io.LimitReader(resp.Body, maxSize+1) // +1 to detect overflow

	// Check status code first
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read error body for error message (limited)
		errBody, _ := io.ReadAll(limitedBody)
		return nil, fmt.Errorf("http status %d: %s", resp.StatusCode, string(errBody))
	}

	// Decode JSON response directly from reader
	var respBody any
	decoder := json.NewDecoder(limitedBody)
	if err := decoder.Decode(&respBody); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Apply response jq if configured
	if def.RespBodyJQ != nil {
		result, err := def.RespBodyJQ.Run(respBody)
		if err != nil {
			return nil, fmt.Errorf("extract response: %w", err)
		}
		// Parse the JSON result back to any
		var parsed any
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			return result, nil // Return as string if not valid JSON
		}
		return parsed, nil
	}

	return respBody, nil
}

// expandEnvVars expands ${VAR} patterns in the string with environment variables.
func expandEnvVars(s string) string {
	return os.Expand(s, func(key string) string {
		return os.Getenv(key)
	})
}
