package agent

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/pkg/genx/agentcfg"
)

// mustParseJQ creates a JQExpr from a string expression, panicking on error.
func mustParseJQ(expr string) *agentcfg.JQExpr {
	jq := &agentcfg.JQExpr{}
	if err := jq.UnmarshalJSON([]byte(`"` + expr + `"`)); err != nil {
		panic(err)
	}
	return jq
}

func TestHTTPTool_Execute(t *testing.T) {
	// Create HTTP tool with timeout
	client := &http.Client{Timeout: 10 * time.Second}
	httpTool := NewHTTPTool(nil, client) // nil runtime is ok for simple HTTP requests

	t.Run("GET request - httpbin", func(t *testing.T) {
		def := &agentcfg.HTTPTool{
			ToolBase: agentcfg.ToolBase{
				Name:        "get_ip",
				Description: "Get IP address",
				Type:        agentcfg.ToolTypeHTTP,
			},
			Method:   "GET",
			Endpoint: "https://httpbin.org/ip",
		}

		tool, err := httpTool.CreateFuncTool(def)
		if err != nil {
			t.Fatalf("CreateFuncTool error: %v", err)
		}

		result, err := tool.Invoke(context.Background(), nil, "{}")
		if err != nil {
			t.Fatalf("Invoke error: %v", err)
		}

		t.Logf("Result: %v", result)

		// Should return {"origin": "x.x.x.x"}
		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("Expected map, got %T", result)
		}
		if _, ok := resultMap["origin"]; !ok {
			t.Errorf("Expected 'origin' field in response")
		}
	})

	t.Run("POST request with body", func(t *testing.T) {
		def := &agentcfg.HTTPTool{
			ToolBase: agentcfg.ToolBase{
				Name:        "post_data",
				Description: "Post data and echo back",
				Type:        agentcfg.ToolTypeHTTP,
			},
			Method:   "POST",
			Endpoint: "https://httpbin.org/post",
		}

		tool, err := httpTool.CreateFuncTool(def)
		if err != nil {
			t.Fatalf("CreateFuncTool error: %v", err)
		}

		result, err := tool.Invoke(context.Background(), nil, `{"name":"test","value":123}`)
		if err != nil {
			t.Fatalf("Invoke error: %v", err)
		}

		t.Logf("Result: %v", result)

		// httpbin echoes back the posted JSON in "json" field
		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("Expected map, got %T", result)
		}
		jsonData, ok := resultMap["json"].(map[string]any)
		if !ok {
			t.Errorf("Expected 'json' field in response")
		}
		if jsonData["name"] != "test" {
			t.Errorf("Expected name='test', got %v", jsonData["name"])
		}
	})

	t.Run("POST with req_body_jq transform", func(t *testing.T) {
		def := &agentcfg.HTTPTool{
			ToolBase: agentcfg.ToolBase{
				Name:        "post_transformed",
				Description: "Post with jq transform",
				Type:        agentcfg.ToolTypeHTTP,
			},
			Method:    "POST",
			Endpoint:  "https://httpbin.org/post",
			ReqBodyJQ: mustParseJQ(`{data: .}`), // Transform: wrap args in "data" field
		}

		tool, err := httpTool.CreateFuncTool(def)
		if err != nil {
			t.Fatalf("CreateFuncTool error: %v", err)
		}

		result, err := tool.Invoke(context.Background(), nil, `{"city":"Beijing"}`)
		if err != nil {
			t.Fatalf("Invoke error: %v", err)
		}

		t.Logf("Result: %v", result)

		resultMap := result.(map[string]any)
		jsonData := resultMap["json"].(map[string]any)
		dataField := jsonData["data"].(map[string]any)
		if dataField["city"] != "Beijing" {
			t.Errorf("Expected city='Beijing', got %v", dataField["city"])
		}
	})

	t.Run("GET with resp_body_jq extract", func(t *testing.T) {
		def := &agentcfg.HTTPTool{
			ToolBase: agentcfg.ToolBase{
				Name:        "get_headers_host",
				Description: "Get only the Host header",
				Type:        agentcfg.ToolTypeHTTP,
			},
			Method:     "GET",
			Endpoint:   "https://httpbin.org/headers",
			RespBodyJQ: mustParseJQ(`.headers.Host`), // Extract only the Host header
		}

		tool, err := httpTool.CreateFuncTool(def)
		if err != nil {
			t.Fatalf("CreateFuncTool error: %v", err)
		}

		result, err := tool.Invoke(context.Background(), nil, "{}")
		if err != nil {
			t.Fatalf("Invoke error: %v", err)
		}

		t.Logf("Result: %v", result)

		// Should return just "httpbin.org"
		if result != "httpbin.org" {
			t.Errorf("Expected 'httpbin.org', got %v", result)
		}
	})
}

func TestHTTPTool_Execute_Method(t *testing.T) {
	// Create HTTP tool with timeout
	client := &http.Client{Timeout: 10 * time.Second}
	httpTool := NewHTTPTool(nil, client)

	t.Run("Execute with empty args", func(t *testing.T) {
		def := &agentcfg.HTTPTool{
			ToolBase: agentcfg.ToolBase{
				Name:        "get_ip",
				Description: "Get IP address",
				Type:        agentcfg.ToolTypeHTTP,
			},
			Method:   "GET",
			Endpoint: "https://httpbin.org/ip",
		}

		result, err := httpTool.Execute(context.Background(), def, "")
		if err != nil {
			t.Fatalf("Execute error: %v", err)
		}

		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("Expected map, got %T", result)
		}
		if _, ok := resultMap["origin"]; !ok {
			t.Errorf("Expected 'origin' field in response")
		}
	})

	t.Run("Execute with JSON args", func(t *testing.T) {
		def := &agentcfg.HTTPTool{
			ToolBase: agentcfg.ToolBase{
				Name:        "post_data",
				Description: "Post data",
				Type:        agentcfg.ToolTypeHTTP,
			},
			Method:   "POST",
			Endpoint: "https://httpbin.org/post",
		}

		result, err := httpTool.Execute(context.Background(), def, `{"key":"value"}`)
		if err != nil {
			t.Fatalf("Execute error: %v", err)
		}

		resultMap := result.(map[string]any)
		jsonData := resultMap["json"].(map[string]any)
		if jsonData["key"] != "value" {
			t.Errorf("Expected key='value', got %v", jsonData["key"])
		}
	})

	t.Run("Execute with invalid JSON args", func(t *testing.T) {
		def := &agentcfg.HTTPTool{
			ToolBase: agentcfg.ToolBase{
				Name:        "test",
				Description: "Test",
				Type:        agentcfg.ToolTypeHTTP,
			},
			Method:   "GET",
			Endpoint: "https://httpbin.org/get",
		}

		_, err := httpTool.Execute(context.Background(), def, "invalid json")
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})
}

func TestJQExpr_Run_Unit(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		input    any
		expected string // JSON string result
	}{
		{
			name:     "simple field access",
			expr:     ".name",
			input:    map[string]any{"name": "test", "value": 123},
			expected: `"test"`,
		},
		{
			name:     "nested field",
			expr:     ".user.name",
			input:    map[string]any{"user": map[string]any{"name": "alice"}},
			expected: `"alice"`,
		},
		{
			name:     "wrap in object",
			expr:     `{data: .}`,
			input:    map[string]any{"foo": "bar"},
			expected: `{"data":{"foo":"bar"}}`,
		},
		{
			name:     "array first element",
			expr:     ".[0]",
			input:    []any{"a", "b", "c"},
			expected: `"a"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jq := mustParseJQ(tt.expr)
			result, err := jq.Run(tt.input)
			if err != nil {
				t.Fatalf("Run error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Result mismatch:\ngot:  %s\nwant: %s", result, tt.expected)
			}
		})
	}
}
